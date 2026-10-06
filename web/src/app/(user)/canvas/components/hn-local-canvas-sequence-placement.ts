import { addSequenceItem, type HNSequenceItem } from "@/services/hn/local-editorial";
import { localBackendURL } from "@/services/hn/local-reference";
import { validHNLocalReceipt } from "./hn-local-canvas-prepare";
import { HNLocalArchiveController, type HNArchiveTarget } from "./hn-local-canvas-archive";
import { HNLocalCandidateController, validHNLocalCandidateReceipt, type HNCandidateDependencies } from "./hn-local-canvas-candidate";
import { HNLocalSelectionController, validHNLocalSelectionReceipt, type HNSelectionOwner } from "./hn-local-canvas-selection";
import { browserPlacementJournal, closedObject, emptyPlacementLedger, placementJournalKey, placementOwnerKeys, samePlacementOwner, validPlacementLedger, validHNLocalSequencePlacementReceipt, uuid4, type PlacementAttempt, type PlacementJournal, type PlacementLedger, type PlacementRejection } from "./hn-local-sequence-placement-journal";
import { CanvasNodeType, type CanvasNodeData, type HNLocalCandidateReceipt, type HNLocalSequencePlacementReceipt } from "../types";

export const HN_PLACEMENT_EXPLANATION = "此操作会在主序列末尾新增一个剪辑项；不会生成视频，也不会导出，不代表 AI/Provider 生成成功。";
export const HN_PLACEMENT_NO_READ = "本页没有可靠的主序列查询；回执只记录一次已确认的加入操作。";
export const HN_PLACEMENT_CONFIRMATION = "每次成功调用都会新增一个剪辑项；结果未确认时不能重新发送。";
export const HN_PLACEMENT_SELECTION_RELOAD = "历史选择回执不能确认当前选择；本地后端将在新增前校验。若已改选其他候选，本次将被拒绝。";
export const HN_PLACEMENT_UNKNOWN = "加入结果未确认；可能已经创建剪辑项，不会自动重试。请停止，等待后续恢复方案。";
export const HN_PLACEMENT_RELOAD = "历史加入回执未复核，不能确认主序列当前内容或顺序。";
export const HN_PLACEMENT_SUCCESS = "已收到加入主序列回执；不表示当前顺序仍未改变。";
export const HN_PLACEMENT_DUPLICATE = "此已知版本已有成功加入回执；本阶段不提供重复加入。";
const errorText = "本地历史或序列记录无法可靠确认，已停止；未确认的请求不能重新发送。";
const rejectionText = "本地后端拒绝了新增；候选当前选择或归档完整性不符合要求。";
class PlacementError extends Error { constructor(public rejection?: PlacementRejection) { super(rejection ? rejectionText : errorText); } }
function fail(): never { throw new PlacementError(); }
export type PlacementPhase = "SELECTION_NOT_ELIGIBLE" | "NOT_CONFIRMED_PLACED" | "PLACING" | "PLACED_CURRENT_SESSION" | "PLACEMENT_OUTCOME_UNKNOWN" | "PLACEMENT_RELOADED_UNVERIFIED" | "ERROR";
export type PlacementEntry = { phase: PlacementPhase; running: boolean; receipt?: HNLocalSequencePlacementReceipt; error?: string; sessionIntent?: string; inspection: number };
export type PlacementDependencies = HNCandidateDependencies & { placementJournal?: PlacementJournal };
const canonical = (v: unknown): string => JSON.stringify(v, (_key, value) => value && typeof value === "object" && !Array.isArray(value) ? Object.fromEntries(Object.keys(value).sort().map((k) => [k, value[k]])) : value);
function historical(t: HNArchiveTarget): HNArchiveTarget {
    const n = t.node, p = n.metadata?.hnLocalPrepared;
    if (n.type !== CanvasNodeType.Video || !p || !validHNLocalReceipt(p, t.canvasProjectId, n.id, p.hnProjectId)) fail();
    return { canvasProjectId: t.canvasProjectId, node: { id: n.id, type: n.type, title: "", width: 0, height: 0, position: { x: 0, y: 0 }, metadata: { hnLocalPrepared: structuredClone(p), hnLocalArchive: structuredClone(n.metadata?.hnLocalArchive), hnLocalCandidate: structuredClone(n.metadata?.hnLocalCandidate), hnLocalSelection: structuredClone(n.metadata?.hnLocalSelection) } } };
}
const revision = (t: HNArchiveTarget) => canonical(historical(t));
function validProjection(v: unknown, ledger: PlacementLedger, target: HNArchiveTarget) {
    if (v == null) return true;
    const owner = Object.fromEntries(placementOwnerKeys.map((k) => [k, (v as HNSelectionOwner)[k]])) as HNSelectionOwner;
    if (!validHNLocalSequencePlacementReceipt(v, owner) || v.canvasProjectId !== target.canvasProjectId || v.sourceNodeId !== target.node.id) return false;
    return ledger.entries.some((a) => a.state === "PLACED" && canonical(a.receipt) === canonical(v));
}
function backendTime(v: unknown) {
    return typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString().slice(0, 19) === v.slice(0, 19);
}
export function validPlacementItem(v: unknown, owner: HNSelectionOwner): v is HNSequenceItem {
    return closedObject(v, ["sequenceItemId", "projectId", "sequenceId", "orderIndex", "shotId", "candidateId", "resultId", "createdAt", "updatedAt"]) && uuid4(v.sequenceItemId) && v.projectId === owner.hnProjectId && v.sequenceId === "main" && v.shotId === owner.shotId && v.candidateId === owner.candidateId && v.resultId === owner.resultId && Number.isSafeInteger(v.orderIndex) && (v.orderIndex as number) >= 0 && backendTime(v.createdAt) && backendTime(v.updatedAt);
}
const rejected: Record<number, Record<string, PlacementRejection>> = {
    400: Object.fromEntries(["HN 剪辑只接受 JSON", "HN 剪辑输入无效", "HN 剪辑输入含不支持字段", "HN 剪辑标识或输入无效"].map((s) => [s, "INPUT_REJECTED"])),
    403: Object.fromEntries(["HN 剪辑需要显式本机请求标记", "HN 参考冻结仅允许本机请求", "HN 参考冻结不接受此页面来源"].map((s) => [s, "LOCAL_REQUEST_REJECTED"])),
    409: { EDITORIAL_OWNERSHIP_CONFLICT: "OWNERSHIP_REJECTED" },
    503: { "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT": "BACKEND_DISABLED" },
};
export function hnPlacementRequest(owner: HNSelectionOwner, request: typeof fetch, dispatched: () => void, beforePost: () => Promise<void>, afterDiscovery: () => void) {
    let discovered = false, posted = false, base = "";
    return (async (url, options = {}) => {
        const method = options.method || "GET", discovery = url === "/api/hn/local-endpoint" && method === "GET" && !discovered && !posted;
        const headers = new Headers(options.headers);
        if (options.credentials && options.credentials !== "omit" || discovery && (options.body != null || [...headers].length !== 0) || !discovery && [...headers.keys()].sort().join(",") !== "content-type,x-hn-local-request") fail();
        const expected = `${base}/api/hn/projects/${encodeURIComponent(owner.hnProjectId)}/sequences/main/items`;
        if (!discovery && (String(url) !== expected || !discovered || posted || method !== "POST" || options.body !== JSON.stringify({ candidateId: owner.candidateId }) || new Headers(options.headers).get("Content-Type") !== "application/json" || new Headers(options.headers).get("X-HN-Local-Request") !== "1")) fail();
        if (discovery) discovered = true; else { await beforePost(); posted = true; dispatched(); }
        let response: Response, body: unknown;
        try { response = await request(url, { ...options, credentials: "omit", redirect: "error", ...(discovery ? { cache: "no-store" } : {}), signal: AbortSignal.timeout(30_000) }); body = await response.clone().json(); } catch { fail(); }
        if (!closedObject(body, ["code", "data", "msg"])) fail();
        if (!discovery && body.code === 1 && body.data === null && typeof body.msg === "string" && rejected[response.status]?.[body.msg]) throw new PlacementError(rejected[response.status][body.msg]);
        if (response.status !== 200 || body.code !== 0 || body.msg !== "ok") fail();
        if (discovery) { if (!closedObject(body.data, ["url"]) || typeof body.data.url !== "string") fail(); try { base = localBackendURL(body.data.url); } catch { fail(); } afterDiscovery(); }
        else if (!validPlacementItem(body.data, owner)) fail();
        return response;
    }) as typeof fetch;
}
export function mergeHNLocalSequencePlacementReceipt(nodes: CanvasNodeData[], receipt: HNLocalSequencePlacementReceipt, activeProject: string, captured: HNArchiveTarget) {
    const owner = Object.fromEntries(placementOwnerKeys.map((k) => [k, receipt[k]])) as HNSelectionOwner;
    if (activeProject !== receipt.canvasProjectId || !validHNLocalSequencePlacementReceipt(receipt, owner)) return nodes;
    return nodes.map((node) => {
        try {
            const t = historical(captured), c = t.node.metadata!.hnLocalCandidate!, p = t.node.metadata!.hnLocalPrepared!, a = t.node.metadata!.hnLocalArchive, s = t.node.metadata!.hnLocalSelection;
            return node.id === receipt.sourceNodeId && samePlacementOwner(c, receipt) && c.availabilityStatus === "ARCHIVED" && c.shotId === p.shotId && c.generationId === p.generationId && c.preparedFrozenHash === p.frozenHash && a?.outcome === "ARCHIVED" && a.resultId === receipt.resultId && a.archiveJobId === receipt.archiveJobId && a.sourceMediaFingerprint === receipt.sourceMediaFingerprint && validHNLocalSelectionReceipt(s, receipt) && s.intentId === receipt.selectionIntentId && revision({ canvasProjectId: activeProject, node }) === revision(captured) ? { ...node, metadata: { ...node.metadata, hnLocalSequencePlacement: receipt } } : node;
        } catch { return node; }
    });
}

type Cache = { phase: "loading" | "valid" | "error"; ledger?: PlacementLedger; task?: Promise<void> };
export class HNLocalSequencePlacementController {
    private cache = new Map<string, Cache>();
    private sequences = new Map<string, symbol>();
    private entries = new Map<string, PlacementEntry>();
    private alive = true;
    private epoch = 0;
    constructor(private selection: HNLocalSelectionController, private notify: () => void = () => {}) {
        selection.coordinator.setAdmissionGate((t, operation) => {
            const p = t.node.metadata?.hnLocalPrepared; if (!p) return false;
            const c = this.cache.get(p.hnProjectId); if (c?.phase !== "valid" || !c.ledger) return false;
            const pending = c.ledger.entries.filter((e) => e.state === "PLACING" || e.state === "UNKNOWN");
            return operation === "placement" ? pending.length === 0 : !pending.some((e) => e.owner.shotId === p.shotId);
        });
    }
    activate() { this.alive = true; }
    dispose() { this.alive = false; this.epoch++; }
    entry(project: string, node: string): PlacementEntry { const k = JSON.stringify([project, node]); if (!this.entries.has(k)) this.entries.set(k, { phase: "SELECTION_NOT_ELIGIBLE", running: false, inspection: 0 }); return this.entries.get(k)!; }
    async hydrate(project: string, journal: PlacementJournal = browserPlacementJournal) {
        const prior = this.cache.get(project); if (prior?.task) return prior.task;
        if (prior?.phase === "valid" || prior?.phase === "error" || this.sequences.has(project)) return;
        const c: Cache = { phase: "loading" }; this.cache.set(project, c); this.notify();
        c.task = (async () => {
            try { const v = await journal.getItem(placementJournalKey(project)); const l = v == null ? emptyPlacementLedger(project) : v; if (!await validPlacementLedger(l, project)) fail(); c.ledger = structuredClone(l as PlacementLedger); c.phase = "valid"; }
            catch { c.phase = "error"; }
            finally { c.task = undefined; this.notify(); }
        })();
        return c.task;
    }
    private async capture(read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController, d: PlacementDependencies) {
        const t = historical(read()), c = t.node.metadata?.hnLocalCandidate, se = this.selection.entry(t.canvasProjectId, t.node.id), ce = candidate.entry(t.canvasProjectId, t.node.id), s = t.node.metadata?.hnLocalSelection;
        if (archive.entry(t.canvasProjectId, t.node.id).running || ce.running || se.running || !["CANDIDATE_READY", "CANDIDATE_RELOADED_UNVERIFIED"].includes(ce.phase) || !["SELECTED_CURRENT_SESSION", "SELECTION_RELOADED_UNVERIFIED"].includes(se.phase) || !c || !ce.receipt || canonical(c) !== canonical(ce.receipt)) fail();
        const owner = Object.fromEntries(placementOwnerKeys.map((k) => [k, c[k]])) as HNSelectionOwner;
        if (!validHNLocalSelectionReceipt(s, owner) || !se.receipt || canonical(s) !== canonical(se.receipt)) fail();
        const r = await archive.readHistoricalArchiveReceipt(() => historical(read()), d);
        if (r.outcome !== "ARCHIVED" || !await validHNLocalCandidateReceipt(c, t, r) || revision(t) !== revision(read())) fail();
        return { target: t, owner, selectionIntentId: s.intentId, candidate: c as HNLocalCandidateReceipt };
    }
    async inspect(read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController, d: PlacementDependencies = {}) {
        let t: HNArchiveTarget; try { t = historical(read()); } catch { return; }
        const e = this.entry(t.canvasProjectId, t.node.id), serial = ++e.inspection, epoch = this.epoch;
        if (!this.alive || e.running || e.phase === "PLACEMENT_OUTCOME_UNKNOWN") return;
        const active = () => this.alive && epoch === this.epoch && serial === e.inspection && !e.running && e.phase !== "PLACEMENT_OUTCOME_UNKNOWN";
        const project = t.node.metadata!.hnLocalPrepared!.hnProjectId;
        try {
            await this.hydrate(project, d.placementJournal); if (!active()) return;
            const c = this.cache.get(project); if (c?.phase !== "valid" || !c.ledger) fail();
            if (c.ledger.entries.some((a) => a.state === "PLACING" || a.state === "UNKNOWN")) { e.phase = "PLACEMENT_OUTCOME_UNKNOWN"; return; }
            const selection = this.selection.entry(t.canvasProjectId, t.node.id), candidateEntry = candidate.entry(t.canvasProjectId, t.node.id);
            if (!["SELECTED_CURRENT_SESSION", "SELECTION_RELOADED_UNVERIFIED"].includes(selection.phase) || !["CANDIDATE_READY", "CANDIDATE_RELOADED_UNVERIFIED"].includes(candidateEntry.phase)) { e.phase = "SELECTION_NOT_ELIGIBLE"; e.error = undefined; e.receipt = undefined; return; }
            const captured = await this.capture(read, archive, candidate, d); if (!active()) return;
            const attempt = c.ledger.entries.find((a) => a.owner.candidateId === captured.owner.candidateId), projection = read().node.metadata?.hnLocalSequencePlacement;
            if (!validProjection(projection, c.ledger, captured.target)) { c.phase = "error"; fail(); }
            if (attempt && !samePlacementOwner(attempt.owner, captured.owner)) fail();
            e.error = undefined; e.receipt = attempt?.state === "PLACED" ? structuredClone(attempt.receipt) : undefined;
            e.phase = attempt?.state === "PLACED" ? attempt.placementIntentId === e.sessionIntent ? "PLACED_CURRENT_SESSION" : "PLACEMENT_RELOADED_UNVERIFIED" : attempt?.state === "REJECTED" ? "ERROR" : "NOT_CONFIRMED_PLACED";
            if (attempt?.state === "REJECTED") e.error = rejectionText;
        } catch { if (active()) { e.phase = "ERROR"; e.error = errorText; } }
        finally { if (active()) this.notify(); }
    }
    async place(read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController, d: PlacementDependencies, confirm: (owner: HNSelectionOwner) => Promise<boolean>, receive: (receipt: HNLocalSequencePlacementReceipt, target: HNArchiveTarget) => void) {
        let initial: HNArchiveTarget; try { initial = historical(read()); } catch { return false; }
        const project = initial.node.metadata!.hnLocalPrepared!.hnProjectId, e = this.entry(initial.canvasProjectId, initial.node.id), epoch = this.epoch, identity = Symbol("main");
        if (!this.alive || e.running || this.sequences.has(project) || e.phase === "PLACEMENT_OUTCOME_UNKNOWN") return false;
        this.sequences.set(project, identity);
        const token = this.selection.coordinator.acquire(initial, "placement"); if (!token) { this.sequences.delete(project); return false; }
        const prior = e.phase; e.running = true; e.phase = "PLACING"; e.inspection++; this.notify();
        const journal = d.placementJournal || browserPlacementJournal;
        let dispatched = false, pending = false, captured: Awaited<ReturnType<HNLocalSequencePlacementController["capture"]>> | undefined, attempt: PlacementAttempt | undefined;
        const current = () => { if (!this.alive || epoch !== this.epoch || this.sequences.get(project) !== identity || revision(read()) !== revision(initial)) fail(); };
        const recheck = async () => { current(); const now = await this.capture(read, archive, candidate, d); current(); if (!captured || !samePlacementOwner(now.owner, captured.owner) || now.selectionIntentId !== captured.selectionIntentId) fail(); };
        const seal = async (a: PlacementAttempt) => {
            const c = this.cache.get(project); if (!c?.ledger) fail();
            const value: PlacementLedger = { ...c.ledger, revisionId: crypto.randomUUID(), entries: [...c.ledger.entries.filter((p) => p.owner.candidateId !== a.owner.candidateId), a] };
            // Admission changes synchronously, before persistence or release.
            c.ledger = structuredClone(value);
            await journal.setItem(placementJournalKey(project), structuredClone(value));
            const stored = await journal.getItem(placementJournalKey(project));
            if (!await validPlacementLedger(stored, project) || canonical(stored) !== canonical(value)) fail();
        };
        const unknown = async () => {
            e.phase = "PLACEMENT_OUTCOME_UNKNOWN"; e.error = HN_PLACEMENT_UNKNOWN;
            if (attempt) { const a: PlacementAttempt = { owner: attempt.owner, selectionIntentId: attempt.selectionIntentId, placementIntentId: attempt.placementIntentId, observedAt: attempt.observedAt, state: "UNKNOWN" }; try { await seal(a); } catch { /* Cached UNKNOWN and retained pending stay fail closed. */ } }
            else { const c = this.cache.get(project); if (c) c.phase = "error"; }
        };
        try {
            const cache = this.cache.get(project); if (cache?.phase !== "valid" || !cache.ledger) fail();
            let stored: unknown;
            try { stored = await journal.getItem(placementJournalKey(project)); } catch { cache.phase = "error"; fail(); } current();
            if (stored == null && cache.ledger.entries.length || stored != null && (!await validPlacementLedger(stored, project) || canonical(stored) !== canonical(cache.ledger))) { cache.phase = "error"; fail(); }
            current(); if (cache.ledger.entries.some((a) => a.state === "PLACING" || a.state === "UNKNOWN")) fail();
            captured = await this.capture(read, archive, candidate, d); current();
            const previous = cache.ledger.entries.find((a) => a.owner.candidateId === captured!.owner.candidateId);
            if (previous?.state === "PLACED" && samePlacementOwner(previous.owner, captured.owner)) { e.receipt = structuredClone(previous.receipt); e.phase = previous.placementIntentId === e.sessionIntent ? "PLACED_CURRENT_SESSION" : "PLACEMENT_RELOADED_UNVERIFIED"; e.error = undefined; return false; }
            if (previous && (!samePlacementOwner(previous.owner, captured.owner) || previous.state !== "REJECTED") || !previous && cache.ledger.entries.length >= 256) fail();
            const projection = read().node.metadata?.hnLocalSequencePlacement;
            if (!validProjection(projection, cache.ledger, captured.target)) fail();
            if (!await confirm(structuredClone(captured.owner))) { e.phase = prior; return false; } await recheck();
            const request = hnPlacementRequest(captured.owner, d.request || fetch, () => { dispatched = true; }, async () => {
                await recheck();
                attempt = { owner: captured!.owner, selectionIntentId: captured!.selectionIntentId, placementIntentId: crypto.randomUUID(), observedAt: new Date().toISOString(), state: "PLACING" };
                pending = true; await seal(attempt); await recheck();
            }, current);
            // The helper only consumes identity/ARCHIVED here. Receipt label/time are not server facts and never sent.
            const c = captured.candidate;
            const item = await addSequenceItem({ projectId: project, sequenceId: "main", candidate: { candidateId: c.candidateId, projectId: project, shotId: c.shotId, generationId: c.generationId, resultId: c.resultId, availabilityStatus: "ARCHIVED", label: "", createdAt: "", updatedAt: "" } }, { request });
            if (!attempt || !validPlacementItem(item, captured.owner)) fail();
            const receipt: HNLocalSequencePlacementReceipt = { version: 1, ...captured.owner, sequenceId: "main", sequenceItemId: item.sequenceItemId, orderIndex: item.orderIndex, placementIntentId: attempt.placementIntentId, selectionIntentId: attempt.selectionIntentId, observedAt: attempt.observedAt };
            await seal({ ...attempt, state: "PLACED", receipt });
            e.receipt = receipt; e.sessionIntent = receipt.placementIntentId; e.phase = "PLACED_CURRENT_SESSION"; e.error = undefined;
            // A strictly acknowledged old owner can be sealed without publishing into a new node/project.
            try { current(); receive(receipt, captured.target); } catch { /* Historical ledger already prevents duplicate append. */ }
            return true;
        } catch (err) {
            if (dispatched && !(err instanceof PlacementError && err.rejection)) await unknown();
            else if (pending && attempt) {
                try { await seal({ ...attempt, state: "REJECTED", errorClass: err instanceof PlacementError && err.rejection ? err.rejection : "LOCAL_PREPOST_ABORT" }); e.phase = "ERROR"; e.error = rejectionText; } catch { await unknown(); }
            } else { e.phase = "ERROR"; e.error = errorText; }
            return false;
        } finally { e.running = false; this.selection.coordinator.release(token); if (this.sequences.get(project) === identity) this.sequences.delete(project); if (this.alive && epoch === this.epoch) this.notify(); }
    }
}
