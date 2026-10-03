"use client";

import { useEffect, useState } from "react";
import { App, Button, Modal } from "antd";
import { canvasThemes } from "@/lib/canvas-theme";
import { useThemeStore } from "@/stores/use-theme-store";
import { HN_LOCAL_CONFIRMATION, HN_LOCAL_ERRORS, HN_LOCAL_EXPLANATION, HNLocalPrepareController, type HNLocalDependencies, type HNLocalEntry, type HNLocalIntent } from "./hn-local-canvas-prepare";
import type { HNLocalPreparedReceipt } from "../types";

export function HNLocalPrepareContent({ entry, onPrepare }: { entry: HNLocalEntry; onPrepare: () => void }) {
    const receipt = entry.receipt;
    const wording = entry.phase === "CURRENT" ? "已本地准备（PREPARED），未提交" : entry.phase === "STALE" ? HN_LOCAL_ERRORS.HN_PREPARED_STALE : entry.phase === "RELOADED_UNVERIFIED" ? HN_LOCAL_ERRORS.HN_RECEIPT_UNVERIFIED : entry.phase === "PREPARING" ? "正在冻结本地准备…" : entry.error ? HN_LOCAL_ERRORS[entry.error] : "尚未本地准备";
    return <div className="space-y-4 text-sm">
        <p>{HN_LOCAL_EXPLANATION}</p>
        <p role="status" data-hn-local-state={entry.phase}>{wording}</p>
        {receipt ? <dl className="space-y-2 break-words">
            <dt>上次冻结摘要（PREPARED 回执）</dt>
            <dd>ShotID：{receipt.shotId}</dd><dd>GenerationID：{receipt.generationId}</dd>
            <dd>FrozenHash：{receipt.frozenHash.slice(0, 12)}</dd><dd>准备时间：{receipt.preparedAt}</dd>
            <dd>模型：{receipt.snapshot.model || "未指定（本地意图）"} · 时长：{receipt.snapshot.seconds} 秒</dd>
            <dd>分辨率：{receipt.snapshot.vquality} · 尺寸/比例：{receipt.snapshot.size} · 参考数：{receipt.snapshot.referenceCount}</dd>
        </dl> : null}
        <Button type="primary" disabled={entry.running} loading={entry.running} onClick={onPrepare}>{receipt || entry.attempted ? "创建新的本地准备" : "冻结本地准备"}</Button>
    </div>;
}

type Props = {
    open: boolean; canvasProjectId: string; nodeId: string | null; controller: HNLocalPrepareController;
    readIntent: (nodeId: string) => HNLocalIntent; dependencies: HNLocalDependencies;
    onReceipt: (receipt: HNLocalPreparedReceipt) => void; onClose: () => void; intentRevision: unknown;
};
export function CanvasHNLocalPrepareDialog({ open, canvasProjectId, nodeId, controller, readIntent, dependencies, onReceipt, onClose, intentRevision }: Props) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const { modal } = App.useApp();
    const [preview, setPreview] = useState<{ model: string; seconds: string; size: string; vquality: string; references: number } | null>(null);
    useEffect(() => {
        if (!open || !nodeId) return;
        let inspecting = false;
        const inspect = () => {
            if (inspecting) return;
            try {
                const intent = readIntent(nodeId), p = intent.parameters;
                setPreview({ model: intent.model, seconds: p.videoSeconds as string, size: p.size as string, vquality: p.vquality as string, references: intent.references.length });
                inspecting = true;
                void controller.inspect(intent, dependencies).catch((error) => controller.reject(canvasProjectId, nodeId, error)).finally(() => { inspecting = false; });
            } catch (error) { setPreview(null); controller.reject(canvasProjectId, nodeId, error); }
        };
        inspect();
        // Visible-dialog local Blob recheck only: no HTTP, HN query or Provider polling.
        const timer = setInterval(inspect, 2000);
        return () => clearInterval(timer);
    }, [open, nodeId, canvasProjectId, controller, readIntent, dependencies, intentRevision]);
    const entry = nodeId ? controller.entry(canvasProjectId, nodeId) : null;
    const prepare = () => {
        if (!nodeId) return;
        void controller.prepare(() => readIntent(nodeId), dependencies,
            () => new Promise<boolean>((resolve) => { modal.confirm({ title: "创建新的本地准备", content: HN_LOCAL_CONFIRMATION, okText: "创建", cancelText: "取消", onOk: () => resolve(true), onCancel: () => resolve(false) }); }), onReceipt);
    };
    return <Modal title="本地视频准备" open={open && !!nodeId} centered footer={null} onCancel={onClose} styles={{ body: { color: theme.node.text } }}>
        {preview ? <p className="mb-4 text-sm">当前意图预览：{preview.model || "未指定（本地意图）"} · {preview.seconds} 秒 · {preview.vquality} · {preview.size} · {preview.references} 个参考</p> : null}
        {entry ? <HNLocalPrepareContent entry={entry} onPrepare={prepare} /> : <p>{HN_LOCAL_EXPLANATION}</p>}
    </Modal>;
}
