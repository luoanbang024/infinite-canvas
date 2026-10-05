"use client";

import { useEffect } from "react";
import { App, Button, Modal } from "antd";
import { canvasThemes } from "@/lib/canvas-theme";
import { useThemeStore } from "@/stores/use-theme-store";
import { HN_ARCHIVE_EXPLANATION, HN_ARCHIVE_NEW_CONFIRMATION, HN_ARCHIVE_ERRORS, HNLocalArchiveController, type HNArchiveEntry, type HNArchiveTarget, type HNArchiveDependencies } from "./hn-local-canvas-archive";
import type { HNLocalArchiveReceipt } from "../types";
import { HN_CANDIDATE_EXPLANATION, HN_CANDIDATE_HISTORICAL, HNLocalCandidateController, type HNCandidateDependencies } from "./hn-local-canvas-candidate";
import { CanvasHNLocalCandidateSection, HNLocalCandidateFacts } from "./canvas-hn-local-candidate-section";
import type { HNLocalCandidateReceipt } from "../types";

export function HNLocalArchiveContent({ entry, onArchive, candidateRunning = false }: { entry: HNArchiveEntry; onArchive: () => void; candidateRunning?: boolean }) {
    const r = entry.receipt;
    const wording = entry.running ? "正在归档本地视频…" : entry.error ? HN_ARCHIVE_ERRORS[entry.error] : entry.phase === "ARCHIVED" ? "本地字节已归档" : entry.phase === "RELOADED_UNVERIFIED" ? HN_ARCHIVE_ERRORS.HN_ARCHIVE_UNVERIFIED : entry.phase === "ARCHIVE_OUTCOME_UNKNOWN" ? HN_ARCHIVE_ERRORS.HN_ARCHIVE_OUTCOME_UNKNOWN : entry.phase === "SOURCE_CHANGED" ? HN_ARCHIVE_ERRORS.HN_ARCHIVE_SOURCE_CHANGED : entry.phase === "ARCHIVE_FAILED_RETRYABLE" ? HN_ARCHIVE_ERRORS.HN_ARCHIVE_FAILED_RETRYABLE : "尚未归档本地视频";
    const label = entry.action === "retry" ? "显式重试同一归档" : entry.action === "new" ? "创建新的本地归档" : "附加并归档本地视频";
    return <div className="space-y-4 text-sm">
        <p>{HN_ARCHIVE_EXPLANATION}</p>
        <p role="status" data-hn-archive-state={entry.phase}>{wording}</p>
        {r ? <dl className="space-y-2 break-words">
            <dt>历史冻结请求 / 本地归档回执</dt>
            <dd>ShotID：{r.shotId}</dd><dd>GenerationID：{r.generationId}</dd>
            {"resultId" in r ? <><dd>ResultID：{r.resultId}</dd><dd>ArchiveJobID：{r.archiveJobId}</dd></> : "knownJob" in r && r.knownJob ? <><dd>ResultID：{r.knownJob.resultId}</dd><dd>ArchiveJobID：{r.knownJob.archiveJobId}</dd></> : null}
            <dd>上次源字节：{r.sourceByteLength} · {r.sourceMimeType} · SHA-256：{r.sourceSHA256.slice(0, 12)}</dd>
            <dd>浏览器观察时间：{r.observedAt}</dd>
        </dl> : null}
        {entry.source ? <p>当前本地视频：{entry.source.byteLength} bytes · {entry.source.mimeType} · SHA-256：{entry.source.sha256.slice(0, 12)}</p> : null}
        {entry.action === "new" ? <p>{HN_ARCHIVE_NEW_CONFIRMATION}</p> : null}
        {entry.phase === "ARCHIVE_OUTCOME_UNKNOWN" && !entry.action ? <p>本轮无法查询未知的 Result/ArchiveJob；请停止，等待后续恢复方案。</p> : null}
        <Button type="primary" disabled={entry.running || candidateRunning || !entry.action} loading={entry.running} onClick={onArchive}>{label}</Button>
    </div>;
}
type Props = {
    open: boolean; canvasProjectId: string; nodeId: string | null; controller: HNLocalArchiveController;
    readTarget: (nodeId: string) => HNArchiveTarget; dependencies: HNArchiveDependencies;
    onReceipt: (receipt: HNLocalArchiveReceipt) => void; onClose: () => void; targetRevision: unknown;
    candidate?: { controller: HNLocalCandidateController; dependencies: HNCandidateDependencies; onReceipt: (receipt: HNLocalCandidateReceipt, archive: Extract<HNLocalArchiveReceipt, { outcome: "ARCHIVED" }>) => void };
};
export function runHNArchiveIfCandidateIdle(candidate: HNLocalCandidateController | undefined, project: string, node: string, action: () => void) {
    if (candidate?.entry(project, node).running) return false;
    action(); return true;
}
export function CanvasHNLocalArchiveDialog({ open, canvasProjectId, nodeId, controller, readTarget, dependencies, onReceipt, onClose, targetRevision, candidate }: Props) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)], { modal } = App.useApp();
    let archiveRevision = "";
    try { if (open && nodeId) { const t = readTarget(nodeId), { hnLocalCandidate: _candidate, ...metadata } = t.node.metadata || {}; archiveRevision = JSON.stringify({ ...t, node: { ...t.node, metadata } }); } } catch { /* No target cannot authorize a write. */ }
    useEffect(() => {
        if (!open || !nodeId) return;
        // Opening only reads local Blob/journal; no discovery, HTTP write or journal write.
        try { void controller.inspect(readTarget(nodeId), dependencies); } catch { /* Removed target cannot authorize a write. */ }
    }, [open, nodeId, canvasProjectId, controller, readTarget, dependencies, archiveRevision]);
    useEffect(() => {
        if (!open || !nodeId || !candidate) return;
        void candidate.controller.inspect(() => readTarget(nodeId), controller, candidate.dependencies);
    }, [open, nodeId, canvasProjectId, controller, readTarget, candidate?.controller, candidate?.dependencies, targetRevision]);
    const entry = nodeId ? controller.entry(canvasProjectId, nodeId) : null;
    const candidateEntry = nodeId && candidate ? candidate.controller.entry(canvasProjectId, nodeId) : null;
    const archive = () => {
        if (!nodeId) return;
        runHNArchiveIfCandidateIdle(candidate?.controller, canvasProjectId, nodeId, () => {
        void controller.archive(() => readTarget(nodeId), dependencies, (action, owner) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: action === "retry" ? "显式重试同一归档" : action === "new" ? "创建新的本地归档" : "附加并归档本地视频", content: <div><p>{HN_ARCHIVE_EXPLANATION}</p><p>ShotID：{owner.shotId}</p><p>GenerationID：{owner.generationId}</p><p>{owner.sourceByteLength} bytes · {owner.sourceMimeType} · SHA-256：{owner.sourceSHA256.slice(0, 12)}</p>{action === "new" ? <p>{HN_ARCHIVE_NEW_CONFIRMATION}</p> : null}</div>, okText: "确认", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }), onReceipt);
        });
    };
    const ensure = (revalidate: boolean) => {
        if (!nodeId || !candidate || controller.entry(canvasProjectId, nodeId).running) return;
        void candidate.controller.ensure(() => readTarget(nodeId), controller, candidate.dependencies, (historical) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: revalidate ? "重新确认候选" : "将已归档版本加入候选", content: <div><p>{HN_CANDIDATE_EXPLANATION}</p><p>{HN_CANDIDATE_HISTORICAL}</p><HNLocalCandidateFacts entry={{ phase: "ENSURING_CANDIDATE", running: true, inspection: 0, target: historical }} /></div>, okText: "确认", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }), candidate.onReceipt, revalidate);
    };
    return <Modal title="归档本地视频" open={open && !!nodeId} centered footer={null} onCancel={onClose} styles={{ body: { color: theme.node.text } }}>
        {entry ? <HNLocalArchiveContent entry={entry} onArchive={archive} candidateRunning={candidateEntry?.running} /> : <p>{HN_ARCHIVE_EXPLANATION}</p>}
        {candidateEntry ? <CanvasHNLocalCandidateSection entry={candidateEntry} archiveRunning={!!entry?.running} onEnsure={ensure} /> : null}
    </Modal>;
}
