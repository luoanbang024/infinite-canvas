import { freezeLocalReference, isHNProjectId, localBackendURL } from "./local-reference";

// SOURCE_BASELINE_REVIEW_REQUIRED_ON_NEXT_BASELINE_ADOPTION.
export const HN_GENERATION_SOURCE_BASELINE = "418ffbde3dbea33d374356588cb672336ec38353";
export type HNReferenceRole = "reference" | "firstFrame" | "lastFrame";
export type HNReferenceBinding = { referenceVersionId: string; sha256: string; role: HNReferenceRole };
export type HNGenerationPrepareInput = {
    projectId: string; nodeId: string; promptSnapshot: string;
    protocol?: string; providerIdentity?: string; model?: string; connectionId?: string;
    parameters: Record<string, string | { prompt: string; duration: string }[]>;
    references: { storageKey: string; role: HNReferenceRole }[];
    sourceBaseline: string;
};
export type HNPreparedGeneration = {
    generationId: string; projectId: string; nodeId: string; sourceBaseline: string;
    frozen: true; frozenHash: string; status: "PREPARED"; submissionState: "PREPARED";
    referenceBindings: HNReferenceBinding[]; createdAt: string;
};
type Dependencies = { readBlob: (key: string) => Promise<Blob | null>; request?: typeof fetch };

const unsafeText = /(https?:\/\/|data:|bearer\s|-----BEGIN|sk-proj-|ghp_|github_pat_|api[_-]?key\s*[:=]|(?:token|password|secret|authorization|cookie|credential|signature)\s*[:=])/i;
const parameterNames = new Set(["size", "videoSeconds", "vquality", "videoMode", "videoNegativePrompt", "videoMultiShot", "videoShotType", "videoGenerateAudio", "videoWatermark", "videoCharacterOrientation"]);
function safeText(value: unknown, max = 32768): asserts value is string {
    if (typeof value !== "string" || value.length > max || unsafeText.test(value)) throw new Error("HN 准备只接受非敏感业务文本，不接受 URL 或凭据");
}

// Closed vocabulary + sorted object keys. Array order is part of the request identity.
export function canonicalHNParameters(parameters: HNGenerationPrepareInput["parameters"]) {
    if (!parameters || Array.isArray(parameters) || typeof parameters !== "object") throw new Error("参数必须是 JSON 对象");
    const result: HNGenerationPrepareInput["parameters"] = {};
    for (const key of Object.keys(parameters).sort()) {
        const value = parameters[key];
        if (parameterNames.has(key)) { safeText(value); result[key] = value; }
        else if (key === "videoMultiPrompt") {
            if (!Array.isArray(value) || value.length > 64) throw new Error("多段提示词无效");
            result[key] = value.map((shot) => {
                if (!shot || Object.keys(shot).sort().join(",") !== "duration,prompt") throw new Error("多段提示词含不支持字段");
                safeText(shot.prompt); safeText(shot.duration);
                return { duration: shot.duration, prompt: shot.prompt };
            });
        } else throw new Error("HN 准备参数含不支持字段");
    }
    return result;
}

// An explicit call creates a new attempt. No Canvas mutation, submit, poll or retry.
export async function prepareLocalGeneration(input: HNGenerationPrepareInput, dependencies: Dependencies): Promise<HNPreparedGeneration> {
    if (!isHNProjectId(input.projectId)) throw new Error("项目标识不兼容；需审核 ID 映射");
    if (input.sourceBaseline !== HN_GENERATION_SOURCE_BASELINE) throw new Error("Generation 基线需要审核");
    safeText(input.nodeId, 128); safeText(input.promptSnapshot);
    if (!input.nodeId.trim() || !input.promptSnapshot.trim()) throw new Error("HN 准备需要节点标识及非空提示词");
    for (const value of [input.protocol, input.providerIdentity, input.model, input.connectionId]) if (value !== undefined) safeText(value, 256);
    const parameters = canonicalHNParameters(input.parameters);
    if (!Array.isArray(input.references) || input.references.length > 32) throw new Error("参考列表无效");
    input = { ...input, parameters, references: input.references.map(({ storageKey, role }) => ({ storageKey, role })) };
    // Preflight all Blobs before any write; never partially freeze an unsupported bundle.
    const blobs = new Map<string, Blob>();
    for (const reference of input.references) {
        if (!["reference", "firstFrame", "lastFrame"].includes(reference.role) || !reference.storageKey?.startsWith("image:")) throw new Error("R4 仅支持具有精确本地 Blob 的图片参考");
        const blob = await dependencies.readBlob(reference.storageKey);
        if (!blob || !blob.size || blob.size > 16 * 1024 * 1024) throw new Error("参考本地图片已丢失、为空或超过 16 MiB");
        blobs.set(reference.storageKey, blob);
    }
    const request = dependencies.request || fetch;
    const referenceBindings: HNReferenceBinding[] = [];
    for (const reference of input.references) {
        const version = await freezeLocalReference({ projectId: input.projectId, storageKey: reference.storageKey }, { readBlob: async (key) => blobs.get(key) || null, request });
        if (version.projectId !== input.projectId || version.kind !== "image" || !isHNProjectId(version.id) || !/^[a-f0-9]{64}$/.test(version.sha256)) throw new Error("ReferenceVersion 返回标识或哈希无效");
        referenceBindings.push({ referenceVersionId: version.id, sha256: version.sha256, role: reference.role });
    }
    const discovery = await request("/api/hn/local-endpoint", { credentials: "omit", cache: "no-store" });
    const endpoint = await discovery.json() as { code: number; data?: { url: string } };
    if (!discovery.ok || endpoint.code !== 0 || !endpoint.data) throw new Error("HN 准备需要本机后端地址");
    const base = localBackendURL(endpoint.data.url);
    // Explicit projection: extra caller properties and broad configs never cross this boundary.
    const payload = { nodeId: input.nodeId, promptSnapshot: input.promptSnapshot, protocol: input.protocol, providerIdentity: input.providerIdentity, model: input.model, connectionId: input.connectionId, parameters, referenceBindings, sourceBaseline: input.sourceBaseline };
    const response = await request(`${base}/api/hn/projects/${encodeURIComponent(input.projectId)}/generations/prepare`, {
        method: "POST", credentials: "omit", headers: { "Content-Type": "application/json", "X-HN-Local-Request": "1" }, body: JSON.stringify(payload),
    });
    const result = await response.json() as { code: number; data?: HNPreparedGeneration; msg?: string };
    if (!response.ok || result.code !== 0 || !result.data) throw new Error(result.msg || "HN Generation 准备失败");
    const generation = result.data;
    if (!isHNProjectId(generation.generationId) || generation.projectId !== input.projectId || generation.nodeId !== input.nodeId || generation.sourceBaseline !== input.sourceBaseline || generation.frozen !== true || generation.status !== "PREPARED" || generation.submissionState !== "PREPARED" || !/^[a-f0-9]{64}$/.test(generation.frozenHash) || JSON.stringify(generation.referenceBindings) !== JSON.stringify(referenceBindings)) throw new Error("HN 冻结 Generation 返回状态或绑定无效");
    return generation;
}
