import { test, after } from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import { exportCommandHash, type ExportCommand, type ExportStatus, type ExportReceipt } from "@/services/hn/local-export-commands";
import { readExportJournal, sealExportJournalRecord, validExportJournalRecord, exportJournalPrefix, exportJournalKey, unresolvedExport, type ExportJournal, type ExportJournalRecord } from "./hn-local-sequence-export-journal";
export function exportRecord(project = "project", intent = crypto.randomUUID()): ExportJournalRecord { const now = "2026-10-09T00:00:00.000Z"; return { journalVersion: 1, projectId: project, sequenceId: "main", createdAt: now, updatedAt: now, command: { protocolVersion: 1, exportIntentId: intent, expectedRevision: "3", orderedSequenceItemIds: ["item-A", "item-B"], format: "hn-offline-bundle-v1" }, state: "PREPARED", status: null }; }
export function exportMemory() {
    const values = new Map<string, unknown>(), events: string[] = [];
    const journal: ExportJournal = { async keys() { events.push("keys"); return [...values.keys()]; }, async getItem(k) { const v = values.get(k) ?? null; events.push("read:" + (v as ExportJournalRecord | null)?.state); return structuredClone(v); }, async setItem(k, r) { events.push("write:" + r.state); values.set(k, structuredClone(r)); return r; } };
    return { values, events, journal };
}
export async function exportStatus(project: string, c: ExportCommand, outcome: ExportStatus["outcome"] = "COMMITTED", ownedId?: string | null): Promise<ExportStatus> {
    const base = { protocolVersion: 1 as const, projectId: project, sequenceId: "main" as const, exportIntentId: c.exportIntentId };
    if (outcome === "NOT_OBSERVED") return { ...base, outcome, command: null, canonicalRequestHash: null, exportId: null, attemptCount: 0, errorClass: null, receipt: null };
    const terminal = ["COMMITTED", "REJECTED", "FAILED"].includes(outcome), rejected = outcome === "REJECTED", failed = outcome === "FAILED", exportId = rejected ? null : ownedId || crypto.randomUUID(), path = exportId ? "exports/commands-v1/" + exportId : null, hash = await exportCommandHash(c);
    const errorClass = rejected ? "SEQUENCE_REVISION_CONFLICT" : failed ? "EXPORT_SOURCE_INTEGRITY" : outcome === "RETRYABLE" ? "EXPORT_IO_RETRYABLE" : outcome === "RECOVERY_BLOCKED" ? "EXPORT_RECOVERY_BLOCKED" : null;
    const receipt: ExportReceipt | null = terminal ? { ...base, canonicalRequestHash: hash, expectedRevision: c.expectedRevision, observedRevision: rejected ? (BigInt(c.expectedRevision) + BigInt(1)).toString() : c.expectedRevision, orderedSequenceItemIds: [...c.orderedSequenceItemIds], format: c.format, outcome: outcome as ExportReceipt["outcome"], exportId, bundleRelativePath: path, manifestJsonRelativePath: path ? path + "/ordered-manifest.json" : null, manifestCsvRelativePath: path ? path + "/ordered-manifest.csv" : null, itemCount: rejected ? 0 : c.orderedSequenceItemIds.length, snapshotHash: rejected ? null : "a".repeat(64), manifestJsonSHA256: outcome === "COMMITTED" ? "b".repeat(64) : null, manifestCsvSHA256: outcome === "COMMITTED" ? "c".repeat(64) : null, commandMarkerSHA256: outcome === "COMMITTED" ? "d".repeat(64) : null, completedAt: "2026-10-09T00:00:00Z", errorClass } : null;
    return { ...base, outcome, command: structuredClone(c), canonicalRequestHash: hash, exportId, attemptCount: rejected || outcome === "RESERVED" ? 0 : 1, errorClass, receipt };
}
export function r34Evidence(name: string, data: unknown) { const dir = process.env.HN_R34_EVIDENCE; if (dir) { mkdirSync(dir, { recursive: true }); writeFileSync(dir + "/" + name + ".json", JSON.stringify(data, null, 2)); } }
const facts: unknown[] = [];
test("R34 journal strict closed record/project/main/version/time/command/state binding", async () => {
    const r = exportRecord(), key = exportJournalKey(r.projectId, r.command.exportIntentId); assert.equal(await validExportJournalRecord(r, r.projectId, key), true); assert.equal(exportJournalPrefix("project"), "hn-r34-export-v1/project/main/");
    for (const bad of [{ ...r, extra: 1 }, { ...r, journalVersion: 2 }, { ...r, projectId: "other" }, { ...r, sequenceId: "other" }, { ...r, createdAt: "2026-02-30T00:00:00.000Z" }, { ...r, updatedAt: "2025-01-01T00:00:00.000Z" }, { ...r, command: { ...r.command, protocolVersion: 2 } }, { ...r, command: { ...r.command, expectedRevision: "03" } }, { ...r, command: { ...r.command, orderedSequenceItemIds: ["A", "A"] } }, { ...r, state: "NOT_OBSERVED" }, { ...r, state: "KNOWN_NONTERMINAL", status: await exportStatus(r.projectId, r.command, "NOT_OBSERVED") }, { ...r, status: {} }]) assert.equal(await validExportJournalRecord(bad, r.projectId, key), false);
    assert.equal(await validExportJournalRecord(r, "other", key), false); assert.equal(await validExportJournalRecord(r, r.projectId, key + "x"), false); facts.push({ scenario: "strict-closed-binding", pass: true });
});
test("R34 journal all allowed states exact status and immutable terminal receipts", async () => {
    for (const state of ["PREPARED", "DISPATCHING", "UNKNOWN", "ABORTED_PRE_DISPATCH"] as const) { const m = exportMemory(), r = { ...exportRecord(), state }; assert.deepEqual(await sealExportJournalRecord(m.journal, r), r); assert.equal(unresolvedExport(r), state !== "ABORTED_PRE_DISPATCH"); }
    for (const outcome of ["IN_PROGRESS", "RESERVED", "COPYING", "READY_TO_FINALIZE", "RETRYABLE", "RECOVERY_BLOCKED", "COMMITTED", "REJECTED", "FAILED"] as const) {
        const m = exportMemory(), r = exportRecord(), status = await exportStatus(r.projectId, r.command, outcome), record = { ...r, state: ["COMMITTED", "REJECTED", "FAILED"].includes(outcome) ? outcome as "COMMITTED" | "REJECTED" | "FAILED" : "KNOWN_NONTERMINAL" as const, status };
        await sealExportJournalRecord(m.journal, record); assert.equal(await validExportJournalRecord({ ...record, status: { ...status, exportIntentId: crypto.randomUUID() } }, r.projectId, exportJournalKey(r.projectId, r.command.exportIntentId)), false);
        if (["COMMITTED", "REJECTED", "FAILED"].includes(outcome)) { await assert.rejects(sealExportJournalRecord(m.journal, { ...record, state: "UNKNOWN", status: null })); await assert.rejects(sealExportJournalRecord(m.journal, { ...record, updatedAt: "2026-10-09T01:00:00.000Z" })); }
        facts.push({ scenario: "status-binding-" + outcome, record });
    }
    const r = exportRecord(), s = { ...await exportStatus(r.projectId, r.command, "NOT_OBSERVED"), outcome: "IN_PROGRESS" as const }; assert.equal(await validExportJournalRecord({ ...r, state: "KNOWN_NONTERMINAL", status: s }, r.projectId, exportJournalKey(r.projectId, r.command.exportIntentId)), true);
});
test("R34 per-intent durable readback identity conflicts and corrupt project fail closed", async () => {
    for (const state of ["PREPARED", "DISPATCHING"] as const) {
        const m = exportMemory(), r = { ...exportRecord(), state }; await sealExportJournalRecord(m.journal, r);
        for (const changed of [{ ...r, command: { ...r.command, expectedRevision: "4" } }, { ...r, command: { ...r.command, orderedSequenceItemIds: [...r.command.orderedSequenceItemIds].reverse() } }, { ...r, createdAt: "2026-10-08T00:00:00.000Z" }]) await assert.rejects(sealExportJournalRecord(m.journal, changed));
        const broken = { ...m.journal, getItem: async (k: string) => { const v = await m.journal.getItem(k) as ExportJournalRecord; return v ? { ...v, command: { ...v.command, expectedRevision: "99" } } : null; } }; await assert.rejects(sealExportJournalRecord(broken, r));
    }
    const m = exportMemory(); m.values.set(exportJournalKey("project", crypto.randomUUID()), { broken: true }); await assert.rejects(readExportJournal(m.journal, "project")); assert.deepEqual(await readExportJournal(m.journal, "other"), []);
});
test("R34 separate unresolved intents survive JSON/localforage-shaped reopen and inspect is read-only", async () => {
    const m = exportMemory(), a = { ...exportRecord(), state: "UNKNOWN" as const }, b = { ...exportRecord(), state: "DISPATCHING" as const }, foreign = exportRecord("other");
    for (const r of [a, b, foreign]) await sealExportJournalRecord(m.journal, r);
    const serialized = JSON.stringify([...m.values]), reopened = new Map<string, unknown>(JSON.parse(serialized)); let writes = 0;
    const journal: ExportJournal = { keys: async () => [...reopened.keys()], getItem: async k => structuredClone(reopened.get(k) ?? null), setItem: async () => { writes++; assert.fail("inspection write"); } };
    const records = await readExportJournal(journal, "project"); assert.equal(records.length, 2); assert.ok(records.every(unresolvedExport)); assert.equal((await readExportJournal(journal, "other")).length, 1); assert.equal(writes, 0); facts.push({ scenario: "json-reopen-project-isolation", records, serialized });
});
after(() => r34Evidence("journal", facts));
