import { CanvasNodeType, type CanvasNodeData } from "../types";
import { isHNProjectId } from "@/services/hn/local-reference";
import type { LocalArchiveInput } from "@/services/hn/local-result-archive";

// Explicitly attach exact local bytes to a supplied Generation. This is not
// evidence that the node was produced by that Generation or by a Provider.
export function buildHNLocalResultArchiveInput(projectId: string, generationId: string, node: CanvasNodeData): LocalArchiveInput {
    if (!isHNProjectId(projectId) || !isHNProjectId(generationId)) throw new Error("需要显式同项目 HN Generation 标识");
    const storageKey = node.metadata?.storageKey;
    if (node.type !== CanvasNodeType.Video || !node.metadata?.content || (node.metadata.status && node.metadata.status !== "success") || !storageKey || !/^(video|file):[^\s:/\\?&#]+$/.test(storageKey)) throw new Error("需要已有媒体内容的成功/本地视频节点");
    return { projectId, generationId, storageKey };
}
