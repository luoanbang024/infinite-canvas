#!/usr/bin/env bash
# Canonical local verification entry point for HN AI Infinite Canvas — POSIX shell peer.
#
# The PowerShell entry point (verify_local.ps1) is canonical on the Windows host this project is
# developed on. This file provides the same contract for POSIX shells: identical inputs, identical
# gates, identical identity pins, identical no-network / no-install boundary, and identical
# first-failure behaviour.
#
# It has NOT been executed end to end on a POSIX host. What has been verified on this machine is
# syntax (`bash -n`) and plan parity against the PowerShell entry point (`--dry-run`). Do not read
# more than that into it.
#
# Usage:
#   ./verify_local.sh --bun /path/to/bun --go /path/to/go \
#                     --go-mod-cache /path/to/gomodcache --audit-dir /path/to/audit \
#                     [--repo /path/to/repo] [--gates G01,G02] [--dry-run]
set -u

PIN_BUN_SHA="0187f68d843f825a72ada4a7eca60db896ed753759a7f8252edcd31ac1bf1b9c"
PIN_BUN_VERSION="1.3.14"
PIN_GO_SHA="7609132b0d289bf42ca9dba23b849824ecdf4af5323094e9cfb5ea1633e3866a"
PIN_GO_VERSION="1.25.14"
BASE_COMMIT="a97af3f7be1953874ce222f13c451e231aaf0662"
R38_BRANCH="feature/p0-b-r38-canonical-verification-entry-point"

CTRL="./src/app/(user)/canvas/components"
RECEIVER_PATTERN="R36 receiver-sensitive"
CONTRACT_TESTS=(
  "TestHNR22ProductionHTTPFrontendResponseLossRecovery"
  "TestHNR26ProductionHTTPFrontendCommittedLossBarrier"
  "TestR28ProductionHTTPBunClientRecovery"
)
CONTRACT_PATTERN="^(${CONTRACT_TESTS[0]}|${CONTRACT_TESTS[1]}|${CONTRACT_TESTS[2]})\$"

ALLOWLIST=(
  "tools/verify/verify_local.ps1"
  "tools/verify/verify_local.sh"
  "tools/verify/README.md"
  "docs/hn/canonical_verification_entry_point.md"
  "PATCH_LEDGER.md"
  "UPSTREAM_BASELINE.json"
  "docs/progress/todo.md"
  "docs/progress/pending-test.md"
)

BUN_PATH=""
GO_PATH=""
GOMODCACHE=""
AUDIT_DIR=""
REPO_ROOT=""
GATES=""
DRY_RUN=0

die() { printf 'RESULT = FAIL\nFAILED_GATE = %s\nREASON = %s\n' "$1" "$2" >&2; exit "${3:-2}"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --bun) BUN_PATH="$2"; shift 2 ;;
    --go) GO_PATH="$2"; shift 2 ;;
    --go-mod-cache) GOMODCACHE="$2"; shift 2 ;;
    --audit-dir) AUDIT_DIR="$2"; shift 2 ;;
    --repo) REPO_ROOT="$2"; shift 2 ;;
    --gates) GATES="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) die "ARGUMENTS" "unknown argument: $1" 13 ;;
  esac
done

[ -n "$BUN_PATH" ] || die "ARGUMENTS" "--bun is required" 13
[ -n "$GO_PATH" ] || die "ARGUMENTS" "--go is required" 13
[ -n "$GOMODCACHE" ] || die "ARGUMENTS" "--go-mod-cache is required" 13
[ -n "$AUDIT_DIR" ] || die "ARGUMENTS" "--audit-dir is required" 13
if [ -z "$REPO_ROOT" ]; then REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"; fi
WEB="$REPO_ROOT/web"
BRIDGE="$REPO_ROOT/canvas-agent/native/comfy-bridge"

mkdir -p "$AUDIT_DIR/gate" "$AUDIT_DIR/preflight" || die "AUDIT_DIR" "cannot create $AUDIT_DIR" 12

sha_of() { if [ -f "$1" ]; then sha256sum "$1" | awk '{print $1}'; else echo ""; fi; }

# --- identity pins -----------------------------------------------------------------------------
BUN_SHA="$(sha_of "$BUN_PATH")"
[ "$BUN_SHA" = "$PIN_BUN_SHA" ] || die "BUN_IDENTITY" "pinned Bun sha256 mismatch or unreadable: $BUN_PATH" 10
GO_SHA="$(sha_of "$GO_PATH")"
[ "$GO_SHA" = "$PIN_GO_SHA" ] || die "GO_IDENTITY" "pinned Go sha256 mismatch or unreadable: $GO_PATH" 11
[ -d "$GOMODCACHE" ] || die "GO_MOD_CACHE" "Go module cache not found: $GOMODCACHE" 12

# --- git prerequisite --------------------------------------------------------------------------
command -v git >/dev/null 2>&1 || die "GIT_PREREQUISITE" \
  "git is not resolvable on PATH; the frontend suite asserts historical byte preservation via git show" 20

cd "$REPO_ROOT" || die "REPOSITORY_STATE" "cannot enter $REPO_ROOT" 21
HEAD_SHA="$(git --no-optional-locks rev-parse HEAD)"
git --no-optional-locks cat-file -e "${BASE_COMMIT}^{commit}" 2>/dev/null \
  || die "REPOSITORY_STATE" "baseline commit object missing; frontend tests require git object access" 21
git --no-optional-locks rev-list --count HEAD >/dev/null 2>&1 \
  || die "REPOSITORY_STATE" "reachable history unavailable" 21
BRANCH="$(git --no-optional-locks rev-parse --abbrev-ref HEAD)"

OUTSIDE=""
while IFS= read -r line; do
  [ -n "$line" ] || continue
  p="${line#???}"
  p="${p%\"}"; p="${p#\"}"
  found=0
  for a in "${ALLOWLIST[@]}"; do [ "$p" = "$a" ] && found=1; done
  [ "$found" -eq 0 ] && OUTSIDE="$OUTSIDE $p"
done <<EOF
$(git --no-optional-locks status --porcelain=v1 -uall)
EOF
if [ -n "$OUTSIDE" ] || { [ -n "$(git --no-optional-locks status --porcelain=v1 -uall)" ] && [ "$BRANCH" != "$R38_BRANCH" ]; }; then
  die "DIRTY_WORKTREE" "worktree is dirty and not limited to this round's allowlisted files (branch=$BRANCH)$OUTSIDE" 22
fi

# --- gate plan ---------------------------------------------------------------------------------
ALL_GATES=(G01 G02 G03 G04 G05A G05B G06 G07 G08)
if [ -z "$GATES" ]; then
  SELECTED=("${ALL_GATES[@]}")
else
  IFS=',' read -r -a SELECTED <<< "$GATES"
  for s in "${SELECTED[@]}"; do
    ok=0
    for a in "${ALL_GATES[@]}"; do [ "$s" = "$a" ] && ok=1; done
    [ "$ok" -eq 1 ] || die "GATE_SELECTION" "unknown gate id: $s" 13
  done
fi

gate_argv() {
  case "$1" in
    G01) printf '%s\036%s\036%s\036%s\036%s\036%s\036%s' \
           "$BUN_PATH" "--no-env-file" "test" "-t" "$RECEIVER_PATTERN" \
           "$CTRL/hn-local-canvas-sequence-reorder.test.ts" \
           "$CTRL/hn-local-canvas-sequence-export.test.ts" ;;
    G02) printf '%s\036%s\036%s\036%s' "$BUN_PATH" "--no-env-file" "test" "src" ;;
    G03) printf '%s\036%s\036%s\036%s\036%s' "$BUN_PATH" "--no-env-file" "test" "$CTRL" "./src/services/hn" ;;
    G04) printf '%s\036%s\036%s\036%s\036%s\036%s\036%s\036%s' \
           "$BUN_PATH" "--no-env-file" "x" "--no-install" "tsc" "--noEmit" "--incremental" "false" ;;
    G05A) printf '%s\036%s\036%s\036%s\036%s\036%s\036%s\036%s' \
           "$GO_PATH" "test" "./handler" "-run" "$CONTRACT_PATTERN" "-v" "-count=1" "-timeout=30m" ;;
    G05B) printf '%s\036%s\036%s\036%s\036%s' "$GO_PATH" "test" "./..." "-count=1" "-timeout=30m" ;;
    G06) printf '%s\036%s\036%s\036%s' "$GO_PATH" "test" "./..." "-count=1" ;;
    G07) printf '%s\036%s\036%s' "$GO_PATH" "mod" "verify" ;;
    G08) printf '%s\036%s\036%s\036%s' "$BUN_PATH" "--no-env-file" "run" "build" ;;
  esac
}

gate_cwd() {
  case "$1" in
    G01|G02|G03|G04|G08) printf '%s' "$WEB" ;;
    G05A|G05B|G07) printf '%s' "$REPO_ROOT" ;;
    G06) printf '%s' "$BRIDGE" ;;
  esac
}

if [ "$DRY_RUN" -eq 1 ]; then
  {
    printf '{"entry_point":"tools/verify/verify_local.sh","execution_host":"posix","result":"DRY_RUN",'
    printf '"repo_root":"%s","audit_dir":"%s","toolchain":{"bun":{"sha256":"%s","matches":%s},' \
      "$REPO_ROOT" "$AUDIT_DIR" "$BUN_SHA" "true"
    printf '"go":{"sha256":"%s","matches":%s}},"gates":[' "$GO_SHA" "true"
    first=1
    for g in "${SELECTED[@]}"; do
      [ "$first" -eq 0 ] && printf ','
      first=0
      raw="$(gate_argv "$g")"
      argv_json="$(printf '%s' "$raw" | awk -F'\036' '{out=""; for(i=1;i<=NF;i++){ if($i=="")continue; gsub(/\\/,"\\\\",$i); gsub(/"/,"\\\"",$i); out=out (out==""?"":",") "\"" $i "\"" } print "[" out "]"}')"
      cwd="$(gate_cwd "$g")"
      cwd_json="$(printf '%s' "$cwd" | sed 's/\\/\\\\/g; s/"/\\"/g')"
      printf '{"id":"%s","argv":%s,"cwd":"%s"}' "$g" "$argv_json" "$cwd_json"
    done
    printf ']}\n'
  } > "$AUDIT_DIR/verify_plan.json"
  echo "RESULT = DRY_RUN"
  echo "PLAN_GATES = $(IFS=','; echo "${SELECTED[*]}")"
  exit 0
fi

# --- tool smoke tests --------------------------------------------------------------------------
BUN_VER="$("$BUN_PATH" --version 2>/dev/null | tr -d '\r')"
[ "$BUN_VER" = "$PIN_BUN_VERSION" ] || die "TOOL_SMOKE_TEST" \
  "pinned Bun did not report $PIN_BUN_VERSION under the child environment (got '$BUN_VER')" 14
GO_VER="$("$GO_PATH" version 2>/dev/null | tr -d '\r')"
case "$GO_VER" in
  *"$PIN_GO_VERSION"*) : ;;
  *) die "TOOL_SMOKE_TEST" "pinned Go did not report $PIN_GO_VERSION (got '$GO_VER')" 15 ;;
esac

# --- gates -------------------------------------------------------------------------------------
go_env() (
  unset HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy
  export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOTELEMETRY=off
  export GOMODCACHE="$GOMODCACHE" GOCACHE="$AUDIT_DIR/gocache" GOTMPDIR="$1"
  export HN_R22_BUN="$BUN_PATH" HN_R26_BUN="$BUN_PATH"
  "$@"
)
plain_env() (
  unset HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy
  unset HN_R22_BUN HN_R26_BUN
  "$@"
)

count_key() { grep -cE "^[[:space:]]*[0-9]+ $1\$" "$2" 2>/dev/null | tail -1; }

for g in "${SELECTED[@]}"; do
  d="$AUDIT_DIR/gate/$g"; mkdir -p "$d/temp"
  case "$g" in
    G01) cwd="$WEB"; cmd=( "$BUN_PATH" --no-env-file test -t "$RECEIVER_PATTERN" \
            "$CTRL/hn-local-canvas-sequence-reorder.test.ts" \
            "$CTRL/hn-local-canvas-sequence-export.test.ts" ); runner=plain ;;
    G02) cwd="$WEB"; cmd=( "$BUN_PATH" --no-env-file test src ); runner=plain ;;
    G03) cwd="$WEB"; cmd=( "$BUN_PATH" --no-env-file test "$CTRL" ./src/services/hn ); runner=plain ;;
    G04) cwd="$WEB"; cmd=( "$BUN_PATH" --no-env-file x --no-install tsc --noEmit --incremental false ); runner=plain ;;
    G05A) cwd="$REPO_ROOT"; cmd=( "$GO_PATH" test ./handler -run "$CONTRACT_PATTERN" -v -count=1 -timeout=30m ); runner=go ;;
    G05B) cwd="$REPO_ROOT"; cmd=( "$GO_PATH" test ./... -count=1 -timeout=30m ); runner=go ;;
    G06) cwd="$BRIDGE"; cmd=( "$GO_PATH" test ./... -count=1 ); runner=go ;;
    G07) cwd="$REPO_ROOT"; cmd=( "$GO_PATH" mod verify ); runner=go ;;
    G08) cwd="$WEB"; cmd=( "$BUN_PATH" --no-env-file run build ); runner=plain ;;
  esac

  ( cd "$cwd" && if [ "$runner" = "go" ]; then go_env "${cmd[@]}"; else plain_env "${cmd[@]}"; fi ) \
      > "$d/stdout.txt" 2> "$d/stderr.txt"
  rc=$?
  pass_n="$(count_key pass "$d/stdout.txt" | tail -1)"; [ -n "$pass_n" ] || pass_n=""
  skip_n="$(count_key skip "$d/stdout.txt" | tail -1)"; [ -n "$skip_n" ] || skip_n=""
  fail_n="$(count_key fail "$d/stdout.txt" | tail -1)"; [ -n "$fail_n" ] || fail_n=""

  ok=0
  case "$g" in
    G01) [ "$rc" -eq 0 ] && [ "$pass_n" = "2" ] && [ "${fail_n:-0}" = "0" ] && ok=1 ;;
    G02) [ "$rc" -eq 0 ] && [ "$pass_n" = "239" ] && [ "$skip_n" = "2" ] && [ "${fail_n:-0}" = "0" ] && ok=1 ;;
    G03) [ "$rc" -eq 0 ] && [ "$pass_n" = "221" ] && [ "$skip_n" = "2" ] && [ "${fail_n:-0}" = "0" ] && ok=1 ;;
    G04) [ "$rc" -eq 0 ] && ok=1 ;;
    G05A)
      act=0; skp=0
      for t in "${CONTRACT_TESTS[@]}"; do
        grep -qE "^--- PASS: $t" "$d/stdout.txt" && act=$((act + 1))
        grep -qE "^--- SKIP: $t" "$d/stdout.txt" && skp=$((skp + 1))
      done
      [ "$rc" -eq 0 ] && [ "$act" -eq 3 ] && [ "$skp" -eq 0 ] && ok=1
      pass_n="$act" ;;
    G05B) [ "$rc" -eq 0 ] && ok=1 ;;
    G06) [ "$rc" -eq 0 ] && ok=1 ;;
    G07) [ "$rc" -eq 0 ] && grep -qi "all modules verified" "$d/stdout.txt" && ok=1 ;;
    G08) [ "$rc" -eq 0 ] && ok=1 ;;
  esac

  if [ "$ok" -eq 1 ]; then
    echo "[$g] PASS exit=$rc pass=${pass_n:-} skip=${skip_n:-} fail=${fail_n:-}"
  else
    echo "[$g] FAIL exit=$rc pass=${pass_n:-} skip=${skip_n:-} fail=${fail_n:-}"
    die "$g" "gate $g did not meet its expected result" 2
  fi
done

echo "RESULT = PASS"
echo "AUDIT_DIR = $AUDIT_DIR"
exit 0
