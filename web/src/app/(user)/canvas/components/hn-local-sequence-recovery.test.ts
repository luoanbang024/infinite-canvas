import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import { selectionFixture } from "./hn-local-canvas-selection.test";
import { HNLocalSequencePlacementController, mergeHNLocalSequencePlacementReceipt, type PlacementProjection } from "./hn-local-canvas-sequence-placement";
import { placementV2Key, placementJournalKey, validV2Ledger, type PlacementV2Journal, type PlacementV2Ledger, type PlacementJournal } from "./hn-local-sequence-placement-journal";
import { readMainSequence, validCommandReceipt, type PlacementCommand, type PlacementReceipt } from "@/services/hn/local-sequence-placement";

const evidence: unknown[] = [];
export async function recoveryFixture() {
    const s = await selectionFixture(); await s.run();
    const values = new Map<string, unknown>(), v1values = new Map<string, unknown>(), counters = { post: 0, lookup: 0, discovery: 0, item: 0, merge: 0, confirmation: 0 };
    let failWrite = false, readMode = "normal", responseMode = "normal", observed = true, corrupt = false;
    const journal: PlacementV2Journal = { async getItem(k) { const v = structuredClone(values.get(k) ?? null); if (v && readMode === "corrupt") return { ...v, revisionId: crypto.randomUUID() }; return v; }, async setItem(k, v) { if (failWrite) throw Error("synthetic storage unavailable"); values.set(k, structuredClone(v)); return v; } };
    const v1: PlacementJournal = { async getItem(k) { return structuredClone(v1values.get(k) ?? null); }, async setItem(k, v) { v1values.set(k, structuredClone(v)); return v; } };
    const server = new Map<string, PlacementReceipt>(), bodies: PlacementCommand[] = [];
    const request = (async (u, o) => {
        assert.equal(o?.credentials, "omit"); assert.equal(o?.redirect, "error"); assert.ok(o?.signal);
        if (u === "/api/hn/local-endpoint") { counters.discovery++; return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); }
        assert.equal(new Headers(o?.headers).get("X-HN-Local-Request"), "1");
        const base = `http://127.0.0.1:8085/api/hn/projects/${s.owner.hnProjectId}/sequences/main/placement-commands`;
        let data: unknown;
        if (o?.method === "POST") {
            assert.equal(u, base); counters.post++; const command = JSON.parse(o.body as string) as PlacementCommand; bodies.push(command);
            const ledger = values.get(placementV2Key(s.owner.hnProjectId)) as PlacementV2Ledger; assert.ok(ledger.entries.some((e) => e.command.placementIntentId === command.placementIntentId && ["PLACING", "UNKNOWN"].includes(e.state)));
            let receipt = server.get(command.placementIntentId);
            if (!receipt) {
                const rejected = responseMode.startsWith("reject");
                if (!rejected) counters.item++;
                const { protocolVersion: _v, placementIntentId: _i, ...owner } = command;
                receipt = { protocolVersion: 1, projectId: s.owner.hnProjectId, sequenceId: "main", placementIntentId: command.placementIntentId, owner, outcome: "COMMITTED", errorClass: null, originalItem: { sequenceItemId: crypto.randomUUID(), projectId: s.owner.hnProjectId, sequenceId: "main", orderIndex: counters.item - 1, candidateId: command.candidateId, shotId: command.shotId, resultId: command.resultId, createdAt: "2026-10-06T00:00:00Z", updatedAt: "2026-10-06T00:00:00Z" } };
                if (rejected) receipt = { ...receipt, outcome: "REJECTED", originalItem: null, errorClass: "OWNERSHIP_REJECTED" };
                server.set(command.placementIntentId, receipt);
            }
            if (responseMode.endsWith("loss")) throw Error("synthetic terminal response loss");
            data = receipt;
        } else {
            assert.equal(o?.method, "GET"); assert.equal(o?.body, undefined); counters.lookup++;
            if (responseMode === "lookup-error") throw Error("synthetic lookup failure");
            if (String(u).endsWith("/items")) return Response.json({ code: 0, data: { protocolVersion: 1, projectId: s.owner.hnProjectId, sequenceId: "main", itemsComplete: true, items: [...server.values()].flatMap((r) => r.originalItem ? [r.originalItem] : []) }, msg: "ok" });
            const intent = String(u).slice(base.length + 1); assert.equal(u, base + "/" + intent);
            const stored = server.get(intent);
            data = observed && stored ? stored : { protocolVersion: 1, projectId: s.owner.hnProjectId, sequenceId: "main", placementIntentId: intent, outcome: "NOT_OBSERVED" };
            if (corrupt) data = { ...(data as object), extra: true };
        }
        return Response.json({ code: 0, data, msg: "ok" });
    }) as typeof fetch;
    const d = { ...s.d, placementJournal: v1, placementV2Journal: journal, request };
    const controller = new HNLocalSequencePlacementController(s.controller, () => {}, true);
    const receive = (p: PlacementProjection, t: ReturnType<typeof s.f.current>) => { assert.equal(p.version, 2); counters.merge++; s.f.setTarget({ ...s.f.current(), node: mergeHNLocalSequencePlacementReceipt([s.f.current().node], p, s.f.project, t)[0] }); };
    const confirm = async () => { counters.confirmation++; return true; };
    const run = () => controller.place(s.f.current, s.f.archive, s.f.c, d, confirm, receive);
    const entry = () => controller.entry(s.f.project, s.f.nodeId);
    const inspect = () => controller.inspect(s.f.current, s.f.archive, s.f.c, d);
    await inspect();
    return { s, d, values, v1values, journal, controller, counters, server, bodies, run, entry, inspect, receive, confirm, setWrite: (v: boolean) => failWrite = v, setRead: (v: string) => readMode = v, setResponse: (v: string) => responseMode = v, setObserved: (v: boolean) => observed = v, setCorrupt: (v: boolean) => corrupt = v };
}
test("R28 v2 durable prepost gate, confirmed receipt/reload and known-success repeat zero", async () => {
    const f = await recoveryFixture(); assert.equal(f.entry().phase, "NOT_CONFIRMED_PLACED"); assert.equal(await f.run(), true); assert.equal(f.counters.post, 1); assert.equal(await f.run(), false); assert.equal(f.counters.post, 1);
    assert.ok(await validV2Ledger(f.values.get(placementV2Key(f.s.owner.hnProjectId)), f.s.owner.hnProjectId));
    const reload = new HNLocalSequencePlacementController(f.s.controller, () => {}, true); await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d); assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).phase, "PLACEMENT_RELOADED_UNVERIFIED"); assert.equal(f.counters.lookup, 0);
    for (const mode of ["write", "readback"]) { const bad = await recoveryFixture(); if (mode === "write") bad.setWrite(true); else bad.setRead("corrupt"); assert.equal(await bad.run(), false); assert.equal(bad.counters.post, 0); assert.equal(bad.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); }
    evidence.push({ scenario: "durable-gate-known-repeat", post: f.counters.post, item: f.counters.item, providerCalls: 0 });
});
test("R28 response loss, exact GET recovery and NOT_OBSERVED cannot release UNKNOWN", async () => {
    const f = await recoveryFixture(); f.setResponse("loss"); assert.equal(await f.run(), false); assert.equal(f.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(f.counters.post, 1); assert.equal(f.counters.item, 1);
    const states = [f.s.f.archive.entry(f.s.f.project, f.s.f.nodeId).phase, f.s.f.entry().phase, f.s.entry().phase];
    for (const op of ["archive", "candidate", "selection", "placement"] as const) assert.equal(f.s.controller.coordinator.acquire(f.s.f.current(), op, f.s.owner), undefined);
    f.setObserved(false); assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive), false); assert.equal(f.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN");
    f.setObserved(true); f.setCorrupt(true); assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive), false); assert.equal(f.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN");
    f.setCorrupt(false); assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive), true); assert.equal(f.counters.post, 1); assert.equal(f.counters.item, 1); assert.equal(f.entry().phase, "PLACED_CURRENT_SESSION");
    assert.deepEqual(states, [f.s.f.archive.entry(f.s.f.project, f.s.f.nodeId).phase, f.s.f.entry().phase, f.s.entry().phase]);
    evidence.push({ scenario: "committed-loss-get", ...f.counters, promotion: "NONE", notObservedRetained: true, malformedRetained: true });
});
test("R28 explicit continuation exact same intent/payload; repeated loss never fresh resends", async () => {
    const f = await recoveryFixture(); f.setResponse("loss"); await f.run(); const first = structuredClone(f.bodies[0]);
    assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive, async () => false), false); assert.equal(f.counters.post, 1);
    assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive, f.confirm), false); assert.equal(f.counters.post, 2); assert.equal(f.counters.item, 1); assert.deepEqual(f.bodies[1], first);
    f.setResponse("normal"); assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive, f.confirm), true); assert.deepEqual(f.bodies[2], first); assert.equal(f.counters.item, 1);
    evidence.push({ scenario: "explicit-same-key-continuation", ...f.counters, allPayloadsEqual: true });
});
test("R28 strict REJECTED read resolves only its v2 target", async () => {
    const f = await recoveryFixture(); f.setResponse("reject-loss"); await f.run(); assert.equal(f.counters.item, 0);
    assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive), true); assert.equal(f.entry().phase, "ERROR"); assert.equal(f.entry().recoverable, false); assert.equal(f.counters.post, 1); assert.equal(f.counters.merge, 0);
});

test("R28 terminal seal ambiguity, lookup errors and snapshot never attribute UNKNOWN", async () => {
    const f = await recoveryFixture(); const raw = f.d.request;
    f.d.request = (async (u, o) => { const response = await raw(u, o); if (o?.method === "POST") f.setWrite(true); return response; }) as typeof fetch;
    assert.equal(await f.run(), false); assert.equal(f.counters.item, 1); assert.equal(f.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(f.counters.merge, 0);
    f.setWrite(false); f.setResponse("lookup-error"); assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive), false); assert.equal(f.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN");
    f.setResponse("normal"); await f.controller.readSnapshot(f.s.f.current, f.d); assert.equal(f.entry().snapshot?.items.length, 1); assert.equal(f.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(f.counters.post, 1);
    assert.equal(await f.controller.recover(f.s.f.current, f.d, f.receive), true); assert.equal(f.counters.post, 1); assert.equal(f.counters.merge, 1);
});

test("R28 pre-dispatch lifecycle drift and v2 projection/payload mismatch fail closed", async () => {
    const f = await recoveryFixture(); const running = f.controller.place(f.s.f.current, f.s.f.archive, f.s.f.c, f.d, async () => { f.controller.dispose(); return true; }, f.receive);
    assert.equal(await running, false); assert.equal(f.counters.post, 0);
    const mismatch = await recoveryFixture(); await mismatch.run(); const p = structuredClone(mismatch.s.f.current().node.metadata!.hnLocalSequencePlacementV2!);
    p.command.preparedFrozenHash = "e".repeat(64); p.owner.preparedFrozenHash = p.command.preparedFrozenHash; p.receipt.owner.preparedFrozenHash = p.command.preparedFrozenHash;
    mismatch.s.f.setTarget({ ...mismatch.s.f.current(), node: { ...mismatch.s.f.current().node, metadata: { ...mismatch.s.f.current().node.metadata, hnLocalSequencePlacementV2: p } } });
    const reload = new HNLocalSequencePlacementController(mismatch.s.controller, () => {}, true); await reload.inspect(mismatch.s.f.current, mismatch.s.f.archive, mismatch.s.f.c, mismatch.d); assert.equal(reload.entry(mismatch.s.f.project, mismatch.s.f.nodeId).phase, "ERROR"); assert.equal(mismatch.counters.post, 1);
});

test("R28 simultaneous v1/v2 UNKNOWN permits read recovery without releasing v1", async () => {
    const f = await recoveryFixture(); f.setResponse("loss"); await f.run();
    const v1 = { version: 1, hnProjectId: f.s.owner.hnProjectId, sequenceId: "main", revisionId: crypto.randomUUID(), entries: [{ owner: f.s.owner, selectionIntentId: f.s.entry().receipt!.intentId, placementIntentId: crypto.randomUUID(), observedAt: new Date().toISOString(), state: "UNKNOWN" }] }; f.v1values.set(placementJournalKey(f.s.owner.hnProjectId), v1);
    const reload = new HNLocalSequencePlacementController(f.s.controller, () => {}, true); await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d); assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).legacyUnknown, true);
    assert.equal(await reload.recover(f.s.f.current, f.d, f.receive, f.confirm), false); assert.equal(await reload.recover(f.s.f.current, f.d, f.receive), true);
    await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d); assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(f.counters.post, 1); assert.deepEqual(f.v1values.get(placementJournalKey(f.s.owner.hnProjectId)), v1);
});

test("R28 snapshot must be complete, unique, ordered and bounded", async () => {
    const f = await recoveryFixture(); await f.run(); const snap = await readMainSequence(f.s.owner.hnProjectId, f.d.request); assert.equal(snap.itemsComplete, true); assert.equal(snap.items.length, 1);
    for (const bad of [{ ...snap, itemsComplete: false }, { ...snap, items: [snap.items[0], snap.items[0]] }, { ...snap, items: Array(257).fill(snap.items[0]) }, { ...snap, extra: true }]) {
        const request = (async (u, o) => u === "/api/hn/local-endpoint" ? f.d.request(u, o) : Response.json({ code: 0, data: bad, msg: "ok" })) as typeof fetch;
        await assert.rejects(() => readMainSequence(f.s.owner.hnProjectId, request));
    }
});
test("R28 lost journal strict v2 projection keeps key and requires exact server lookup", async () => {
    const f = await recoveryFixture(); await f.run(); const projection = structuredClone(f.s.f.current().node.metadata!.hnLocalSequencePlacementV2!); f.values.clear();
    const reload = new HNLocalSequencePlacementController(f.s.controller, () => {}, true); await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d); assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(f.counters.lookup, 0);
    assert.equal(await reload.recover(f.s.f.current, f.d, f.receive), true); assert.equal(f.counters.post, 1); assert.equal(f.s.f.current().node.metadata!.hnLocalSequencePlacementV2!.command.placementIntentId, projection.command.placementIntentId);
    const lost = await recoveryFixture(); assert.equal(lost.entry().receiptV2, undefined); assert.equal(await lost.controller.recover(lost.s.f.current, lost.d, lost.receive), false); assert.equal(lost.counters.lookup, 0);
});
test("R28 v1 UNKNOWN never migrates/sends old UUID; v1/v2 barriers compose", async () => {
    const f = await recoveryFixture(); const v1 = { version: 1, hnProjectId: f.s.owner.hnProjectId, sequenceId: "main", revisionId: crypto.randomUUID(), entries: [{ owner: f.s.owner, selectionIntentId: f.s.entry().receipt!.intentId, placementIntentId: crypto.randomUUID(), observedAt: new Date().toISOString(), state: "UNKNOWN" }] }; f.v1values.set(placementJournalKey(f.s.owner.hnProjectId), v1);
    const reload = new HNLocalSequencePlacementController(f.s.controller, () => {}, true); await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d); assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).recoverable, false); assert.equal(await reload.place(f.s.f.current, f.s.f.archive, f.s.f.c, f.d, f.confirm, f.receive), false); assert.equal(await reload.recover(f.s.f.current, f.d, f.receive, f.confirm), false); assert.equal(f.counters.post, 0); assert.equal(f.counters.lookup, 0); assert.deepEqual(f.v1values.get(placementJournalKey(f.s.owner.hnProjectId)), v1);
    for (const op of ["archive", "candidate", "selection", "placement"] as const) assert.equal(f.s.controller.coordinator.acquire(f.s.f.current(), op, f.s.owner), undefined);
});

test("R28 v1 known-success keeps its own receipt and blocks fresh v2", async () => {
    const f = await recoveryFixture(), placementIntentId = crypto.randomUUID(), selectionIntentId = f.s.entry().receipt!.intentId, observedAt = new Date().toISOString();
    const receipt = { version: 1, ...f.s.owner, sequenceId: "main", sequenceItemId: crypto.randomUUID(), orderIndex: 0, placementIntentId, selectionIntentId, observedAt };
    f.v1values.set(placementJournalKey(f.s.owner.hnProjectId), { version: 1, hnProjectId: f.s.owner.hnProjectId, sequenceId: "main", revisionId: crypto.randomUUID(), entries: [{ owner: f.s.owner, placementIntentId, selectionIntentId, observedAt, state: "PLACED", receipt }] });
    const reload = new HNLocalSequencePlacementController(f.s.controller, () => {}, true); await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d);
    assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).receiptV2, undefined); assert.deepEqual(reload.entry(f.s.f.project, f.s.f.nodeId).receipt, receipt);
    assert.equal(await reload.place(f.s.f.current, f.s.f.archive, f.s.f.c, f.d, f.confirm, f.receive), false); assert.equal(f.counters.post, 0);
});

test("R28 recovery cannot discard another cached command history after journal drift", async () => {
    const f = await recoveryFixture(); f.setResponse("loss"); await f.run(); const key = placementV2Key(f.s.owner.hnProjectId), ledger = structuredClone(f.values.get(key) as PlacementV2Ledger);
    const other = structuredClone(ledger.entries[0]); other.command.placementIntentId = crypto.randomUUID(); other.state = "UNKNOWN"; ledger.entries.push(other); ledger.revisionId = crypto.randomUUID(); f.values.set(key, ledger);
    const reload = new HNLocalSequencePlacementController(f.s.controller, () => {}, true); await reload.inspect(f.s.f.current, f.s.f.archive, f.s.f.c, f.d);
    f.values.set(key, { ...ledger, entries: [ledger.entries[0]], revisionId: crypto.randomUUID() });
    assert.equal(await reload.recover(f.s.f.current, f.d, f.receive), false); assert.equal(f.counters.lookup, 0); assert.equal(f.counters.post, 1); assert.equal(reload.entry(f.s.f.project, f.s.f.nodeId).phase, "PLACEMENT_OUTCOME_UNKNOWN");
});
test("R28 first-await interlocks and continuation synchronous guard", async () => {
    const f = await recoveryFixture(); let release!: (v: boolean) => void; const wait = new Promise<boolean>((r) => release = r); const running = f.controller.place(f.s.f.current, f.s.f.archive, f.s.f.c, f.d, () => wait, f.receive);
    for (const op of ["archive", "candidate", "selection", "placement"] as const) assert.equal(f.s.controller.coordinator.acquire(f.s.f.current(), op, f.s.owner), undefined); assert.equal(await f.run(), false); release(true); assert.equal(await running, true);
    const unknown = await recoveryFixture(); unknown.setResponse("loss"); await unknown.run(); let finish!: (v: boolean) => void; const pending = new Promise<boolean>((r) => finish = r); const continuation = unknown.controller.recover(unknown.s.f.current, unknown.d, unknown.receive, () => pending); assert.equal(await unknown.controller.recover(unknown.s.f.current, unknown.d, unknown.receive, unknown.confirm), false); finish(false); await continuation; assert.equal(unknown.counters.post, 1);
});
test("R28 strict command receipt excludes extra/foreign/unsafe response facts", async () => {
    const f = await recoveryFixture(); await f.run(); const r = f.server.values().next().value!, p = f.bodies[0];
    for (const bad of [{ ...r, extra: true }, { ...r, owner: { ...r.owner, resultId: "foreign" } }, { ...r, originalItem: { ...r.originalItem, candidateId: "foreign" } }, { ...r, outcome: "COMMITTED", errorClass: "OWNERSHIP_REJECTED" }]) assert.equal(validCommandReceipt(bad, f.s.owner.hnProjectId, p), false);
});
test("R28 record UI semantic counters", async () => { if (process.env.HN_R28_UI_EVIDENCE) await writeFile(process.env.HN_R28_UI_EVIDENCE, JSON.stringify({ runtime: "real controller + injected storage/request; no DOM/IndexedDB certification", scenarios: evidence }, null, 2)); });
