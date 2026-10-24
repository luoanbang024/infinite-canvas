# Canonical local verification entry point

## What changed

R38 adds four files and edits four governance documents. No product or test byte changes.

| path | role |
|---|---|
| `tools/verify/verify_local.ps1` | canonical entry point on this Windows host |
| `tools/verify/verify_local.sh` | POSIX peer, same contract |
| `tools/verify/README.md` | usage, gates, exit codes, limits |
| `docs/hn/canonical_verification_entry_point.md` | this record |

Edited for governance only: `PATCH_LEDGER.md`, `UPSTREAM_BASELINE.json`, `docs/progress/todo.md`,
`docs/progress/pending-test.md`.

## The problem it closes

Three facts were established by the R38-00 scope discovery and are worth repeating because they are
the whole justification:

1. **CI could not fire for this project.** `ci.yml` triggers on `push: branches: [main, master]`.
   Work lands on `our-main` and `feature/*`. `origin/main` is a divergent line that misses 100+
   commits. No push to a working branch could trigger CI.
2. **Even if it fired, it would run no frontend test and no typecheck.** The jobs were `go-test`
   (`go test ./...`) and `web-build` (`bun install --frozen-lockfile`, `bun run build`).
   `web/package.json` had no `test` and no `typecheck` script.
3. **Three bridge contract tests were dormant.** `TestHNR22ProductionHTTPFrontendResponseLossRecovery`,
   `TestHNR26ProductionHTTPFrontendCommittedLossBarrier` and
   `TestR28ProductionHTTPBunClientRecovery` each read `HN_R22_BUN` / `HN_R26_BUN` and call `t.Skip`
   when unset. Nothing committed set them, so `go test ./...` skipped them, and the frontend opt-in
   cases they drive skipped too because the fixture comes from those tests.

The third point matters most: the coverage existed but was never invoked. A skipped test and a
passing test were indistinguishable in every report this project produced.

## Zero-edit activation probe

Before the branch was created and before any file was edited, the three tests were run twice at
exact `a97af3f7be1953874ce222f13c451e231aaf0662` with the pinned Go and the offline environment:

| run | gate variables | result |
|---|---|---|
| baseline | unset | 3 RUN / **3 SKIP** / 0 PASS |
| activated | `HN_R22_BUN` = `HN_R26_BUN` = pinned Bun | 3 RUN / **3 PASS** / 0 SKIP |

Each test asserts server-observed HTTP command counts and protected-record equality from inside the
Go test and only reaches those assertions by spawning the pinned Bun to execute the real frontend
file, so a PASS is positive evidence that the bridge actually ran.

## Gates

The entry point reproduces Gate G and adds the activation gate:

`G01` receiver-sensitive 2 PASS / 0 FAIL · `G02` FULL 239 / 2 / 0 · `G03` FOCUSED 221 / 2 / 0 ·
`G04` TypeScript · `G05A` the three contract tests, 3 PASS / 0 SKIP · `G05B` root Go ·
`G06` Bridge Go · `G07` `go mod verify` · `G08` production build.

`G05A` is the new one. Everything else is the previously-established Gate G, now reproducible by one
command instead of a hand-written runner per round.

Toolchain pins are enforced, not documented: Bun `0187f68d…1b9c` (1.3.14) before any gate, Go
`7609132b…866a` (1.25.14) before the Go gates, Go held offline with `GOPROXY=off`, `GOSUMDB=off`,
`GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`, `GOTELEMETRY=off` and the fixed audited module cache.

## Two findings worth carrying forward

### F-01 — `ProcessStartInfo.EnvironmentVariables` is unusable in script scope here

On this host, reading that property inside a `.ps1` yields `$null` even though the underlying object
exists, so the first implementation's `Clear()` failed. Children are now given a controlled
environment by rewriting the current process environment for the duration of a single child call and
restoring it in `finally`.

### F-02 — a stripped environment can look exactly like a failed gate

The first implementation built a strict allow-list child environment. The pinned Bun then exited 1
with **no output at all**, because one host-injected variable was missing. The runner dutifully
reported "gate G01 failed", which would have sent a reader after the product instead of the
environment. Fixed two ways: the child environment is now a deny-list, and a tool smoke test runs
before any gate so this class of problem reports as `TOOL_SMOKE_TEST`.

### F-03 — one existing frontend test is checkout-representation sensitive

While preparing the N2 throwaway clone, `R22 add-only historical method leaves every prior R20
source byte unchanged` failed in a fresh clone but passes in the long-lived worktree. Cause: that test
compares `git show <sha>:<path>` bytes (LF) against the working-tree file. The long-lived worktree
holds LF files, but a fresh clone on Windows with `core.autocrlf=true` checks them out as CRLF.
It is not a product defect and it is **not** fixed here — editing that test is outside the allowlist.
It is recorded because the next person to "just clone and run the suite" will hit it.

## Negative controls

`N1` wrong Bun file → `BUN_IDENTITY`, exit 10, zero gate directories created.
`N2` throwaway audit-local clone with one synthetic failing assertion → exit 2, `failed_gate = G02`,
source repository unchanged, and in the isolated re-run exactly one test failed (the injected one).
`N3` child `PATH` without Git → `GIT_PREREQUISITE`, exit 20, before the frontend gates.

The N2 clone was built with `git clone --no-hardlinks` into the audit root, its remote removed so
"network-free" holds by construction, and `web/node_modules` attached by junction so the 600 MB+
dependency tree was never copied. `git worktree` was not used, because that would write into the
source repository's Git metadata.

## What stays unclaimed

- Real-browser acceptance: `NOT_CLAIMED`, unchanged since R18. This tool runs Bun/Node suites.
- `NETWORK_ISOLATION`: `NOT_AVAILABLE`. Proxy variables are stripped and no network verb is used, but
  absence of an observed call is not isolation.
- POSIX end-to-end execution: **not** claimed. Only `bash -n` and gate-plan parity were verified.
- CI alignment: deliberately out of scope. `ci.yml` and `web/package.json` remain byte-identical to
  upstream v0.8.0 and are untouched.
