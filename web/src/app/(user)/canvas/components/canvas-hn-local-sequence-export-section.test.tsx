import { test, after } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { isValidElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CanvasHNLocalExportSection, CanvasHNLocalSequenceSection, type ExportSectionProps } from "./canvas-hn-local-sequence-section";
import { HN_EXPORT_CONFIRMATION, HN_EXPORT_CONTINUATION, HN_EXPORT_EXPLANATION, HN_EXPORT_HEALTH } from "./hn-local-canvas-sequence-export";
import { exportFixture } from "./hn-local-canvas-sequence-export.test";
import { r34Evidence } from "./hn-local-sequence-export-journal.test";
const facts: unknown[] = [];
function props(f: ReturnType<typeof exportFixture>): ExportSectionProps { return { entry: f.entry(), canSubmit: f.c.canSubmit(f.project), onLoad: () => { void f.load(); }, onRefresh: () => { void f.c.refresh(f.project); }, onSubmit: () => { void f.submit(); }, onLookup: id => { void f.c.lookup(f.project, id); }, onContinue: id => { void f.c.continueSame(f.project, id, async () => true); }, onVerify: id => { void f.c.verifyBundle(f.project, id); } }; }
function buttons(node: unknown): { children?: unknown; disabled?: boolean; onClick?: () => void }[] { if (Array.isArray(node)) return node.flatMap(buttons); if (!isValidElement(node)) return []; const p = node.props as { children?: unknown; disabled?: boolean; onClick?: () => void }; return [...(p.onClick ? [p] : []), ...buttons(p.children)]; }
test("R34 separate sequence-global subsection render zero HTTP/write; placement and reorder coexist", async () => {
    const f = exportFixture(); const p = props(f), html = renderToStaticMarkup(<CanvasHNLocalExportSection {...p} />);
    for (const text of ["主序列安全导出", "加载安全导出", "重新读取导出快照", "初始化本地安全导出元数据", "不改变序列顺序", "不代表 AI/Provider 成功", "整个 HN 项目的 main"]) assert.ok(html.includes(text), text);
    assert.equal(f.counts.http, 0); assert.equal(f.m.values.size, 0); assert.equal(buttons(CanvasHNLocalExportSection(p)).find(b => b.children === "确认导出当前主序列")?.disabled, true);
    const reorder = { entry: { running: false, records: [], snapshot: null, draft: [], currentVerified: false, storageBlocked: false, error: null, laterChanges: false }, canSubmit: false, onLoad() {}, onRefresh() {}, onMove() {}, onSubmit() {}, onLookup() {}, onContinue() {} };
    const combined = renderToStaticMarkup(<CanvasHNLocalSequenceSection entry={{ phase: "SELECTION_NOT_ELIGIBLE", running: false, inspection: 0 }} blocked selectionReloaded={false} onPlace={() => assert.fail("render write")} reorder={reorder} sequenceExport={p} />);
    assert.ok(combined.indexOf("主序列安全排序") < combined.indexOf("主序列安全导出")); assert.ok(combined.includes("加入主序列")); assert.equal(f.counts.http, 0); facts.push({ state: "open", html, combined });
});
test("R34 snapshot ordered IDs/shot/candidate/revision exact and controlled actions", async () => {
    const f = exportFixture(); await f.load(); const p = props(f), html = renderToStaticMarkup(<CanvasHNLocalExportSection {...p} />);
    for (const text of ["SequenceID：main", "sequenceRevision：3", "项数：2", "SequenceItemID：item-0", "CandidateID：candidate-item-0", "ShotID：shot-item-0", "导出顺序不可编辑", "安全排序"]) assert.ok(html.includes(text), text);
    assert.ok(html.indexOf("SequenceItemID：item-0") < html.indexOf("SequenceItemID：item-1")); assert.ok(!html.includes("draggable")); assert.ok(!html.includes("打开文件夹")); assert.ok(!html.includes("打开剪映"));
    let clicked = ""; p.onSubmit = () => { clicked = "submit"; }; buttons(CanvasHNLocalExportSection(p)).find(b => b.children === "确认导出当前主序列")!.onClick!(); assert.equal(clicked, "submit"); assert.equal(f.counts.post, 0); facts.push({ state: "snapshot", html });
});
test("R34 unresolved list per intent actions and known server facts; no new export", async () => {
    const f = exportFixture(); await f.load(); f.setOutcome("RETRYABLE"); await f.submit(); const p = props(f), first = p.entry.records[0]; p.entry.records.push({ ...first, command: { ...first.command, exportIntentId: crypto.randomUUID(), expectedRevision: "7" }, state: "UNKNOWN", status: null });
    const html = renderToStaticMarkup(<CanvasHNLocalExportSection {...p} />); for (const text of ["检查服务器状态", "继续同一次导出", "预期 revision：7", "服务器状态：RETRYABLE", "ExportID：", "attemptCount：1", "EXPORT_IO_RETRYABLE"]) assert.ok(html.includes(text), text);
    assert.equal((html.match(/检查服务器状态/g) || []).length, 2); assert.equal(buttons(CanvasHNLocalExportSection(p)).find(b => b.children === "确认导出当前主序列")?.disabled, true);
    let lookup = "", continuation = ""; p.onLookup = id => { lookup = id; }; p.onContinue = id => { continuation = id; }; const controls = buttons(CanvasHNLocalExportSection(p)); controls.filter(b => b.children === "检查服务器状态")[1].onClick!(); controls.filter(b => b.children === "继续同一次导出")[0].onClick!(); assert.equal(lookup, p.entry.records[1].command.exportIntentId); assert.equal(continuation, first.command.exportIntentId); facts.push({ state: "multi-unresolved", html });
});
test("R34 terminal receipt separate from later sequence and explicit current bundle health", async () => {
    for (const health of ["VERIFIED", "MISSING", "CORRUPT"] as const) {
        const f = exportFixture(); await f.load(); await f.submit(); let html = renderToStaticMarkup(<CanvasHNLocalExportSection {...props(f)} />); assert.ok(html.includes("当前导出包尚未在本页显式验证")); assert.equal(f.counts.verify, 0);
        f.setCurrent(["later"], "9"); await f.c.refresh(f.project); f.setHealth(health); await f.c.verifyBundle(f.project, f.intent()); html = renderToStaticMarkup(<CanvasHNLocalExportSection {...props(f)} />);
        for (const text of ["历史导出命令", "历史导出回执", "ExportID：", "历史 sequenceRevision：3", "itemCount：2", "项目相对导出目录：exports/commands-v1/", "ordered-manifest.json", "ordered-manifest.csv", "当前主序列已发生后续变化", "不证明当前 main 顺序", "验证当前导出包", HN_EXPORT_HEALTH[health], "验证观察时间"]) assert.ok(html.includes(text), text);
        assert.ok(!html.includes("E:\\")); assert.equal(f.counts.post, 1); assert.equal(f.entry().records[0].state, "COMMITTED"); facts.push({ state: "health-" + health, html });
    }
});
test("R34 REJECTED/FAILED static meaning, confirmation and continuation warn mutation", async () => {
    for (const outcome of ["REJECTED", "FAILED"] as const) { const f = exportFixture(); await f.load(); f.setOutcome(outcome); await f.submit(); const html = renderToStaticMarkup(<CanvasHNLocalExportSection {...props(f)} />); assert.ok(html.includes(outcome === "REJECTED" ? "未创建导出包" : "终态本地源完整性失败")); assert.ok(html.includes(outcome === "REJECTED" ? "不会自动重新提交" : "不是 UNKNOWN")); assert.equal(buttons(CanvasHNLocalExportSection(props(f))).some(b => b.children === "验证当前导出包"), false); facts.push({ state: outcome, html }); }
    for (const text of ["新的本地离线导出包", "sequenceRevision", "完整有序项", "新 intent", "不会自动重试", "不证明 AI/Provider 成功"]) assert.ok(HN_EXPORT_CONFIRMATION.includes(text), text);
    for (const text of ["同一条导出命令", "不会创建新的 exportIntentId 或 ExportID", "可能现在开始导出", "不会自动切换", "不是只读确认"]) assert.ok(HN_EXPORT_CONTINUATION.includes(text), text);
    assert.ok(HN_EXPORT_EXPLANATION.includes("不生成视频"));
});
test("R34 page-owned optional dialog wiring: effect only maps and inspects journal, no export auto-effect", () => {
    const page = readFileSync(new URL("../[id]/canvas-client-page.tsx", import.meta.url), "utf8"), dialog = readFileSync(new URL("./canvas-hn-local-archive-dialog.tsx", import.meta.url), "utf8");
    assert.ok(page.includes("useRef<HNLocalSequenceExportController | null>(null)")); assert.ok(page.includes("sequenceExport={localExportController.current!}")); assert.ok(dialog.includes("sequenceExport?: HNLocalSequenceExportController"));
    const effect = dialog.slice(dialog.indexOf("    useEffect(() => {"), dialog.indexOf("    const hn =")); assert.ok(effect.includes("mapHNCanvasProjectId")); assert.ok(effect.includes("sequenceExport?.inspect(hn)")); assert.doesNotMatch(effect, /\.load\(|\.submit\(|\.lookup\(|\.continueSame\(|\.verifyBundle\(/);
    assert.ok(dialog.includes("HN_EXPORT_CONFIRMATION")); assert.ok(dialog.includes("HN_EXPORT_CONTINUATION")); assert.ok(dialog.includes("onVerify: (intent: string)")); assert.ok(!dialog.includes("hnLocalExport:")); facts.push({ state: "source-wiring", effect, controllerOwnedBy: "CANVAS_PAGE_HN_PROJECT_MAIN", exportMetadata: "NONE" });
});
after(() => r34Evidence("ui-rendered", facts));
