param(
    [string]$BaselineVersion = "1.1.0",
    [string]$TargetVersion = "1.1.5",
    [string]$TestRoot = "C:\MakaBrowserUpdateTest",
    [int]$RunnerWaitSeconds = 90,
    [switch]$SkipPublish
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$outputDir = Join-Path $repoRoot "publish\output"
$baselineDir = Join-Path $TestRoot "baseline"
$targetDir = Join-Path $TestRoot "target"
$installRoot = Join-Path $env:LOCALAPPDATA "Programs\Maka Browser"
$stateRoot = Join-Path $env:LOCALAPPDATA "Maka Browser"
$statePath = Join-Path $stateRoot "app-update\state.json"
$manifestPath = Join-Path $targetDir "app-update-stable.json"
$targetZip = Join-Path $targetDir "MakaBrowser-$TargetVersion-windows-amd64.zip"
$baselineInstaller = Join-Path $baselineDir "MakaBrowser-Setup-$BaselineVersion.exe"
$targetInstaller = Join-Path $targetDir "MakaBrowser-Setup-$TargetVersion.exe"
$extractDir = Join-Path $targetDir "extracted"
$harnessDir = Join-Path $repoRoot "backend\cmd\app-update-e2e"
$harnessPath = Join-Path $harnessDir "main.go"

function Write-Step {
    param([string]$Text)
    Write-Host ""
    Write-Host "== $Text =="
}

function Require-File {
    param([string]$Path, [string]$Label)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Label missing: $Path"
    }
}

function Wait-ForFile {
    param([string]$Path, [string]$Label, [int]$TimeoutSeconds = 10)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            return
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    Require-File -Path $Path -Label $Label
}

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "required command not found: $Name"
    }
}

function Get-FileSHA256OrMissing {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return "<missing>"
    }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
}

function Read-AppUpdateStateOrNull {
    if (-not (Test-Path -LiteralPath $statePath -PathType Leaf)) {
        return $null
    }
    return Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
}

function Invoke-Native {
    param(
        [string]$FilePath,
        [string[]]$Arguments = @()
    )
    $global:LASTEXITCODE = 0
    & $FilePath @Arguments
    $exitCode = 0
    if (Test-Path variable:LASTEXITCODE) {
        $exitCode = $global:LASTEXITCODE
    }
    if ($exitCode -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $exitCode"
    }
}

function Stop-MakaBrowser {
    foreach ($name in @("ant-chrome", "xray", "sing-box")) {
        foreach ($process in @(Get-Process $name -ErrorAction SilentlyContinue)) {
            try {
                Stop-Process -Id $process.Id -Force -ErrorAction Stop
            }
            catch {
                Write-Warning "could not stop process $name pid=$($process.Id): $($_.Exception.Message)"
            }
        }
    }
    foreach ($image in @("ant-chrome.exe", "xray.exe", "sing-box.exe")) {
        & "$env:WINDIR\System32\cmd.exe" /c "taskkill /F /T /IM $image >NUL 2>NUL"
        $global:LASTEXITCODE = 0
    }
    Start-Sleep -Milliseconds 1200
}

function Seed-PreservedDirectories {
    foreach ($preserved in @("runtime", "diagnostics")) {
        $dir = Join-Path $installRoot $preserved
        New-Item -ItemType Directory -Force $dir | Out-Null
        Set-Content -LiteralPath (Join-Path $dir "app-update-preserve-marker.txt") -Value "preserve:$preserved" -Encoding UTF8
    }
}

function Seed-UserData {
    $dataDir = Join-Path $installRoot "data"
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    Set-Content -LiteralPath (Join-Path $dataDir "app.db") -Value "app-update-user-data-marker" -Encoding UTF8
}

function Reset-E2EInstallRoot {
    Write-Step "Reset e2e install root"
    Stop-MakaBrowser
    $deadline = (Get-Date).AddSeconds(15)
    do {
        Remove-Item -Recurse -Force $installRoot -ErrorAction SilentlyContinue
        if (-not (Test-Path -LiteralPath $installRoot)) {
            break
        }
        Stop-MakaBrowser
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    if (Test-Path -LiteralPath $installRoot) {
        throw "failed to reset install root: $installRoot"
    }
    Remove-Item -Recurse -Force (Join-Path $stateRoot "app-update") -ErrorAction SilentlyContinue
}

function Wait-ForUpdateRunner {
    Write-Step "Wait for runner"
    $deadline = (Get-Date).AddSeconds($RunnerWaitSeconds)
    $lastStateJson = "<missing>"
    do {
        Start-Sleep -Seconds 1
        $state = Read-AppUpdateStateOrNull
        if ($null -ne $state -and ($state.PSObject.Properties.Name -contains "status")) {
            $lastStateJson = ($state | ConvertTo-Json -Depth 10 -Compress)
            $status = [string]$state.status
            if ($status -in @("succeeded", "failed_manual_repair", "rolled_back")) {
                Write-Host "Runner terminal status: $status"
                return
            }
        }
    } while ((Get-Date) -lt $deadline)
    throw "runner did not reach terminal status after $RunnerWaitSeconds seconds; lastState=$lastStateJson"
}

function Copy-ReleaseArtifacts {
    param(
        [string]$Version,
        [string]$Destination
    )
    New-Item -ItemType Directory -Force $Destination | Out-Null
    Copy-Item -LiteralPath (Join-Path $outputDir "MakaBrowser-Setup-$Version.exe") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "MakaBrowser-$Version-windows-amd64.zip") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "MakaBrowser-$Version-windows-amd64.zip.sha256") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "app-update-stable.json") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "app-update-stable.json.sha256") -Destination $Destination -Force
}

function Test-ReleaseArtifacts {
    param([string]$Version)
    $required = @(
        (Join-Path $outputDir "MakaBrowser-Setup-$Version.exe"),
        (Join-Path $outputDir "MakaBrowser-$Version-windows-amd64.zip"),
        (Join-Path $outputDir "MakaBrowser-$Version-windows-amd64.zip.sha256"),
        (Join-Path $outputDir "app-update-stable.json"),
        (Join-Path $outputDir "app-update-stable.json.sha256")
    )
    foreach ($path in $required) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            return $false
        }
    }
    return $true
}

function Copy-PrebuiltTargetArtifacts {
    if (-not (Test-ReleaseArtifacts -Version $TargetVersion)) {
        return $false
    }
    Write-Step "Use prebuilt target artifacts $TargetVersion"
    Copy-ReleaseArtifacts -Version $TargetVersion -Destination $targetDir
    return $true
}

function Restore-PrebuiltTargetArtifacts {
    Write-Step "Restore prebuilt target artifacts $TargetVersion"
    New-Item -ItemType Directory -Force $outputDir | Out-Null
    Copy-Item -LiteralPath (Join-Path $targetDir "MakaBrowser-Setup-$TargetVersion.exe") -Destination $outputDir -Force
    Copy-Item -LiteralPath (Join-Path $targetDir "MakaBrowser-$TargetVersion-windows-amd64.zip") -Destination $outputDir -Force
    Copy-Item -LiteralPath (Join-Path $targetDir "MakaBrowser-$TargetVersion-windows-amd64.zip.sha256") -Destination $outputDir -Force
    Copy-Item -LiteralPath (Join-Path $targetDir "app-update-stable.json") -Destination $outputDir -Force
    Copy-Item -LiteralPath (Join-Path $targetDir "app-update-stable.json.sha256") -Destination $outputDir -Force
}

function Publish-Version {
    param([string]$Version, [string]$Destination)
    Write-Step "Publish $Version"
    Remove-Item -Recurse -Force $outputDir -ErrorAction SilentlyContinue
    & (Join-Path $repoRoot "bat\publish.ps1") -Target "W" -ReleaseVersion $Version
    Invoke-Native -FilePath "python" -Arguments @(
        (Join-Path $repoRoot "tools\app-update\verify-app-update-package.py"),
        (Join-Path $outputDir "app-update-stable.json"),
        (Join-Path $outputDir "MakaBrowser-$Version-windows-amd64.zip"),
        "windows-amd64"
    )
    Copy-ReleaseArtifacts -Version $Version -Destination $Destination
}

function Write-Harness {
    Write-Step "Write app-update e2e harness"
    New-Item -ItemType Directory -Force $harnessDir | Out-Null
    @"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ant-chrome/backend/internal/appupdate"
)

func dump(label string, value any) {
	data, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println("===", label, "===")
	fmt.Println(string(data))
}

func main() {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		panic("LOCALAPPDATA is empty")
	}

	installRoot := filepath.Join(localAppData, "Programs", "Maka Browser")
	stateRoot := filepath.Join(localAppData, "Maka Browser")
	currentExe := filepath.Join(installRoot, "ant-chrome.exe")
	manifestPath := ``$ManifestPath``

	if _, err := os.Stat(currentExe); err != nil {
		panic(err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		panic(err)
	}

	layout := appupdate.NewLayout(installRoot, stateRoot)
	manager := appupdate.Manager{
		LocalAppVersion: "$BaselineVersion",
		Layout: layout,
		Platform: appupdate.WindowsBackend{
			CurrentExePath: currentExe,
			CurrentAppVersion: "$BaselineVersion",
		},
		ManifestProvider: appupdate.DefaultManifestProvider(func() appupdate.ManifestSourceResolution {
			return appupdate.ManifestSourceResolution{
				URL: manifestPath,
				Source: "local-e2e",
			}
		}),
	}

	ctx := context.Background()
	check, err := manager.Check(ctx)
	if err != nil {
		panic(err)
	}
	dump("check", check)

	download, err := manager.Download(ctx)
	if err != nil {
		panic(err)
	}
	dump("download", download)

	apply, err := manager.Apply(ctx)
	if err != nil {
		panic(err)
	}
	dump("apply", apply)
	fmt.Println("Apply runner spawned. This process will exit now.")
	time.Sleep(500 * time.Millisecond)
}
"@ | Set-Content -LiteralPath $harnessPath -Encoding UTF8
}

function Assert-UpdateSucceeded {
    Write-Step "Verify app-update result"
    Require-File -Path $statePath -Label "app-update state.json"
    $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
    Write-Host "State at verify: $($state | ConvertTo-Json -Depth 10 -Compress)"
    if ([string]$state.localAppVersion -ne $TargetVersion) {
        $lastError = ""
        if ($state.PSObject.Properties.Name -contains "lastError") {
            $lastError = ($state.lastError | ConvertTo-Json -Depth 10 -Compress)
        }
        throw "localAppVersion mismatch: expected $TargetVersion, got $($state.localAppVersion); lastError=$lastError"
    }
    if ($state.PSObject.Properties.Name -contains "lastError" -and $null -ne $state.lastError) {
        $lastErrorCode = ""
        $lastErrorMessage = ""
        if ($state.lastError.PSObject.Properties.Name -contains "code") {
            $lastErrorCode = [string]$state.lastError.code
        }
        if ($state.lastError.PSObject.Properties.Name -contains "message") {
            $lastErrorMessage = [string]$state.lastError.message
        }
        if ($lastErrorCode.Trim() -ne "" -or $lastErrorMessage.Trim() -ne "") {
            $lastError = ($state.lastError | ConvertTo-Json -Depth 10 -Compress)
            throw "app-update lastError is not empty: $lastError"
        }
    }

    Remove-Item -Recurse -Force $extractDir -ErrorAction SilentlyContinue
    Expand-Archive -LiteralPath $targetZip -DestinationPath $extractDir -Force
    $installedExe = Join-Path $installRoot "ant-chrome.exe"
    $expectedExe = Join-Path $extractDir "ant-chrome.exe"
    Require-File -Path $installedExe -Label "installed ant-chrome.exe"
    Require-File -Path $expectedExe -Label "target zip ant-chrome.exe"
    $installedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $installedExe).Hash
    $expectedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $expectedExe).Hash
    if ($installedHash -ne $expectedHash) {
        $plan = $null
        if (
            (Test-Path -LiteralPath $statePath -PathType Leaf) -and
            ($state.PSObject.Properties.Name -contains "planPath")
        ) {
            $planPath = [string]$state.planPath
            if ($planPath.Trim() -ne "" -and (Test-Path -LiteralPath $planPath -PathType Leaf)) {
                $plan = Get-Content -LiteralPath $planPath -Raw | ConvertFrom-Json
            }
        }
        Write-Host "Installed exe path: $installedExe"
        Write-Host "Expected exe path: $expectedExe"
        Write-Host "Installed exe sha256: $installedHash"
        Write-Host "Expected exe sha256: $expectedHash"
        if ($null -ne $plan) {
            $stagedExe = Join-Path ([string]$plan.stagedPath) "ant-chrome.exe"
            $runnerExe = [string]$plan.runnerPath
            Write-Host "Plan: $($plan | ConvertTo-Json -Depth 10 -Compress)"
            Write-Host "Staged exe sha256: $(Get-FileSHA256OrMissing -Path $stagedExe)"
            Write-Host "Runner exe sha256: $(Get-FileSHA256OrMissing -Path $runnerExe)"
        }
        Write-Host "State: $($state | ConvertTo-Json -Depth 10 -Compress)"
        throw "installed exe hash mismatch: expected $expectedHash, got $installedHash"
    }

    Require-File -Path (Join-Path $installRoot "data\app.db") -Label "data\app.db"
    foreach ($preserved in @("runtime", "diagnostics")) {
        $preservedDir = Join-Path $installRoot $preserved
        $markerPath = Join-Path $preservedDir "app-update-preserve-marker.txt"
        if (-not (Test-Path -LiteralPath $preservedDir -PathType Container)) {
            throw "preserved directory missing: $preserved"
        }
        Require-File -Path $markerPath -Label "$preserved preserve marker"
        $marker = (Get-Content -LiteralPath $markerPath -Raw).Trim()
        if ($marker -ne "preserve:$preserved") {
            throw "preserved marker mismatch for ${preserved}: $marker"
        }
    }
    Require-File -Path (Join-Path $installRoot "config.yaml") -Label "config.yaml"

    Write-Host "[OK] Windows app-update e2e passed"
    Write-Host "Installed exe sha256: $installedHash"
    Write-Host "State: $($state | ConvertTo-Json -Depth 10 -Compress)"
}

Write-Step "Preflight"
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw "windows-app-update-e2e.ps1 must run on Windows"
}
Require-Command "go"
Require-Command "python"
Require-File -Path (Join-Path $repoRoot "bat\publish.ps1") -Label "bat\publish.ps1"
New-Item -ItemType Directory -Force $TestRoot | Out-Null

$targetPrebuilt = $false
if (-not $SkipPublish) {
    $targetPrebuilt = Copy-PrebuiltTargetArtifacts
    Publish-Version -Version $BaselineVersion -Destination $baselineDir
    if ($targetPrebuilt) {
        Write-Step "Skip target publish $TargetVersion"
    }
    else {
        Publish-Version -Version $TargetVersion -Destination $targetDir
    }
}

Require-File -Path $baselineInstaller -Label "baseline installer"
Require-File -Path $targetInstaller -Label "target installer"
Require-File -Path $manifestPath -Label "target app-update manifest"
Require-File -Path $targetZip -Label "target app-update zip"

Write-Step "Install baseline $BaselineVersion"
Reset-E2EInstallRoot
Invoke-Native -FilePath $baselineInstaller -Arguments @("/S")
Stop-MakaBrowser
Wait-ForFile -Path (Join-Path $installRoot "ant-chrome.exe") -Label "baseline ant-chrome.exe" -TimeoutSeconds 15
Seed-UserData
Seed-PreservedDirectories
if (Test-Path -LiteralPath (Join-Path $installRoot "data\app.db") -PathType Leaf) {
    $beforeDataHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $installRoot "data\app.db")).Hash
} else {
    $beforeDataHash = ""
}

Write-Step "Configure local manifest"
[Environment]::SetEnvironmentVariable("DESKTOP_APP_UPDATE_MANIFEST_URL", $manifestPath, "User")
$env:DESKTOP_APP_UPDATE_MANIFEST_URL = $manifestPath

Write-Harness
Write-Step "Run Check -> Download -> Apply"
Push-Location $repoRoot
try {
    Invoke-Native -FilePath "go" -Arguments @("run", ".\backend\cmd\app-update-e2e")
}
finally {
    Pop-Location
}

Wait-ForUpdateRunner
Stop-MakaBrowser

if ($beforeDataHash -ne "") {
    $afterDataHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $installRoot "data\app.db")).Hash
    if ($beforeDataHash -ne $afterDataHash) {
        throw "data\app.db hash changed: before $beforeDataHash, after $afterDataHash"
    }
}

Assert-UpdateSucceeded

if ($targetPrebuilt) {
    Restore-PrebuiltTargetArtifacts
}
