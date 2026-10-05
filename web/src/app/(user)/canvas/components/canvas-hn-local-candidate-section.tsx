"use client";

import { Button } from "antd";
import { HN_CANDIDATE_EXPLANATION, HN_CANDIDATE_HISTORICAL, HN_CANDIDATE_REVALIDATION, HN_CANDIDATE_ERRORS, type HNCandidateEntry } from "./hn-local-canvas-candidate";

export function HNLocalCandidateFacts({ entry }: { entry: HNCandidateEntry }) {
    const r = entry.target;
    return r ? <dl className="space-y-2 break-words"><dt>历史已归档本地版本</dt><dd>ShotID：{r.shotId}</dd><dd>GenerationID：{r.generationId}</dd><dd>ResultID：{r.resultId}</dd><dd>ArchiveJobID：{r.archiveJobId}</dd><dd>{r.sourceByteLength} bytes · {r.sourceMimeType} · SHA-256：{r.sourceSHA256.slice(0, 12)}</dd></dl> : null;
}
export function CanvasHNLocalCandidateSection({ entry, archiveRunning, onEnsure }: { entry: HNCandidateEntry; archiveRunning: boolean; onEnsure: (revalidate: boolean) => void }) {
    const ready = entry.phase === "CANDIDATE_READY", recovery = entry.phase === "CANDIDATE_OUTCOME_UNKNOWN" || entry.phase === "CANDIDATE_RELOADED_UNVERIFIED";
    const wording = entry.running ? "正在确认本地候选…" : entry.error ? HN_CANDIDATE_ERRORS[entry.error] : ready ? "已归档的本地版本可作为剪辑候选。" : entry.phase === "CANDIDATE_RELOADED_UNVERIFIED" ? HN_CANDIDATE_ERRORS.HN_CANDIDATE_UNVERIFIED : entry.phase === "CANDIDATE_OUTCOME_UNKNOWN" ? HN_CANDIDATE_ERRORS.HN_CANDIDATE_OUTCOME_UNKNOWN : entry.phase === "NOT_CANDIDATE" ? "尚未确认候选回执；这不表示服务器没有候选。" : HN_CANDIDATE_ERRORS.HN_CANDIDATE_ARCHIVE_NOT_ELIGIBLE;
    const blocked = entry.running || archiveRunning || !entry.target || entry.phase === "ARCHIVE_NOT_ELIGIBLE";
    return <section className="mt-6 space-y-4 text-sm" aria-label="本地剪辑候选">
        <h3>本地剪辑候选</h3><p>{HN_CANDIDATE_EXPLANATION}</p><p>{HN_CANDIDATE_HISTORICAL}</p>
        <p role="status" data-hn-candidate-state={entry.phase}>{wording}</p>
        <HNLocalCandidateFacts entry={entry} />
        {entry.receipt && entry.target && entry.receipt.resultId === entry.target.resultId && entry.receipt.archiveJobId === entry.target.archiveJobId ? <p className="break-words">CandidateID：{entry.receipt.candidateId}</p> : null}
        {recovery || entry.target ? <p>{HN_CANDIDATE_REVALIDATION}</p> : null}
        <Button disabled={blocked || ready} loading={entry.running} onClick={() => onEnsure(recovery)}>{recovery ? "重新确认候选" : "将已归档版本加入候选"}</Button>
        {ready ? <Button disabled={blocked} onClick={() => onEnsure(true)}>重新确认候选</Button> : null}
    </section>;
}
