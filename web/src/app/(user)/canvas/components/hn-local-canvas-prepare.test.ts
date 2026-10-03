import { test } from "node:test";
import assert from "node:assert/strict";
import { CanvasNodeType, type CanvasNodeData, type HNLocalPreparedReceipt } from "../types";
import { canHNLocalPrepare, captureHNLocalIntent, HNLocalError, HNLocalPrepareController, hnLocalSHA256, mapHNCanvasProjectId, mergeHNLocalReceipt, preflightHNLocalIntent, validHNLocalReceipt, type HNLocalVideoBusiness } from "./hn-local-canvas-prepare";

export const business = (): HNLocalVideoBusiness => ({ model: "", size: "16:9", videoSeconds: "4", vquality: "768P", videoMode: "std", videoNegativePrompt: "", videoMultiShot: "false", videoShotType: "intelligence", videoMultiPrompt: [], videoGenerateAudio: "false", videoWatermark: "false", videoCharacterOrientation: "video" });
export const video = (id = "_video"): CanvasNodeData => ({ id, type: CanvasNodeType.Video, title: "intent", position: { x: 0, y: 0 }, width: 300, height: 200, metadata: { prompt: "A quiet landscape", status: "idle" } });
export const image = (id: string, storageKey = "image:local"): CanvasNodeData => ({ ...video(id), type: CanvasNodeType.Image, metadata: { content: "local-preview", storageKey, mimeType: "image/png" } });
export const intent = (nodes = [video()]) => captureHNLocalIntent("_Project", nodes[0].id, nodes, nodes.slice(1).map((n) => ({ id: `edge-${n.id}`, fromNodeId: n.id, toNodeId: nodes[0].id })), business());

const bun = (globalThis as unknown as { Bun: {
    serve(options: { hostname: string; port: number; fetch: (request: Request) => Response | Promise<Response> }): { url: URL; stop(force?: boolean): void };
    write(path: string, value: string): Promise<number>;
} }).Bun;
export function localFixture() {
    const counters = { discovery: 0, shot: 0, reference: 0, prepare: 0, forbidden: 0 };
    const bodies: Record<string, unknown>[] = [];
    let responseMode = "ok";
    const server = bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(request) {
        const path = new URL(request.url).pathname;
        assert.equal(request.headers.get("authorization"), null); assert.equal(request.headers.get("cookie"), null);
        const projectId = path.split("/")[4];
        if (request.method !== "POST" || request.headers.get("X-HN-Local-Request") !== "1") { counters.forbidden++; return new Response("rejected", { status: 403 }); }
        if (path.endsWith("/shots/ensure")) { counters.shot++; const p = await request.json(); return Response.json({ code: 0, data: { shotId: `shot-${p.sourceNodeId}`, projectId, sourceNodeId: p.sourceNodeId, label: "Shot", createdAt: "2026-10-04T00:00:00Z", updatedAt: "2026-10-04T00:00:00Z" } }); }
        if (path.endsWith("/references")) {
            counters.reference++; const form = await request.formData(), blob = form.get("file") as Blob;
            return Response.json({ code: 0, data: { id: `reference-${counters.reference}`, projectId, kind: "image", sha256: await hnLocalSHA256(await blob.arrayBuffer()) } });
        }
        if (path.endsWith("/generations/prepare")) {
            counters.prepare++; const p = await request.json(); bodies.push(p);
            if (responseMode === "500") return Response.json({ code: 1 }, { status: 500 });
            if (responseMode === "400") return Response.json({ code: 1 }, { status: 400 });
            if (responseMode === "malformed") return new Response("malformed");
            return Response.json({ code: 0, data: { generationId: `generation-${counters.prepare}`, projectId, nodeId: p.nodeId, shotId: p.shotId, sourceBaseline: p.sourceBaseline, frozen: true, frozenHash: "a".repeat(64), status: "PREPARED", submissionState: "PREPARED", referenceBindings: p.referenceBindings, createdAt: "2026-10-04T00:00:00Z" } });
        }
        counters.forbidden++; return new Response("forbidden", { status: 404 });
    } });
    let bytes = "exact local bytes", reads = 0;
    const dependencies = { readBlob: async () => { reads++; return new Blob([bytes], { type: "image/png" }); }, request: (async (url, options) => {
        if (url === "/api/hn/local-endpoint") { counters.discovery++; return Response.json({ code: 0, data: { url: server.url.origin } }); }
        return fetch(url, options);
    }) as typeof fetch };
    return { counters, bodies, dependencies, setBytes: (value: string) => { bytes = value; }, setMode: (value: string) => { responseMode = value; }, reads: () => reads, close: () => server.stop(true) };
}
const evidence: unknown[] = [];
async function evidenceRow(value: unknown) {
    evidence.push(value); const path = process.env.HN_R18_FRONTEND_EVIDENCE;
    if (path) await bun.write(path, JSON.stringify(evidence, null, 2));
}
test("mapping preserves exact case, leading symbols; unsafe IDs rejected", async () => {
    for (const id of ["normal", "_first", "-first", "CON", "A".repeat(128)]) {
        const a = await mapHNCanvasProjectId(id); assert.match(a, /^canvas-[a-f0-9]{64}$/); assert.equal(a, await mapHNCanvasProjectId(id));
    }
    assert.notEqual(await mapHNCanvasProjectId("abc"), await mapHNCanvasProjectId("ABC"));
    for (const id of ["", " spaces", "../escape", "a/b", "é", "a".repeat(129)]) await assert.rejects(mapHNCanvasProjectId(id), HNLocalError);
});
test("toolbar and controller use clean empty Video eligibility", () => {
    assert.equal(canHNLocalPrepare(video()), true);
    for (const patch of [{ content: "media" }, { workflowRef: {} }, { videoTaskId: "task" }, { videoTaskVideoId: "task" }, { status: "loading" }, { status: "error" }, { status: "success" }]) {
        const n = video(); n.metadata = { ...n.metadata, ...patch } as CanvasNodeData["metadata"]; assert.equal(canHNLocalPrepare(n), false); assert.throws(() => intent([n]));
    }
    for (const id of ["", "/path", "bad\nnode", "x".repeat(129), "token=fixture"]) { const n = video(id); assert.equal(canHNLocalPrepare(n), false); }
});
test("closed scalar projection never reads synthetic secret getters; provider facts remain unbound", async () => {
    const config = business();
    for (const key of ["apiKey", "baseUrl", "localChannels", "publicChannels", "token", "channelId"]) Object.defineProperty(config, key, { enumerable: true, get() { throw new Error("forbidden config read"); } });
    const n = video(); n.metadata!.cameraControl = { enabled: true, camera: "arri-alexa", lens: "cooke-s4", focalLength: 35, aperture: 2.8 };
    const local = captureHNLocalIntent("_Project", n.id, [n], [], config);
    const frozen = await preflightHNLocalIntent(local, async () => null);
    assert.notEqual(local.promptSnapshot, n.metadata!.prompt); assert.equal(frozen.input.protocol, ""); assert.equal(frozen.input.providerIdentity, ""); assert.equal("connectionId" in frozen.input, false);
    assert.equal(Object.keys(frozen.input).some((k) => /key|channel|token/i.test(k)), false);
});
test("reference roles/order, duplicate frames, missing selections and unsafe bundles", async () => {
    const n = video(); n.metadata!.firstFrameNodeId = "first"; n.metadata!.lastFrameNodeId = "last";
    const refs = [image("ref"), image("first", "image:first"), image("last", "image:last")];
    const local = intent([n, ...refs]); assert.deepEqual(local.references.map((r) => r.role), ["reference", "firstFrame", "lastFrame"]);
    n.metadata!.lastFrameNodeId = "first"; assert.deepEqual(intent([n, ...refs]).references.filter((r) => r.storageKey === "image:first").map((r) => r.role), ["firstFrame", "lastFrame"]);
    n.metadata!.lastFrameNodeId = "missing"; assert.throws(() => intent([n, ...refs]));
    for (const bad of [image("bad", "server:remote"), { ...image("bad"), type: CanvasNodeType.Video }, { ...image("bad"), type: CanvasNodeType.Audio }, { ...image("bad"), metadata: {} }]) assert.throws(() => intent([video(), bad]));
    const element = video(); element.metadata!.klingElementList = [{ name: "element", nodeIds: ["x"] }]; assert.throws(() => intent([element]));
    const raw = video(); raw.metadata!.references = ["remote-reference"]; assert.throws(() => intent([raw]));
});
test("all Blob preflight before Shot writes and safe errors", async () => {
    const fixture = localFixture();
    try {
        for (const blob of [null, new Blob([]), new Blob([new Uint8Array(16 * 1024 * 1024 + 1)])]) {
            const controller = new HNLocalPrepareController(); await controller.prepare(() => intent([video(), image("ref")]), { ...fixture.dependencies, readBlob: async () => blob }, async () => true, () => assert.fail("no receipt"));
            assert.equal(controller.entry("_Project", "_video").error, "HN_LOCAL_REFERENCE_MISSING");
        }
        assert.equal(fixture.counters.shot, 0); assert.equal(fixture.counters.prepare, 0);
    } finally { fixture.close(); }
});
test("one server-observed POST, page-scoped duplicate guard, confirmed new attempt, reopen and receipt preservation", async () => {
    const f = localFixture(); let current = intent([video(), image("ref")]); let receipts: HNLocalPreparedReceipt[] = []; const c = new HNLocalPrepareController();
    try {
        await c.inspect(current, f.dependencies); assert.equal(f.counters.prepare, 0); assert.equal(f.counters.shot, 0);
        const commit = (r: HNLocalPreparedReceipt) => { receipts.push(r); current = { ...current, receipt: r }; };
        const first = c.prepare(() => current, f.dependencies, async () => assert.fail("first needs no confirm"), commit);
        assert.equal(c.entry("_Project", "_video").running, true);
        assert.equal(await c.prepare(() => current, f.dependencies, async () => true, commit), false); // close/reopen uses same page controller
        assert.equal(await first, true); assert.equal(f.counters.prepare, 1); assert.equal(c.entry("_Project", "_video").phase, "CURRENT");
        assert.equal(await c.prepare(() => current, f.dependencies, async () => false, commit), false); assert.equal(f.counters.prepare, 1);
        await c.prepare(() => current, f.dependencies, async () => true, commit); assert.equal(f.counters.prepare, 2);
        assert.equal(receipts[0].shotId, receipts[1].shotId); assert.notEqual(receipts[0].generationId, receipts[1].generationId);
        const reloaded = new HNLocalPrepareController(); await reloaded.inspect(current, f.dependencies); assert.equal(reloaded.entry("_Project", "_video").phase, "RELOADED_UNVERIFIED"); assert.equal(f.counters.prepare, 2);
        for (const body of f.bodies) { assert.equal(body.protocol, ""); assert.equal(body.providerIdentity, ""); assert.equal("connectionId" in body, false); }
        assert.equal(f.counters.forbidden, 0);
        await evidenceRow({ scenario: "explicit-attempts-and-reload", serverObserved: f.counters, shotIDs: receipts.map((r) => r.shotId), generationIDs: receipts.map((r) => r.generationId), reload: "RELOADED_UNVERIFIED", providerBoundCalls: 0 });
    } finally { f.close(); }
});
test("fingerprint catches all business changes and same-key bytes; ignores visual facts", async () => {
    const f = localFixture(); const nodes = [video(), image("ref")]; const original = intent(nodes); const base = await preflightHNLocalIntent(original, f.dependencies.readBlob);
    try {
        for (const [key, value] of Object.entries({ prompt: "changed", model: "other", seconds: "8", size: "9:16", vquality: "1080", mode: "pro", negativePrompt: "cloud", multiShot: "true", shotType: "customize", generateAudio: "true", watermark: "true", characterOrientation: "image" })) {
            const n = { ...nodes[0], metadata: { ...nodes[0].metadata, [key]: value } }; assert.notEqual((await preflightHNLocalIntent(intent([n, nodes[1]]), f.dependencies.readBlob)).fingerprint, base.fingerprint, key);
        }
        const visual = { ...nodes[0], title: "renamed", position: { x: 44, y: 66 } }; assert.equal((await preflightHNLocalIntent(intent([visual, nodes[1]]), f.dependencies.readBlob)).fingerprint, base.fingerprint);
        f.setBytes("changed exact bytes"); assert.notEqual((await preflightHNLocalIntent(original, f.dependencies.readBlob)).fingerprint, base.fingerprint);
        const reversed = { ...original, parameters: Object.fromEntries(Object.entries(original.parameters).reverse()) }; assert.equal((await preflightHNLocalIntent(reversed, f.dependencies.readBlob)).fingerprint, (await preflightHNLocalIntent(original, f.dependencies.readBlob)).fingerprint);
    } finally { f.close(); }
});
test("in-flight intent snapshot is immutable, current edits become STALE; cached bytes do not change", async () => {
    const f = localFixture(); let current = intent([video(), image("ref")]); const c = new HNLocalPrepareController(); let receipt: HNLocalPreparedReceipt | undefined;
    const request: typeof fetch = async (url, opts) => {
        if (typeof url === "string" && url.endsWith("/shots/ensure")) { current.promptSnapshot = "edited while waiting"; current.parameters.videoSeconds = "9"; f.setBytes("new current bytes"); }
        return f.dependencies.request(url, opts);
    };
    try {
        await c.prepare(() => current, { ...f.dependencies, request }, async () => true, (r) => { receipt = r; });
        assert.equal(f.bodies[0].promptSnapshot, "A quiet landscape"); assert.equal((f.bodies[0].parameters as Record<string, string>).videoSeconds, "4"); assert.equal(receipt?.snapshot.seconds, "4"); assert.equal(c.entry("_Project", "_video").phase, "STALE");
        current = { ...current, receipt }; const reloaded = new HNLocalPrepareController(); await reloaded.inspect(current, f.dependencies); assert.equal(reloaded.entry("_Project", "_video").phase, "STALE");
        await evidenceRow({ scenario: "edit-during-request", phase: "STALE", frozenSeconds: receipt?.snapshot.seconds, currentSeconds: current.parameters.videoSeconds, serverObserved: f.counters });
    } finally { f.close(); }
});
test("foreign/malformed receipt rejected; merge changes only own receipt, not legacy prompt/status", async () => {
    const f = localFixture(); let receipt!: HNLocalPreparedReceipt;
    try {
        await new HNLocalPrepareController().prepare(() => intent(), f.dependencies, async () => true, (r) => { receipt = r; });
        assert.equal(validHNLocalReceipt(receipt, "_Project", "_video", receipt.hnProjectId), true);
        for (const bad of [{ ...receipt, version: 2 }, { ...receipt, sourceNodeId: "other" }, { ...receipt, preparedAt: "2026-02-31T00:00:00Z" }, { ...receipt, snapshot: { ...receipt.snapshot, referenceCount: 33 } }, { ...receipt, extra: "forbidden" }]) assert.equal(validHNLocalReceipt(bad, "_Project", "_video", receipt.hnProjectId), false);
        const nodes = [video(), video("B")], merged = mergeHNLocalReceipt(nodes, receipt, "_Project"); assert.deepEqual({ ...merged[0].metadata, hnLocalPrepared: undefined }, { ...nodes[0].metadata, hnLocalPrepared: undefined }); assert.equal(merged[1], nodes[1]); assert.equal(mergeHNLocalReceipt(nodes, receipt, "other"), nodes);
    } finally { f.close(); }
});
test("deleted/replaced target and disposed page cannot receive an old receipt; switching dialog to B keeps A's identity", async () => {
    for (const invalidation of ["delete", "dispose", "switch"]) {
        const f = localFixture(); const c = new HNLocalPrepareController(); let count = 0;
        const request: typeof fetch = async (url, opts) => { const response = await f.dependencies.request(url, opts); if (typeof url === "string" && url.endsWith("/generations/prepare")) { if (invalidation === "dispose") c.dispose(); else c.syncTargets(invalidation === "switch" ? "other" : "_Project", [video("B")]); } return response; };
        try { await c.prepare(() => intent(), { ...f.dependencies, request }, async () => true, () => { count++; }); assert.equal(count, 0); } finally { f.close(); }
    }
});
test("POST ambiguity never retries or leaks raw errors; explicit rejection differs", async () => {
    for (const mode of ["500", "malformed", "drop", "400"]) {
        const f = localFixture(), c = new HNLocalPrepareController(); f.setMode(mode);
        const request: typeof fetch = async (url, opts) => { const response = await f.dependencies.request(url, opts); if (mode === "drop" && typeof url === "string" && url.endsWith("/generations/prepare")) throw new Error("private response details"); return response; };
        try {
            await c.prepare(() => intent(), { ...f.dependencies, request }, async () => true, () => assert.fail("no receipt")); assert.equal(f.counters.prepare, 1);
            const e = c.entry("_Project", "_video"); assert.equal(e.phase, mode === "400" ? "ERROR" : "OUTCOME_UNKNOWN"); assert.equal(JSON.stringify(e).includes("private response"), false);
            await c.inspect(intent(), f.dependencies); assert.equal(f.counters.prepare, 1); assert.equal(e.phase, mode === "400" ? "ERROR" : "OUTCOME_UNKNOWN");
            await evidenceRow({ scenario: mode, phase: e.phase, serverObserved: f.counters, automaticRetry: 0 });
        } finally { f.close(); }
    }
});

test("upstream text, exclusion, camera, multishot, reference order and frame identity define staleness", async () => {
    const f = localFixture();
    try {
        const n = video(), refs = [image("first"), image("last")];
        const text: CanvasNodeData = { ...video("text"), type: CanvasNodeType.Text, metadata: { content: "Upstream words" } };
        const nodes = [n, ...refs, text], edges = nodes.slice(1).map((v) => ({ id: `edge-${v.id}`, fromNodeId: v.id, toNodeId: n.id }));
        const capture = (list = nodes, connections = edges, config = business()) => captureHNLocalIntent("_Project", n.id, list, connections, config);
        const fingerprint = async (i: ReturnType<typeof capture>) => (await preflightHNLocalIntent(i, f.dependencies.readBlob)).fingerprint;
        const baseline = await fingerprint(capture());
        const changedText = { ...text, metadata: { content: "Different upstream words" } };
        assert.notEqual(await fingerprint(capture([n, ...refs, changedText])), baseline);
        const excluded = { ...n, metadata: { ...n.metadata, excludeUpstreamText: true } };
        assert.notEqual(await fingerprint(capture([excluded, ...refs, text])), baseline);
        const camera = { ...n, metadata: { ...n.metadata, cameraControl: { enabled: true, camera: "arri-alexa", lens: "cooke-s4", focalLength: 35, aperture: 2.8 } } };
        assert.notEqual(await fingerprint(capture([camera, ...refs, text])), baseline);
        const multi = business(); multi.videoMultiPrompt = [{ prompt: "One", duration: "2" }, { prompt: "Two", duration: "2" }];
        assert.notEqual(await fingerprint(capture(nodes, edges, multi)), baseline);
        assert.notEqual(await fingerprint(capture(nodes, [...edges].reverse())), baseline);
        const frame = { ...n, metadata: { ...n.metadata, firstFrameNodeId: "first", lastFrameNodeId: "last" } };
        const swapped = { ...frame, metadata: { ...frame.metadata, firstFrameNodeId: "last", lastFrameNodeId: "first" } };
        // Both selected frame nodes intentionally use the same key and same bytes.
        assert.notEqual(await fingerprint(capture([frame, ...refs, text])), await fingerprint(capture([swapped, ...refs, text])));
        await evidenceRow({ scenario: "request-defining-staleness", upstreamText: true, excludeUpstreamText: true, camera: true, multiPrompt: true, referenceOrder: true, sameByteFrameIdentity: true });
    } finally { f.close(); }
});

test("Dialog target B does not steal A receipt; deleted then recreated A stays invalidated", async () => {
    for (const mode of ["switch-dialog", "replace"]) {
        const f = localFixture(), c = new HNLocalPrepareController(); let attached: HNLocalPreparedReceipt | undefined;
        const request: typeof fetch = async (url, opts) => {
            const response = await f.dependencies.request(url, opts);
            if (typeof url === "string" && url.endsWith("/generations/prepare")) {
                if (mode === "replace") { c.syncTargets("_Project", [video("B")]); c.syncTargets("_Project", [video(), video("B")]); }
                else { c.entry("_Project", "B"); await c.inspect(intent([video("B")]), f.dependencies); }
            }
            return response;
        };
        try {
            await c.prepare(() => intent(), { ...f.dependencies, request }, async () => true, (r) => { attached = r; });
            assert.equal(c.entry("_Project", "B").receipt, undefined);
            assert.equal(attached?.sourceNodeId, mode === "replace" ? undefined : "_video");
            assert.equal(f.counters.prepare, 1);
        } finally { f.close(); }
    }
});

test("backend unavailable and reference hash mismatch reject before generation POST", async () => {
    const f = localFixture();
    try {
        for (const failure of ["endpoint", "reference", "shot-conflict"]) {
            const c = new HNLocalPrepareController();
            const request: typeof fetch = async (url, opts) => {
                if (failure === "endpoint" && url === "/api/hn/local-endpoint") return new Response("invalid local discovery", { status: 503 });
                if (failure === "shot-conflict" && typeof url === "string" && url.endsWith("/shots/ensure")) return new Response("private conflict", { status: 409 });
                const response = await f.dependencies.request(url, opts);
                if (failure === "reference" && typeof url === "string" && url.endsWith("/references")) { const data = await response.json(); data.data.sha256 = "f".repeat(64); return Response.json(data); }
                return response;
            };
            await c.prepare(() => intent([video(), image("ref")]), { ...f.dependencies, request }, async () => true, () => assert.fail("no receipt"));
            assert.equal(c.entry("_Project", "_video").error, failure === "endpoint" ? "HN_LOCAL_BACKEND_UNAVAILABLE" : failure === "shot-conflict" ? "HN_SHOT_IDENTITY_CONFLICT" : "HN_PREPARE_REJECTED");
        }
        assert.equal(f.counters.prepare, 0);
    } finally { f.close(); }
});

test("project switch before target-sync effect cannot attach via stale callback", async () => {
    const f = localFixture(), c = new HNLocalPrepareController(); let current = intent(), attachments = 0;
    const request: typeof fetch = async (url, opts) => { const response = await f.dependencies.request(url, opts); if (typeof url === "string" && url.endsWith("/generations/prepare")) current = { ...current, canvasProjectId: "AnotherProject" }; return response; };
    try { await c.prepare(() => current, { ...f.dependencies, request }, async () => true, () => { attachments++; }); assert.equal(attachments, 0); assert.equal(f.counters.prepare, 1); }
    finally { f.close(); }
});
