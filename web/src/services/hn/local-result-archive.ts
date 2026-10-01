import { isHNProjectId, localBackendURL } from "./local-reference";

export const HN_ARCHIVE_MAX_BYTES = 64 * 1024 * 1024;
export type LocalArchiveInput = { projectId: string; generationId: string; storageKey: string };
export type LocalArchiveRetryInput = { projectId: string; archiveJobId: string; storageKey: string };
export type LocalArchiveFacts = {
    resultId: string; generationId: string; archiveJobId: string; resultStatus: string; archiveStatus: string;
    archivedRelativePath: string; sha256: string; byteLength: number; mimeType: string;
};
type Dependencies = { readBlob?: (key: string) => Promise<Blob | null>; request?: typeof fetch };
export class LocalArchiveFailure extends Error {
    constructor(public readonly archive: LocalArchiveFacts) { super("本地归档未完成；请使用同一个 ArchiveJob 显式重试"); }
}

function validateFacts(value: LocalArchiveFacts, id: string, retry: boolean, mime: string) {
    if (!value || ![value.resultId, value.generationId, value.archiveJobId].every((id) => typeof id === "string" && isHNProjectId(id)) || (retry ? value.archiveJobId : value.generationId) !== id || value.mimeType !== mime) throw new Error("归档返回标识无效");
}

async function archive(projectId: string, id: string, storageKey: string, retry: boolean, dependencies: Dependencies): Promise<LocalArchiveFacts> {
    if (!isHNProjectId(projectId) || !isHNProjectId(id)) throw new Error("项目或归档标识不兼容");
    if (!/^(video|file):[^\s:/\\?&#]+$/.test(storageKey)) throw new Error("R5 只接受 video:/file: 本地持久化视频，不接受远端或 server 来源");
    const readBlob = dependencies.readBlob || (await import("@/services/file-storage")).getMediaBlob;
    const blob = await readBlob(storageKey);
    if (!blob || !blob.size || blob.size > HN_ARCHIVE_MAX_BYTES || !["video/mp4", "video/webm"].includes(blob.type)) throw new Error("需要非空且不超过 64 MiB 的本地 MP4/WebM Blob");
    // Known input expectation; foundation remains the authoritative streamed byte/hash verifier.
    const sha256 = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", await blob.arrayBuffer())), (n) => n.toString(16).padStart(2, "0")).join("");
    const request = dependencies.request || fetch;
    const discovery = await request("/api/hn/local-endpoint", { credentials: "omit", cache: "no-store" });
    const endpoint = await discovery.json() as { code: number; data?: { url: string } };
    if (!discovery.ok || endpoint.code !== 0 || !endpoint.data) throw new Error("HN 归档需要本机后端地址");
    const base = localBackendURL(endpoint.data.url);
    const form = new FormData();
    form.append("file", blob, "local-video.bin"); form.append("resultKind", "video"); form.append("mimeType", blob.type); form.append("sha256", sha256);
    const suffix = retry ? `archive-jobs/${encodeURIComponent(id)}/retry` : `generations/${encodeURIComponent(id)}/results/local-archive`;
    const response = await request(`${base}/api/hn/projects/${encodeURIComponent(projectId)}/${suffix}`, { method: "POST", credentials: "omit", headers: { "X-HN-Local-Request": "1" }, body: form });
    const payload = await response.json() as { code: number; data?: LocalArchiveFacts };
    if (payload.data) validateFacts(payload.data, id, retry, blob.type);
    if (!response.ok || payload.code !== 0 || !payload.data) {
        if (response.status === 422 && payload.data) throw new LocalArchiveFailure(payload.data);
        throw new Error("本地归档请求失败");
    }
    const facts = payload.data;
    const extension = blob.type === "video/mp4" ? "mp4" : "webm";
    if (facts.archiveStatus !== "ARCHIVED" || facts.resultStatus !== "ARCHIVED" || facts.sha256 !== sha256 || facts.byteLength !== blob.size || facts.archivedRelativePath !== `generated/${facts.generationId}/${facts.resultId}/media.${extension}`) throw new Error("归档返回字节或路径验证失败");
    return facts;
}

// These explicit methods attach local bytes; they do not assert Provider success.
export function archiveLocalResult(input: LocalArchiveInput, dependencies: Dependencies = {}) {
    return archive(input.projectId, input.generationId, input.storageKey, false, dependencies);
}
export function retryLocalArchive(input: LocalArchiveRetryInput, dependencies: Dependencies = {}) {
    return archive(input.projectId, input.archiveJobId, input.storageKey, true, dependencies);
}
