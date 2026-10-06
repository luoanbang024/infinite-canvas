import { test } from "node:test";
import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CanvasHNLocalSequenceSection } from "./canvas-hn-local-sequence-section";

test("R28 v2 actual section distinguishes read lookup, explicit mutation and legacy barrier", () => {
    for (const legacyUnknown of [false, true]) {
        const html = renderToStaticMarkup(createElement(CanvasHNLocalSequenceSection, { entry: { phase: "PLACEMENT_OUTCOME_UNKNOWN", running: false, inspection: 0, protocolV2: true, recoverable: true, legacyUnknown }, blocked: true, selectionReloaded: true, onPlace: () => assert.fail("SSR POST"), onLookup: () => assert.fail("SSR GET"), onContinue: () => assert.fail("SSR continuation"), onRead: () => assert.fail("SSR snapshot") }));
        assert.ok(html.includes("检查服务器结果")); assert.ok(html.includes("继续同一次加入操作")); assert.ok(html.includes("读取主序列快照")); assert.ok(html.includes("不证明当前选择、当前顺序、媒体完整性或 AI/Provider 成功"));
        assert.equal(html.includes("本页没有可靠的主序列查询"), false);
        if (legacyUnknown) assert.ok(html.includes("旧版加入操作仍未确认"));
        else assert.ok(html.includes("可明确检查服务器结果"));
    }
});
