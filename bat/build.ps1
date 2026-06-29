Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

function Invoke-NativeCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    $argText = if ($Arguments.Count -gt 0) { " $($Arguments -join ' ')" } else { "" }
    Write-Host ">> $FilePath$argText"
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath$argText failed with exit code $LASTEXITCODE"
    }
}

function Resolve-RequiredCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -eq $command -or [string]::IsNullOrWhiteSpace($command.Source)) {
        $fallbacks = @()
        switch ($Name.ToLowerInvariant()) {
            "go" {
                if (-not [string]::IsNullOrWhiteSpace($env:GOROOT)) {
                    $fallbacks += (Join-Path $env:GOROOT "bin\go.exe")
                }
                $fallbacks += @(
                    "C:\Program Files\Go\bin\go.exe",
                    "C:\Go\bin\go.exe"
                )
            }
            "wails" {
                if (-not [string]::IsNullOrWhiteSpace($env:GOPATH)) {
                    $fallbacks += (Join-Path $env:GOPATH "bin\wails.exe")
                }
                if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
                    $fallbacks += (Join-Path $env:USERPROFILE "go\bin\wails.exe")
                }
            }
            "npm" {
                $fallbacks += @(
                    "C:\Program Files\nodejs\npm.cmd",
                    "C:\Program Files (x86)\nodejs\npm.cmd"
                )
            }
        }

        foreach ($candidate in $fallbacks) {
            if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
                return $candidate
            }
        }

        throw "Required command not found in PATH: $Name"
    }

    return $command.Source
}

function Assert-RequiredSourceFiles {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Action,
        [Parameter(Mandatory = $true)]
        [string[]]$Paths
    )

    $missing = @()
    foreach ($relativePath in $Paths) {
        $fullPath = Join-Path $repoRoot $relativePath
        if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
            $missing += $relativePath
        }
    }

    if ($missing.Count -gt 0) {
        throw "$Action requires a complete source tree. Missing files: $($missing -join ', ')"
    }
}

function Test-TcpEndpoint {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Host,
        [Parameter(Mandatory = $true)]
        [int]$Port,
        [int]$TimeoutMs = 1200
    )

    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect($Host, $Port, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne($TimeoutMs, $false)) {
            return $false
        }

        $client.EndConnect($async)
        return $true
    }
    catch {
        return $false
    }
    finally {
        $client.Dispose()
    }
}

function Get-OptionalEnvValue {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $item = Get-Item ("Env:" + $Name) -ErrorAction SilentlyContinue
    if ($null -eq $item) {
        return $null
    }

    return $item.Value
}

function Enable-OptionalBuildProxy {
    $proxyCandidates = @(@(
        (Get-OptionalEnvValue -Name "ANT_BROWSER_BUILD_PROXY_URL"),
        (Get-OptionalEnvValue -Name "HTTPS_PROXY"),
        (Get-OptionalEnvValue -Name "HTTP_PROXY"),
        (Get-OptionalEnvValue -Name "https_proxy"),
        (Get-OptionalEnvValue -Name "http_proxy")
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })

    if ($proxyCandidates.Count -eq 0) {
        Write-Host "[0/7] No build proxy configured, using direct network."
        Write-Host ""
        return $false
    }

    $proxyValue = $proxyCandidates[0].Trim()
    $proxyUri = $null
    if (-not [System.Uri]::TryCreate($proxyValue, [System.UriKind]::Absolute, [ref]$proxyUri)) {
        Write-Host "[0/7] Ignoring invalid build proxy: $proxyValue"
        Write-Host ""
        return $false
    }

    if ([string]::IsNullOrWhiteSpace($proxyUri.Host) -or $proxyUri.Port -le 0) {
        Write-Host "[0/7] Ignoring unusable build proxy: $proxyValue"
        Write-Host ""
        return $false
    }

    Write-Host "[0/7] Validating build proxy..."
    if (-not (Test-TcpEndpoint -Host $proxyUri.Host -Port $proxyUri.Port)) {
        Write-Host "[WARN] Build proxy unreachable, falling back to direct network: $proxyValue"
        Write-Host ""
        return $false
    }

    $env:HTTP_PROXY = $proxyValue
    $env:HTTPS_PROXY = $proxyValue
    $env:http_proxy = $proxyValue
    $env:https_proxy = $proxyValue
    if ([string]::IsNullOrWhiteSpace($env:GOPROXY)) {
        $env:GOPROXY = "https://goproxy.cn,direct"
    }

    Write-Host "OK proxy configured: $proxyValue"
    Write-Host ""
    return $true
}

try {
    Write-Host "========================================"
    Write-Host "  Maka Browser - Build Script"
    Write-Host "========================================"
    Write-Host ""
    Write-Host "Current workdir: $repoRoot"
    Write-Host ""

    $originalEnv = @(
        @{ Name = "HTTP_PROXY"; Value = $env:HTTP_PROXY },
        @{ Name = "HTTPS_PROXY"; Value = $env:HTTPS_PROXY },
        @{ Name = "http_proxy"; Value = $env:http_proxy },
        @{ Name = "https_proxy"; Value = $env:https_proxy },
        @{ Name = "GOPROXY"; Value = $env:GOPROXY },
        @{ Name = "PATH"; Value = $env:PATH },
        @{ Name = "GOROOT"; Value = $env:GOROOT }
    )
    Enable-OptionalBuildProxy | Out-Null

    Assert-RequiredSourceFiles -Action "Building from source" -Paths @(
        "go.mod",
        "go.sum",
        "main.go",
        "wails.json"
    )

    $npmExe = Resolve-RequiredCommand -Name "npm"
    $goExe = Resolve-RequiredCommand -Name "go"
    $wailsExe = Resolve-RequiredCommand -Name "wails"
    $goBinDir = Split-Path -Path $goExe -Parent
    $goRootDir = Split-Path -Path $goBinDir -Parent

    if (-not [string]::IsNullOrWhiteSpace($goBinDir)) {
        $pathEntries = @($env:PATH -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        $hasGoBin = $false
        foreach ($entry in $pathEntries) {
            if ($entry.TrimEnd('\').ToLowerInvariant() -eq $goBinDir.TrimEnd('\').ToLowerInvariant()) {
                $hasGoBin = $true
                break
            }
        }

        if (-not $hasGoBin) {
            $env:PATH = "$goBinDir;$env:PATH"
        }
    }

    if ([string]::IsNullOrWhiteSpace($env:GOROOT) -and -not [string]::IsNullOrWhiteSpace($goRootDir)) {
        $env:GOROOT = $goRootDir
    }

    Write-Host "Resolved commands:"
    Write-Host "  npm   = $npmExe"
    Write-Host "  go    = $goExe"
    Write-Host "  wails = $wailsExe"
    Write-Host "  GOROOT = $env:GOROOT"
    Write-Host "  PATH+go = $goBinDir"
    Write-Host ""

    Write-Host "[1/7] Installing frontend dependencies..."
    Push-Location (Join-Path $repoRoot "frontend")
    try {
        $env:BROWSERSLIST_IGNORE_OLD_DATA = "1"
        Invoke-NativeCommand -FilePath $npmExe -Arguments @("ci", "--prefer-offline", "--no-audit", "--no-fund")
        Invoke-NativeCommand -FilePath $npmExe -Arguments @("run", "ensure:native")
    }
    finally {
        Remove-Item Env:BROWSERSLIST_IGNORE_OLD_DATA -ErrorAction SilentlyContinue
        Pop-Location
    }

    Write-Host ""
    Write-Host "[2/7] Installing Go dependencies..."
    Invoke-NativeCommand -FilePath $goExe -Arguments @("version")
    Invoke-NativeCommand -FilePath $goExe -Arguments @("mod", "download")
    Invoke-NativeCommand -FilePath $goExe -Arguments @("mod", "tidy")

    Write-Host ""
    Write-Host "[3/7] Ensuring frontend\dist exists..."
    $frontendDist = Join-Path $repoRoot "frontend/dist"
    $tempDistCreated = $false
    if (-not (Test-Path -LiteralPath $frontendDist)) {
        New-Item -ItemType Directory -Path $frontendDist -Force | Out-Null
        Set-Content -LiteralPath (Join-Path $frontendDist "index.html") -Value "" -Encoding ascii
        $tempDistCreated = $true
        Write-Host "OK temporary dist directory created"
    } else {
        Write-Host "OK dist directory already exists"
    }

    Write-Host ""
    Write-Host "[4/7] Generating Wails bindings..."
    $originalWailsBin = $env:WAILS_BIN
    try {
        $env:WAILS_BIN = $wailsExe
        Invoke-NativeCommand -FilePath "cmd" -Arguments @("/c", "call bat\generate-bindings.bat --no-pause")
    }
    finally {
        if ($null -eq $originalWailsBin) {
            Remove-Item Env:WAILS_BIN -ErrorAction SilentlyContinue
        }
        else {
            $env:WAILS_BIN = $originalWailsBin
        }
    }

    $binaryPath = Join-Path $repoRoot "build/bin/ant-chrome.exe"

    Write-Host ""
    Write-Host "[5/7] Building frontend..."
    if ($tempDistCreated -and (Test-Path -LiteralPath $frontendDist)) {
        Remove-Item -LiteralPath $frontendDist -Recurse -Force -ErrorAction SilentlyContinue
    }
    Push-Location (Join-Path $repoRoot "frontend")
    try {
        $env:BROWSERSLIST_IGNORE_OLD_DATA = "1"
        Invoke-NativeCommand -FilePath $npmExe -Arguments @("run", "build")
    }
    finally {
        Remove-Item Env:BROWSERSLIST_IGNORE_OLD_DATA -ErrorAction SilentlyContinue
        Pop-Location
    }

    Write-Host ""
    Write-Host "[6/7] Building app..."
    Invoke-NativeCommand -FilePath $wailsExe -Arguments @("build")

    if ($tempDistCreated -and (Test-Path -LiteralPath $frontendDist)) {
        Remove-Item -LiteralPath $frontendDist -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host ""
    Write-Host "[7/7] Copying runtime dependencies..."
    $binDir = Join-Path $repoRoot "bin"
    $targetDir = Join-Path $repoRoot "build/bin/bin"
    if (Test-Path -LiteralPath $binDir -PathType Container) {
        Copy-Item -LiteralPath $binDir -Destination $targetDir -Recurse -Force
        Write-Host "OK copied bin directory to build\bin\bin\"
    } else {
        Write-Host "[WARN] bin directory not found, skipping copy"
    }

    Write-Host ""
    Write-Host "========================================"
    Write-Host "  OK build completed"
    Write-Host "========================================"
    Write-Host ""
    Write-Host "Executable: build\bin\ant-chrome.exe"
    exit 0
}
catch {
    Write-Host ""
    Write-Host "[ERROR] $($_.Exception.Message)"
    exit 1
}
finally {
    foreach ($entry in $originalEnv) {
        if ($null -eq $entry.Value) {
            Remove-Item ("Env:" + $entry.Name) -ErrorAction SilentlyContinue
        }
        else {
            Set-Item ("Env:" + $entry.Name) $entry.Value
        }
    }
}
