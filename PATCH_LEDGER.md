# Patch Ledger

PROJECT_ID: HN_AI_IC
BASE_UPSTREAM: tigerowo/infinite-canvas v0.8.0 @ edd4452cb9b0d93dbb9c1ea5acdea1ee14015800

This ledger tracks OUR changes that alter upstream-owned product behavior or source. Governance-only bootstrap files are not product patches.

## Active patches

None.

## Rules

For every future upstream patch, record at minimum:

| Field | Required |
|---|---|
| PATCH_ID | Yes |
| Upstream area/file | Yes |
| OUR change | Yes |
| Reason | Yes |
| Introduced in commit | Yes |
| Upstream replacement/removal condition | Yes |
| Data/schema impact | Yes when applicable |
| Rollback note | Yes when risk is non-trivial |

Prefer extension points over direct Canvas/Node/Agent/Provider/Workflow/Asset/Director core modifications.
