import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { writeFile } from "node:fs/promises";
import { CanvasNodeType, type CanvasNodeMetadata, type HNLocalPreparedReceipt, type HNLocalArchiveReceipt } from "../types";
import { HNLocalArchiveController, validHNLocalArchiveReceipt, sourceMediaFingerprint, mergeHNLocalArchiveReceipt, type HNArchiveTarget, type HNArchiveJournal, type HNArchiveDependencies } from "./hn-local-canvas-archive";
import { mapHNCanvasProjectId } from "./hn-local-canvas-prepare";

const evidence: Record<string, unknown>[] = [];
export const mp4 = new Uint8Array([0, 0, 0, 24, 102, 116, 121, 112, 105, 115, 111, 109, 0, 0, 0, 0, 105, 115, 111, 109, 109, 112, 52, 50]);
export async function archiveFixture() {
    const project = "_Project", nodeId = "_video", mapped = await mapHNCanvasProjectId(project);
    const prepared: HNLocalPreparedReceipt = { version: 1, canvasProjectId: project, hnProjectId: mapped, sourceNodeId: nodeId, shotId: "shot", generationId: "generation", frozenHash: "a".repeat(64), preparedAt: "2026-10-04T00:00:00.000Z", sourceBaseline: "16047f46e2186373ea824e12e84ae8dfa2ccde32", requestFingerprint: "b".repeat(64), snapshot: { model: "local intent", seconds: "4", vquality: "768P", size: "16:9", referenceCount: 0 } };
    let target: HNArchiveTarget = { canvasProjectId: project, node: { id: nodeId, type: CanvasNodeType.Video, title: "Local", width: 100, height: 100, position: { x: 0, y: 0 }, metadata: { content: "blob:local", storageKey: "video:local", status: "success", hnLocalPrepared: prepared } } };
    let blob: Blob | null = new Blob([mp4], { type: "video/mp4" });
    const store = new Map<string, HNLocalArchiveReceipt>();
    const counts = { post: 0, fresh: 0, retry: 0, discovery: 0, blob: 0, journalWrite: 0, forbidden: 0 };
    let mode = "success", mutate: ((payload: Record<string, unknown>) => void) | undefined;
    const journal: HNArchiveJournal = { getItem: async (k) => structuredClone(store.get(k) || null), setItem: async (k, v) => { counts.journalWrite++; store.set(k, structuredClone(v)); return v; } };
    const request = (async (url, options) => {
        assert.equal(options?.credentials, "omit"); assert.equal(options?.redirect, "error");
        if (url === "/api/hn/local-endpoint") { counts.discovery++; return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" } }); }
        assert.equal(options?.method, "POST"); assert.equal(new Headers(options?.headers).get("X-HN-Local-Request"), "1");
        const fresh = String(url) === `http://127.0.0.1:8085/api/hn/projects/${mapped}/generations/generation/results/local-archive`;
        if (!fresh && String(url) !== `http://127.0.0.1:8085/api/hn/projects/${mapped}/archive-jobs/job-1/retry`) { counts.forbidden++; assert.fail("forbidden boundary"); }
        counts.post++; if (fresh) counts.fresh++; else counts.retry++;
        const body = options!.body as FormData, data = body.get("file") as Blob;
        assert.deepEqual([...body.keys()].sort(), ["file", "mimeType", "resultKind", "sha256"]);
        const saved = store.get(JSON.stringify([project, nodeId])); assert.equal(saved?.outcome, "ARCHIVING");
        assert.equal(saved!.sourceSHA256, body.get("sha256"));
        if (mode === "drop") throw new Error("synthetic raw path /private and response must not escape");
        if (mode === "malformed") return new Response("not JSON");
        if (mode === "reject" || mode === "unavailable") return Response.json({ code: 1, msg: "raw /private must never display" }, { status: mode === "reject" ? 400 : 503 });
        const failed = mode === "failed";
        const facts = { resultId: fresh ? `result-${counts.fresh}` : "result-1", generationId: "generation", archiveJobId: fresh ? `job-${counts.fresh}` : "job-1", resultStatus: failed ? "ARCHIVE_FAILED" : "ARCHIVED", archiveStatus: failed ? "FAILED" : "ARCHIVED", archivedRelativePath: failed ? "" : `generated/generation/${fresh ? `result-${counts.fresh}` : "result-1"}/media.mp4`, sha256: failed ? "" : body.get("sha256"), byteLength: failed ? 0 : data.size, mimeType: data.type };
        const payload: Record<string, unknown> = { code: failed ? 1 : 0, data: facts }; mutate?.(payload);
        return Response.json(payload, { status: failed ? 422 : 200 });
    }) as typeof fetch;
    const d: HNArchiveDependencies = { journal, request, readBlob: async () => { counts.blob++; return blob; } };
    const c = new HNLocalArchiveController();
    const receive = (r: HNLocalArchiveReceipt) => { target = { ...target, node: mergeHNLocalArchiveReceipt([target.node], r, target.canvasProjectId)[0] }; };
    const run = (confirm: Parameters<HNLocalArchiveController["archive"]>[2] = async () => true) => c.archive(() => target, d, confirm, receive);
    return { c, d, counts, store, prepared, run, receive, target: () => target, setTarget: (t: HNArchiveTarget) => { target = t; }, setBlob: (b: Blob | null) => { blob = b; }, mode: (v: string) => { mode = v; }, mutate: (f: typeof mutate) => { mutate = f; }, entry: () => c.entry(project, nodeId) };
}
const deferred = () => { let resolve!: () => void; const promise = new Promise<void>((r) => { resolve = r; }); return { promise, resolve }; };
async function settled(f: Awaited<ReturnType<typeof archiveFixture>>) { await f.c.inspect(f.target(), f.d); }

test("open local inspection zero requests/writes; exact first success and byte locator duplicate", async () => {
    const f = await archiveFixture(); await settled(f); await settled(f);
    assert.equal(f.counts.post, 0); assert.equal(f.counts.discovery, 0); assert.equal(f.counts.journalWrite, 0);
    assert.equal(await f.run(), true); assert.equal(f.entry().phase, "ARCHIVED");
    const r = f.entry().receipt!; assert.equal(await validHNLocalArchiveReceipt(r, "_Project", "_video", f.prepared), true);
    assert.equal(await f.run(), false); assert.equal(f.counts.post, 1);
    f.setTarget({ ...f.target(), node: { ...f.target().node, metadata: { ...f.target().node.metadata, storageKey: "file:other" } } });
    assert.equal(await f.run(), false); assert.equal(f.counts.post, 1); assert.equal(f.entry().action, undefined);
    assert.equal(JSON.stringify(r).includes("storageKey"), false); assert.equal(JSON.stringify(r).includes("blob:"), false);
    evidence.push({ scenario: "same-byte-and-new-locator", ...f.counts, phase: f.entry().phase });
});
test("closed receipts reject every unsafe/foreign field, pair, hash, size, timestamp and owner", async () => {
    const f = await archiveFixture(); await f.run(); const r = f.entry().receipt!;
    const mutations = [{ extra: "unexpected" }, { canvasProjectId: "other" }, { hnProjectId: "wrong" }, { sourceNodeId: "other" }, { shotId: "other" }, { generationId: "other" }, { preparedFrozenHash: "c".repeat(64) }, { sourceSHA256: "A".repeat(64) }, { sourceByteLength: 0 }, { sourceByteLength: 64 * 1024 * 1024 + 1 }, { sourceMimeType: "audio/mpeg" }, { sourceMediaFingerprint: "0".repeat(64) }, { attemptId: "not-uuid" }, { observedAt: "2026-02-31T00:00:00.000Z" }, { resultId: "../private" }, { archiveJobId: "https://invalid" }, { sha256: "c".repeat(64) }, { resultStatus: "RECEIVED" }];
    for (const m of mutations) assert.equal(await validHNLocalArchiveReceipt({ ...r, ...m }, "_Project", "_video", f.prepared), false, JSON.stringify(m));
    for (const value of [null, [], { ...r, outcome: "invalid" }]) assert.equal(await validHNLocalArchiveReceipt(value, "_Project", "_video", f.prepared), false);
    assert.equal(r.sourceMediaFingerprint, await sourceMediaFingerprint(r.sourceSHA256, r.sourceByteLength, r.sourceMimeType));
});
test("preflight rejects invalid prepared owner and source before HTTP; remote never read", async () => {
    for (const metadata of [{ hnLocalPrepared: undefined }, { hnLocalPrepared: { invalid: true } }, { hnLocalPrepared: { ...(await archiveFixture()).prepared, sourceNodeId: "foreign" } }, { storageKey: "server:local" }, { storageKey: "https://remote.invalid" }, { workflowRef: {} }, { videoTaskId: "task" }, { channelId: "channel" }, { status: "loading" }]) {
        const f = await archiveFixture(); f.setTarget({ ...f.target(), node: { ...f.target().node, metadata: { ...f.target().node.metadata, ...metadata } as unknown as CanvasNodeMetadata } });
        assert.equal(await f.run(), false); assert.equal(f.counts.post, 0); assert.equal(f.counts.blob, 0); assert.equal(f.entry().action, undefined);
    }
    for (const b of [null, new Blob([], { type: "video/mp4" }), new Blob([mp4], { type: "audio/mpeg" }), new Blob([new Uint8Array(64 * 1024 * 1024 + 1)], { type: "video/mp4" })]) { const f = await archiveFixture(); f.setBlob(b); await f.run(); assert.equal(f.counts.post, 0); assert.equal(f.counts.journalWrite, 0); }
});
test("lock acquired synchronously through confirmation, delayed journal and dialog reopen", async () => {
    const f = await archiveFixture(), gate = deferred(), entered = deferred();
    const first = f.run(async () => { entered.resolve(); await gate.promise; return true; });
    assert.equal(f.entry().running, true); await entered.promise;
    assert.equal(await f.run(), false); await settled(f); assert.equal(f.entry().running, true);
    gate.resolve(); await first; assert.equal(f.counts.post, 1);
    const second = await archiveFixture(), saved = second.d.journal!.setItem, delay = deferred(), started = deferred();
    second.d.journal!.setItem = async (key, receipt) => { if (receipt.outcome === "ARCHIVING") { started.resolve(); await delay.promise; } return saved(key, receipt); };
    const pending = second.run(); await started.promise; assert.equal(second.counts.post, 0); assert.equal(await second.run(), false); delay.resolve(); await pending; assert.equal(second.counts.post, 1);
    evidence.push({ scenario: "page-lock-and-await-journal", ...second.counts });
});
test("journal fail/read-back mismatch/deleted barrier -> zero POST and safe static error", async () => {
    for (const mode of ["set-fail", "read-fail", "read-null", "read-wrong-attempt"]) {
        const f = await archiveFixture(), set = f.d.journal!.setItem, get = f.d.journal!.getItem;
        f.d.journal!.setItem = async (key, receipt) => { if (mode === "set-fail") throw new Error("raw /private"); return set(key, receipt); };
        f.d.journal!.getItem = async (key) => { const r = await get(key); if (!r) return r; if (mode === "read-fail") throw new Error("raw /private"); if (mode === "read-null") return null; if (mode === "read-wrong-attempt") return { ...(r as object), attemptId: crypto.randomUUID() }; return r; };
        await f.run(); assert.equal(f.counts.post, 0); assert.equal(f.entry().error, "HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
    }
});
test("independent owner may archive while another exact owner confirmation is locked", async () => {
    const f = await archiveFixture(), gate = deferred(), entered = deferred();
    const first = f.run(async () => { entered.resolve(); await gate.promise; return true; }); await entered.promise;
    const t = f.target(), other: HNArchiveTarget = { ...t, node: { ...t.node, id: "_other", metadata: { ...t.node.metadata, hnLocalPrepared: { ...f.prepared, sourceNodeId: "_other", generationId: "other-generation", shotId: "other-shot" } } } };
    let otherPosts = 0;
    const request = (async (url, init) => {
        if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" } });
        assert.equal(String(url), `http://127.0.0.1:8085/api/hn/projects/${f.prepared.hnProjectId}/generations/other-generation/results/local-archive`); otherPosts++;
        const form = init!.body as FormData, blob = form.get("file") as Blob;
        return Response.json({ code: 0, data: { resultId: "other-result", archiveJobId: "other-job", generationId: "other-generation", resultStatus: "ARCHIVED", archiveStatus: "ARCHIVED", archivedRelativePath: "generated/other-generation/other-result/media.mp4", sha256: form.get("sha256"), byteLength: blob.size, mimeType: blob.type } });
    }) as typeof fetch;
    assert.equal(await f.c.archive(() => other, { ...f.d, request }, async () => true, () => {}), true); assert.equal(otherPosts, 1); assert.equal(f.entry().running, true);
    gate.resolve(); await first; assert.equal(f.counts.post, 1);
});
test("journal replacement during discovery blocks wire POST; disposal cannot dispatch", async () => {
    const f = await archiveFixture(), request = f.d.request!;
    f.d.request = (async (url, init) => { const response = await request(url, init); if (url === "/api/hn/local-endpoint") f.store.clear(); return response; }) as typeof fetch;
    await f.run(); assert.equal(f.counts.post, 0); assert.equal(f.entry().error, "HN_ARCHIVE_LOCAL_STATE_UNAVAILABLE");
    const disposed = await archiveFixture(); await disposed.run(async () => { disposed.c.dispose(); return true; }); assert.equal(disposed.counts.post, 0);
});
test("changed byte known success requires separate explicit confirmation; cancellation0 POST", async () => {
    const f = await archiveFixture(); await f.run(); f.setBlob(new Blob([mp4, "B"], { type: "video/mp4" })); await settled(f);
    assert.equal(f.entry().phase, "SOURCE_CHANGED"); assert.equal(f.entry().action, "new");
    assert.equal(await f.run(async (action) => { assert.equal(action, "new"); return false; }), false); assert.equal(f.counts.post, 1);
    assert.equal(await f.run(), true); assert.equal(f.counts.fresh, 2); assert.equal((f.entry().receipt as { resultId: string }).resultId, "result-2");
    evidence.push({ scenario: "changed-bytes-explicit-new", ...f.counts });
});
test("known 422 status matrix, same exact job owner retry, changed bytes cannot retarget", async () => {
    for (const pair of [["ARCHIVE_FAILED", "FAILED"], ["RECEIVED", "PENDING"], ["RECEIVED", "COPYING"], ["RECEIVED", "FINALIZING"]]) {
        const f = await archiveFixture(); f.mode("failed"); f.mutate((p) => Object.assign(p.data as object, { resultStatus: pair[0], archiveStatus: pair[1] })); await f.run();
        assert.equal(f.entry().receipt?.outcome, "ARCHIVE_FAILED_RETRYABLE");
        f.setBlob(new Blob([mp4, "change"], { type: "video/mp4" })); await f.run(); assert.equal(f.counts.post, 1);
        f.setBlob(new Blob([mp4], { type: "video/mp4" })); f.mode("success"); f.mutate(undefined); assert.equal(await f.run(), true);
        assert.equal(f.counts.fresh, 1); assert.equal(f.counts.retry, 1); assert.equal((f.entry().receipt as { resultId: string }).resultId, "result-1");
        evidence.push({ scenario: "same-job-retry", pair, ...f.counts });
    }
});
test("invalid success/422 facts lose fresh retry authority and become UNKNOWN", async () => {
    const changes = [{ generationId: "foreign" }, { resultId: "../private" }, { archiveJobId: "bad/url" }, { sha256: "0".repeat(64) }, { byteLength: 9 }, { archivedRelativePath: "private" }, { extra: "unsafe" }];
    for (const m of changes) { const f = await archiveFixture(); f.mutate((p) => Object.assign(p.data as object, m)); await f.run(); assert.equal(f.entry().phase, "ARCHIVE_OUTCOME_UNKNOWN"); await f.run(); assert.equal(f.counts.post, 1); }
    for (const pair of [["ARCHIVE_FAILED", "INCONSISTENT"], ["ARCHIVED", "FAILED"], ["RECEIVED", "UNKNOWN"], ["ARCHIVE_FAILED", "FINALIZING"]]) { const f = await archiveFixture(); f.mode("failed"); f.mutate((p) => Object.assign(p.data as object, { resultStatus: pair[0], archiveStatus: pair[1] })); await f.run(); assert.equal(f.entry().receipt?.outcome, "ARCHIVE_OUTCOME_UNKNOWN"); assert.equal(f.entry().action, undefined); }
});
test("response loss and malformed JSON -> UNKNOWN no resend even after undo/reload/byte change", async () => {
    for (const mode of ["drop", "malformed"]) {
        const f = await archiveFixture(); f.mode(mode); await f.run();
        assert.equal(f.entry().phase, "ARCHIVE_OUTCOME_UNKNOWN"); assert.equal(f.entry().action, undefined);
        f.setTarget({ ...f.target(), node: { ...f.target().node, metadata: { ...f.target().node.metadata, hnLocalArchive: undefined } } });
        const reloaded = new HNLocalArchiveController(); await reloaded.inspect(f.target(), f.d); assert.equal(reloaded.entry("_Project", "_video").phase, "ARCHIVE_OUTCOME_UNKNOWN");
        await reloaded.archive(f.target, f.d, async () => true, f.receive); f.setBlob(new Blob([mp4, "changed"], { type: "video/mp4" })); await f.run(); assert.equal(f.counts.post, 1);
        evidence.push({ scenario: "fresh-unknown-no-resend", mode, ...f.counts });
    }
});
test("retry response loss retains only prior job IDs; wrong retry Result/Generation cannot authorize new job", async () => {
    for (const alteration of [null, { resultId: "wrong" }, { generationId: "wrong" }, { archiveJobId: "wrong" }]) {
        const f = await archiveFixture(); f.mode("failed"); await f.run();
        f.mode(alteration ? "success" : "drop"); f.mutate(alteration ? (p) => Object.assign(p.data as object, alteration) : undefined); await f.run();
        assert.equal(f.entry().phase, "ARCHIVE_OUTCOME_UNKNOWN"); assert.equal(f.entry().action, "retry");
        f.mode("success"); f.mutate(undefined); await f.run(); assert.equal(f.counts.fresh, 1); assert.equal(f.counts.retry, 2); assert.equal(f.entry().phase, "ARCHIVED");
    }
});
test("reload known success/failure remains UNVERIFIED and pending becomes UNKNOWN; journal alone can't write", async () => {
    for (const failed of [false, true]) {
        const f = await archiveFixture(); f.mode(failed ? "failed" : "success"); await f.run();
        const r = new HNLocalArchiveController(); await r.inspect(f.target(), f.d); const e = r.entry("_Project", "_video"); assert.equal(e.phase, "RELOADED_UNVERIFIED"); assert.equal(e.action, failed ? "retry" : undefined);
        f.setTarget({ ...f.target(), node: { ...f.target().node, metadata: { ...f.target().node.metadata, hnLocalPrepared: undefined } } }); await r.inspect(f.target(), f.d); assert.ok(e.receipt); assert.equal(e.action, undefined); await r.archive(f.target, f.d, async () => true, f.receive); assert.equal(f.counts.post, 1);
    }
    const f = await archiveFixture(); f.mode("drop"); await f.run(); const key = JSON.stringify(["_Project", "_video"]);
    f.store.set(key, { ...f.store.get(key)!, outcome: "ARCHIVING", knownJob: null });
    f.setTarget({ ...f.target(), node: { ...f.target().node, metadata: { ...f.target().node.metadata, hnLocalArchive: undefined } } });
    const r = new HNLocalArchiveController(); await r.inspect(f.target(), f.d); assert.equal(r.entry("_Project", "_video").phase, "ARCHIVE_OUTCOME_UNKNOWN");
});
test("target switches during confirmation cancel before POST; late response only journal original owner", async () => {
    const f = await archiveFixture(); await f.run(async () => { f.setTarget({ ...f.target(), canvasProjectId: "other" }); return true; }); assert.equal(f.counts.post, 0);
    const late = await archiveFixture(), request = late.d.request!, gate = deferred(), entered = deferred();
    late.d.request = (async (url, options) => { const response = await request(url, options); if (options?.method === "POST") { entered.resolve(); await gate.promise; } return response; }) as typeof fetch;
    const work = late.run(); await entered.promise; late.setTarget({ ...late.target(), canvasProjectId: "other" }); gate.resolve(); await work;
    assert.equal(late.target().node.metadata?.hnLocalArchive, undefined); assert.equal(late.store.size, 1); assert.equal([...late.store.values()][0].canvasProjectId, "_Project");
});
test("known handler rejection versus discovery/no-dispatch and final journal-save ambiguity", async () => {
    for (const mode of ["reject", "unavailable"]) { const f = await archiveFixture(); f.mode(mode); await f.run(); assert.equal(f.entry().receipt?.outcome, "NOT_ARCHIVED"); assert.equal(f.entry().error, mode === "reject" ? "HN_ARCHIVE_REJECTED" : "HN_LOCAL_BACKEND_UNAVAILABLE"); }
    const f = await archiveFixture(); f.d.request = (async () => { throw new Error("raw /private"); }) as typeof fetch; await f.run(); assert.equal(f.counts.post, 0); assert.equal(f.entry().error, "HN_LOCAL_BACKEND_UNAVAILABLE");
    const final = await archiveFixture(), save = final.d.journal!.setItem;
    final.d.journal!.setItem = async (k, v) => { if (v.outcome !== "ARCHIVING") throw new Error("synthetic write failure"); return save(k, v); };
    await final.run(); assert.equal(final.entry().phase, "ARCHIVE_OUTCOME_UNKNOWN"); await final.run(); assert.equal(final.counts.post, 1);
});
test("actual localhost HTTP server observes exactly one POST despite repeated same-byte actions", async () => {
    const f = await archiveFixture(); let observed = 0;
    const server = createServer(async (req, res) => {
        assert.equal(req.method, "POST"); assert.ok(req.url?.endsWith("/results/local-archive")); assert.equal(req.headers["x-hn-local-request"], "1");
        const parts: Buffer[] = []; for await (const chunk of req) parts.push(Buffer.from(chunk)); assert.ok(Buffer.concat(parts).includes(Buffer.from(mp4))); observed++;
        const r = [...f.store.values()][0];
        res.setHeader("Content-Type", "application/json"); res.end(JSON.stringify({ code: 0, data: { resultId: "result-1", archiveJobId: "job-1", generationId: "generation", resultStatus: "ARCHIVED", archiveStatus: "ARCHIVED", archivedRelativePath: "generated/generation/result-1/media.mp4", sha256: r.sourceSHA256, byteLength: r.sourceByteLength, mimeType: r.sourceMimeType } }));
    });
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve)); const address = server.address(); assert.ok(address && typeof address !== "string");
    f.d.request = (async (url, options) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: `http://127.0.0.1:${address.port}` } }) : fetch(url, options)) as typeof fetch;
    try { await f.run(); await f.run(); assert.equal(observed, 1); evidence.push({ scenario: "real-loopback-http", serverObservedPost: observed, providerRequests: 0 }); }
    finally { await new Promise<void>((resolve) => server.close(() => resolve())); }
    if (process.env.HN_R20_EVIDENCE) await writeFile(process.env.HN_R20_EVIDENCE, JSON.stringify({ scope: "synthetic fixtures only", cases: evidence }, null, 2));
});
