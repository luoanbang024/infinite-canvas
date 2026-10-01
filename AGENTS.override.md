# HN AI Infinite Canvas — Execution Override

This file is the repository-local override for the HN_AI_IC fork. It is intentionally small. Keep all upstream `AGENTS.md` engineering conventions unless they conflict with this file or with the user's currently authorized HN execution.

## Authority

- Project: `HN_AI_IC` — HN AI Infinite Canvas — Local AI Video Workbench.
- Authoritative project policy: the current HN startup/continuity package `ACTIVE_BASELINE.md` (Project Policy: Infinite Canvas V1.0).
- Current authorized execution: the package's `CURRENT_EXECUTION.md`.
- Upstream candidate for this bootstrap: `tigerowo/infinite-canvas v0.8.0` at `edd4452cb9b0d93dbb9c1ea5acdea1ee14015800`.
- Baseline status remains `CANDIDATE` until independent review/adoption.

## Overrides of upstream AGENTS.md

1. The user's authorization of the current HN execution is permission for in-scope tracked edits. Do not ask for separate approval before each file edit, deletion, or formatting action that is already explicitly in scope.
2. Run the build, typecheck, tests, and runtime checks required by the current HN execution after modifications. The upstream instruction to skip build/test after edits does not apply to HN verification work.

## HN boundaries

- Prefer upstream-first, minimal-delta, extension-first changes.
- Do not modify unrelated files or broadly reformat/refactor.
- Do not change dependency versions or lockfiles unless a reviewed execution explicitly authorizes it.
- Do not expose API keys, tokens, cookies, private keys, or other secrets in Git, logs, screenshots, or completion packages.
- Do not perform paid generation, destructive data operations, irreversible migration, or release/publish actions without the required human gate.
- Do not port code from Secondary/Reference projects unless a later execution explicitly authorizes the source, exact scope, commit, and license.
- If the HN package is unavailable or Project/Execution/Source identity conflicts, stop only the affected action and report the exact conflict.
