import { isHNProjectId } from "@/services/hn/local-reference";
import { initializeReorder, readReorderSnapshot, type ReorderSnapshot } from "@/services/hn/local-sequence-reorder";
import { initializeExportProtocol, executeExportCommand, lookupExportCommand, verifyExportBundle, type ExportCommand, type ExportStatus, type ExportBundleHealth } from "@/services/hn/local-export-commands";
import { browserExportJournal, readExportJournal, sealExportJournalRecord, sameExportCommand, stableExportRecord, unresolvedExport, terminalExport, type ExportJournal, type ExportJournalRecord } from "./hn-local-sequence-export-journal";
export const HN_EXPORT_EXPLANATION = "加载可能初始化本地安全导出元数据，不导出媒体、不改变序列顺序、不生成视频，也不代表 AI/Provider 成功。导出作用于整个 HN 项目的 main，与打开对话框的视频节点无关。";
export const HN_EXPORT_CONFIRMATION = "从当前已读取的主序列快照创建一个新的本地离线导出包。命令绑定 exact sequenceRevision 与完整有序项；保留前序列若已改变，服务器拒绝而不会改用其他快照。保留成功后，后来排序、加入或删除不改变此历史导出。同一快照再次确认的新 intent 会有意创建新导出包。响应不会自动重试；导出不生成媒体，不证明 AI/Provider 成功。";
export const HN_EXPORT_CONTINUATION = "这是同一条导出命令的继续执行，不会创建新的 exportIntentId 或 ExportID。若此前命令尚未保留，服务器可能现在开始导出；若已经完成，只返回原历史结果；若仍在处理中或恢复受阻，服务器会返回对应状态。不会自动切换到新的 Sequence 快照。这是明确的写入操作，不是只读确认。";
export const HN_EXPORT_UNKNOWN = "导出结果未确认，可能已经提交；不会自动重试。禁止创建新的导出命令。";
export const HN_EXPORT_NOT_OBSERVED = "服务器当前只读快照中尚未观察到该导出命令；这不能证明此前的 POST 永远不会提交。";
export const HN_EXPORT_HISTORICAL = "回执只证明此 exact 命令历史上已提交；不证明当前 main 顺序、当前 Selection 或永久的磁盘健康，也不代表 AI/Provider 生成成功。";
export const HN_EXPORT_HEALTH = {
    VERIFIED: "在本次验证时，当前本地导出包与历史回执匹配。",
    MISSING: "历史导出命令仍然是 COMMITTED，但当前本地导出包缺失。不会自动重建。",
    CORRUPT: "历史导出命令仍然是 COMMITTED，但当前本地导出包完整性验证失败。不会自动重建。",
};
export type ExportEntry = { running: boolean; records: ExportJournalRecord[]; snapshot: ReorderSnapshot | null; currentVerified: boolean; storageBlocked: boolean; error: string | null; health: Record<string, { health: ExportBundleHealth["health"]; observedAt: string }> };
type Dependencies = { journal?: ExportJournal; request?: typeof fetch; uuid?: () => string; now?: () => string };
export class HNLocalSequenceExportController {
    private entries = new Map<string, ExportEntry>();
    private readonly journal: ExportJournal; private readonly request: typeof fetch; private readonly uuid: () => string; private readonly now: () => string;
    constructor(private readonly changed: () => void = () => {}, deps: Dependencies = {}) { this.journal = deps.journal || browserExportJournal; this.request = deps.request || fetch; this.uuid = deps.uuid || (() => crypto.randomUUID()); this.now = deps.now || (() => new Date().toISOString()); }
    private state(project: string) { let e = this.entries.get(project); if (!e) { e = { running: false, records: [], snapshot: null, currentVerified: false, storageBlocked: false, error: null, health: {} }; this.entries.set(project, e); } return e; }
    entry(project: string): ExportEntry { return structuredClone(this.state(project)); }
    canSubmit(project: string) { const e = this.state(project); return isHNProjectId(project) && !e.running && !e.storageBlocked && !e.records.some(unresolvedExport) && e.currentVerified && !!e.snapshot && e.snapshot.items.length > 0 && e.snapshot.items.length <= 256; }
    private async run(project: string, action: (e: ExportEntry) => Promise<boolean>) {
        if (!isHNProjectId(project)) return false; const e = this.state(project); if (e.running) return false;
        e.running = true; e.error = null; this.changed(); // page/project/main lock synchronously precedes first await
        try { return await action(e); } catch { e.error = "安全导出操作未完成；不会自动发送或重试。"; return false; } finally { e.running = false; this.changed(); }
    }
    private put(e: ExportEntry, r: ExportJournalRecord) { e.records = [...e.records.filter(x => x.command.exportIntentId !== r.command.exportIntentId), structuredClone(r)].sort((a, b) => a.createdAt.localeCompare(b.createdAt) || a.command.exportIntentId.localeCompare(b.command.exportIntentId)); }
    private async scan(project: string, e: ExportEntry) {
        try {
            const records = await readExportJournal(this.journal, project);
            for (const old of e.records) {
                const found = records.find(r => r.command.exportIntentId === old.command.exportIntentId);
                if (!found) records.push(structuredClone(old));
                else if (!sameExportCommand(old.command, found.command) || old.createdAt !== found.createdAt || (terminalExport(old) && stableExportRecord(old) !== stableExportRecord(found))) throw Error("identity");
                else if (unresolvedExport(old) && (found.state === "PREPARED" || found.state === "ABORTED_PRE_DISPATCH")) records[records.indexOf(found)] = structuredClone(old);
            }
            e.records = records; e.storageBlocked = false; return true;
        } catch { e.storageBlocked = true; e.error = "导出日志无法可靠读取；已停止新导出。"; return false; }
    }
    inspect(project: string) { return this.run(project, e => this.scan(project, e)); } // journal read only: zero HTTP and zero writes
    private async snapshot(project: string, e: ExportEntry) { e.currentVerified = false; try { e.snapshot = await readReorderSnapshot(project, this.request); e.currentVerified = true; return true; } catch { e.error = "当前导出快照未读取；历史导出命令仍然保留。"; return false; } }
    load(project: string) { return this.run(project, async e => { if (!await this.scan(project, e)) return false; await initializeReorder(project, this.request); await initializeExportProtocol(project, this.request); return this.snapshot(project, e); }); }
    refresh(project: string) { return this.run(project, e => this.snapshot(project, e)); }
    private async settle(project: string, e: ExportEntry, r: ExportJournalRecord, status: ExportStatus) {
        if (status.outcome === "NOT_OBSERVED") throw Error("unobserved");
        const state = ["COMMITTED", "REJECTED", "FAILED"].includes(status.outcome) ? status.outcome as "COMMITTED" | "REJECTED" | "FAILED" : "KNOWN_NONTERMINAL";
        let sealed: ExportJournalRecord;
        try { sealed = await sealExportJournalRecord(this.journal, { ...r, state, status, updatedAt: this.now() }); } catch (error) { e.storageBlocked = true; throw error; }
        this.put(e, sealed); this.changed();
        if (terminalExport(sealed)) await this.snapshot(project, e); // optional readonly current preview, never another export
        return true;
    }
    private async dispatch(project: string, e: ExportEntry, r: ExportJournalRecord, continuation: boolean) {
        let dispatched = false, latest = r;
        const request = (async (input: Parameters<typeof fetch>[0], init?: RequestInit) => {
            if (init?.method === "POST") {
                const u = new URL(String(input));
                if (u.pathname !== `/api/hn/projects/${encodeURIComponent(project)}/sequences/main/export-commands` || init.body !== JSON.stringify(r.command) || dispatched) throw Error("dispatch identity");
                latest = await sealExportJournalRecord(this.journal, { ...r, state: "DISPATCHING", status: null, updatedAt: this.now() });
                this.put(e, latest); e.currentVerified = false;
                dispatched = true; // only after exact durable DISPATCHING readback, immediately before actual forwarding
            }
            return this.request(input, init);
        }) as typeof fetch;
        try { const status = await executeExportCommand(project, r.command, request); return await this.settle(project, e, latest, status); }
        catch {
            latest = { ...latest, state: dispatched || continuation ? "UNKNOWN" : "ABORTED_PRE_DISPATCH", status: null, updatedAt: this.now() };
            this.put(e, latest);
            try { this.put(e, await sealExportJournalRecord(this.journal, latest)); } catch { e.storageBlocked = true; /* durable DISPATCHING or in-memory unresolved barrier remains */ }
            e.error = latest.state === "UNKNOWN" ? HN_EXPORT_UNKNOWN : "发送前已停止；导出命令 POST 为零。请检查本机服务或日志存储。"; return false;
        }
    }
    submit(project: string, confirm: (command: ExportCommand) => Promise<boolean>) {
        if (!this.canSubmit(project)) return Promise.resolve(false);
        return this.run(project, async e => {
            if (!await this.scan(project, e) || e.records.some(unresolvedExport)) return false;
            const preview: ExportCommand = { protocolVersion: 1, exportIntentId: "00000000-0000-4000-8000-000000000000", expectedRevision: e.snapshot!.sequenceRevision, orderedSequenceItemIds: e.snapshot!.items.map(i => i.sequenceItemId), format: "hn-offline-bundle-v1" };
            if (!await confirm(structuredClone(preview))) return false;
            if (!await this.scan(project, e) || e.records.some(unresolvedExport)) return false;
            const now = this.now(), r: ExportJournalRecord = { journalVersion: 1, projectId: project, sequenceId: "main", createdAt: now, updatedAt: now, command: { ...preview, exportIntentId: this.uuid() }, state: "PREPARED", status: null };
            try { this.put(e, await sealExportJournalRecord(this.journal, r)); } catch { e.storageBlocked = true; e.error = "导出日志未可靠保存；导出命令 POST 为零。"; return false; }
            return this.dispatch(project, e, r, false);
        });
    }
    lookup(project: string, intent: string) { return this.run(project, async e => {
        if (!await this.scan(project, e)) return false; const r = e.records.find(r => r.command.exportIntentId === intent && unresolvedExport(r)); if (!r) return false;
        try { const status = await lookupExportCommand(project, r.command, this.request); if (status.outcome === "NOT_OBSERVED") { e.error = HN_EXPORT_NOT_OBSERVED; return false; } return await this.settle(project, e, r, status); }
        catch { e.error = HN_EXPORT_UNKNOWN; return false; }
    }); }
    continueSame(project: string, intent: string, confirm: (command: ExportCommand) => Promise<boolean>) { return this.run(project, async e => {
        if (!await this.scan(project, e)) return false; const r = e.records.find(r => r.command.exportIntentId === intent && unresolvedExport(r));
        if (!r || !await confirm(structuredClone(r.command))) return false;
        if (!await this.scan(project, e)) return false; const current = e.records.find(x => x.command.exportIntentId === intent && unresolvedExport(x));
        if (!current || !sameExportCommand(current.command, r.command)) return false;
        return this.dispatch(project, e, current, true);
    }); }
    verifyBundle(project: string, intent: string) { return this.run(project, async e => {
        if (!await this.scan(project, e)) return false; const r = e.records.find(r => r.command.exportIntentId === intent && r.state === "COMMITTED"); if (!r) return false;
        delete e.health[intent]; const result = await verifyExportBundle(project, r.command, this.request);
        if (result.exportId !== r.status!.receipt!.exportId) throw Error("导出回执身份冲突");
        e.health[intent] = { health: result.health, observedAt: this.now() }; return true; // ephemeral only, no journal write or rebuild
    }); }
}
