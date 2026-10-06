import { test } from "node:test";
import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CanvasHNLocalSequenceSection } from "./canvas-hn-local-sequence-section";
import { HN_PLACEMENT_EXPLANATION, HN_PLACEMENT_NO_READ, HN_PLACEMENT_UNKNOWN, HN_PLACEMENT_RELOAD, type PlacementPhase } from "./hn-local-canvas-sequence-placement";

test("R26 actual section SSR: exact wording/states and only explicit placement action", () => {
    for (const phase of ["SELECTION_NOT_ELIGIBLE", "NOT_CONFIRMED_PLACED", "PLACING", "PLACED_CURRENT_SESSION", "PLACEMENT_OUTCOME_UNKNOWN", "PLACEMENT_RELOADED_UNVERIFIED", "ERROR"] as PlacementPhase[]) {
        const html = renderToStaticMarkup(createElement(CanvasHNLocalSequenceSection, { entry: { phase, running: phase === "PLACING", inspection: 0 }, blocked: false, selectionReloaded: true, onPlace: () => assert.fail("SSR mutation") }));
        assert.ok(html.includes(HN_PLACEMENT_EXPLANATION)); assert.ok(html.includes(HN_PLACEMENT_NO_READ)); assert.ok(html.includes("加入主序列")); assert.ok(html.includes(`data-hn-placement-state="${phase}"`));
        if (["SELECTION_NOT_ELIGIBLE", "PLACING", "PLACED_CURRENT_SESSION", "PLACEMENT_OUTCOME_UNKNOWN", "PLACEMENT_RELOADED_UNVERIFIED"].includes(phase)) assert.ok(html.includes('disabled=""'));
        if (phase === "PLACEMENT_OUTCOME_UNKNOWN") assert.ok(html.includes(HN_PLACEMENT_UNKNOWN)); if (phase === "PLACEMENT_RELOADED_UNVERIFIED") assert.ok(html.includes(HN_PLACEMENT_RELOAD));
        assert.equal(/重新发送|清除屏障|导出按钮|当前序列总数/.test(html), false);
    }
    const element = CanvasHNLocalSequenceSection({ entry: { phase: "NOT_CONFIRMED_PLACED", running: false, inspection: 0 }, blocked: false, selectionReloaded: false, onPlace: () => {} }); assert.equal(element.type, "section");
});
