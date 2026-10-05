import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { CanvasHNLocalArchiveDialog, runHNArchiveIfCandidateIdle } from "./canvas-hn-local-archive-dialog";
import { HNLocalArchiveController } from "./hn-local-canvas-archive";
import { HNLocalCandidateController, mergeHNLocalCandidateReceipt } from "./hn-local-canvas-candidate";
import { candidateFixture } from "./hn-local-canvas-candidate.test";

test("R22 actual page wiring is page-owned, existing toolbar-only, no Generate/Blob/Provider edge", async () => {
    const page = await readFile(new URL("../[id]/canvas-client-page.tsx", import.meta.url), "utf8"), dialog = await readFile(new URL("./canvas-hn-local-archive-dialog.tsx", import.meta.url), "utf8"), module = await readFile(new URL("./hn-local-canvas-candidate.ts", import.meta.url), "utf8");
    assert.ok(page.includes("useRef<HNLocalCandidateController")); assert.ok(page.includes("new HNLocalCandidateController")); assert.ok(page.includes("toolbarNode.metadata?.hnLocalArchive || toolbarNode.metadata?.hnLocalCandidate")); assert.ok(page.includes("onLocalArchive={(node) => setLocalArchiveNodeId(node.id)}"));
    assert.ok(page.includes("metadata: { ...node.metadata, ...videoMetadata(video), errorDetails: undefined }")); assert.ok(page.includes("mergeHNLocalCandidateReceipt(current, receipt, localArchiveSnapshotRef.current.projectId, archive)"));
    assert.ok(module.includes('import { ensureCandidate } from "@/services/hn/local-editorial"')); assert.equal(/createVideoGenerationTask|submitWorkflowTask|SubmitFrozenGeneration|SubmitMiniMax|PollMiniMax|ArchiveMiniMax|downloadRemoteMedia|uploadMediaFile|autoSyncToCloud|useConfigStore|useUserStore|selectCandidate|addSequenceItem|reorderSequence|commitArchivedResultToSequence|exportLocalSequence|\.readBlob\(/.test(module), false);
    assert.ok(dialog.includes('title="归档本地视频"')); assert.ok(dialog.includes("runHNArchiveIfCandidateIdle")); assert.ok(dialog.includes("candidate?.controller"));
    assert.ok(dialog.includes("hnLocalCandidate: _candidate")); assert.ok(dialog.includes("dependencies, archiveRevision"));
});

test("R22 Candidate-only safe merge and ordinary Canvas roundtrip preserve archive/prepared/media/status", async () => {
    const f = await candidateFixture(), original = structuredClone(f.current().node); await f.run(); const r = f.entry().receipt!;
    const merged = mergeHNLocalCandidateReceipt([original], r, f.project, f.receipt)[0];
    assert.deepEqual({ ...merged.metadata, hnLocalCandidate: undefined }, { ...original.metadata, hnLocalCandidate: undefined }); assert.equal(merged.metadata?.status, "success");
    assert.equal(mergeHNLocalCandidateReceipt([original], r, "other", f.receipt)[0], original);
    assert.equal(mergeHNLocalCandidateReceipt([{ ...original, metadata: { ...original.metadata, hnLocalPrepared: { ...f.prepared, generationId: "other" } } }], r, f.project, f.receipt)[0].metadata?.hnLocalCandidate, undefined);
    const pending = { ...original, metadata: { ...original.metadata, hnLocalArchive: { ...f.receipt, resultId: "new-result" } } }; assert.equal(mergeHNLocalCandidateReceipt([pending], r, f.project, f.receipt)[0], pending);
    const roundtrip = JSON.parse(JSON.stringify(merged)); f.setTarget({ ...f.current(), node: roundtrip }); const reload = new HNLocalCandidateController(); await reload.inspect(f.current, new HNLocalArchiveController(), f.d); assert.equal(reload.entry(f.project, f.nodeId).phase, "CANDIDATE_RELOADED_UNVERIFIED");
});

test("R22 Dialog mount/preview is zero HTTP; metadata marker is never write authority", async () => {
    const f = await candidateFixture(); const before = structuredClone(f.counts);
    renderToStaticMarkup(createElement(CanvasHNLocalArchiveDialog, { open: true, canvasProjectId: f.project, nodeId: f.nodeId, controller: f.archive, readTarget: () => f.current(), dependencies: { journal: f.journal, readBlob: async () => assert.fail("SSR must not read Blob") }, onReceipt: () => assert.fail("mount must not archive"), onClose: () => {}, targetRevision: f.current(), candidate: { controller: f.c, dependencies: f.d, onReceipt: f.receive } }));
    await f.inspect(); assert.deepEqual(f.counts, before);
    f.setTarget({ ...f.current(), node: { ...f.current().node, metadata: { hnLocalCandidate: { invalid: true } as never } } }); assert.equal(await f.run(), false); assert.equal(f.counts.post, 0);
});

test("R22 production archive-entry interlock blocks both simultaneous orders during confirmation/reopen", async () => {
    const f = await candidateFixture(); let releaseCandidate!: (ok: boolean) => void;
    const pendingCandidate = f.c.ensure(f.current, f.archive, f.d, () => new Promise<boolean>((r) => { releaseCandidate = r; }), f.receive);
    let archiveActions = 0;
    assert.equal(runHNArchiveIfCandidateIdle(f.c, f.project, f.nodeId, () => { archiveActions++; }), false); await f.inspect(); assert.equal(runHNArchiveIfCandidateIdle(f.c, f.project, f.nodeId, () => { archiveActions++; }), false); assert.equal(archiveActions, 0);
    while (!releaseCandidate) await new Promise((r) => setTimeout(r, 1)); releaseCandidate(false); assert.equal(await pendingCandidate, false);
    const f2 = await candidateFixture(); let releaseArchive!: (ok: boolean) => void; let archivePromise!: Promise<boolean>;
    const mp4 = new Uint8Array([0, 0, 0, 24, 102, 116, 121, 112, 105, 115, 111, 109, 0, 0, 0, 0, 105, 115, 111, 109, 109, 112, 52, 50]);
    assert.equal(runHNArchiveIfCandidateIdle(f2.c, f2.project, f2.nodeId, () => {
        archivePromise = f2.archive.archive(f2.current, { journal: f2.journal, readBlob: async () => new Blob([mp4], { type: "video/mp4" }), request: async () => assert.fail("cancelled archive cannot POST") }, () => new Promise<boolean>((r) => { releaseArchive = r; }), () => assert.fail("cancelled archive has no receipt"));
    }), true);
    assert.equal(f2.archive.entry(f2.project, f2.nodeId).running, true); assert.equal(await f2.run(), false); await f2.inspect(); assert.equal(await f2.run(), false); assert.equal(f2.counts.post, 0);
    while (!releaseArchive) await new Promise((r) => setTimeout(r, 1)); releaseArchive(false); assert.equal(await archivePromise, false); assert.equal(f2.counts.post, 0);
});

test("R22 add-only historical method leaves every prior R20 source byte unchanged", async () => {
    const path = "web/src/app/(user)/canvas/components/hn-local-canvas-archive.ts";
    const original = execFileSync("git", ["show", "53fdd43a7c2f849145c35cb053164afcab9dfdfb:" + path]);
    const current = await readFile(new URL("./hn-local-canvas-archive.ts", import.meta.url));
    const start = current.indexOf(Buffer.from("    async readHistoricalArchiveReceipt(")), end = current.indexOf(Buffer.from("    async inspect("), start);
    assert.ok(start > 0 && end > start); assert.deepEqual(Buffer.concat([current.subarray(0, start), current.subarray(end)]), original);
    const method = current.subarray(start, end).toString(); assert.ok(method.includes("this.history(")); assert.equal(/\.setItem\(|\.notify\(|await source\(|await owner\(|\.inspect\(|readBlob|fetch\(/.test(method), false);
});

test("R22 production Canvas store synthetic persistence and reload without account access", async () => {
    if (!process.env.HN_R22_STORE_ISOLATION) {
        const output = execFileSync(process.execPath, ["--no-env-file", "test", fileURLToPath(import.meta.url), "--test-name-pattern", "R22 production Canvas store synthetic"], { env: { ...process.env, HN_R22_STORE_ISOLATION: "1" }, timeout: 30_000 });
        assert.ok(output.toString().includes("R22_STORE_ROUNDTRIP_PASS")); return;
    }
    const require = createRequire(import.meta.url), mock = require("bun:test").mock;
    const storage = new Map<string, string>(); let remote = 0;
    const forbidden = async () => { remote++; assert.fail("account/cloud requests forbidden in synthetic local roundtrip"); };
    mock.module("@/lib/localforage-storage", () => ({ localForageStorage: { getItem: async (key: string) => storage.get(key) || null, setItem: async (key: string, value: string) => { storage.set(key, value); }, removeItem: async (key: string) => { storage.delete(key); } } }));
    mock.module("@/stores/use-user-store", () => ({ useUserStore: { getState: () => ({ token: null }), persist: { hasHydrated: () => true, onFinishHydration: () => () => {} } } }));
    mock.module("@/services/api/canvas-tasks", () => ({ listCanvasProjects: forbidden, saveCanvasProject: forbidden, syncCanvasProjects: forbidden }));
    mock.module("@/services/api/user-config", () => ({ fetchUserConfig: forbidden }));
    const f = await candidateFixture(); mock.module("nanoid", () => ({ nanoid: () => f.project }));
    const { useCanvasStore } = await import("../stores/use-canvas-store");
    await useCanvasStore.persist.rehydrate(); assert.equal(useCanvasStore.getState().createProject("Synthetic R22"), f.project);
    useCanvasStore.getState().updateProject(f.project, { nodes: [f.current().node] });
    await f.run(); useCanvasStore.getState().updateProject(f.project, { nodes: [f.current().node] });
    await new Promise((r) => setTimeout(r, 500));
    const serialized = storage.get("infinite-canvas:canvas_store"); assert.ok(serialized?.includes("hnLocalCandidate"));
    const persisted = JSON.parse(serialized!).state.projects[0].nodes[0]; assert.deepEqual(persisted.metadata.hnLocalPrepared, f.prepared); assert.deepEqual(persisted.metadata.hnLocalArchive, f.receipt);
    useCanvasStore.setState({ projects: [] }); await useCanvasStore.persist.rehydrate();
    const reopened = useCanvasStore.getState().openProject(f.project)!; assert.deepEqual(reopened.nodes[0].metadata?.hnLocalCandidate, f.entry().receipt);
    f.setTarget({ ...f.current(), node: reopened.nodes[0] }); const reload = new HNLocalCandidateController(); await reload.inspect(f.current, new HNLocalArchiveController(), f.d);
    assert.equal(reload.entry(f.project, f.nodeId).phase, "CANDIDATE_RELOADED_UNVERIFIED"); assert.equal(remote, 0);
    console.log("R22_STORE_ROUNDTRIP_PASS: production Zustand Canvas store, synthetic persistence adapter and null account stub; no real browser IndexedDB claim");
});
