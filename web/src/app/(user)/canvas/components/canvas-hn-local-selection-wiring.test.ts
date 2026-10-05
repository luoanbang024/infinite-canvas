import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CanvasHNLocalArchiveDialog } from "./canvas-hn-local-archive-dialog";
import { selectionFixture } from "./hn-local-canvas-selection.test";
import { mergeHNLocalSelectionReceipt, HNLocalSelectionController } from "./hn-local-canvas-selection";

test("R24 page/Shot coordinator gates existing Dialog handlers; no toolbar or Provider edge", async () => {
    const page = await readFile(new URL("../[id]/canvas-client-page.tsx", import.meta.url), "utf8"), dialog = await readFile(new URL("./canvas-hn-local-archive-dialog.tsx", import.meta.url), "utf8"), module = await readFile(new URL("./hn-local-canvas-selection.ts", import.meta.url), "utf8");
    assert.ok(page.includes("useRef<HNLocalSelectionController")); assert.ok(page.includes("new HNLocalSelectionController")); assert.ok(page.includes("mergeHNLocalSelectionReceipt(current, receipt, localArchiveSnapshotRef.current.projectId, target)"));
    assert.ok(dialog.includes('runOperation("archive"')); assert.ok(dialog.includes('runOperation("candidate"')); assert.ok(dialog.includes("selection.controller.select("));
    assert.ok(dialog.includes("hnLocalCandidate: _candidate, hnLocalSelection: _selection")); assert.ok(dialog.includes("dependencies, archiveRevision")); assert.ok(dialog.includes("runHNArchiveIfCandidateIdle"));
    assert.ok(module.includes('import { selectCandidate } from "@/services/hn/local-editorial"')); assert.equal(/ensureCandidate|addSequenceItem|reorderSequence|commitArchivedResultToSequence|exportLocalSequence|createVideoGenerationTask|submitWorkflowTask|SubmitMiniMax|PollMiniMax|ArchiveMiniMax|downloadRemoteMedia|readBlob|storageKey|useUserStore|useConfigStore/.test(module), false);
    const toolbar = await readFile(new URL("./canvas-node-hover-toolbar.tsx", import.meta.url), "utf8"); assert.equal(toolbar.includes("onLocalSelect"), false);
    const base = "0f35cda5cd7e30be1afcb1bbccce49391ca2b614", path = "web/src/app/(user)/canvas/[id]/canvas-client-page.tsx";
    const original = execFileSync("git", ["show", base + ":" + path], { encoding: "utf8" });
    for (const marker of ["    const handleUploadRequest =", "    const handleGenerateNode ="]) {
        const start = original.indexOf(marker); assert.ok(start >= 0); const end = original.indexOf("    const ", start + marker.length);
        assert.ok(page.replace(/\r\n/g, "\n").includes(original.slice(start, end)), marker);
    }
});

test("R24 actual Dialog SSR/inspect writes zero, merges only selection, restart stays unverified", async () => {
    const s = await selectionFixture(); await s.inspect();
    renderToStaticMarkup(createElement(CanvasHNLocalArchiveDialog, { open: true, canvasProjectId: s.f.project, nodeId: s.f.nodeId, controller: s.f.archive, readTarget: s.f.current, dependencies: { journal: s.f.journal, readBlob: async () => assert.fail("SSR cannot read media") }, onReceipt: () => assert.fail("SSR cannot archive"), onClose: () => {}, targetRevision: s.f.current(), candidate: { controller: s.f.c, dependencies: s.f.d, onReceipt: s.f.receive }, selection: { controller: s.controller, onReceipt: s.receive } }));
    assert.equal(s.counts.post, 0); s.unchanged();
    const original = structuredClone(s.f.current()); assert.equal(await s.run(), true); const receipt = s.entry().receipt!;
    const restored = JSON.parse(JSON.stringify(s.f.current())); s.f.setTarget(restored); const restart = new HNLocalSelectionController();
    await restart.inspect(s.f.current, s.f.archive, s.f.c, s.d); assert.equal(restart.entry(s.f.project, s.f.nodeId).phase, "SELECTION_RELOADED_UNVERIFIED"); assert.equal(s.counts.post, 1);
    const onlySelection = mergeHNLocalSelectionReceipt([original.node], receipt, s.f.project, original)[0]; assert.deepEqual({ ...onlySelection.metadata, hnLocalSelection: undefined }, { ...original.node.metadata, hnLocalSelection: undefined });
    const replacement = { ...original.node, metadata: { ...original.node.metadata, hnLocalCandidate: { ...original.node.metadata!.hnLocalCandidate!, resultId: "new-result" } } };
    assert.equal(mergeHNLocalSelectionReceipt([replacement], receipt, s.f.project, original)[0], replacement);
    const archiveRevision = (t: typeof original) => { const { hnLocalCandidate: _c, hnLocalSelection: _s, ...metadata } = t.node.metadata!; return JSON.stringify({ ...t, node: { ...t.node, metadata } }); };
    assert.equal(archiveRevision(original), archiveRevision(s.f.current())); assert.equal(s.f.counts.blob, 0);
});
