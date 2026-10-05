"use client";

import { Button } from "antd";
import { HN_CANDIDATE_EXPLANATION, HN_CANDIDATE_HISTORICAL, HN_CANDIDATE_REVALIDATION, HN_CANDIDATE_ERRORS, type HNCandidateEntry } from "./hn-local-canvas-candidate";
import { HN_SELECTION_EXPLANATION, HN_SELECTION_REPEAT, HN_SELECTION_NO_READ, HN_SELECTION_CANDIDATE_UNVERIFIED, HN_SELECTION_ERRORS, type HNSelectionEntry } from "./hn-local-canvas-selection";

export function HNLocalCandidateFacts({ entry }: { entry: HNCandidateEntry }) {
    const r = entry.target;
    return r ? <dl className="space-y-2 break-words"><dt>历史已归档本地版本</dt><dd>ShotID：{r.shotId}</dd><dd>GenerationID：{r.generationId}</dd><dd>ResultID：{r.resultId}</dd><dd>ArchiveJobID：{r.archiveJobId}</dd><dd>{r.sourceByteLength} bytes · {r.sourceMimeType} · SHA-256：{r.sourceSHA256.slice(0, 12)}</dd></dl> : null;
}
export function CanvasHNLocalCandidateSection({ entry, archiveRunning, onEnsure, candidateBlocked = false, selection }: { entry: HNCandidateEntry; archiveRunning: boolean; onEnsure: (revalidate: boolean) => void; candidateBlocked?: boolean; selection?: { entry: HNSelectionEntry; blocked: boolean; onSelect: () => void } }) {
    const ready = entry.phase === "CANDIDATE_READY", recovery = entry.phase === "CANDIDATE_OUTCOME_UNKNOWN" || entry.phase === "CANDIDATE_RELOADED_UNVERIFIED";
    const wording = entry.running ? "正在确认本地候选…" : entry.error ? HN_CANDIDATE_ERRORS[entry.error] : ready ? "已归档的本地版本可作为剪辑候选。" : entry.phase === "CANDIDATE_RELOADED_UNVERIFIED" ? HN_CANDIDATE_ERRORS.HN_CANDIDATE_UNVERIFIED : entry.phase === "CANDIDATE_OUTCOME_UNKNOWN" ? HN_CANDIDATE_ERRORS.HN_CANDIDATE_OUTCOME_UNKNOWN : entry.phase === "NOT_CANDIDATE" ? "尚未确认候选回执；这不表示服务器没有候选。" : HN_CANDIDATE_ERRORS.HN_CANDIDATE_ARCHIVE_NOT_ELIGIBLE;
    const blocked = entry.running || archiveRunning || candidateBlocked || !entry.target || entry.phase === "ARCHIVE_NOT_ELIGIBLE";
    const s = selection?.entry;
    const repeat = s && ["SELECTED_CURRENT_SESSION", "SELECTION_OUTCOME_UNKNOWN", "SELECTION_RELOADED_UNVERIFIED"].includes(s.phase);
    const selectionWording = s?.phase === "SELECTED_CURRENT_SESSION" ? "本次选择已获本地后端确认；其他操作仍可能改变选择。" : s?.phase === "SELECTION_RELOADED_UNVERIFIED" ? HN_SELECTION_ERRORS.HN_SELECTION_UNVERIFIED : s?.phase === "SELECTION_OUTCOME_UNKNOWN" ? HN_SELECTION_ERRORS.HN_SELECTION_UNKNOWN : s?.phase === "SELECTING" ? "正在选择此候选…" : s?.error ? HN_SELECTION_ERRORS[s.error] : s?.phase === "CANDIDATE_NOT_ELIGIBLE" ? HN_SELECTION_ERRORS.HN_SELECTION_NOT_ELIGIBLE : "本页尚无选择确认；这不表示服务器没有当前选择。";
    return <section className="mt-6 space-y-4 text-sm" aria-label="本地剪辑候选">
        <h3>本地剪辑候选</h3><p>{HN_CANDIDATE_EXPLANATION}</p><p>{HN_CANDIDATE_HISTORICAL}</p>
        <p role="status" data-hn-candidate-state={entry.phase}>{wording}</p>
        <HNLocalCandidateFacts entry={entry} />
        {entry.receipt && entry.target && entry.receipt.resultId === entry.target.resultId && entry.receipt.archiveJobId === entry.target.archiveJobId ? <p className="break-words">CandidateID：{entry.receipt.candidateId}</p> : null}
        {recovery || entry.target ? <p>{HN_CANDIDATE_REVALIDATION}</p> : null}
        <Button disabled={blocked || ready} loading={entry.running} onClick={() => onEnsure(recovery)}>{recovery ? "重新确认候选" : "将已归档版本加入候选"}</Button>
        {ready ? <Button disabled={blocked} onClick={() => onEnsure(true)}>重新确认候选</Button> : null}
        {selection && s ? <div className="space-y-3" aria-label="本地候选选择">
            <p>{HN_SELECTION_EXPLANATION}</p><p>{HN_SELECTION_NO_READ}</p>
            {entry.phase === "CANDIDATE_RELOADED_UNVERIFIED" ? <p>{HN_SELECTION_CANDIDATE_UNVERIFIED}</p> : null}
            <p role="status" data-hn-selection-state={s.phase}>{selectionWording}</p>
            {s.owner ? <p className="break-words">ShotID：{s.owner.shotId} · CandidateID：{s.owner.candidateId} · ResultID：{s.owner.resultId}</p> : null}
            {repeat ? <p>{HN_SELECTION_REPEAT}</p> : null}
            <Button disabled={selection.blocked || s.running || !s.owner || !(ready || entry.phase === "CANDIDATE_RELOADED_UNVERIFIED")} loading={s.running} onClick={selection.onSelect}>{repeat ? "重新选择此候选" : "选择此候选"}</Button>
        </div> : null}
    </section>;
}
