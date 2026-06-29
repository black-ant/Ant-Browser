param(
    [string]$Target,
    [string]$ReleaseVersion
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$script:Version = ""
$script:LinuxArch = ""
$script:WindowsDone = $false
$script:LinuxDone = $false

function Write-Section {
    param([string]$Text)

    Write-Host "========================================"
    Write-Host "  $Text"
    Write-Host "========================================"
}

function Get-TrimmedText {
    param([AllowNull()][string]$Value)

    if ($null -eq $Value) {
        return ""
    }
    return $Value.Trim()
}

function Assert-VersionValue {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Value,
        [string]$Source = "版本号"
    )

    $trimmed = Get-TrimmedText $Value
    if ($trimmed -eq "") {
        throw "$Source 不能为空"
    }
    if ($trimmed -notmatch '^\d+\.\d+\.\d+(?:-[0-9A-Za-z\.-]+)?(?:\+[0-9A-Za-z\.-]+)?$') {
        throw "$Source 格式无效: $trimmed`n  期望示例: 1.1.0 或 1.1.0-beta.1"
    }
    return $trimmed
}

function Invoke-NativeCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        $argText = if ($Arguments.Count -gt 0) { " $($Arguments -join ' ')" } else { "" }
        throw "$FilePath$argText failed with exit code $LASTEXITCODE"
    }
}

function Get-FileSha256 {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
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

function Resolve-Version {
    param([string]$ExplicitVersion)

    $explicit = Get-TrimmedText $ExplicitVersion
    if ($explicit -ne "") {
        $script:Version = Assert-VersionValue -Value $explicit -Source "传入版本号"
        Write-Host "[1/3] 使用传入版本号..."
        Write-Host "✓ 版本号: $script:Version"
        Write-Host ""
        return
    }

    Write-Host "[1/3] 读取版本号..."
    $wailsConfigPath = Join-Path $repoRoot "wails.json"
    if (-not (Test-Path -LiteralPath $wailsConfigPath -PathType Leaf)) {
        throw "无法读取版本号：缺少 wails.json"
    }

    $wailsConfig = Get-Content -LiteralPath $wailsConfigPath -Raw | ConvertFrom-Json
    $resolvedVersion = Get-TrimmedText ([string]$wailsConfig.info.productVersion)
    if ($resolvedVersion -eq "") {
        throw "无法从 wails.json 读取版本号"
    }

    $script:Version = Assert-VersionValue -Value $resolvedVersion -Source "wails.json productVersion"
    Write-Host "✓ 版本号: $script:Version"
    Write-Host ""
}

function Invoke-WithTemporaryWailsVersion {
    param(
        [Parameter(Mandatory = $true)]
        [scriptblock]$ScriptBlock
    )

    $wailsConfigPath = Join-Path $repoRoot "wails.json"
    if (-not (Test-Path -LiteralPath $wailsConfigPath -PathType Leaf)) {
        & $ScriptBlock
        return
    }

    $currentConfig = Get-Content -LiteralPath $wailsConfigPath -Raw | ConvertFrom-Json
    $currentVersion = Get-TrimmedText ([string]$currentConfig.info.productVersion)
    if ($currentVersion -eq $script:Version) {
        & $ScriptBlock
        return
    }

    Write-Host "  临时覆盖 wails.json productVersion: $currentVersion -> $script:Version"
    $originalBytes = [System.IO.File]::ReadAllBytes($wailsConfigPath)
    try {
        $currentConfig.info.productVersion = $script:Version
        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        $jsonText = ($currentConfig | ConvertTo-Json -Depth 100)
        [System.IO.File]::WriteAllText($wailsConfigPath, $jsonText + "`n", $utf8NoBom)
        & $ScriptBlock
    }
    finally {
        [System.IO.File]::WriteAllBytes($wailsConfigPath, $originalBytes)
    }
}

function Resolve-PublishTarget {
    param([string]$InputTarget)

    $normalized = (Get-TrimmedText $InputTarget).ToUpperInvariant()
    $mapping = @{
        "W"       = "WINDOWS"
        "WINDOWS" = "WINDOWS"
        "L"       = "LINUX"
        "LINUX"   = "LINUX"
        "B"       = "BOTH"
        "BOTH"    = "BOTH"
    }

    if ($normalized -ne "") {
        if (-not $mapping.ContainsKey($normalized)) {
            throw "无效的打包目标: $InputTarget`n  支持参数: W/L/B 或 WINDOWS/LINUX/BOTH"
        }
        $resolvedTarget = $mapping[$normalized]
        Write-Host "[2/3] 使用预设打包目标: $resolvedTarget"
        Write-Host ""
        return $resolvedTarget
    }

    Write-Host "[2/3] 选择打包平台..."
    Write-Host ""
    Write-Host "  [W] Windows"
    Write-Host "  [L] Linux（通过 Docker Desktop 执行）"
    Write-Host "  [B] Windows + Linux"
    Write-Host ""

    while ($true) {
        $choice = (Read-Host "请选择打包目标 [W/L/B]").Trim().ToUpperInvariant()
        if ($choice -eq "") {
            continue
        }
        if ($mapping.ContainsKey($choice)) {
            $resolvedTarget = $mapping[$choice]
            Write-Host "✓ 已选择: $resolvedTarget"
            Write-Host ""
            return $resolvedTarget
        }
        Write-Host "✗ 未选择有效目标" -ForegroundColor Yellow
    }
}

function Resolve-NsisPath {
    Write-Host "[Windows] 检测 NSIS 安装..."
    Write-Host "  支持环境变量：MAKENSIS_PATH / NSIS_PATH / NSIS_HOME"
    Write-Host ""

    $candidates = @()
    if ($env:MAKENSIS_PATH) {
        $candidates += $env:MAKENSIS_PATH
    }
    if ($env:NSIS_PATH) {
        $candidates += (Join-Path $env:NSIS_PATH "makensis.exe")
        $candidates += $env:NSIS_PATH
    }
    if ($env:NSIS_HOME) {
        $candidates += (Join-Path $env:NSIS_HOME "makensis.exe")
    }

    $whereMakensis = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($whereMakensis) {
        $candidates += $whereMakensis.Source
    }

    $candidates += @(
        "C:\Program Files (x86)\NSIS\makensis.exe",
        "C:\Program Files\NSIS\makensis.exe"
    )

    foreach ($candidate in $candidates) {
        $value = Get-TrimmedText $candidate
        if ($value -eq "") {
            continue
        }
        if (Test-Path -LiteralPath $value -PathType Leaf) {
            Write-Host "✓ NSIS 已就绪: $value"
            Write-Host ""
            return $value
        }
    }

    throw "未找到 NSIS（makensis.exe）`n`n  请安装 NSIS 后，通过以下任一方式配置（PowerShell）：`n    setx MAKENSIS_PATH ""D:\tools\NSIS\makensis.exe""`n    setx NSIS_PATH     ""D:\tools\NSIS""`n    setx NSIS_HOME     ""D:\tools\NSIS""`n`n  或下载安装：https://nsis.sourceforge.io/Download"
}

function Build-WindowsBinary {
    Write-Host "[Windows] 执行 Wails 构建..."
    $binaryPath = Join-Path $repoRoot "build/bin/ant-chrome.exe"
    Assert-RequiredSourceFiles -Action "Windows packaging" -Paths @(
        "go.mod",
        "go.sum",
        "main.go",
        "wails.json"
    )

    $previousGoProxy = $env:GOPROXY
    try {
        $env:GOPROXY = "https://goproxy.cn,direct"
        Push-Location (Join-Path $repoRoot "frontend")
        try {
            Write-Host "[Windows] 预检前端依赖..."
            Invoke-NativeCommand -FilePath "npm" -Arguments @("ci", "--prefer-offline", "--no-audit", "--no-fund")
            Invoke-NativeCommand -FilePath "npm" -Arguments @("run", "ensure:native")
            Write-Host "✓ 前端依赖已就绪"
            Write-Host ""
        }
        finally {
            Pop-Location
        }
        Invoke-NativeCommand -FilePath "wails" -Arguments @("build")
    }
    finally {
        $env:GOPROXY = $previousGoProxy
    }

    if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
        throw "构建产物不存在: build\bin\ant-chrome.exe"
    }
    Write-Host "✓ 构建成功: build\bin\ant-chrome.exe"
    Write-Host ""
}

function Assert-RuntimeHashes {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Target
    )

    $manifestPath = Join-Path $repoRoot "publish/runtime-manifest.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw "缺少运行时清单: publish\runtime-manifest.json"
    }

    Write-Host "[Windows] 校验运行时哈希..."
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    $entries = @($manifest.files | Where-Object { $_.targets -contains $Target })
    if ($entries.Count -eq 0) {
        throw "运行时清单中不存在目标平台: $Target"
    }

    $errors = New-Object System.Collections.Generic.List[string]
    foreach ($entry in $entries) {
        $relativePath = Get-TrimmedText ([string]$entry.path)
        $expectedHash = (Get-TrimmedText ([string]$entry.sha256)).ToLowerInvariant()

        if ($relativePath -eq "") {
            $errors.Add("manifest entry has empty path")
            continue
        }
        if ($expectedHash -eq "" -or $expectedHash.Contains("todo_replace_with_sha256")) {
            $errors.Add("${relativePath}: sha256 is not initialized")
            continue
        }

        $fullPath = Join-Path $repoRoot ($relativePath -replace '/', [System.IO.Path]::DirectorySeparatorChar)
        if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
            $errors.Add("${relativePath}: file not found")
            continue
        }

        $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $fullPath).Hash.ToLowerInvariant()
        if ($actualHash -ne $expectedHash) {
            $errors.Add("${relativePath}: sha256 mismatch (expected $expectedHash, got $actualHash)")
        }
    }

    if ($errors.Count -gt 0) {
        throw "运行时哈希校验失败:`n  - $($errors -join "`n  - ")"
    }

    Write-Host "✓ 运行时哈希校验通过: $Target"
    Write-Host ""
}

function Test-PeExecutable {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath
    )

    if (-not (Test-Path -LiteralPath $FilePath -PathType Leaf)) {
        return $false
    }

    try {
        $stream = [System.IO.File]::OpenRead($FilePath)
        try {
            if ($stream.Length -lt 2) {
                return $false
            }

            $header = New-Object byte[] 2
            $read = $stream.Read($header, 0, $header.Length)
            return ($read -eq 2 -and $header[0] -eq 0x4D -and $header[1] -eq 0x5A)
        }
        finally {
            $stream.Dispose()
        }
    }
    catch {
        return $false
    }
}

function Copy-DirectoryContents {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SourceDir,
        [Parameter(Mandatory = $true)]
        [string]$DestinationDir
    )

    if (-not (Test-Path -LiteralPath $SourceDir -PathType Container)) {
        return
    }

    New-Item -ItemType Directory -Path $DestinationDir -Force | Out-Null

    foreach ($entry in (Get-ChildItem -LiteralPath $SourceDir -Force)) {
        Copy-Item -LiteralPath $entry.FullName -Destination (Join-Path $DestinationDir $entry.Name) -Recurse -Force
    }
}

function Resolve-WindowsWorkspaceAgentRoot {
    $configured = Get-TrimmedText $env:ANT_BROWSER_WORKSPACE_AGENT_ROOT
    if ($configured -ne "") {
        if (-not [System.IO.Path]::IsPathRooted($configured)) {
            $configured = Join-Path $repoRoot $configured
        }
        return $configured
    }

    $repoAgentRoot = Join-Path $repoRoot "apps\agent"
    if (Test-Path -LiteralPath (Join-Path $repoAgentRoot "src\server\index.mjs") -PathType Leaf) {
        return $repoAgentRoot
    }

    return "C:\AntBrowserReleaseResources\apps\agent"
}

function Resolve-WindowsWorkspaceNodeRoot {
    $configured = Get-TrimmedText $env:ANT_BROWSER_WORKSPACE_NODE_ROOT
    if ($configured -ne "") {
        if (-not [System.IO.Path]::IsPathRooted($configured)) {
            $configured = Join-Path $repoRoot $configured
        }
        return $configured
    }

    $repoNodeRoot = Join-Path $repoRoot "runtime\node"
    if (Test-Path -LiteralPath (Join-Path $repoNodeRoot "node.exe") -PathType Leaf) {
        return $repoNodeRoot
    }

    $defaultRunnerNodeRoot = "C:\AntBrowserReleaseResources\runtime\node"
    if (Test-Path -LiteralPath (Join-Path $defaultRunnerNodeRoot "node.exe") -PathType Leaf) {
        return $defaultRunnerNodeRoot
    }

    return ""
}

function Copy-RequiredDirectoryContents {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SourceDir,
        [Parameter(Mandatory = $true)]
        [string]$DestinationDir,
        [Parameter(Mandatory = $true)]
        [string]$RequiredFile,
        [Parameter(Mandatory = $true)]
        [string]$DisplayName
    )

    if (-not (Test-Path -LiteralPath $SourceDir -PathType Container)) {
        throw "缺少 $DisplayName 目录: $SourceDir"
    }
    if (-not (Test-Path -LiteralPath (Join-Path $SourceDir $RequiredFile) -PathType Leaf)) {
        throw "缺少 $DisplayName 必需文件: $(Join-Path $SourceDir $RequiredFile)"
    }

    Copy-DirectoryContents -SourceDir $SourceDir -DestinationDir $DestinationDir
    Write-Host "✓ 复制 $DisplayName"
}

function Copy-WindowsWorkspaceNodeRuntime {
    param(
        [AllowNull()]
        [AllowEmptyString()]
        [string]$SourceDir,
        [Parameter(Mandatory = $true)]
        [string]$DestinationDir
    )

    if ($SourceDir -ne "" -and (Test-Path -LiteralPath (Join-Path $SourceDir "node.exe") -PathType Leaf)) {
        Copy-DirectoryContents -SourceDir $SourceDir -DestinationDir $DestinationDir
        Write-Host "✓ 复制 Node.js 运行时"
        return
    }

    $nodeCommand = Get-Command "node.exe" -ErrorAction SilentlyContinue
    if ($null -eq $nodeCommand) {
        $nodeCommand = Get-Command "node" -ErrorAction SilentlyContinue
    }
    if ($null -eq $nodeCommand -or -not (Test-Path -LiteralPath $nodeCommand.Source -PathType Leaf)) {
        throw "缺少 Node.js 运行时: 未找到 runtime\node\node.exe，且 PATH 中没有 node.exe"
    }

    New-Item -ItemType Directory -Path $DestinationDir -Force | Out-Null
    Copy-Item -LiteralPath $nodeCommand.Source -Destination (Join-Path $DestinationDir "node.exe") -Force
    Write-Host "✓ 从 PATH 复制 Node.js 运行时: $($nodeCommand.Source)"
}

function Resolve-WindowsChromeRoot {
    $configured = Get-TrimmedText $env:ANT_BROWSER_WINDOWS_CHROME_ROOT
    if ($configured -ne "") {
        if (-not [System.IO.Path]::IsPathRooted($configured)) {
            $configured = Join-Path $repoRoot $configured
        }
        return $configured
    }

    $defaultRunnerChromeRoot = "C:\AntBrowserReleaseResources\chrome"
    if ((Get-TrimmedText $env:GITHUB_ACTIONS) -eq "true") {
        return $defaultRunnerChromeRoot
    }

    return Join-Path $repoRoot "chrome"
}

function Resolve-WindowsChromeRequirement {
    $configured = Get-TrimmedText $env:ANT_BROWSER_REQUIRE_WINDOWS_CHROME
    if ($configured -ne "") {
        return ($configured -eq "1")
    }

    return ((Get-TrimmedText $env:GITHUB_ACTIONS) -eq "true")
}

function Copy-WindowsChromePayload {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ChromeRoot,
        [Parameter(Mandatory = $true)]
        [string]$StagingDir,
        [bool]$RequireChrome = $false
    )

    if (-not (Test-Path -LiteralPath $ChromeRoot -PathType Container)) {
        $message = "缺少可打包的 Windows 浏览器内核目录: $ChromeRoot"
        if ($RequireChrome) {
            throw $message
        }
        Write-Host "[WARN] $message，Windows 安装包将不包含浏览器内核"
        return $false
    }

    $stagingChromeDir = Join-Path $StagingDir "chrome"
    $chromeReadme = Join-Path $ChromeRoot "README.md"
    $copiedCores = @()
    $rootExecutable = Join-Path $ChromeRoot "chrome.exe"

    if (Test-PeExecutable -FilePath $rootExecutable) {
        Copy-DirectoryContents -SourceDir $ChromeRoot -DestinationDir $stagingChromeDir
        $copiedCores += "chrome"
    }
    else {
        if (Test-Path -LiteralPath $chromeReadme -PathType Leaf) {
            New-Item -ItemType Directory -Path $stagingChromeDir -Force | Out-Null
            Copy-Item -LiteralPath $chromeReadme -Destination (Join-Path $stagingChromeDir "README.md") -Force
        }

        foreach ($entry in (Get-ChildItem -LiteralPath $ChromeRoot -Force)) {
            if (-not $entry.PSIsContainer) {
                continue
            }

            $candidateExe = Join-Path $entry.FullName "chrome.exe"
            if (-not (Test-PeExecutable -FilePath $candidateExe)) {
                continue
            }

            New-Item -ItemType Directory -Path $stagingChromeDir -Force | Out-Null
            Copy-Item -LiteralPath $entry.FullName -Destination (Join-Path $stagingChromeDir $entry.Name) -Recurse -Force
            $copiedCores += $entry.Name
        }
    }

    if ($copiedCores.Count -gt 0) {
        Write-Host ("✓ 自动打包 Windows 内核: {0}" -f ($copiedCores -join ", "))
        return $true
    }

    $message = "缺少可打包的 Windows 浏览器内核: $ChromeRoot"
    if ($RequireChrome) {
        throw $message
    }

    if (Test-Path -LiteralPath $chromeReadme -PathType Leaf) {
        Write-Host "✓ 保留 chrome\README.md（未发现可打包的 Windows 内核）"
    }
    else {
        Write-Host "[WARN] 未发现可打包的 Windows 内核，且缺少 chrome\README.md"
    }
    return $false
}

function New-WindowsStaging {
    Write-Host "[Windows] 组装 staging 目录..."

    $stagingDir = Join-Path $repoRoot "publish/staging"
    $releaseConfig = Join-Path $repoRoot "publish/config.init.yaml"
    $binaryPath = Join-Path $repoRoot "build/bin/ant-chrome.exe"
    $binDir = Join-Path $repoRoot "bin"
    $chromeRoot = Resolve-WindowsChromeRoot
    $requireChrome = Resolve-WindowsChromeRequirement
    $workspaceAgentRoot = Resolve-WindowsWorkspaceAgentRoot
    $workspaceNodeRoot = Resolve-WindowsWorkspaceNodeRoot

    if (Test-Path -LiteralPath $stagingDir) {
        Remove-Item -LiteralPath $stagingDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $stagingDir -Force | Out-Null

    Copy-Item -LiteralPath $binaryPath -Destination (Join-Path $stagingDir "ant-chrome.exe") -Force
    if (-not (Test-Path -LiteralPath (Join-Path $stagingDir "ant-chrome.exe") -PathType Leaf)) {
        throw "staging 中缺少 ant-chrome.exe"
    }
    Write-Host "✓ 复制 ant-chrome.exe"

    if (-not (Test-Path -LiteralPath $releaseConfig -PathType Leaf)) {
        throw "未找到发布配置模板: publish\config.init.yaml"
    }
    Copy-Item -LiteralPath $releaseConfig -Destination (Join-Path $stagingDir "config.yaml") -Force
    Write-Host "✓ 复制发布配置模板 publish\config.init.yaml -> config.yaml"

    $runtimeManifestSource = Join-Path $repoRoot "publish/runtime-manifest.json"
    if (-not (Test-Path -LiteralPath $runtimeManifestSource -PathType Leaf)) {
        throw "缺少运行时清单: publish\runtime-manifest.json"
    }
    $stagingPublishDir = Join-Path $stagingDir "publish"
    New-Item -ItemType Directory -Path $stagingPublishDir -Force | Out-Null
    Copy-Item -LiteralPath $runtimeManifestSource -Destination (Join-Path $stagingPublishDir "runtime-manifest.json") -Force
    Write-Host "✓ 复制运行时清单 publish\runtime-manifest.json"

    $stagingBinDir = Join-Path $stagingDir "bin"
    New-Item -ItemType Directory -Path $stagingBinDir -Force | Out-Null

    foreach ($required in @("xray.exe", "sing-box.exe")) {
        $source = Join-Path $binDir $required
        if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
            throw "缺少运行时文件: bin\$required"
        }
        Copy-Item -LiteralPath $source -Destination (Join-Path $stagingBinDir $required) -Force
    }
    Write-Host "✓ 复制 bin\（xray.exe, sing-box.exe）"

    Copy-RequiredDirectoryContents `
        -SourceDir $workspaceAgentRoot `
        -DestinationDir (Join-Path $stagingDir "apps\agent") `
        -RequiredFile "src\server\index.mjs" `
        -DisplayName "workspace agent 运行时"

    Copy-WindowsWorkspaceNodeRuntime `
        -SourceDir $workspaceNodeRoot `
        -DestinationDir (Join-Path $stagingDir "runtime\node")

    Copy-WindowsChromePayload -ChromeRoot $chromeRoot -StagingDir $stagingDir -RequireChrome $requireChrome | Out-Null

    New-Item -ItemType Directory -Path (Join-Path $stagingDir "data") -Force | Out-Null
    Write-Host "✓ 创建空 data 目录（不打包 app.db，首次启动自动初始化）"
    Write-Host ""
    Write-Host "✓ staging 目录组装完成"
    Write-Host ""

    return $stagingDir
}

function Invoke-WindowsPackaging {
    param(
        [Parameter(Mandatory = $true)]
        [string]$MakensisPath,
        [Parameter(Mandatory = $true)]
        [string]$StagingDir
    )

    Write-Host "[Windows] 调用 NSIS 打包..."
    $outputDir = Join-Path $repoRoot "publish/output"
    if (-not (Test-Path -LiteralPath $outputDir)) {
        New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    }

    $installerPath = Join-Path $repoRoot "publish/installer.nsi"
    $installerContent = [System.IO.File]::ReadAllText($installerPath, [System.Text.Encoding]::UTF8)
    [System.IO.File]::WriteAllText($installerPath, $installerContent, [System.Text.UTF8Encoding]::new($true))

    $stagingAbs = (Resolve-Path $StagingDir).Path
    $compressionMode = if ($env:ANT_BROWSER_PUBLISH_BEST_COMPRESSION -eq "1") { "best" } else { "fast" }
    $useNsisConfig = ($env:ANT_BROWSER_NSIS_USE_CONFIG -eq "1")
    Write-Host "  压缩模式: $compressionMode"
    if ($useNsisConfig) {
        Write-Host "  NSIS 全局配置: enabled"
    }
    else {
        Write-Host "  NSIS 全局配置: disabled (/NOCONFIG)"
    }
    $nsisArguments = @(
        "/DVERSION=$script:Version",
        "/DSTAGINGDIR=$stagingAbs",
        "publish\installer.nsi"
    )
    if (-not $useNsisConfig) {
        $nsisArguments = @("/NOCONFIG") + $nsisArguments
    }
    if ($compressionMode -eq "best") {
        $nsisArguments = @("/DBESTCOMPRESSION") + $nsisArguments
    }
    Invoke-NativeCommand -FilePath $MakensisPath -Arguments $nsisArguments

    Write-Host "✓ Windows 安装包生成成功"
    Write-Host ""
}

function New-WindowsAppUpdateArtifacts {
    param(
        [Parameter(Mandatory = $true)]
        [string]$StagingDir
    )

    Write-Host "[Windows] 生成 app-update 全量包..."

    $outputDir = Join-Path $repoRoot "publish/output"
    New-Item -ItemType Directory -Path $outputDir -Force | Out-Null

    $zipName = "MakaBrowser-$script:Version-windows-amd64.zip"
    $zipPath = Join-Path $outputDir $zipName
    $zipShaPath = "$zipPath.sha256"
    $manifestPath = Join-Path $outputDir "app-update-stable.json"
    $manifestShaPath = "$manifestPath.sha256"
    $payloadRoot = Join-Path $repoRoot "publish/update-payload"

    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $zipShaPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $manifestPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $manifestShaPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $payloadRoot -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $payloadRoot -Force | Out-Null

    try {
        foreach ($entry in (Get-ChildItem -LiteralPath $StagingDir -Force)) {
            if ($entry.Name -ieq "data") {
                continue
            }
            Copy-Item -LiteralPath $entry.FullName -Destination (Join-Path $payloadRoot $entry.Name) -Recurse -Force
        }

        $runtimeManifestSource = Join-Path $repoRoot "publish/runtime-manifest.json"
        if (-not (Test-Path -LiteralPath $runtimeManifestSource -PathType Leaf)) {
            throw "缺少运行时清单: publish\runtime-manifest.json"
        }
        $payloadPublishDir = Join-Path $payloadRoot "publish"
        New-Item -ItemType Directory -Path $payloadPublishDir -Force | Out-Null
        Copy-Item -LiteralPath $runtimeManifestSource -Destination (Join-Path $payloadPublishDir "runtime-manifest.json") -Force

        Compress-Archive -Path (Join-Path $payloadRoot "*") -DestinationPath $zipPath -Force

        $zipItem = Get-Item -LiteralPath $zipPath
        $zipSha = Get-FileSha256 -Path $zipPath
        "$zipSha  $zipName" | Set-Content -LiteralPath $zipShaPath -Encoding ascii

        $manifest = [ordered]@{
            schemaVersion = 1
            product = "Maka Browser"
            channel = "stable"
            version = $script:Version
            generatedAt = (Get-Date).ToUniversalTime().ToString("o")
            packages = @(
                [ordered]@{
                    target = "windows-amd64"
                    version = $script:Version
                    payloadType = "full"
                    url = $zipName
                    sha256 = $zipSha
                    size = $zipItem.Length
                }
            )
        }
        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 10), $utf8NoBom)
        $manifestSha = Get-FileSha256 -Path $manifestPath
        "$manifestSha  app-update-stable.json" | Set-Content -LiteralPath $manifestShaPath -Encoding ascii

        Write-Host "✓ 更新包生成成功: publish\output\$zipName"
        Write-Host "✓ 更新清单生成成功: publish\output\app-update-stable.json"
        Write-Host ""
    }
    finally {
        Remove-Item -LiteralPath $payloadRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Remove-WindowsStaging {
    param([string]$StagingDir)

    if ($StagingDir -and (Test-Path -LiteralPath $StagingDir)) {
        Write-Host "[Windows] 清理临时文件..."
        Remove-Item -LiteralPath $StagingDir -Recurse -Force
        Write-Host "✓ staging 目录已清理"
        Write-Host ""
    }
}

function Publish-Windows {
    Write-Host "[3/3] 开始 Windows 打包..."
    Write-Host ""

    $makensisPath = Resolve-NsisPath
    Assert-RuntimeHashes -Target "windows-amd64"
    Build-WindowsBinary

    $stagingDir = $null
    try {
        $stagingDir = New-WindowsStaging
        Invoke-WindowsPackaging -MakensisPath $makensisPath -StagingDir $stagingDir
        New-WindowsAppUpdateArtifacts -StagingDir $stagingDir
    }
    finally {
        Remove-WindowsStaging -StagingDir $stagingDir
    }

    $script:WindowsDone = $true
    Write-Host "✓ Windows 打包完成"
    Write-Host ""
}

function Publish-Linux {
    Write-Host "[3/3] 开始 Linux 打包..."
    Write-Host ""

    $linuxScript = Join-Path $repoRoot "publish/linux/publish-linux-docker.ps1"
    $archOutFile = Join-Path $env:TEMP ("ant-browser-linux-arch-" + [guid]::NewGuid().ToString("N") + ".txt")
    if (Test-Path -LiteralPath $archOutFile) {
        Remove-Item -LiteralPath $archOutFile -Force
    }

    try {
        & powershell -NoProfile -ExecutionPolicy Bypass -File $linuxScript -RepoRoot $repoRoot -ArchOutFile $archOutFile -Version $script:Version
        if ($LASTEXITCODE -ne 0) {
            throw "Linux 打包失败"
        }

        if (Test-Path -LiteralPath $archOutFile -PathType Leaf) {
            $script:LinuxArch = (Get-Content -LiteralPath $archOutFile -Raw).Trim()
        }
    }
    finally {
        Remove-Item -LiteralPath $archOutFile -Force -ErrorAction SilentlyContinue
    }

    $script:LinuxDone = $true
    Write-Host "✓ Linux 打包完成"
    Write-Host ""
}

try {
    Write-Section "Maka Browser - 发布打包脚本"
    Write-Host ""
    Write-Host "当前工作目录: $repoRoot"
    Write-Host ""

    Resolve-Version -ExplicitVersion $ReleaseVersion
    $publishTarget = Resolve-PublishTarget -InputTarget $Target

    Invoke-WithTemporaryWailsVersion {
        switch ($publishTarget) {
            "WINDOWS" {
                Publish-Windows
            }
            "LINUX" {
                Publish-Linux
            }
            "BOTH" {
                Publish-Windows
                Publish-Linux
            }
            default {
                throw "不支持的打包目标: $publishTarget"
            }
        }
    }

    Write-Host ""
    Write-Section "✓ 发布完成！"
    Write-Host ""
    if ($script:WindowsDone) {
        Write-Host "Windows 安装包: publish\output\MakaBrowser-Setup-$script:Version.exe"
    }
    if ($script:LinuxDone) {
        Write-Host "Linux 产物目录: publish\output\"
        if ($script:LinuxArch -ne "") {
            Write-Host "Linux 架构: $script:LinuxArch"
        }
    }
    Write-Host ""
    Write-Host "提示：用户安装后可将旧的 data\ 目录粘贴到安装目录覆盖初始数据"
    exit 0
}
catch {
    Write-Host ""
    Write-Section "✗ 发布失败"
    Write-Host ""
    Write-Host $_.Exception.Message
    exit 1
}
