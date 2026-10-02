import type { AiConfig } from "@/stores/use-config-store";
import type { CameraControlOptions } from "../types";
import type { NodeGenerationContext } from "./canvas-node-generation";
import { buildHNVideoGenerationPrepareInput } from "./hn-video-generation-prepare";
import { ensureLocalShot } from "@/services/hn/local-shot";
import { prepareLocalShotGeneration } from "@/services/hn/local-generation";

// Snapshot the existing R4 business projection before the first asynchronous ensure.
// This internal explicit helper adds no button, submit path or Canvas schema field.
export async function prepareHNShotVideoGeneration(projectId: string, node: { id: string; title?: string }, config: AiConfig, context: NodeGenerationContext, dependencies: Parameters<typeof prepareLocalShotGeneration>[1], cameraControl?: CameraControlOptions) {
    const input = buildHNVideoGenerationPrepareInput(projectId, node.id, config, context, cameraControl);
    const shot = await ensureLocalShot({ projectId, sourceNodeId: input.nodeId, label: node.title }, dependencies);
    return prepareLocalShotGeneration({ ...input, shotId: shot.shotId }, dependencies);
}
