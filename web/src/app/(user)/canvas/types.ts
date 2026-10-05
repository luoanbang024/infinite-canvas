export type Position = {
    x: number;
    y: number;
};

export type ViewportTransform = {
    x: number;
    y: number;
    k: number;
};

export enum CanvasNodeType {
    Image = "image",
    Panorama = "panorama",
    Text = "text",
    Config = "config",
    Video = "video",
    Audio = "audio",
    Director = "director",
    Group = "group",
}

export type CanvasNodeStatus = "idle" | "success" | "loading" | "error";
export type CanvasGenerationMode = "text" | "image" | "video" | "audio";
export type CanvasImageGenerationType = "generation" | "edit";

export type CameraControlOptions = {
    enabled: boolean;
    camera: string;
    lens: string;
    focalLength: number;
    aperture: number;
};

export type HNLocalPreparedReceipt = {
    version: 1; canvasProjectId: string; hnProjectId: string; sourceNodeId: string;
    shotId: string; generationId: string; frozenHash: string; preparedAt: string;
    sourceBaseline: string; requestFingerprint: string;
    snapshot: { model: string; seconds: string; vquality: string; size: string; referenceCount: number };
};

export type CanvasNodeMetadata = {
    hnLocalPrepared?: HNLocalPreparedReceipt;
    hnLocalArchive?: HNLocalArchiveReceipt;
    hnLocalCandidate?: HNLocalCandidateReceipt;
    content?: string;
    groupId?: string;
    composerContent?: string;
    prompt?: string;
    excludeUpstreamText?: boolean;
    status?: CanvasNodeStatus;
    errorDetails?: string;
    fontSize?: number;
    generationMode?: CanvasGenerationMode;
    generationType?: CanvasImageGenerationType;
    model?: string;
    workflowRef?: import("@/lib/workflow-channel").WorkflowRef;
    channelId?: string;
    size?: string;
    quality?: string;
    count?: number;
    seconds?: string;
    vquality?: string;
    mode?: string;
    negativePrompt?: string;
    generateAudio?: string;
    characterOrientation?: string;
    watermark?: string;
    audioVoice?: string;
    audioFormat?: string;
    audioSpeed?: string;
    audioInstructions?: string;
    grokTtsVoice?: string;
    grokTtsLanguage?: string;
    grokTtsFormat?: string;
    grokTtsSpeed?: string;
    glmTtsVoice?: string;
    glmTtsFormat?: string;
    glmTtsSpeed?: string;
    mimoTtsVoice?: string;
    mimoTtsFormat?: string;
    mimoVoiceDesignPrompt?: string;
    geminiTtsVoice?: string;
    mimoVoiceCloneAudioNodeId?: string;
    referenceAudioNodeId?: string;
    references?: string[];
    naturalWidth?: number;
    naturalHeight?: number;
    freeResize?: boolean;
    isBatchRoot?: boolean;
    batchRootId?: string;
    batchChildIds?: string[];
    batchUsesReferenceImages?: boolean;
    primaryImageId?: string;
    imageBatchExpanded?: boolean;
    storageKey?: string;
    mimeType?: string;
    bytes?: number;
    durationMs?: number;
    startedAt?: number;
    progress?: number;
    imageTaskId?: string;
    imageTaskResultId?: string;
    audioTaskId?: string;
    audioTaskResultId?: string;
    videoTaskId?: string;
    videoTaskVideoId?: string;
    firstFrameNodeId?: string;
    lastFrameNodeId?: string;
    multiShot?: string;
    shotType?: string;
    klingImageNodeIds?: string[];
    klingMultiPrompt?: { textNodeId?: string; duration?: string }[];
    klingElementList?: { name?: string; description?: string; nodeIds?: string[] }[];
    cameraControl?: CameraControlOptions;
    panoramaSourcePrompt?: string;
    panoramaFinalPrompt?: string;
    panoramaProjection?: "equirectangular";
    directorProject?: unknown;
};

export type HNLocalArchiveOwner = {
    version: 1; canvasProjectId: string; hnProjectId: string; sourceNodeId: string;
    shotId: string; generationId: string; preparedFrozenHash: string; attemptId: string; observedAt: string;
    sourceSHA256: string; sourceByteLength: number; sourceMimeType: "video/mp4" | "video/webm"; sourceMediaFingerprint: string;
};
export type HNLocalCandidateReceipt = {
    version: 1; canvasProjectId: string; hnProjectId: string; sourceNodeId: string;
    shotId: string; generationId: string; preparedFrozenHash: string;
    resultId: string; archiveJobId: string; sourceMediaFingerprint: string;
    candidateId: string; availabilityStatus: "ARCHIVED"; observedAt: string;
};
export type HNLocalKnownJob = { resultId: string; archiveJobId: string };
export type HNLocalArchiveReceipt = HNLocalArchiveOwner & (
    | { outcome: "NOT_ARCHIVED" }
    | { outcome: "ARCHIVING" | "ARCHIVE_OUTCOME_UNKNOWN"; knownJob: HNLocalKnownJob | null }
    | { outcome: "ARCHIVE_FAILED_RETRYABLE"; resultId: string; archiveJobId: string; resultStatus: "ARCHIVE_FAILED" | "RECEIVED"; archiveStatus: "FAILED" | "PENDING" | "COPYING" | "FINALIZING" }
    | { outcome: "ARCHIVED"; resultId: string; archiveJobId: string; resultStatus: "ARCHIVED"; archiveStatus: "ARCHIVED"; sha256: string; byteLength: number; mimeType: "video/mp4" | "video/webm" }
);

export type CanvasDirectorPanorama = {
    edgeId: string;
    sourceNodeId: string;
    imageUrl: string;
    fileName: string;
    projectionMode: "equirectangular" | "backdrop";
};

export type CanvasDirectorCapture = {
    dataUrl: string;
    fileName: string;
};

export type CanvasDirectorVideo = {
    blob: Blob;
    fileName: string;
    width: number;
    height: number;
    durationSeconds: number;
};

export type CanvasNodeData = {
    id: string;
    type: CanvasNodeType;
    title: string;
    position: Position;
    width: number;
    height: number;
    metadata?: CanvasNodeMetadata;
};

export type CanvasConnection = {
    id: string;
    fromNodeId: string;
    toNodeId: string;
};

export type CanvasAssistantReference = {
    id: string;
    type: CanvasNodeType;
    title: string;
    label?: string;
    dataUrl?: string;
    url?: string;
    storageKey?: string;
    mimeType?: string;
    text?: string;
};

export type InsertAssetPayload =
    | { kind: "text"; content: string; title: string; assetId?: string; source?: "asset" | "library" }
    | { kind: "image"; dataUrl: string; title: string; storageKey?: string; assetId?: string; width?: number; height?: number; bytes?: number; mimeType?: string; source?: "asset" | "library" }
    | { kind: "video"; url: string; title: string; storageKey?: string; assetId?: string; width?: number; height?: number; bytes?: number; mimeType?: string; source?: "asset" | "library" }
    | { kind: "audio"; url: string; title: string; storageKey?: string; assetId?: string; bytes?: number; mimeType?: string; durationMs?: number; source?: "asset" | "library" };

export type PendingAgentAsset = {
    nodeId: string;
    payload: InsertAssetPayload;
    reference: CanvasAssistantReference;
};

export type CanvasPendingAgentRequest = {
    prompt: string;
    assets: PendingAgentAsset[];
    skills: CanvasAgentSkillSelection[];
};

export type CanvasAssistantImage = {
    id: string;
    dataUrl: string;
    storageKey?: string;
    prompt: string;
    source?: "asset" | "library";
};

export type CanvasAgentSkillSelection = {
    id: string;
    name: string;
    source: "system" | "user";
};

export const MAX_CANVAS_AGENT_SKILLS = 5;

export type CanvasAgentPhase =
    | "intake"
    | "concept"
    | "script"
    | "breakdown"
    | "references"
    | "storyboard"
    | "video"
    | "audio"
    | "review"
    | "complete";

export type CanvasAgentConfig = {
    mode?: "api" | "codex";
    codexModel?: string;
    codexEffort?: string;
    textApiMode: "chat" | "responses";
    textStreaming?: boolean;
    textReasoningEnabled?: boolean;
    autoGenerateMedia: boolean;
    imageQuality: string;
    imageSize: string;
    videoQuality: string;
    videoSize: string;
};

export type CanvasAgentState = {
    phase: CanvasAgentPhase;
    brief?: string;
    targetDurationSeconds?: number;
    approvedPlan?: string;
    approvedNodeIds: string[];
    referenceNodeIds: string[];
    pendingTaskIds: string[];
    completedTaskIds: string[];
};

export type CanvasAgentContent =
    | string
    | Array<
        | { type: "text"; text: string }
        | { type: "image_url"; image_url: { url: string } }
    >;

export type CanvasAgentToolCall = {
    id: string;
    name: string;
    arguments: Record<string, unknown>;
    argumentsError?: string;
};

export type CanvasAgentProtocolMessage =
    | { role: "user" | "system"; content: CanvasAgentContent }
    | { role: "assistant"; content?: string; reasoningContent?: string; responseItems?: unknown[]; toolCalls?: CanvasAgentToolCall[] }
    | { role: "tool"; content: string; toolCallId: string; name: string };

export type CanvasAssistantMessageStatus = "thinking" | "running" | "waiting" | "success" | "error";

export type CanvasAssistantMessage = {
    id: string;
    role: "user" | "assistant";
    text: string;
    status?: CanvasAssistantMessageStatus;
    activity?: string;
    references?: CanvasAssistantReference[];
    images?: CanvasAssistantImage[];
    skills?: CanvasAgentSkillSelection[];
    skillsSelected?: boolean;
};

export type CanvasAgentJsonFallbackMode = "structured-json" | "prompt-json";
export type CanvasAgentToolMode = "native" | CanvasAgentJsonFallbackMode;

export type CanvasAssistantSession = {
    provider?: "api" | "codex";
    codexThreadId?: string;
    codexServiceId?: string;
    id: string;
    title: string;
    messages: CanvasAssistantMessage[];
    agentState: CanvasAgentState;
    protocolMessages: CanvasAgentProtocolMessage[];
    jsonToolFallbackKey?: string;
    jsonToolFallbackMode?: CanvasAgentJsonFallbackMode;
    activeSkills?: CanvasAgentSkillSelection[];
    contextCheckpoint?: string;
    createdAt: string;
    updatedAt: string;
};

export type ConnectionHandle = {
    nodeId: string;
    handleType: "source" | "target";
};

export type SelectionBox = {
    startWorldX: number;
    startWorldY: number;
    currentWorldX: number;
    currentWorldY: number;
    additive: boolean;
    initialSelectedNodeIds: string[];
};

export type ContextMenuState =
    | {
        type: "node";
        x: number;
        y: number;
        nodeId: string;
    }
    | {
        type: "connection";
        x: number;
        y: number;
        connectionId: string;
    };
