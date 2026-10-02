import assert from "node:assert/strict";
import test from "node:test";
import { defaultConfig } from "@/stores/use-config-store";
import { buildHNVideoGenerationPrepareInput } from "@/app/(user)/canvas/components/hn-video-generation-prepare";
import type { NodeGenerationContext } from "@/app/(user)/canvas/components/canvas-node-generation";
import { HN_GENERATION_SOURCE_BASELINE } from "./local-generation";
import { HN_MINIMAX_OFFICIAL_IDENTITY } from "./minimax-official-identity";

const context: NodeGenerationContext = { prompt: "synthetic frozen prompt", referenceImages: [], firstFrame: null, lastFrame: null, referenceVideos: [], referenceAudios: [], videoMultiPrompt: [], videoElementList: [], textCount: 0, imageCount: 0, videoCount: 0, audioCount: 0 };
const channel = { id: "official-h3", protocol: "metaso" as const, name: "fixture", models: ["MiniMax-H3"], baseUrl: "https://api.minimax.io", apiKey: "SYNTHETIC_NOT_SERIALIZED" };
const config = () => ({ ...defaultConfig, channelMode: "local" as const, model: "MiniMax-H3", videoModel: "MiniMax-H3", videoChannelId: channel.id, localChannels: [channel] });

test("R12 exact official H3 identity freezes nonsecret metadata and reviewed new baseline", () => {
    for (const baseUrl of ["https://api.minimax.io", "https://api.minimax.io/", "https://api.minimax.io:443/", "HTTPS://API.MINIMAX.IO", " https://api.minimax.io "]) {
        const c = { ...config(), localChannels: [{ ...channel, baseUrl }] }; const before = JSON.stringify(c);
        const result = buildHNVideoGenerationPrepareInput("test", "video", c, context);
        assert.equal(result.protocol, "metaso"); assert.equal(result.providerIdentity, HN_MINIMAX_OFFICIAL_IDENTITY);
        assert.equal(result.connectionId, channel.id); assert.equal(result.sourceBaseline, "16047f46e2186373ea824e12e84ae8dfa2ccde32");
        assert.equal(HN_GENERATION_SOURCE_BASELINE, result.sourceBaseline); assert.equal(JSON.stringify(c), before);
        for (const secret of [channel.apiKey, "api.minimax.io", "baseUrl", "apiKey", "localChannels", "Authorization"]) assert.equal(JSON.stringify(result).includes(secret), false);
    }
});

test("R12 gateways, ambiguous channel selection and non-root URLs never freeze official identity", () => {
    for (const baseUrl of ["https://metaso.cn/api/minimax", "https://api.minimax.cn", "http://api.minimax.io", "https://api.minimax.io.evil.invalid", "https://evilapi.minimax.io", "https://api.minimax.io:444", "https://user@api.minimax.io", "https://@api.minimax.io", "https://api.minimax.io/v2", "https://api.minimax.io/?x=1", "https://api.minimax.io?", "https://api.minimax.io/#", "https://api.minimax.io/../", "https://api.minimax.io\\"]) {
        const c = { ...config(), localChannels: [{ ...channel, baseUrl }] };
        assert.equal(buildHNVideoGenerationPrepareInput("test", "video", c, context).providerIdentity, "metaso");
    }
    for (const c of [{ ...config(), videoChannelId: "missing" }, { ...config(), model: "MiniMax-H3-Max" }, { ...config(), localChannels: [{ ...channel, models: [] }] }, { ...config(), channelMode: "remote" as const }]) {
        assert.notEqual(buildHNVideoGenerationPrepareInput("test", "video", c, context).providerIdentity, HN_MINIMAX_OFFICIAL_IDENTITY);
    }
    const other = { ...config(), localChannels: [{ ...channel, protocol: "ark" as const }] };
    assert.equal(buildHNVideoGenerationPrepareInput("test", "video", other, context).providerIdentity, "ark");
});
