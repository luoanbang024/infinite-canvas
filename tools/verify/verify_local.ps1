<#
.SYNOPSIS
  Canonical local verification entry point for HN AI Infinite Canvas.

.DESCRIPTION
  Reproduces the project's Gate G locally with the pinned toolchain and additionally activates the
  three dormant Go-to-Bun bridge contract tests by setting HN_R22_BUN / HN_R26_BUN for the Go gates.

  Boundaries enforced here:
    * no dependency install / update / download;
    * no network operation of its own (Go pinned offline; proxy variables stripped from every child);
    * no browser, Provider, credential or user-data access (credential-shaped variables are stripped
      rather than forwarded);
    * every log, temp file, shim copy and machine-readable result is written under -AuditDir only.

  Environment model: children inherit the host environment MINUS credential-shaped variables, MINUS
  proxy variables, and MINUS the keys this script controls (which are then set explicitly). A strict
  allow-list was tried first and rejected: this host injects a variable the pinned Bun needs, and
  stripping it made Bun exit silently with no output, which then looked like a failed gate. A tool
  smoke test now runs before any gate, so that class of problem is reported as TOOL_SMOKE_TEST and
  is never misattributed to the product.

.PARAMETER BunPath
  Absolute path to the pinned Bun 1.3.14 executable.

.PARAMETER GoPath
  Absolute path to the pinned Go 1.25.14 executable.

.PARAMETER GoModCache
  Absolute path to the audited Go module cache. Required; never populated by this script.

.PARAMETER AuditDir
  Absolute path to a caller-provided audit directory. All output goes here.

.PARAMETER RepoRoot
  Repository root. Defaults to two levels above this script.

.PARAMETER Gates
  Optional comma-separated subset, e.g. "G02" or "G05A,G05B". Default: every gate.

.PARAMETER DryRun
  Resolve and print the gate plan as JSON without executing anything.

.EXAMPLE
  .\verify_local.ps1 -BunPath C:\tc\bun.exe -GoPath C:\tc\go.exe `
      -GoModCache C:\tc\gomodcache -AuditDir D:\audit\run1
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$BunPath,
    [Parameter(Mandatory = $true)][string]$GoPath,
    [Parameter(Mandatory = $true)][string]$GoModCache,
    [Parameter(Mandatory = $true)][string]$AuditDir,
    [string]$RepoRoot = "",
    [string]$Gates = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

# --------------------------------------------------------------------------- pinned identity
$PinBunSha = "0187f68d843f825a72ada4a7eca60db896ed753759a7f8252edcd31ac1bf1b9c"
$PinBunVersion = "1.3.14"
$PinGoSha = "7609132b0d289bf42ca9dba23b849824ecdf4af5323094e9cfb5ea1633e3866a"
$PinGoVersion = "1.25.14"
$BaseCommit = "a97af3f7be1953874ce222f13c451e231aaf0662"
$R38Branch = "feature/p0-b-r38-canonical-verification-entry-point"

# --------------------------------------------------------------------------- gate constants
$Ctrl = "./src/app/(user)/canvas/components"
$ReceiverSensitivePattern = "R36 receiver-sensitive"
$ContractTests = @(
    "TestHNR22ProductionHTTPFrontendResponseLossRecovery",
    "TestHNR26ProductionHTTPFrontendCommittedLossBarrier",
    "TestR28ProductionHTTPBunClientRecovery"
)
$ContractPattern = "^(" + ($ContractTests -join "|") + ")$"

$Allowlist = @(
    "tools/verify/verify_local.ps1",
    "tools/verify/verify_local.sh",
    "tools/verify/README.md",
    "docs/hn/canonical_verification_entry_point.md",
    "PATCH_LEDGER.md",
    "UPSTREAM_BASELINE.json",
    "docs/progress/todo.md",
    "docs/progress/pending-test.md"
)

# Variables that must never be forwarded to a child process.
$StripNamePatterns = @("TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "APIKEY",
                       "API_KEY", "PRIVATE_KEY", "PROXY", "AUTH", "SESSION", "COOKIE")
# Proxy variables matter even when a host uses a non-obvious casing.
$StripExact = @("http_proxy", "https_proxy", "all_proxy", "no_proxy",
                "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY")
# Variables this script owns: removed from the inherited copy, then set deliberately.
$ControlledKeys = @("GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOFLAGS", "GOTELEMETRY",
                    "GOMODCACHE", "GOCACHE", "GOTMPDIR",
                    "HN_R22_BUN", "HN_R26_BUN", "NEXT_TELEMETRY_DISABLED", "TEMP", "TMP")

# --------------------------------------------------------------------------- helpers
function Write-Utf8([string]$Path, [string]$Text) {
    $dir = Split-Path -Parent $Path
    if ($dir -and -not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }
    [IO.File]::WriteAllText($Path, $Text, (New-Object Text.UTF8Encoding($false)))
}

function Write-JsonFile([string]$Path, $Object) {
    Write-Utf8 $Path ($Object | ConvertTo-Json -Depth 12)
}

function Get-Sha256([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Quote-Arg([string]$a) {
    if ($a -eq "") { return '""' }
    if ($a -match '^[A-Za-z0-9_\-\./:\\=\(\)\{\},+@%]+$') { return $a }
    return '"' + ($a -replace '"', '\"') + '"'
}

function Get-TestCount([string]$Text, [string]$Key) {
    $vals = @()
    foreach ($line in ($Text -split "`r?`n")) {
        $t = $line.Trim()
        if ($t.EndsWith($Key)) {
            $first = ($t -split '\s+')[0]
            $n = 0
            if ([int]::TryParse($first, [ref]$n)) { $vals += $n }
        }
    }
    if ($vals.Count -eq 0) { return $null }
    return $vals[$vals.Count - 1]
}

function New-ChildEnv([string]$TempDir) {
    $e = @{}
    foreach ($entry in [System.Environment]::GetEnvironmentVariables().GetEnumerator()) {
        $k = [string]$entry.Key
        $upper = $k.ToUpperInvariant()
        $drop = $false
        foreach ($p in $StripNamePatterns) { if ($upper.Contains($p)) { $drop = $true } }
        foreach ($p in $StripExact) { if ($k -eq $p) { $drop = $true } }
        foreach ($p in $ControlledKeys) { if ($upper -eq $p.ToUpperInvariant()) { $drop = $true } }
        if (-not $drop) { $e[$k] = [string]$entry.Value }
    }
    if (-not (Test-Path -LiteralPath $TempDir)) {
        New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
    }
    $e["TEMP"] = $TempDir
    $e["TMP"] = $TempDir
    return $e
}

function New-GoEnv([string]$TempDir) {
    $e = New-ChildEnv $TempDir
    $e["GOPROXY"] = "off"
    $e["GOSUMDB"] = "off"
    $e["GOTOOLCHAIN"] = "local"
    $e["GOFLAGS"] = "-mod=readonly"
    $e["GOTELEMETRY"] = "off"
    $e["GOMODCACHE"] = $GoModCache
    $e["GOCACHE"] = (Join-Path $AuditDir "gocache")
    $e["GOTMPDIR"] = $TempDir
    return $e
}

function Invoke-Native {
    param(
        [string]$Id,
        [string[]]$Argv,
        [string]$Cwd,
        [hashtable]$ChildEnv,
        [int]$TimeoutSec,
        [string]$Root = "gate"
    )
    $dir = Join-Path (Join-Path $AuditDir $Root) $Id
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $dir "temp") | Out-Null

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Argv[0]
    if ($Argv.Count -gt 1) {
        $psi.Arguments = (($Argv[1..($Argv.Count - 1)]) | ForEach-Object { Quote-Arg $_ }) -join " "
    } else {
        $psi.Arguments = ""
    }
    $psi.WorkingDirectory = $Cwd
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8

    # Windows PowerShell 5.1 exposes ProcessStartInfo.EnvironmentVariables as null in script scope,
    # so the child environment is produced by rewriting this process's own environment for the
    # duration of the call and letting the child inherit it. The previous environment is snapshotted
    # and restored in finally, so no change outlives a single call.
    $snapshot = @{}
    foreach ($de in [System.Environment]::GetEnvironmentVariables().GetEnumerator()) {
        $snapshot[[string]$de.Key] = [string]$de.Value
    }

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $stdout = ""
    $stderr = ""
    $rc = $null
    $timedOut = $false
    try {
        foreach ($k in @($snapshot.Keys)) { [System.Environment]::SetEnvironmentVariable([string]$k, $null) }
        foreach ($k in $ChildEnv.Keys) {
            [System.Environment]::SetEnvironmentVariable([string]$k, [string]$ChildEnv[$k])
        }

        $p = New-Object System.Diagnostics.Process
        $p.StartInfo = $psi
        [void]$p.Start()
        $outTask = $p.StandardOutput.ReadToEndAsync()
        $errTask = $p.StandardError.ReadToEndAsync()
        $exited = $p.WaitForExit($TimeoutSec * 1000)
        if (-not $exited) {
            $timedOut = $true
            try { $p.Kill() } catch { }
            [void]$p.WaitForExit(15000)
        }
        try { $stdout = $outTask.Result } catch { }
        try { $stderr = $errTask.Result } catch { }
        if (-not $timedOut) { $rc = $p.ExitCode }
        $p.Dispose()
    } finally {
        $sw.Stop()
        foreach ($k in @([System.Environment]::GetEnvironmentVariables().Keys)) {
            [System.Environment]::SetEnvironmentVariable([string]$k, $null)
        }
        foreach ($k in $snapshot.Keys) {
            [System.Environment]::SetEnvironmentVariable([string]$k, [string]$snapshot[$k])
        }
    }

    Write-Utf8 (Join-Path $dir "stdout.txt") $stdout
    Write-Utf8 (Join-Path $dir "stderr.txt") $stderr
    $psi = $null

    return @{
        id = $Id; argv = $Argv; cwd = $Cwd; exit_code = $rc; timed_out = $timedOut
        duration_ms = [int]$sw.ElapsedMilliseconds
        stdout_bytes = $stdout.Length; stderr_bytes = $stderr.Length
        text = ($stdout + "`n" + $stderr)
        env_relevant = @{
            GOPROXY = $ChildEnv["GOPROXY"]; GOSUMDB = $ChildEnv["GOSUMDB"]
            GOTOOLCHAIN = $ChildEnv["GOTOOLCHAIN"]; GOFLAGS = $ChildEnv["GOFLAGS"]
            GOTELEMETRY = $ChildEnv["GOTELEMETRY"]
            GOMODCACHE = $ChildEnv["GOMODCACHE"]; GOCACHE = $ChildEnv["GOCACHE"]
            HN_R22_BUN = $ChildEnv["HN_R22_BUN"]; HN_R26_BUN = $ChildEnv["HN_R26_BUN"]
            NEXT_TELEMETRY_DISABLED = $ChildEnv["NEXT_TELEMETRY_DISABLED"]
            PATH = $ChildEnv["PATH"]
        }
    }
}

# --------------------------------------------------------------------------- bootstrap
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}
$Web = Join-Path $RepoRoot "web"
$Bridge = Join-Path $RepoRoot "canvas-agent\native\comfy-bridge"

New-Item -ItemType Directory -Force -Path $AuditDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $AuditDir "gate") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $AuditDir "preflight") | Out-Null

$state = @{
    entry_point = "tools/verify/verify_local.ps1"
    execution_host = "windows"
    started_utc = (Get-Date).ToUniversalTime().ToString("o")
    repo_root = $RepoRoot
    audit_dir = $AuditDir
    dry_run = [bool]$DryRun
    gate_selection = $Gates
    toolchain = @{}
    preconditions = @{}
    gates = @()
    result = "NOT_RUN"
    failed_gate = $null
    failed_reason = $null
}

function Complete-Run([string]$Result, [string]$FailedGate, [string]$Reason, [int]$ExitCode) {
    $state.result = $Result
    $state.failed_gate = $FailedGate
    $state.failed_reason = $Reason
    $state.finished_utc = (Get-Date).ToUniversalTime().ToString("o")
    Write-JsonFile (Join-Path $AuditDir "verify_state.json") $state
    Write-Host ("RESULT = " + $Result)
    if ($FailedGate) { Write-Host ("FAILED_GATE = " + $FailedGate) }
    if ($Reason) { Write-Host ("REASON = " + $Reason) }
    Write-Host ("AUDIT_DIR = " + $AuditDir)
    exit $ExitCode
}

# --------------------------------------------------------------------------- Bun identity
$bunSha = Get-Sha256 $BunPath
$state.toolchain["bun"] = @{ path = $BunPath; sha256 = $bunSha; expected_sha256 = $PinBunSha
                             expected_version = $PinBunVersion; matches = ($bunSha -eq $PinBunSha) }
if ($bunSha -ne $PinBunSha) {
    $state.preconditions["bun_identity"] = "FAIL"
    Complete-Run "FAIL" "BUN_IDENTITY" ("pinned Bun sha256 mismatch or unreadable: " + $BunPath) 10
}
$state.preconditions["bun_identity"] = "PASS"

# --------------------------------------------------------------------------- Go identity
$goSha = Get-Sha256 $GoPath
$state.toolchain["go"] = @{ path = $GoPath; sha256 = $goSha; expected_sha256 = $PinGoSha
                            expected_version = $PinGoVersion; matches = ($goSha -eq $PinGoSha) }
if ($goSha -ne $PinGoSha) {
    $state.preconditions["go_identity"] = "FAIL"
    Complete-Run "FAIL" "GO_IDENTITY" ("pinned Go sha256 mismatch or unreadable: " + $GoPath) 11
}
$state.preconditions["go_identity"] = "PASS"

if (-not (Test-Path -LiteralPath $GoModCache -PathType Container)) {
    $state.preconditions["go_mod_cache"] = "FAIL"
    Complete-Run "FAIL" "GO_MOD_CACHE" ("Go module cache not found: " + $GoModCache) 12
}
$state.preconditions["go_mod_cache"] = "PASS"

# --------------------------------------------------------------------------- git prerequisite
$gitCmd = Get-Command git -ErrorAction SilentlyContinue
if (-not $gitCmd) {
    $state.preconditions["git_resolvable"] = "FAIL"
    Complete-Run "FAIL" "GIT_PREREQUISITE" ("git is not resolvable on PATH; the frontend suite " +
        "asserts historical byte preservation through git show, so this must fail closed") 20
}
$state.preconditions["git_resolvable"] = "PASS"
$state.toolchain["git"] = @{ path = $gitCmd.Source; version = ((& git --version) -join " ") }

$refuse = $null
$refuseCode = 0
$refuseReason = ""
# Git emits CRLF notices on stderr. With ErrorActionPreference=Stop, the host shell turns native
# stderr into a terminating error, which killed the first positive run at `git diff --check`.
# Explicit exit-code checks are used throughout this block, so warnings are simply tolerated.
$savedEap = $ErrorActionPreference
Push-Location $RepoRoot
try {
    $ErrorActionPreference = "Continue"
    $head = (& git --no-optional-locks rev-parse HEAD).Trim()
    $abbrev = (& git --no-optional-locks rev-parse --abbrev-ref HEAD).Trim()
    $state.git = @{ head = $head; branch = $abbrev }

    & git --no-optional-locks cat-file -e ($BaseCommit + "^{commit}") 2>$null
    $baseObjOk = ($LASTEXITCODE -eq 0)
    $historyCount = 0
    $hc = (& git --no-optional-locks rev-list --count HEAD 2>$null)
    if ($hc) { $historyCount = [int]$hc }

    if ($baseObjOk -and $historyCount -gt 0) {
        $state.preconditions["repository_objects"] = "PASS"
    } else {
        $state.preconditions["repository_objects"] = "FAIL"
        $refuse = "REPOSITORY_STATE"
        $refuseCode = 21
        $refuseReason = "baseline commit object or reachable history is missing; frontend tests require git object access"
    }

    $diffOut = (& git --no-optional-locks diff --check 2>$null | Out-String)
    $diffRc = $LASTEXITCODE
    if ($diffRc -eq 0 -and [string]::IsNullOrWhiteSpace($diffOut)) {
        $state.preconditions["git_diff_check"] = "PASS"
    } else {
        $state.preconditions["git_diff_check"] = "FAIL"
    }

    $statusRaw = @(& git --no-optional-locks status --porcelain=v1 -uall)
    $changed = @()
    foreach ($line in $statusRaw) {
        if ($null -eq $line) { continue }
        $l = [string]$line
        if ($l.Length -le 3) { continue }
        $p = $l.Substring(3).Trim()
        if ($p.StartsWith('"') -and $p.EndsWith('"')) { $p = $p.Substring(1, $p.Length - 2) }
        $changed += $p
    }
    $state.git["changed_paths"] = $changed

    if (-not $refuse) {
        if ($changed.Count -gt 0) {
            $outside = @($changed | Where-Object { $Allowlist -notcontains $_ })
            $allowedDirty = ($abbrev -eq $R38Branch) -and ($outside.Count -eq 0)
            if ($allowedDirty) {
                $state.preconditions["worktree"] = "PASS_DIRTY_ALLOWLISTED"
            } else {
                $state.preconditions["worktree"] = "FAIL"
                $refuse = "DIRTY_WORKTREE"
                $refuseCode = 22
                $refuseReason = ("worktree is dirty and not limited to this round's allowlisted files " +
                                 "(branch=" + $abbrev + "); unexpected: " + ($outside -join ", "))
            }
        } else {
            $state.preconditions["worktree"] = "PASS_CLEAN"
        }
    }
} finally {
    $ErrorActionPreference = $savedEap
    Pop-Location
}

if ($refuse) {
    Complete-Run "FAIL" $refuse $refuseReason $refuseCode
}

# --------------------------------------------------------------------------- gate plan
$allGates = @("G01", "G02", "G03", "G04", "G05A", "G05B", "G06", "G07", "G08")
$selected = $allGates
if (-not [string]::IsNullOrWhiteSpace($Gates)) {
    $selected = @($Gates.Split(",") | ForEach-Object { $_.Trim().ToUpperInvariant() } | Where-Object { $_ })
    $unknown = @($selected | Where-Object { $allGates -notcontains $_ })
    if ($unknown.Count -gt 0) {
        Complete-Run "FAIL" "GATE_SELECTION" ("unknown gate id(s): " + ($unknown -join ", ")) 13
    }
}

$plan = @()
foreach ($g in $selected) {
    if ($g -eq "G01") {
        $plan += @{ id = "G01"; kind = "FRONTEND_RECEIVER_SENSITIVE"
                    argv = @($BunPath, "--no-env-file", "test", "-t", $ReceiverSensitivePattern,
                             ($Ctrl + "/hn-local-canvas-sequence-reorder.test.ts"),
                             ($Ctrl + "/hn-local-canvas-sequence-export.test.ts"))
                    cwd = $Web; expect = "2 PASS / 0 FAIL" }
    } elseif ($g -eq "G02") {
        $plan += @{ id = "G02"; kind = "FRONTEND_FULL"
                    argv = @($BunPath, "--no-env-file", "test", "src")
                    cwd = $Web; expect = "239 PASS / 2 SKIP / 0 FAIL" }
    } elseif ($g -eq "G03") {
        $plan += @{ id = "G03"; kind = "FRONTEND_FOCUSED"
                    argv = @($BunPath, "--no-env-file", "test", $Ctrl, "./src/services/hn")
                    cwd = $Web; expect = "221 PASS / 2 SKIP / 0 FAIL" }
    } elseif ($g -eq "G04") {
        $plan += @{ id = "G04"; kind = "TYPESCRIPT"
                    argv = @($BunPath, "--no-env-file", "x", "--no-install", "tsc",
                             "--noEmit", "--incremental", "false")
                    cwd = $Web; expect = "exit 0" }
    } elseif ($g -eq "G05A") {
        $plan += @{ id = "G05A"; kind = "ACTIVATED_CONTRACT_TESTS"
                    argv = @($GoPath, "test", "./handler", "-run", $ContractPattern,
                             "-v", "-count=1", "-timeout=30m")
                    cwd = $RepoRoot; expect = "3 RUN / 3 PASS / 0 SKIP / 0 FAIL"; go = $true }
    } elseif ($g -eq "G05B") {
        $plan += @{ id = "G05B"; kind = "ROOT_GO"
                    argv = @($GoPath, "test", "./...", "-count=1", "-timeout=30m")
                    cwd = $RepoRoot; expect = "exit 0"; go = $true }
    } elseif ($g -eq "G06") {
        $plan += @{ id = "G06"; kind = "BRIDGE_GO"
                    argv = @($GoPath, "test", "./...", "-count=1")
                    cwd = $Bridge; expect = "exit 0"; go = $true }
    } elseif ($g -eq "G07") {
        $plan += @{ id = "G07"; kind = "GO_MOD_VERIFY"
                    argv = @($GoPath, "mod", "verify")
                    cwd = $RepoRoot; expect = "all modules verified"; go = $true }
    } elseif ($g -eq "G08") {
        $plan += @{ id = "G08"; kind = "PRODUCTION_BUILD"
                    argv = @($BunPath, "--no-env-file", "run", "build")
                    cwd = $Web; expect = "exit 0" }
    }
}
$state.plan = $plan

if ($DryRun) {
    $state.result = "DRY_RUN"
    Write-JsonFile (Join-Path $AuditDir "verify_plan.json") $state
    Write-Host "RESULT = DRY_RUN"
    Write-Host ("PLAN_GATES = " + ($selected -join ","))
    exit 0
}

# ------------------------------------------------------------------- tool smoke tests
# A stripped or otherwise hostile environment can make a pinned binary exit before printing
# anything. Without this probe that would surface as a failed gate and send the reader after the
# product instead of the environment, so it is checked explicitly and fails closed.
$bunSmoke = Invoke-Native -Id "PRE_BUN" -Argv @($BunPath, "--version") `
    -Cwd $RepoRoot -ChildEnv (New-ChildEnv (Join-Path $AuditDir "preflight\bun_temp")) `
    -TimeoutSec 120 -Root "preflight"
$bunVersionText = ($bunSmoke.text).Trim()
$state.toolchain["bun"]["reported_version"] = $bunVersionText
$bunSmokeOk = ($bunSmoke.exit_code -eq 0) -and $bunVersionText.Contains($PinBunVersion)
if ($bunSmokeOk) { $state.preconditions["bun_smoke_test"] = "PASS" }
else { $state.preconditions["bun_smoke_test"] = "FAIL" }
if (-not $bunSmokeOk) {
    Complete-Run "FAIL" "TOOL_SMOKE_TEST" ("pinned Bun did not answer --version under the child " +
        "environment (exit=" + $bunSmoke.exit_code + "); this is an environment problem, not a gate") 14
}

$goSmoke = Invoke-Native -Id "PRE_GO" -Argv @($GoPath, "version") `
    -Cwd $RepoRoot -ChildEnv (New-GoEnv (Join-Path $AuditDir "preflight\go_temp")) `
    -TimeoutSec 120 -Root "preflight"
$goVersionText = ($goSmoke.text).Trim()
$state.toolchain["go"]["reported_version"] = $goVersionText
$goSmokeOk = ($goSmoke.exit_code -eq 0) -and $goVersionText.Contains($PinGoVersion)
if ($goSmokeOk) { $state.preconditions["go_smoke_test"] = "PASS" }
else { $state.preconditions["go_smoke_test"] = "FAIL" }
if (-not $goSmokeOk) {
    Complete-Run "FAIL" "TOOL_SMOKE_TEST" ("pinned Go did not answer version under the child " +
        "environment (exit=" + $goSmoke.exit_code + ")") 15
}
Write-Host ("[PRE] tool smoke PASS  bun=" + $bunVersionText + "  go=" + $goVersionText)

# --------------------------------------------------------------------------- G08 shim
$shimDir = Join-Path $AuditDir "shim"
if ($selected -contains "G08") {
    New-Item -ItemType Directory -Force -Path $shimDir | Out-Null
    foreach ($n in @("bun.exe", "node.exe")) {
        Copy-Item -LiteralPath $BunPath -Destination (Join-Path $shimDir $n) -Force
    }
    $shimBun = Get-Sha256 (Join-Path $shimDir "bun.exe")
    $shimNode = Get-Sha256 (Join-Path $shimDir "node.exe")
    $shimOk = ($shimBun -eq $PinBunSha) -and ($shimNode -eq $PinBunSha)
    $state.shim = @{ dir = $shimDir; bun_exe = $shimBun; node_exe = $shimNode; both_match = $shimOk }
    Write-JsonFile (Join-Path $AuditDir "SHIM_IDENTITY.json") $state.shim
    if (-not $shimOk) {
        Complete-Run "FAIL" "G08_SHIM" "audit-local Bun-as-Node shim identity mismatch" 23
    }
}

# --------------------------------------------------------------------------- run gates
foreach ($g in $plan) {
    $gateDir = Join-Path (Join-Path $AuditDir "gate") $g.id
    New-Item -ItemType Directory -Force -Path $gateDir | Out-Null
    $temp = Join-Path $gateDir "temp"

    if ($g.go) {
        $childEnv = New-GoEnv $temp
        $childEnv["HN_R22_BUN"] = $BunPath
        $childEnv["HN_R26_BUN"] = $BunPath
    } else {
        $childEnv = New-ChildEnv $temp
    }

    $timeout = 1800
    if ($g.id -eq "G01") { $timeout = 600 }
    if ($g.id -eq "G03") { $timeout = 900 }
    if ($g.id -eq "G04") { $timeout = 900 }
    if ($g.id -eq "G05B") { $timeout = 2100 }
    if ($g.id -eq "G06") { $timeout = 1200 }
    if ($g.id -eq "G07") { $timeout = 600 }
    if ($g.id -eq "G08") { $timeout = 2400 }

    if ($g.id -eq "G08") {
        $nodeBin = Join-Path $Web "node_modules\.bin"
        $childEnv["PATH"] = @(
            $shimDir,
            (Split-Path -Parent $BunPath),
            (Split-Path -Parent $GoPath),
            $nodeBin,
            "C:\Windows\system32",
            "C:\Windows",
            "C:\Windows\System32\Wbem"
        ) -join ";"
        $childEnv["NEXT_TELEMETRY_DISABLED"] = "1"
    }

    $r = Invoke-Native -Id $g.id -Argv $g.argv -Cwd $g.cwd -ChildEnv $childEnv -TimeoutSec $timeout

    $counts = @{
        pass = Get-TestCount $r.text "pass"
        skip = Get-TestCount $r.text "skip"
        fail = Get-TestCount $r.text "fail"
    }

    $activated = $null
    $skipped = $null
    $failedNamed = $null
    $ok = $false

    if ($g.id -eq "G01") {
        $ok = ($r.exit_code -eq 0) -and ($counts.pass -eq 2) -and ($counts.fail -eq 0)
    } elseif ($g.id -eq "G02") {
        $ok = ($r.exit_code -eq 0) -and ($counts.pass -eq 239) -and ($counts.skip -eq 2) -and ($counts.fail -eq 0)
    } elseif ($g.id -eq "G03") {
        $ok = ($r.exit_code -eq 0) -and ($counts.pass -eq 221) -and ($counts.skip -eq 2) -and ($counts.fail -eq 0)
    } elseif ($g.id -eq "G04") {
        $ok = ($r.exit_code -eq 0)
    } elseif ($g.id -eq "G05A") {
        $activated = 0
        $skipped = 0
        $failedNamed = 0
        foreach ($t in $ContractTests) {
            $re = [regex]::Escape($t)
            if ($r.text -match ("(?m)^--- PASS: " + $re)) { $activated++ }
            if ($r.text -match ("(?m)^--- SKIP: " + $re)) { $skipped++ }
            if ($r.text -match ("(?m)^--- FAIL: " + $re)) { $failedNamed++ }
        }
        $ok = ($r.exit_code -eq 0) -and ($activated -eq 3) -and ($skipped -eq 0) -and ($failedNamed -eq 0)
    } elseif ($g.id -eq "G05B") {
        $ok = ($r.exit_code -eq 0)
    } elseif ($g.id -eq "G06") {
        $ok = ($r.exit_code -eq 0)
    } elseif ($g.id -eq "G07") {
        $ok = ($r.exit_code -eq 0) -and ($r.text.ToLowerInvariant().Contains("all modules verified"))
    } elseif ($g.id -eq "G08") {
        $ok = ($r.exit_code -eq 0)
    }

    $entry = @{
        id = $g.id; kind = $g.kind; expect = $g.expect; pass = $ok
        exit_code = $r.exit_code; timed_out = $r.timed_out; duration_ms = $r.duration_ms
        argv = $r.argv; cwd = $r.cwd; counts = $counts
        activated_contract_tests = $activated
        skipped_contract_tests = $skipped
        failed_contract_tests = $failedNamed
        env_relevant = $r.env_relevant
        stdout = (Join-Path $gateDir "stdout.txt")
        stderr = (Join-Path $gateDir "stderr.txt")
    }
    $state.gates += $entry
    Write-JsonFile (Join-Path $gateDir "meta.json") $entry

    $verdict = "FAIL"
    if ($ok) { $verdict = "PASS" }
    Write-Host ("[" + $g.id + "] " + $verdict + " exit=" + $r.exit_code +
                " pass=" + $counts.pass + " skip=" + $counts.skip + " fail=" + $counts.fail +
                " in " + [int]($r.duration_ms / 1000) + "s")

    if (-not $ok) {
        $why = "gate " + $g.id + " did not meet: " + $g.expect
        if ($g.id -eq "G05A") {
            $why = $why + " (activated=" + $activated + " skipped=" + $skipped + " failed=" + $failedNamed + ")"
        }
        Complete-Run "FAIL" $g.id $why 2
    }
}

Complete-Run "PASS" $null $null 0
