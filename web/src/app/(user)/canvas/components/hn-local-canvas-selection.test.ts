import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { writeFile } from "node:fs/promises";
import { candidateFixture } from "./hn-local-canvas-candidate.test";
import { HNLocalCandidateController } from "./hn-local-canvas-candidate";
import { HNLocalArchiveController } from "./hn-local-canvas-archive";
import { HNLocalSelectionController, HNShotOperationCoordinator, hnSelectionRequest, validHNLocalSelectionReceipt, mergeHNLocalSelectionReceipt, type HNSelectionOwner } from "./hn-local-canvas-selection";
import type { HNLocalSelectionReceipt } from "../types";

const evidence: object[] = [];
export async function selectionFixture() {
    const f = await candidateFixture(); await f.run();
    const baseline = structuredClone(f.counts), c = f.entry().receipt!;
    const { version: _v, availabilityStatus: _a, observedAt: _t, ...owner } = c;
    const controller = new HNLocalSelectionController();
    const counts = { discovery: 0, post: 0, confirmation: 0, merge: 0, forbidden: 0 };
    let response: (() => Promise<Response>) | undefined, discovery: (() => Response) | undefined;
    const request = (async (url, options) => {
        assert.equal(options?.credentials, "omit"); assert.equal(options?.redirect, "error"); assert.ok(options?.signal);
        if (url === "/api/hn/local-endpoint") { counts.discovery++; assert.equal(options?.cache, "no-store"); return discovery ? discovery() : Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); }
        if (String(url) !== `http://127.0.0.1:8085/api/hn/projects/${f.prepared.hnProjectId}/shots/shot/candidates/candidate/select`) { counts.forbidden++; assert.fail("unexpected selection boundary"); }
        counts.post++; assert.equal(options?.method, "POST"); assert.equal(options?.body, "{}"); assert.equal(new Headers(options?.headers).get("Content-Type"), "application/json"); assert.equal(new Headers(options?.headers).get("X-HN-Local-Request"), "1");
        return response ? response() : Response.json({ code: 0, data: { shotId: "shot", selectedCandidateId: "candidate" }, msg: "ok" });
    }) as typeof fetch;
    const d = { journal: f.journal, request };
    const receive = (receipt: HNLocalSelectionReceipt, target: ReturnType<typeof f.current>) => { counts.merge++; f.setTarget({ ...f.current(), node: mergeHNLocalSelectionReceipt([f.current().node], receipt, f.project, target)[0] }); };
    const confirm = async () => { counts.confirmation++; return true; };
    const run = (confirmation = confirm, deps = d) => controller.select(f.current, f.archive, f.c, deps, confirmation, receive);
    const inspect = () => controller.inspect(f.current, f.archive, f.c, d);
    const entry = () => controller.entry(f.project, f.nodeId);
    const unchanged = () => assert.deepEqual(f.counts, baseline);
    return { f, owner: owner as HNSelectionOwner, controller, counts, d, receive, confirm, run, inspect, entry, unchanged, setResponse: (r?: () => Promise<Response>) => { response = r; }, setDiscovery: (r?: () => Response) => { discovery = r; } };
}

test("R24 explicit selection, repeated new mutation, exact receipt and metadata-only merge", async () => {
    const s = await selectionFixture(), original = structuredClone(s.f.current().node);
    await s.inspect(); assert.equal(s.entry().phase, "NOT_CONFIRMED_SELECTED"); assert.equal(s.counts.post, 0);
    assert.equal(await s.run(async () => false), false); assert.equal(s.counts.post, 0);
    assert.equal(await s.run(), true); assert.equal(s.entry().phase, "SELECTED_CURRENT_SESSION"); const first = s.entry().receipt!;
    assert.equal(Object.keys(first).length, 13); assert.equal(validHNLocalSelectionReceipt(first, s.owner), true);
    assert.equal(await s.run(), true); assert.notEqual(s.entry().receipt!.intentId, first.intentId); assert.equal(s.counts.post, 2);
    const merged = s.f.current().node;
    assert.deepEqual({ ...merged.metadata, hnLocalSelection: undefined }, { ...original.metadata, hnLocalSelection: undefined });
    assert.equal(merged.metadata?.status, "success"); s.unchanged();
    for (const v of [null, [], { ...first, version: 2 }, { ...first, intentId: "not-uuid" }, { ...first, observedAt: "2026-02-31T00:00:00.000Z" }, { ...first, extra: "synthetic-private" }]) assert.equal(validHNLocalSelectionReceipt(v, s.owner), false);
    for (const key of Object.keys(first)) { const partial = { ...first } as Record<string, unknown>; delete partial[key]; assert.equal(validHNLocalSelectionReceipt(partial, s.owner), false, key); }
    for (const key of Object.keys(s.owner)) assert.equal(validHNLocalSelectionReceipt({ ...first, [key]: "foreign" }, s.owner), false, key);
    assert.equal(mergeHNLocalSelectionReceipt([original], first, "foreign", s.f.current())[0], original);
    assert.equal(mergeHNLocalSelectionReceipt([original], { ...first, candidateId: "other" }, s.f.project, s.f.current())[0], original);
    evidence.push({ scenario: "explicit-and-repeat", ...s.counts, intentChanges: true, currentBlobReads: 0, forbiddenSideEffects: 0 });
});

test("R24 reload advisory, Candidate-unverified allowed, current media never read", async () => {
    const s = await selectionFixture(); assert.equal(await s.run(), true); const before = s.counts.post;
    const reload = new HNLocalSelectionController();
    s.f.entry().phase = "CANDIDATE_RELOADED_UNVERIFIED"; s.f.archive.entry(s.f.project, s.f.nodeId).phase = "RELOADED_UNVERIFIED";
    const metadata = s.f.current().node.metadata!;
    for (const k of ["content", "storageKey"]) Object.defineProperty(metadata, k, { configurable: true, enumerable: true, get() { assert.fail("Selection accessed current media field"); } });
    await reload.inspect(s.f.current, s.f.archive, s.f.c, s.d); assert.equal(reload.entry(s.f.project, s.f.nodeId).phase, "SELECTION_RELOADED_UNVERIFIED"); assert.equal(s.counts.post, before);
    // The test callback checks only the safe projection; production merge preserves media normally.
    assert.equal(await reload.select(s.f.current, s.f.archive, s.f.c, s.d, s.confirm, () => {}), true);
    assert.equal(s.f.entry().phase, "CANDIDATE_RELOADED_UNVERIFIED"); assert.equal(s.f.archive.entry(s.f.project, s.f.nodeId).phase, "RELOADED_UNVERIFIED"); s.unchanged();
    evidence.push({ scenario: "reload-unverified-and-no-media-access", reloadPost: 0, explicitPost: 1, archivePromotion: "NONE", candidatePromotion: "NONE" });
});

test("R24 every ineligible Candidate/history owner blocks before HTTP", async () => {
    for (const phase of ["CANDIDATE_OUTCOME_UNKNOWN", "ENSURING_CANDIDATE", "NOT_CANDIDATE", "ARCHIVE_NOT_ELIGIBLE", "ERROR"] as const) {
        const s = await selectionFixture(); s.f.entry().phase = phase;
        await s.inspect(); assert.equal(await s.run(), false, phase); assert.equal(s.counts.post, 0); assert.equal(s.counts.discovery, 0); s.unchanged();
    }
    for (const mode of ["missing", "pending", "failed", "foreign", "candidate-owner", "prepared-owner", "extra-selection", "disposed"]) {
        const s = await selectionFixture(), key = JSON.stringify([s.f.project, s.f.nodeId]);
        if (mode === "missing") s.f.store.delete(key);
        if (mode === "pending") { const { resultId, archiveJobId, resultStatus, archiveStatus, sha256, byteLength, mimeType, ...rest } = s.f.receipt; s.f.store.set(key, { ...rest, outcome: "ARCHIVING", knownJob: { resultId, archiveJobId } }); }
        if (mode === "failed") s.f.store.set(key, { ...s.f.receipt, outcome: "ARCHIVE_FAILED_RETRYABLE", resultStatus: "ARCHIVE_FAILED", archiveStatus: "FAILED" });
        if (mode === "foreign") s.f.store.set(key, { ...s.f.receipt, canvasProjectId: "foreign" });
        if (mode === "candidate-owner") s.f.current().node.metadata!.hnLocalCandidate!.resultId = "foreign";
        if (mode === "prepared-owner") s.f.current().node.metadata!.hnLocalPrepared!.sourceNodeId = "foreign";
        if (mode === "extra-selection") s.f.current().node.metadata!.hnLocalSelection = { bad: true } as never;
        if (mode === "disposed") s.controller.dispose();
        assert.equal(await s.run(), false, mode); assert.equal(s.counts.post, 0); assert.equal(s.counts.discovery, 0); s.unchanged();
    }
});

test("R24 all six directions, Shot contention, distinct Shots and node locks before first await", async () => {
    const directions: string[] = [];
    for (const first of ["archive", "candidate", "selection"] as const) for (const second of ["archive", "candidate", "selection"] as const) {
        if (first === second) continue;
        const s = await selectionFixture(); await s.inspect(); let release!: (v: boolean) => void, called = 0;
        const hold = () => new Promise<boolean>((resolve) => { release = resolve; });
        const pending = first === "selection" ? s.run(hold) : s.controller.runOperation(first, s.f.current, s.f.archive, s.f.c, hold);
        assert.equal(s.controller.coordinator.blocked(s.f.current(), second, s.owner), true);
        const attempt = second === "selection" ? s.run() : s.controller.runOperation(second, s.f.current, s.f.archive, s.f.c, async () => { called++; return true; });
        assert.equal(await attempt, false); assert.equal(called, 0); assert.equal(s.counts.post, 0);
        while (!release) await new Promise((r) => setTimeout(r, 1)); release(false); await pending;
        directions.push(first + "->" + second);
    }
    const s = await selectionFixture(), coordinator = new HNShotOperationCoordinator();
    const token = coordinator.acquire(s.f.current(), "selection", s.owner)!; assert.ok(token);
    const otherNode = structuredClone(s.f.current()); otherNode.node.id = "_other"; otherNode.node.metadata!.hnLocalPrepared!.sourceNodeId = "_other";
    assert.equal(coordinator.acquire(otherNode, "archive"), undefined);
    const otherShot = structuredClone(otherNode); otherShot.node.metadata!.hnLocalPrepared!.shotId = "other-shot";
    const independent = coordinator.acquire(otherShot, "candidate"); assert.ok(independent);
    coordinator.release(token); const newer = coordinator.acquire(s.f.current(), "candidate")!; coordinator.release(token); assert.equal(coordinator.blocked(s.f.current(), "archive"), true); coordinator.release(newer); coordinator.release(independent!);
    evidence.push({ scenario: "six-directions-and-shot-lock", directions, pass: directions.length === 6, distinctShotIndependent: true, staleReleaseSafe: true });
});

test("R24 committed response loss has actual server counts, UNKNOWN barrier and explicit same-target mutation", async () => {
    const s = await selectionFixture(); let posts = 0, selected = "", lose = true;
    const server = createServer((req, res) => {
        if (req.method !== "POST" || req.url !== `/api/hn/projects/${s.owner.hnProjectId}/shots/shot/candidates/candidate/select`) { res.writeHead(400).end(); return; }
        posts++; selected = "candidate";
        if (lose) { req.socket.destroy(); return; }
        res.setHeader("Content-Type", "application/json"); res.end(JSON.stringify({ code: 0, data: { shotId: "shot", selectedCandidateId: selected }, msg: "ok" }));
    });
    await new Promise<void>((r) => server.listen(0, "127.0.0.1", r)); const address = server.address(); assert.ok(address && typeof address !== "string");
    const request = (async (url, options) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: `http://127.0.0.1:${address.port}` }, msg: "ok" }) : fetch(url, options)) as typeof fetch;
    try {
        await s.inspect(); assert.equal(await s.run(s.confirm, { ...s.d, request }), false); assert.equal(posts, 1); assert.equal(selected, "candidate"); assert.equal(s.entry().phase, "SELECTION_OUTCOME_UNKNOWN");
        await s.inspect(); await new Promise((r) => setTimeout(r, 15)); assert.equal(posts, 1); assert.equal(s.entry().running, false);
        for (const operation of ["archive", "candidate"] as const) assert.equal(await s.controller.runOperation(operation, s.f.current, s.f.archive, s.f.c, async () => assert.fail("UNKNOWN bypass")), false);
        const oldCandidate = s.f.current().node.metadata!.hnLocalCandidate!; s.f.current().node.metadata!.hnLocalCandidate = { ...oldCandidate, candidateId: "new-target" }; s.f.entry().receipt = s.f.current().node.metadata!.hnLocalCandidate;
        assert.equal(await s.run(s.confirm, { ...s.d, request }), false); assert.equal(posts, 1);
        s.f.current().node.metadata!.hnLocalCandidate = oldCandidate; s.f.entry().receipt = oldCandidate;
        assert.equal(await s.run(async () => false, { ...s.d, request }), false); assert.equal(s.entry().phase, "SELECTION_OUTCOME_UNKNOWN"); assert.equal(posts, 1);
        lose = false; assert.equal(await s.run(s.confirm, { ...s.d, request }), true); assert.equal(posts, 2);
        assert.equal(s.controller.coordinator.blocked(s.f.current(), "archive"), false); s.unchanged();
        evidence.push({ scenario: "server-committed-response-loss", serverObservedFirstPost: 1, automaticAdditionalPost: 0, explicitReselectPost: 1, finalTotalPost: posts, sameShotUnknownBarrier: "PASS", providerBoundCalls: 0 });
    } finally { server.closeAllConnections(); await new Promise<void>((r) => server.close(() => r())); }
});

test("R24 strict rejected/ambiguous envelopes never disclose raw backend text", async () => {
    const cases = [
        [400, { code: 1, data: null, msg: "HN 剪辑标识或输入无效" }, "ERROR"], [403, { code: 1, data: null, msg: "HN 参考冻结不接受此页面来源" }, "ERROR"], [409, { code: 1, data: null, msg: "EDITORIAL_OWNERSHIP_CONFLICT" }, "ERROR"], [503, { code: 1, data: null, msg: "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT" }, "ERROR"],
        [500, { code: 1, data: null, msg: "HN 剪辑失败，请检查本地项目完整性" }, "SELECTION_OUTCOME_UNKNOWN"], [409, { code: 1, data: null, msg: "synthetic-private" }, "SELECTION_OUTCOME_UNKNOWN"], [200, { code: 0, data: { shotId: "shot", selectedCandidateId: "foreign" }, msg: "ok" }, "SELECTION_OUTCOME_UNKNOWN"], [200, { code: 0, data: { shotId: "shot", selectedCandidateId: "candidate", url: "synthetic-private" }, msg: "ok" }, "SELECTION_OUTCOME_UNKNOWN"], [200, { code: 0, data: { shotId: "shot", selectedCandidateId: "candidate" }, msg: "ok", extra: true }, "SELECTION_OUTCOME_UNKNOWN"],
    ] as const;
    for (const [status, body, phase] of cases) {
        const s = await selectionFixture(); s.setResponse(async () => Response.json(body, { status })); assert.equal(await s.run(), false); assert.equal(s.entry().phase, phase); assert.equal(s.counts.post, 1); assert.equal(s.counts.merge, 0); assert.equal(JSON.stringify(s.entry()).includes("synthetic-private"), false);
    }
    for (const mode of ["malformed", "drop", "timeout", "redirect"]) {
        const s = await selectionFixture(); s.setResponse(async () => { if (mode === "malformed") return new Response("synthetic-private"); throw new DOMException("synthetic-private", mode === "timeout" ? "TimeoutError" : "NetworkError"); });
        assert.equal(await s.run(), false); assert.equal(s.entry().phase, "SELECTION_OUTCOME_UNKNOWN"); assert.equal(s.counts.post, 1); await s.inspect(); assert.equal(s.counts.post, 1);
    }
    const s = await selectionFixture(); s.setDiscovery(() => Response.json({ code: 0, data: { url: "https://remote.invalid" }, msg: "ok" })); assert.equal(await s.run(), false); assert.equal(s.counts.post, 0);
});

test("R24 request wrapper rejects duplicate/alternate route/body and preserves Response for helper", async () => {
    const s = await selectionFixture(); let dispatch = 0;
    const wrapper = hnSelectionRequest(s.owner, s.d.request, () => { dispatch++; }, async () => {});
    const discovery = await wrapper("/api/hn/local-endpoint"); assert.equal((await discovery.json()).code, 0);
    const url = `http://127.0.0.1:8085/api/hn/projects/${s.owner.hnProjectId}/shots/shot/candidates/candidate/select`;
    const options = { method: "POST", body: "{}", headers: { "Content-Type": "application/json", "X-HN-Local-Request": "1" } };
    const invalidRequests: [string, RequestInit][] = [["https://remote.invalid", options], [url, { ...options, body: JSON.stringify({ intentId: crypto.randomUUID() }) }], [url, { ...options, headers: {} }]];
    for (const [u, o] of invalidRequests) await assert.rejects(wrapper(u, o));
    const response = await wrapper(url, options); assert.equal((await response.json()).data.selectedCandidateId, "candidate"); assert.equal(dispatch, 1); await assert.rejects(wrapper(url, options)); assert.equal(dispatch, 1);
});

test("R24 rapid duplicates and owner/lifecycle drift before/after POST fail safely", async () => {
    for (const stage of ["confirmation", "before-post", "after-post", "dispose", "deleted"]) {
        const s = await selectionFixture(); let release!: (v: boolean) => void;
        const pending = s.run(() => new Promise<boolean>((r) => { release = r; }));
        assert.equal(s.entry().running, true); assert.equal(await s.run(), false);
        while (!release) await new Promise((r) => setTimeout(r, 1));
        const change = () => { s.f.setTarget({ ...s.f.current(), canvasProjectId: "foreign" }); };
        if (stage === "confirmation") change();
        if (stage === "before-post") s.setDiscovery(() => { change(); return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); });
        if (["after-post", "dispose", "deleted"].includes(stage)) s.setResponse(async () => { if (stage === "dispose") { s.controller.dispose(); s.controller.activate(); } else if (stage === "deleted") s.f.setTarget({ ...s.f.current(), node: { ...s.f.current().node, id: "replacement" } }); else change(); return Response.json({ code: 0, data: { shotId: "shot", selectedCandidateId: "candidate" }, msg: "ok" }); });
        release(true); assert.equal(await pending, false); assert.equal(s.counts.merge, 0); assert.equal(s.counts.post, stage === "confirmation" || stage === "before-post" ? 0 : 1); if (s.counts.post) assert.equal(s.entry().phase, "SELECTION_OUTCOME_UNKNOWN");
    }
});

test("R24 evidence output contains only synthetic safe counters", async () => { if (process.env.HN_R24_EVIDENCE) await writeFile(process.env.HN_R24_EVIDENCE, JSON.stringify({ execution: "R24", evidence, providerBoundCalls: 0, realProviderCalls: 0 }, null, 2)); });

test("R24 native redirect is not followed and pending first journal await is locked", async () => {
    const s = await selectionFixture(); let posts = 0, followed = 0;
    const server = createServer((req, res) => { if (req.url === "/forbidden") { followed++; res.end(); return; } posts++; res.writeHead(302, { Location: "/forbidden" }); res.end(); });
    await new Promise<void>((r) => server.listen(0, "127.0.0.1", r)); const a = server.address(); assert.ok(a && typeof a !== "string");
    const request = (async (url, options) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: `http://127.0.0.1:${a.port}` }, msg: "ok" }) : fetch(url, options)) as typeof fetch;
    try { assert.equal(await s.run(s.confirm, { ...s.d, request }), false); assert.equal(s.entry().phase, "SELECTION_OUTCOME_UNKNOWN"); assert.equal(posts, 1); assert.equal(followed, 0); }
    finally { server.closeAllConnections(); await new Promise<void>((r) => server.close(() => r())); }
    const pending = await selectionFixture(); let release!: () => void;
    pending.f.setRead(() => new Promise<void>((r) => { release = r; }));
    const command = pending.run(async () => false); assert.equal(pending.entry().running, true); assert.equal(await pending.run(), false);
    assert.equal(await pending.controller.runOperation("archive", pending.f.current, pending.f.archive, pending.f.c, async () => assert.fail("first-await bypass")), false);
    while (!release) await new Promise((r) => setTimeout(r, 1));
    pending.f.setRead(); release(); assert.equal(await command, false); assert.equal(pending.counts.post, 0);
});
