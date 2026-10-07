"use client";

import { useEffect, useState } from "react";
import { App, Button, Modal } from "antd";
import { canvasThemes } from "@/lib/canvas-theme";
import { useThemeStore } from "@/stores/use-theme-store";
import { HN_ARCHIVE_EXPLANATION, HN_ARCHIVE_NEW_CONFIRMATION, HN_ARCHIVE_ERRORS, HNLocalArchiveController, type HNArchiveEntry, type HNArchiveTarget, type HNArchiveDependencies } from "./hn-local-canvas-archive";
import type { HNLocalArchiveReceipt } from "../types";
import { HN_CANDIDATE_EXPLANATION, HN_CANDIDATE_HISTORICAL, HNLocalCandidateController, type HNCandidateDependencies } from "./hn-local-canvas-candidate";
import { CanvasHNLocalCandidateSection, HNLocalCandidateFacts } from "./canvas-hn-local-candidate-section";
import type { HNLocalCandidateReceipt } from "../types";
import { HNLocalSelectionController, HN_SELECTION_EXPLANATION, HN_SELECTION_REPEAT, HN_SELECTION_NO_READ } from "./hn-local-canvas-selection";
import type { HNLocalSelectionReceipt } from "../types";
import type { HNLocalSequencePlacementReceipt } from "../types";
import { HNLocalSequencePlacementController, HN_PLACEMENT_CONFIRMATION, type PlacementProjection } from "./hn-local-canvas-sequence-placement";
import { CanvasHNLocalSequenceSection } from "./canvas-hn-local-sequence-section";
import { HNLocalSequenceReorderController, HN_REORDER_CONFIRMATION, HN_REORDER_CONTINUATION } from "./hn-local-canvas-sequence-reorder";
import { mapHNCanvasProjectId } from "./hn-local-canvas-prepare";

export function HNLocalArchiveContent({ entry, onArchive, candidateRunning = false, operationBlocked = false }: { entry: HNArchiveEntry; onArchive: () => void; candidateRunning?: boolean; operationBlocked?: boolean }) {
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
        <Button type="primary" disabled={entry.running || candidateRunning || operationBlocked || !entry.action} loading={entry.running} onClick={onArchive}>{label}</Button>
    </div>;
}
type Props = {
    open: boolean; canvasProjectId: string; nodeId: string | null; controller: HNLocalArchiveController;
    readTarget: (nodeId: string) => HNArchiveTarget; dependencies: HNArchiveDependencies;
    onReceipt: (receipt: HNLocalArchiveReceipt) => void; onClose: () => void; targetRevision: unknown;
    candidate?: { controller: HNLocalCandidateController; dependencies: HNCandidateDependencies; onReceipt: (receipt: HNLocalCandidateReceipt, archive: Extract<HNLocalArchiveReceipt, { outcome: "ARCHIVED" }>) => void };
    selection?: { controller: HNLocalSelectionController; onReceipt: (receipt: HNLocalSelectionReceipt, target: HNArchiveTarget) => void };
    reorder?: HNLocalSequenceReorderController;
    placement?: { controller: HNLocalSequencePlacementController; onReceipt: (receipt: PlacementProjection, target: HNArchiveTarget) => void };
};
export function runHNArchiveIfCandidateIdle(candidate: HNLocalCandidateController | undefined, project: string, node: string, action: () => void) {
    if (candidate?.entry(project, node).running) return false;
    action(); return true;
}
export function CanvasHNLocalArchiveDialog({ open, canvasProjectId, nodeId, controller, readTarget, dependencies, onReceipt, onClose, targetRevision, candidate, selection, placement, reorder }: Props) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)], { modal } = App.useApp();
    const [reorderProject, setReorderProject] = useState<{ canvas: string; hn: string } | null>(null);
    useEffect(() => {
        if (!open || !reorder) return;
        let active = true;
        void mapHNCanvasProjectId(canvasProjectId).then(hn => { if (active) { setReorderProject({ canvas: canvasProjectId, hn }); void reorder.inspect(hn); } }).catch(() => { if (active) setReorderProject(null); });
        return () => { active = false; };
    }, [open, canvasProjectId, reorder]); // local hashing + journal reads only; no HTTP or write on open
    const hn = reorderProject?.canvas === canvasProjectId ? reorderProject.hn : null;
    const reorderControls = reorder && hn ? {
        entry: reorder.entry(hn), canSubmit: reorder.canSubmit(hn),
        onLoad: () => { void reorder.load(hn); }, onRefresh: () => { void reorder.refresh(hn); },
        onMove: (id: string, delta: -1 | 1) => reorder.move(hn, id, delta),
        onSubmit: () => { void reorder.submit(hn, command => new Promise<boolean>(resolve => {
            modal.confirm({ title: "确认提交主序列排序", content: <div><p>{HN_REORDER_CONFIRMATION}</p><p>sequenceRevision：{command.expectedRevision}</p><p>草稿：{command.desiredSequenceItemIds.join(" → ")}</p></div>, okText: "确认排序", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        })); },
        onLookup: (intent: string) => { void reorder.lookup(hn, intent); },
        onContinue: (intent: string) => { void reorder.continueSame(hn, intent, command => new Promise<boolean>(resolve => {
            modal.confirm({ title: "继续同一次排序操作", content: <div><p>{HN_REORDER_CONTINUATION}</p><p>Intent：{command.reorderIntentId}</p><p>sequenceRevision：{command.expectedRevision}</p></div>, okText: "确认继续同一次操作", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        })); },
    } : undefined;
    let archiveRevision = "";
    try { if (open && nodeId) { const t = readTarget(nodeId), { hnLocalCandidate: _candidate, hnLocalSelection: _selection, hnLocalSequencePlacement: _placement, hnLocalSequencePlacementV2: _placementV2, ...metadata } = t.node.metadata || {}; archiveRevision = JSON.stringify({ ...t, node: { ...t.node, metadata } }); } } catch { /* No target cannot authorize a write. */ }
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
    const selectionEntry = nodeId && selection ? selection.controller.entry(canvasProjectId, nodeId) : null;
    useEffect(() => {
        if (open && nodeId && candidate && selection) void selection.controller.inspect(() => readTarget(nodeId), controller, candidate.controller, candidate.dependencies);
    }, [open, nodeId, canvasProjectId, controller, candidate?.controller, candidate?.dependencies, selection?.controller, readTarget, targetRevision, candidateEntry?.phase]);
    useEffect(() => {
        if (open && nodeId && candidate && selection && placement) void placement.controller.inspect(() => readTarget(nodeId), controller, candidate.controller, candidate.dependencies);
    }, [open, nodeId, canvasProjectId, controller, candidate?.controller, candidate?.dependencies, placement?.controller, selectionEntry?.phase, candidateEntry?.phase, readTarget, targetRevision]);
    const operationBlocked = (operation: "archive" | "candidate" | "selection" | "placement") => {
        if (!selection || !nodeId) return false;
        try { return selection.controller.coordinator.blocked(readTarget(nodeId), operation, operation === "selection" ? selectionEntry?.owner : undefined); } catch { return true; }
    };
    const archive = () => {
        if (!nodeId) return;
        const action = () => controller.archive(() => readTarget(nodeId), dependencies, (action, owner) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: action === "retry" ? "显式重试同一归档" : action === "new" ? "创建新的本地归档" : "附加并归档本地视频", content: <div><p>{HN_ARCHIVE_EXPLANATION}</p><p>ShotID：{owner.shotId}</p><p>GenerationID：{owner.generationId}</p><p>{owner.sourceByteLength} bytes · {owner.sourceMimeType} · SHA-256：{owner.sourceSHA256.slice(0, 12)}</p>{action === "new" ? <p>{HN_ARCHIVE_NEW_CONFIRMATION}</p> : null}</div>, okText: "确认", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }), onReceipt);
        if (selection) void selection.controller.runOperation("archive", () => readTarget(nodeId), controller, candidate?.controller, action);
        else runHNArchiveIfCandidateIdle(candidate?.controller, canvasProjectId, nodeId, () => { void action(); });
    };
    const ensure = (revalidate: boolean) => {
        if (!nodeId || !candidate || controller.entry(canvasProjectId, nodeId).running) return;
        const action = () => candidate.controller.ensure(() => readTarget(nodeId), controller, candidate.dependencies, (historical) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: revalidate ? "重新确认候选" : "将已归档版本加入候选", content: <div><p>{HN_CANDIDATE_EXPLANATION}</p><p>{HN_CANDIDATE_HISTORICAL}</p><HNLocalCandidateFacts entry={{ phase: "ENSURING_CANDIDATE", running: true, inspection: 0, target: historical }} /></div>, okText: "确认", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }), candidate.onReceipt, revalidate);
        if (selection) void selection.controller.runOperation("candidate", () => readTarget(nodeId), controller, candidate.controller, action);
        else void action();
    };
    const select = () => {
        if (!nodeId || !candidate || !selection) return;
        void selection.controller.select(() => readTarget(nodeId), controller, candidate.controller, candidate.dependencies, (owner) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: "选择此候选", content: <div><p>{HN_SELECTION_EXPLANATION}</p><p>{HN_SELECTION_NO_READ}</p><p>{HN_SELECTION_REPEAT}</p><p>ShotID：{owner.shotId}</p><p>CandidateID：{owner.candidateId}</p><p>ResultID：{owner.resultId}</p></div>, okText: "确认选择", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }), selection.onReceipt);
    };
    const place = () => {
        if (!nodeId || !candidate || !selection || !placement) return;
        void placement.controller.place(() => readTarget(nodeId), controller, candidate.controller, candidate.dependencies, (owner) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: "加入主序列", content: <div><p>SequenceID：main</p><p>ShotID：{owner.shotId}</p><p>CandidateID：{owner.candidateId}</p><p>ResultID：{owner.resultId}</p><p>{placement.controller.entry(canvasProjectId, nodeId).protocolV2 ? "此确认发起一个新的加入命令；不会自动重发，已确认的同一候选不重复加入。" : HN_PLACEMENT_CONFIRMATION}</p></div>, okText: "确认加入", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }), placement.onReceipt);
    };
    const recoverPlacement = (continuation: boolean) => {
        if (!nodeId || !candidate || !placement) return;
        void placement.controller.recover(() => readTarget(nodeId), candidate.dependencies, placement.onReceipt, continuation ? (owner) => new Promise<boolean>((resolve) => {
            modal.confirm({ title: "继续同一次加入操作", content: <div><p>若此前未提交，此操作可能现在新增一个剪辑项；若已经提交，只返回原回执，不会再次加入。</p><p>这是一项明确的写入操作，不是只读确认。不会生成新 intent。</p><p>CandidateID：{owner.candidateId}</p></div>, okText: "确认继续同一次操作", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) });
        }) : undefined);
    };
    return <Modal title="归档本地视频" open={open && !!nodeId} centered footer={null} onCancel={onClose} styles={{ body: { color: theme.node.text } }}>
        {entry ? <HNLocalArchiveContent entry={entry} onArchive={archive} candidateRunning={candidateEntry?.running} operationBlocked={operationBlocked("archive")} /> : <p>{HN_ARCHIVE_EXPLANATION}</p>}
        {candidateEntry ? <CanvasHNLocalCandidateSection entry={candidateEntry} archiveRunning={!!entry?.running} onEnsure={ensure} candidateBlocked={operationBlocked("candidate")} selection={selectionEntry ? { entry: selectionEntry, blocked: !!entry?.running || !!candidateEntry.running || operationBlocked("selection"), onSelect: select } : undefined} /> : null}
        {nodeId && placement ? <CanvasHNLocalSequenceSection reorder={reorderControls} entry={placement.controller.entry(canvasProjectId, nodeId)} blocked={operationBlocked("placement") || !selectionEntry || !["SELECTED_CURRENT_SESSION", "SELECTION_RELOADED_UNVERIFIED"].includes(selectionEntry.phase)} selectionReloaded={selectionEntry?.phase === "SELECTION_RELOADED_UNVERIFIED"} onPlace={place} onLookup={() => recoverPlacement(false)} onContinue={() => recoverPlacement(true)} onRead={() => { if (candidate) void placement.controller.readSnapshot(() => readTarget(nodeId), candidate.dependencies); }} /> : null}
    </Modal>;
}
