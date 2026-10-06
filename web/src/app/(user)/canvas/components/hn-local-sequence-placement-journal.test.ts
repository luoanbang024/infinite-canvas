import { test } from "node:test";
import assert from "node:assert/strict";
import { placementFixture } from "./hn-local-canvas-sequence-placement.test";
import { emptyPlacementLedger, validPlacementLedger, validHNLocalSequencePlacementReceipt, placementJournalKey, type PlacementLedger } from "./hn-local-sequence-placement-journal";
import { HNLocalSequencePlacementController } from "./hn-local-canvas-sequence-placement";

test("R26 closed ledger unions, owner mapping, uniqueness, capacity, no raw fields", async () => {
    const p = await placementFixture(); assert.equal(await p.run(), true); const good = structuredClone(p.values.values().next().value as PlacementLedger), receipt = p.entry().receipt!;
    assert.equal(await validPlacementLedger(good, p.s.owner.hnProjectId), true); assert.equal(Object.keys(good).length, 5); assert.equal(placementJournalKey(p.s.owner.hnProjectId), JSON.stringify(["v1", p.s.owner.hnProjectId, "main"]));
    const mutations: ((v: PlacementLedger) => void)[] = [v => v.entries.push(v.entries[0]), v => v.sequenceId = "other" as "main", v => v.revisionId = "invalid", v => v.entries[0].owner.canvasProjectId = "other", v => v.entries[0].selectionIntentId = crypto.randomUUID(), v => v.entries[0].observedAt = "2026-02-31T00:00:00.000Z", v => Object.assign(v.entries[0], { rawBody: "synthetic" }), v => Object.assign(v.entries[0].owner, { extra: "synthetic" }), v => v.entries = Array(257).fill(v.entries[0])];
    for (const mutate of mutations) { const v = structuredClone(good); mutate(v); assert.equal(await validPlacementLedger(v, p.s.owner.hnProjectId), false); }
    for (const key of Object.keys(receipt)) { const v: Record<string, unknown> = { ...receipt }; delete v[key]; assert.equal(validHNLocalSequencePlacementReceipt(v, p.s.owner), false); }
    for (const state of ["PLACING", "UNKNOWN", "REJECTED"] as const) { const v = structuredClone(good), e = v.entries[0]; const common = { owner: e.owner, selectionIntentId: e.selectionIntentId, placementIntentId: e.placementIntentId, observedAt: e.observedAt }; v.entries = [state === "REJECTED" ? { ...common, state, errorClass: "INPUT_REJECTED" } : { ...common, state }]; assert.equal(await validPlacementLedger(v, p.s.owner.hnProjectId), true); }
    const bad = structuredClone(good); delete (bad as unknown as Record<string, unknown>).revisionId; assert.equal(await validPlacementLedger(bad, p.s.owner.hnProjectId), false);
});
test("R26 pending before dispatch/reload, missing journal projection, malformed hydrate fail closed", async () => {
    const p = await placementFixture(); assert.equal(await p.run(), true); const good = structuredClone(p.values.values().next().value as PlacementLedger);
    for (const mode of ["missing", "malformed", "pending"] as const) {
        const v = mode === "missing" ? null : mode === "malformed" ? { ...good, unexpected: true } : { ...good, entries: good.entries.map(e => ({ owner: e.owner, placementIntentId: e.placementIntentId, selectionIntentId: e.selectionIntentId, observedAt: e.observedAt, state: "PLACING" })) };
        const journal = { getItem: async () => structuredClone(v), setItem: async () => assert.fail("inspect writes ledger") };
        const c = new HNLocalSequencePlacementController(p.s.controller); await c.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, { ...p.d, placementJournal: journal });
        assert.equal(c.entry(p.s.f.project, p.s.f.nodeId).phase, mode === "pending" ? "PLACEMENT_OUTCOME_UNKNOWN" : "ERROR"); assert.equal(await c.place(p.s.f.current, p.s.f.archive, p.s.f.c, { ...p.d, placementJournal: journal }, p.confirm, p.receive), false); assert.equal(p.counts.post, 1);
    }
});
test("R26 hydrate first read gate, capacity 256 is client-only and never evicts", async () => {
    const p = await placementFixture(), c = new HNLocalSequencePlacementController(p.s.controller);
    let resolve!: (v: unknown) => void; const waiting = new Promise<unknown>(r => resolve = r); const d = { ...p.d, placementJournal: { getItem: async () => waiting, setItem: async () => assert.fail("no write") } };
    const inspect = c.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, d);
    for (const operation of ["archive", "candidate", "selection", "placement"] as const) assert.equal(p.s.controller.coordinator.acquire(p.s.f.current(), operation, p.s.owner), undefined);
    resolve(null); await inspect; const l = emptyPlacementLedger(p.s.owner.hnProjectId);
    for (let i = 0; i < 256; i++) l.entries.push({ owner: { ...p.s.owner, candidateId: "candidate-" + i }, placementIntentId: crypto.randomUUID(), selectionIntentId: crypto.randomUUID(), observedAt: new Date().toISOString(), state: "REJECTED", errorClass: "INPUT_REJECTED" });
    assert.equal(await validPlacementLedger(l, p.s.owner.hnProjectId), true); p.values.set(placementJournalKey(p.s.owner.hnProjectId), l);
    const full = new HNLocalSequencePlacementController(p.s.controller); await full.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, p.d); assert.equal(await full.place(p.s.f.current, p.s.f.archive, p.s.f.c, p.d, p.confirm, p.receive), false); assert.equal(p.counts.post, 0); assert.equal(l.entries.length, 256);
});
