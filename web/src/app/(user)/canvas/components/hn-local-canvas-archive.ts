import { archiveLocalResult, retryLocalArchive, LocalArchiveFailure, HN_ARCHIVE_MAX_BYTES, type LocalArchiveFacts } from "@/services/hn/local-result-archive";
import { isHNProjectId, localBackendURL } from "@/services/hn/local-reference";
import { mapHNCanvasProjectId, validHNLocalReceipt, hnLocalSHA256 } from "./hn-local-canvas-prepare";
import { buildHNLocalResultArchiveInput } from "./hn-local-result-archive";
import { CanvasNodeType, type CanvasNodeData, type HNLocalArchiveOwner, type HNLocalArchiveReceipt, type HNLocalKnownJob, type HNLocalPreparedReceipt } from "../types";

export const HN_ARCHIVE_EXPLANATION = "仅把当前本地视频附加到此节点的历史冻结请求并归档；不证明 AI/Provider 生成成功，不调用模型。";
export const HN_ARCHIVE_NEW_CONFIRMATION = "会创建新的 Result 和 ArchiveJob，保留之前的本地归档；这不是重试。";
export const HN_ARCHIVE_ERRORS = {
    HN_ARCHIVE_PREPARED_MISSING: "此节点没有有效的历史本地准备回执，无法附加归档。",
    HN_ARCHIVE_OWNER_MISMATCH: "本地回执与当前画布或节点不一致，已停止。",
    HN_ARCHIVE_LOCAL_VIDEO_MISSING: "本地视频已丢失或为空，请重新选择本地文件。",
    HN_ARCHIVE_STORAGE_UNSUPPORTED: "仅支持浏览器持久化的本地视频；当前来源不支持本地归档。",
    HN_ARCHIVE_MIME_UNSUPPORTED: "仅支持 MP4/WebM 本地视频。",
    HN_ARCHIVE_TOO_LARGE: "本地视频超过 64 MiB，未发送归档请求。",
    HN_ARCHIVE_REJECTED: "本地归档请求被拒绝；没有创建新的结果，请检查历史准备和视频格式。",
    HN_ARCHIVE_FAILED_RETRYABLE: "归档未完成；可显式重试同一归档，不会创建新的结果。",
    HN_ARCHIVE_OUTCOME_UNKNOWN: "归档结果未确认；可能已写入，不会自动重试或创建新结果。",
    HN_ARCHIVE_SOURCE_CHANGED: "当前视频字节已变更，与上次归档尝试不一致。",
    HN_LOCAL_BACKEND_UNAVAILABLE: "本地归档服务不可用，请检查本机服务和项目目录配置。",
    HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE: "本地尝试记录无法可靠保存，已停止发送归档请求。",
    HN_ARCHIVE_UNVERIFIED: "历史本地归档回执，未复核",
} as const;
type ErrorCode = keyof typeof HN_ARCHIVE_ERRORS;
class ArchiveError extends Error { constructor(public readonly code: ErrorCode) { super(HN_ARCHIVE_ERRORS[code]); } }
function fail(code: ErrorCode): never { throw new ArchiveError(code); }
type Phase = HNLocalArchiveReceipt["outcome"] | "SOURCE_CHANGED" | "RELOADED_UNVERIFIED";
export type HNArchiveTarget = { canvasProjectId: string; node: CanvasNodeData };
export type HNArchiveEntry = { phase: Phase; running: boolean; receipt?: HNLocalArchiveReceipt; source?: Source; action?: "fresh" | "retry" | "new"; error?: ErrorCode; sessionAttempt?: string; inspection: number };
type Source = { sha256: string; byteLength: number; mimeType: "video/mp4" | "video/webm"; fingerprint: string };
export type HNArchiveJournal = { getItem: (key: string) => Promise<unknown>; setItem: (key: string, value: HNLocalArchiveReceipt) => Promise<unknown> };
export type HNArchiveDependencies = { readBlob: (key: string) => Promise<Blob | null>; request?: typeof fetch; journal?: HNArchiveJournal };
let journalPromise: Promise<HNArchiveJournal> | undefined;
function localJournal() {
    return journalPromise ??= import("localforage").then(({ default: localforage }) => localforage.createInstance({ name: "infinite-canvas", storeName: "hn_local_archive_attempts" }));
}
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const keys = (v: object, expected: string[]) => Object.keys(v).sort().join(",") === expected.slice().sort().join(",");
const id = (v: unknown): v is string => typeof v === "string" && isHNProjectId(v);
const hash = (v: unknown): v is string => typeof v === "string" && /^[a-f0-9]{64}$/.test(v);
const uuid = (v: unknown) => typeof v === "string" && /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/.test(v);
const time = (v: unknown) => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString() === v;
const mime = (v: unknown): v is Source["mimeType"] => v === "video/mp4" || v === "video/webm";
const length = (v: unknown) => Number.isSafeInteger(v) && (v as number) > 0 && (v as number) <= HN_ARCHIVE_MAX_BYTES;
const failedPair = (result: unknown, archive: unknown) => result === "ARCHIVE_FAILED" && archive === "FAILED" || result === "RECEIVED" && ["PENDING", "COPYING", "FINALIZING"].includes(archive as string);
function knownJob(v: unknown): v is HNLocalKnownJob { return object(v) && keys(v, ["resultId", "archiveJobId"]) && id(v.resultId) && id(v.archiveJobId); }
export function sourceMediaFingerprint(sha256: string, byteLength: number, mimeType: string) {
    return hnLocalSHA256(new TextEncoder().encode(JSON.stringify({ version: 1, sha256, byteLength, mimeType })));
}
export async function validHNLocalArchiveReceipt(value: unknown, project: string, node: string, prepared?: HNLocalPreparedReceipt): Promise<boolean> {
    if (!object(value)) return false;
    if (!/^[A-Za-z0-9_-]{1,128}$/.test(node)) return false;
    let mapped: string;
    try { mapped = await mapHNCanvasProjectId(project); } catch { return false; }
    const r = value;
    const common = ["version", "canvasProjectId", "hnProjectId", "sourceNodeId", "shotId", "generationId", "preparedFrozenHash", "attemptId", "observedAt", "sourceSHA256", "sourceByteLength", "sourceMimeType", "sourceMediaFingerprint", "outcome"];
    const extra = r.outcome === "NOT_ARCHIVED" ? [] : r.outcome === "ARCHIVING" || r.outcome === "ARCHIVE_OUTCOME_UNKNOWN" ? ["knownJob"] : r.outcome === "ARCHIVE_FAILED_RETRYABLE" ? ["resultId", "archiveJobId", "resultStatus", "archiveStatus"] : r.outcome === "ARCHIVED" ? ["resultId", "archiveJobId", "resultStatus", "archiveStatus", "sha256", "byteLength", "mimeType"] : null;
    if (!extra || !keys(r, [...common, ...extra]) || r.version !== 1 || r.canvasProjectId !== project || r.hnProjectId !== mapped || r.sourceNodeId !== node || !id(r.shotId) || !id(r.generationId) || !hash(r.preparedFrozenHash) || !uuid(r.attemptId) || !time(r.observedAt) || !hash(r.sourceSHA256) || !length(r.sourceByteLength) || !mime(r.sourceMimeType) || !hash(r.sourceMediaFingerprint)) return false;
    if (prepared && (!validHNLocalReceipt(prepared, project, node, mapped) || r.shotId !== prepared.shotId || r.generationId !== prepared.generationId || r.preparedFrozenHash !== prepared.frozenHash)) return false;
    if (r.sourceMediaFingerprint !== await sourceMediaFingerprint(r.sourceSHA256, r.sourceByteLength as number, r.sourceMimeType)) return false;
    if (r.outcome === "ARCHIVING" || r.outcome === "ARCHIVE_OUTCOME_UNKNOWN") return r.knownJob === null || knownJob(r.knownJob);
    if (r.outcome === "NOT_ARCHIVED") return true;
    if (!id(r.resultId) || !id(r.archiveJobId)) return false;
    if (r.outcome === "ARCHIVE_FAILED_RETRYABLE") return failedPair(r.resultStatus, r.archiveStatus);
    return r.resultStatus === "ARCHIVED" && r.archiveStatus === "ARCHIVED" && r.sha256 === r.sourceSHA256 && r.byteLength === r.sourceByteLength && r.mimeType === r.sourceMimeType;
}
export function canShowHNLocalArchive(node: CanvasNodeData | null | undefined, history = false) {
    return !!node && node.type === CanvasNodeType.Video && (!!node.metadata?.content || history);
}
async function owner(target: HNArchiveTarget) {
    const { node, canvasProjectId } = target, m = node.metadata;
    if (!/^[A-Za-z0-9_-]{1,128}$/.test(node.id)) fail("HN_ARCHIVE_OWNER_MISMATCH");
    let mapped: string;
    try { mapped = await mapHNCanvasProjectId(canvasProjectId); } catch { fail("HN_ARCHIVE_OWNER_MISMATCH"); }
    const prepared = m?.hnLocalPrepared;
    if (!prepared) fail("HN_ARCHIVE_PREPARED_MISSING");
    if (!validHNLocalReceipt(prepared, canvasProjectId, node.id, mapped)) fail("HN_ARCHIVE_OWNER_MISMATCH");
    if (node.type !== CanvasNodeType.Video || !m?.content || m.status && m.status !== "success" || m.workflowRef || m.videoTaskId || m.videoTaskVideoId || m.channelId || m.imageTaskId || m.audioTaskId) fail("HN_ARCHIVE_OWNER_MISMATCH");
    if (!m.storageKey || !/^(video|file):[^\s:/\\?&#]+$/.test(m.storageKey)) fail("HN_ARCHIVE_STORAGE_UNSUPPORTED");
    return prepared;
}
async function source(target: HNArchiveTarget, d: HNArchiveDependencies) {
    const prepared = await owner(target);
    let blob: Blob | null;
    try { blob = await d.readBlob(target.node.metadata!.storageKey!); } catch { fail("HN_ARCHIVE_LOCAL_VIDEO_MISSING"); }
    if (!blob || !blob.size) fail("HN_ARCHIVE_LOCAL_VIDEO_MISSING");
    if (blob.size > HN_ARCHIVE_MAX_BYTES) fail("HN_ARCHIVE_TOO_LARGE");
    if (!mime(blob.type)) fail("HN_ARCHIVE_MIME_UNSUPPORTED");
    const sha256 = await hnLocalSHA256(await blob.arrayBuffer());
    return { prepared, blob, facts: { sha256, byteLength: blob.size, mimeType: blob.type, fingerprint: await sourceMediaFingerprint(sha256, blob.size, blob.type) } };
}
function receiptOwner(r: HNLocalArchiveReceipt): HNLocalArchiveOwner {
    const { version, canvasProjectId, hnProjectId, sourceNodeId, shotId, generationId, preparedFrozenHash, attemptId, observedAt, sourceSHA256, sourceByteLength, sourceMimeType, sourceMediaFingerprint } = r;
    return { version, canvasProjectId, hnProjectId, sourceNodeId, shotId, generationId, preparedFrozenHash, attemptId, observedAt, sourceSHA256, sourceByteLength, sourceMimeType, sourceMediaFingerprint };
}
function jobOf(r?: HNLocalArchiveReceipt): HNLocalKnownJob | null {
    if (r?.outcome === "ARCHIVE_FAILED_RETRYABLE") return { resultId: r.resultId, archiveJobId: r.archiveJobId };
    if (r?.outcome === "ARCHIVE_OUTCOME_UNKNOWN" || r?.outcome === "ARCHIVING") return r.knownJob;
    return null;
}
export function mergeHNLocalArchiveReceipt(nodes: CanvasNodeData[], receipt: HNLocalArchiveReceipt, activeProject: string) {
    if (activeProject !== receipt.canvasProjectId) return nodes;
    return nodes.map((node) => {
        const p = node.metadata?.hnLocalPrepared;
        return node.type === CanvasNodeType.Video && node.id === receipt.sourceNodeId && p && validHNLocalReceipt(p, activeProject, node.id, receipt.hnProjectId) && p.shotId === receipt.shotId && p.generationId === receipt.generationId && p.frozenHash === receipt.preparedFrozenHash ? { ...node, metadata: { ...node.metadata, hnLocalArchive: receipt } } : node;
    });
}
function validateResponse(f: unknown, r: HNLocalArchiveOwner, known: HNLocalKnownJob | null, success: boolean): f is LocalArchiveFacts {
    if (!object(f) || !keys(f, ["resultId", "generationId", "archiveJobId", "resultStatus", "archiveStatus", "archivedRelativePath", "sha256", "byteLength", "mimeType"]) || !id(f.resultId) || !id(f.archiveJobId) || f.generationId !== r.generationId || f.mimeType !== r.sourceMimeType || known && (f.resultId !== known.resultId || f.archiveJobId !== known.archiveJobId)) return false;
    if (success) return f.resultStatus === "ARCHIVED" && f.archiveStatus === "ARCHIVED" && f.sha256 === r.sourceSHA256 && f.byteLength === r.sourceByteLength && f.archivedRelativePath === `generated/${r.generationId}/${f.resultId}/media.${r.sourceMimeType === "video/mp4" ? "mp4" : "webm"}`;
    return failedPair(f.resultStatus, f.archiveStatus) && f.archivedRelativePath === "" && f.sha256 === "" && f.byteLength === 0;
}

// One active page owns these locks. The awaited journal is a local safety barrier,
// not server idempotency, a global multi-tab lock or an HN/Canvas transaction.
export class HNLocalArchiveController {
    private entries = new Map<string, HNArchiveEntry>();
    private alive = true;
    private epoch = 0;
    constructor(private notify: () => void = () => {}) {}
    activate() { this.alive = true; }
    dispose() { this.alive = false; this.epoch++; }
    key(project: string, node: string) { return JSON.stringify([project, node]); }
    entry(project: string, node: string): HNArchiveEntry {
        const key = this.key(project, node);
        if (!this.entries.has(key)) this.entries.set(key, { phase: "NOT_ARCHIVED", running: false, inspection: 0 });
        return this.entries.get(key)!;
    }
    private classify(e: HNArchiveEntry) {
        e.action = undefined;
        e.phase = e.receipt?.outcome === "ARCHIVE_OUTCOME_UNKNOWN" ? "ARCHIVE_OUTCOME_UNKNOWN" : e.receipt ? e.receipt.attemptId === e.sessionAttempt ? e.receipt.outcome : "RELOADED_UNVERIFIED" : "NOT_ARCHIVED";
        if (!e.source || e.error) return;
        const r = e.receipt, same = r?.sourceMediaFingerprint === e.source.fingerprint;
        if (!r || r.outcome === "NOT_ARCHIVED") { e.phase = "NOT_ARCHIVED"; e.action = "fresh"; }
        else if (r.outcome === "ARCHIVE_OUTCOME_UNKNOWN") { e.phase = "ARCHIVE_OUTCOME_UNKNOWN"; if (same && r.knownJob) e.action = "retry"; }
        else if (!same) { e.phase = "SOURCE_CHANGED"; if (r.outcome === "ARCHIVED") e.action = "new"; }
        else { e.phase = r.attemptId === e.sessionAttempt ? r.outcome : "RELOADED_UNVERIFIED"; if (r.outcome === "ARCHIVE_FAILED_RETRYABLE") e.action = "retry"; }
    }
    private async history(target: HNArchiveTarget, journal: HNArchiveJournal): Promise<HNLocalArchiveReceipt | undefined> {
        let raw: unknown;
        try { raw = await journal.getItem(this.key(target.canvasProjectId, target.node.id)); } catch { fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"); }
        const canvas = target.node.metadata?.hnLocalArchive;
        const memory = this.entry(target.canvasProjectId, target.node.id).receipt;
        for (const value of [raw, canvas, memory]) if (value != null && !await validHNLocalArchiveReceipt(value, target.canvasProjectId, target.node.id)) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
        const saved = raw as HNLocalArchiveReceipt | null, projection = canvas;
        if (saved && projection && saved.attemptId === projection.attemptId && JSON.stringify(saved) !== JSON.stringify(projection)) {
            // A pending barrier may legitimately lag a completed Canvas response;
            // journal wins conservatively. Never trust a projection to clear it.
            if (saved.outcome !== "ARCHIVING" && saved.outcome !== "ARCHIVE_OUTCOME_UNKNOWN") fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
        }
        if (saved && projection && saved.attemptId !== projection.attemptId && saved.observedAt <= projection.observedAt) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
        if (memory?.outcome === "ARCHIVE_OUTCOME_UNKNOWN") {
            if (saved && saved.attemptId !== memory.attemptId) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
            return memory;
        }
        const r = saved || projection || memory;
        return r?.outcome === "ARCHIVING" ? { ...r, outcome: "ARCHIVE_OUTCOME_UNKNOWN" } : r;
    }
    async readHistoricalArchiveReceipt(readCurrent: () => HNArchiveTarget, d: Pick<HNArchiveDependencies, "journal"> = {}) {
        const capture = () => {
            const t = readCurrent();
            return { canvasProjectId: t.canvasProjectId, node: { ...t.node, metadata: { ...t.node.metadata, hnLocalPrepared: structuredClone(t.node.metadata?.hnLocalPrepared), hnLocalArchive: structuredClone(t.node.metadata?.hnLocalArchive) } } };
        };
        const target = capture(), epoch = this.epoch, e = this.entry(target.canvasProjectId, target.node.id);
        if (!this.alive || e.running) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
        const mapped = await mapHNCanvasProjectId(target.canvasProjectId), prepared = target.node.metadata.hnLocalPrepared;
        if (target.node.type !== CanvasNodeType.Video || typeof target.node.id !== "string" || !/^[A-Za-z0-9_-]{1,128}$/.test(target.node.id) || !validHNLocalReceipt(prepared, target.canvasProjectId, target.node.id, mapped)) fail("HN_ARCHIVE_OWNER_MISMATCH");
        const journal = d.journal || await localJournal();
        // Missing durable history cannot be replaced by a stale Canvas/cache receipt.
        const r = await this.history(target, { ...journal, getItem: async (key) => { const value = await journal.getItem(key); if (value == null) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"); return value; } });
        if (!r) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
        if (!await validHNLocalArchiveReceipt(r, target.canvasProjectId, target.node.id, prepared)) fail("HN_ARCHIVE_OWNER_MISMATCH");
        const current = capture();
        if (!this.alive || epoch !== this.epoch || e.running || current.canvasProjectId !== target.canvasProjectId || current.node.id !== target.node.id || current.node.type !== CanvasNodeType.Video || !validHNLocalReceipt(current.node.metadata.hnLocalPrepared, target.canvasProjectId, target.node.id, mapped) || JSON.stringify(current.node.metadata.hnLocalPrepared) !== JSON.stringify(prepared) || JSON.stringify(current.node.metadata.hnLocalArchive) !== JSON.stringify(target.node.metadata.hnLocalArchive)) fail("HN_ARCHIVE_OWNER_MISMATCH");
        return structuredClone(r);
    }
    async inspect(target: HNArchiveTarget, d: HNArchiveDependencies) {
        const e = this.entry(target.canvasProjectId, target.node.id); if (e.running) return;
        const revision = ++e.inspection, epoch = this.epoch;
        try {
            const journal = d.journal || await localJournal(), r = await this.history(target, journal);
            if (this.alive && epoch === this.epoch && revision === e.inspection && !e.running) e.receipt = r;
            const current = await source(target, d);
            if (r && !await validHNLocalArchiveReceipt(r, target.canvasProjectId, target.node.id, current.prepared)) fail("HN_ARCHIVE_OWNER_MISMATCH");
            if (this.alive && epoch === this.epoch && revision === e.inspection && !e.running) { e.receipt = r; e.source = current.facts; e.error = undefined; this.classify(e); this.notify(); }
        } catch (err) {
            if (this.alive && epoch === this.epoch && revision === e.inspection && !e.running) { e.error = err instanceof ArchiveError ? err.code : "HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"; this.classify(e); this.notify(); }
        }
    }
    async archive(readCurrent: () => HNArchiveTarget, d: HNArchiveDependencies, confirm: (action: "fresh" | "retry" | "new", owner: HNLocalArchiveOwner) => Promise<boolean>, onReceipt: (r: HNLocalArchiveReceipt) => void) {
        let target: HNArchiveTarget;
        try { const t = readCurrent(); target = { canvasProjectId: t.canvasProjectId, node: { ...t.node, metadata: { ...t.node.metadata } } }; } catch { return false; }
        const e = this.entry(target.canvasProjectId, target.node.id);
        if (!this.alive || e.running) return false;
        e.running = true; e.phase = "ARCHIVING"; e.action = undefined; e.inspection++; this.notify();
        const epoch = this.epoch, key = this.key(target.canvasProjectId, target.node.id);
        let sent = false, rejected = false, pending: HNLocalArchiveReceipt | undefined, journal: HNArchiveJournal | undefined, prior: HNLocalArchiveReceipt | undefined;
        const active = () => this.alive && epoch === this.epoch;
        try {
            journal = d.journal || await localJournal(); prior = await this.history(target, journal);
            const captured = await source(target, d);
            if (prior && !await validHNLocalArchiveReceipt(prior, target.canvasProjectId, target.node.id, captured.prepared)) fail("HN_ARCHIVE_OWNER_MISMATCH");
            e.receipt = prior; e.source = captured.facts; e.error = undefined; this.classify(e);
            const action = e.action; e.action = undefined; e.phase = "ARCHIVING";
            if (!action) return false;
            const p = captured.prepared, s = captured.facts, known = action === "retry" ? jobOf(prior) : null;
            const common: HNLocalArchiveOwner = { version: 1, canvasProjectId: target.canvasProjectId, hnProjectId: p.hnProjectId, sourceNodeId: target.node.id, shotId: p.shotId, generationId: p.generationId, preparedFrozenHash: p.frozenHash, attemptId: crypto.randomUUID(), observedAt: new Date().toISOString(), sourceSHA256: s.sha256, sourceByteLength: s.byteLength, sourceMimeType: s.mimeType, sourceMediaFingerprint: s.fingerprint };
            if (!await confirm(action, common) || !active()) return false;
            const checkTarget = async () => {
                const now = readCurrent(), own = await owner(now);
                if (!active() || now.canvasProjectId !== target.canvasProjectId || now.node.id !== target.node.id || own.shotId !== p.shotId || own.generationId !== p.generationId || own.frozenHash !== p.frozenHash || now.node.metadata?.storageKey !== target.node.metadata?.storageKey) fail("HN_ARCHIVE_OWNER_MISMATCH");
            };
            await checkTarget();
            pending = { ...common, outcome: "ARCHIVING", knownJob: known };
            try {
                await journal.setItem(key, pending);
                const back = await journal.getItem(key);
                if (!await validHNLocalArchiveReceipt(back, target.canvasProjectId, target.node.id, p) || JSON.stringify(back) !== JSON.stringify(pending)) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
            } catch { fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"); }
            e.receipt = pending;
            let base = "", discoveryCount = 0;
            const request: typeof fetch = async (url, options) => {
                if (url === "/api/hn/local-endpoint" && (options?.method || "GET") === "GET" && ++discoveryCount === 1 && !sent) {
                    try {
                        const response = await (d.request || fetch)(url, { ...options, credentials: "omit", redirect: "error" });
                        const payload = await response.clone().json();
                        if (!response.ok || payload.code !== 0 || !object(payload.data) || !keys(payload.data, ["url"]) || typeof payload.data.url !== "string") fail("HN_LOCAL_BACKEND_UNAVAILABLE");
                        base = localBackendURL(payload.data.url); return response;
                    } catch { fail("HN_LOCAL_BACKEND_UNAVAILABLE"); }
                }
                const suffix = known ? `archive-jobs/${known.archiveJobId}/retry` : `generations/${p.generationId}/results/local-archive`;
                if (typeof url !== "string" || !base || url !== `${base}/api/hn/projects/${p.hnProjectId}/${suffix}` || options?.method !== "POST" || sent) fail("HN_ARCHIVE_REJECTED");
                await checkTarget();
                // Validate pending again just before wire dispatch; no other writer
                // may remove/replace the barrier during discovery.
                try { const back = await journal!.getItem(key); if (!await validHNLocalArchiveReceipt(back, target.canvasProjectId, target.node.id, p) || JSON.stringify(back) !== JSON.stringify(pending)) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"); } catch { fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"); }
                sent = true;
                const response = await (d.request || fetch)(url, { ...options, credentials: "omit", redirect: "error", headers: { "X-HN-Local-Request": "1" } });
                const payload: unknown = await response.clone().json();
                if (!object(payload) || !keys(payload, ["code", "data", "msg"]) && !keys(payload, ["code", "msg"]) && !keys(payload, ["code", "data"]) && !keys(payload, ["code"])) fail("HN_ARCHIVE_OUTCOME_UNKNOWN");
                if ([400, 403, 503].includes(response.status) && Number.isInteger(payload.code) && payload.code !== 0 && (payload.data === undefined || payload.data === null)) { rejected = true; fail(response.status === 503 ? "HN_LOCAL_BACKEND_UNAVAILABLE" : "HN_ARCHIVE_REJECTED"); }
                if (response.status === 422 && payload.code === 1 && validateResponse(payload.data, common, known, false)) return response;
                if (response.ok && payload.code === 0 && validateResponse(payload.data, common, known, true)) return response;
                fail("HN_ARCHIVE_OUTCOME_UNKNOWN");
            };
            let final: HNLocalArchiveReceipt;
            try {
                const dependencies = { request, readBlob: async () => captured.blob };
                const facts = known ? await retryLocalArchive({ projectId: p.hnProjectId, archiveJobId: known.archiveJobId, storageKey: target.node.metadata!.storageKey! }, dependencies) : await archiveLocalResult(buildHNLocalResultArchiveInput(p.hnProjectId, p.generationId, target.node), dependencies);
                final = { ...common, observedAt: new Date().toISOString(), outcome: "ARCHIVED", resultId: facts.resultId, archiveJobId: facts.archiveJobId, resultStatus: "ARCHIVED", archiveStatus: "ARCHIVED", sha256: facts.sha256, byteLength: facts.byteLength, mimeType: s.mimeType };
            } catch (err) {
                if (!(err instanceof LocalArchiveFailure) || !validateResponse(err.archive, common, known, false)) throw err;
                const f = err.archive;
                final = { ...common, observedAt: new Date().toISOString(), outcome: "ARCHIVE_FAILED_RETRYABLE", resultId: f.resultId, archiveJobId: f.archiveJobId, resultStatus: f.resultStatus as "ARCHIVE_FAILED" | "RECEIVED", archiveStatus: f.archiveStatus as "FAILED" | "PENDING" | "COPYING" | "FINALIZING" };
            }
            try { await journal.setItem(key, final); const back = await journal.getItem(key); if (!await validHNLocalArchiveReceipt(back, target.canvasProjectId, target.node.id, p) || JSON.stringify(back) !== JSON.stringify(final)) fail("HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE"); } catch { fail("HN_ARCHIVE_OUTCOME_UNKNOWN"); }
            e.receipt = final; e.sessionAttempt = final.attemptId; e.error = undefined;
            if (active()) onReceipt(final);
            return final.outcome === "ARCHIVED";
        } catch (err) {
            if (sent && !rejected && pending) {
                const unknown: HNLocalArchiveReceipt = { ...receiptOwner(pending), outcome: "ARCHIVE_OUTCOME_UNKNOWN", knownJob: jobOf(pending) };
                e.receipt = unknown; e.error = undefined;
                try { await journal!.setItem(key, unknown); } catch { /* Pending on disk still blocks fresh after reload. */ }
                if (active()) onReceipt(unknown);
            } else {
                e.receipt = prior; e.error = err instanceof ArchiveError ? err.code : "HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE";
                // Do not clear a marker whose write/read-back was uncertain.
                if (pending && e.error !== "HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE") {
                    const restored: HNLocalArchiveReceipt = prior || { ...receiptOwner(pending), outcome: "NOT_ARCHIVED" };
                    try { await journal!.setItem(key, restored); e.receipt = restored; } catch { e.receipt = { ...receiptOwner(pending), outcome: "ARCHIVE_OUTCOME_UNKNOWN", knownJob: jobOf(pending) }; e.error = undefined; }
                }
            }
            return false;
        } finally {
            e.running = false; this.classify(e);
            if (e.receipt?.outcome === "ARCHIVE_OUTCOME_UNKNOWN") e.phase = "ARCHIVE_OUTCOME_UNKNOWN";
            if (active()) this.notify();
        }
    }
}
