import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { mergeHNLocalArchiveReceipt, HNLocalArchiveController } from "./hn-local-canvas-archive";
import { archiveFixture } from "./hn-local-canvas-archive.test";

test("actual page integration preserves existing upload spread and keeps archive callback independent", async () => {
    const page = await readFile(new URL("../[id]/canvas-client-page.tsx", import.meta.url), "utf8"), toolbar = await readFile(new URL("./canvas-node-hover-toolbar.tsx", import.meta.url), "utf8");
    assert.ok(page.includes("metadata: { ...node.metadata, ...videoMetadata(video), errorDetails: undefined }"));
    assert.ok(page.includes("onLocalArchive={(node) => setLocalArchiveNodeId(node.id)}")); assert.ok(toolbar.includes("onClick: () => onLocalArchive(node)"));
    assert.ok(page.includes("new HNLocalArchiveController")); assert.ok(page.includes("useRef<HNLocalArchiveController"));
    assert.ok(page.includes("readBlob: getMediaBlob")); assert.ok(page.includes("mergeHNLocalArchiveReceipt(current, receipt, localArchiveSnapshotRef.current.projectId)"));
    const f = await archiveFixture(); await f.run(); const r = f.entry().receipt!, original = f.target().node;
    const merged = mergeHNLocalArchiveReceipt([original], r, "_Project")[0];
    assert.equal(merged.metadata?.status, "success"); assert.equal(merged.metadata?.hnLocalPrepared, original.metadata?.hnLocalPrepared);
    assert.deepEqual({ ...merged.metadata, hnLocalArchive: undefined }, { ...original.metadata, hnLocalArchive: undefined });
    assert.equal(mergeHNLocalArchiveReceipt([original], r, "foreign")[0], original);
    assert.equal(mergeHNLocalArchiveReceipt([{ ...original, id: "other" }], r, "_Project")[0].metadata?.hnLocalArchive, original.metadata?.hnLocalArchive);
    assert.equal(mergeHNLocalArchiveReceipt([{ ...original, metadata: { ...original.metadata, hnLocalPrepared: { ...f.prepared, generationId: "other" }, hnLocalArchive: undefined } }], r, "_Project")[0].metadata?.hnLocalArchive, undefined);
});
test("new graph allows only R5 local service; no Provider/credential/cloud/editorial imports or calls", async () => {
    for (const file of ["./hn-local-canvas-archive.ts", "./canvas-hn-local-archive-dialog.tsx"]) {
        const source = await readFile(new URL(file, import.meta.url), "utf8");
        assert.equal(/createVideoGenerationTask|submitWorkflowTask|SubmitFrozenGeneration|SubmitMiniMax|PollMiniMax|ArchiveMiniMax|downloadRemoteMedia|uploadMediaFile|autoSyncToCloud|useConfigStore|useUserStore|ensureCandidate|selectCandidate|addSequenceItem|reorderSequence|commitArchivedResultToSequence|exportLocalSequence/.test(source), false);
    }
});
test("undo removing receipt cannot bypass journal/page guard; independent owner lock remains independent", async () => {
    const f = await archiveFixture(); await f.run();
    f.setTarget({ ...f.target(), node: { ...f.target().node, metadata: { ...f.target().node.metadata, hnLocalArchive: undefined } } });
    await f.run(); assert.equal(f.counts.post, 1);
    const reload = new HNLocalArchiveController(); await reload.inspect(f.target(), f.d); assert.equal(reload.entry("_Project", "_video").action, undefined);
    assert.notEqual(reload.entry("_Project", "_video"), reload.entry("_Project", "other")); assert.notEqual(reload.entry("other", "_video"), reload.entry("_Project", "_video"));
});
