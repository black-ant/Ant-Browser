param(
    [string]$AppRoot = '.',
    [switch]$Apply,
    [switch]$RepairRisky,
    [string[]]$Only
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

try {
    $goCmd = Get-Command go -ErrorAction Stop
} catch {
    throw "Go was not found in PATH. Install Go before running this script."
}

$resolvedAppRoot = $AppRoot
if (-not [System.IO.Path]::IsPathRooted($resolvedAppRoot)) {
    $resolvedAppRoot = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $resolvedAppRoot))
}

$toolArgs = @(
    'run',
    './backend/cmd/profile-recover',
    '--app-root',
    $resolvedAppRoot
)

if ($Apply) {
    $toolArgs += '--apply'
}

if ($RepairRisky) {
    $toolArgs += @('--repair-strategy', 'risky')
}

if ($Only -and $Only.Count -gt 0) {
    $joined = ($Only | ForEach-Object { $_.Trim() } | Where-Object { $_ }) -join ','
    if ($joined) {
        $toolArgs += @('--only', $joined)
    }
}

Write-Host "RepoRoot: $repoRoot"
Write-Host "AppRoot:  $resolvedAppRoot"
if ($Apply) {
    Write-Host "Mode:     apply"
    Write-Host "Notice:   Close ZwBrowser before apply mode."
} else {
    Write-Host "Mode:     preview"
}
if ($RepairRisky) {
    Write-Host "Repair:   risky"
}

Push-Location $repoRoot
try {
    & $goCmd.Source @toolArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}
