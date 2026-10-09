import { isHNProjectId } from "@/services/hn/local-reference";
import { exportCanonical, validExportCommand, validExportStatus, type ExportCommand, type ExportStatus } from "@/services/hn/local-export-commands";
export type ExportJournalState = "PREPARED" | "DISPATCHING" | "UNKNOWN" | "KNOWN_NONTERMINAL" | "COMMITTED" | "REJECTED" | "FAILED" | "ABORTED_PRE_DISPATCH";
export type ExportJournalRecord = { journalVersion: 1; projectId: string; sequenceId: "main"; createdAt: string; updatedAt: string; command: ExportCommand; state: ExportJournalState; status: ExportStatus | null };
export type ExportJournal = { keys: () => Promise<string[]>; getItem: (key: string) => Promise<unknown>; setItem: (key: string, record: ExportJournalRecord) => Promise<unknown> };
export const exportJournalPrefix = (project: string) => `hn-r34-export-v1/${encodeURIComponent(project)}/main/`;
export const exportJournalKey = (project: string, intent: string) => exportJournalPrefix(project) + intent;
export const unresolvedExport = (r: ExportJournalRecord) => ["PREPARED", "DISPATCHING", "UNKNOWN", "KNOWN_NONTERMINAL"].includes(r.state);
export const terminalExport = (r: ExportJournalRecord) => ["COMMITTED", "REJECTED", "FAILED"].includes(r.state);
export const sameExportCommand = (a: ExportCommand, b: ExportCommand) => a.exportIntentId === b.exportIntentId && exportCanonical(a) === exportCanonical(b);
const time = (v: unknown): v is string => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString() === v;
export const stableExportRecord = (v: unknown): string => JSON.stringify(v && typeof v === "object" && !Array.isArray(v) ? Object.fromEntries(Object.entries(v).sort(([a], [b]) => a.localeCompare(b)).map(([k, x]) => [k, JSON.parse(stableExportRecord(x))])) : Array.isArray(v) ? v.map(x => JSON.parse(stableExportRecord(x))) : v);
export async function validExportJournalRecord(v: unknown, project: string, key: string): Promise<boolean> {
    if (!v || typeof v !== "object" || Array.isArray(v) || Object.keys(v).sort().join(",") !== "command,createdAt,journalVersion,projectId,sequenceId,state,status,updatedAt") return false;
    const r = v as ExportJournalRecord;
    if (!isHNProjectId(project) || r.projectId !== project || r.sequenceId !== "main" || r.journalVersion !== 1 || !time(r.createdAt) || !time(r.updatedAt) || r.updatedAt < r.createdAt || !validExportCommand(r.command) || key !== exportJournalKey(project, r.command.exportIntentId)) return false;
    if (terminalExport(r)) return !!r.status && r.status.outcome === r.state && await validExportStatus(r.status, project, r.command);
    if (r.state === "KNOWN_NONTERMINAL") return !!r.status && ["IN_PROGRESS", "RESERVED", "COPYING", "READY_TO_FINALIZE", "RETRYABLE", "RECOVERY_BLOCKED"].includes(r.status.outcome) && await validExportStatus(r.status, project, r.command);
    return ["PREPARED", "DISPATCHING", "UNKNOWN", "ABORTED_PRE_DISPATCH"].includes(r.state) && r.status === null;
}
export async function sealExportJournalRecord(journal: ExportJournal, record: ExportJournalRecord) {
    const r = structuredClone(record), key = exportJournalKey(r.projectId, r.command.exportIntentId);
    if (!await validExportJournalRecord(r, r.projectId, key)) throw Error("导出日志无效");
    const previous = await journal.getItem(key);
    if (previous !== null && previous !== undefined) {
        if (!await validExportJournalRecord(previous, r.projectId, key)) throw Error("导出日志完整性冲突");
        const p = previous as ExportJournalRecord;
        if (!sameExportCommand(p.command, r.command) || p.createdAt !== r.createdAt || r.updatedAt < p.updatedAt || (terminalExport(p) && stableExportRecord(p) !== stableExportRecord(r))) throw Error("导出日志身份冲突");
    }
    const expected = stableExportRecord(r);
    await journal.setItem(key, structuredClone(r));
    const read = await journal.getItem(key);
    if (!await validExportJournalRecord(read, r.projectId, key) || stableExportRecord(read) !== expected) throw Error("导出日志未可靠保存");
    return structuredClone(read as ExportJournalRecord);
}
export async function readExportJournal(journal: ExportJournal, project: string) {
    if (!isHNProjectId(project)) throw Error("导出项目无效");
    const keys = (await journal.keys()).filter(k => k.startsWith(exportJournalPrefix(project))).sort();
    if (new Set(keys).size !== keys.length) throw Error("导出日志完整性冲突");
    const records: ExportJournalRecord[] = [];
    for (const key of keys) {
        const v = await journal.getItem(key);
        if (!await validExportJournalRecord(v, project, key)) throw Error("导出日志完整性冲突");
        records.push(structuredClone(v as ExportJournalRecord));
    }
    return records;
}
// Dedicated per-intent localforage store; no placement/reorder record or key reuse.
let store: Promise<LocalForage> | undefined;
const storage = () => store ||= import("localforage").then(({ default: f }) => f.createInstance({ name: "infinite-canvas", storeName: "hn_local_sequence_export_v1" }));
export const browserExportJournal: ExportJournal = {
    async keys() { return (await storage()).keys(); }, async getItem(k) { return (await storage()).getItem(k); }, async setItem(k, r) { return (await storage()).setItem(k, r); },
};
