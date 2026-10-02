import type { NodeGenerationContext } from "./canvas-node-generation";
import type { CameraControlOptions } from "../types";
import { applyCameraPrompt } from "../utils/canvas-camera";
import { channelProtocolForConfig, type AiConfig } from "@/stores/use-config-store";
import { canonicalHNParameters, HN_GENERATION_SOURCE_BASELINE, type HNGenerationPrepareInput } from "@/services/hn/local-generation";
import { HN_MINIMAX_OFFICIAL_IDENTITY, isHNMiniMaxOfficialChannel } from "@/services/hn/minimax-official-identity";

// Consumes the same effective config/context as Canvas video generation, without
// entering its live submit path or copying URLs, channel configs or credentials.
export function buildHNVideoGenerationPrepareInput(projectId: string, nodeId: string, config: AiConfig, context: NodeGenerationContext, cameraControl?: CameraControlOptions): HNGenerationPrepareInput {
    if (config.videoWorkflowRef || context.videoElementList.length || config.videoElementList.some((element) => element.name || element.description || element.references.length)) throw new Error("R4 暂不支持 workflow 或 element 媒体包");
    if (context.referenceVideos.length || context.referenceAudios.length) throw new Error("R4 暂不支持视频或音频参考");
    const references: HNGenerationPrepareInput["references"] = [];
    const add = (image: NodeGenerationContext["firstFrame"], role: HNGenerationPrepareInput["references"][number]["role"]) => {
        if (!image) return;
        if (!image.storageKey?.startsWith("image:")) throw new Error("R4 需要精确本地图片 Blob，不支持 server/remote 参考");
        references.push({ storageKey: image.storageKey, role });
    };
    context.referenceImages.forEach((image) => add(image, "reference"));
    add(context.firstFrame, "firstFrame"); add(context.lastFrame, "lastFrame");
    const protocol = channelProtocolForConfig(config);
    const connectionId = config.videoChannelId || config.activeChannelId || undefined;
    const channel = config.channelMode === "local" ? config.localChannels.find((item) => item.id === connectionId) : undefined;
    const providerIdentity = protocol === "metaso" && channel && isHNMiniMaxOfficialChannel(channel, config.model) ? HN_MINIMAX_OFFICIAL_IDENTITY : protocol;
    return {
        projectId, nodeId, promptSnapshot: applyCameraPrompt(context.prompt.trim(), cameraControl), protocol, providerIdentity,
        model: config.model, connectionId,
        sourceBaseline: HN_GENERATION_SOURCE_BASELINE, references,
        parameters: canonicalHNParameters({ size: config.size, videoSeconds: config.videoSeconds, vquality: config.vquality, videoMode: config.videoMode, videoNegativePrompt: config.videoNegativePrompt, videoMultiShot: config.videoMultiShot, videoShotType: config.videoShotType, videoMultiPrompt: context.videoMultiPrompt.map(({ prompt, duration }) => ({ prompt, duration })), videoGenerateAudio: config.videoGenerateAudio, videoWatermark: config.videoWatermark, videoCharacterOrientation: config.videoCharacterOrientation }),
    };
}
