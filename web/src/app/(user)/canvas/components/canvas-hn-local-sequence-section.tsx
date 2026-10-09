"use client";
import { Button } from "antd";
import { HN_EXPORT_EXPLANATION, HN_EXPORT_UNKNOWN, HN_EXPORT_HISTORICAL, HN_EXPORT_HEALTH, type ExportEntry } from "./hn-local-canvas-sequence-export";
import { unresolvedExport, terminalExport } from "./hn-local-sequence-export-journal";
import { HN_REORDER_LOAD, HN_REORDER_EXPLANATION, HN_REORDER_UNKNOWN, type ReorderEntry } from "./hn-local-canvas-sequence-reorder";
import { unresolvedReorder } from "./hn-local-sequence-reorder-journal";
import { HN_PLACEMENT_EXPLANATION, HN_PLACEMENT_NO_READ, HN_PLACEMENT_UNKNOWN, HN_PLACEMENT_RELOAD, HN_PLACEMENT_SUCCESS, HN_PLACEMENT_DUPLICATE, HN_PLACEMENT_SELECTION_RELOAD, type PlacementEntry } from "./hn-local-canvas-sequence-placement";

export function CanvasHNLocalSequenceSection({ entry, blocked, selectionReloaded, onPlace, onLookup, onContinue, onRead, reorder, sequenceExport }: { entry: PlacementEntry; blocked: boolean; selectionReloaded: boolean; onPlace: () => void; onLookup?: () => void; onContinue?: () => void; onRead?: () => void; reorder?: ReorderSectionProps; sequenceExport?: ExportSectionProps }) {
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
        {reorder ? <CanvasHNLocalReorderSection {...reorder} /> : null}
        {sequenceExport ? <CanvasHNLocalExportSection {...sequenceExport} /> : null}
    </section>;
}

export type ReorderSectionProps = { entry: ReorderEntry; canSubmit: boolean; onLoad: () => void; onRefresh: () => void; onMove: (id: string, delta: -1 | 1) => void; onSubmit: () => void; onLookup: (intent: string) => void; onContinue: (intent: string) => void };
export function CanvasHNLocalReorderSection({ entry: e, canSubmit, onLoad, onRefresh, onMove, onSubmit, onLookup, onContinue }: ReorderSectionProps) {
    const unresolved = e.records.filter(unresolvedReorder), locked = e.running || e.storageBlocked || unresolved.length > 0;
    return <section className="mt-6 space-y-3" aria-label="主序列安全排序">
        <h4>主序列安全排序</h4><p>{HN_REORDER_EXPLANATION}</p>
        <p>排序作用于整个 HN 项目的 main 序列，与打开此对话框的视频节点无关。</p>
        <div className="flex flex-wrap gap-2"><Button disabled={e.running} onClick={onLoad}>{HN_REORDER_LOAD}</Button><Button disabled={e.running || !e.snapshot} onClick={onRefresh}>重新读取当前顺序（重置草稿）</Button></div>
        {e.error ? <p role="status">{e.error}</p> : null}
        {e.running ? <p role="status">正在处理安全排序…</p> : null}
        <p>{e.currentVerified ? "当前主序列快照（读取时）" : "当前顺序未读取"}</p>
        {e.snapshot ? <div><p>SequenceID：main · sequenceRevision：{e.snapshot.sequenceRevision}</p><p>上次读取的顺序：{e.snapshot.items.map(i => i.sequenceItemId).join(" → ") || "空序列"}</p><p>草稿仅在本页暂存；上移、下移不发送请求。确认后才提交排序。</p>
            <ol>{e.draft.map((id, index) => { const item = e.snapshot!.items.find(i => i.sequenceItemId === id)!; return <li key={id} className="space-y-1 break-words"><p>草稿 {index + 1} · 读取时 {e.snapshot!.items.indexOf(item) + 1} · SequenceItemID：{id} · CandidateID：{item.candidateId} · ShotID：{item.shotId}</p><Button disabled={locked || index === 0} onClick={() => onMove(id, -1)}>上移</Button><Button disabled={locked || index === e.draft.length - 1} onClick={() => onMove(id, 1)}>下移</Button></li>; })}</ol>
        </div> : null}
        <Button type="primary" disabled={!canSubmit || locked} onClick={onSubmit}>确认提交排序</Button>
        {unresolved.length ? <div><p role="status">{HN_REORDER_UNKNOWN}</p><p>存在未确认命令时禁止创建新的排序命令；NOT_OBSERVED 也不能解除限制。</p>{unresolved.map(r => <div key={r.command.reorderIntentId}><p>Intent：{r.command.reorderIntentId.slice(0,8)} · 预期 revision：{r.command.expectedRevision}</p><Button disabled={e.running} onClick={() => onLookup(r.command.reorderIntentId)}>检查服务器结果</Button><Button disabled={e.running} onClick={() => onContinue(r.command.reorderIntentId)}>继续同一次排序操作</Button></div>)}</div> : null}
        {e.records.filter(r => !!r.receipt).map(r => <div key={r.command.reorderIntentId} className="break-words" data-hn-reorder-outcome={r.state}><p>历史排序命令回执 · {r.command.reorderIntentId}</p><p>{r.state === "COMMITTED" ? `此命令已提交于历史 revision ${r.receipt!.appliedRevision}` : "排序冲突，未覆盖较新的状态；此命令没有排序效果。请读取新快照并重新编辑、确认。"}</p><p>历史回执不证明当前顺序、媒体完整性或 AI/Provider 成功。</p>{r.state === "COMMITTED" && e.currentVerified && e.snapshot && (BigInt(e.snapshot.sequenceRevision) > BigInt(r.receipt!.appliedRevision!) || JSON.stringify(e.snapshot.items.map(i => i.sequenceItemId)) !== JSON.stringify(r.command.desiredSequenceItemIds)) ? <p>序列已发生后续变化；不重新应用历史顺序。</p> : null}</div>)}
        {e.storageBlocked ? <p>日志完整性无法确认；已停止排序写入。</p> : null}
    </section>;
}

export type ExportSectionProps = { entry: ExportEntry; canSubmit: boolean; onLoad: () => void; onRefresh: () => void; onSubmit: () => void; onLookup: (intent: string) => void; onContinue: (intent: string) => void; onVerify: (intent: string) => void };
export function CanvasHNLocalExportSection({ entry: e, canSubmit, onLoad, onRefresh, onSubmit, onLookup, onContinue, onVerify }: ExportSectionProps) {
    const unresolved = e.records.filter(unresolvedExport);
    return <section className="mt-6 space-y-3" aria-label="主序列安全导出">
        <h4>主序列安全导出</h4><p>{HN_EXPORT_EXPLANATION}</p>
        <div className="flex flex-wrap gap-2"><Button disabled={e.running} onClick={onLoad}>加载安全导出</Button><Button disabled={e.running || !e.snapshot} onClick={onRefresh}>重新读取导出快照</Button></div>
        {e.error ? <p role="status">{e.error}</p> : null}
        {e.running ? <p role="status">正在处理安全导出；不会自动发送下一次请求…</p> : null}
        <p>{e.currentVerified ? "当前主序列导出快照（读取时）" : "当前主序列状态未核验"}</p>
        {e.snapshot ? <div><p>SequenceID：main · sequenceRevision：{e.snapshot.sequenceRevision} · 项数：{e.snapshot.items.length}</p><p>仅为读取时的预览；导出顺序不可编辑。如需改变顺序，请先使用安全排序，再重新读取导出快照。</p><ol>{e.snapshot.items.map((i, index) => <li key={i.sequenceItemId} className="break-words">{index + 1} · SequenceItemID：{i.sequenceItemId} · CandidateID：{i.candidateId} · ShotID：{i.shotId}</li>)}</ol>{e.snapshot.items.length === 0 ? <p>主序列为空，不能提交导出。</p> : null}</div> : null}
        <Button type="primary" disabled={!canSubmit || e.running || e.storageBlocked || unresolved.length > 0} onClick={onSubmit}>确认导出当前主序列</Button>
        {unresolved.length ? <div><p>存在未终结的导出命令，禁止创建新 intent；逐条检查或明确继续，不能按相似快照合并。</p>{unresolved.map(r => <div key={r.command.exportIntentId} className="break-words" data-hn-export-state={r.state}><p>Intent：{r.command.exportIntentId.slice(0, 8)} · 预期 revision：{r.command.expectedRevision}</p><p>{r.state === "KNOWN_NONTERMINAL" ? `服务器状态：${r.status!.outcome} · ExportID：${r.status!.exportId || "尚未保留"} · attemptCount：${r.status!.attemptCount} · ${r.status!.errorClass || "无静态错误"}` : r.state === "PREPARED" ? "命令已在浏览器准备；不会自动发送。" : HN_EXPORT_UNKNOWN}</p><Button disabled={e.running} onClick={() => onLookup(r.command.exportIntentId)}>检查服务器状态</Button><Button disabled={e.running} onClick={() => onContinue(r.command.exportIntentId)}>继续同一次导出</Button></div>)}</div> : null}
        {e.records.filter(terminalExport).map(r => { const receipt = r.status!.receipt!, health = e.health[r.command.exportIntentId]; const later = e.currentVerified && e.snapshot && (e.snapshot.sequenceRevision !== r.command.expectedRevision || JSON.stringify(e.snapshot.items.map(i => i.sequenceItemId)) !== JSON.stringify(r.command.orderedSequenceItemIds)); return <div key={r.command.exportIntentId} className="space-y-2 break-words" data-hn-export-state={r.state}>
            <p>历史导出命令 · {r.command.exportIntentId} · {r.state}</p>
            {r.state === "COMMITTED" ? <><p>{HN_EXPORT_HISTORICAL}</p><dl><dt>历史导出回执</dt><dd>ExportID：{receipt.exportId}</dd><dd>历史 sequenceRevision：{receipt.expectedRevision} · itemCount：{receipt.itemCount}</dd><dd>项目相对导出目录：{receipt.bundleRelativePath}</dd><dd>JSON：{receipt.manifestJsonRelativePath}</dd><dd>CSV：{receipt.manifestCsvRelativePath}</dd></dl>{later ? <p>当前主序列已发生后续变化；历史导出仍对应原快照。</p> : null}<Button disabled={e.running} onClick={() => onVerify(r.command.exportIntentId)}>验证当前导出包</Button><p>历史回执与当前导出包健康分开；验证可能读取整个本地导出包，仅在明确点击后执行。</p>{health ? <p role="status">{HN_EXPORT_HEALTH[health.health]} · 验证观察时间：{health.observedAt}</p> : <p>当前导出包尚未在本页显式验证。</p>}</> : r.state === "REJECTED" ? <p>此命令被拒绝，未创建导出包。若序列 revision 或有序项已变化，请重新读取导出快照；不会自动重新提交。静态原因：{receipt.errorClass}</p> : <p>此 exact 命令已达到终态本地源完整性失败；没有 COMMITTED 导出回执。这不是 UNKNOWN，不会自动创建新导出。</p>}
        </div>; })}
        {e.storageBlocked ? <p>导出日志完整性无法确认；已停止新导出写入。</p> : null}
    </section>;
}
