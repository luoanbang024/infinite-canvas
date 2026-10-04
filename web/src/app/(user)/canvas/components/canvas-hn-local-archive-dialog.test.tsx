import { test } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { writeFile } from "node:fs/promises";
import { HNLocalArchiveContent, CanvasHNLocalArchiveDialog } from "./canvas-hn-local-archive-dialog";
import { HN_ARCHIVE_EXPLANATION, HN_ARCHIVE_NEW_CONFIRMATION, HNLocalArchiveController, type HNArchiveEntry } from "./hn-local-canvas-archive";
import { CanvasNodeHoverToolbar } from "./canvas-node-hover-toolbar";
import type { ComponentProps } from "react";
import { archiveFixture } from "./hn-local-canvas-archive.test";

test("archive markup renders mandatory explanation and exact safe states/actions without credentials/editorial", async () => {
    const f = await archiveFixture(); await f.run(); const receipt = f.entry().receipt!;
    const states: [HNArchiveEntry["phase"], string, HNArchiveEntry["action"]][] = [["NOT_ARCHIVED", "尚未归档本地视频", "fresh"], ["ARCHIVING", "正在归档本地视频", undefined], ["ARCHIVED", "本地字节已归档", undefined], ["ARCHIVE_FAILED_RETRYABLE", "显式重试同一归档", "retry"], ["ARCHIVE_OUTCOME_UNKNOWN", "可能已写入", undefined], ["SOURCE_CHANGED", "视频字节已变更", "new"], ["RELOADED_UNVERIFIED", "历史本地归档回执，未复核", undefined]];
    const output = [];
    for (const [phase, text, action] of states) {
        const html = renderToStaticMarkup(<HNLocalArchiveContent entry={{ ...f.entry(), phase, action, running: phase === "ARCHIVING", receipt }} onArchive={() => assert.fail("render must not archive")} />);
        assert.ok(html.includes(HN_ARCHIVE_EXPLANATION)); assert.ok(html.includes(text)); assert.ok(html.includes(`data-hn-archive-state="${phase}"`));
        assert.equal(/apiKey|Authorization|余额|账户|渠道选择|Candidate|Sequence|导出|AI 生成成功|Provider 成功/.test(html), false);
        assert.equal(html.includes(receipt.sourceSHA256), false); assert.ok(html.includes(receipt.sourceSHA256.slice(0, 12)));
        if (!action || phase === "ARCHIVING") assert.ok(html.includes("disabled"));
        if (action === "new") assert.ok(html.includes(HN_ARCHIVE_NEW_CONFIRMATION));
        output.push({ phase, html });
    }
    if (process.env.HN_R20_UI_EVIDENCE) await writeFile(process.env.HN_R20_UI_EVIDENCE, JSON.stringify({ runtime: "ReactDOM SSR, not browser IndexedDB acceptance", states: output }, null, 2));
});
test("Dialog opening/rendering performs zero GET/POST/journal writes; controller preview reads only", async () => {
    const f = await archiveFixture();
    renderToStaticMarkup(<CanvasHNLocalArchiveDialog open canvasProjectId="_Project" nodeId="_video" controller={f.c} readTarget={f.target} dependencies={f.d} onReceipt={f.receive} onClose={() => {}} targetRevision={f.target()} />);
    await f.c.inspect(f.target(), f.d); await f.c.inspect(f.target(), f.d);
    assert.equal(f.counts.post, 0); assert.equal(f.counts.discovery, 0); assert.equal(f.counts.journalWrite, 0);
});
test("actual hover toolbar separates archive from prepare/upload and render never calls action", async () => {
    const f = await archiveFixture();
    const callbacks = Object.fromEntries(["onKeep", "onLeave", "onInfo", "onDecreaseFont", "onIncreaseFont", "onToggleDialog", "onGenerateImage", "onUpload", "onExtractAudio", "onTrimAudio", "onDownload", "onSaveAsset", "onUploadMediaToCloud", "onUploadImageToCloud", "onFreezeReference", "onLocalPrepare", "onLocalArchive", "onMaskEdit", "onCrop", "onSplit", "onUpscale", "onSuperResolve", "onAngle", "onViewImage", "onReversePrompt", "onRetry", "onToggleFreeResize", "onDelete"].map((key) => [key, () => assert.fail("render can't dispatch")]));
    const html = renderToStaticMarkup(<CanvasNodeHoverToolbar {...({ ...callbacks, node: f.target().node, viewport: { x: 0, y: 0, k: 1 } } as ComponentProps<typeof CanvasNodeHoverToolbar>)} />);
    assert.ok(html.includes("归档本地视频")); assert.ok(html.includes("替换视频")); assert.equal(html.includes("本地准备"), false);
    const c = new HNLocalArchiveController(); assert.equal(c.entry("_Project", "_video").running, false);
});
