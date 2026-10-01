import assert from "node:assert/strict";
import test from "node:test";
import { freezeLocalReference, isHNProjectId, localBackendURL, logicalReferenceId, type LocalReferenceVersion } from "./local-reference";
import { POST as proxyPost } from "../../app/api/[...path]/route";

test("reference identity is stable, source/project separated, safe and unambiguous", async () => {
    const id = await logicalReferenceId("canvas-a", "image:a");
    assert.equal(id, await logicalReferenceId("canvas-a", "image:a"));
    assert.notEqual(id, await logicalReferenceId("canvas-a", "image:b"));
    assert.notEqual(id, await logicalReferenceId("canvas-b", "image:a"));
    assert.notEqual(await logicalReferenceId("ab", "c"), await logicalReferenceId("a", "bc"));
    assert.match(id, /^ref-[a-f0-9]{64}$/);
    for (const invalid of ["", "../escape", "_canvas", "CON", "Lpt1", "a:b", "a\\b"]) assert.equal(isHNProjectId(invalid), false);
});

test("only loopback HTTP backend discovery is accepted", () => {
    assert.equal(localBackendURL("http://127.0.0.1:8083"), "http://127.0.0.1:8083");
    for (const bad of ["https://remote.example", "http://127.0.0.1.evil.example", "http://user:secret@localhost", "http://localhost/path", "http://localhost/?x=1"]) assert.throws(() => localBackendURL(bad));
});

test("freeze submits exact existing Blob with custom header, no credentials and unchanged source", async () => {
    const source = Object.freeze({ projectId: "canvas-a", storageKey: "image:a" });
    const blob = new Blob([new Uint8Array([137, 80, 78, 71, 0, 42, 255])], { type: "image/png" });
    const result = { id: "version-a", projectId: source.projectId, logicalReferenceId: await logicalReferenceId(source.projectId, source.storageKey), sha256: "a".repeat(64) } as LocalReferenceVersion;
    let calls = 0;
    const request = (async (url, options) => {
        calls++;
        assert.equal(options?.credentials, "omit");
        if (calls === 1) { assert.equal(url, "/api/hn/local-endpoint"); return Response.json({ code: 0, data: { url: "http://127.0.0.1:8083" } }); }
        assert.equal(url, "http://127.0.0.1:8083/api/hn/projects/canvas-a/references");
        assert.deepEqual(options?.headers, { "X-HN-Local-Request": "1" });
        const body = options?.body as FormData;
        assert.equal(body.get("logicalReferenceId"), result.logicalReferenceId);
        assert.equal(body.get("kind"), "image");
        assert.deepEqual(new Uint8Array(await (body.get("file") as Blob).arrayBuffer()), new Uint8Array(await blob.arrayBuffer()));
        return Response.json({ code: 0, data: result });
    }) as typeof fetch;
    const frozen = await freezeLocalReference(source, { readBlob: async (key) => { assert.equal(key, "image:a"); return blob; }, request });
    assert.deepEqual(frozen, result);
    assert.equal(calls, 2);
    assert.deepEqual(source, { projectId: "canvas-a", storageKey: "image:a" });
});

test("missing, empty, remote and incompatible source fail without a write or source replacement", async () => {
    let calls = 0;
    const request = (async () => { calls++; throw new Error("unexpected network"); }) as typeof fetch;
    for (const [projectId, storageKey, blob] of [["canvas-a", "image:a", null], ["canvas-a", "image:a", new Blob()], ["canvas-a", "server:a", new Blob(["a"])], ["_canvas", "image:a", new Blob(["a"])]] as const) {
        const source = Object.freeze({ projectId, storageKey });
        await assert.rejects(freezeLocalReference(source, { readBlob: async () => blob, request }));
        assert.equal(source.storageKey, storageKey);
    }
    assert.equal(calls, 0);
});

test("controlled disabled/error response preserves original image identity and bytes", async () => {
    const source = Object.freeze({ projectId: "canvas-a", storageKey: "image:a" });
    const blob = new Blob(["unchanged"]);
    const request = (async (url) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: "http://localhost:8083" } }) : Response.json({ code: 1, msg: "HN_PROJECTS_ROOT 未配置" }, { status: 503 })) as typeof fetch;
    await assert.rejects(freezeLocalReference(source, { readBlob: async () => blob, request }), /HN_PROJECTS_ROOT/);
    assert.equal(source.storageKey, "image:a");
    assert.equal(await blob.text(), "unchanged");
});

test("generic Next proxy cannot convert HN writes into trusted loopback requests", async () => {
    const response = await proxyPost({} as Parameters<typeof proxyPost>[0], { params: Promise.resolve({ path: ["hn", "projects", "canvas-a", "references"] }) });
    assert.equal(response.status, 403);
    assert.equal((await response.json()).code, 1);
});
