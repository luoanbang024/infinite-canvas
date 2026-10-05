import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { writeFile } from "node:fs/promises";
import { CanvasNodeType, type HNLocalArchiveReceipt, type HNLocalCandidateReceipt, type HNLocalPreparedReceipt } from "../types";
import { mapHNCanvasProjectId } from "./hn-local-canvas-prepare";
import { HNLocalArchiveController, sourceMediaFingerprint, type HNArchiveTarget, type HNArchiveJournal } from "./hn-local-canvas-archive";
import { HNLocalCandidateController, validHNLocalCandidateReceipt, mergeHNLocalCandidateReceipt, type HNCandidateDependencies } from "./hn-local-canvas-candidate";

const evidence: object[] = [];
export async function candidateFixture() {
    const project = "_Project", nodeId = "_video", mapped = await mapHNCanvasProjectId(project);
    const prepared: HNLocalPreparedReceipt = { version: 1, canvasProjectId: project, hnProjectId: mapped, sourceNodeId: nodeId, shotId: "shot", generationId: "generation", frozenHash: "a".repeat(64), preparedAt: "2026-10-04T00:00:00.000Z", sourceBaseline: "16047f46e2186373ea824e12e84ae8dfa2ccde32", requestFingerprint: "b".repeat(64), snapshot: { model: "local intent", seconds: "4", vquality: "768P", size: "16:9", referenceCount: 0 } };
    const receipt: Extract<HNLocalArchiveReceipt, { outcome: "ARCHIVED" }> = { version: 1, canvasProjectId: project, hnProjectId: mapped, sourceNodeId: nodeId, shotId: "shot", generationId: "generation", preparedFrozenHash: prepared.frozenHash, attemptId: "11111111-1111-4111-8111-111111111111", observedAt: "2026-10-04T01:00:00.000Z", sourceSHA256: "c".repeat(64), sourceByteLength: 24, sourceMimeType: "video/mp4", sourceMediaFingerprint: await sourceMediaFingerprint("c".repeat(64), 24, "video/mp4"), outcome: "ARCHIVED", resultId: "result", archiveJobId: "job", resultStatus: "ARCHIVED", archiveStatus: "ARCHIVED", sha256: "c".repeat(64), byteLength: 24, mimeType: "video/mp4" };
    let target: HNArchiveTarget = { canvasProjectId: project, node: { id: nodeId, type: CanvasNodeType.Video, title: "Local", width: 100, height: 100, position: { x: 0, y: 0 }, metadata: { content: "blob:current", storageKey: "video:current", status: "success", hnLocalPrepared: prepared, hnLocalArchive: receipt } } };
    const store = new Map<string, unknown>([[JSON.stringify([project, nodeId]), structuredClone(receipt)]]);
    const counts = { discovery: 0, post: 0, blob: 0, journalWrite: 0, forbidden: 0, confirmation: 0, merge: 0 };
    let readHook: (() => Promise<void>) | undefined, responseHook: (() => Promise<Response>) | undefined, discoveryHook: (() => void) | undefined;
    const journal: HNArchiveJournal = { getItem: async (k) => { if (readHook) await readHook(); return structuredClone(store.get(k) ?? null); }, setItem: async (k, v) => { counts.journalWrite++; store.set(k, structuredClone(v)); return v; } };
    const facts = () => ({ candidateId: "candidate", projectId: mapped, shotId: receipt.shotId, generationId: receipt.generationId, resultId: receipt.resultId, label: "Historical Local", availabilityStatus: "ARCHIVED", createdAt: "2026-10-04T01:00:00Z", updatedAt: "2026-10-04T01:00:00.000123Z" });
    const request = (async (url, options) => {
        assert.equal(options?.credentials, "omit"); assert.equal(options?.redirect, "error");
        if (url === "/api/hn/local-endpoint") { counts.discovery++; assert.equal(options?.cache, "no-store"); discoveryHook?.(); return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" }, msg: "ok" }); }
        if (String(url) !== `http://127.0.0.1:8085/api/hn/projects/${mapped}/shots/shot/candidates/ensure`) { counts.forbidden++; assert.fail("unexpected boundary"); }
        counts.post++; assert.equal(options?.method, "POST"); assert.equal(new Headers(options?.headers).get("X-HN-Local-Request"), "1");
        assert.deepEqual(JSON.parse(String(options?.body)), { resultId: "result", label: "Local Archive" });
        return responseHook ? responseHook() : Response.json({ code: 0, data: facts(), msg: "ok" });
    }) as typeof fetch;
    const archive = new HNLocalArchiveController(), c = new HNLocalCandidateController(), d = { journal, request, readBlob: async () => { counts.blob++; assert.fail("Candidate must not read current Blob"); } } satisfies HNCandidateDependencies & { readBlob: () => Promise<never> };
    const current = () => target, entry = () => c.entry(project, nodeId);
    const receive = (r: HNLocalCandidateReceipt, archived: typeof receipt) => { counts.merge++; target = { ...target, node: mergeHNLocalCandidateReceipt([target.node], r, target.canvasProjectId, archived)[0] }; };
    const confirm = async () => { counts.confirmation++; return true; };
    return { project, nodeId, prepared, receipt, store, journal, counts, archive, c, d, current, entry, receive, facts, confirm,
        run: (revalidate = false) => c.ensure(current, archive, d, confirm, receive, revalidate),
        inspect: () => c.inspect(current, archive, d),
        setTarget: (v: HNArchiveTarget) => { target = v; },
        setRead: (f?: () => Promise<void>) => { readHook = f; },
        setResponse: (f?: () => Promise<Response>) => { responseHook = f; },
        setDiscovery: (f?: () => void) => { discoveryHook = f; },
    };
}

test("R22 historical read is detached, does not inspect Blob/write/HTTP or promote R20 phase", async () => {
    const f = await candidateFixture();
    const before = structuredClone(f.archive.entry(f.project, f.nodeId));
    const r = await f.archive.readHistoricalArchiveReceipt(f.current, f.d);
    assert.deepEqual(r, f.receipt); r.shotId = "changed";
    assert.equal(f.receipt.shotId, "shot"); assert.deepEqual(f.archive.entry(f.project, f.nodeId), before);
    await f.inspect(); assert.equal(f.entry().phase, "NOT_CANDIDATE"); assert.equal(f.counts.post, 0); assert.equal(f.counts.discovery, 0); assert.equal(f.counts.journalWrite, 0);
    f.archive.entry(f.project, f.nodeId).phase = "RELOADED_UNVERIFIED";
    assert.equal(await f.run(), true); assert.equal(f.counts.post, 1); assert.equal(f.archive.entry(f.project, f.nodeId).phase, "RELOADED_UNVERIFIED");
    assert.equal(f.counts.blob, 0); assert.equal(f.counts.forbidden, 0); assert.equal(f.entry().phase, "CANDIDATE_READY");
    assert.equal(await f.run(), false); assert.equal(f.counts.post, 1);
    assert.equal(await f.run(true), true); assert.equal(f.counts.post, 2); assert.equal(f.entry().receipt?.candidateId, "candidate");
    evidence.push({ scenario: "historical-only-safe-label-repeat", ...f.counts, candidateId: "candidate", R20Phase: "RELOADED_UNVERIFIED" });
});

test("R22 historical eligibility ignores changed/missing/remote current media and status", async () => {
    for (const metadata of [{ content: undefined, storageKey: undefined }, { storageKey: "server:unsupported", content: "https://media.invalid/current" }, { storageKey: "file:changed", status: "loading" }, { status: "error", videoTaskId: "different-current-task", channelId: "not-read" }] as const) {
        const f = await candidateFixture(); f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { ...f.current().node.metadata, ...metadata } } });
        assert.equal(await f.run(), true); assert.equal(f.counts.blob, 0); assert.equal(f.counts.post, 1);
        assert.equal(f.entry().receipt?.resultId, "result"); await f.inspect(); assert.equal(f.entry().phase, "CANDIDATE_READY");
    }
});

test("R22 missing/foreign/malformed durable history never falls back to Canvas/cache", async () => {
    for (const mode of ["missing", "read-error", "extra", "foreign", "wrong-generation", "conflict", "disposed", "active-archive", "missing-prepared", "non-video"]) {
        const f = await candidateFixture(), key = JSON.stringify([f.project, f.nodeId]);
        f.archive.entry(f.project, f.nodeId).receipt = structuredClone(f.receipt);
        if (mode === "missing") f.store.delete(key);
        if (mode === "read-error") f.setRead(async () => { throw new Error("raw private path must not escape"); });
        if (mode === "extra") f.store.set(key, { ...f.receipt, extra: "unexpected" });
        if (mode === "foreign") f.store.set(key, { ...f.receipt, canvasProjectId: "other" });
        if (mode === "wrong-generation") f.store.set(key, { ...f.receipt, generationId: "other" });
        if (mode === "conflict") f.store.set(key, { ...f.receipt, observedAt: "2026-10-04T02:00:00.000Z" });
        if (mode === "disposed") f.archive.dispose();
        if (mode === "active-archive") f.archive.entry(f.project, f.nodeId).running = true;
        if (mode === "missing-prepared") f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { ...f.current().node.metadata, hnLocalPrepared: undefined } } });
        if (mode === "non-video") f.setTarget({ ...f.current(), node: { ...f.current().node, type: CanvasNodeType.Image } });
        assert.equal(await f.run(), false, mode); assert.equal(f.counts.post, 0, mode); assert.equal(f.counts.discovery, 0); assert.equal(f.counts.confirmation, 0); assert.equal(f.counts.merge, 0);
    }
});

test("R22 pending UNKNOWN and failed pairs block Candidate even over older Canvas success", async () => {
    for (const outcome of ["ARCHIVING", "ARCHIVE_OUTCOME_UNKNOWN", "ARCHIVE_FAILED_RETRYABLE"]) {
        const f = await candidateFixture(); const { resultId, archiveJobId, resultStatus, archiveStatus, sha256, byteLength, mimeType, ...common } = f.receipt;
        const r = outcome === "ARCHIVE_FAILED_RETRYABLE" ? { ...common, outcome, resultId, archiveJobId, resultStatus: "ARCHIVE_FAILED", archiveStatus: "FAILED" } : { ...common, outcome, knownJob: { resultId, archiveJobId } };
        f.store.set(JSON.stringify([f.project, f.nodeId]), r);
        if (outcome === "ARCHIVE_FAILED_RETRYABLE") f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { ...f.current().node.metadata, hnLocalArchive: r as HNLocalArchiveReceipt } } });
        await f.inspect(); assert.equal(f.entry().phase, "ARCHIVE_NOT_ELIGIBLE"); assert.equal(await f.run(), false); assert.equal(f.counts.post, 0);
    }
});

test("R22 closed Candidate receipt rejects every owner/id/hash/time/extra mismatch", async () => {
    const f = await candidateFixture(); await f.run(); const r = f.entry().receipt!;
    const mutations = [{ version: 2 }, { canvasProjectId: "other" }, { hnProjectId: "other" }, { sourceNodeId: "other" }, { shotId: "other" }, { generationId: "other" }, { resultId: "other" }, { archiveJobId: "other" }, { candidateId: "../unsafe" }, { preparedFrozenHash: "d".repeat(64) }, { sourceMediaFingerprint: "d".repeat(64) }, { availabilityStatus: "RECEIVED" }, { observedAt: "2026-02-31T00:00:00.000Z" }, { observedAt: "2026-10-04T00:00:00Z" }, { label: "not persisted" }, { selected: true }, { path: "forbidden" }];
    for (const m of mutations) assert.equal(await validHNLocalCandidateReceipt({ ...r, ...m }, f.current(), f.receipt), false, JSON.stringify(m));
    for (const v of [null, [], { ...r, provider: "forbidden" }]) assert.equal(await validHNLocalCandidateReceipt(v, f.current(), f.receipt), false);
    assert.equal(Object.keys(r).length, 13); assert.equal(await validHNLocalCandidateReceipt(r, f.current(), f.receipt), true);
});

test("R22 reload is unverified, local preview is zero HTTP, explicit confirmation restores same ID", async () => {
    const f = await candidateFixture(); await f.run();
    const raw = JSON.parse(JSON.stringify(f.current())); f.setTarget(raw);
    const reload = new HNLocalCandidateController(), archive = new HNLocalArchiveController();
    const before = f.counts.post; await reload.inspect(f.current, archive, f.d);
    assert.equal(reload.entry(f.project, f.nodeId).phase, "CANDIDATE_RELOADED_UNVERIFIED"); assert.equal(f.counts.post, before);
    assert.equal(await reload.ensure(f.current, archive, f.d, f.confirm, f.receive), true);
    assert.equal(reload.entry(f.project, f.nodeId).receipt?.candidateId, "candidate"); assert.equal(f.counts.post, before + 1);
    f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { ...f.current().node.metadata, hnLocalCandidate: undefined } } });
    const noReceipt = new HNLocalCandidateController(); await noReceipt.inspect(f.current, archive, f.d); assert.equal(noReceipt.entry(f.project, f.nodeId).phase, "NOT_CANDIDATE");
    evidence.push({ scenario: "reload-advisory-explicit-only", reloadPhase: "CANDIDATE_RELOADED_UNVERIFIED", recoveredCandidate: "candidate" });
});

test("R22 synchronous page lock survives confirmation, duplicate calls, and close/reopen preview", async () => {
    const f = await candidateFixture(); let release!: (v: boolean) => void;
    const pending = f.c.ensure(f.current, f.archive, f.d, () => new Promise<boolean>((resolve) => { release = resolve; }), f.receive);
    assert.equal(f.entry().running, true); await f.inspect(); assert.equal(f.entry().running, true);
    assert.equal(await f.run(), false); assert.equal(f.counts.post, 0);
    while (!release) await new Promise((r) => setTimeout(r, 1));
    release(true); assert.equal(await pending, true); assert.equal(f.counts.post, 1);
    assert.notEqual(f.c.entry(f.project, f.nodeId), f.c.entry(f.project, "other")); assert.notEqual(f.c.entry(f.project, f.nodeId), f.c.entry("other", f.nodeId));
});

test("R22 first await lock blocks even while journal is pending", async () => {
    const f = await candidateFixture(); let release!: () => void;
    f.setRead(() => new Promise<void>((r) => { release = r; })); const pending = f.run(); assert.equal(f.entry().running, true);
    assert.equal(await f.run(), false); assert.equal(f.counts.post, 0);
    while (!release) await new Promise((r) => setTimeout(r, 1)); f.setRead(); release(); assert.equal(await pending, true); assert.equal(f.counts.post, 1);
});

test("R22 cancel, project/node/owner switch, disposal or newer archive during discovery sends no POST", async () => {
    for (const mode of ["cancel", "project", "node", "owner", "dispose", "archive-unknown", "archive-result", "during-discovery"]) {
        const f = await candidateFixture();
        const mutate = () => {
            if (mode === "project") f.setTarget({ ...f.current(), canvasProjectId: "other" });
            if (mode === "node") f.setTarget({ ...f.current(), node: { ...f.current().node, id: "other" } });
            if (mode === "owner") f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { ...f.current().node.metadata, hnLocalPrepared: { ...f.prepared, generationId: "other" } } } });
            if (mode === "dispose") f.c.dispose();
            if (["archive-unknown", "during-discovery"].includes(mode)) { const { resultId, archiveJobId, resultStatus, archiveStatus, sha256, byteLength, mimeType, ...common } = f.receipt; f.store.set(JSON.stringify([f.project, f.nodeId]), { ...common, outcome: "ARCHIVING", knownJob: null }); }
            if (mode === "archive-result") { const newer = { ...f.receipt, resultId: "new-result", archiveJobId: "new-job", attemptId: "22222222-2222-4222-8222-222222222222", observedAt: "2026-10-04T02:00:00.000Z" }; f.store.set(JSON.stringify([f.project, f.nodeId]), newer); }
        };
        if (mode === "during-discovery") f.setDiscovery(mutate);
        assert.equal(await f.c.ensure(f.current, f.archive, f.d, async () => { if (mode !== "during-discovery") mutate(); return mode !== "cancel"; }, f.receive), false, mode);
        assert.equal(f.counts.post, 0, mode); assert.equal(f.counts.merge, 0);
    }
});

test("R22 late success never merges replacement owner and is UNKNOWN", async () => {
    for (const mode of ["project", "node", "owner", "dispose", "archive"]) {
        const f = await candidateFixture();
        f.setResponse(async () => {
            if (mode === "project") f.setTarget({ ...f.current(), canvasProjectId: "other" });
            if (mode === "node") f.setTarget({ ...f.current(), node: { ...f.current().node, id: "other" } });
            if (mode === "owner") f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { ...f.current().node.metadata, hnLocalPrepared: { ...f.prepared, generationId: "other" } } } });
            if (mode === "dispose") f.c.dispose();
            if (mode === "archive") f.store.delete(JSON.stringify([f.project, f.nodeId]));
            return Response.json({ code: 0, data: f.facts(), msg: "ok" });
        });
        assert.equal(await f.run(), false); assert.equal(f.counts.post, 1); assert.equal(f.counts.merge, 0); assert.equal(f.entry().phase, "CANDIDATE_OUTCOME_UNKNOWN");
    }
});

test("R22 every malformed or foreign success is UNKNOWN without leaking raw msg", async () => {
    const bad = [{ extra: true }, { selected: true }, { candidateId: "../bad" }, { projectId: "other" }, { shotId: "other" }, { generationId: "other" }, { resultId: "other" }, { availabilityStatus: "PENDING" }, { label: "https://private.invalid" }, { label: "Bearer fake-forbidden" }, { createdAt: "2026-02-31T00:00:00Z" }, { updatedAt: "yesterday" }];
    for (const mutation of bad) { const f = await candidateFixture(); f.setResponse(async () => Response.json({ code: 0, data: { ...f.facts(), ...mutation }, msg: "ok" })); assert.equal(await f.run(), false, JSON.stringify(mutation)); assert.equal(f.entry().phase, "CANDIDATE_OUTCOME_UNKNOWN"); assert.equal(f.counts.merge, 0); assert.equal(f.entry().receipt, undefined); }
    for (const response of [new Response("not-json"), Response.json({ code: 0, data: null, msg: "ok" }), Response.json({ code: 0, data: {}, msg: "raw private" }), Response.json({ code: 1, data: null, msg: "raw private" }, { status: 500 }), Response.json({ code: 1, data: null, msg: "raw private" }, { status: 422 }), Response.json({ code: 0, data: {}, msg: "ok", extra: true })]) {
        const f = await candidateFixture(); f.setResponse(async () => response); assert.equal(await f.run(), false); assert.equal(f.entry().phase, "CANDIDATE_OUTCOME_UNKNOWN"); assert.equal(f.entry().error, "HN_CANDIDATE_OUTCOME_UNKNOWN");
    }
});

test("R22 strict known rejection and identity contradiction stop without replacing receipt", async () => {
    for (const [status, msg] of [[400, "HN 剪辑标识或输入无效"], [403, "HN 剪辑需要显式本机请求标记"], [503, "HN 剪辑未启用：未配置 HN_PROJECTS_ROOT"], [409, "EDITORIAL_OWNERSHIP_CONFLICT"], [409, "CANDIDATE_IDENTITY_CONFLICT"]] as const) {
        const f = await candidateFixture(); f.setResponse(async () => Response.json({ code: 1, data: null, msg }, { status })); assert.equal(await f.run(), false); assert.equal(f.entry().phase, "ERROR"); assert.equal(f.counts.merge, 0);
    }
    const f = await candidateFixture(); await f.run(); const original = f.entry().receipt;
    f.setResponse(async () => Response.json({ code: 0, data: { ...f.facts(), candidateId: "other-candidate" }, msg: "ok" }));
    assert.equal(await f.run(true), false); assert.equal(f.entry().phase, "ERROR"); assert.equal(f.entry().error, "HN_CANDIDATE_IDENTITY_CONFLICT"); assert.equal(f.entry().receipt, original); assert.equal(f.current().node.metadata?.hnLocalCandidate?.candidateId, "candidate");
});

test("R22 discovery forbids remote/userinfo/query/alternate path and redirects", async () => {
    for (const url of ["https://provider.invalid", "http://127.0.0.1:8080/other", "http://user:password@localhost:8080", "http://localhost:8080/?unexpected=1", "http://192.0.2.1", "http://localhost:8080/#bad"]) {
        const f = await candidateFixture(); const request = (async () => Response.json({ code: 0, data: { url }, msg: "ok" })) as typeof fetch;
        assert.equal(await f.c.ensure(f.current, f.archive, { ...f.d, request }, f.confirm, f.receive), false); assert.equal(f.counts.post, 0); assert.equal(f.entry().phase, "ERROR");
    }
    const f = await candidateFixture(); const server = createServer((req, res) => { f.counts.discovery++; res.writeHead(302, { Location: "/forbidden" }); res.end(); });
    await new Promise<void>((r) => server.listen(0, "127.0.0.1", r)); const address = server.address(); assert.ok(address && typeof address !== "string");
    try { const request = ((url, init) => fetch(url === "/api/hn/local-endpoint" ? `http://127.0.0.1:${address.port}/discovery` : url, init)) as typeof fetch; assert.equal(await f.c.ensure(f.current, f.archive, { ...f.d, request }, f.confirm, f.receive), false); assert.equal(f.counts.discovery, 1); assert.equal(f.counts.post, 0); } finally { server.close(); }
});

test("R22 actual localhost POST count, lost response, explicit recovery, Candidate count one", async () => {
    const f = await candidateFixture(), candidates = new Map<string, ReturnType<typeof f.facts>>(); let posts = 0, forbidden = 0, lose = true;
    const server = createServer(async (req, res) => {
        if (req.method !== "POST" || req.url !== `/api/hn/projects/${f.prepared.hnProjectId}/shots/shot/candidates/ensure`) { forbidden++; res.writeHead(404).end(); return; }
        posts++; const parts: Buffer[] = []; for await (const part of req) parts.push(part); const body = JSON.parse(Buffer.concat(parts).toString()); assert.deepEqual(body, { resultId: "result", label: "Local Archive" });
        if (!candidates.has(body.resultId)) candidates.set(body.resultId, f.facts()); res.setHeader("Content-Type", "application/json"); res.end(JSON.stringify({ code: 0, data: candidates.get(body.resultId), msg: "ok" }));
    });
    await new Promise<void>((r) => server.listen(0, "127.0.0.1", r)); const address = server.address(); assert.ok(address && typeof address !== "string");
    const request = (async (url, options) => {
        if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: `http://127.0.0.1:${address.port}` }, msg: "ok" });
        const response = await fetch(url, options); if (lose) { lose = false; await response.text(); throw new Error("synthetic response loss after commit"); } return response;
    }) as typeof fetch;
    try {
        const d = { ...f.d, request }; const first = f.c.ensure(f.current, f.archive, d, f.confirm, f.receive); assert.equal(await f.c.ensure(f.current, f.archive, d, f.confirm, f.receive), false);
        assert.equal(await first, false); assert.equal(posts, 1); assert.equal(candidates.size, 1); assert.equal(f.entry().phase, "CANDIDATE_OUTCOME_UNKNOWN");
        await f.c.inspect(f.current, f.archive, d); await new Promise((r) => setTimeout(r, 10)); assert.equal(posts, 1);
        assert.equal(await f.c.ensure(f.current, f.archive, d, f.confirm, f.receive), true); assert.equal(posts, 2); assert.equal(candidates.size, 1); assert.equal(f.entry().receipt?.candidateId, "candidate"); assert.equal(forbidden, 0);
        evidence.push({ scenario: "actual-localhost-response-loss", observedFirstPOST: 1, autoRetryPOST: 0, explicitRecoveryAdditionalPOST: 1, candidateCount: 1, sameCandidateId: true, forbiddenRequests: forbidden });
    } finally { server.close(); }
});

test("R22 actual production handler bridge response-loss recovery", { skip: !process.env.HN_R22_BRIDGE_FIXTURE }, async () => {
    const fixture = JSON.parse(process.env.HN_R22_BRIDGE_FIXTURE!), f = await candidateFixture();
    const r = fixture.archive as Extract<HNLocalArchiveReceipt, { outcome: "ARCHIVED" }>;
    f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { hnLocalPrepared: fixture.prepared, hnLocalArchive: r, storageKey: "server:unsupported-current" } } });
    f.store.set(JSON.stringify([f.project, f.nodeId]), r); let lose = true;
    const request = (async (url, options) => {
        const response = await fetch(url === "/api/hn/local-endpoint" ? fixture.base + "/api/hn/local-endpoint" : url, options);
        if (url !== "/api/hn/local-endpoint" && lose) { lose = false; await response.text(); throw new Error("synthetic lost success response"); } return response;
    }) as typeof fetch;
    const d = { ...f.d, request }; assert.equal(await f.c.ensure(f.current, f.archive, d, f.confirm, f.receive), false); assert.equal(f.entry().phase, "CANDIDATE_OUTCOME_UNKNOWN");
    await f.c.inspect(f.current, f.archive, d); assert.equal(await f.c.ensure(f.current, f.archive, d, f.confirm, f.receive), true);
    assert.equal(f.entry().receipt?.resultId, r.resultId); assert.equal(f.counts.merge, 1);
    if (process.env.HN_R22_BRIDGE_OUTPUT) await writeFile(process.env.HN_R22_BRIDGE_OUTPUT, JSON.stringify({ receipt: f.entry().receipt, sourceBlobRead: 0, autoRetry: 0, fakeOnly: true }, null, 2));
});

test("R22 record safe frontend evidence", async () => { if (process.env.HN_R22_EVIDENCE) await writeFile(process.env.HN_R22_EVIDENCE, JSON.stringify({ runtime: "Injected page controller + actual localhost HTTP; no browser DOM/IndexedDB acceptance claimed", scenarios: evidence }, null, 2)); });
