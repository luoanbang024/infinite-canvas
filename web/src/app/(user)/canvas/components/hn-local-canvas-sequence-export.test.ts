import { test, after } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { exportCanonical, type ExportCommand, type ExportStatus, type ExportBundleHealth } from "@/services/hn/local-export-commands";
import { HNLocalSequenceExportController } from "./hn-local-canvas-sequence-export";
import { exportJournalKey, unresolvedExport, type ExportJournalRecord } from "./hn-local-sequence-export-journal";
import { exportMemory, exportRecord, exportStatus, r34Evidence } from "./hn-local-sequence-export-journal.test";
export function exportFixture(count = 2) {
    const project = "project", m = exportMemory(), counts = { http: 0, discovery: 0, initializeReorder: 0, initializeExport: 0, snapshot: 0, post: 0, lookup: 0, verify: 0, uuid: 0, forbidden: 0 };
    let current = Array.from({ length: count }, (_, i) => "item-" + i), revision = "3", mode = "ok", discoveryFail = false, snapshotFail = false, outcome: ExportStatus["outcome"] = "COMMITTED", lookupMode = "normal", health: ExportBundleHealth["health"] = "VERIFIED", tick = 0;
    const stored = new Map<string, ExportStatus>(), posts: ExportCommand[] = [], order: string[] = [], projections: string[] = [];
    const response = (data: unknown) => Response.json({ code: 0, data, msg: "ok" });
    const request = (async (input: Parameters<typeof fetch>[0], init?: RequestInit) => {
        const url = String(input); counts.http++;
        assert.equal(init?.credentials, "omit"); assert.equal(init?.redirect, "error"); assert.equal(init?.cache, "no-store");
        if (url === "/api/hn/local-endpoint") { counts.discovery++; if (discoveryFail) throw Error("synthetic discovery unavailable"); return response({ url: "http://127.0.0.1:3018" }); }
        const path = new URL(url).pathname, owner = decodeURIComponent(path.split("/")[4]); assert.equal((init?.headers as Record<string, string>)["X-HN-Local-Request"], "1");
        if (path.endsWith("/reorder-protocol/initialize")) { counts.initializeReorder++; order.push("R30-init"); return response({ protocolVersion: 1, projectId: owner, sequenceId: "main", sequenceRevision: revision }); }
        if (path.endsWith("/export-protocol/initialize")) { counts.initializeExport++; order.push("R33-init"); return response({ protocolVersion: 1, projectId: owner, sequenceId: "main" }); }
        if (path.endsWith("/reorder-snapshot")) { counts.snapshot++; order.push("snapshot"); if (snapshotFail) throw Error("synthetic snapshot unavailable"); return response({ protocolVersion: 1, projectId: owner, sequenceId: "main", sequenceRevision: revision, itemsComplete: true, items: current.map((id, orderIndex) => ({ sequenceItemId: id, projectId: owner, sequenceId: "main", orderIndex, shotId: "shot-" + id, candidateId: "candidate-" + id, resultId: "result-" + id, createdAt: "2026-10-09T00:00:00Z", updatedAt: "2026-10-09T00:00:00Z" })) }); }
        if (path.endsWith("/export-commands") && init?.method === "POST") {
            const c = JSON.parse(init.body as string) as ExportCommand, disk = m.values.get(exportJournalKey(owner, c.exportIntentId)) as ExportJournalRecord;
            assert.equal(disk.state, "DISPATCHING"); assert.deepEqual(disk.command, c); assert.equal(m.events.at(-1), "read:DISPATCHING"); m.events.push("actual-export-POST"); counts.post++; posts.push(structuredClone(c));
            if (mode === "drop-before") throw Error("synthetic dispatch ambiguity before reservation");
            const previous = stored.get(c.exportIntentId); if (previous) assert.equal(exportCanonical(previous.command!), exportCanonical(c));
            const status = previous?.receipt ? previous : await exportStatus(owner, c, outcome, previous?.exportId); stored.set(c.exportIntentId, status);
            if (mode === "drop-after") throw Error("synthetic response lost after commit"); if (mode === "invalid") return new Response("{", { headers: { "Content-Type": "application/json" } });
            return response(status);
        }
        if (path.endsWith("/bundle-verification")) { counts.verify++; const id = path.split("/").at(-2)!, s = stored.get(id)!; return response({ protocolVersion: 1, projectId: owner, sequenceId: "main", exportIntentId: id, exportId: s.exportId, health }); }
        if (path.includes("/export-commands/") && init?.method === "GET") {
            counts.lookup++; const id = path.split("/").at(-1)!, c = posts.find(c => c.exportIntentId === id) || (m.values.get(exportJournalKey(owner, id)) as ExportJournalRecord)?.command;
            if (lookupMode === "error") throw Error("synthetic lookup unavailable");
            if (lookupMode === "absent" || !stored.has(id)) return response(await exportStatus(owner, c, "NOT_OBSERVED"));
            if (lookupMode !== "normal") return response(await exportStatus(owner, c, lookupMode as ExportStatus["outcome"], stored.get(id)?.exportId));
            return response(stored.get(id));
        }
        counts.forbidden++; assert.fail("unexpected local route");
    }) as typeof fetch;
    const deps = { journal: m.journal, request, uuid: () => { counts.uuid++; return crypto.randomUUID(); }, now: () => new Date(Date.UTC(2026, 9, 9, 2, 0, 0, tick++)).toISOString() };
    let c: HNLocalSequenceExportController;
    c = new HNLocalSequenceExportController(() => { for (const r of c.entry(project).records.filter(r => ["COMMITTED", "REJECTED", "FAILED"].includes(r.state))) { assert.deepEqual(m.values.get(exportJournalKey(project, r.command.exportIntentId)), r); projections.push(r.state); } }, deps);
    return { project, m, counts, stored, posts, order, projections, deps, c, entry: () => c.entry(project), load: () => c.load(project), submit: (confirm: (c: ExportCommand) => Promise<boolean> = async () => true) => c.submit(project, confirm), intent: () => c.entry(project).records[0].command.exportIntentId, setMode: (v: string) => { mode = v; }, setDiscovery: (v: boolean) => { discoveryFail = v; }, setSnapshotFailure: (v: boolean) => { snapshotFail = v; }, setOutcome: (v: ExportStatus["outcome"]) => { outcome = v; }, setLookup: (v: string) => { lookupMode = v; }, setHealth: (v: ExportBundleHealth["health"]) => { health = v; }, setCurrent: (ids: string[], rev: string) => { current = [...ids]; revision = rev; } };
}
const facts: unknown[] = [];
test("R34 inspect/open is journal read only; explicit ordered load and readonly refresh", async () => {
    const f = exportFixture(); assert.equal(await f.c.inspect(f.project), true); assert.equal(f.counts.http, 0); assert.equal(f.m.values.size, 0); assert.ok(!f.m.events.some(x => x.startsWith("write:")));
    assert.equal(await f.load(), true); assert.deepEqual(f.order, ["R30-init", "R33-init", "snapshot"]); assert.equal(f.counts.post, 0); assert.equal(f.counts.verify, 0); assert.equal(f.entry().snapshot?.sequenceRevision, "3"); assert.deepEqual(f.entry().snapshot?.items.map(i => i.sequenceItemId), ["item-0", "item-1"]);
    await f.c.refresh(f.project); assert.equal(f.counts.initializeReorder, 1); assert.equal(f.counts.initializeExport, 1); facts.push({ scenario: "open-load-refresh", counts: f.counts, order: f.order, snapshot: f.entry().snapshot });
});
test("R34 snapshot 0/1/256/257 and invalid authority bounds", async () => {
    for (const n of [0, 1, 256, 257]) { const f = exportFixture(n); assert.equal(await f.load(), n <= 256); assert.equal(f.c.canSubmit(f.project), n >= 1 && n <= 256); if (n === 1 || n === 256) { assert.equal(await f.submit(), true); assert.equal(f.posts[0].orderedSequenceItemIds.length, n); } assert.equal(f.counts.post, n === 1 || n === 256 ? 1 : 0); }
    for (const [ids, revision] of [[["same", "same"], "3"], [["A"], "03"], [["A"], "9223372036854775808"]] as [string[], string][]) { const f = exportFixture(); f.setCurrent(ids, revision); assert.equal(await f.load(), false); assert.equal(f.c.canSubmit(f.project), false); assert.equal(f.counts.post, 0); }
});
test("R34 cancellation zero UUID/persistence/POST; confirmation mutation cannot change command", async () => {
    const f = exportFixture(); await f.load(); assert.equal(await f.submit(async () => false), false); assert.equal(f.counts.uuid, 0); assert.equal(f.counts.post, 0); assert.equal(f.m.values.size, 0);
    await f.submit(async c => { c.expectedRevision = "9"; c.orderedSequenceItemIds.reverse(); return true; }); assert.equal(f.posts[0].expectedRevision, "3"); assert.deepEqual(f.posts[0].orderedSequenceItemIds, ["item-0", "item-1"]);
});
test("R34 PREPARED/DISPATCHING set failure, mutated write, strict readback failure gate POST zero", async () => {
    for (const state of ["PREPARED", "DISPATCHING"] as const) for (const kind of ["write", "readback", "mutate-write"]) {
        const f = exportFixture(); await f.load(); const set = f.m.journal.setItem, get = f.m.journal.getItem; let written = false;
        f.m.journal.setItem = async (k, r) => { if (r.state === state) { written = true; if (kind === "write") throw Error("synthetic storage unavailable"); if (kind === "mutate-write") r.command.expectedRevision = "9"; } return set(k, r); };
        f.m.journal.getItem = async k => { const r = await get(k) as ExportJournalRecord | null; return kind === "readback" && written && r?.state === state ? { ...r, command: { ...r.command, expectedRevision: "9" } } : r; };
        assert.equal(await f.submit(), false); assert.equal(f.counts.post, 0); facts.push({ scenario: state + "-" + kind, counts: f.counts, entry: f.entry(), events: f.m.events });
    }
});
test("R34 discovery failure after PREPARED is ABORTED with zero POST and explicit later new intent", async () => {
    const f = exportFixture(); await f.load(); f.setDiscovery(true); assert.equal(await f.submit(), false); assert.equal(f.counts.post, 0); assert.equal(f.entry().records[0].state, "ABORTED_PRE_DISPATCH"); f.setDiscovery(false); assert.equal(await f.submit(), true); assert.equal(f.counts.uuid, 2); assert.equal(f.counts.post, 1); facts.push({ scenario: "pre-dispatch-aborted", records: f.entry().records, events: f.m.events });
});
test("R34 terminal responses durably sealed before success; no automatic verification", async () => {
    for (const outcome of ["COMMITTED", "REJECTED", "FAILED"] as const) { const f = exportFixture(); await f.load(); f.setOutcome(outcome); assert.equal(await f.submit(), true); assert.equal(f.counts.post, 1); assert.equal(f.counts.verify, 0); assert.equal(f.entry().records[0].state, outcome); assert.ok(f.projections.includes(outcome)); assert.equal(f.entry().records.some(unresolvedExport), false); facts.push({ scenario: "terminal-" + outcome, records: f.entry().records, events: f.m.events, counts: f.counts }); }
});
test("R34 every known nonterminal durably remains unresolved including early IN_PROGRESS", async () => {
    for (const outcome of ["IN_PROGRESS", "RESERVED", "COPYING", "READY_TO_FINALIZE", "RETRYABLE", "RECOVERY_BLOCKED"] as const) { const f = exportFixture(); await f.load(); f.setOutcome(outcome); assert.equal(await f.submit(), true); assert.equal(f.entry().records[0].state, "KNOWN_NONTERMINAL"); assert.equal(f.entry().records[0].status?.outcome, outcome); assert.equal(f.c.canSubmit(f.project), false); assert.equal(await f.submit(), false); assert.equal(f.counts.post, 1); assert.equal(f.counts.uuid, 1); facts.push({ scenario: "nonterminal-" + outcome, records: f.entry().records }); }
});
test("R34 dispatched drop/invalid/terminal durability failure UNKNOWN and no automatic retry", async () => {
    for (const mode of ["drop-before", "drop-after", "invalid", "terminal-write", "terminal-read", "unknown-write"]) {
        const f = exportFixture(); await f.load(); f.setMode(mode.startsWith("terminal") ? "ok" : mode === "unknown-write" ? "drop-after" : mode); const set = f.m.journal.setItem, get = f.m.journal.getItem;
        f.m.journal.setItem = async (k, r) => { if ((mode === "terminal-write" && r.state === "COMMITTED") || (mode === "unknown-write" && r.state === "UNKNOWN")) throw Error("synthetic seal failure"); return set(k, r); };
        f.m.journal.getItem = async k => { const r = await get(k) as ExportJournalRecord | null; if (mode === "terminal-read" && r?.state === "COMMITTED") throw Error("synthetic read failure"); return r; };
        assert.equal(await f.submit(), false); assert.equal(f.entry().records[0].state, "UNKNOWN"); assert.equal(f.counts.post, 1); assert.equal(await f.submit(), false); assert.equal(f.counts.uuid, 1); assert.equal(f.projections.includes("COMMITTED"), false); facts.push({ scenario: mode, entry: f.entry(), counts: f.counts, disk: [...f.m.values] });
        if (mode === "unknown-write") { const reopened = new HNLocalSequenceExportController(() => {}, f.deps); await reopened.inspect(f.project); assert.equal(reopened.entry(f.project).records[0].state, "DISPATCHING"); }
    }
});
test("R34 reload DISPATCHING/UNKNOWN remain unresolved; lookup absent/error no write or POST", async () => {
    for (const state of ["DISPATCHING", "UNKNOWN"] as const) { const f = exportFixture(); await f.load(); f.setMode("drop-after"); await f.submit(); const key = exportJournalKey(f.project, f.intent()), r = f.m.values.get(key) as ExportJournalRecord; f.m.values.set(key, { ...r, state });
        const c = new HNLocalSequenceExportController(() => {}, f.deps); await c.inspect(f.project); assert.equal(c.entry(f.project).records[0].state, state); f.setLookup("absent"); const before = JSON.stringify([...f.m.values]); assert.equal(await c.lookup(f.project, f.intent()), false); assert.equal(JSON.stringify([...f.m.values]), before); assert.ok(c.entry(f.project).error?.includes("不能证明")); f.setLookup("error"); assert.equal(await c.lookup(f.project, f.intent()), false); assert.equal(f.counts.post, 1); assert.equal(c.entry(f.project).records[0].state, state); facts.push({ scenario: "reload-" + state, entry: c.entry(f.project), counts: f.counts }); }
});
test("R34 exact lookup terminal and all known nonterminal recovery without POST", async () => {
    for (const outcome of ["COMMITTED", "REJECTED", "FAILED", "COPYING", "RETRYABLE", "RECOVERY_BLOCKED"] as const) { const f = exportFixture(); await f.load(); f.setMode("drop-after"); await f.submit(); f.setLookup(outcome); assert.equal(await f.c.lookup(f.project, f.intent()), true); assert.equal(f.entry().records[0].status?.outcome, outcome); assert.equal(f.counts.post, 1); assert.equal(f.counts.uuid, 1); facts.push({ scenario: "lookup-" + outcome, entry: f.entry(), counts: f.counts }); }
});
test("R34 same-key continuation exact payload, no UUID, one POST per confirmation; later sequence ignored", async () => {
    for (const outcome of ["COMMITTED", "RETRYABLE", "RECOVERY_BLOCKED"] as const) { const f = exportFixture(); await f.load(); f.setMode("drop-before"); await f.submit(); const original = structuredClone(f.posts[0]); f.setCurrent(["later"], "9"); f.setMode("ok"); f.setOutcome(outcome); assert.equal(await f.c.continueSame(f.project, f.intent(), async () => false), false); assert.equal(f.counts.post, 1); assert.equal(await f.c.continueSame(f.project, f.intent(), async c => { assert.deepEqual(c, original); c.expectedRevision = "999"; return true; }), true); assert.deepEqual(f.posts[1], original); assert.equal(f.counts.uuid, 1); assert.equal(f.counts.post, 2); assert.equal(f.entry().records[0].status?.outcome, outcome); facts.push({ scenario: "continuation-" + outcome, posts: f.posts, counts: f.counts, records: f.entry().records }); }
    const f = exportFixture(); await f.load(); f.setMode("drop-before"); await f.submit(); assert.equal(await f.c.continueSame(f.project, f.intent(), async () => true), false); assert.equal(f.counts.post, 2); assert.equal(f.entry().records[0].state, "UNKNOWN"); assert.equal(f.counts.uuid, 1);
});
test("R34 multiple unresolved preserve independent commands and cache barrier after journal drift", async () => {
    const f = exportFixture(), a = { ...exportRecord(), state: "UNKNOWN" as const }, b = { ...exportRecord(), state: "DISPATCHING" as const }; for (const r of [a, b]) f.m.values.set(exportJournalKey(f.project, r.command.exportIntentId), r);
    f.stored.set(a.command.exportIntentId, await exportStatus(f.project, a.command)); await f.load(); assert.equal(await f.submit(), false); assert.equal(f.counts.uuid, 0); await f.c.lookup(f.project, a.command.exportIntentId); assert.equal(f.entry().records.length, 2); assert.equal(f.entry().records.filter(unresolvedExport).length, 1); assert.equal(f.m.values.size, 2); assert.equal(f.counts.post, 0);
    f.m.values.delete(exportJournalKey(f.project, b.command.exportIntentId)); await f.c.inspect(f.project); assert.equal(f.entry().records.filter(unresolvedExport).length, 1); assert.equal(f.c.canSubmit(f.project), false); assert.equal(f.c.entry("other").records.length, 0); facts.push({ scenario: "multi-intent-drift", records: f.entry().records, counts: f.counts });
});
test("R34 terminal history allows separately confirmed new intent even same snapshot; fresh current state separate", async () => {
    const f = exportFixture(); await f.load(); await f.submit(); const first = structuredClone(f.entry().records[0]), posts = f.counts.post; assert.equal(await f.submit(async () => false), false); assert.equal(f.counts.post, posts); await f.submit(); assert.equal(f.counts.uuid, 2); assert.equal(f.counts.post, 2); assert.notEqual(f.posts[0].exportIntentId, f.posts[1].exportIntentId); assert.equal(exportCanonical(f.posts[0]), exportCanonical(f.posts[1]));
    f.setCurrent(["later"], "9"); await f.c.refresh(f.project); assert.deepEqual(f.entry().records.find(r => r.command.exportIntentId === first.command.exportIntentId), first); assert.equal(f.entry().snapshot?.sequenceRevision, "9"); f.setSnapshotFailure(true); assert.equal(await f.c.refresh(f.project), false); assert.equal(f.entry().currentVerified, false); assert.equal(f.entry().records[0].state, "COMMITTED"); facts.push({ scenario: "same-snapshot-explicit-new-and-current-change", records: f.entry().records, counts: f.counts });
});
test("R34 bundle health explicit ephemeral and non-mutating; no rebuild of missing/corrupt", async () => {
    for (const health of ["VERIFIED", "MISSING", "CORRUPT"] as const) { const f = exportFixture(); await f.load(); await f.submit(); const before = JSON.stringify([...f.m.values]), id = f.intent(); assert.equal(f.counts.verify, 0); f.setHealth(health); assert.equal(await f.c.verifyBundle(f.project, id), true); assert.equal(f.entry().health[id].health, health); assert.ok(f.entry().health[id].observedAt); assert.equal(JSON.stringify([...f.m.values]), before); assert.equal(f.counts.post, 1); const reopened = new HNLocalSequenceExportController(() => {}, f.deps); await reopened.inspect(f.project); assert.deepEqual(reopened.entry(f.project).health, {}); assert.equal(reopened.entry(f.project).records[0].state, "COMMITTED"); facts.push({ scenario: "health-" + health, entry: f.entry(), counts: f.counts }); }
    const f = exportFixture(); await f.load(); f.setMode("drop-before"); await f.submit(); assert.equal(await f.c.verifyBundle(f.project, f.intent()), false); assert.equal(f.counts.verify, 0);
});
test("R34 synchronous project lock before await; re-scan after confirm sees another tab intent", async () => {
    const f = exportFixture(); await f.load(); let accept!: (v: boolean) => void; const confirmation = new Promise<boolean>(resolve => { accept = resolve; }); const one = f.submit(() => confirmation); assert.equal(f.entry().running, true); assert.equal(await f.submit(), false); assert.equal(await f.c.refresh(f.project), false); assert.equal(f.c.entry("other").running, false); accept(true); await one; assert.equal(f.counts.post, 1);
    const g = exportFixture(); await g.load(); await g.submit(async () => { const r = { ...exportRecord(), state: "UNKNOWN" as const }; g.m.values.set(exportJournalKey(g.project, r.command.exportIntentId), r); return true; }); assert.equal(g.counts.post, 0); assert.equal(g.counts.uuid, 0); assert.equal(g.entry().records.length, 1);
});
test("R34 corrupt journal blocks inspect/load/new export; no backend or browser media path", async () => {
    const f = exportFixture(); f.m.values.set(exportJournalKey(f.project, crypto.randomUUID()), { corrupt: true }); assert.equal(await f.c.inspect(f.project), false); assert.equal(await f.load(), false); assert.equal(f.entry().storageBlocked, true); assert.equal(f.counts.http, 0); assert.equal(await f.submit(), false);
    const source = readFileSync(new URL("./hn-local-canvas-sequence-export.ts", import.meta.url), "utf8"); assert.doesNotMatch(source, /from ["'][^"']*local-export["']|getMediaBlob|readBlob|downloadRemoteMedia|createVideoGenerationTask|createObjectURL|\.arrayBuffer\(/); assert.equal(f.counts.forbidden, 0);
});
test("R34 actual localhost server observes durable-dispatched POST one-effect recovery and explicit repeat", async () => {
    const { createServer } = await import("node:http"), f = exportFixture(); let posts = 0, lose = true;
    const server = createServer(async (req, res) => { try { let body = ""; for await (const chunk of req) body += chunk; if (req.method === "POST" && req.url!.endsWith("/export-commands")) posts++; const response = await f.deps.request("http://127.0.0.1:3018" + req.url!, { method: req.method, body: body || undefined, headers: { "X-HN-Local-Request": "1" }, credentials: "omit", redirect: "error", cache: "no-store" }); res.writeHead(response.status, { "Content-Type": "application/json" }); if (lose && req.method === "POST" && req.url!.endsWith("/export-commands")) { lose = false; res.end("{"); } else res.end(await response.text()); } catch { res.writeHead(500); res.end("{}"); } });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve)); const address = server.address() as import("node:net").AddressInfo;
    const request = (async (input: Parameters<typeof fetch>[0], init?: RequestInit) => String(input) === "/api/hn/local-endpoint" ? Response.json({ code: 0, data: { url: "http://127.0.0.1:" + address.port }, msg: "ok" }) : fetch(input, init)) as typeof fetch;
    const c = new HNLocalSequenceExportController(() => {}, { ...f.deps, request });
    try { await c.load(f.project); assert.equal(await c.submit(f.project, async () => true), false); assert.equal(posts, 1); assert.equal(f.stored.size, 1); const id = c.entry(f.project).records[0].command.exportIntentId; assert.equal(await c.submit(f.project, async () => true), false); assert.equal(posts, 1); assert.equal(await c.continueSame(f.project, id, async () => true), true); assert.equal(posts, 2); assert.equal(f.stored.size, 1); assert.deepEqual(f.posts[0], f.posts[1]); await c.submit(f.project, async () => true); assert.equal(posts, 3); assert.equal(f.stored.size, 2); const before = posts; await c.verifyBundle(f.project, id); assert.equal(posts, before); facts.push({ scenario: "actual-localhost", serverObservedExportPost: posts, effects: f.stored.size, automaticPostRetry: 0, sameKeyContinuation: f.posts.slice(0, 2), explicitNewIntent: f.posts[2], events: f.m.events, records: c.entry(f.project).records, health: c.entry(f.project).health }); } finally { await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); }
});
after(() => r34Evidence("controller", facts));

test("R34 bundle health cannot attach to a different historical ExportID", async () => {
    const f = exportFixture(); await f.load(); await f.submit(); const r = structuredClone(f.entry().records[0]), before = JSON.stringify([...f.m.values]);
    const changed = await exportStatus(f.project, r.command); assert.notEqual(changed.exportId, r.status!.exportId); f.stored.set(r.command.exportIntentId, changed);
    assert.equal(await f.c.verifyBundle(f.project, r.command.exportIntentId), false); assert.deepEqual(f.entry().health, {}); assert.equal(JSON.stringify([...f.m.values]), before); assert.equal(f.counts.post, 1);
});
