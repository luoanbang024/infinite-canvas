import { canonicalHNParameters, HN_GENERATION_SOURCE_BASELINE, prepareLocalShotGeneration, type HNGenerationPrepareInput } from "@/services/hn/local-generation";
import { ensureLocalShot, validateHNShotInput } from "@/services/hn/local-shot";
import { isHNProjectId, localBackendURL } from "@/services/hn/local-reference";
import { CanvasNodeType, type CanvasConnection, type CanvasNodeData, type HNLocalPreparedReceipt } from "../types";
import { applyCameraPrompt } from "../utils/canvas-camera";
import { buildNodeGenerationContext, buildNodeGenerationInputs } from "./canvas-node-generation";

export type HNLocalPhase = "UNPREPARED" | "PREPARING" | "CURRENT" | "STALE" | "RELOADED_UNVERIFIED" | "ERROR" | "OUTCOME_UNKNOWN";
export const HN_LOCAL_EXPLANATION = "仅冻结本地业务请求，不调用模型、不生成视频。渠道未绑定。";
export const HN_LOCAL_CONFIRMATION = "这会创建新的 Generation，保留已有准备记录。是否继续？";
export const HN_LOCAL_ERRORS = {
    HN_PROJECT_ID_UNSUPPORTED: "当前画布项目标识不支持本地准备。",
    HN_SOURCE_NODE_UNSUPPORTED: "请选择没有生成任务的空视频节点进行本地准备。",
    HN_LOCAL_BACKEND_UNAVAILABLE: "本地准备服务不可用，请检查本机服务和项目目录配置。",
    HN_REFERENCE_BUNDLE_UNSUPPORTED: "本地准备仅支持本地图片参考，暂不支持远端、视频、音频或工作流素材包。",
    HN_LOCAL_REFERENCE_MISSING: "本地参考图片已丢失、为空或超过限制，请重新选择。",
    HN_SHOT_IDENTITY_CONFLICT: "本地镜头身份冲突，已停止准备。",
    HN_PREPARE_REJECTED: "本地准备未完成，请检查提示词和本地参考。",
    HN_PREPARED_STALE: "内容已变更，需重新本地准备。",
    HN_PREPARE_OUTCOME_UNKNOWN: "准备结果未确认；不会自动重试。",
    HN_RECEIPT_UNVERIFIED: "历史准备回执，未复核。",
} as const;
export class HNLocalError extends Error {
    constructor(public readonly code: keyof typeof HN_LOCAL_ERRORS) { super(HN_LOCAL_ERRORS[code]); }
}
function fail(code: keyof typeof HN_LOCAL_ERRORS): never { throw new HNLocalError(code); }
const unsafe = /(https?:\/\/|data:|bearer\s|-----BEGIN|sk-proj-|ghp_|github_pat_|api[_-]?key\s*[:=]|(?:token|password|secret|authorization|cookie|credential|signature)\s*[:=])/i;
function safeText(value: unknown, max: number): value is string {
    if (typeof value !== "string") return false;
    const bytes = new TextEncoder().encode(value);
    return bytes.length <= max && new TextDecoder().decode(bytes) === value && !unsafe.test(value);
}
const hashText = (value: unknown): value is string => typeof value === "string" && /^[a-f0-9]{64}$/.test(value);
function timestamp(value: unknown): value is string {
    return typeof value === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(value) && Number.isFinite(Date.parse(value)) && new Date(value).toISOString().slice(0, 19) === value.slice(0, 19);
}
export async function hnLocalSHA256(bytes: BufferSource) {
    return Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)), (n) => n.toString(16).padStart(2, "0")).join("");
}
export async function mapHNCanvasProjectId(id: string) {
    if (typeof id !== "string" || !/^[A-Za-z0-9_-]{1,128}$/.test(id)) fail("HN_PROJECT_ID_UNSUPPORTED");
    return `canvas-${await hnLocalSHA256(new TextEncoder().encode(id))}`;
}
export function canHNLocalPrepare(node: CanvasNodeData | null | undefined): node is CanvasNodeData {
    if (!node || node.type !== CanvasNodeType.Video) return false;
    const m = node.metadata;
    if (m?.content || m?.workflowRef || m?.videoTaskId || m?.videoTaskVideoId || m?.status !== undefined && m.status !== "idle") return false;
    try { validateHNShotInput({ projectId: "preflight", sourceNodeId: node.id, label: "Shot" }); return true; } catch { return false; }
}

// Closed business facts only; no runtime config/channel/store dependency.
export type HNLocalVideoBusiness = {
    model: string; size: string; videoSeconds: string; vquality: string; videoMode: string;
    videoNegativePrompt: string; videoMultiShot: string; videoShotType: string;
    videoMultiPrompt: { prompt: string; duration: string }[];
    videoGenerateAudio: string; videoWatermark: string; videoCharacterOrientation: string;
};
export type HNLocalIntent = {
    canvasProjectId: string; nodeId: string; promptSnapshot: string; model: string;
    parameters: HNGenerationPrepareInput["parameters"];
    references: { storageKey: string; role: "reference" | "firstFrame" | "lastFrame"; nodeId: string }[];
    receipt?: HNLocalPreparedReceipt;
};
export function captureHNLocalIntent(canvasProjectId: string, nodeId: string, nodes: CanvasNodeData[], connections: CanvasConnection[], business: HNLocalVideoBusiness): HNLocalIntent {
    if (!/^[A-Za-z0-9_-]{1,128}$/.test(canvasProjectId)) fail("HN_PROJECT_ID_UNSUPPORTED");
    const node = nodes.find((n) => n.id === nodeId);
    if (!canHNLocalPrepare(node)) fail("HN_SOURCE_NODE_UNSUPPORTED");
    const m = node.metadata;
    const inputs = buildNodeGenerationInputs(nodeId, nodes, connections);
    const context = buildNodeGenerationContext(nodeId, nodes, connections, m?.prompt || "");
    const selectedIds = [m?.firstFrameNodeId, m?.lastFrameNodeId, ...(m?.klingImageNodeIds || [])].filter((id): id is string => Boolean(id));
    if (inputs.some((input) => input.type === "video" || input.type === "audio") || context.referenceVideos.length || context.referenceAudios.length || context.videoElementList.length || m?.klingElementList?.some((e) => e.name || e.description || e.nodeIds?.length) || m?.references?.length) fail("HN_REFERENCE_BUNDLE_UNSUPPORTED");
    if (selectedIds.some((id) => !inputs.some((input) => input.nodeId === id && input.image?.storageKey?.startsWith("image:")))) fail("HN_REFERENCE_BUNDLE_UNSUPPORTED");
    // Connected media without content must not silently become text inputs.
    const configId = connections.find((c) => c.fromNodeId === nodeId && nodes.some((n) => n.id === c.toNodeId && n.type === CanvasNodeType.Config))?.toNodeId;
    const resourceIds = connections.filter((c) => c.toNodeId === (configId || nodeId) && c.fromNodeId !== nodeId).map((c) => c.fromNodeId);
    if (resourceIds.some((id) => { const n = nodes.find((item) => item.id === id); return !n || n.type === CanvasNodeType.Video || n.type === CanvasNodeType.Audio || (n.type === CanvasNodeType.Image || n.type === CanvasNodeType.Panorama) && (!n.metadata?.content || !n.metadata.storageKey?.startsWith("image:")); })) fail("HN_REFERENCE_BUNDLE_UNSUPPORTED");
    const references: HNLocalIntent["references"] = [];
    const add = (image: typeof context.firstFrame, role: HNLocalIntent["references"][number]["role"]) => {
        if (!image) return;
        if (!image.storageKey?.startsWith("image:") || !safeText(image.id, 128)) fail("HN_REFERENCE_BUNDLE_UNSUPPORTED");
        references.push({ storageKey: image.storageKey, role, nodeId: image.id });
    };
    context.referenceImages.forEach((image) => add(image, "reference")); add(context.firstFrame, "firstFrame"); add(context.lastFrame, "lastFrame");
    if (references.length > 32) fail("HN_REFERENCE_BUNDLE_UNSUPPORTED");
    const promptSnapshot = applyCameraPrompt(context.prompt.trim(), m?.cameraControl);
    const model = m?.model ?? business.model;
    if (!safeText(promptSnapshot, 32768) || !promptSnapshot.trim() || !safeText(model, 256)) fail("HN_PREPARE_REJECTED");
    let parameters: HNLocalIntent["parameters"];
    try {
        parameters = canonicalHNParameters({ size: m?.size ?? business.size, videoSeconds: m?.seconds ?? business.videoSeconds, vquality: m?.vquality ?? business.vquality, videoMode: m?.mode ?? business.videoMode, videoNegativePrompt: m?.negativePrompt ?? business.videoNegativePrompt, videoMultiShot: m?.multiShot ?? business.videoMultiShot, videoShotType: m?.shotType ?? business.videoShotType, videoMultiPrompt: (context.videoMultiPrompt.length ? context.videoMultiPrompt : business.videoMultiPrompt).map(({ prompt, duration }) => ({ prompt, duration })), videoGenerateAudio: m?.generateAudio ?? business.videoGenerateAudio, videoWatermark: m?.watermark ?? business.videoWatermark, videoCharacterOrientation: m?.characterOrientation ?? business.videoCharacterOrientation });
        for (const value of Object.values(parameters)) if (typeof value === "string" ? !safeText(value, 32768) : value.some((s) => !safeText(s.prompt, 32768) || !safeText(s.duration, 32768))) fail("HN_PREPARE_REJECTED");
    } catch { fail("HN_PREPARE_REJECTED"); }
    if (![parameters.size, parameters.videoSeconds, parameters.vquality].every((v) => safeText(v, 256))) fail("HN_PREPARE_REJECTED");
    return { canvasProjectId, nodeId, promptSnapshot, model, parameters, references, receipt: m?.hnLocalPrepared };
}
export type HNLocalDependencies = { readBlob: (key: string) => Promise<Blob | null>; request?: typeof fetch };
export async function preflightHNLocalIntent(intent: HNLocalIntent, readBlob: HNLocalDependencies["readBlob"]) {
    intent = { ...intent, parameters: canonicalHNParameters(intent.parameters), references: intent.references.map((r) => ({ ...r })) };
    const projectId = await mapHNCanvasProjectId(intent.canvasProjectId);
    const blobs = new Map<string, Blob>();
    const referenceFacts = [];
    for (const reference of intent.references) {
        let blob = blobs.get(reference.storageKey);
        try { blob ??= await readBlob(reference.storageKey) ?? undefined; } catch { fail("HN_LOCAL_REFERENCE_MISSING"); }
        if (!blob || !blob.size || blob.size > 16 * 1024 * 1024) fail("HN_LOCAL_REFERENCE_MISSING");
        blobs.set(reference.storageKey, blob);
        referenceFacts.push({ role: reference.role, storageKey: reference.storageKey, nodeId: reference.nodeId, sha256: await hnLocalSHA256(await blob.arrayBuffer()), byteLength: blob.size, mimeType: blob.type });
    }
    const input: HNGenerationPrepareInput = { projectId, nodeId: intent.nodeId, promptSnapshot: intent.promptSnapshot, model: intent.model, protocol: "", providerIdentity: "", parameters: canonicalHNParameters(intent.parameters), sourceBaseline: HN_GENERATION_SOURCE_BASELINE, references: intent.references.map(({ storageKey, role }) => ({ storageKey, role })) };
    const fingerprint = await hnLocalSHA256(new TextEncoder().encode(JSON.stringify({ schemaVersion: 1, canvasProjectId: intent.canvasProjectId, hnProjectId: projectId, sourceNodeId: input.nodeId, promptSnapshot: input.promptSnapshot, model: input.model, protocol: "", providerIdentity: "", parameters: input.parameters, sourceBaseline: input.sourceBaseline, references: referenceFacts })));
    return { input, blobs, fingerprint, referenceFacts };
}
export function validHNLocalReceipt(value: unknown, canvasProjectId: string, nodeId: string, hnProjectId: string): value is HNLocalPreparedReceipt {
    if (!value || typeof value !== "object" || Array.isArray(value)) return false;
    const r = value as HNLocalPreparedReceipt;
    if (Object.keys(r).sort().join(",") !== "canvasProjectId,frozenHash,generationId,hnProjectId,preparedAt,requestFingerprint,shotId,snapshot,sourceBaseline,sourceNodeId,version") return false;
    if (r.version !== 1 || r.canvasProjectId !== canvasProjectId || r.hnProjectId !== hnProjectId || r.sourceNodeId !== nodeId || typeof r.shotId !== "string" || !isHNProjectId(r.shotId) || typeof r.generationId !== "string" || !isHNProjectId(r.generationId) || !hashText(r.frozenHash) || !hashText(r.requestFingerprint) || !timestamp(r.preparedAt) || r.sourceBaseline !== HN_GENERATION_SOURCE_BASELINE) return false;
    const s = r.snapshot;
    return !!s && typeof s === "object" && Object.keys(s).sort().join(",") === "model,referenceCount,seconds,size,vquality" && [s.model, s.seconds, s.vquality, s.size].every((v) => safeText(v, 256)) && Number.isInteger(s.referenceCount) && s.referenceCount >= 0 && s.referenceCount <= 32;
}
export function mergeHNLocalReceipt(nodes: CanvasNodeData[], receipt: HNLocalPreparedReceipt, activeProjectId: string) {
    if (receipt.canvasProjectId !== activeProjectId) return nodes;
    return nodes.map((node) => node.id === receipt.sourceNodeId && canHNLocalPrepare(node) ? { ...node, metadata: { ...node.metadata, hnLocalPrepared: receipt } } : node);
}
export type HNLocalEntry = { phase: HNLocalPhase; running: boolean; attempted: boolean; receipt?: HNLocalPreparedReceipt; error?: keyof typeof HN_LOCAL_ERRORS; sessionGenerationId?: string; valid: boolean; inspection: number };

// Owned once by the Canvas page, never by a disposable dialog.
export class HNLocalPrepareController {
    private entries = new Map<string, HNLocalEntry>();
    private alive = true;
    private epoch = 0;
    constructor(private notify: () => void = () => {}) {}
    activate() { this.alive = true; }
    dispose() { this.alive = false; this.epoch++; }
    key(project: string, node: string) { return JSON.stringify([project, node]); }
    entry(project: string, node: string): HNLocalEntry {
        const key = this.key(project, node);
        if (!this.entries.has(key)) this.entries.set(key, { phase: "UNPREPARED", running: false, attempted: false, valid: true, inspection: 0 });
        return this.entries.get(key)!;
    }
    syncTargets(project: string, nodes: CanvasNodeData[]) {
        for (const [key, entry] of this.entries) { const [p, id] = JSON.parse(key); if (p !== project || !nodes.some((n) => n.id === id && canHNLocalPrepare(n))) entry.valid = false; }
    }
    async inspect(intent: HNLocalIntent, dependencies: HNLocalDependencies) {
        const e = this.entry(intent.canvasProjectId, intent.nodeId); if (e.running) return;
        const revision = ++e.inspection, epoch = this.epoch;
        const project = await mapHNCanvasProjectId(intent.canvasProjectId);
        const receipt = validHNLocalReceipt(intent.receipt, intent.canvasProjectId, intent.nodeId, project) ? intent.receipt : undefined;
        let phase: HNLocalPhase = "UNPREPARED";
        if (receipt) {
            try { const current = await preflightHNLocalIntent(intent, dependencies.readBlob); phase = current.fingerprint !== receipt.requestFingerprint ? "STALE" : receipt.generationId === e.sessionGenerationId ? "CURRENT" : "RELOADED_UNVERIFIED"; }
            catch { phase = "STALE"; }
        }
        if (this.alive && epoch === this.epoch && revision === e.inspection && !e.running) { e.receipt = receipt; if (!e.error) e.phase = phase; this.notify(); }
    }
    reject(project: string, node: string, error: unknown) {
        const e = this.entry(project, node); if (e.running) return;
        e.error = error instanceof HNLocalError ? error.code : "HN_PREPARE_REJECTED";
        e.phase = e.receipt ? "STALE" : "ERROR"; this.notify();
    }
    async prepare(readCurrent: () => HNLocalIntent, dependencies: HNLocalDependencies, confirm: () => Promise<boolean>, onReceipt: (receipt: HNLocalPreparedReceipt) => void) {
        // Capture and lock synchronously, including the confirmation lifetime.
        let intent: HNLocalIntent;
        try { const current = readCurrent(); intent = { ...current, parameters: canonicalHNParameters(current.parameters), references: current.references.map((r) => ({ ...r })) }; } catch { return false; }
        const e = this.entry(intent.canvasProjectId, intent.nodeId);
        if (!this.alive || e.running) return false;
        e.running = true; e.valid = true; const epoch = this.epoch;
        const previousPhase = e.phase; e.phase = "PREPARING"; e.inspection++; this.notify();
        let prepareSent = false, rejected = false;
        try {
            const mapped = await mapHNCanvasProjectId(intent.canvasProjectId);
            const prior = validHNLocalReceipt(intent.receipt, intent.canvasProjectId, intent.nodeId, mapped);
            if ((prior || e.attempted) && !await confirm()) { e.phase = previousPhase; return false; }
            if (!this.alive || epoch !== this.epoch || !e.valid) { e.phase = previousPhase; return false; }
            e.attempted = true;
            const frozen = await preflightHNLocalIntent(intent, dependencies.readBlob);
            if (!this.alive || epoch !== this.epoch || !e.valid) { e.phase = previousPhase; return false; }
            let referenceResponseIndex = 0;
            const request: typeof fetch = async (url, options) => {
                const method = options?.method || "GET";
                if (url === "/api/hn/local-endpoint" && method === "GET") {
                    try {
                        const response = await (dependencies.request || fetch)(url, options);
                        const endpoint = await response.clone().json();
                        if (!response.ok || endpoint.code !== 0 || !endpoint.data || typeof endpoint.data.url !== "string") fail("HN_LOCAL_BACKEND_UNAVAILABLE");
                        localBackendURL(endpoint.data.url);
                        return response;
                    } catch { fail("HN_LOCAL_BACKEND_UNAVAILABLE"); }
                }
                if (typeof url !== "string") fail("HN_PREPARE_REJECTED");
                const parsed = new URL(url); localBackendURL(parsed.origin);
                if (method !== "POST" || !["shots/ensure", "references", "generations/prepare"].some((path) => parsed.pathname === `/api/hn/projects/${frozen.input.projectId}/${path}`) || parsed.search || parsed.hash || parsed.username || parsed.password) fail("HN_PREPARE_REJECTED");
                if (parsed.pathname.endsWith("/generations/prepare")) {
                    if (prepareSent) fail("HN_PREPARE_REJECTED");
                    prepareSent = true;
                }
                const response = await (dependencies.request || fetch)(url, options);
                if (prepareSent && [400, 403].includes(response.status)) rejected = true;
                if (response.status === 409 && parsed.pathname.endsWith("/shots/ensure")) fail("HN_SHOT_IDENTITY_CONFLICT");
                if (response.status === 503 && !prepareSent) fail("HN_LOCAL_BACKEND_UNAVAILABLE");
                if (response.ok && parsed.pathname.endsWith("/references")) {
                    try {
                        const version = await response.clone().json();
                        if (version.code !== 0 || version.data?.sha256 !== frozen.referenceFacts[referenceResponseIndex++]?.sha256) fail("HN_PREPARE_REJECTED");
                    } catch { fail("HN_PREPARE_REJECTED"); }
                }
                return response;
            };
            const shot = await ensureLocalShot({ projectId: frozen.input.projectId, sourceNodeId: intent.nodeId, label: "Shot" }, { request });
            const generation = await prepareLocalShotGeneration({ ...frozen.input, shotId: shot.shotId }, { request, readBlob: async (key) => frozen.blobs.get(key) || null });
            if (!timestamp(generation.createdAt)) fail("HN_PREPARE_REJECTED");
            if (generation.referenceBindings.some((b, i) => b.sha256 !== frozen.referenceFacts[i]?.sha256)) fail("HN_PREPARE_REJECTED");
            const p = frozen.input.parameters;
            const receipt: HNLocalPreparedReceipt = { version: 1, canvasProjectId: intent.canvasProjectId, hnProjectId: frozen.input.projectId, sourceNodeId: intent.nodeId, shotId: shot.shotId, generationId: generation.generationId, frozenHash: generation.frozenHash, preparedAt: generation.createdAt, sourceBaseline: HN_GENERATION_SOURCE_BASELINE, requestFingerprint: frozen.fingerprint, snapshot: { model: intent.model, seconds: p.videoSeconds as string, vquality: p.vquality as string, size: p.size as string, referenceCount: generation.referenceBindings.length } };
            if (!validHNLocalReceipt(receipt, intent.canvasProjectId, intent.nodeId, mapped)) fail("HN_PREPARE_REJECTED");
            let current = "", targetCurrent = false;
            try { const now = readCurrent(); if (now.canvasProjectId === intent.canvasProjectId && now.nodeId === intent.nodeId) { targetCurrent = true; current = (await preflightHNLocalIntent(now, dependencies.readBlob)).fingerprint; } } catch { /* Old frozen facts remain historical. */ }
            if (this.alive && epoch === this.epoch && e.valid) {
                e.error = undefined; e.receipt = receipt; e.sessionGenerationId = receipt.generationId;
                e.phase = current === frozen.fingerprint ? "CURRENT" : "STALE"; if (targetCurrent) onReceipt(receipt);
            }
            return true;
        } catch (error) {
            if (this.alive && epoch === this.epoch) {
                e.error = prepareSent && !rejected ? "HN_PREPARE_OUTCOME_UNKNOWN" : error instanceof HNLocalError ? error.code : "HN_PREPARE_REJECTED";
                e.phase = e.error === "HN_PREPARE_OUTCOME_UNKNOWN" ? "OUTCOME_UNKNOWN" : "ERROR";
            }
            return false;
        } finally { e.running = false; if (this.alive && epoch === this.epoch) this.notify(); }
    }
}
