import { isHNProjectId, localBackendURL } from "./local-reference";

export type HNCandidate = { candidateId: string; projectId: string; shotId: string; generationId: string; resultId: string; label: string; availabilityStatus: "ARCHIVED"; createdAt: string; updatedAt: string };
export type HNSelection = { shotId: string; selectedCandidateId: string };
export type HNSequenceItem = { sequenceItemId: string; projectId: string; sequenceId: string; orderIndex: number; shotId: string; candidateId: string; resultId: string; createdAt: string; updatedAt: string };
export type HNEditorialInput = { projectId: string; shotId: string; resultId: string; candidateLabel?: string; sequenceId?: string };
type Dependencies = { request?: typeof fetch };
const unsafeLabel = /([a-z][a-z0-9+.-]*:\/\/|^(?:[a-z]:[\\/]|[\\/])|[\x00-\x1f\x7f]|^(?:data|blob):|bearer\s|-----BEGIN|sk-proj-|ghp_|github_pat_|api[_-]?key\s*[:=]|(?:token|password|secret|authorization|cookie|credential|signature)\s*[:=])/i;
function safeLabel(value: unknown): value is string {
    if (typeof value !== "string") return false;
    const bytes = new TextEncoder().encode(value);
    return bytes.length <= 256 && new TextDecoder().decode(bytes) === value && !unsafeLabel.test(value);
}
function ids(...values: unknown[]) {
    if (values.some((value) => typeof value !== "string" || !isHNProjectId(value))) throw new Error("HN 剪辑标识无效");
}
const timestamp = (value: unknown) => typeof value === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(value) && Number.isFinite(Date.parse(value));

async function command<T>(projectId: string, path: string, payload: object, { request = fetch }: Dependencies): Promise<T> {
    const discovery = await request("/api/hn/local-endpoint", { credentials: "omit", cache: "no-store" });
    const endpoint = await discovery.json() as { code: number; data?: { url: string } };
    if (!discovery.ok || endpoint.code !== 0 || !endpoint.data) throw new Error("HN 剪辑需要本机后端地址");
    const response = await request(`${localBackendURL(endpoint.data.url)}/api/hn/projects/${encodeURIComponent(projectId)}/${path}`, {
        method: "POST", credentials: "omit", headers: { "Content-Type": "application/json", "X-HN-Local-Request": "1" }, body: JSON.stringify(payload),
    });
    const result = await response.json() as { code: number; data?: T; msg?: string };
    if (!response.ok || result.code !== 0 || !result.data) throw new Error(result.msg || "HN 剪辑操作失败");
    return result.data;
}

// Arrival is distinct from selection. Unexpected selection facts are rejected.
export async function ensureCandidate(input: { projectId: string; shotId: string; resultId: string; label?: string }, dependencies: Dependencies = {}): Promise<HNCandidate> {
    const { projectId, shotId, resultId, label = "" } = input;
    ids(projectId, shotId, resultId); if (!safeLabel(label)) throw new Error("HN Candidate 标签无效");
    const c = await command<HNCandidate>(projectId, `shots/${shotId}/candidates/ensure`, { resultId, label }, dependencies);
    ids(c.candidateId, c.generationId);
    if (c.projectId !== projectId || c.shotId !== shotId || c.resultId !== resultId || c.availabilityStatus !== "ARCHIVED" || !safeLabel(c.label) || !timestamp(c.createdAt) || !timestamp(c.updatedAt) || "selectedCandidateId" in c || "selected" in c) throw new Error("HN Candidate 返回归属或状态无效");
    return c;
}

export async function selectCandidate(input: { projectId: string; shotId: string; candidateId: string }, dependencies: Dependencies = {}): Promise<HNSelection> {
    const { projectId, shotId, candidateId } = input; ids(projectId, shotId, candidateId);
    const s = await command<HNSelection>(projectId, `shots/${shotId}/candidates/${candidateId}/select`, {}, dependencies);
    if (s.shotId !== shotId || s.selectedCandidateId !== candidateId) throw new Error("HN Candidate 选择返回归属无效");
    return s;
}

function validateItem(i: HNSequenceItem, projectId: string, sequenceId: string) {
    ids(i.sequenceItemId, i.shotId, i.candidateId, i.resultId);
    if (i.projectId !== projectId || i.sequenceId !== sequenceId || !Number.isSafeInteger(i.orderIndex) || i.orderIndex < 0 || !timestamp(i.createdAt) || !timestamp(i.updatedAt)) throw new Error("HN SequenceItem 返回归属或顺序无效");
}

export async function addSequenceItem(input: { projectId: string; sequenceId: string; candidate: HNCandidate }, dependencies: Dependencies = {}): Promise<HNSequenceItem> {
    const { projectId, sequenceId } = input; const candidate = { ...input.candidate };
    ids(projectId, sequenceId, candidate.candidateId, candidate.shotId, candidate.resultId, candidate.generationId);
    if (candidate.projectId !== projectId || candidate.availabilityStatus !== "ARCHIVED") throw new Error("HN Candidate 归属或状态无效");
    const i = await command<HNSequenceItem>(projectId, `sequences/${sequenceId}/items`, { candidateId: candidate.candidateId }, dependencies);
    validateItem(i, projectId, sequenceId);
    if (i.shotId !== candidate.shotId || i.candidateId !== candidate.candidateId || i.resultId !== candidate.resultId) throw new Error("HN SequenceItem 返回绑定无效");
    return i;
}

// Pass every current item in its desired order. Snapshot before discovery and
// reject any returned change beyond orderIndex, including updatedAt.
export async function reorderSequence(input: { projectId: string; sequenceId: string; items: HNSequenceItem[] }, dependencies: Dependencies = {}): Promise<HNSequenceItem[]> {
    const { projectId, sequenceId } = input; ids(projectId, sequenceId);
    if (!Array.isArray(input.items) || input.items.length > 256) throw new Error("HN Sequence reorder 列表无效");
    const items = input.items.map((i) => ({ ...i }));
    const seen = new Set<string>();
    for (const i of items) {
        validateItem(i, projectId, sequenceId);
        if (seen.has(i.sequenceItemId)) throw new Error("HN Sequence reorder 不接受重复 item");
        seen.add(i.sequenceItemId);
    }
    const ordered = await command<HNSequenceItem[]>(projectId, `sequences/${sequenceId}/reorder`, { sequenceItemIds: items.map((i) => i.sequenceItemId) }, dependencies);
    if (!Array.isArray(ordered) || ordered.length !== items.length) throw new Error("HN Sequence reorder 返回列表无效");
    ordered.forEach((i, index) => {
        validateItem(i, projectId, sequenceId);
        const expected = { ...items[index], orderIndex: index };
        if (Object.keys(i).sort().join(",") !== Object.keys(expected).sort().join(",") || Object.entries(expected).some(([key, value]) => (i as unknown as Record<string, unknown>)[key] !== value)) throw new Error("HN Sequence reorder 改变了顺序以外的字段");
    });
    return ordered;
}

// Explicit compound command; default sequence is documented "main", caller may override.
// No Canvas state, media read, Generation, archive or Provider dependency is accepted.
export async function commitArchivedResultToSequence(input: HNEditorialInput, dependencies: Dependencies = {}) {
    const { projectId, shotId, resultId, candidateLabel = "", sequenceId = "main" } = input;
    ids(projectId, shotId, resultId, sequenceId); if (!safeLabel(candidateLabel)) throw new Error("HN Candidate 标签无效");
    const candidate = await ensureCandidate({ projectId, shotId, resultId, label: candidateLabel }, dependencies);
    const selection = await selectCandidate({ projectId, shotId, candidateId: candidate.candidateId }, dependencies);
    const item = await addSequenceItem({ projectId, sequenceId, candidate }, dependencies);
    return { candidate, selection, item };
}
