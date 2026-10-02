import assert from "node:assert/strict";
import test from "node:test";
import { ensureCandidate, selectCandidate, addSequenceItem, commitArchivedResultToSequence, type HNCandidate, type HNSequenceItem } from "./local-editorial";

const timestamp = "2026-10-02T00:00:00Z";
const candidate = (): HNCandidate => ({ candidateId: "candidate-a", projectId: "test", shotId: "shot-a", generationId: "generation-a", resultId: "result-a", label: "Candidate", availabilityStatus: "ARCHIVED", createdAt: timestamp, updatedAt: timestamp });
const item = (): HNSequenceItem => ({ sequenceItemId: "item-a", projectId: "test", sequenceId: "main", orderIndex: 0, shotId: "shot-a", candidateId: "candidate-a", resultId: "result-a", createdAt: timestamp, updatedAt: timestamp });
const input = () => ({ projectId: "test", shotId: "shot-a", resultId: "result-a" });
function transport() {
    const calls: string[] = [], payloads: object[] = []; let selected = "", placements = 0;
    const request = (async (url, options) => {
        assert.equal(options?.credentials, "omit");
        if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8087" } });
        const path = String(url); calls.push(path); assert.ok(path.startsWith("http://127.0.0.1:8087/api/hn/projects/test/"));
        assert.equal(options?.method, "POST"); assert.equal((options?.headers as Record<string, string>)["X-HN-Local-Request"], "1");
        const body = JSON.parse(options?.body as string); payloads.push(body);
        if (path.endsWith("/ensure")) return Response.json({ code: 0, data: candidate() });
        if (path.endsWith("/select")) { assert.deepEqual(body, {}); selected = "candidate-a"; return Response.json({ code: 0, data: { shotId: "shot-a", selectedCandidateId: selected } }); }
        assert.ok(path.endsWith("/items")); assert.equal(selected, "candidate-a");
        const sequenceId = path.split("/sequences/")[1].split("/")[0];
        return Response.json({ code: 0, data: { ...item(), sequenceId, sequenceItemId: `item-${++placements}`, orderIndex: placements - 1 } });
    }) as typeof fetch;
    return { request, calls, payloads, selection: () => selected };
}

test("Candidate ensure does not select/add; local endpoint/header/credentials and exact payload", async () => {
    const t = transport(); const c = await ensureCandidate({ ...input(), label: "A", extra: "not projected" } as ReturnType<typeof input>, t);
    assert.equal(c.candidateId, "candidate-a"); assert.equal(t.selection(), ""); assert.equal(t.calls.length, 1); assert.deepEqual(t.payloads, [{ resultId: "result-a", label: "A" }]);
    await selectCandidate({ projectId: "test", shotId: "shot-a", candidateId: c.candidateId }, t); assert.equal(t.calls.length, 2);
    const a = await addSequenceItem({ projectId: "test", sequenceId: "main", candidate: c }, t), b = await addSequenceItem({ projectId: "test", sequenceId: "main", candidate: c }, t);
    assert.notEqual(a.sequenceItemId, b.sequenceItemId); assert.deepEqual(t.payloads[2], { candidateId: c.candidateId });
});

test("explicit orchestration ensure -> select -> add; snapshots input and leaves caller Canvas facts unchanged", async () => {
    const t = transport(); const source = { ...input(), candidateLabel: "A", sequenceId: "custom" }; const before = JSON.stringify(source);
    await commitArchivedResultToSequence(source, t); assert.equal(JSON.stringify(source), before);
    assert.deepEqual(t.calls.map((s) => s.split("/").at(-1)), ["ensure", "select", "items"]);
    assert.ok(t.calls[2].includes("/sequences/custom/"));
    const request = (async (url, opts) => { source.shotId = "later"; source.resultId = "later"; source.sequenceId = "later"; return t.request(url, opts); }) as typeof fetch;
    const facts = await commitArchivedResultToSequence(source, { request }); assert.equal(facts.candidate.shotId, "shot-a"); assert.equal(facts.item.sequenceId, "custom");
    const def = await commitArchivedResultToSequence(input(), t); assert.equal(def.item.sequenceId, "main");
});

test("unsafe input/sequence/labels rejected before writes; no generation/media/Provider dependencies", async () => {
    const t = transport();
    for (const patch of [{ projectId: "../escape" }, { shotId: "" }, { resultId: "https://invalid.example" }, { candidateLabel: "C:\\fixture" }, { candidateLabel: "token=fixture" }, { candidateLabel: "镜".repeat(86) }, { candidateLabel: "\ud800" }, { sequenceId: "../escape" }, { sequenceId: "CON" }]) await assert.rejects(commitArchivedResultToSequence({ ...input(), ...patch }, t));
    await assert.rejects(selectCandidate({ projectId: "test", shotId: "shot-a", candidateId: "../invalid" }, t));
    await assert.rejects(addSequenceItem({ projectId: "test", sequenceId: "main", candidate: { ...candidate(), projectId: "foreign" } }, t));
    assert.deepEqual(t.calls, []);
});

test("malformed Candidate ownership/status/metadata/implicit selection rejected; failure never proceeds", async () => {
    for (const patch of [{ candidateId: "../bad" }, { generationId: "" }, { projectId: "foreign" }, { shotId: "foreign" }, { resultId: "foreign" }, { availabilityStatus: "RECEIVED" }, { label: "https://invalid.example" }, { createdAt: "invalid" }, { updatedAt: undefined }, { selectedCandidateId: "candidate-a" }]) {
        let calls = 0;
        const request = (async (url) => { if (url === "/api/hn/local-endpoint") return Response.json({ code: 0, data: { url: "http://127.0.0.1:8087" } }); calls++; return Response.json({ code: 0, data: { ...candidate(), ...patch } }); }) as typeof fetch;
        await assert.rejects(commitArchivedResultToSequence(input(), { request })); assert.equal(calls, 1);
    }
});

test("malformed selection/item ownership rejected and remote discovery never posted", async () => {
    for (const patch of [{ selectedCandidateId: "other" }, { shotId: "other" }]) {
        const t = transport(); const request = (async (url, opts) => String(url).endsWith("/select") ? Response.json({ code: 0, data: { shotId: "shot-a", selectedCandidateId: "candidate-a", ...patch } }) : t.request(url, opts)) as typeof fetch;
        await assert.rejects(commitArchivedResultToSequence(input(), { request })); assert.equal(t.calls.length, 1);
    }
    for (const patch of [{ sequenceItemId: "../bad" }, { projectId: "foreign" }, { sequenceId: "foreign" }, { shotId: "foreign" }, { candidateId: "foreign" }, { resultId: "foreign" }, { orderIndex: -1 }, { orderIndex: 0.5 }, { updatedAt: undefined }]) {
        const request = (async (url) => url === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: "http://127.0.0.1:8087" } }) : Response.json({ code: 0, data: { ...item(), ...patch } })) as typeof fetch;
        await assert.rejects(addSequenceItem({ projectId: "test", sequenceId: "main", candidate: candidate() }, { request }));
    }
    let calls = 0; const request = (async () => { calls++; return Response.json({ code: 0, data: { url: "https://invalid.example" } }); }) as typeof fetch;
    await assert.rejects(ensureCandidate(input(), { request })); assert.equal(calls, 1);
});
