import assert from "node:assert/strict";
import test from "node:test";
import { archiveLocalResult, retryLocalArchive, LocalArchiveFailure, type LocalArchiveFacts } from "./local-result-archive";
import { buildHNLocalResultArchiveInput } from "@/app/(user)/canvas/components/hn-local-result-archive";
import { CanvasNodeType, type CanvasNodeData } from "@/app/(user)/canvas/types";

const bytes = new Uint8Array([0, 0, 0, 24, 102, 116, 121, 112, 105, 115, 111, 109, 0, 0, 0, 0, 105, 115, 111, 109, 109, 112, 52, 50]);
const blob = new Blob([bytes], { type: "video/mp4" });
const input = { projectId: "local", generationId: "generation", storageKey: "video:local" };
function transport(failed = false, alter?: (facts: LocalArchiveFacts) => void) {
    const calls: string[] = [];
    const request = (async (url, init) => {
        calls.push(String(url)); assert.equal(init?.credentials, "omit");
        if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8085" } });
        assert.match(String(url), /^http:\/\/127\.0\.0\.1:8085\/api\/hn\/projects\/local\/(generations\/generation\/results\/local-archive|archive-jobs\/job\/retry)$/);
        assert.equal((init?.headers as Record<string, string>)["X-HN-Local-Request"], "1");
        const form = init?.body as FormData;
        assert.deepEqual([...form.keys()].sort(), ["file", "mimeType", "resultKind", "sha256"]);
        assert.deepEqual(new Uint8Array(await (form.get("file") as Blob).arrayBuffer()), bytes);
        const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)), (n) => n.toString(16).padStart(2, "0")).join("");
        assert.equal(form.get("sha256"), hash);
        const facts: LocalArchiveFacts = { resultId: "result", generationId: "generation", archiveJobId: "job", resultStatus: failed ? "ARCHIVE_FAILED" : "ARCHIVED", archiveStatus: failed ? "FAILED" : "ARCHIVED", archivedRelativePath: failed ? "" : "generated/generation/result/media.mp4", byteLength: failed ? 0 : bytes.length, sha256: failed ? "" : hash, mimeType: "video/mp4" };
        alter?.(facts); return Response.json({ code: failed ? 1 : 0, data: facts }, { status: failed ? 422 : 200 });
    }) as typeof fetch;
    return { calls, request, readBlob: async () => blob };
}

test("video/file local keys send exact multipart bytes/header without source mutation", async () => {
    for (const storageKey of ["video:local", "file:local"]) {
        const source = Object.freeze({ ...input, storageKey }); const dependency = transport();
        const result = await archiveLocalResult(source, dependency);
        assert.equal(result.resultStatus, "ARCHIVED"); assert.equal(source.storageKey, storageKey); assert.equal(dependency.calls.length, 2);
    }
});

test("remote/server/nonpersistent keys rejected before Blob lookup or network", async () => {
    for (const storageKey of ["server:abc", "http://invalid.example", "https://invalid.example/?sig=fixture", "blob:abc", "image:abc", "audio:abc", "video:", "file:https://invalid.example"]) {
        await assert.rejects(archiveLocalResult({ ...input, storageKey }, { readBlob: async () => { throw new Error("must not lookup"); }, request: (async () => { throw new Error("must not request"); }) as typeof fetch }));
    }
});

test("missing/empty/nonvideo Blob rejected before a write", async () => {
    for (const value of [null, new Blob([], { type: "video/mp4" }), new Blob([bytes], { type: "audio/mpeg" }), new Blob([bytes], { type: "image/png" })]) {
        const dependency = transport(); await assert.rejects(archiveLocalResult(input, { ...dependency, readBlob: async () => value })); assert.deepEqual(dependency.calls, []);
    }
});

test("failed archive exposes same job ID; explicit retry calls only that job endpoint", async () => {
    let job = "";
    try { await archiveLocalResult(input, transport(true)); assert.fail("failure expected"); } catch (error) { assert.ok(error instanceof LocalArchiveFailure); job = error.archive.archiveJobId; }
    const dependency = transport(); const retry = await retryLocalArchive({ projectId: "local", archiveJobId: job, storageKey: input.storageKey }, dependency);
    assert.equal(retry.archiveJobId, job); assert.equal(retry.resultId, "result"); assert.ok(dependency.calls[1].endsWith("/archive-jobs/job/retry"));
    assert.equal(dependency.calls.some((url) => url.includes("prepare") || url.includes("local-archive")), false);
});

test("archive response rejects wrong identity/hash/size/path/status", async () => {
    for (const alter of [(v: LocalArchiveFacts) => { v.generationId = "wrong"; }, (v: LocalArchiveFacts) => { v.sha256 = "0".repeat(64); }, (v: LocalArchiveFacts) => { v.byteLength++; }, (v: LocalArchiveFacts) => { v.archivedRelativePath = "C:/private/video.mp4"; }, (v: LocalArchiveFacts) => { v.archiveStatus = "FAILED"; }]) await assert.rejects(archiveLocalResult(input, transport(false, alter)));
});

test("Canvas helper explicitly attaches bytes, retains node/media identity and requires Generation", () => {
    const node: CanvasNodeData = { id: "video-node", title: "Imported local video", type: CanvasNodeType.Video, position: { x: 0, y: 0 }, width: 100, height: 100, metadata: { status: "success", content: "blob:local", storageKey: "file:local" } };
    const before = JSON.stringify(node); assert.deepEqual(buildHNLocalResultArchiveInput("local", "generation", node), { ...input, storageKey: "file:local" }); assert.equal(JSON.stringify(node), before);
    assert.throws(() => buildHNLocalResultArchiveInput("local", "", node));
    for (const bad of [{ ...node, type: CanvasNodeType.Image }, { ...node, metadata: { ...node.metadata, status: "loading" as const } }, { ...node, metadata: { ...node.metadata, storageKey: "server:abc" } }]) assert.throws(() => buildHNLocalResultArchiveInput("local", "generation", bad));
});
