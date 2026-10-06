"use client";
import { Button } from "antd";
import { HN_PLACEMENT_EXPLANATION, HN_PLACEMENT_NO_READ, HN_PLACEMENT_UNKNOWN, HN_PLACEMENT_RELOAD, HN_PLACEMENT_SUCCESS, HN_PLACEMENT_DUPLICATE, HN_PLACEMENT_SELECTION_RELOAD, type PlacementEntry } from "./hn-local-canvas-sequence-placement";

export function CanvasHNLocalSequenceSection({ entry, blocked, selectionReloaded, onPlace }: { entry: PlacementEntry; blocked: boolean; selectionReloaded: boolean; onPlace: () => void }) {
    const r = entry.receipt, known = entry.phase === "PLACED_CURRENT_SESSION" || entry.phase === "PLACEMENT_RELOADED_UNVERIFIED";
    const status = entry.phase === "PLACEMENT_OUTCOME_UNKNOWN" ? HN_PLACEMENT_UNKNOWN : entry.phase === "PLACEMENT_RELOADED_UNVERIFIED" ? HN_PLACEMENT_RELOAD : entry.phase === "PLACED_CURRENT_SESSION" ? HN_PLACEMENT_SUCCESS : entry.running ? "正在加入主序列…" : entry.error || (entry.phase === "SELECTION_NOT_ELIGIBLE" ? "请先明确选择一个有效的历史本地候选。" : "尚无本地确认的加入回执。");
    return <section className="mt-6 space-y-3 text-sm" aria-label="本地序列">
        <h3>本地序列</h3><p>{HN_PLACEMENT_EXPLANATION}</p><p>{HN_PLACEMENT_NO_READ}</p>
        {selectionReloaded ? <p>{HN_PLACEMENT_SELECTION_RELOAD}</p> : null}
        <p role="status" data-hn-placement-state={entry.phase}>{status}</p>
        {known ? <p>{HN_PLACEMENT_DUPLICATE}</p> : null}
        {r ? <dl className="space-y-1 break-words"><dt>一次已确认操作的历史回执</dt><dd>SequenceID：main</dd><dd>SequenceItemID：{r.sequenceItemId}</dd><dd>创建时 orderIndex：{r.orderIndex}</dd><dd>ShotID：{r.shotId}</dd><dd>CandidateID：{r.candidateId}</dd><dd>ResultID：{r.resultId}</dd><dd>浏览器观察时间：{r.observedAt}</dd></dl> : null}
        <Button type="primary" loading={entry.running} disabled={blocked || entry.running || known || entry.phase === "PLACEMENT_OUTCOME_UNKNOWN" || entry.phase === "SELECTION_NOT_ELIGIBLE"} onClick={onPlace}>加入主序列</Button>
    </section>;
}
