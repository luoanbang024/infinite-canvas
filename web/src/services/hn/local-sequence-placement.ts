import { isHNProjectId, localBackendURL } from "./local-reference";
import type { HNSequenceItem } from "./local-editorial";

export type PlacementOwner = { candidateId: string; shotId: string; generationId: string; resultId: string; archiveJobId: string; preparedFrozenHash: string };
export type PlacementCommand = PlacementOwner & { protocolVersion: 1; placementIntentId: string };
export type PlacementReceipt = { protocolVersion: 1; projectId: string; sequenceId: "main"; placementIntentId: string; owner: PlacementOwner; outcome: "COMMITTED" | "REJECTED"; originalItem: HNSequenceItem | null; errorClass: "OWNERSHIP_REJECTED" | null };
export type PlacementNotObserved = { protocolVersion: 1; projectId: string; sequenceId: "main"; placementIntentId: string; outcome: "NOT_OBSERVED" };
export type MainSequenceSnapshot = { protocolVersion: 1; projectId: string; sequenceId: "main"; itemsComplete: true; items: HNSequenceItem[] };
export const placementCanonical = (v: unknown): string => JSON.stringify(v, (_k, x) => x && typeof x === "object" && !Array.isArray(x) ? Object.fromEntries(Object.keys(x).sort().map((k) => [k, x[k]])) : x);
const closed = (v: unknown, keys: string[]): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v) && Object.keys(v).sort().join(",") === keys.sort().join(",");
const uuid = (s: unknown) => typeof s === "string" && /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/.test(s);
const ownerKeys = ["candidateId", "shotId", "generationId", "resultId", "archiveJobId", "preparedFrozenHash"];
export function validCommand(v: unknown): v is PlacementCommand {
    return closed(v, [...ownerKeys, "protocolVersion", "placementIntentId"]) && v.protocolVersion === 1 && uuid(v.placementIntentId) && ownerKeys.slice(0, 5).every((k) => typeof v[k] === "string" && isHNProjectId(v[k] as string)) && typeof v.preparedFrozenHash === "string" && /^[a-f0-9]{64}$/.test(v.preparedFrozenHash);
}
const time = (s: unknown) => typeof s === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(s) && Number.isFinite(Date.parse(s)) && new Date(s).toISOString().slice(0, 19) === s.slice(0, 19);
function validItem(v: unknown, project: string): v is HNSequenceItem {
    return closed(v, ["sequenceItemId", "projectId", "sequenceId", "orderIndex", "shotId", "candidateId", "resultId", "createdAt", "updatedAt"]) && uuid(v.sequenceItemId) && v.projectId === project && v.sequenceId === "main" && Number.isSafeInteger(v.orderIndex) && (v.orderIndex as number) >= 0 && ["shotId", "candidateId", "resultId"].every((k) => typeof v[k] === "string" && isHNProjectId(v[k] as string)) && time(v.createdAt) && time(v.updatedAt);
}
export function validCommandReceipt(v: unknown, project: string, command: PlacementCommand): v is PlacementReceipt {
    if (!validCommand(command) || !closed(v, ["protocolVersion", "projectId", "sequenceId", "placementIntentId", "owner", "outcome", "originalItem", "errorClass"]) || v.protocolVersion !== 1 || v.projectId !== project || v.sequenceId !== "main" || v.placementIntentId !== command.placementIntentId || !closed(v.owner, [...ownerKeys]) || ownerKeys.some((k) => v.owner && (v.owner as Record<string, unknown>)[k] !== command[k as keyof PlacementOwner])) return false;
    if (v.outcome === "REJECTED") return v.originalItem === null && v.errorClass === "OWNERSHIP_REJECTED";
    return v.outcome === "COMMITTED" && v.errorClass === null && validItem(v.originalItem, project) && v.originalItem.shotId === command.shotId && v.originalItem.candidateId === command.candidateId && v.originalItem.resultId === command.resultId;
}
async function boundedJSON(response: Response) {
    if (!response.body) throw Error("HN 序列响应无法确认");
    const reader = response.body.getReader(), chunks: Uint8Array[] = []; let bytes = 0;
    try { while (true) { const { done, value } = await reader.read(); if (done) break; bytes += value.byteLength; if (bytes > 1_048_576) throw Error("HN 序列响应超出范围"); chunks.push(value); } }
    finally { await reader.cancel().catch(() => {}); }
    const data = new Uint8Array(bytes); let offset = 0; for (const chunk of chunks) { data.set(chunk, offset); offset += chunk.byteLength; }
    return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(data)) as unknown;
}
async function exchange(project: string, path: string, command: PlacementCommand | undefined, request: typeof fetch) {
    if (!isHNProjectId(project)) throw Error("HN 序列标识无效");
    const options = { credentials: "omit" as const, redirect: "error" as const, signal: AbortSignal.timeout(30_000), cache: "no-store" as const };
    const discovery = await request("/api/hn/local-endpoint", options), found = await boundedJSON(discovery);
    if (discovery.status !== 200 || !closed(found, ["code", "data", "msg"]) || found.code !== 0 || found.msg !== "ok" || !closed(found.data, ["url"]) || typeof found.data.url !== "string") throw Error("HN 本机地址无法确认");
    const response = await request(`${localBackendURL(found.data.url)}/api/hn/projects/${encodeURIComponent(project)}/sequences/main/${path}`, {
        ...options, method: command ? "POST" : "GET", headers: command ? { "Content-Type": "application/json", "X-HN-Local-Request": "1" } : { "X-HN-Local-Request": "1" }, ...(command ? { body: JSON.stringify(command) } : {}),
    });
    const body = await boundedJSON(response);
    if (response.status !== 200 || !closed(body, ["code", "data", "msg"]) || body.code !== 0 || body.msg !== "ok") throw Error("HN 服务器结果无法确认；不会重新发送");
    return body.data;
}
export async function executePlacement(project: string, command: PlacementCommand, request: typeof fetch = fetch) {
    if (!validCommand(command)) throw Error("HN 加入命令无效"); const captured = structuredClone(command);
    const result = await exchange(project, "placement-commands", captured, request);
    if (!validCommandReceipt(result, project, captured)) throw Error("HN 加入回执不匹配"); return result;
}
export async function lookupPlacement(project: string, command: PlacementCommand, request: typeof fetch = fetch): Promise<PlacementReceipt | PlacementNotObserved> {
    if (!validCommand(command)) throw Error("HN 加入命令无效"); const captured = structuredClone(command);
    const result = await exchange(project, `placement-commands/${encodeURIComponent(captured.placementIntentId)}`, undefined, request);
    if (validCommandReceipt(result, project, captured)) return result;
    if (closed(result, ["protocolVersion", "projectId", "sequenceId", "placementIntentId", "outcome"]) && result.protocolVersion === 1 && result.projectId === project && result.sequenceId === "main" && result.placementIntentId === captured.placementIntentId && result.outcome === "NOT_OBSERVED") return result as PlacementNotObserved;
    throw Error("HN 加入回执不匹配");
}
export async function readMainSequence(project: string, request: typeof fetch = fetch): Promise<MainSequenceSnapshot> {
    const result = await exchange(project, "items", undefined, request);
    if (!closed(result, ["protocolVersion", "projectId", "sequenceId", "itemsComplete", "items"]) || result.protocolVersion !== 1 || result.projectId !== project || result.sequenceId !== "main" || result.itemsComplete !== true || !Array.isArray(result.items) || result.items.length > 256) throw Error("HN 主序列快照无效");
    const ids = new Set<string>(), indices = new Set<number>(); let last = -1;
    for (const item of result.items) { if (!validItem(item, project) || ids.has(item.sequenceItemId) || indices.has(item.orderIndex) || item.orderIndex < last) throw Error("HN 主序列完整性无法确认"); ids.add(item.sequenceItemId); indices.add(item.orderIndex); last = item.orderIndex; }
    return result as MainSequenceSnapshot;
}
