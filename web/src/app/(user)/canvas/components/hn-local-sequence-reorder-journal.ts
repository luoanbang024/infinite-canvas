import { isHNProjectId } from "@/services/hn/local-reference";
import { reorderCanonical, validReorderCommand, validReorderReceipt, type ReorderCommand, type ReorderReceipt } from "@/services/hn/local-sequence-reorder";
export type ReorderJournalState = "PREPARED" | "DISPATCHING" | "UNKNOWN" | "COMMITTED" | "CONFLICT" | "ABORTED_PRE_DISPATCH";
export type ReorderJournalRecord = { journalVersion: 1; projectId: string; sequenceId: "main"; createdAt: string; updatedAt: string; command: ReorderCommand; state: ReorderJournalState; receipt: ReorderReceipt | null };
export type ReorderJournal = { keys: () => Promise<string[]>; getItem: (key: string) => Promise<unknown>; setItem: (key: string, record: ReorderJournalRecord) => Promise<unknown> };
export const reorderJournalPrefix = (project: string) => `hn-r31-reorder-v1/${encodeURIComponent(project)}/main/`;
export const reorderJournalKey = (project: string, intent: string) => reorderJournalPrefix(project) + intent;
export const unresolvedReorder = (r: ReorderJournalRecord) => r.state === "DISPATCHING" || r.state === "UNKNOWN";
export const sameReorderCommand = (a: ReorderCommand, b: ReorderCommand) => a.reorderIntentId === b.reorderIntentId && reorderCanonical(a) === reorderCanonical(b);
const time = (v: unknown): v is string => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString() === v;
export async function validReorderRecord(v: unknown, project: string, key: string): Promise<boolean> {
 if (!v || typeof v !== "object" || Array.isArray(v) || Object.keys(v).sort().join(",") !== "command,createdAt,journalVersion,projectId,receipt,sequenceId,state,updatedAt") return false;
 const r = v as ReorderJournalRecord;
 if (!isHNProjectId(project) || r.projectId !== project || r.journalVersion !== 1 || r.sequenceId !== "main" || !time(r.createdAt) || !time(r.updatedAt) || r.updatedAt < r.createdAt || !validReorderCommand(r.command) || key !== reorderJournalKey(project, r.command.reorderIntentId)) return false;
 if (r.state === "COMMITTED" || r.state === "CONFLICT") return !!r.receipt && r.receipt.outcome === r.state && await validReorderReceipt(r.receipt, project, r.command);
 return ["PREPARED", "DISPATCHING", "UNKNOWN", "ABORTED_PRE_DISPATCH"].includes(r.state) && r.receipt === null;
}
const stable = (v: unknown): string => JSON.stringify(v && typeof v === "object" && !Array.isArray(v) ? Object.fromEntries(Object.entries(v).sort(([a],[b]) => a.localeCompare(b)).map(([k,x]) => [k,JSON.parse(stable(x))])) : Array.isArray(v) ? v.map(x => JSON.parse(stable(x))) : v);
export async function sealReorderRecord(journal: ReorderJournal, record: ReorderJournalRecord) {
 const r = structuredClone(record), key = reorderJournalKey(r.projectId,r.command.reorderIntentId);
 if (!await validReorderRecord(r,r.projectId,key)) throw Error("排序日志无效");
 const previous = await journal.getItem(key);
 if (previous !== null && previous !== undefined) {
  if (!await validReorderRecord(previous,r.projectId,key)) throw Error("排序日志完整性冲突");
  const p = previous as ReorderJournalRecord;
  if (!sameReorderCommand(p.command,r.command) || p.createdAt !== r.createdAt || (p.receipt && stable(p) !== stable(r))) throw Error("排序日志身份冲突");
 }
 const expected = stable(r);
 await journal.setItem(key,structuredClone(r)); const read = await journal.getItem(key);
 if (!await validReorderRecord(read,r.projectId,key) || stable(read) !== expected) throw Error("排序日志未可靠保存");
 return structuredClone(read as ReorderJournalRecord);
}
export async function readReorderJournal(journal: ReorderJournal, project: string) {
 if (!isHNProjectId(project)) throw Error("排序项目无效");
 const keys = (await journal.keys()).filter(k => k.startsWith(reorderJournalPrefix(project))).sort();
 if (new Set(keys).size !== keys.length) throw Error("排序日志完整性冲突");
 const records: ReorderJournalRecord[] = [];
 for (const key of keys) { const v = await journal.getItem(key); if (!await validReorderRecord(v,project,key)) throw Error("排序日志完整性冲突"); records.push(structuredClone(v as ReorderJournalRecord)); }
 return records;
}
// Separate per-intent records; never read or clobber the placement journal.
let store: Promise<LocalForage> | undefined;
const storage = () => store ||= import("localforage").then(({ default: f }) => f.createInstance({ name:"infinite-canvas",storeName:"hn_local_sequence_reorder_v1" }));
export const browserReorderJournal: ReorderJournal = {
 async keys() { return (await storage()).keys(); }, async getItem(k) { return (await storage()).getItem(k); }, async setItem(k,r) { return (await storage()).setItem(k,r); },
};
