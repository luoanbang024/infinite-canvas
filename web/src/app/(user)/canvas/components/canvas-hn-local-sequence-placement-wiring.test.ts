import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { placementFixture } from "./hn-local-canvas-sequence-placement.test";
import { HNLocalSequencePlacementController, mergeHNLocalSequencePlacementReceipt } from "./hn-local-canvas-sequence-placement";

test("R26 exact UI wiring, six new admission edges and frozen R24 controller/request raw bytes", async () => {
    const base = "2d901cb0ce215f11c98d6c607a65eed1262f4548", root = "web/src/app/(user)/canvas/";
    const read = (name: string) => readFile(new URL(name, import.meta.url), "utf8");
    const dialog = await read("./canvas-hn-local-archive-dialog.tsx"), page = await read("../[id]/canvas-client-page.tsx"), controller = await read("./hn-local-canvas-sequence-placement.ts"), selection = await read("./hn-local-canvas-selection.ts");
    assert.ok(dialog.includes("CanvasHNLocalSequenceSection")); assert.ok(dialog.includes("hnLocalSequencePlacement: _placement")); assert.ok(dialog.includes('okText: "确认加入"')); assert.ok(page.includes("useRef<HNLocalSequencePlacementController")); assert.ok(page.includes(".hydrate(project)"));
    for (const marker of ["export class HNLocalSelectionController", "const rejections:"]) {
        const original = execFileSync("git", ["show", base + ":" + root + "components/hn-local-canvas-selection.ts"], { encoding: "utf8" }); assert.equal(selection.slice(selection.indexOf(marker)).replace(/\r\n/g, "\n"), original.slice(original.indexOf(marker)), marker);
    }
    for (const file of ["hn-local-canvas-archive.ts", "hn-local-canvas-candidate.ts"]) {
        const original = execFileSync("git", ["show", base + ":" + root + "components/" + file]); const raw = await readFile(new URL(file, import.meta.url)); assert.equal(raw.toString().replace(/\r\n/g, "\n"), original.toString());
    }
    assert.ok(controller.includes('import { addSequenceItem, type HNSequenceItem }')); assert.equal(/ensureCandidate\(|selectCandidate\(|reorderSequence\(|commitArchivedResultToSequence\(|exportLocalSequence\(|readBlob|createVideoGenerationTask|downloadRemoteMedia|useUserStore|useConfigStore/.test(controller), false);
});
test("R26 safe projection roundtrip/reopen is unverified, late target does not receive receipt", async () => {
    const p = await placementFixture(), before = structuredClone(p.s.f.current()); assert.equal(await p.run(), true); const receipt = p.entry().receipt!;
    const restored = JSON.parse(JSON.stringify(p.s.f.current())); p.s.f.setTarget(restored);
    const reload = new HNLocalSequencePlacementController(p.s.controller); await reload.inspect(p.s.f.current, p.s.f.archive, p.s.f.c, p.d); assert.equal(reload.entry(p.s.f.project, p.s.f.nodeId).phase, "PLACEMENT_RELOADED_UNVERIFIED");
    const newer = structuredClone(before.node); newer.metadata!.hnLocalSelection!.intentId = crypto.randomUUID(); assert.equal(mergeHNLocalSequencePlacementReceipt([newer], receipt, p.s.f.project, before)[0], newer);
    assert.equal(mergeHNLocalSequencePlacementReceipt([before.node], receipt, "foreign", before)[0], before.node);
    const archiveRevision = (t: typeof before) => { const { hnLocalCandidate: _c, hnLocalSelection: _s, hnLocalSequencePlacement: _p, ...metadata } = t.node.metadata!; return JSON.stringify({ ...t, node: { ...t.node, metadata } }); };
    assert.equal(archiveRevision(before), archiveRevision(restored)); assert.equal(p.s.f.counts.blob, 0);
});
