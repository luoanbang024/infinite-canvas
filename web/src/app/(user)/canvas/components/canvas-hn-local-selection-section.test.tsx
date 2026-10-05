import { test } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { CanvasHNLocalCandidateSection } from "./canvas-hn-local-candidate-section";
import { HNLocalArchiveContent } from "./canvas-hn-local-archive-dialog";
import { selectionFixture } from "./hn-local-canvas-selection.test";
import { HN_SELECTION_EXPLANATION, HN_SELECTION_REPEAT, HN_SELECTION_NO_READ, HN_SELECTION_CANDIDATE_UNVERIFIED, type HNSelectionPhase } from "./hn-local-canvas-selection";

test("R24 Selection presentation is advisory and a repeat is a new mutation in every state", async () => {
    const s = await selectionFixture(); await s.inspect();
    for (const phase of ["CANDIDATE_NOT_ELIGIBLE", "NOT_CONFIRMED_SELECTED", "SELECTING", "SELECTED_CURRENT_SESSION", "SELECTION_OUTCOME_UNKNOWN", "SELECTION_RELOADED_UNVERIFIED", "ERROR"] as HNSelectionPhase[]) {
        const html = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={s.f.entry()} archiveRunning={false} onEnsure={() => assert.fail("render cannot ensure")} selection={{ entry: { ...s.entry(), phase, running: phase === "SELECTING", ...(phase === "ERROR" ? { error: "HN_SELECTION_CONFLICT" } : {}) }, blocked: phase === "CANDIDATE_NOT_ELIGIBLE", onSelect: () => assert.fail("render cannot select") }} />);
        assert.ok(html.includes(HN_SELECTION_EXPLANATION)); assert.ok(html.includes(HN_SELECTION_NO_READ)); assert.ok(html.includes(`data-hn-selection-state="${phase}"`));
        const repeat = ["SELECTED_CURRENT_SESSION", "SELECTION_OUTCOME_UNKNOWN", "SELECTION_RELOADED_UNVERIFIED"].includes(phase);
        assert.ok(html.includes(repeat ? "重新选择此候选" : "选择此候选")); if (repeat) assert.ok(html.includes(HN_SELECTION_REPEAT));
        if (phase === "SELECTION_RELOADED_UNVERIFIED") assert.ok(html.includes("不能确认服务器当前选择"));
        if (phase === "SELECTED_CURRENT_SESSION") assert.ok(html.includes("其他操作仍可能改变选择"));
        if (phase === "NOT_CONFIRMED_SELECTED") assert.ok(html.includes("这不表示服务器没有当前选择"));
        assert.equal(/刷新当前选择|复核当前选择|只读确认|当前服务器已选中|timeline|API Key|余额|渠道选择|<button[^>]*>[^<]*(?:加入序列|导出)/.test(html), false);
    }
    s.f.entry().phase = "CANDIDATE_RELOADED_UNVERIFIED";
    const html = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={s.f.entry()} archiveRunning={false} onEnsure={() => {}} selection={{ entry: s.entry(), blocked: false, onSelect: () => {} }} />);
    assert.ok(html.includes(HN_SELECTION_CANDIDATE_UNVERIFIED)); assert.equal(s.counts.post, 0);
});

test("R24 controlled buttons keep Candidate-only defaults and disable both existing actions", async () => {
    const s = await selectionFixture(); await s.inspect(); let selected = 0;
    const section = CanvasHNLocalCandidateSection({ entry: s.f.entry(), archiveRunning: false, onEnsure: () => assert.fail("Selection cannot ensure"), selection: { entry: s.entry(), blocked: false, onSelect: () => { selected++; } } });
    const find = (node: unknown): (() => void) | undefined => {
        if (!node || typeof node !== "object") return;
        const e = node as { props?: { "aria-label"?: string; children?: unknown; onClick?: () => void } };
        if (e.props?.["aria-label"] === "本地候选选择") {
            const children = e.props.children as { props?: { onClick?: () => void } }[];
            return children.find((c) => c?.props?.onClick)?.props?.onClick;
        }
        for (const c of Array.isArray(e.props?.children) ? e.props.children : [e.props?.children]) { const action = find(c); if (action) return action; }
    };
    const action = find(section); assert.ok(action); assert.equal(selected, 0); action(); assert.equal(selected, 1);
    const html = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={s.f.entry()} archiveRunning={false} candidateBlocked onEnsure={() => {}} selection={{ entry: s.entry(), blocked: true, onSelect: () => {} }} />);
    assert.ok((html.match(/disabled/g) || []).length >= 3);
    const archiveHTML = renderToStaticMarkup(<HNLocalArchiveContent entry={{ phase: "NOT_ARCHIVED", running: false, inspection: 0, action: "fresh" }} operationBlocked onArchive={() => {}} />); assert.ok(archiveHTML.includes("disabled"));
    const candidateOnly = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={s.f.entry()} archiveRunning={false} onEnsure={() => {}} />); assert.equal(candidateOnly.includes("本地候选选择"), false);
});
