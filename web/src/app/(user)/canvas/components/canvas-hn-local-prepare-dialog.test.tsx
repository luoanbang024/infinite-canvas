import { test } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { HNLocalPrepareContent } from "./canvas-hn-local-prepare-dialog";
import { CanvasNodeHoverToolbar } from "./canvas-node-hover-toolbar";
import { CanvasNodeType } from "../types";
import type { ComponentProps } from "react";
import { writeFile } from "node:fs/promises";
import { HN_LOCAL_EXPLANATION, HNLocalPrepareController, type HNLocalEntry, type HNLocalPhase } from "./hn-local-canvas-prepare";
import { intent, localFixture, video } from "./hn-local-canvas-prepare.test";

test("existing React rendering gives exact states, safe receipt/hash, disabled duplicate action", async () => {
    const f = localFixture(), c = new HNLocalPrepareController();
    try {
        await c.prepare(() => intent(), f.dependencies, async () => true, () => {});
        const receipt = c.entry("_Project", "_video").receipt!;
        const states: [HNLocalPhase, string][] = [["UNPREPARED", "尚未本地准备"], ["PREPARING", "正在冻结本地准备"], ["CURRENT", "已本地准备（PREPARED），未提交"], ["STALE", "内容已变更"], ["RELOADED_UNVERIFIED", "历史准备回执，未复核"], ["ERROR", "本地准备未完成"], ["OUTCOME_UNKNOWN", "准备结果未确认"]];
        const rendered: { phase: string; html: string }[] = [];
        for (const [phase, wording] of states) {
            const entry: HNLocalEntry = { phase, running: phase === "PREPARING", attempted: phase !== "UNPREPARED", valid: true, inspection: 0, receipt: phase === "UNPREPARED" ? undefined : receipt, error: phase === "ERROR" ? "HN_PREPARE_REJECTED" : phase === "OUTCOME_UNKNOWN" ? "HN_PREPARE_OUTCOME_UNKNOWN" : undefined };
            const html = renderToStaticMarkup(<HNLocalPrepareContent entry={entry} onPrepare={() => {}} />);
            assert.ok(html.includes(HN_LOCAL_EXPLANATION)); assert.ok(html.includes(wording)); assert.ok(html.includes(`data-hn-local-state="${phase}"`));
            assert.equal(html.includes("a".repeat(64)), false); if (entry.receipt) assert.ok(html.includes("a".repeat(12)));
            if (entry.running) assert.match(html, /disabled/);
            assert.equal(/apiKey|balance|entitlement|Authorization|channelId/.test(html), false);
            rendered.push({ phase, html });
        }
        if (process.env.HN_R18_UI_EVIDENCE) await writeFile(process.env.HN_R18_UI_EVIDENCE, JSON.stringify({ runtime: "ReactDOM renderToStaticMarkup", states: rendered }, null, 2));
    } finally { f.close(); }
});
test("event-level controller action: opening/reopening zero writes, second action requires confirm, does not clear prompt", async () => {
    const f = localFixture(), c = new HNLocalPrepareController(); let current = intent(); let confirmations = 0;
    try {
        const original = video(); await c.inspect(current, f.dependencies); await c.inspect(current, f.dependencies); assert.equal(f.counters.shot, 0); assert.equal(f.counters.prepare, 0);
        const click = () => c.prepare(() => current, f.dependencies, async () => { confirmations++; return true; }, (r) => { current = { ...current, receipt: r }; });
        await click(); assert.equal(confirmations, 0); await click(); assert.equal(confirmations, 1); assert.equal(f.counters.prepare, 2);
        assert.equal(current.promptSnapshot, original.metadata!.prompt); assert.equal(original.metadata!.status, "idle"); assert.equal(original.metadata!.videoTaskId, undefined);
    } finally { f.close(); }
});

test("actual hover toolbar renders local prepare only for clean Video and preserves existing controls", () => {
    const callbacks = Object.fromEntries(["onKeep", "onLeave", "onInfo", "onDecreaseFont", "onIncreaseFont", "onToggleDialog", "onGenerateImage", "onUpload", "onExtractAudio", "onTrimAudio", "onDownload", "onSaveAsset", "onUploadMediaToCloud", "onUploadImageToCloud", "onFreezeReference", "onLocalPrepare", "onMaskEdit", "onCrop", "onSplit", "onUpscale", "onSuperResolve", "onAngle", "onViewImage", "onReversePrompt", "onRetry", "onToggleFreeResize", "onDelete"].map((key) => [key, () => assert.fail("render must not invoke action")]));
    const render = (node = video()) => renderToStaticMarkup(<CanvasNodeHoverToolbar {...({ ...callbacks, node, viewport: { x: 0, y: 0, k: 1 } } as ComponentProps<typeof CanvasNodeHoverToolbar>)} />);
    assert.ok(render().includes("本地准备"));
    for (const node of [{ ...video(), type: CanvasNodeType.Text }, { ...video(), type: CanvasNodeType.Image }, { ...video(), metadata: { content: "uploaded" } }, { ...video(), metadata: { status: "loading" as const } }, { ...video(), metadata: { status: "error" as const } }, { ...video(), metadata: { videoTaskId: "old-task" } }]) assert.equal(render(node).includes("本地准备"), false);
    assert.ok(render({ ...video(), metadata: { status: "error" } }).includes("重试"));
});
