import { isHNProjectId, localBackendURL } from "./local-reference";
import type { HNSequenceItem } from "./local-editorial";
export type SequenceRevision = string;
export type ReorderCommand = { protocolVersion: 1; reorderIntentId: string; expectedRevision: string; desiredSequenceItemIds: string[] };
export type ReorderState = { protocolVersion: 1; projectId: string; sequenceId: "main"; sequenceRevision: string };
export type ReorderSnapshot = ReorderState & { itemsComplete: true; items: HNSequenceItem[] };
export type ReorderReceipt = { protocolVersion: 1; projectId: string; sequenceId: "main"; reorderIntentId: string; canonicalRequestHash: string; expectedRevision: string; outcome: "COMMITTED" | "CONFLICT"; observedRevision: string; appliedRevision: string | null; desiredSequenceItemIds: string[]; errorClass: "SEQUENCE_REVISION_CONFLICT" | "SEQUENCE_SET_CONFLICT" | null };
export type ReorderNotObserved = { protocolVersion: 1; projectId: string; sequenceId: "main"; reorderIntentId: string; outcome: "NOT_OBSERVED" };
const closed = (v: unknown, keys: string[]): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v) && Object.keys(v).sort().join(",") === [...keys].sort().join(",");
const uuid = (v: unknown) => typeof v === "string" && /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/.test(v);
const id = (v: unknown): v is string => typeof v === "string" && isHNProjectId(v);
export const validSequenceRevision = (v: unknown): v is string => typeof v === "string" && /^(0|[1-9][0-9]{0,18})$/.test(v) && BigInt(v) <= BigInt("9223372036854775807");
const validIDs = (v: unknown): v is string[] => Array.isArray(v) && v.length <= 256 && v.every(id) && new Set(v).size === v.length;
export function validReorderCommand(v: unknown): v is ReorderCommand {
 return closed(v, ["protocolVersion", "reorderIntentId", "expectedRevision", "desiredSequenceItemIds"]) && v.protocolVersion === 1 && uuid(v.reorderIntentId) && validSequenceRevision(v.expectedRevision) && validIDs(v.desiredSequenceItemIds);
}
export const reorderCanonical = (c: ReorderCommand) => JSON.stringify({ expectedRevision: c.expectedRevision, desiredSequenceItemIds: c.desiredSequenceItemIds });
async function canonicalHash(c: ReorderCommand) {
 const b = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(reorderCanonical(c)));
 return Array.from(new Uint8Array(b), (v) => v.toString(16).padStart(2, "0")).join("");
}
const time = (v: unknown) => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString().slice(0,19) === v.slice(0,19);
function validItem(v: unknown, project: string): v is HNSequenceItem {
 return closed(v, ["sequenceItemId","projectId","sequenceId","orderIndex","shotId","candidateId","resultId","createdAt","updatedAt"]) && id(v.sequenceItemId) && v.projectId === project && v.sequenceId === "main" && Number.isSafeInteger(v.orderIndex) && (v.orderIndex as number) >= 0 && id(v.shotId) && id(v.candidateId) && id(v.resultId) && time(v.createdAt) && time(v.updatedAt);
}
function validState(v: unknown, project: string): v is ReorderState {
 return closed(v, ["protocolVersion","projectId","sequenceId","sequenceRevision"]) && v.protocolVersion === 1 && v.projectId === project && v.sequenceId === "main" && validSequenceRevision(v.sequenceRevision);
}
export async function validReorderReceipt(v: unknown, project: string, c: ReorderCommand): Promise<boolean> {
 if (!validReorderCommand(c) || !closed(v, ["protocolVersion","projectId","sequenceId","reorderIntentId","canonicalRequestHash","expectedRevision","outcome","observedRevision","appliedRevision","desiredSequenceItemIds","errorClass"]) || v.protocolVersion !== 1 || v.projectId !== project || v.sequenceId !== "main" || v.reorderIntentId !== c.reorderIntentId || v.expectedRevision !== c.expectedRevision || !validIDs(v.desiredSequenceItemIds) || JSON.stringify(v.desiredSequenceItemIds) !== JSON.stringify(c.desiredSequenceItemIds) || !validSequenceRevision(v.observedRevision) || v.canonicalRequestHash !== await canonicalHash(c)) return false;
 if (v.outcome === "COMMITTED") return v.errorClass === null && v.observedRevision === c.expectedRevision && validSequenceRevision(v.appliedRevision) && BigInt(v.appliedRevision) === BigInt(v.observedRevision) + BigInt(1);
 return v.outcome === "CONFLICT" && v.appliedRevision === null && ((v.errorClass === "SEQUENCE_REVISION_CONFLICT" && v.observedRevision !== c.expectedRevision) || (v.errorClass === "SEQUENCE_SET_CONFLICT" && v.observedRevision === c.expectedRevision));
}
async function boundedJSON(r: Response) {
 if (!r.body || r.redirected) throw Error("HN 排序结果无法确认");
 const reader = r.body.getReader(), chunks: Uint8Array[] = []; let count = 0;
 try { while (true) { const {done,value} = await reader.read(); if (done) break; count += value.byteLength; if (count > 1_048_576) throw Error("HN 排序响应超出范围"); chunks.push(value); } } finally { await reader.cancel().catch(() => {}); }
 const data = new Uint8Array(count); let offset = 0; for (const b of chunks) { data.set(b,offset); offset += b.byteLength; }
 return JSON.parse(new TextDecoder("utf-8",{fatal:true}).decode(data)) as unknown;
}
async function exchange(project: string, path: string, input: object | undefined, request: typeof fetch) {
 if (!id(project)) throw Error("HN 排序项目无效");
 const body = input ? JSON.stringify(input) : undefined;
 if (body && new TextEncoder().encode(body).byteLength > 65_536) throw Error("HN 排序请求超出范围");
 const options = {credentials:"omit" as const,redirect:"error" as const,cache:"no-store" as const,signal:AbortSignal.timeout(30_000)};
 const discovery = await request("/api/hn/local-endpoint",options), found = await boundedJSON(discovery);
 if (discovery.status !== 200 || !closed(found,["code","data","msg"]) || found.code !== 0 || found.msg !== "ok" || !closed(found.data,["url"]) || typeof found.data.url !== "string") throw Error("HN 本机地址无法确认");
 const r = await request(localBackendURL(found.data.url)+"/api/hn/projects/"+encodeURIComponent(project)+"/sequences/main/"+path,{...options,method:input?"POST":"GET",headers:input?{"Content-Type":"application/json","X-HN-Local-Request":"1"}:{"X-HN-Local-Request":"1"},...(body?{body}:{})});
 const v = await boundedJSON(r);
 if (!closed(v,["code","data","msg"])) throw Error("HN 排序结果无法确认；不会重新发送");
 if (r.status === 200 && v.code === 0 && v.msg === "ok") return v.data;
 if (path === "reorder-commands" && r.status === 409 && v.code === 1 && closed(v.data,["protocolVersion","projectId","sequenceId","reorderIntentId","canonicalRequestHash","expectedRevision","outcome","observedRevision","appliedRevision","desiredSequenceItemIds","errorClass"]) && v.data.outcome === "CONFLICT" && v.msg === v.data.errorClass) return v.data;
 throw Error("HN 排序结果无法确认；不会重新发送");
}
export async function initializeReorder(project: string, request: typeof fetch = fetch): Promise<ReorderState> {
 const v = await exchange(project,"reorder-protocol/initialize",{protocolVersion:1},request); if (!validState(v,project)) throw Error("HN 排序协议状态无效"); return v;
}
export async function readReorderSnapshot(project: string, request: typeof fetch = fetch): Promise<ReorderSnapshot> {
 const v = await exchange(project,"reorder-snapshot",undefined,request);
 if (!closed(v,["protocolVersion","projectId","sequenceId","itemsComplete","sequenceRevision","items"]) || !validState({protocolVersion:v.protocolVersion,projectId:v.projectId,sequenceId:v.sequenceId,sequenceRevision:v.sequenceRevision},project) || v.itemsComplete !== true || !Array.isArray(v.items) || v.items.length > 256) throw Error("HN 排序完整快照无效");
 const ids = new Set<string>(), indices = new Set<number>(); let last = -1;
 for (const i of v.items) { if (!validItem(i,project) || ids.has(i.sequenceItemId) || indices.has(i.orderIndex) || i.orderIndex < last) throw Error("HN 排序完整性冲突"); ids.add(i.sequenceItemId); indices.add(i.orderIndex); last = i.orderIndex; }
 return v as ReorderSnapshot;
}
export async function executeReorder(project: string, command: ReorderCommand, request: typeof fetch = fetch): Promise<ReorderReceipt> {
 if (!validReorderCommand(command)) throw Error("HN 排序命令无效"); const captured = structuredClone(command);
 const v = await exchange(project,"reorder-commands",captured,request); if (!await validReorderReceipt(v,project,captured)) throw Error("HN 排序回执不匹配"); return v as ReorderReceipt;
}
export async function lookupReorder(project: string, command: ReorderCommand, request: typeof fetch = fetch): Promise<ReorderReceipt | ReorderNotObserved> {
 if (!validReorderCommand(command)) throw Error("HN 排序命令无效"); const captured = structuredClone(command);
 const v = await exchange(project,"reorder-commands/"+encodeURIComponent(captured.reorderIntentId),undefined,request);
 if (await validReorderReceipt(v,project,captured)) return v as ReorderReceipt;
 if (closed(v,["protocolVersion","projectId","sequenceId","reorderIntentId","outcome"]) && v.protocolVersion === 1 && v.projectId === project && v.sequenceId === "main" && v.reorderIntentId === captured.reorderIntentId && v.outcome === "NOT_OBSERVED") return v as ReorderNotObserved;
 throw Error("HN 排序回执不匹配");
}

