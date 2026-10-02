// Non-secret metadata only. No fallback channel, protocol rename or config serialization.
export const HN_MINIMAX_OFFICIAL_IDENTITY = "minimax-official-global-v2";
export function isHNMiniMaxOfficialChannel(channel: { protocol: string; baseUrl: string; models: string[] }, model: string) {
    return model === "MiniMax-H3" && channel.protocol === "metaso" && channel.models.includes(model)
        && /^https:\/\/api\.minimax\.io(?::443)?\/?$/i.test(channel.baseUrl.trim());
}
