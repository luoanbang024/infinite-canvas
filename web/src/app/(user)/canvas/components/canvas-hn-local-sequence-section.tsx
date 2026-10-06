"use client";
import { Button } from "antd";
import { HN_PLACEMENT_EXPLANATION, HN_PLACEMENT_NO_READ, HN_PLACEMENT_UNKNOWN, HN_PLACEMENT_RELOAD, HN_PLACEMENT_SUCCESS, HN_PLACEMENT_DUPLICATE, HN_PLACEMENT_SELECTION_RELOAD, type PlacementEntry } from "./hn-local-canvas-sequence-placement";

export function CanvasHNLocalSequenceSection({ entry, blocked, selectionReloaded, onPlace, onLookup, onContinue, onRead }: { entry: PlacementEntry; blocked: boolean; selectionReloaded: boolean; onPlace: () => void; onLookup?: () => void; onContinue?: () => void; onRead?: () => void }) {
    const r = entry.receipt, known = entry.phase === "PLACED_CURRENT_SESSION" || entry.phase === "PLACEMENT_RELOADED_UNVERIFIED";
    const unknown = entry.protocolV2 && !entry.legacyUnknown ? "加入结果未确认；可能已经创建剪辑项，不会自动重试。可明确检查服务器结果，或确认继续同一次加入操作。" : HN_PLACEMENT_UNKNOWN;
    const status = entry.phase === "PLACEMENT_OUTCOME_UNKNOWN" ? unknown : entry.phase === "PLACEMENT_RELOADED_UNVERIFIED" ? HN_PLACEMENT_RELOAD : entry.phase === "PLACED_CURRENT_SESSION" ? HN_PLACEMENT_SUCCESS : entry.running ? "正在加入主序列…" : entry.error || (entry.phase === "SELECTION_NOT_ELIGIBLE" ? "请先明确选择一个有效的历史本地候选。" : "尚无本地确认的加入回执。");
    return <section className="mt-6 space-y-3 text-sm" aria-label="本地序列">
        <h3>本地序列</h3><p>{HN_PLACEMENT_EXPLANATION}</p><p>{entry.protocolV2 ? "服务器回执只证明这一次命令已提交；不证明当前选择、当前顺序、媒体完整性或 AI/Provider 成功。" : HN_PLACEMENT_NO_READ}</p>
        {entry.legacyUnknown ? <p>旧版加入操作仍未确认；不会迁移、重新发送或推测其结果。</p> : null}
        {selectionReloaded ? <p>{HN_PLACEMENT_SELECTION_RELOAD}</p> : null}
        <p role="status" data-hn-placement-state={entry.phase}>{status}</p>
        {known ? <p>{HN_PLACEMENT_DUPLICATE}</p> : null}
        {r ? <dl className="space-y-1 break-words"><dt>一次已确认操作的历史回执</dt><dd>SequenceID：main</dd><dd>SequenceItemID：{r.sequenceItemId}</dd><dd>创建时 orderIndex：{r.orderIndex}</dd><dd>ShotID：{r.shotId}</dd><dd>CandidateID：{r.candidateId}</dd><dd>ResultID：{r.resultId}</dd><dd>浏览器观察时间：{r.observedAt}</dd></dl> : null}
        {entry.receiptV2?.receipt.originalItem ? <dl className="space-y-1 break-words"><dt>服务器命令的历史加入回执</dt><dd>IntentID：{entry.receiptV2.command.placementIntentId}</dd><dd>SequenceItemID：{entry.receiptV2.receipt.originalItem.sequenceItemId}</dd><dd>创建时 orderIndex：{entry.receiptV2.receipt.originalItem.orderIndex}</dd></dl> : null}
        <Button type="primary" loading={entry.running} disabled={blocked || entry.running || known || entry.phase === "PLACEMENT_OUTCOME_UNKNOWN" || entry.phase === "SELECTION_NOT_ELIGIBLE"} onClick={onPlace}>加入主序列</Button>
        {entry.protocolV2 ? <div className="flex flex-wrap gap-2"><Button disabled={entry.running || !entry.recoverable} onClick={onLookup}>检查服务器结果</Button><Button disabled={entry.running || !entry.recoverable || entry.legacyUnknown} onClick={onContinue}>继续同一次加入操作</Button><Button disabled={entry.running} onClick={onRead}>读取主序列快照</Button></div> : null}
        {entry.snapshot ? <div><p>读取时的主序列快照；不用于推断未确认操作的归因。</p><ol>{entry.snapshot.items.map((i) => <li key={i.sequenceItemId}>{i.orderIndex} · {i.sequenceItemId} · {i.candidateId}</li>)}</ol></div> : null}
    </section>;
}
