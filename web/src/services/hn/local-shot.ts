import { isHNProjectId, localBackendURL } from "./local-reference";

export type HNShot = { shotId: string; projectId: string; sourceNodeId: string; label: string; createdAt: string; updatedAt: string };
export type HNShotInput = { projectId: string; sourceNodeId: string; label?: string };
const unsafeText = /([a-z][a-z0-9+.-]*:\/\/|^(?:[a-z]:[\\/]|[\\/])|[\x00-\x1f\x7f]|^(?:data|blob):|bearer\s|-----BEGIN|sk-proj-|ghp_|github_pat_|api[_-]?key\s*[:=]|(?:token|password|secret|authorization|cookie|credential|signature)\s*[:=])/i;
function safeText(value: unknown, max: number): value is string {
    if (typeof value !== "string") return false;
    const bytes = new TextEncoder().encode(value);
    return bytes.length <= max && new TextDecoder().decode(bytes) === value && !unsafeText.test(value);
}
export function validateHNShotInput(input: HNShotInput) {
    if (typeof input.projectId !== "string" || !isHNProjectId(input.projectId) || !safeText(input.sourceNodeId, 128) || !input.sourceNodeId.trim() || !safeText(input.label ?? "", 256)) throw new Error("HN Shot 需要有效项目、来源节点及非敏感标签");
}

// Explicit local ensure; neither Canvas metadata nor an existing Shot label is changed.
export async function ensureLocalShot(input: HNShotInput, { request = fetch }: { request?: typeof fetch } = {}): Promise<HNShot> {
    validateHNShotInput(input);
    const { projectId, sourceNodeId, label = "" } = input;
    const discovery = await request("/api/hn/local-endpoint", { credentials: "omit", cache: "no-store" });
    const endpoint = await discovery.json() as { code: number; data?: { url: string } };
    if (!discovery.ok || endpoint.code !== 0 || !endpoint.data) throw new Error("HN Shot 需要本机后端地址");
    const response = await request(`${localBackendURL(endpoint.data.url)}/api/hn/projects/${encodeURIComponent(projectId)}/shots/ensure`, {
        method: "POST", credentials: "omit", headers: { "Content-Type": "application/json", "X-HN-Local-Request": "1" }, body: JSON.stringify({ sourceNodeId, label }),
    });
    const result = await response.json() as { code: number; data?: HNShot; msg?: string };
    if (!response.ok || result.code !== 0 || !result.data) throw new Error(result.msg || "HN Shot ensure 失败");
    const shot = result.data;
    const timestamp = (value: unknown) => typeof value === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(value) && Number.isFinite(Date.parse(value));
    if (typeof shot.shotId !== "string" || !isHNProjectId(shot.shotId) || shot.projectId !== projectId || shot.sourceNodeId !== sourceNodeId || !safeText(shot.label, 256) || !timestamp(shot.createdAt) || !timestamp(shot.updatedAt)) throw new Error("HN Shot 返回标识或元数据无效");
    return shot;
}
