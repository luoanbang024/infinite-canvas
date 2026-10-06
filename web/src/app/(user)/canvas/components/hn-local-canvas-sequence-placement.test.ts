import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { writeFile } from "node:fs/promises";
import { selectionFixture } from "./hn-local-canvas-selection.test";
import { HNLocalSelectionController } from "./hn-local-canvas-selection";
import { HNLocalSequencePlacementController, hnPlacementRequest, validPlacementItem, mergeHNLocalSequencePlacementReceipt } from "./hn-local-canvas-sequence-placement";
import { placementJournalKey, type PlacementLedger, type PlacementJournal } from "./hn-local-sequence-placement-journal";
import type { HNLocalSequencePlacementReceipt } from "../types";

const evidence: object[] = [];
export async function placementFixture() {
    const s = await selectionFixture(); await s.run();
    const controller = new HNLocalSequencePlacementController(s.controller), values = new Map<string, unknown>(), counts = { post: 0, get: 0, writes: 0, confirmation: 0, merge: 0 };
    let getHook: ((k: string) => Promise<unknown>) | undefined, setHook: ((k: string, v: PlacementLedger) => Promise<unknown>) | undefined, response: (() => Promise<Response>) | undefined;
    const journal: PlacementJournal = { getItem: async (k) => getHook ? getHook(k) : structuredClone(values.get(k) ?? null), setItem: async (k, v) => { counts.writes++; if (setHook) return setHook(k, v); values.set(k, structuredClone(v)); return v; } };
    const item = () => ({ sequenceItemId: crypto.randomUUID(), projectId: s.owner.hnProjectId, sequenceId: "main", orderIndex: 7, shotId: s.owner.shotId, candidateId: s.f.current().node.metadata!.hnLocalCandidate!.candidateId, resultId: s.f.current().node.metadata!.hnLocalCandidate!.resultId, createdAt: "2026-10-06T00:00:00.123456789Z", updatedAt: "2026-10-06T00:00:00Z" });
    const request = (async (u, o) => { assert.equal(o?.credentials, "omit"); assert.equal(o?.redirect, "error"); assert.ok(o?.signal);
        if (u === "/api/hn/local-endpoint") { counts.get++; return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); }
        assert.equal(String(u), `http://127.0.0.1:8085/api/hn/projects/${s.owner.hnProjectId}/sequences/main/items`); assert.equal(o?.body, JSON.stringify({ candidateId: s.f.current().node.metadata!.hnLocalCandidate!.candidateId }));
        counts.post++; const ledger = values.get(placementJournalKey(s.owner.hnProjectId)) as PlacementLedger; assert.equal(ledger.entries.at(-1)?.state, "PLACING");
        return response ? response() : Response.json({ code: 0, data: item(), msg: "ok" });
    }) as typeof fetch;
    const d = { ...s.d, request, placementJournal: journal };
    const entry = () => controller.entry(s.f.project, s.f.nodeId);
    const receive = (r: HNLocalSequencePlacementReceipt, t: ReturnType<typeof s.f.current>) => { counts.merge++; const v = values.get(placementJournalKey(r.hnProjectId)) as PlacementLedger; assert.equal(v.entries.at(-1)?.state, "PLACED"); s.f.setTarget({ ...s.f.current(), node: mergeHNLocalSequencePlacementReceipt([s.f.current().node], r, s.f.project, t)[0] }); };
    const confirm = async () => { counts.confirmation++; return true; };
    const run = (confirmation = confirm, deps = d, receiver = receive) => controller.place(s.f.current, s.f.archive, s.f.c, deps, confirmation, receiver);
    const inspect = () => controller.inspect(s.f.current, s.f.archive, s.f.c, d);
    await inspect();
    return { s, controller, journal, values, counts, d, run, inspect, entry, receive, item, confirm, setGet: (h?: typeof getHook) => getHook = h, setSet: (h?: typeof setHook) => setHook = h, setResponse: (h?: typeof response) => response = h };
}
test("R26 explicit append seals before merge, duplicate/new intent/reload are blocked", async () => {
    const p = await placementFixture(); assert.equal(p.entry().phase, "NOT_CONFIRMED_PLACED"); assert.equal(p.counts.post, 0); assert.equal(p.counts.writes, 0);
    assert.equal(await p.run(async () => false), false); assert.equal(p.counts.get, 0); assert.equal(p.counts.writes, 0);
    const before = structuredClone(p.s.f.current()); assert.equal(await p.run(), true); const receipt = p.entry().receipt!;
    assert.equal(Object.keys(receipt).length, 17); assert.equal(p.counts.post, 1); assert.equal(p.counts.merge, 1);
    assert.deepEqual({ ...p.s.f.current().node.metadata, hnLocalSequencePlacement: undefined }, { ...before.node.metadata, hnLocalSequencePlacement: undefined });
    assert.equal(await p.run(), false); assert.equal(p.counts.post, 1);
    delete p.s.f.current().node.metadata!.hnLocalSequencePlacement; await p.s.run(); assert.equal(await p.run(), false); assert.equal(p.counts.post, 1);
    const reload = new HNLocalSequencePlacementController(p.s.controller); await reload.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, p.d); assert.equal(reload.entry(p.s.f.project, p.s.f.nodeId).phase, "PLACEMENT_RELOADED_UNVERIFIED");
    assert.equal(await reload.place(p.s.f.current, p.s.f.archive, p.s.f.c, p.d, p.confirm, p.receive), false); assert.equal(p.counts.post, 1); p.s.unchanged();
    evidence.push({ scenario: "durable-success-and-duplicates", ...p.counts, reload: "PLACEMENT_RELOADED_UNVERIFIED", providerCalls: 0 });
});
test("R26 metadata-only capture, selection reload preserved and after-ack owner drift seals history", async () => {
    const p = await placementFixture(); const metadata = p.s.f.current().node.metadata!;
    for (const key of ["content", "storageKey"]) Object.defineProperty(metadata, key, { configurable: true, get() { assert.fail("current media access"); } });
    p.s.entry().phase = "SELECTION_RELOADED_UNVERIFIED"; p.s.f.entry().phase = "CANDIDATE_RELOADED_UNVERIFIED"; p.s.f.archive.entry(p.s.f.project, p.s.f.nodeId).phase = "RELOADED_UNVERIFIED";
    assert.equal(await p.run(p.confirm, p.d, () => {}), true); assert.equal(p.s.entry().phase, "SELECTION_RELOADED_UNVERIFIED"); assert.equal(p.s.f.entry().phase, "CANDIDATE_RELOADED_UNVERIFIED"); p.s.unchanged();
    const q = await placementFixture(); q.setResponse(async () => { const i = q.item(); q.s.f.current().node.metadata!.hnLocalPrepared!.generationId = "new-owner"; return Response.json({ code: 0, data: i, msg: "ok" }); });
    assert.equal(await q.run(), true); assert.equal(q.counts.merge, 0); assert.equal((q.values.values().next().value as PlacementLedger).entries[0].state, "PLACED");
});
test("R26 new Candidate after known success keeps old entry and appends explicitly", async () => {
    const p = await placementFixture(); assert.equal(await p.run(), true);
    const c = p.s.f.current().node.metadata!.hnLocalCandidate!; const newCandidate = { ...c, candidateId: "candidate-two" }; p.s.f.current().node.metadata!.hnLocalCandidate = newCandidate; p.s.f.entry().receipt = newCandidate;
    const selection = { ...p.s.entry().receipt!, candidateId: "candidate-two", intentId: crypto.randomUUID() };
    p.s.f.current().node.metadata!.hnLocalSelection = selection; p.s.entry().receipt = selection;
    await p.inspect(); assert.equal(p.entry().phase, "NOT_CONFIRMED_PLACED"); assert.equal(await p.run(), true);
    assert.equal((p.values.values().next().value as PlacementLedger).entries.length, 2); assert.equal(p.counts.post, 2);
});
test("R26 durable journal errors/read-back corruption pre-gate zero POST", async () => {
    for (const mode of ["set-throw", "get-throw", "null", "revision", "attempt", "owner", "extra"] as const) {
        const p = await placementFixture();
        if (mode === "set-throw") p.setSet(async () => { throw Error("synthetic"); });
        else p.setGet(async (k) => { const v = structuredClone(p.values.get(k) ?? null) as PlacementLedger | null; if (!v) return null; if (mode === "get-throw") throw Error("synthetic"); if (mode === "null") return null; if (mode === "revision") v.revisionId = crypto.randomUUID(); if (mode === "attempt") v.entries[0].placementIntentId = crypto.randomUUID(); if (mode === "owner") v.entries[0].owner.resultId = "foreign"; if (mode === "extra") (v as unknown as Record<string, unknown>).extra = true; return v; });
        assert.equal(await p.run(), false); assert.equal(p.counts.post, 0, mode); assert.equal(p.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN", mode); assert.equal(p.s.controller.coordinator.blocked(p.s.f.current(), "archive"), true);
        evidence.push({ scenario: "journal-pre-gate-" + mode, post: 0, state: p.entry().phase });
    }
    const p = await placementFixture(); p.setSet(async (k, v) => { if (v.entries.at(-1)?.state === "PLACED") throw Error("synthetic final seal loss"); p.values.set(k, structuredClone(v)); return v; });
    assert.equal(await p.run(), false); assert.equal(p.counts.post, 1); assert.equal(p.counts.merge, 0); assert.equal(p.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(await p.run(), false); assert.equal(p.counts.post, 1);
});
test("R26 valid known rejection seals REJECTED; opaque failures stay UNKNOWN", async () => {
    for (const status of [400, 403, 409, 503]) {
        const p = await placementFixture(), msgs: Record<number, string> = { 400: "HN 剪辑输入无效", 403: "HN 剪辑需要显式本机请求标记", 409: "EDITORIAL_OWNERSHIP_CONFLICT", 503: "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT" };
        p.setResponse(async () => Response.json({ code: 1, data: null, msg: msgs[status] }, { status })); assert.equal(await p.run(), false); assert.equal((p.values.values().next().value as PlacementLedger).entries[0].state, "REJECTED");
        p.setResponse(); assert.equal(await p.run(), true); assert.equal(p.counts.post, 2);
    }
    for (const response of [async () => Response.json({ code: 1, data: null, msg: "SEQUENCE_IDENTITY_CONFLICT" }, { status: 409 }), async () => Response.json({ code: 1, data: null, msg: "HN 剪辑失败，请检查本地项目完整性" }, { status: 500 }), async () => new Response("<html>synthetic</html>"), async () => Response.json(null), async () => { throw Error("synthetic timeout/drop"); }]) {
        const p = await placementFixture(); p.setResponse(response); assert.equal(await p.run(), false); assert.equal(p.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(await p.run(), false); assert.equal(p.counts.post, 1);
    }
});
test("R26 strict response closure/timestamps/owner and request forwarding policy", async () => {
    const p = await placementFixture(), i = p.item(); assert.equal(validPlacementItem(i, p.s.owner), true);
    for (const v of [{ ...i, extra: true }, { ...i, sequenceItemId: "CON" }, { ...i, candidateId: "foreign" }, { ...i, orderIndex: -1 }, { ...i, orderIndex: 1.2 }, { ...i, createdAt: "2026-02-31T00:00:00Z" }, { ...i, createdAt: "2026-10-06T00:00:00.1234567890Z" }]) assert.equal(validPlacementItem(v, p.s.owner), false);
    for (const key of Object.keys(i)) { const v: Record<string, unknown> = { ...i }; delete v[key]; assert.equal(validPlacementItem(v, p.s.owner), false); }
    let calls = 0; const request = hnPlacementRequest(p.s.owner, (async () => { calls++; return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); }) as typeof fetch, () => {}, async () => {}, () => {});
    for (const url of ["https://synthetic.invalid", "/api/account", "/api/hn/projects/x/sequences/main/items"]) await assert.rejects(request(url, { method: "POST" })); assert.equal(calls, 0);
    await request("/api/hn/local-endpoint"); await assert.rejects(request("http://127.0.0.1:8085/api/hn/projects/x/sequences/main/items", { method: "POST", body: "{}" })); assert.equal(calls, 1);
});
test("R26 first-await locks all twelve directions, same Sequence contention and stale release", async () => {
    const p = await placementFixture(); let resolve!: (b: boolean) => void; const waiting = new Promise<boolean>((r) => resolve = r);
    const command = p.run(() => waiting);
    for (const op of ["archive", "candidate"] as const) assert.equal(await p.s.controller.runOperation(op, p.s.f.current, p.s.f.archive, p.s.f.c, async () => assert.fail("placement lock bypass")), false);
    assert.equal(await p.s.run(), false); assert.equal(await p.run(), false);
    resolve(false); await command;
    for (const op of ["archive", "candidate", "selection"] as const) { const token = p.s.controller.coordinator.acquire(p.s.f.current(), op)!; assert.ok(token); assert.equal(await p.run(), false); p.s.controller.coordinator.release(token); }
    const t = p.s.controller.coordinator.acquire(p.s.f.current(), "placement")!; p.s.controller.coordinator.release(t); const newer = p.s.controller.coordinator.acquire(p.s.f.current(), "archive")!; p.s.controller.coordinator.release(t); assert.equal(p.s.controller.coordinator.blocked(p.s.f.current(), "candidate"), true); p.s.controller.coordinator.release(newer);
    evidence.push({ scenario: "first-await-interlock", preservedR24Directions: 6, addedDirections: 6, post: 0 });
});
test("R26 actual localhost response loss and redirect leave durable main barrier", async () => {
    for (const redirect of [false, true]) {
        const p = await placementFixture(); let posts = 0, destinations = 0;
        const destination = createServer((_req, res) => { destinations++; res.end("unexpected"); }); await new Promise<void>((r) => destination.listen(0, "127.0.0.1", r));
        const da = destination.address(); assert.ok(da && typeof da !== "string");
        const server = createServer((req, res) => { posts++; if (redirect) { res.writeHead(307, { Location: `http://127.0.0.1:${da.port}` }); res.end(); } else req.socket.destroy(); }); await new Promise<void>((r) => server.listen(0, "127.0.0.1", r)); const a = server.address(); assert.ok(a && typeof a !== "string");
        const d = { ...p.d, request: (async (u, o) => u === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: `http://127.0.0.1:${a.port}` }, msg: "ok" }) : fetch(u, o)) as typeof fetch };
        try {
            assert.equal(await p.run(p.confirm, d), false); assert.equal(posts, 1); assert.equal(destinations, 0); await p.inspect(); assert.equal(await p.run(), false);
            const reloadSelection = new HNLocalSelectionController(), reload = new HNLocalSequencePlacementController(reloadSelection); await reload.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, d); assert.equal(reload.entry(p.s.f.project, p.s.f.nodeId).phase, "PLACEMENT_OUTCOME_UNKNOWN");
            for (const op of ["archive", "candidate", "selection"] as const) assert.equal(reloadSelection.coordinator.acquire(p.s.f.current(), op, p.s.owner), undefined);
            const other = structuredClone(p.s.f.current()); other.node.metadata!.hnLocalPrepared!.shotId = "another-shot";
            assert.equal(reloadSelection.coordinator.acquire(other, "placement"), undefined); const token = reloadSelection.coordinator.acquire(other, "archive"); assert.ok(token); reloadSelection.coordinator.release(token!);
            assert.equal(posts, 1); evidence.push({ scenario: redirect ? "actual-redirect" : "actual-localhost-drop", serverObservedPOST: posts, redirectDestinationRequests: destinations, additionalPOST: 0 });
        } finally { server.closeAllConnections(); destination.closeAllConnections(); await Promise.all([new Promise<void>((r) => server.close(() => r())), new Promise<void>((r) => destination.close(() => r()))]); }
    }
});
test("R26 actual production handler bridge committed loss", { skip: !process.env.HN_R26_BRIDGE_FIXTURE }, async () => {
    const f = JSON.parse(process.env.HN_R26_BRIDGE_FIXTURE!), p = await placementFixture();
    p.s.f.setTarget({ ...p.s.f.current(), node: { ...p.s.f.current().node, metadata: { hnLocalPrepared: f.prepared, hnLocalArchive: f.archive, hnLocalCandidate: f.candidate, hnLocalSelection: f.selection } } });
    p.s.f.store.set(JSON.stringify([p.s.f.project, p.s.f.nodeId]), f.archive); p.s.f.entry().receipt = f.candidate; p.s.entry().receipt = f.selection;
    const controller = new HNLocalSequencePlacementController(p.s.controller), d = { ...p.d, request: (async (u, o) => fetch(u === "/api/hn/local-endpoint" ? f.base + "/api/hn/local-endpoint" : u, o)) as typeof fetch };
    await controller.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, d);
    assert.equal(await controller.place(p.s.f.current, p.s.f.archive, p.s.f.c, d, p.confirm, p.receive), false); assert.equal(controller.entry(p.s.f.project, p.s.f.nodeId).phase, "PLACEMENT_OUTCOME_UNKNOWN");
    await controller.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, d); assert.equal(await controller.place(p.s.f.current, p.s.f.archive, p.s.f.c, d, p.confirm, p.receive), false);
    const restart = new HNLocalSequencePlacementController(p.s.controller); await restart.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, d); assert.equal(restart.entry(p.s.f.project, p.s.f.nodeId).phase, "PLACEMENT_OUTCOME_UNKNOWN");
    assert.equal(await restart.place(p.s.f.current, p.s.f.archive, p.s.f.c, d, p.confirm, p.receive), false);
    if (process.env.HN_R26_BRIDGE_OUTPUT) await writeFile(process.env.HN_R26_BRIDGE_OUTPUT, JSON.stringify({ unknown: true, automaticRetry: 0, reloadPost: 0, providerCalls: 0, blobReads: 0 }));
});
test("R26 owner/lifecycle drift at confirmation, discovery and pending read-back sends zero POST", async () => {
    for (const boundary of ["confirm", "discovery", "set", "readback", "dispose"] as const) {
        const p = await placementFixture(); const drift = () => p.s.f.current().node.metadata!.hnLocalSelection!.intentId = crypto.randomUUID();
        const d = { ...p.d, request: (async (u, o) => { if (u === "/api/hn/local-endpoint") { if (boundary === "discovery") drift(); return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); } return p.d.request(u, o); }) as typeof fetch };
        if (boundary === "set") p.setSet(async (k, v) => { p.values.set(k, structuredClone(v)); drift(); return v; });
        if (boundary === "readback") p.setGet(async k => { const v = structuredClone(p.values.get(k) ?? null); if (v) drift(); return v; });
        const result = await p.run(async () => { if (boundary === "confirm") drift(); if (boundary === "dispose") p.controller.dispose(); return true; }, d);
        assert.equal(result, false); assert.equal(p.counts.post, 0, boundary); assert.equal(p.counts.merge, 0);
    }
    evidence.push({ scenario: "prepost-owner-epoch-await-drift", boundaries: ["confirm", "discovery", "set", "readback", "dispose"], post: 0 });
});
test("R26 Sequence cross-Shot contention, synchronous first-read/write lock and unknown callback barrier", async () => {
    for (const boundary of ["read", "write"] as const) {
        const p = await placementFixture(); let resolve!: () => void; const waiting = new Promise<void>(r => resolve = r); let calls = 0;
        if (boundary === "read") p.setGet(async k => { if (calls++ === 0) await waiting; return structuredClone(p.values.get(k) ?? null); });
        else p.setSet(async (k, v) => { if (calls++ === 0) await waiting; p.values.set(k, structuredClone(v)); return v; });
        const command = p.run();
        // Wait until the instrumented boundary; the synchronous token already prevents all same-Shot writes.
        if (boundary === "write") for (let i = 0; i < 30 && calls === 0; i++) await new Promise(r => setTimeout(r, 1));
        for (const op of ["archive", "candidate"] as const) assert.equal(await p.s.controller.runOperation(op, p.s.f.current, p.s.f.archive, p.s.f.c, async () => assert.fail("pending journal bypass")), false);
        assert.equal(await p.s.run(), false);
        const other = structuredClone(p.s.f.current()); other.node.id = "other_node"; other.node.metadata!.hnLocalPrepared!.sourceNodeId = "other_node"; other.node.metadata!.hnLocalPrepared!.shotId = "other-shot";
        assert.equal(await p.controller.place(() => other, p.s.f.archive, p.s.f.c, p.d, p.confirm, p.receive), false); assert.equal(p.counts.post, 0);
        const independent = p.s.controller.coordinator.acquire(other, "archive"); assert.ok(independent); p.s.controller.coordinator.release(independent!);
        resolve(); assert.equal(await command, true);
    }
    const p = await placementFixture(); p.setResponse(async () => { throw Error("synthetic loss"); }); assert.equal(await p.run(), false);
    for (const op of ["archive", "candidate"] as const) assert.equal(await p.s.controller.runOperation(op, p.s.f.current, p.s.f.archive, p.s.f.c, async () => assert.fail("UNKNOWN actual callback bypass")), false);
    assert.equal(await p.s.run(), false); assert.equal(p.counts.post, 1);
    const q = await placementFixture(); const token = q.s.controller.coordinator.acquire(q.s.f.current(), "selection", q.s.owner)!; q.s.controller.coordinator.markUnknown(token, q.s.owner); q.s.controller.coordinator.release(token); assert.equal(await q.run(), false); assert.equal(q.counts.post, 0);
    evidence.push({ scenario: "first-journal-boundaries-and-unknown-callbacks", directions: 12, crossShotPlacementBlocked: true, otherShotArchiveAllowed: true, selectionUnknownBlocksPlacement: true });
});
test("R26 all dispatched invalid success shapes are UNKNOWN, credentials never forwarded", async () => {
    for (const field of ["extra", "createdAt", "sequenceItemId", "projectId", "sequenceId", "orderIndex", "shotId", "candidateId", "resultId"] as const) {
        const p = await placementFixture(); p.setResponse(async () => { const item: Record<string, unknown> = p.item(); item[field] = field === "orderIndex" ? -1 : "foreign-synthetic"; return Response.json({ code: 0, data: item, msg: "ok" }); });
        assert.equal(await p.run(), false); assert.equal(p.entry().phase, "PLACEMENT_OUTCOME_UNKNOWN"); assert.equal(p.counts.post, 1); assert.equal(p.counts.merge, 0);
    }
    const p = await placementFixture(); let forwarded = 0; const request = hnPlacementRequest(p.s.owner, (async () => { forwarded++; return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); }) as typeof fetch, () => {}, async () => {}, () => {});
    await assert.rejects(request("/api/hn/local-endpoint", { credentials: "include" })); await assert.rejects(request("/api/hn/local-endpoint", { headers: { Authorization: "synthetic-only" } })); assert.equal(forwarded, 0);
});
test("R26 record semantic evidence", async () => { if (process.env.HN_R26_EVIDENCE) await writeFile(process.env.HN_R26_EVIDENCE, JSON.stringify({ runtime: "page controller + memory journal + actual localhost; real DOM/IndexedDB NOT_CLAIMED", scenarios: evidence }, null, 2)); });
