import { test } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { CanvasHNLocalCandidateSection, HNLocalCandidateFacts } from "./canvas-hn-local-candidate-section";
import { HN_CANDIDATE_EXPLANATION, HN_CANDIDATE_HISTORICAL, HN_CANDIDATE_REVALIDATION, type HNCandidatePhase } from "./hn-local-canvas-candidate";
import { HNLocalArchiveContent } from "./canvas-hn-local-archive-dialog";
import { candidateFixture } from "./hn-local-canvas-candidate.test";

test("R22 presentation keeps Candidate-only semantics and historical safe facts in every state", async () => {
    const f = await candidateFixture(); await f.run();
    const states: HNCandidatePhase[] = ["NOT_CANDIDATE", "ENSURING_CANDIDATE", "CANDIDATE_READY", "CANDIDATE_OUTCOME_UNKNOWN", "CANDIDATE_RELOADED_UNVERIFIED", "ARCHIVE_NOT_ELIGIBLE", "ERROR"];
    for (const phase of states) {
        const entry = { ...f.entry(), phase, running: phase === "ENSURING_CANDIDATE" };
        const html = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={entry} archiveRunning={false} onEnsure={() => assert.fail("render cannot ensure")} />);
        assert.ok(html.includes("本地剪辑候选")); assert.ok(html.includes(HN_CANDIDATE_EXPLANATION)); assert.ok(html.includes(HN_CANDIDATE_HISTORICAL)); assert.ok(html.includes(`data-hn-candidate-state="${phase}"`));
        assert.ok(html.includes("ShotID：shot")); assert.ok(html.includes("GenerationID：generation")); assert.ok(html.includes("ResultID：result")); assert.ok(html.includes("ArchiveJobID：job")); assert.ok(html.includes("24 bytes")); assert.ok(html.includes("video/mp4")); assert.ok(html.includes(f.receipt.sourceSHA256.slice(0, 12))); assert.equal(html.includes(f.receipt.sourceSHA256), false);
        assert.equal(/selectCandidate|SequenceItem|timeline|导出|余额|API Key|渠道选择|AI 生成成功|Provider 成功|已选中/.test(html), false);
        if (phase === "CANDIDATE_OUTCOME_UNKNOWN" || phase === "CANDIDATE_RELOADED_UNVERIFIED") assert.ok(html.includes("重新确认候选"));
        if (phase === "NOT_CANDIDATE") assert.ok(html.includes("这不表示服务器没有候选"));
        if (phase === "CANDIDATE_RELOADED_UNVERIFIED") assert.ok(html.includes(HN_CANDIDATE_REVALIDATION));
        if (["ENSURING_CANDIDATE", "ARCHIVE_NOT_ELIGIBLE", "CANDIDATE_READY"].includes(phase)) assert.ok(html.includes("disabled"));
    }
});

test("R22 isolated section emits only deliberate ensure/revalidation, archive interlock disables it", async () => {
    const f = await candidateFixture(); await f.inspect(); let calls = 0;
    const section = CanvasHNLocalCandidateSection({ entry: f.entry(), archiveRunning: false, onEnsure: () => { calls++; } });
    const children = section.props.children as { type: unknown; props?: { disabled?: boolean; onClick?: () => void } }[];
    const button = children.find((item) => item?.props?.onClick); assert.ok(button?.props?.onClick); assert.equal(calls, 0); button.props.onClick(); assert.equal(calls, 1);
    const html = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={f.entry()} archiveRunning onEnsure={() => assert.fail("render cannot submit")} />); assert.ok(html.includes("disabled"));
    const archiveHTML = renderToStaticMarkup(<HNLocalArchiveContent entry={{ phase: "NOT_ARCHIVED", running: false, inspection: 0, action: "fresh" }} candidateRunning onArchive={() => assert.fail("render cannot archive")} />); assert.ok(archiveHTML.includes("disabled"));
});

test("R22 facts without verified target do not display fabricated CandidateID or current Blob", () => {
    const html = renderToStaticMarkup(<CanvasHNLocalCandidateSection entry={{ phase: "ARCHIVE_NOT_ELIGIBLE", running: false, inspection: 0 }} archiveRunning={false} onEnsure={() => assert.fail("not authorized")} />);
    assert.equal(html.includes("CandidateID"), false); assert.equal(html.includes("blob:"), false); assert.ok(html.includes("disabled"));
    assert.equal(renderToStaticMarkup(<HNLocalCandidateFacts entry={{ phase: "NOT_CANDIDATE", running: false, inspection: 0 }} />), "");
});
