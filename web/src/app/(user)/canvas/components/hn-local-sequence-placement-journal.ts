import { validHNLocalSelectionReceipt, type HNSelectionOwner } from "./hn-local-canvas-selection";
import { mapHNCanvasProjectId } from "./hn-local-canvas-prepare";
import type { HNLocalSequencePlacementReceipt } from "../types";
import { placementCanonical, validCommand, validCommandReceipt, type PlacementCommand, type PlacementReceipt } from "@/services/hn/local-sequence-placement";

export const placementOwnerKeys = ["canvasProjectId", "hnProjectId", "sourceNodeId", "shotId", "generationId", "preparedFrozenHash", "resultId", "archiveJobId", "sourceMediaFingerprint", "candidateId"] as const;
export const closedObject = (v: unknown, names: readonly string[]): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v) && Object.keys(v).sort().join(",") === [...names].sort().join(",");
export const uuid4 = (v: unknown): v is string => typeof v === "string" && /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/.test(v);
export const browserTime = (v: unknown): v is string => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(v) && Number.isFinite(Date.parse(v)) && new Date(v).toISOString() === v;
export const samePlacementOwner = (a: HNSelectionOwner, b: HNSelectionOwner) => placementOwnerKeys.every((k) => a[k] === b[k]);
export function validPlacementOwner(v: unknown): v is HNSelectionOwner {
    return closedObject(v, placementOwnerKeys) && validHNLocalSelectionReceipt({ ...v, version: 1, intentId: "11111111-1111-4111-8111-111111111111", observedAt: "2026-10-06T00:00:00.000Z" }, v as HNSelectionOwner);
}
export function validHNLocalSequencePlacementReceipt(v: unknown, owner: HNSelectionOwner): v is HNLocalSequencePlacementReceipt {
    return closedObject(v, [...placementOwnerKeys, "version", "sequenceId", "sequenceItemId", "orderIndex", "placementIntentId", "selectionIntentId", "observedAt"]) && v.version === 1 && v.sequenceId === "main" && validPlacementOwner(owner) && samePlacementOwner(v as HNSelectionOwner, owner) && uuid4(v.sequenceItemId) && uuid4(v.placementIntentId) && uuid4(v.selectionIntentId) && Number.isSafeInteger(v.orderIndex) && (v.orderIndex as number) >= 0 && browserTime(v.observedAt);
}
export type PlacementRejection = "LOCAL_PREPOST_ABORT" | "INPUT_REJECTED" | "LOCAL_REQUEST_REJECTED" | "OWNERSHIP_REJECTED" | "BACKEND_DISABLED";
type Common = { owner: HNSelectionOwner; selectionIntentId: string; placementIntentId: string; observedAt: string };
export type PlacementAttempt = Common & ({ state: "PLACING" | "UNKNOWN" } | { state: "PLACED"; receipt: HNLocalSequencePlacementReceipt } | { state: "REJECTED"; errorClass: PlacementRejection });
export type PlacementLedger = { version: 1; hnProjectId: string; sequenceId: "main"; revisionId: string; entries: PlacementAttempt[] };
export type PlacementJournal = { getItem: (key: string) => Promise<unknown>; setItem: (key: string, value: PlacementLedger) => Promise<unknown> };
export const placementJournalKey = (project: string) => JSON.stringify(["v1", project, "main"]);
export function emptyPlacementLedger(project: string): PlacementLedger { return { version: 1, hnProjectId: project, sequenceId: "main", revisionId: crypto.randomUUID(), entries: [] }; }
export async function validPlacementLedger(v: unknown, project: string): Promise<boolean> {
    if (!closedObject(v, ["version", "hnProjectId", "sequenceId", "revisionId", "entries"]) || v.version !== 1 || v.hnProjectId !== project || v.sequenceId !== "main" || !uuid4(v.revisionId) || !Array.isArray(v.entries) || v.entries.length > 256) return false;
    const candidates = new Set<string>(), intents = new Set<string>(), items = new Set<string>();
    for (const e of v.entries) {
        const common = ["owner", "selectionIntentId", "placementIntentId", "observedAt", "state"];
        if (!e || !closedObject(e, [...common, ...(e.state === "PLACED" ? ["receipt"] : e.state === "REJECTED" ? ["errorClass"] : [])]) || !validPlacementOwner(e.owner) || e.owner.hnProjectId !== project || await mapHNCanvasProjectId(e.owner.canvasProjectId) !== project || !uuid4(e.selectionIntentId) || !uuid4(e.placementIntentId) || !browserTime(e.observedAt)) return false;
        if (candidates.has(e.owner.candidateId) || intents.has(e.placementIntentId)) return false;
        candidates.add(e.owner.candidateId); intents.add(e.placementIntentId);
        if (e.state === "PLACED") {
            if (!validHNLocalSequencePlacementReceipt(e.receipt, e.owner) || e.receipt.placementIntentId !== e.placementIntentId || e.receipt.selectionIntentId !== e.selectionIntentId || e.receipt.observedAt !== e.observedAt || items.has(e.receipt.sequenceItemId)) return false;
            items.add(e.receipt.sequenceItemId);
        } else if (e.state === "REJECTED") {
            if (!["LOCAL_PREPOST_ABORT", "INPUT_REJECTED", "LOCAL_REQUEST_REJECTED", "OWNERSHIP_REJECTED", "BACKEND_DISABLED"].includes(e.errorClass as string)) return false;
        } else if (e.state !== "PLACING" && e.state !== "UNKNOWN") return false;
    }
    return true;
}
// A single aggregate record is the target history and Sequence barrier together.
let store: Promise<PlacementJournal> | undefined;
export const browserPlacementJournal: PlacementJournal = {
    async getItem(key) { store ||= import("localforage").then(({ default: localforage }) => localforage.createInstance({ name: "infinite-canvas", storeName: "hn_local_sequence_placement_attempts" })); return (await store).getItem(key); },
    async setItem(key, value) { store ||= import("localforage").then(({ default: localforage }) => localforage.createInstance({ name: "infinite-canvas", storeName: "hn_local_sequence_placement_attempts" })); return (await store).setItem(key, value); },
};

export type PlacementV2Attempt = { owner: HNSelectionOwner; selectionIntentId: string; observedAt: string; command: PlacementCommand; state: "PLACING" | "UNKNOWN" | "COMMITTED" | "REJECTED"; receipt?: PlacementReceipt };
export type PlacementV2Ledger = { version: 2; protocolVersion: 1; hnProjectId: string; sequenceId: "main"; revisionId: string; entries: PlacementV2Attempt[] };
export type PlacementV2Journal = { getItem: (key: string) => Promise<unknown>; setItem: (key: string, value: PlacementV2Ledger) => Promise<unknown> };
export type PlacementV2Projection = { version: 2; owner: HNSelectionOwner; selectionIntentId: string; observedAt: string; command: PlacementCommand; receipt: PlacementReceipt };
export const placementV2Key = (project: string) => JSON.stringify(["v2", project, "main"]);
export const commandForOwner = (owner: HNSelectionOwner, intent: string): PlacementCommand => ({ protocolVersion: 1, placementIntentId: intent, candidateId: owner.candidateId, shotId: owner.shotId, generationId: owner.generationId, resultId: owner.resultId, archiveJobId: owner.archiveJobId, preparedFrozenHash: owner.preparedFrozenHash });
export function validV2Attempt(v: unknown, project: string): v is PlacementV2Attempt {
    if (!v || typeof v !== "object") return false;
    const e = v as PlacementV2Attempt, terminal = e.state === "COMMITTED" || e.state === "REJECTED";
    return closedObject(v, ["owner", "selectionIntentId", "observedAt", "command", "state", ...(terminal ? ["receipt"] : [])]) && validPlacementOwner(e.owner) && e.owner.hnProjectId === project && uuid4(e.selectionIntentId) && browserTime(e.observedAt) && validCommand(e.command) && placementCanonical(commandForOwner(e.owner, e.command.placementIntentId)) === placementCanonical(e.command) && (terminal ? validCommandReceipt(e.receipt, project, e.command) && e.receipt.outcome === e.state : e.state === "PLACING" || e.state === "UNKNOWN");
}
export async function validV2Ledger(v: unknown, project: string): Promise<boolean> {
    if (!closedObject(v, ["version", "protocolVersion", "hnProjectId", "sequenceId", "revisionId", "entries"]) || v.version !== 2 || v.protocolVersion !== 1 || v.hnProjectId !== project || v.sequenceId !== "main" || !uuid4(v.revisionId) || !Array.isArray(v.entries) || v.entries.length > 256) return false;
    const intents = new Set<string>(), items = new Set<string>();
    for (const e of v.entries) {
        if (!validV2Attempt(e, project) || await mapHNCanvasProjectId(e.owner.canvasProjectId) !== project || intents.has(e.command.placementIntentId)) return false;
        intents.add(e.command.placementIntentId);
        if (e.receipt?.originalItem) { const id = e.receipt.originalItem.sequenceItemId; if (items.has(id)) return false; items.add(id); }
    }
    return true;
}
export function validV2Projection(v: unknown): v is PlacementV2Projection {
    if (!closedObject(v, ["version", "owner", "selectionIntentId", "observedAt", "command", "receipt"]) || v.version !== 2) return false;
    const p = v as PlacementV2Projection;
    return validV2Attempt({ owner: p.owner, selectionIntentId: p.selectionIntentId, observedAt: p.observedAt, command: p.command, receipt: p.receipt, state: p.receipt?.outcome }, p.owner?.hnProjectId);
}
let storeV2: Promise<PlacementV2Journal> | undefined;
export const browserPlacementV2Journal: PlacementV2Journal = {
    async getItem(key) { storeV2 ||= import("localforage").then(({ default: f }) => f.createInstance({ name: "infinite-canvas", storeName: "hn_local_sequence_placement_commands_v2" })); return (await storeV2).getItem(key); },
    async setItem(key, value) { storeV2 ||= import("localforage").then(({ default: f }) => f.createInstance({ name: "infinite-canvas", storeName: "hn_local_sequence_placement_commands_v2" })); return (await storeV2).setItem(key, value); },
};
