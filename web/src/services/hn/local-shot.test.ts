import assert from "node:assert/strict";
import test from "node:test";
import { ensureLocalShot, type HNShot } from "./local-shot";
import { prepareLocalShotGeneration, HN_GENERATION_SOURCE_BASELINE, type HNGenerationPrepareInput } from "./local-generation";
import { prepareHNShotVideoGeneration } from "@/app/(user)/canvas/components/hn-shot-generation-prepare";
import { defaultConfig } from "@/stores/use-config-store";
import type { NodeGenerationContext } from "@/app/(user)/canvas/components/canvas-node-generation";

const context = (): NodeGenerationContext => ({ prompt: "synthetic R6 intent", referenceImages: [], firstFrame: null, lastFrame: null, referenceVideos: [], referenceAudios: [], videoMultiPrompt: [], videoElementList: [], textCount: 0, imageCount: 0, videoCount: 0, audioCount: 0 });
const config = () => ({ ...defaultConfig, model: "synthetic-video", apiKey: "SYNTHETIC_NEVER_SERIALIZE", baseUrl: "https://invalid.example/?sig=fixture" });
const input = (): HNGenerationPrepareInput & { shotId: string } => ({ projectId: "test", nodeId: "video", shotId: "shot-1", promptSnapshot: "synthetic", sourceBaseline: HN_GENERATION_SOURCE_BASELINE, parameters: { videoSeconds: "6" }, references: [] });
const timestamp = "2026-10-02T00:00:00Z";
function transport() {
    const calls: string[] = [], payloads: Record<string, unknown>[] = [], shots = new Map<string, HNShot>();
    let generations = 0, versions = 0;
    const request = (async (url, options) => {
        calls.push(String(url)); assert.equal(options?.credentials, "omit");
        if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8086" } });
        assert.equal((options?.headers as Record<string, string>)["X-HN-Local-Request"], "1");
        assert.ok(String(url).startsWith("http://127.0.0.1:8086/api/hn/projects/test/"));
        if (String(url).endsWith("/references")) {
            assert.equal(await ((options?.body as FormData).get("file") as Blob).text(), "exact image");
            return Response.json({ code: 0, data: { id: `version-${++versions}`, projectId: "test", kind: "image", sha256: String(versions).repeat(64) } });
        }
        const body = JSON.parse(options?.body as string); payloads.push(body);
        if (String(url).endsWith("/shots/ensure")) {
            if (!shots.has(body.sourceNodeId)) shots.set(body.sourceNodeId, { shotId: `shot-${shots.size + 1}`, projectId: "test", sourceNodeId: body.sourceNodeId, label: body.label || "Shot", createdAt: timestamp, updatedAt: timestamp });
            return Response.json({ code: 0, data: shots.get(body.sourceNodeId) });
        }
        assert.ok(String(url).endsWith("/generations/prepare")); // No Provider/submit/result/editor endpoint permitted.
        return Response.json({ code: 0, data: { generationId: `generation-${++generations}`, projectId: "test", nodeId: body.nodeId, shotId: body.shotId, sourceBaseline: body.sourceBaseline, referenceBindings: body.referenceBindings, frozen: true, frozenHash: "a".repeat(64), status: "PREPARED", submissionState: "PREPARED", createdAt: timestamp } });
    }) as typeof fetch;
    return { calls, payloads, shots, request, readBlob: async () => new Blob(["exact image"], { type: "image/png" }) };
}

test("ensure uses local HN header/credentials and exact stable node identity; label changes do not rename", async () => {
    const t = transport(); const a = await ensureLocalShot({ projectId: "test", sourceNodeId: "node/relative:1", label: "Initial" }, t);
    const b = await ensureLocalShot({ projectId: "test", sourceNodeId: "node/relative:1", label: "Changed" }, t);
    assert.deepEqual(a, b); assert.equal(a.label, "Initial");
    const c = await ensureLocalShot({ projectId: "test", sourceNodeId: "Node/relative:1" }, t);
    assert.notEqual(c.shotId, a.shotId); assert.equal(c.label, "Shot");
});

test("invalid source/project/label rejected before network; exact Unicode identity is bounded in bytes", async () => {
    const t = transport();
    for (const patch of [{ sourceNodeId: "" }, { sourceNodeId: " " }, { sourceNodeId: "https://invalid.example/" }, { sourceNodeId: "blob:fixture" }, { sourceNodeId: "token=fixture" }, { sourceNodeId: "C:\\fixture" }, { sourceNodeId: "a".repeat(129) }, { sourceNodeId: "镜".repeat(43) }, { sourceNodeId: "\ud800" }, { projectId: "../escape" }, { label: "Bearer fixture" }, { label: "a".repeat(257) }]) await assert.rejects(ensureLocalShot({ projectId: "test", sourceNodeId: "node", ...patch }, t));
    assert.deepEqual(t.calls, []);
});

test("Shot malformed response and disabled/conflict/remote discovery are controlled failures", async () => {
    const good: HNShot = { shotId: "shot-a", projectId: "test", sourceNodeId: "node", label: "Initial", createdAt: timestamp, updatedAt: timestamp };
    for (const patch of [{ shotId: "../invalid" }, { projectId: "foreign" }, { sourceNodeId: "other" }, { label: "token=fixture" }, { createdAt: "invalid" }, { updatedAt: undefined }]) {
        const request = (async (url) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: "http://127.0.0.1:8086" } }) : Response.json({ code: 0, data: { ...good, ...patch } })) as typeof fetch;
        await assert.rejects(ensureLocalShot({ projectId: "test", sourceNodeId: "node" }, { request }));
    }
    for (const status of [409, 503]) {
        const request = (async (url) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: "http://127.0.0.1:8086" } }) : Response.json({ code: 1, msg: status === 409 ? "SHOT_IDENTITY_CONFLICT" : "disabled" }, { status })) as typeof fetch;
        await assert.rejects(ensureLocalShot({ projectId: "test", sourceNodeId: "node" }, { request }));
    }
    await assert.rejects(ensureLocalShot({ projectId: "test", sourceNodeId: "node" }, { request: (async () => Response.json({ code: 0, data: { url: "https://invalid.example/" } })) as typeof fetch }));
});

test("ensure snapshots source before asynchronous discovery", async () => {
    const t = transport(); const source = { projectId: "test", sourceNodeId: "original", label: "Initial" };
    const request = (async (url, opts) => { if (url === "/api/hn/local-endpoint") source.sourceNodeId = "changed"; return t.request(url, opts); }) as typeof fetch;
    const s = await ensureLocalShot(source, { request }); assert.equal(s.sourceNodeId, "original");
});

test("Shot-aware prepare requires Shot ID and validates returned binding; current baseline exact", async () => {
    const t = transport();
    for (const shotId of ["", "../invalid", undefined]) await assert.rejects(prepareLocalShotGeneration({ ...input(), shotId } as ReturnType<typeof input>, t));
    assert.deepEqual(t.calls, []);
    const source = input();
    const request = (async (url, opts) => { source.shotId = "later-edit"; return t.request(url, opts); }) as typeof fetch;
    const g = await prepareLocalShotGeneration(source, { ...t, request }); assert.equal(g.shotId, "shot-1");
    assert.equal(t.payloads[0].shotId, "shot-1"); assert.equal(g.sourceBaseline, "16047f46e2186373ea824e12e84ae8dfa2ccde32");
    const wrong = (async (url, opts) => { const response = await t.request(url, opts); if (String(url).endsWith("/prepare")) { const data = await response.json(); data.data.shotId = "other"; return Response.json(data); } return response; }) as typeof fetch;
    await assert.rejects(prepareLocalShotGeneration(input(), { ...t, request: wrong }));
    await assert.rejects(prepareLocalShotGeneration({ ...input(), sourceBaseline: "418ffbde3dbea33d374356588cb672336ec38353" }, t));
});

test("Canvas orchestration ensures Shot before freeze, keeps R4 projection and node unchanged, new attempts same Shot", async () => {
    const t = transport(); const node = Object.freeze({ id: "video", title: "Initial" }); const ctx = context();
    const image = (storageKey: string) => ({ id: storageKey, name: "image", type: "image/png", dataUrl: "data:image/png;base64,fixture", url: "https://invalid.example/", storageKey });
    ctx.referenceImages = [image("image:reference")]; ctx.firstFrame = image("image:first"); ctx.lastFrame = image("image:last");
    const cfg = config(); const before = JSON.stringify({ node, ctx, cfg });
    const a = await prepareHNShotVideoGeneration("test", node, cfg, ctx, t);
    const b = await prepareHNShotVideoGeneration("test", { ...node, title: "Changed" }, cfg, ctx, t);
    assert.equal(JSON.stringify({ node, ctx, cfg }), before); assert.notEqual(a.generationId, b.generationId); assert.equal(a.shotId, b.shotId);
    assert.equal(t.shots.size, 1); assert.equal(t.shots.get("video")?.label, "Initial");
    assert.deepEqual(a.referenceBindings.map((r) => r.role), ["reference", "firstFrame", "lastFrame"]);
    assert.ok(t.calls.indexOf("http://127.0.0.1:8086/api/hn/projects/test/shots/ensure") < t.calls.indexOf("http://127.0.0.1:8086/api/hn/projects/test/generations/prepare"));
    for (const forbidden of ["SYNTHETIC_NEVER_SERIALIZE", "apiKey", "baseUrl", "invalid.example", "localChannels"]) assert.equal(JSON.stringify(t.payloads).includes(forbidden), false);
});

test("R4 unsupported media/workflow projection remains rejected before ensure", async () => {
    const t = transport();
    for (const ctx of [{ ...context(), referenceVideos: [{}] }, { ...context(), referenceAudios: [{}] }, { ...context(), firstFrame: { storageKey: "server:a" } }]) await assert.rejects(prepareHNShotVideoGeneration("test", { id: "video" }, config(), ctx as NodeGenerationContext, t));
    await assert.rejects(prepareHNShotVideoGeneration("test", { id: "video" }, { ...config(), videoWorkflowRef: {} } as ReturnType<typeof config>, context(), t));
    assert.deepEqual(t.calls, []);
});
