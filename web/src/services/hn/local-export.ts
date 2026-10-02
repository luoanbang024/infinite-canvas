import { isHNProjectId, localBackendURL } from "./local-reference";

export type HNExportItem = { exportId: string; sequenceIndex: number; sequenceItemId: string; shotId: string; candidateId: string; resultId: string; relativePath: string; sha256: string; byteLength: number; duration?: number };
export type HNExport = { schemaVersion: 1; exportId: string; projectId: string; sequenceId: string; bundleRelativePath: string; manifestJsonRelativePath: string; manifestCsvRelativePath: string; itemCount: number; items: HNExportItem[] };
type Dependencies = { request?: typeof fetch };
const safeId = (value: unknown): value is string => typeof value === "string" && isHNProjectId(value);
function validateExport(value: HNExport, projectId: string, sequenceId: string) {
    if (!value || value.schemaVersion !== 1 || value.projectId !== projectId || value.sequenceId !== sequenceId || !safeId(value.exportId)) throw new Error("HN 导出返回归属无效");
    const bundle = `exports/${value.exportId}`;
    if (value.bundleRelativePath !== bundle || value.manifestJsonRelativePath !== `${bundle}/ordered-manifest.json` || value.manifestCsvRelativePath !== `${bundle}/ordered-manifest.csv` || !Array.isArray(value.items) || value.items.length === 0 || value.itemCount !== value.items.length) throw new Error("HN 导出清单无效");
    const seen = new Set<string>(), paths = new Set<string>();
    for (const [index, item] of value.items.entries()) {
        if (!item || item.exportId !== value.exportId || item.sequenceIndex !== index + 1 || ![item.sequenceItemId, item.shotId, item.candidateId, item.resultId].every(safeId) || seen.has(item.sequenceItemId)) throw new Error("HN 导出 placement 或顺序无效");
        // Foundation generates a flat media directory; no path normalization or fallback.
        if (typeof item.relativePath !== "string" || !item.relativePath.startsWith(`${bundle}/media/${String(index + 1).padStart(3, "0")}_${item.resultId}.`) || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9]+$/.test(item.relativePath.slice(`${bundle}/media/`.length)) || paths.has(item.relativePath) || !/^[a-f0-9]{64}$/.test(item.sha256) || !Number.isSafeInteger(item.byteLength) || item.byteLength <= 0 || (item.duration !== undefined && (!Number.isFinite(item.duration) || item.duration < 0))) throw new Error("HN 导出媒体事实无效");
        seen.add(item.sequenceItemId); paths.add(item.relativePath);
    }
}

// Each explicit call creates a new bundle. Do not retry automatically after an
// uncertain response: the previous bundle may already have completed.
export async function exportLocalSequence(input: { projectId: string; sequenceId: string }, { request = fetch }: Dependencies = {}): Promise<HNExport> {
    const { projectId, sequenceId } = input;
    if (![projectId, sequenceId].every(safeId)) throw new Error("HN 导出标识无效");
    const discovery = await request("/api/hn/local-endpoint", { credentials: "omit", cache: "no-store" });
    const endpoint = await discovery.json() as { code: number; data?: { url: string } };
    if (!discovery.ok || endpoint.code !== 0 || !endpoint.data) throw new Error("HN 导出需要本机后端地址");
    const response = await request(`${localBackendURL(endpoint.data.url)}/api/hn/projects/${projectId}/sequences/${sequenceId}/export`, { method: "POST", credentials: "omit", headers: { "Content-Type": "application/json", "X-HN-Local-Request": "1" }, body: "{}" });
    const result = await response.json() as { code: number; data?: HNExport };
    if (!response.ok || result.code !== 0 || !result.data) throw new Error("HN 本地导出未完成");
    validateExport(result.data, projectId, sequenceId);
    return result.data;
}
