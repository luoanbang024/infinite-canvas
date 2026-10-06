import { selectCandidate } from "@/services/hn/local-editorial";
import { isHNProjectId, localBackendURL } from "@/services/hn/local-reference";
import { validHNLocalReceipt } from "./hn-local-canvas-prepare";
import { HNLocalArchiveController, type HNArchiveTarget } from "./hn-local-canvas-archive";
import { HNLocalCandidateController, validHNLocalCandidateReceipt, type HNCandidateDependencies } from "./hn-local-canvas-candidate";
import { CanvasNodeType, type CanvasNodeData, type HNLocalCandidateReceipt, type HNLocalSelectionReceipt } from "../types";

export const HN_SELECTION_EXPLANATION = "选择会更新此镜头当前使用的候选；不会加入序列，也不会导出，不代表 AI/Provider 生成成功。";
export const HN_SELECTION_REPEAT = "再次选择会重新写入当前选择，并可能覆盖之后由其他操作设置的候选。";
export const HN_SELECTION_NO_READ = "本页没有可靠的当前选择查询；再次选择是写入操作。";
export const HN_SELECTION_CANDIDATE_UNVERIFIED = "历史候选回执未复核；选择时本地后端会校验该版本并写入选择。";
export const HN_SELECTION_ERRORS = {
    HN_SELECTION_NOT_ELIGIBLE: "没有可用的历史候选回执，未发送选择请求。",
    HN_SELECTION_OWNER_MISMATCH: "选择目标与当前画布、节点或历史候选不一致，已停止。",
    HN_SELECTION_CONFLICT: "候选归属或归档完整性校验未通过，未完成本次选择。",
    HN_SELECTION_BACKEND_UNAVAILABLE: "本地选择服务不可用，未发送选择请求。",
    HN_SELECTION_REJECTED: "本地后端拒绝了本次选择，请检查候选回执。",
    HN_SELECTION_UNKNOWN: "选择结果未确认，可能已经写入；不会自动重试。再次选择可能覆盖后续选择。",
    HN_SELECTION_UNVERIFIED: "历史选择回执未复核，不能确认服务器当前选择。",
    HN_SELECTION_BUSY: "此镜头有进行中或结果未确认的操作，请先处理原操作。",
} as const;
type ErrorCode = keyof typeof HN_SELECTION_ERRORS;
class SelectionError extends Error { constructor(public readonly code: ErrorCode, public readonly rejected = false) { super(HN_SELECTION_ERRORS[code]); } }
function fail(code: ErrorCode): never { throw new SelectionError(code); }
export type HNSelectionOwner = Omit<HNLocalSelectionReceipt, "version" | "intentId" | "observedAt">;
export type HNSelectionPhase = "CANDIDATE_NOT_ELIGIBLE" | "NOT_CONFIRMED_SELECTED" | "SELECTING" | "SELECTED_CURRENT_SESSION" | "SELECTION_OUTCOME_UNKNOWN" | "SELECTION_RELOADED_UNVERIFIED" | "ERROR";
export type HNSelectionEntry = { phase: HNSelectionPhase; running: boolean; inspection: number; owner?: HNSelectionOwner; receipt?: HNLocalSelectionReceipt; sessionIntent?: string; error?: ErrorCode };
export type HNSelectionDependencies = HNCandidateDependencies;
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const keys = (v: object, expected: readonly string[]) => Object.keys(v).sort().join(",") === [...expected].sort().join(",");
const safe = (v: unknown): v is string => typeof v === "string" && isHNProjectId(v);
const canvasId = (v: unknown): v is string => typeof v === "string" && /^[A-Za-z0-9_-]{1,128}$/.test(v);
const hash = (v: unknown) => typeof v === "string" && /^[a-f0-9]{64}$/.test(v);
const ownerKeys = ["canvasProjectId", "hnProjectId", "sourceNodeId", "shotId", "generationId", "preparedFrozenHash", "resultId", "archiveJobId", "sourceMediaFingerprint", "candidateId"] as const;
const same = (a: HNSelectionOwner, b: HNSelectionOwner) => ownerKeys.every((k) => a[k] === b[k]);
function shape(v: unknown): v is HNLocalSelectionReceipt {
    if (!object(v) || !keys(v, [...ownerKeys, "version", "intentId", "observedAt"])) return false;
    return v.version === 1 && canvasId(v.canvasProjectId) && canvasId(v.sourceNodeId) && [v.hnProjectId, v.shotId, v.generationId, v.resultId, v.archiveJobId, v.candidateId].every(safe) && hash(v.preparedFrozenHash) && hash(v.sourceMediaFingerprint) && typeof v.intentId === "string" && /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/.test(v.intentId) && typeof v.observedAt === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(v.observedAt) && Number.isFinite(Date.parse(v.observedAt)) && new Date(v.observedAt).toISOString() === v.observedAt;
}
export function validHNLocalSelectionReceipt(v: unknown, owner: HNSelectionOwner): v is HNLocalSelectionReceipt { return shape(v) && same(v, owner); }

// Project only historical owner fields. Current media/content/storage are not read.
function historicalTarget(t: HNArchiveTarget): HNArchiveTarget {
    const n = t.node, p = n.metadata?.hnLocalPrepared;
    if (n.type !== CanvasNodeType.Video || !canvasId(n.id) || !canvasId(t.canvasProjectId) || !p || !validHNLocalReceipt(p, t.canvasProjectId, n.id, p.hnProjectId)) fail("HN_SELECTION_OWNER_MISMATCH");
    return { canvasProjectId: t.canvasProjectId, node: { id: n.id, type: n.type, title: "", width: 0, height: 0, position: { x: 0, y: 0 }, metadata: { hnLocalPrepared: structuredClone(p), hnLocalArchive: structuredClone(n.metadata?.hnLocalArchive), hnLocalCandidate: structuredClone(n.metadata?.hnLocalCandidate) } } };
}
const revision = (t: HNArchiveTarget) => JSON.stringify(historicalTarget(t));
type Operation = "archive" | "candidate" | "selection" | "placement";
type Token = { shotKey: string; nodeKey: string; identity: symbol };

export class HNShotOperationCoordinator {
    private admission?: (target: HNArchiveTarget, operation: Operation) => boolean;
    setAdmissionGate(gate: (target: HNArchiveTarget, operation: Operation) => boolean) { this.admission = gate; }
    private shots = new Map<string, Token>();
    private nodes = new Map<string, Token>();
    private unknown = new Map<string, HNSelectionOwner>();
    constructor(private notify: () => void = () => {}) {}
    private identifiers(target: HNArchiveTarget) {
        const t = historicalTarget(target), p = t.node.metadata!.hnLocalPrepared!;
        return { shotKey: JSON.stringify([p.hnProjectId, p.shotId]), nodeKey: JSON.stringify([t.canvasProjectId, t.node.id]) };
    }
    acquire(target: HNArchiveTarget, operation: Operation, owner?: HNSelectionOwner): Token | undefined {
        try {
            if (this.admission && !this.admission(target, operation)) return;
            const ids = this.identifiers(target), barrier = this.unknown.get(ids.shotKey);
            if (this.shots.has(ids.shotKey) || this.nodes.has(ids.nodeKey) || barrier && (operation !== "selection" || !owner || !same(barrier, owner))) return;
            const token = { ...ids, identity: Symbol(operation) }; this.shots.set(token.shotKey, token); this.nodes.set(token.nodeKey, token); this.notify(); return token;
        } catch { return; }
    }
    release(token: Token) {
        if (this.shots.get(token.shotKey) === token) this.shots.delete(token.shotKey);
        if (this.nodes.get(token.nodeKey) === token) this.nodes.delete(token.nodeKey);
        this.notify();
    }
    markUnknown(token: Token, owner: HNSelectionOwner) { this.unknown.set(token.shotKey, structuredClone(owner)); this.notify(); }
    clearUnknown(token: Token, owner: HNSelectionOwner) { const prior = this.unknown.get(token.shotKey); if (prior && same(prior, owner)) this.unknown.delete(token.shotKey); this.notify(); }
    allowsSelection(token: Token, owner: HNSelectionOwner) { const b = this.unknown.get(token.shotKey); return this.shots.get(token.shotKey) === token && this.nodes.get(token.nodeKey) === token && token.shotKey === JSON.stringify([owner.hnProjectId, owner.shotId]) && (!b || same(b, owner)); }
    blocked(target: HNArchiveTarget, operation: Operation, owner?: HNSelectionOwner) {
        if (this.admission && !this.admission(target, operation)) return true;
        try { const ids = this.identifiers(target), b = this.unknown.get(ids.shotKey); return this.shots.has(ids.shotKey) || this.nodes.has(ids.nodeKey) || !!b && (operation !== "selection" || !owner || !same(b, owner)); } catch { return true; }
    }
}

const rejections: Record<number, Record<string, ErrorCode>> = {
    400: Object.fromEntries(["HN 剪辑只接受 JSON", "HN 剪辑输入无效", "HN 剪辑输入含不支持字段", "HN 剪辑标识或输入无效"].map((s) => [s, "HN_SELECTION_REJECTED"])),
    403: Object.fromEntries(["HN 剪辑需要显式本机请求标记", "HN 参考冻结仅允许本机请求", "HN 参考冻结不接受此页面来源"].map((s) => [s, "HN_SELECTION_REJECTED"])),
    409: { EDITORIAL_OWNERSHIP_CONFLICT: "HN_SELECTION_CONFLICT" },
    503: { "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT": "HN_SELECTION_BACKEND_UNAVAILABLE" },
};
export function hnSelectionRequest(owner: HNSelectionOwner, request: typeof fetch, dispatched: () => void, beforePost: () => Promise<void>) {
    let discovered = false, posted = false, base = "";
    return (async (url, options = {}) => {
        const method = options.method || "GET", discovery = url === "/api/hn/local-endpoint" && method === "GET" && !discovered && !posted;
        const expected = `${base}/api/hn/projects/${encodeURIComponent(owner.hnProjectId)}/shots/${owner.shotId}/candidates/${owner.candidateId}/select`;
        if (!discovery && (String(url) !== expected || !discovered || posted || method !== "POST" || options.body !== "{}" || new Headers(options.headers).get("Content-Type") !== "application/json" || new Headers(options.headers).get("X-HN-Local-Request") !== "1")) fail("HN_SELECTION_OWNER_MISMATCH");
        if (discovery) discovered = true; else { await beforePost(); posted = true; dispatched(); }
        let response: Response, body: unknown;
        try { response = await request(url, { ...options, credentials: "omit", redirect: "error", ...(discovery ? { cache: "no-store" } : {}), signal: AbortSignal.timeout(30_000) }); body = await response.clone().json(); } catch { fail(discovery ? "HN_SELECTION_BACKEND_UNAVAILABLE" : "HN_SELECTION_UNKNOWN"); }
        if (!object(body) || !keys(body, ["code", "data", "msg"])) fail(discovery ? "HN_SELECTION_BACKEND_UNAVAILABLE" : "HN_SELECTION_UNKNOWN");
        if (!discovery && body.code === 1 && body.data === null && typeof body.msg === "string") { const code = rejections[response.status]?.[body.msg]; if (code) throw new SelectionError(code, true); }
        if (response.status !== 200 || body.code !== 0 || body.msg !== "ok" || !object(body.data)) fail(discovery ? "HN_SELECTION_BACKEND_UNAVAILABLE" : "HN_SELECTION_UNKNOWN");
        if (discovery) { if (!keys(body.data, ["url"]) || typeof body.data.url !== "string") fail("HN_SELECTION_BACKEND_UNAVAILABLE"); try { base = localBackendURL(body.data.url); } catch { fail("HN_SELECTION_BACKEND_UNAVAILABLE"); } }
        else if (!keys(body.data, ["shotId", "selectedCandidateId"]) || body.data.shotId !== owner.shotId || body.data.selectedCandidateId !== owner.candidateId) fail("HN_SELECTION_UNKNOWN");
        return response;
    }) as typeof fetch;
}

export function mergeHNLocalSelectionReceipt(nodes: CanvasNodeData[], receipt: HNLocalSelectionReceipt, activeProject: string, captured: HNArchiveTarget) {
    if (activeProject !== receipt.canvasProjectId || !validHNLocalSelectionReceipt(receipt, receipt)) return nodes;
    return nodes.map((node) => {
        try {
            const t = historicalTarget(captured), c = t.node.metadata?.hnLocalCandidate, p = t.node.metadata!.hnLocalPrepared!, a = t.node.metadata?.hnLocalArchive;
            const ownerMatches = c && c.availabilityStatus === "ARCHIVED" && c.shotId === p.shotId && c.generationId === p.generationId && c.preparedFrozenHash === p.frozenHash && ownerKeys.every((k) => receipt[k] === c[k]);
            const archiveMatches = !a || a.outcome === "ARCHIVED" && a.resultId === receipt.resultId && a.archiveJobId === receipt.archiveJobId && a.sourceMediaFingerprint === receipt.sourceMediaFingerprint;
            return ownerMatches && archiveMatches && node.id === receipt.sourceNodeId && revision({ canvasProjectId: activeProject, node }) === revision(captured) ? { ...node, metadata: { ...node.metadata, hnLocalSelection: receipt } } : node;
        } catch { return node; }
    });
}

export class HNLocalSelectionController {
    readonly coordinator: HNShotOperationCoordinator;
    private entries = new Map<string, HNSelectionEntry>();
    private alive = true;
    private epoch = 0;
    constructor(private notify: () => void = () => {}) { this.coordinator = new HNShotOperationCoordinator(notify); }
    activate() { this.alive = true; }
    dispose() { this.alive = false; this.epoch++; }
    entry(project: string, node: string): HNSelectionEntry { const key = JSON.stringify([project, node]); if (!this.entries.has(key)) this.entries.set(key, { phase: "CANDIDATE_NOT_ELIGIBLE", running: false, inspection: 0 }); return this.entries.get(key)!; }
    async runOperation(operation: "archive" | "candidate", read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController | undefined, action: () => Promise<boolean>) {
        let t: HNArchiveTarget; try { t = historicalTarget(read()); } catch { return false; }
        if (!this.alive || archive.entry(t.canvasProjectId, t.node.id).running || candidate?.entry(t.canvasProjectId, t.node.id).running) return false;
        const token = this.coordinator.acquire(t, operation); if (!token) return false;
        try { return await action(); } finally { this.coordinator.release(token); }
    }
    private async capture(read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController, d: HNSelectionDependencies) {
        const t = historicalTarget(read()), c = t.node.metadata?.hnLocalCandidate, e = candidate.entry(t.canvasProjectId, t.node.id);
        if (archive.entry(t.canvasProjectId, t.node.id).running || e.running || !["CANDIDATE_READY", "CANDIDATE_RELOADED_UNVERIFIED"].includes(e.phase) || !object(c) || !e.receipt || JSON.stringify(c) !== JSON.stringify(e.receipt)) fail("HN_SELECTION_NOT_ELIGIBLE");
        const r = await archive.readHistoricalArchiveReceipt(() => historicalTarget(read()), d);
        if (r.outcome !== "ARCHIVED" || !await validHNLocalCandidateReceipt(c, t, r) || revision(read()) !== revision(t)) fail("HN_SELECTION_OWNER_MISMATCH");
        // The existing async validator is not a TS predicate; narrow only after validation.
        const valid = c as HNLocalCandidateReceipt;
        const owner: HNSelectionOwner = Object.fromEntries(ownerKeys.map((k) => [k, valid[k]])) as HNSelectionOwner;
        return { target: t, owner };
    }
    async inspect(read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController, d: HNSelectionDependencies = {}) {
        let t: HNArchiveTarget; try { t = read(); } catch { return; }
        const e = this.entry(t.canvasProjectId, t.node.id), serial = ++e.inspection, epoch = this.epoch;
        if (!this.alive || e.running || e.phase === "SELECTION_OUTCOME_UNKNOWN") return;
        const active = () => this.alive && epoch === this.epoch && serial === e.inspection && !e.running && e.phase !== "SELECTION_OUTCOME_UNKNOWN";
        try {
            const captured = await this.capture(read, archive, candidate, d); if (!active()) return;
            const r: unknown = read().node.metadata?.hnLocalSelection;
            if (r != null && (!shape(r) || r.canvasProjectId !== captured.owner.canvasProjectId || r.sourceNodeId !== captured.owner.sourceNodeId || r.hnProjectId !== captured.owner.hnProjectId)) fail("HN_SELECTION_OWNER_MISMATCH");
            e.owner = captured.owner; e.receipt = validHNLocalSelectionReceipt(r, captured.owner) ? structuredClone(r) : undefined; e.error = undefined;
            e.phase = e.receipt ? e.receipt.intentId === e.sessionIntent ? "SELECTED_CURRENT_SESSION" : "SELECTION_RELOADED_UNVERIFIED" : "NOT_CONFIRMED_SELECTED";
        } catch (err) { if (active()) { e.phase = err instanceof SelectionError && err.code === "HN_SELECTION_NOT_ELIGIBLE" ? "CANDIDATE_NOT_ELIGIBLE" : "ERROR"; e.owner = undefined; e.receipt = undefined; e.error = err instanceof SelectionError ? err.code : "HN_SELECTION_OWNER_MISMATCH"; } }
        finally { if (active()) this.notify(); }
    }
    async select(read: () => HNArchiveTarget, archive: HNLocalArchiveController, candidate: HNLocalCandidateController, d: HNSelectionDependencies, confirm: (owner: HNSelectionOwner) => Promise<boolean>, onReceipt: (receipt: HNLocalSelectionReceipt, target: HNArchiveTarget) => void) {
        let initial: HNArchiveTarget; try { initial = historicalTarget(read()); } catch { return false; }
        const e = this.entry(initial.canvasProjectId, initial.node.id), prior = e.phase, priorOwner = e.owner, epoch = this.epoch;
        if (!this.alive || e.running || archive.entry(initial.canvasProjectId, initial.node.id).running || candidate.entry(initial.canvasProjectId, initial.node.id).running) return false;
        const token = this.coordinator.acquire(initial, "selection", priorOwner); if (!token) return false;
        e.running = true; e.phase = "SELECTING"; e.inspection++; e.error = undefined; this.notify();
        let dispatched = false, captured: Awaited<ReturnType<HNLocalSelectionController["capture"]>> | undefined;
        const current = () => { if (!this.alive || epoch !== this.epoch || revision(read()) !== revision(initial)) fail("HN_SELECTION_OWNER_MISMATCH"); return read(); };
        const recheck = async () => { const now = await this.capture(current, archive, candidate, d); if (!captured || !same(now.owner, captured.owner) || !this.coordinator.allowsSelection(token, now.owner)) fail("HN_SELECTION_OWNER_MISMATCH"); current(); };
        try {
            captured = await this.capture(current, archive, candidate, d); current();
            if (!this.coordinator.allowsSelection(token, captured.owner) || prior === "SELECTION_OUTCOME_UNKNOWN" && (!priorOwner || !same(captured.owner, priorOwner))) fail("HN_SELECTION_BUSY");
            e.owner = captured.owner;
            const persisted: unknown = current().node.metadata?.hnLocalSelection;
            if (persisted != null && (!shape(persisted) || persisted.canvasProjectId !== captured.owner.canvasProjectId || persisted.sourceNodeId !== captured.owner.sourceNodeId || persisted.hnProjectId !== captured.owner.hnProjectId)) fail("HN_SELECTION_OWNER_MISMATCH");
            await recheck();
            if (!await confirm(structuredClone(captured.owner))) { e.phase = prior; return false; }
            const intentId = crypto.randomUUID(); await recheck();
            const request = hnSelectionRequest(captured.owner, d.request || fetch, () => { dispatched = true; }, recheck);
            await selectCandidate({ projectId: captured.owner.hnProjectId, shotId: captured.owner.shotId, candidateId: captured.owner.candidateId }, { request });
            await recheck();
            const receipt: HNLocalSelectionReceipt = { version: 1, ...captured.owner, intentId, observedAt: new Date().toISOString() };
            if (!validHNLocalSelectionReceipt(receipt, captured.owner)) fail("HN_SELECTION_UNKNOWN");
            onReceipt(receipt, captured.target); current();
            e.receipt = receipt; e.sessionIntent = intentId; e.phase = "SELECTED_CURRENT_SESSION"; this.coordinator.clearUnknown(token, captured.owner); return true;
        } catch (err) {
            const unknown = dispatched && !(err instanceof SelectionError && err.rejected);
            e.phase = unknown || prior === "SELECTION_OUTCOME_UNKNOWN" ? "SELECTION_OUTCOME_UNKNOWN" : "ERROR";
            e.error = unknown ? "HN_SELECTION_UNKNOWN" : err instanceof SelectionError ? err.code : "HN_SELECTION_OWNER_MISMATCH";
            if (unknown && captured) { e.owner = captured.owner; this.coordinator.markUnknown(token, captured.owner); }
            return false;
        } finally { e.running = false; this.coordinator.release(token); if (this.alive && epoch === this.epoch) this.notify(); }
    }
}
