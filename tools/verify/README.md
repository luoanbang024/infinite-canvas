# tools/verify — canonical local verification

One command that reproduces the project's Gate G locally with the pinned toolchain, and that also
exercises the three Go-to-Bun bridge contract tests which were previously never driven by anything.

```
tools/verify/verify_local.ps1     canonical entry point on Windows (this host)
tools/verify/verify_local.sh      POSIX peer with the same contract
```

## Why this exists

Before R38 the verification knowledge lived in people's heads and in per-round scripts:

- CI could not help. `.github/workflows/ci.yml` triggers on `push: branches: [main, master]` while
  work lands on `our-main` and `feature/*`, so no push to a working branch ever triggered it. Even
  if it had, its only web step was `bun install --frozen-lockfile` + `bun run build`; there was no
  test script and no typecheck.
- Three Go contract tests — `TestHNR22ProductionHTTPFrontendResponseLossRecovery`,
  `TestHNR26ProductionHTTPFrontendCommittedLossBarrier` and `TestR28ProductionHTTPBunClientRecovery`
  — guard on `HN_R22_BUN` / `HN_R26_BUN` and call `t.Skip` when unset. Nothing committed ever set
  those variables, so a plain `go test ./...` skipped them, and the frontend opt-in cases they drive
  skipped too because their fixtures come from those very tests.
- The canonical Bun pin was machine-readable in exactly one place: the CI file that never fired.

This entry point turns that into an artefact. It does not change any product or test byte.

## Usage

```powershell
.\tools\verify\verify_local.ps1 `
    -BunPath    C:\path\to\bun.exe `
    -GoPath     C:\path\to\bin\go.exe `
    -GoModCache C:\path\to\go-mod-cache `
    -AuditDir   D:\audit\run-2026-10-25
```

```bash
./tools/verify/verify_local.sh --bun /path/to/bun --go /path/to/go \
    --go-mod-cache /path/to/gomodcache --audit-dir /path/to/audit
```

Optional: `-RepoRoot` (defaults to two levels above the script), `-Gates G01,G05A` to run a subset,
`-DryRun` to resolve and print the gate plan without executing anything.

Everything is written under `-AuditDir`: per-gate `stdout.txt` / `stderr.txt` / `meta.json`,
`preflight/`, `gocache/`, `shim/`, and a final `verify_state.json`. Nothing is written elsewhere,
and nothing is installed, downloaded or updated.

## Gates

| id | gate | expectation |
|---|---|---|
| — | Bun identity | sha256 must equal `0187f68d…1b9c` **before any gate** |
| — | Go identity | sha256 must equal `7609132b…866a` before the Go gates |
| — | git prerequisite | `git` resolvable, baseline object present, history reachable |
| — | tool smoke | `bun --version` and `go version` must answer under the child environment |
| G01 | receiver-sensitive frontend | 2 PASS / 0 FAIL |
| G02 | FULL frontend | 239 PASS / 2 SKIP / 0 FAIL |
| G03 | FOCUSED frontend | 221 PASS / 2 SKIP / 0 FAIL |
| G04 | TypeScript | `tsc --noEmit --incremental false`, exit 0 |
| G05A | **activated** contract tests | all 3 RUN/PASS, **0 SKIP**, 0 FAIL |
| G05B | root Go | `go test ./... -count=1 -timeout=30m`, exit 0 |
| G06 | Bridge Go | `go test ./... -count=1`, exit 0 |
| G07 | `go mod verify` | `all modules verified` |
| G08 | production build | exit 0, via the audit-local Bun-as-Node shim |

G05A and G05B run with `HN_R22_BUN` and `HN_R26_BUN` set to the pinned Bun, which is what makes the
previously dormant bridge tests execute.

Exit codes: `0` pass · `2` a gate failed (named in `verify_state.json`) · `10` Bun identity ·
`11` Go identity · `12` module cache · `13` argument or gate-selection error · `14`/`15` tool smoke ·
`20` git prerequisite · `21` repository state · `22` dirty worktree · `23` G08 shim identity.

## Environment model

Children inherit the host environment **minus** credential-shaped variables (`TOKEN`, `SECRET`,
`PASSWORD`, `CREDENTIAL`, `API_KEY`, `AUTH`, `SESSION`, `COOKIE`, …), **minus** proxy variables, and
**minus** the keys this script owns (`GOPROXY`, `GOSUMDB`, `GOTOOLCHAIN`, `GOFLAGS`, `GOTELEMETRY`,
`GOMODCACHE`, `GOCACHE`, `GOTMPDIR`, `HN_R22_BUN`, `HN_R26_BUN`, `NEXT_TELEMETRY_DISABLED`,
`TEMP`, `TMP`), which are then set explicitly.

A strict allow-list was tried first and **rejected**. This host injects a variable the pinned Bun
needs; stripping it made Bun exit 1 with no output at all, and the first implementation then reported
that as a failed gate. That is exactly the failure mode a verification tool must not have, so the
policy was changed and a tool smoke test was added: an unusable child environment now surfaces as
`TOOL_SMOKE_TEST`, never as a product gate failure.

## Negative controls

The entry point was validated by deliberately breaking it:

- **N1** a non-matching Bun file → rejected at `BUN_IDENTITY` before any gate runs, zero gate dirs.
- **N2** a throwaway clone inside the audit root with one synthetic failing assertion → exit 2,
  `failed_gate = G02`, and the source repository untouched.
- **N3** a child environment whose `PATH` cannot resolve Git → rejected at `GIT_PREREQUISITE` before
  the frontend gates.

A positive run is only meaningful once those three behave as described.

## Limits — what this does not claim

- Real-browser acceptance (IndexedDB persistence, reload, two tabs, theme/layout, rapid clicks) stays
  `NOT_CLAIMED`. This runs Node/Bun-level suites, not a browser.
- `NETWORK_ISOLATION` stays `NOT_AVAILABLE`. Proxy variables are stripped and no network verb is
  used, but that is not isolation.
- The POSIX entry point has **not** been executed end to end on a POSIX host. What was verified here
  is `bash -n` and plan parity against the PowerShell entry point (`ALL_GATES_PARITY` in the R38-01
  audit). Do not read a POSIX PASS into it.
- It reproduces Gate G; it does not replace an independent audit. A green run is evidence, not a
  verdict.
