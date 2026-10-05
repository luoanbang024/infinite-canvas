import { ensureCandidate } from "@/services/hn/local-editorial";
import { isHNProjectId, localBackendURL } from "@/services/hn/local-reference";
import { validHNLocalReceipt } from "./hn-local-canvas-prepare";
import { HNLocalArchiveController, validHNLocalArchiveReceipt, type HNArchiveTarget, type HNArchiveJournal } from "./hn-local-canvas-archive";
import { CanvasNodeType, type CanvasNodeData, type HNLocalArchiveReceipt, type HNLocalCandidateReceipt } from "../types";

export const HN_CANDIDATE_EXPLANATION = "此操作只把已归档的本地版本登记为剪辑候选；不会自动选中，也不会加入序列，不代表 AI/Provider 生成成功。";
export const HN_CANDIDATE_HISTORICAL = "操作的是回执中的历史已归档版本，不会上传或归档当前节点视频。";
export const HN_CANDIDATE_REVALIDATION = "历史归档/候选回执未复核；点击后由本地后端校验该已归档版本，并登记或返回同一候选。";
export const HN_CANDIDATE_ERRORS = {
    HN_CANDIDATE_ARCHIVE_NOT_ELIGIBLE: "没有可用的已归档本地版本回执，未发送候选请求。",
    HN_CANDIDATE_ARCHIVE_UNKNOWN: "归档结果未确认，不能登记候选；不会重新归档或创建新结果。",
    HN_CANDIDATE_OWNER_MISMATCH: "本地回执与当前画布或节点不一致，已停止。",
    HN_CANDIDATE_HISTORY_UNAVAILABLE: "历史归档记录无法可靠读取，未发送候选请求。",
    HN_CANDIDATE_OWNERSHIP_CONFLICT: "已归档版本的归属或完整性校验未通过，已停止。",
    HN_CANDIDATE_IDENTITY_CONFLICT: "同一已归档版本存在冲突候选，已停止；不会覆盖或重新命名。",
    HN_CANDIDATE_BACKEND_UNAVAILABLE: "本地候选服务不可用，未发送候选请求。",
    HN_CANDIDATE_OUTCOME_UNKNOWN: "候选结果未确认，可能已登记；可再次明确点击“重新确认候选”，不会自动重试。",
    HN_CANDIDATE_UNVERIFIED: "历史候选回执未复核；需显式重新确认。",
} as const;
type ErrorCode = keyof typeof HN_CANDIDATE_ERRORS;
class CandidateError extends Error { constructor(public readonly code: ErrorCode, public readonly knownRejection = false) { super(HN_CANDIDATE_ERRORS[code]); } }
function fail(code: ErrorCode): never { throw new CandidateError(code); }
type Archived = Extract<HNLocalArchiveReceipt, { outcome: "ARCHIVED" }>;
export type HNCandidatePhase = "ARCHIVE_NOT_ELIGIBLE" | "NOT_CANDIDATE" | "ENSURING_CANDIDATE" | "CANDIDATE_READY" | "CANDIDATE_OUTCOME_UNKNOWN" | "CANDIDATE_RELOADED_UNVERIFIED" | "ERROR";
export type HNCandidateEntry = { phase: HNCandidatePhase; running: boolean; inspection: number; target?: Archived; receipt?: HNLocalCandidateReceipt; sessionCandidateId?: string; error?: ErrorCode };
export type HNCandidateDependencies = { journal?: HNArchiveJournal; request?: typeof fetch };
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const keys = (v: object, names: string[]) => Object.keys(v).sort().join(",") === names.slice().sort().join(",");
const id = (v: unknown): v is string => typeof v === "string" && isHNProjectId(v);
const hash = (v: unknown) => typeof v === "string" && /^[a-f0-9]{64}$/.test(v);
const time = (v: unknown) => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString() === v;
const backendTime = (v: unknown) => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString().slice(0, 19) === v.slice(0, 19);
function receiptShape(v: unknown, target: HNArchiveTarget): v is HNLocalCandidateReceipt {
    const p = target.node.metadata?.hnLocalPrepared;
    if (!object(v) || !keys(v, ["version", "canvasProjectId", "hnProjectId", "sourceNodeId", "shotId", "generationId", "preparedFrozenHash", "resultId", "archiveJobId", "sourceMediaFingerprint", "candidateId", "availabilityStatus", "observedAt"]) || !p) return false;
    return target.node.type === CanvasNodeType.Video && typeof target.node.id === "string" && /^[A-Za-z0-9_-]{1,128}$/.test(target.node.id) && validHNLocalReceipt(p, target.canvasProjectId, target.node.id, p.hnProjectId) && v.version === 1 && v.canvasProjectId === target.canvasProjectId && v.sourceNodeId === target.node.id && v.hnProjectId === p.hnProjectId && v.shotId === p.shotId && v.generationId === p.generationId && v.preparedFrozenHash === p.frozenHash && [v.shotId, v.generationId, v.resultId, v.archiveJobId, v.candidateId].every(id) && hash(v.preparedFrozenHash) && hash(v.sourceMediaFingerprint) && v.availabilityStatus === "ARCHIVED" && time(v.observedAt);
}
export async function validHNLocalCandidateReceipt(v: unknown, target: HNArchiveTarget, archive: HNLocalArchiveReceipt): Promise<boolean> {
    return receiptShape(v, target) && archive.outcome === "ARCHIVED" && await validHNLocalArchiveReceipt(archive, target.canvasProjectId, target.node.id, target.node.metadata?.hnLocalPrepared) && v.hnProjectId === archive.hnProjectId && v.resultId === archive.resultId && v.archiveJobId === archive.archiveJobId && v.sourceMediaFingerprint === archive.sourceMediaFingerprint;
}
export function mergeHNLocalCandidateReceipt(nodes: CanvasNodeData[], receipt: HNLocalCandidateReceipt, activeProject: string, archive: Archived) {
    if (activeProject !== receipt.canvasProjectId || receipt.hnProjectId !== archive.hnProjectId || receipt.resultId !== archive.resultId || receipt.archiveJobId !== archive.archiveJobId || receipt.sourceMediaFingerprint !== archive.sourceMediaFingerprint) return nodes;
    return nodes.map((node) => {
        if (node.id !== receipt.sourceNodeId || !receiptShape(receipt, { canvasProjectId: activeProject, node })) return node;
        const current = node.metadata?.hnLocalArchive;
        if (current && JSON.stringify(current) !== JSON.stringify(archive)) return node;
        return { ...node, metadata: { ...node.metadata, hnLocalCandidate: receipt } };
    });
}

// Exact rejection messages belong to the current local handler; never display msg.
const rejections: Record<number, Record<string, ErrorCode>> = {
    400: Object.fromEntries(["HN 剪辑只接受 JSON", "HN 剪辑输入无效", "HN 剪辑输入含不支持字段", "HN 剪辑标识或输入无效"].map((s) => [s, "HN_CANDIDATE_OWNER_MISMATCH"])),
    403: { "HN 剪辑需要显式本机请求标记": "HN_CANDIDATE_OWNER_MISMATCH" },
    503: { "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT": "HN_CANDIDATE_BACKEND_UNAVAILABLE" },
    409: { EDITORIAL_OWNERSHIP_CONFLICT: "HN_CANDIDATE_OWNERSHIP_CONFLICT", CANDIDATE_IDENTITY_CONFLICT: "HN_CANDIDATE_IDENTITY_CONFLICT" },
};
function candidateRequest(r: Archived, request: typeof fetch, dispatched: () => void, active: () => boolean, beforePost: () => Promise<void>) {
    let discovered = false, posted = false, base = "";
    return (async (url, options = {}) => {
        if (!active()) fail("HN_CANDIDATE_OWNER_MISMATCH");
        const method = options.method || "GET";
        const discovery = url === "/api/hn/local-endpoint" && method === "GET" && !discovered && !posted;
        const expected = `${base}/api/hn/projects/${encodeURIComponent(r.hnProjectId)}/shots/${r.shotId}/candidates/ensure`;
        if (!discovery && (String(url) !== expected || !discovered || posted || method !== "POST" || options.body !== JSON.stringify({ resultId: r.resultId, label: "Local Archive" }) || new Headers(options.headers).get("Content-Type") !== "application/json" || new Headers(options.headers).get("X-HN-Local-Request") !== "1")) fail("HN_CANDIDATE_OWNER_MISMATCH");
        if (discovery) discovered = true; else { await beforePost(); if (!active()) fail("HN_CANDIDATE_OWNER_MISMATCH"); posted = true; dispatched(); }
        let response: Response;
        try { response = await request(url, { ...options, credentials: "omit", redirect: "error", ...(discovery ? { cache: "no-store" } : {}), signal: AbortSignal.timeout(30_000) }); } catch { fail(discovery ? "HN_CANDIDATE_BACKEND_UNAVAILABLE" : "HN_CANDIDATE_OUTCOME_UNKNOWN"); }
        let body: unknown;
        try { body = await response.clone().json(); } catch { fail(discovery ? "HN_CANDIDATE_BACKEND_UNAVAILABLE" : "HN_CANDIDATE_OUTCOME_UNKNOWN"); }
        if (!object(body) || !keys(body, ["code", "data", "msg"])) fail(discovery ? "HN_CANDIDATE_BACKEND_UNAVAILABLE" : "HN_CANDIDATE_OUTCOME_UNKNOWN");
        if (!discovery && body.code === 1 && body.data === null && typeof body.msg === "string") { const code = rejections[response.status]?.[body.msg]; if (code) throw new CandidateError(code, true); }
        if (response.status !== 200 || body.code !== 0 || body.msg !== "ok" || !object(body.data)) fail(discovery ? "HN_CANDIDATE_BACKEND_UNAVAILABLE" : "HN_CANDIDATE_OUTCOME_UNKNOWN");
        const c = body.data;
        if (discovery) {
            if (!keys(c, ["url"]) || typeof c.url !== "string") fail("HN_CANDIDATE_BACKEND_UNAVAILABLE");
            try { base = localBackendURL(c.url); } catch { fail("HN_CANDIDATE_BACKEND_UNAVAILABLE"); }
        } else if (!keys(c, ["candidateId", "projectId", "shotId", "generationId", "resultId", "label", "availabilityStatus", "createdAt", "updatedAt"]) || !id(c.candidateId) || c.projectId !== r.hnProjectId || c.shotId !== r.shotId || c.generationId !== r.generationId || c.resultId !== r.resultId || c.availabilityStatus !== "ARCHIVED" || !backendTime(c.createdAt) || !backendTime(c.updatedAt)) fail("HN_CANDIDATE_OUTCOME_UNKNOWN");
        // Existing ensureCandidate additionally enforces the original safe-label policy.
        return response;
    }) as typeof fetch;
}

export class HNLocalCandidateController {
    private entries = new Map<string, HNCandidateEntry>();
    private alive = true;
    private epoch = 0;
    constructor(private notify: () => void = () => {}) {}
    activate() { this.alive = true; }
    dispose() { this.alive = false; this.epoch++; }
    entry(project: string, node: string): HNCandidateEntry {
        const key = JSON.stringify([project, node]);
        if (!this.entries.has(key)) this.entries.set(key, { phase: "ARCHIVE_NOT_ELIGIBLE", running: false, inspection: 0 });
        return this.entries.get(key)!;
    }
    private async historical(readCurrent: () => HNArchiveTarget, archive: HNLocalArchiveController, d: HNCandidateDependencies): Promise<Archived> {
        let r: HNLocalArchiveReceipt;
        try { r = await archive.readHistoricalArchiveReceipt(readCurrent, d); } catch { fail("HN_CANDIDATE_HISTORY_UNAVAILABLE"); }
        if (r.outcome === "ARCHIVING" || r.outcome === "ARCHIVE_OUTCOME_UNKNOWN") fail("HN_CANDIDATE_ARCHIVE_UNKNOWN");
        if (r.outcome !== "ARCHIVED") fail("HN_CANDIDATE_ARCHIVE_NOT_ELIGIBLE");
        return r;
    }
    private async prior(target: HNArchiveTarget, r: Archived, e: HNCandidateEntry) {
        const v = target.node.metadata?.hnLocalCandidate;
        if (v != null && !receiptShape(v, target)) fail("HN_CANDIDATE_OWNER_MISMATCH");
        const persisted = v && await validHNLocalCandidateReceipt(v, target, r) ? structuredClone(v) : undefined;
        if (v && v.resultId === r.resultId && !persisted) fail("HN_CANDIDATE_OWNER_MISMATCH");
        const memory = e.receipt && await validHNLocalCandidateReceipt(e.receipt, target, r) ? e.receipt : undefined;
        if (persisted && memory && persisted.candidateId !== memory.candidateId) fail("HN_CANDIDATE_IDENTITY_CONFLICT");
        return memory || persisted;
    }
    async inspect(readCurrent: () => HNArchiveTarget, archive: HNLocalArchiveController, d: HNCandidateDependencies = {}) {
        let target: HNArchiveTarget;
        try { target = readCurrent(); } catch { return; }
        const e = this.entry(target.canvasProjectId, target.node.id), revision = ++e.inspection, epoch = this.epoch;
        if (!this.alive || e.running) return;
        const active = () => this.alive && epoch === this.epoch && revision === e.inspection && !e.running;
        try {
            const r = await this.historical(readCurrent, archive, d), receipt = await this.prior(readCurrent(), r, e);
            if (!active()) return;
            if (e.phase === "CANDIDATE_OUTCOME_UNKNOWN") { if (JSON.stringify(e.target) !== JSON.stringify(r)) fail("HN_CANDIDATE_HISTORY_UNAVAILABLE"); return; }
            e.target = r; e.receipt = receipt; e.error = undefined;
            e.phase = receipt ? receipt.candidateId === e.sessionCandidateId ? "CANDIDATE_READY" : "CANDIDATE_RELOADED_UNVERIFIED" : "NOT_CANDIDATE";
        } catch (err) { if (active()) { if (e.phase !== "CANDIDATE_OUTCOME_UNKNOWN") { e.target = undefined; e.receipt = undefined; e.phase = "ARCHIVE_NOT_ELIGIBLE"; } e.error = err instanceof CandidateError ? err.code : "HN_CANDIDATE_HISTORY_UNAVAILABLE"; } }
        finally { if (active()) this.notify(); }
    }
    async ensure(readCurrent: () => HNArchiveTarget, archive: HNLocalArchiveController, d: HNCandidateDependencies, confirm: (r: Archived) => Promise<boolean>, onReceipt: (receipt: HNLocalCandidateReceipt, r: Archived) => void, revalidate = false) {
        let initial: HNArchiveTarget;
        try { const t = readCurrent(); initial = { ...t, node: { ...t.node, metadata: structuredClone(t.node.metadata) } }; } catch { return false; }
        const e = this.entry(initial.canvasProjectId, initial.node.id);
        if (!this.alive || e.running || archive.entry(initial.canvasProjectId, initial.node.id).running || e.phase === "CANDIDATE_READY" && !revalidate) return false;
        const previous = e.phase, previousTarget = e.target && structuredClone(e.target), epoch = this.epoch;
        e.running = true; e.phase = "ENSURING_CANDIDATE"; e.inspection++; e.error = undefined; this.notify();
        let dispatched = false;
        const active = () => { try { const t = readCurrent(); return this.alive && epoch === this.epoch && t.canvasProjectId === initial.canvasProjectId && t.node.id === initial.node.id && t.node.type === CanvasNodeType.Video && JSON.stringify(t.node.metadata?.hnLocalPrepared) === JSON.stringify(initial.node.metadata?.hnLocalPrepared) && !archive.entry(initial.canvasProjectId, initial.node.id).running; } catch { return false; } };
        const current = () => { if (!active()) fail("HN_CANDIDATE_OWNER_MISMATCH"); return readCurrent(); };
        try {
            const r = await this.historical(current, archive, d), prior = await this.prior(current(), r, e);
            if ((previous === "CANDIDATE_OUTCOME_UNKNOWN" || previous === "CANDIDATE_RELOADED_UNVERIFIED") && previousTarget && JSON.stringify(previousTarget) !== JSON.stringify(r)) fail("HN_CANDIDATE_OWNER_MISMATCH");
            e.target = r;
            if (JSON.stringify(await this.historical(current, archive, d)) !== JSON.stringify(r)) fail("HN_CANDIDATE_OWNER_MISMATCH");
            if (!await confirm(structuredClone(r))) { e.phase = previous; return false; }
            if (JSON.stringify(await this.historical(current, archive, d)) !== JSON.stringify(r)) fail("HN_CANDIDATE_OWNER_MISMATCH");
            const request = candidateRequest(r, d.request || fetch, () => { dispatched = true; }, active, async () => { if (JSON.stringify(await this.historical(current, archive, d)) !== JSON.stringify(r)) fail("HN_CANDIDATE_OWNER_MISMATCH"); });
            const c = await ensureCandidate({ projectId: r.hnProjectId, shotId: r.shotId, resultId: r.resultId, label: "Local Archive" }, { request });
            if (prior && c.candidateId !== prior.candidateId) fail("HN_CANDIDATE_IDENTITY_CONFLICT");
            if (JSON.stringify(await this.historical(current, archive, d)) !== JSON.stringify(r)) fail("HN_CANDIDATE_OUTCOME_UNKNOWN");
            const finalPrior = await this.prior(current(), r, e);
            if (finalPrior && c.candidateId !== finalPrior.candidateId) fail("HN_CANDIDATE_IDENTITY_CONFLICT");
            const receipt: HNLocalCandidateReceipt = { version: 1, canvasProjectId: r.canvasProjectId, hnProjectId: r.hnProjectId, sourceNodeId: r.sourceNodeId, shotId: r.shotId, generationId: r.generationId, preparedFrozenHash: r.preparedFrozenHash, resultId: r.resultId, archiveJobId: r.archiveJobId, sourceMediaFingerprint: r.sourceMediaFingerprint, candidateId: c.candidateId, availabilityStatus: "ARCHIVED", observedAt: new Date().toISOString() };
            if (!await validHNLocalCandidateReceipt(receipt, current(), r) || !active()) fail("HN_CANDIDATE_OUTCOME_UNKNOWN");
            onReceipt(receipt, r); e.receipt = receipt; e.sessionCandidateId = receipt.candidateId; e.phase = "CANDIDATE_READY";
            return true;
        } catch (err) {
            const code = err instanceof CandidateError ? err.code : dispatched ? "HN_CANDIDATE_OUTCOME_UNKNOWN" : "HN_CANDIDATE_HISTORY_UNAVAILABLE";
            e.error = code;
            e.phase = dispatched && !(err instanceof CandidateError && (err.knownRejection || code === "HN_CANDIDATE_IDENTITY_CONFLICT")) ? "CANDIDATE_OUTCOME_UNKNOWN" : "ERROR";
            if (dispatched && e.phase === "CANDIDATE_OUTCOME_UNKNOWN") e.error = "HN_CANDIDATE_OUTCOME_UNKNOWN";
            return false;
        } finally { e.running = false; if (this.alive && epoch === this.epoch) this.notify(); }
    }
}
