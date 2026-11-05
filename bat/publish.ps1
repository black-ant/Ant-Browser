param(
    [string]$Target,
    [string]$Version,
    [ValidateSet("INSTALLER", "PORTABLE", "BOTH")]
    [string]$WindowsFormat
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$script:ResolvedVersion = ""
$script:LinuxArch = ""
$script:WindowsInstallerDone = $false
$script:WindowsPortableDone = $false
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
        $script:ResolvedVersion = Assert-VersionValue -Value $explicit -Source "传入版本号"
        Write-Host "[1/3] 使用传入版本号..."
        Write-Host "✓ 版本号: $script:ResolvedVersion"
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

    $script:ResolvedVersion = Assert-VersionValue -Value $resolvedVersion -Source "wails.json productVersion"
    Write-Host "✓ 版本号: $script:ResolvedVersion"
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
    if ($currentVersion -eq $script:ResolvedVersion) {
        & $ScriptBlock
        return
    }

    Write-Host "  临时覆盖 wails.json productVersion: $currentVersion -> $script:ResolvedVersion"
    $originalBytes = [System.IO.File]::ReadAllBytes($wailsConfigPath)
    try {
        $currentConfig.info.productVersion = $script:ResolvedVersion
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

function Resolve-WindowsFormat {
    param(
        [string]$InputFormat,
        [string]$PublishTarget,
        [bool]$Interactive
    )

    $normalized = (Get-TrimmedText $InputFormat).ToUpperInvariant()
    if ($normalized -ne "") {
        if ($normalized -notin @("INSTALLER", "PORTABLE", "BOTH")) {
            throw "无效的 Windows 输出格式: $InputFormat`n  支持参数: INSTALLER/PORTABLE/BOTH"
        }
        return $normalized
    }

    if ($PublishTarget -notin @("WINDOWS", "BOTH") -or -not $Interactive) {
        return "INSTALLER"
    }

    $formatMapping = @{
        "I"         = "INSTALLER"
        "INSTALLER" = "INSTALLER"
        "Z"         = "PORTABLE"
        "ZIP"       = "PORTABLE"
        "P"         = "PORTABLE"
        "PORTABLE"  = "PORTABLE"
        "B"         = "BOTH"
        "BOTH"      = "BOTH"
    }

    Write-Host "[2/3] 选择 Windows 输出格式..."
    Write-Host ""
    Write-Host "  [I] 安装包（默认）"
    Write-Host "  [Z] 便携 ZIP"
    Write-Host "  [B] 安装包 + 便携 ZIP"
    Write-Host ""

    while ($true) {
        $choice = (Read-Host "请选择 Windows 输出格式 [I/Z/B]").Trim().ToUpperInvariant()
        if ($choice -eq "") {
            $choice = "I"
        }
        if ($formatMapping.ContainsKey($choice)) {
            $resolvedFormat = $formatMapping[$choice]
            Write-Host "✓ 已选择: $resolvedFormat"
            Write-Host ""
            return $resolvedFormat
        }
        Write-Host "✗ 未选择有效输出格式" -ForegroundColor Yellow
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
        Invoke-NativeCommand -FilePath "wails" -Arguments @("build", "-tags", "native_webview2loader")
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

function Copy-WindowsChromePayload {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ChromeRoot,
        [Parameter(Mandatory = $true)]
        [string]$StagingDir
    )

    if (-not (Test-Path -LiteralPath $ChromeRoot -PathType Container)) {
        Write-Host "[WARN] 缺少 chrome\ 目录，Windows 安装包将不包含浏览器内核"
        return
    }

    $stagingChromeDir = Join-Path $StagingDir "chrome"
    $chromeReadme = Join-Path $ChromeRoot "README.md"
    $copiedCores = @()
    $rootExecutable = Join-Path $ChromeRoot "chrome.exe"

    if (Test-PeExecutable -FilePath $rootExecutable) {
        Copy-DirectoryContents -SourceDir $ChromeRoot -DestinationDir $stagingChromeDir
        $copiedCores += "chrome\"
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
        return
    }

    if (Test-Path -LiteralPath $chromeReadme -PathType Leaf) {
        Write-Host "✓ 保留 chrome\README.md（未发现可打包的 Windows 内核）"
    }
    else {
        Write-Host "[WARN] 未发现可打包的 Windows 内核，且缺少 chrome\README.md"
    }
}

function Assert-ReleaseConfigPrivacy {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ConfigPath
    )

    if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf)) {
        throw "发布配置不存在: $ConfigPath"
    }

    $configText = [System.IO.File]::ReadAllText($ConfigPath, [System.Text.Encoding]::UTF8)
    $hasS3Section = $configText -match '(?im)^\s*s3\s*:'
    $hasS3Credential = $configText -match '(?im)^\s*(access_key_id|secret_access_key|session_token)\s*:'
    $hasS3Setting = $configText -match '(?im)^\s*(endpoint|bucket|prefix|force_path_style)\s*:'
    if ($hasS3Section -or $hasS3Credential -or $hasS3Setting) {
        throw "发布配置包含 S3 配置或凭据，禁止进入 Windows 产物: $ConfigPath"
    }
}

function Test-LocalBackupArtifact {
    param(
        [Parameter(Mandatory = $true)]
        [System.IO.FileInfo]$File
    )

    return $File.Name -match '(?i)(?:\.zip|\.jar|\.7z|\.rar|\.tar|\.tar\.gz|\.tgz|\.tar\.xz|\.txz|\.tar\.bz2|\.tbz2|\.bak|\.backup|\.old|\.orig)$'
}

function Assert-WindowsStagingPrivacy {
    param(
        [Parameter(Mandatory = $true)]
        [string]$StagingDir
    )

    if (-not (Test-Path -LiteralPath $StagingDir -PathType Container)) {
        throw "Windows staging 目录不存在: $StagingDir"
    }

    $files = @(Get-ChildItem -LiteralPath $StagingDir -File -Recurse -Force)
    foreach ($file in $files) {
        $relativePath = $file.FullName.Substring($StagingDir.Length).TrimStart('\', '/')
        if ($file.Name -ieq 'backup.local.yaml' -or
            $file.Name -match '(?i)\.local\.(?:yaml|yml|json)$' -or
            (Test-LocalBackupArtifact -File $file) -or
            $relativePath -match '(?i)(^|[\\/])(?:\.local-backup|backup|backups)(?:[\\/]|$)') {
            throw "Windows staging 包含本地配置或备份文件，已阻止打包: $relativePath"
        }
    }

    Assert-ReleaseConfigPrivacy -ConfigPath (Join-Path $StagingDir 'config.yaml')
}

function New-WindowsStaging {
    Write-Host "[Windows] 组装 staging 目录..."

    $stagingDir = Join-Path $repoRoot "publish/staging"
    $releaseConfig = Join-Path $repoRoot "publish/config.init.yaml"
    $binaryPath = Join-Path $repoRoot "build/bin/ant-chrome.exe"
    $binDir = Join-Path $repoRoot "bin"
    $chromeRoot = Join-Path $repoRoot "chrome"

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
    Assert-ReleaseConfigPrivacy -ConfigPath $releaseConfig
    Copy-Item -LiteralPath $releaseConfig -Destination (Join-Path $stagingDir "config.yaml") -Force
    Write-Host "✓ 复制发布配置模板 publish\config.init.yaml -> config.yaml"

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

    Copy-WindowsChromePayload -ChromeRoot $chromeRoot -StagingDir $stagingDir

    $stagingDiagnosticsDir = Join-Path $stagingDir 'data\diagnostics'
    New-Item -ItemType Directory -Path $stagingDiagnosticsDir -Force | Out-Null
    foreach ($diagnosticsScript in @(
        'ant-chrome-diagnostics-watcher.ps1',
        'install-ant-chrome-diagnostics-watcher.ps1'
    )) {
        $diagnosticsSource = Join-Path $repoRoot (Join-Path 'tools' $diagnosticsScript)
        if (-not (Test-Path -LiteralPath $diagnosticsSource -PathType Leaf)) {
            throw ('Missing diagnostics script: ' + $diagnosticsScript)
        }
        Copy-Item -LiteralPath $diagnosticsSource -Destination (Join-Path $stagingDiagnosticsDir $diagnosticsScript) -Force
    }
    Write-Host '[Diagnostics] copied read-only watcher scripts to data\diagnostics'

    New-Item -ItemType Directory -Path (Join-Path $stagingDir "data") -Force | Out-Null
    Write-Host "✓ 创建空 data 目录（不打包 app.db，首次启动自动初始化）"
    Assert-WindowsStagingPrivacy -StagingDir $stagingDir
    Write-Host "✓ staging 隐私检查通过（不含 S3 配置、凭据和备份文件）"
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
        "/DVERSION=$script:ResolvedVersion",
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

function New-WindowsPortableArchive {
    param(
        [Parameter(Mandatory = $true)]
        [string]$StagingDir
    )

    Write-Host "[Windows] 生成便携 ZIP..."
    $outputDir = Join-Path $repoRoot "publish/output"
    if (-not (Test-Path -LiteralPath $outputDir)) {
        New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    }

    $archiveName = "AntBrowser-$script:ResolvedVersion-windows-amd64-portable.zip"
    $archivePath = Join-Path $outputDir $archiveName
    $rootName = "AntBrowser-$script:ResolvedVersion-windows-amd64-portable"
    if (Test-Path -LiteralPath $archivePath) {
        Remove-Item -LiteralPath $archivePath -Force
    }

    Add-Type -AssemblyName System.IO.Compression
    $archiveStream = [System.IO.File]::Open(
        $archivePath,
        [System.IO.FileMode]::CreateNew,
        [System.IO.FileAccess]::Write,
        [System.IO.FileShare]::None
    )
    try {
        $archive = New-Object System.IO.Compression.ZipArchive(
            $archiveStream,
            [System.IO.Compression.ZipArchiveMode]::Create,
            $false
        )
        try {
            $archive.CreateEntry("$rootName/") | Out-Null

            foreach ($directory in (Get-ChildItem -LiteralPath $StagingDir -Directory -Recurse -Force)) {
                $relativePath = $directory.FullName.Substring($StagingDir.Length).TrimStart('\', '/')
                $entryName = "$rootName/$($relativePath.Replace('\', '/'))/"
                $archive.CreateEntry($entryName) | Out-Null
            }

            foreach ($file in (Get-ChildItem -LiteralPath $StagingDir -File -Recurse -Force)) {
                $relativePath = $file.FullName.Substring($StagingDir.Length).TrimStart('\', '/')
                $entryName = "$rootName/$($relativePath.Replace('\', '/'))"
                $entry = $archive.CreateEntry(
                    $entryName,
                    [System.IO.Compression.CompressionLevel]::Optimal
                )
                $inputStream = [System.IO.File]::OpenRead($file.FullName)
                $outputStream = $entry.Open()
                try {
                    $inputStream.CopyTo($outputStream)
                }
                finally {
                    $outputStream.Dispose()
                    $inputStream.Dispose()
                }
            }
        }
        finally {
            $archive.Dispose()
        }
    }
    finally {
        $archiveStream.Dispose()
    }

    if (-not (Test-Path -LiteralPath $archivePath -PathType Leaf)) {
        throw "便携 ZIP 生成失败: $archivePath"
    }

    $script:WindowsPortableDone = $true
    Write-Host "✓ Windows 便携包生成成功: publish\output\$archiveName"
    Write-Host ""
    return $archivePath
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
    param(
        [Parameter(Mandatory = $true)]
        [string]$Format
    )

    Write-Host "[3/3] 开始 Windows 打包..."
    Write-Host "  输出格式: $Format"
    Write-Host ""

    $makensisPath = $null
    if ($Format -in @("INSTALLER", "BOTH")) {
        $makensisPath = Resolve-NsisPath
    }
    Assert-RuntimeHashes -Target "windows-amd64"
    Build-WindowsBinary

    $stagingDir = $null
    try {
        $stagingDir = New-WindowsStaging
        if ($Format -in @("INSTALLER", "BOTH")) {
            Invoke-WindowsPackaging -MakensisPath $makensisPath -StagingDir $stagingDir
            $script:WindowsInstallerDone = $true
        }
        if ($Format -in @("PORTABLE", "BOTH")) {
            New-WindowsPortableArchive -StagingDir $stagingDir | Out-Null
        }
    }
    finally {
        Remove-WindowsStaging -StagingDir $stagingDir
    }

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
        & powershell -NoProfile -ExecutionPolicy Bypass -File $linuxScript -RepoRoot $repoRoot -ArchOutFile $archOutFile -Version $script:ResolvedVersion
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
    Write-Section "Ant Browser - 发布打包脚本"
    Write-Host ""
    Write-Host "当前工作目录: $repoRoot"
    Write-Host ""

    Resolve-Version -ExplicitVersion $Version
    $targetWasProvided = (Get-TrimmedText $Target) -ne ""
    $publishTarget = Resolve-PublishTarget -InputTarget $Target
    $resolvedWindowsFormat = Resolve-WindowsFormat -InputFormat $WindowsFormat -PublishTarget $publishTarget -Interactive (-not $targetWasProvided)

    Invoke-WithTemporaryWailsVersion {
        switch ($publishTarget) {
            "WINDOWS" {
                Publish-Windows -Format $resolvedWindowsFormat
            }
            "LINUX" {
                Publish-Linux
            }
            "BOTH" {
                Publish-Windows -Format $resolvedWindowsFormat
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
    if ($script:WindowsInstallerDone) {
        Write-Host "Windows 安装包: publish\output\AntBrowser-Setup-$script:ResolvedVersion.exe"
    }
    if ($script:WindowsPortableDone) {
        Write-Host "Windows 便携包: publish\output\AntBrowser-$script:ResolvedVersion-windows-amd64-portable.zip"
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
