import assert from "node:assert/strict";
import test from "node:test";
import { defaultConfig } from "@/stores/use-config-store";
import type { NodeGenerationContext } from "@/app/(user)/canvas/components/canvas-node-generation";
import { buildHNVideoGenerationPrepareInput } from "@/app/(user)/canvas/components/hn-video-generation-prepare";
import { applyCameraPrompt } from "@/app/(user)/canvas/utils/canvas-camera";
import { canonicalHNParameters, prepareLocalGeneration, HN_GENERATION_SOURCE_BASELINE, type HNGenerationPrepareInput, type HNReferenceBinding } from "./local-generation";

const context = (): NodeGenerationContext => ({ prompt: "synthetic camera move", referenceImages: [], firstFrame: null, lastFrame: null, referenceVideos: [], referenceAudios: [], videoMultiPrompt: [], videoElementList: [], textCount: 0, imageCount: 0, videoCount: 0, audioCount: 0 });
const config = () => ({ ...defaultConfig, model: "synthetic-video", apiKey: "SYNTHETIC_CREDENTIAL_DO_NOT_SERIALIZE", baseUrl: "https://invalid.example/?sig=fixture" });
const input = () => buildHNVideoGenerationPrepareInput("canvas-test", "_video", config(), context());
const image = (storageKey?: string) => ({ id: "image-node", name: "synthetic", type: "image/png", dataUrl: "data:image/png;base64,fixture", url: "https://invalid.example/?sig=fixture", storageKey });

function mockTransport() {
    let versions = 0;
    const calls: string[] = [];
    const payloads: Record<string, unknown>[] = [];
    const request = (async (url, options) => {
        calls.push(String(url)); assert.equal(options?.credentials, "omit");
        if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8084" } });
        assert.ok(String(url).startsWith("http://127.0.0.1:8084/api/hn/projects/canvas-test/"));
        assert.equal((options?.headers as Record<string, string>)["X-HN-Local-Request"], "1");
        if (String(url).endsWith("/references")) {
            assert.equal(await ((options?.body as FormData).get("file") as Blob).text(), "synthetic exact image bytes");
            return Response.json({ code: 0, data: { id: `version-${++versions}`, projectId: "canvas-test", kind: "image", sha256: String(versions).repeat(64) } });
        }
        assert.equal(String(url), "http://127.0.0.1:8084/api/hn/projects/canvas-test/generations/prepare");
        const payload = JSON.parse(options?.body as string); payloads.push(payload);
        return Response.json({ code: 0, data: { generationId: `generation-${payloads.length}`, projectId: "canvas-test", nodeId: payload.nodeId, sourceBaseline: HN_GENERATION_SOURCE_BASELINE, frozen: true, frozenHash: "a".repeat(64), status: "PREPARED", submissionState: "PREPARED", referenceBindings: payload.referenceBindings, createdAt: "2026-10-01T00:00:00Z" } });
    }) as typeof fetch;
    return { calls, payloads, request, readBlob: async () => new Blob(["synthetic exact image bytes"], { type: "image/png" }) };
}

test("business parameters canonicalize deterministically; config credentials/URLs never serialized", () => {
    assert.equal(JSON.stringify(canonicalHNParameters({ videoSeconds: "6", size: "16:9" })), JSON.stringify(canonicalHNParameters({ size: "16:9", videoSeconds: "6" })));
    const result = input(); const serialized = JSON.stringify(result);
    for (const forbidden of ["SYNTHETIC_CREDENTIAL", "baseUrl", "apiKey", "localChannels", "invalid.example", "credentialRef"]) assert.equal(serialized.includes(forbidden), false);
    assert.equal(result.sourceBaseline, HN_GENERATION_SOURCE_BASELINE);
    assert.equal(result.model, "synthetic-video");
    assert.deepEqual(result.references, []);
    const camera = { enabled: true, camera: "synthetic-camera", lens: "synthetic-lens", focalLength: 35, aperture: 2.8 };
    const ctx = { ...context(), prompt: "  synthetic shot  " };
    assert.equal(buildHNVideoGenerationPrepareInput("canvas-test", "video", config(), ctx, camera).promptSnapshot, applyCameraPrompt(ctx.prompt.trim(), camera));
});

test("T2V uses explicit HN header and new attempts without node or input mutation", async () => {
    const source = Object.freeze(input()); const before = JSON.stringify(source); const transport = mockTransport();
    const a = await prepareLocalGeneration(source, transport); const b = await prepareLocalGeneration(source, transport);
    assert.notEqual(a.generationId, b.generationId); assert.deepEqual(a.referenceBindings, []);
    assert.equal(JSON.stringify(source), before); assert.equal(transport.calls.filter((url) => url.endsWith("/references")).length, 0);
});

test("local reference / firstFrame / lastFrame invoke R3 exact freeze and bind returned ID/hash in order", async () => {
    const ctx = context(); ctx.referenceImages = [image("image:ref")]; ctx.firstFrame = image("image:first"); ctx.lastFrame = image("image:last");
    const before = JSON.stringify(ctx); const source = buildHNVideoGenerationPrepareInput("canvas-test", "_video", config(), ctx); const transport = mockTransport();
    const result = await prepareLocalGeneration(source, transport);
    assert.deepEqual(result.referenceBindings, ["reference", "firstFrame", "lastFrame"].map((role, i) => ({ referenceVersionId: `version-${i + 1}`, sha256: String(i + 1).repeat(64), role })));
    assert.equal(JSON.stringify(ctx), before);
    assert.equal(JSON.stringify(transport.payloads).includes("invalid.example"), false);
    assert.equal(JSON.stringify(transport.payloads).includes("storageKey"), false);
});

test("unsupported Canvas video/audio/remote/workflow/element bundles fail locally", () => {
    const cases = [() => ({ ...context(), referenceVideos: [{}] }), () => ({ ...context(), referenceAudios: [{}] }), () => ({ ...context(), referenceImages: [image("server:a")] }), () => ({ ...context(), firstFrame: image() }), () => ({ ...context(), videoElementList: [{ name: "element", description: "", references: [] }] })];
    for (const build of cases) assert.throws(() => buildHNVideoGenerationPrepareInput("canvas-test", "video", config(), build() as NodeGenerationContext));
    assert.throws(() => buildHNVideoGenerationPrepareInput("canvas-test", "video", { ...config(), videoWorkflowRef: {} } as ReturnType<typeof config>, context()));
});

test("unsafe input, missing Blob and unsupported roles fail before any write", async () => {
    const transport = mockTransport();
    for (const change of [{ projectId: "../escape" }, { sourceBaseline: "852fd2128770136d037f92dfef2655dd3d6ac1d5" }, { promptSnapshot: " " }, { parameters: { apiKey: "fixture" } }, { parameters: { videoNegativePrompt: "Bearer fixture" } }, { references: [{ storageKey: "audio:a", role: "reference" }] }, { references: [{ storageKey: "image:a", role: "video" }] }]) {
        await assert.rejects(prepareLocalGeneration({ ...input(), ...change } as HNGenerationPrepareInput, transport));
    }
    const withImage = { ...input(), references: [{ storageKey: "image:a", role: "reference" as const }] };
    for (const blob of [null, new Blob()]) await assert.rejects(prepareLocalGeneration(withImage, { ...transport, readBlob: async () => blob }));
    assert.deepEqual(transport.calls, []);
});

test("parameter schema rejects broad/nested secret fields, nonfinite types and signed URL literals", () => {
    for (const bad of [[], null, { size: 5 }, { token: "fixture" }, { videoMultiPrompt: [{ prompt: "ok", duration: "1", apiKey: "fixture" }] }, { videoMultiPrompt: [{ prompt: "see https://invalid.example/?sig=fixture", duration: "1" }] }]) assert.throws(() => canonicalHNParameters(bad as unknown as HNGenerationPrepareInput["parameters"]));
});

test("prepare snapshots request facts before asynchronous Blob lookup", async () => {
    const source = { ...input(), references: [{ storageKey: "image:first", role: "firstFrame" as const }] }; const transport = mockTransport();
    const result = await prepareLocalGeneration(source, { ...transport, readBlob: async () => { source.promptSnapshot = "later edit"; source.references[0].storageKey = "image:changed"; return transport.readBlob(); } });
    assert.equal(transport.payloads[0].promptSnapshot, "synthetic camera move"); assert.equal(result.referenceBindings[0].role, "firstFrame");
});

test("disabled API and invalid metadata are controlled errors", async () => {
    const transport = mockTransport();
    for (const data of [null, { frozen: false }, { ...await prepareLocalGeneration(input(), transport), referenceBindings: [{ referenceVersionId: "wrong", sha256: "a".repeat(64), role: "reference" } as HNReferenceBinding] }]) {
        const request = (async (url, opts) => url === "/api/hn/local-endpoint" ? transport.request(url, opts) : Response.json({ code: data ? 0 : 1, data, msg: "HN disabled" }, { status: data ? 200 : 503 })) as typeof fetch;
        await assert.rejects(prepareLocalGeneration(input(), { ...transport, request }));
    }
});
