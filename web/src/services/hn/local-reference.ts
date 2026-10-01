export type LocalReferenceVersion = {
    id: string;
    projectId: string;
    logicalReferenceId: string;
    relativePath: string;
    byteLength: number;
    sha256: string;
    mimeType: string;
    kind: "image";
    schemaVersion: number;
    createdAt: string;
    updatedAt: string;
};

type ReferenceSource = { projectId: string; storageKey: string };
type Dependencies = { readBlob: (key: string) => Promise<Blob | null>; request?: typeof fetch };

export function isHNProjectId(id: string) {
    return /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(id) && !/^(CON|PRN|AUX|NUL|(COM|LPT)[1-9])$/i.test(id);
}

export async function logicalReferenceId(projectId: string, storageKey: string) {
    const hash = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(JSON.stringify([projectId, storageKey])));
    return `ref-${Array.from(new Uint8Array(hash), (value) => value.toString(16).padStart(2, "0")).join("")}`;
}

export function localBackendURL(value: string) {
    const url = new URL(value);
    if (url.protocol !== "http:" || !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) || url.username || url.password || url.search || url.hash || url.pathname !== "/") {
        throw new Error("HN 参考冻结需要本机后端地址");
    }
    return url.origin;
}

// No import, transformation, browser cache, or node update operation here.
export async function freezeLocalReference(source: ReferenceSource, { readBlob, request = fetch }: Dependencies): Promise<LocalReferenceVersion> {
    if (!isHNProjectId(source.projectId)) throw new Error("项目标识不兼容；需审核 ID 映射");
    if (!source.storageKey.startsWith("image:")) throw new Error("只能冻结现有本地图片");
    const blob = await readBlob(source.storageKey);
    if (!blob || blob.size === 0) throw new Error("本地图片已丢失或为空");
    if (blob.size > 16 * 1024 * 1024) throw new Error("参考图片超过 16 MiB 限制");
    const logicalID = await logicalReferenceId(source.projectId, source.storageKey);
    const discovery = await request("/api/hn/local-endpoint", { credentials: "omit", cache: "no-store" });
    const endpoint = await discovery.json() as { code: number; data?: { url: string } };
    if (!discovery.ok || endpoint.code !== 0 || !endpoint.data) throw new Error("HN 参考冻结需要本机后端地址");
    const base = localBackendURL(endpoint.data.url);
    const body = new FormData();
    body.append("file", blob, "reference-image.bin");
    body.append("logicalReferenceId", logicalID);
    body.append("kind", "image");
    // Direct loopback transport preserves real RemoteAddr; omit auth/cookies.
    const response = await request(`${base}/api/hn/projects/${encodeURIComponent(source.projectId)}/references`, {
        method: "POST", credentials: "omit", headers: { "X-HN-Local-Request": "1" }, body,
    });
    const payload = await response.json() as { code: number; data?: LocalReferenceVersion; msg?: string };
    if (!response.ok || payload.code !== 0 || !payload.data) throw new Error(payload.msg || "本地参考冻结失败");
    return payload.data;
}
