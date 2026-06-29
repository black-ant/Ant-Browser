param(
    [Parameter(Mandatory = $true)]
    [string]$ReleaseVersion,
    [Parameter(Mandatory = $true)]
    [string]$CommitSha,
    [string]$Channel = "test",
    [string]$OutputDir = "publish\output",
    [string]$ReportDir = "publish\output"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Version = $ReleaseVersion

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$resolvedOutputDir = Join-Path $repoRoot $OutputDir
$resolvedReportDir = Join-Path $repoRoot $ReportDir
New-Item -ItemType Directory -Force $resolvedReportDir | Out-Null

function Get-Sha256 {
    param([string]$Path)
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Get-Artifact {
    param([string]$RelativePath)

    $path = Join-Path $resolvedOutputDir $RelativePath
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "artifact missing: $path"
    }

    $item = Get-Item -LiteralPath $path
    return [ordered]@{
        name = $RelativePath
        path = $item.FullName
        size = $item.Length
        sha256 = Get-Sha256 $item.FullName
    }
}

$artifacts = @(
    Get-Artifact "MakaBrowser-Setup-$Version.exe"
    Get-Artifact "MakaBrowser-$Version-windows-amd64.zip"
    Get-Artifact "MakaBrowser-$Version-windows-amd64.zip.sha256"
    Get-Artifact "app-update-stable.json"
    Get-Artifact "app-update-stable.json.sha256"
)

$report = [ordered]@{
    schemaVersion = 1
    product = "Maka Browser"
    platform = "windows-amd64"
    channel = $Channel
    version = $Version
    commitSha = $CommitSha
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    runnerName = $env:RUNNER_NAME
    runnerLabels = $env:RUNNER_LABELS
    artifacts = $artifacts
}

$jsonPath = Join-Path $resolvedReportDir "release-report.json"
$mdPath = Join-Path $resolvedReportDir "release-report.md"
$report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $jsonPath -Encoding UTF8

$lines = @()
$lines += "# Windows Release Report"
$lines += ""
$lines += "- Version: ``$Version``"
$lines += "- Channel: ``$Channel``"
$lines += "- Commit: ``$CommitSha``"
$lines += "- Generated: ``$($report.generatedAt)``"
$lines += "- Runner: ``$($report.runnerName)``"
$lines += ""
$lines += "## Artifacts"
$lines += ""
$lines += "| File | Size | SHA256 |"
$lines += "| --- | ---: | --- |"
foreach ($artifact in $artifacts) {
    $lines += "| ``$($artifact.name)`` | $($artifact.size) | ``$($artifact.sha256)`` |"
}
$lines += ""
$lines -join [Environment]::NewLine | Set-Content -LiteralPath $mdPath -Encoding UTF8

Write-Host "[OK] release report written:"
Write-Host $jsonPath
Write-Host $mdPath
