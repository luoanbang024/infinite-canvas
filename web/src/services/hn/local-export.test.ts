import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { exportLocalSequence, type HNExport } from "./local-export";

const item = (index = 1) => ({ exportId: "export-a", sequenceIndex: index, sequenceItemId: `item-${index}`, shotId: "shot-a", candidateId: "candidate-a", resultId: "result-a", relativePath: `exports/export-a/media/00${index}_result-a.mp4`, sha256: "a".repeat(64), byteLength: 24 });
const bundle = (): HNExport => ({ schemaVersion: 1, exportId: "export-a", projectId: "test", sequenceId: "main", bundleRelativePath: "exports/export-a", manifestJsonRelativePath: "exports/export-a/ordered-manifest.json", manifestCsvRelativePath: "exports/export-a/ordered-manifest.csv", itemCount: 2, items: [item(), item(2)] });
const input = () => ({ projectId: "test", sequenceId: "main" });
function transport(data: unknown = bundle(), endpoint = "http://127.0.0.1:8088") {
    const calls: string[] = [];
    const request = (async (url, options) => {
        calls.push(String(url)); assert.equal(options?.credentials, "omit");
        if (url === "/api/hn/local-endpoint") { assert.equal(options?.cache, "no-store"); return Response.json({ code: 0, data: { url: endpoint } }); }
        assert.equal(String(url), "http://127.0.0.1:8088/api/hn/projects/test/sequences/main/export");
        assert.equal(options?.method, "POST"); assert.equal(options?.body, "{}");
        assert.deepEqual(options?.headers, { "Content-Type": "application/json", "X-HN-Local-Request": "1" });
        return Response.json({ code: 0, data });
    }) as typeof fetch;
    return { request, calls };
}

test("export exact local command preserves repeated Result placements and snapshots input before discovery", async () => {
    const t = transport(), source = input();
    const request = (async (url, options) => { source.projectId = "later"; source.sequenceId = "later"; return t.request(url, options); }) as typeof fetch;
    const result = await exportLocalSequence(source, { request }); assert.deepEqual(result, bundle()); assert.equal(t.calls.length, 2);
    assert.equal(result.items[0].resultId, result.items[1].resultId); assert.notEqual(result.items[0].sequenceItemId, result.items[1].sequenceItemId);
});
test("unsafe input IDs reject before network and no Provider/media/editor dependencies", async () => {
    const t = transport();
    for (const id of ["", "CON", "../outside", "a/b", "C:drive", "https://invalid.example"]) for (const patch of [{ projectId: id }, { sequenceId: id }]) await assert.rejects(exportLocalSequence({ ...input(), ...patch }, t));
    assert.equal(t.calls.length, 0);
    const source = readFileSync(new URL("./local-export.ts", import.meta.url), "utf8");
    assert.doesNotMatch(source, /downloadRemoteMedia|readBlob|getBlob|createGeneration|archiveLocalResult|ensureCandidate|reorderSequence|window\.open|child_process|provider\//i);
});
test("remote/credential/path discovery rejects before POST", async () => {
    for (const endpoint of ["https://invalid.example", "http://remote.invalid", "http://user:pass@127.0.0.1", "http://127.0.0.1/path", "http://127.0.0.1?key=fixture"]) {
        const t = transport(bundle(), endpoint); await assert.rejects(exportLocalSequence(input(), t)); assert.equal(t.calls.length, 1);
    }
});
test("malformed bundle ownership/count/completion paths rejected", async () => {
    for (const patch of [{ schemaVersion: 2 }, { projectId: "foreign" }, { sequenceId: "foreign" }, { exportId: "../outside" }, { itemCount: 1 }, { items: [] }, { items: null }, { bundleRelativePath: "E:/private" }, { manifestJsonRelativePath: "exports/other/ordered-manifest.json" }, { manifestCsvRelativePath: "exports/export-a/../other.csv" }]) await assert.rejects(exportLocalSequence(input(), transport({ ...bundle(), ...patch })));
});
test("invalid order/identity/duplicate/path/hash/bytes/duration facts rejected", async () => {
    for (const patch of [{ exportId: "other" }, { sequenceIndex: 0 }, { sequenceIndex: 2 }, { sequenceItemId: "CON" }, { shotId: "" }, { candidateId: "../bad" }, { resultId: "server:bad" }, { relativePath: "exports/export-a/media/../secret.mp4" }, { relativePath: "exports/other/media/001.mp4" }, { relativePath: "exports/export-a/media/001%2fsecret.mp4" }, { relativePath: "exports/export-a/media/CON.mp4" }, { relativePath: "C:\\fixture.mp4" }, { relativePath: "https://invalid.example/video" }, { sha256: "A".repeat(64) }, { sha256: "a" }, { byteLength: 0 }, { byteLength: 1.5 }, { byteLength: Number.MAX_SAFE_INTEGER + 1 }, { duration: -1 }, { duration: "5" }]) {
        await assert.rejects(exportLocalSequence(input(), transport({ ...bundle(), items: [{ ...item(), ...patch }, item(2)] })));
    }
    for (const items of [[item(2), item()], [item(), { ...item(2), sequenceItemId: "item-1" }], [item(), { ...item(2), relativePath: item().relativePath }]]) await assert.rejects(exportLocalSequence(input(), transport({ ...bundle(), items })));
});
test("explicit repeats create distinct bundles; failed/uncertain response never automatically retried", async () => {
    let posts = 0;
    const request = (async (url) => { if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8088" } }); posts++; throw new Error("uncertain transport"); }) as typeof fetch;
    await assert.rejects(exportLocalSequence(input(), { request })); assert.equal(posts, 1);
    for (const response of [Response.json({ code: 1 }), Response.json({ code: 0, data: bundle() }, { status: 409 }), Response.json({ code: 0 })]) {
        const request = (async (url) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: "http://127.0.0.1:8088" } }) : response) as typeof fetch;
        await assert.rejects(exportLocalSequence(input(), { request }));
    }
});
