<#
.SYNOPSIS
Publish a sanitized snapshot to the public repository.

.DESCRIPTION
- Default mode: publish one single-commit sanitized snapshot to the public master branch.
- Optional release branch and tag can be created for the same published commit.
- Private development history is never pushed directly.

.EXAMPLE
.\tools\public-release\publish-public.ps1

.EXAMPLE
.\tools\public-release\publish-public.ps1 -DryRun -SkipRelease -SkipTag

.EXAMPLE
.\tools\public-release\publish-public.ps1 -PublicRemote github -PublishRelease -PublishTag

.EXAMPLE
.\tools\public-release\publish-public.ps1 -CommitMessage "release: public snapshot 1.2.3"

.EXAMPLE
.\tools\public-release\publish-public.ps1 -Version 1.1.0 -PublishRelease -PublishTag

.EXAMPLE
.\tools\public-release\publish-public.ps1 -CommitMessageFile .\publish-message.txt
#>
param(
    [string]$Version,
    [string]$PublicRemote,
    [string]$SourceRef = "master",
    [string]$TargetBranch = "master",
    [string]$ReleasePrefix = "release/",
    [string]$TagPrefix = "v",
    [string[]]$ExcludePaths = @(
        "docs",
        "DEPLOYMENT_GUIDE.md",
        "plan.md",
        "pic",
        "data",
        "snapshots",
        "chrome",
        "build/bin",
        ".vscode",
        ".history",
        ".kiro",
        ".ant-license.json"
    ),

    [switch]$PublishRelease,
    [switch]$SkipRelease,
    [switch]$PublishTag,
    [switch]$SkipTag,
    [switch]$AllowDirtyWorkingTree,
    [switch]$SkipRuntimeCheck,
    [switch]$DryRun,
    [switch]$KeepTempDir,
    [switch]$NonInteractive,
    [string]$CommitterName,
    [string]$CommitterEmail,
    [switch]$IncludeSourceCommit,
    [string]$CommitMessage,
    [string]$CommitMessageFile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$defaultCommitterName = "ZwBrowser Release Bot"
$defaultCommitterEmail = "release-bot@ant-browser.local"

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
        [string]$Source = "publish version"
    )

    $trimmed = Get-TrimmedText $Value
    if ($trimmed -eq "") {
        throw "$Source cannot be empty."
    }
    if ($trimmed -notmatch '^\d+\.\d+\.\d+(?:-[0-9A-Za-z\.-]+)?(?:\+[0-9A-Za-z\.-]+)?$') {
        throw "$Source format is invalid: $trimmed. Expected examples: 1.1.0 or 1.1.0-beta.1"
    }
    return $trimmed
}

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Invoke-Git {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Args,
        [switch]$AllowFailure
    )

    $tempToken = [guid]::NewGuid().ToString("N")
    $stdoutPath = Join-Path $env:TEMP "git-stdout-$tempToken.log"
    $stderrPath = Join-Path $env:TEMP "git-stderr-$tempToken.log"
    $previousErrorActionPreference = $ErrorActionPreference

    try {
        $ErrorActionPreference = "Continue"
        & git @Args 1> $stdoutPath 2> $stderrPath
        $code = $LASTEXITCODE
        $output = @()
        if (Test-Path -LiteralPath $stdoutPath) {
            $output += Get-Content -LiteralPath $stdoutPath
        }
        if (Test-Path -LiteralPath $stderrPath) {
            $output += Get-Content -LiteralPath $stderrPath
        }
    }
    finally {
        $ErrorActionPreference = $previousErrorActionPreference
        Remove-Item -LiteralPath $stdoutPath -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
    }

    if ($code -ne 0 -and -not $AllowFailure) {
        $argText = $Args -join " "
        $outText = $output -join [Environment]::NewLine
        throw "git $argText failed with exit code $code.`n$outText"
    }
    return @{
        Code   = $code
        Output = $output
    }
}

function Get-FirstOutputLine {
    param([string[]]$Lines)

    if (($Lines | Measure-Object).Count -eq 0) {
        return ""
    }
    return $Lines[0].Trim()
}

function Resolve-PublicRemoteUrl {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RemoteOrUrl
    )

    $knownRemotes = (Invoke-Git -Args @("remote")).Output
    foreach ($item in $knownRemotes) {
        if ($item.Trim() -eq $RemoteOrUrl) {
            return Get-FirstOutputLine -Lines (Invoke-Git -Args @("remote", "get-url", $RemoteOrUrl)).Output
        }
    }
    return $RemoteOrUrl
}

function Resolve-DefaultPublicRemote {
    $knownRemotes = @((Invoke-Git -Args @("remote")).Output | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" })
    foreach ($candidate in @("github", "public")) {
        if ($knownRemotes -contains $candidate) {
            return $candidate
        }
    }
    throw "PublicRemote was not provided and no default public remote (github/public) was found. Pass -PublicRemote explicitly."
}

function Resolve-VersionValue {
    param([string]$ExplicitVersion)

    $explicit = Get-TrimmedText $ExplicitVersion
    if ($explicit -ne "") {
        return Assert-VersionValue -Value $explicit -Source "publish version"
    }

    $wailsConfigPath = Join-Path $repoRoot "wails.json"
    if (-not (Test-Path -LiteralPath $wailsConfigPath)) {
        throw "wails.json was not found and -Version was not provided."
    }

    $wailsConfig = Get-Content -LiteralPath $wailsConfigPath -Raw | ConvertFrom-Json
    $resolvedVersion = Get-TrimmedText ([string]$wailsConfig.info.productVersion)
    if ($resolvedVersion -eq "") {
        throw "Could not resolve productVersion from wails.json. Pass -Version explicitly."
    }
    return Assert-VersionValue -Value $resolvedVersion -Source "wails.json productVersion"
}

function Resolve-CommitterIdentity {
    param(
        [string]$ExplicitName,
        [string]$ExplicitEmail
    )

    $resolvedName = Get-TrimmedText $ExplicitName
    if ($resolvedName -eq "") {
        $resolvedName = Get-TrimmedText ([string]$env:PUBLISH_COMMITTER_NAME)
    }
    if ($resolvedName -eq "") {
        $resolvedName = $defaultCommitterName
    }

    $resolvedEmail = Get-TrimmedText $ExplicitEmail
    if ($resolvedEmail -eq "") {
        $resolvedEmail = Get-TrimmedText ([string]$env:PUBLISH_COMMITTER_EMAIL)
    }
    if ($resolvedEmail -eq "") {
        $resolvedEmail = $defaultCommitterEmail
    }
    if ($resolvedEmail -notmatch "^[^@\s]+@[^@\s]+$") {
        throw "Invalid publish committer email: $resolvedEmail"
    }

    return @{
        Name  = $resolvedName
        Email = $resolvedEmail
    }
}

function Get-TrackedStatusLines {
    param([string[]]$StatusArgs)

    return @((Invoke-Git -Args $StatusArgs).Output |
        ForEach-Object { [string]$_ } |
        Where-Object { $_ -match '^[ MARCUD?!][ MARCUD?!] ' } |
        Where-Object { $_ -notmatch '^\?\?' })
}

function Assert-CleanTrackedWorkingTree {
    $unstaged = Invoke-Git -Args @("diff", "--quiet", "--ignore-submodules", "--") -AllowFailure
    if ($unstaged.Code -gt 1) {
        throw "Unable to inspect unstaged tracked-file changes."
    }
    if ($unstaged.Code -eq 1) {
        $dirtyLines = Get-TrackedStatusLines -StatusArgs @("status", "--short")
        $details = $dirtyLines -join [Environment]::NewLine
        throw "Tracked files have unstaged changes. This publish script exports the committed snapshot from $SourceRef, not your local edits.`nCommit/stash these tracked changes first, or use -AllowDirtyWorkingTree if you intentionally want to publish the current committed snapshot only.`n$details"
    }

    $staged = Invoke-Git -Args @("diff", "--cached", "--quiet", "--ignore-submodules", "--") -AllowFailure
    if ($staged.Code -gt 1) {
        throw "Unable to inspect staged tracked-file changes."
    }
    if ($staged.Code -eq 1) {
        $dirtyLines = Get-TrackedStatusLines -StatusArgs @("status", "--short")
        $details = $dirtyLines -join [Environment]::NewLine
        throw "Tracked files have staged but uncommitted changes. This publish script exports the committed snapshot from $SourceRef, not your index/worktree state.`nCommit these tracked changes first, or use -AllowDirtyWorkingTree if you intentionally want to publish the current committed snapshot only.`n$details"
    }
}

function Get-RemoteRefOid {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RemoteUrl,
        [Parameter(Mandatory = $true)]
        [string]$RefName,
        [Parameter(Mandatory = $true)]
        [ValidateSet("heads", "tags")]
        [string]$RefType
    )

    $option = if ($RefType -eq "heads") { "--heads" } else { "--tags" }
    $refPath = if ($RefType -eq "heads") { "refs/heads/$RefName" } else { "refs/tags/$RefName" }
    $result = Invoke-Git -Args @("ls-remote", $option, $RemoteUrl, $refPath)
    if ($result.Code -ne 0) {
        throw "Unable to query remote ref $refPath from $RemoteUrl."
    }

    $firstLine = Get-FirstOutputLine -Lines $result.Output
    if ($firstLine -eq "") {
        return ""
    }

    $parts = $firstLine -split '\s+'
    if ($parts.Count -lt 1) {
        return ""
    }
    return $parts[0].Trim()
}

function Sync-SnapshotToRepo {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SnapshotDir,
        [Parameter(Mandatory = $true)]
        [string]$RepoDir
    )

    Get-ChildItem -LiteralPath $RepoDir -Force |
        Where-Object { $_.Name -ne ".git" } |
        Remove-Item -Recurse -Force

    Get-ChildItem -LiteralPath $SnapshotDir -Force | ForEach-Object {
        $target = Join-Path $RepoDir $_.Name
        Copy-Item -LiteralPath $_.FullName -Destination $target -Recurse -Force
    }
}

function Build-CommitMessage {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Title,
        [Parameter(Mandatory = $true)]
        [string]$Channel,
        [Parameter(Mandatory = $true)]
        [string]$VersionValue,
        [Parameter(Mandatory = $true)]
        [string]$SourceRefValue,
        [string]$SourceCommit,
        [switch]$AppendSourceCommit
    )

    $publishedAtUtc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    $lines = @(
        $Title,
        "",
        "channel: $Channel",
        "version: $VersionValue",
        "source-ref: $SourceRefValue",
        "published-at-utc: $publishedAtUtc"
    )
    if ($AppendSourceCommit -and ((Get-TrimmedText $SourceCommit) -ne "")) {
        $lines += "source-commit: $SourceCommit"
    }
    return $lines -join "`n"
}

function Resolve-CommitMessage {
    param(
        [string]$ExplicitMessage,
        [string]$ExplicitMessageFile,
        [Parameter(Mandatory = $true)]
        [scriptblock]$DefaultMessageFactory
    )

    $message = Get-TrimmedText $ExplicitMessage
    $messageFile = Get-TrimmedText $ExplicitMessageFile

    if ($message -ne "" -and $messageFile -ne "") {
        throw "Do not pass both -CommitMessage and -CommitMessageFile."
    }

    if ($messageFile -ne "") {
        $resolvedPath = $messageFile
        if (-not [System.IO.Path]::IsPathRooted($resolvedPath)) {
            $resolvedPath = Join-Path $repoRoot $resolvedPath
        }
        if (-not (Test-Path -LiteralPath $resolvedPath -PathType Leaf)) {
            throw "Commit message file was not found: $resolvedPath"
        }
        $message = (Get-Content -LiteralPath $resolvedPath -Raw)
        if ((Get-TrimmedText $message) -eq "") {
            throw "Commit message file is empty: $resolvedPath"
        }
        return $message
    }

    if ($message -ne "") {
        return $ExplicitMessage
    }

    return & $DefaultMessageFactory
}

function Invoke-GitCommit {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Message,
        [switch]$AllowEmpty
    )

    $messagePath = Join-Path $env:TEMP ("ant-chrome-commit-message-" + [guid]::NewGuid().ToString("N") + ".txt")
    try {
        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText($messagePath, $Message, $utf8NoBom)

        $commitArgs = @("commit")
        if ($AllowEmpty) {
            $commitArgs += "--allow-empty"
        }
        $commitArgs += @("-F", $messagePath)

        Invoke-Git -Args $commitArgs | Out-Null
    }
    finally {
        Remove-Item -LiteralPath $messagePath -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-GitAnnotatedTag {
    param(
        [Parameter(Mandatory = $true)]
        [string]$TagName,
        [Parameter(Mandatory = $true)]
        [string]$Commitish,
        [Parameter(Mandatory = $true)]
        [string]$Message
    )

    $messagePath = Join-Path $env:TEMP ("ant-chrome-tag-message-" + [guid]::NewGuid().ToString("N") + ".txt")
    try {
        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText($messagePath, $Message, $utf8NoBom)
        Invoke-Git -Args @("tag", "-a", "-f", $TagName, $Commitish, "-F", $messagePath) | Out-Null
    }
    finally {
        Remove-Item -LiteralPath $messagePath -Force -ErrorAction SilentlyContinue
    }
}

function Resolve-ExcludePaths {
    param([string[]]$Paths)

    return @($Paths | ForEach-Object { Get-TrimmedText $_ } | Where-Object { $_ -ne "" })
}

function Join-SnapshotRelativePath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RootDir,
        [Parameter(Mandatory = $true)]
        [string]$RelativePath
    )

    $resolvedPath = $RootDir
    foreach ($segment in ($RelativePath -split "[/\\]+" | Where-Object { $_ -ne "" })) {
        $resolvedPath = Join-Path $resolvedPath $segment
    }
    return $resolvedPath
}

function Remove-ExcludedPathsFromSnapshot {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SnapshotDir,
        [string[]]$Paths
    )

    foreach ($relativePath in (Resolve-ExcludePaths -Paths $Paths)) {
        $targetPath = Join-SnapshotRelativePath -RootDir $SnapshotDir -RelativePath $relativePath
        if (Test-Path -LiteralPath $targetPath) {
            Remove-Item -LiteralPath $targetPath -Recurse -Force
        }
    }
}

function Apply-PublicConfigTemplate {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SnapshotDir
    )

    $templatePath = Join-SnapshotRelativePath -RootDir $SnapshotDir -RelativePath "publish/config.init.yaml"
    if (-not (Test-Path -LiteralPath $templatePath)) {
        throw "Required public config template missing in source snapshot: publish/config.init.yaml"
    }

    $targetPath = Join-SnapshotRelativePath -RootDir $SnapshotDir -RelativePath "config.yaml"
    Copy-Item -LiteralPath $templatePath -Destination $targetPath -Force
}

function Confirm-YesNo {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Prompt,
        [bool]$Default = $false
    )

    $suffix = if ($Default) { "[Y/n]" } else { "[y/N]" }
    $response = Get-TrimmedText (Read-Host "$Prompt $suffix")
    if ($response -eq "") {
        return $Default
    }

    switch -Regex ($response.ToLowerInvariant()) {
        "^(y|yes)$" { return $true }
        "^(n|no)$" { return $false }
        default { throw "Unsupported answer: $response. Use y/yes or n/no." }
    }
}

function Read-NumberChoice {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Prompt,
        [Parameter(Mandatory = $true)]
        [string[]]$Options,
        [int]$Default = 1
    )

    if ($Options.Count -eq 0) {
        throw "Read-NumberChoice requires at least one option."
    }
    if ($Default -lt 1 -or $Default -gt $Options.Count) {
        throw "Default option index $Default is out of range."
    }

    while ($true) {
        Write-Host ""
        Write-Host $Prompt -ForegroundColor Yellow
        for ($i = 0; $i -lt $Options.Count; $i++) {
            $index = $i + 1
            $suffix = if ($index -eq $Default) { " (default)" } else { "" }
            Write-Host "  [$index] $($Options[$i])$suffix"
        }

        $response = Get-TrimmedText (Read-Host "Select 1-$($Options.Count)")
        if ($response -eq "") {
            return $Default
        }
        if ($response -match '^\d+$') {
            $value = [int]$response
            if ($value -ge 1 -and $value -le $Options.Count) {
                return $value
            }
        }
        Write-Host "Invalid choice. Enter 1-$($Options.Count)." -ForegroundColor Yellow
    }
}

function Resolve-InteractiveVersionValue {
    param(
        [Parameter(Mandatory = $true)]
        [string]$DefaultVersion
    )

    $validatedDefault = Assert-VersionValue -Value $DefaultVersion -Source "default version"
    while ($true) {
        Write-Host ""
        Write-Host "Publish version" -ForegroundColor Yellow
        $response = Get-TrimmedText (Read-Host "Version [$validatedDefault]")
        if ($response -eq "") {
            return $validatedDefault
        }

        try {
            return Assert-VersionValue -Value $response -Source "publish version"
        }
        catch {
            Write-Host $_.Exception.Message -ForegroundColor Yellow
        }
    }
}

function Resolve-InteractivePublishScope {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ReleaseBranch,
        [Parameter(Mandatory = $true)]
        [string]$TagName
    )

    $choice = Read-NumberChoice `
        -Prompt "Select publish scope" `
        -Options @(
            "Publish public master only",
            "Publish public master + ${ReleaseBranch}",
            "Publish public master + ${TagName}",
            "Publish public master + ${ReleaseBranch} + ${TagName}"
        ) `
        -Default 1

    switch ($choice) {
        1 { return @{ PublishRelease = $false; PublishTag = $false } }
        2 { return @{ PublishRelease = $true; PublishTag = $false } }
        3 { return @{ PublishRelease = $false; PublishTag = $true } }
        4 { return @{ PublishRelease = $true; PublishTag = $true } }
        default { throw "Unsupported publish scope choice: $choice" }
    }
}

function Resolve-OptionalPublishChoice {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,
        [switch]$Enable,
        [switch]$Disable,
        [switch]$NonInteractive,
        [Parameter(Mandatory = $true)]
        [string]$Prompt
    )

    if ($Enable -and $Disable) {
        throw "Conflicting options for $Name. Do not pass both enable and disable switches."
    }
    if ($Enable) {
        return $true
    }
    if ($Disable) {
        return $false
    }
    if ($NonInteractive) {
        return $false
    }
    return Confirm-YesNo -Prompt $Prompt -Default:$false
}

function Publish-BranchRef {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RemoteName,
        [Parameter(Mandatory = $true)]
        [string]$BranchName,
        [Parameter(Mandatory = $true)]
        [string]$Commitish,
        [string]$RemoteExpectedOid,
        [switch]$AllowRewrite,
        [switch]$DryRun
    )

    $refSpec = "${Commitish}:refs/heads/$BranchName"
    $leaseArg = ""
    if ($AllowRewrite -and ((Get-TrimmedText $RemoteExpectedOid) -ne "")) {
        $leaseArg = "--force-with-lease=refs/heads/${BranchName}:${RemoteExpectedOid}"
    }

    if ($DryRun) {
        if ($leaseArg -ne "") {
            Write-Host "DRY-RUN: skip push -> $RemoteName $refSpec $leaseArg"
        } else {
            Write-Host "DRY-RUN: skip push -> $RemoteName $refSpec"
        }
        return
    }

    if ($leaseArg -ne "") {
        Invoke-Git -Args @("push", $leaseArg, $RemoteName, $refSpec) | Out-Null
    } else {
        Invoke-Git -Args @("push", $RemoteName, $refSpec) | Out-Null
    }
}

function Publish-TagRef {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RemoteName,
        [Parameter(Mandatory = $true)]
        [string]$TagName,
        [Parameter(Mandatory = $true)]
        [string]$Commitish,
        [Parameter(Mandatory = $true)]
        [string]$VersionValue,
        [Parameter(Mandatory = $true)]
        [string]$SourceRefValue,
        [string]$SourceCommit,
        [string]$RemoteExpectedOid,
        [switch]$DryRun,
        [switch]$AppendSourceCommit
    )

    $message = Build-CommitMessage `
        -Title "tag: $TagName" `
        -Channel "tag" `
        -VersionValue $VersionValue `
        -SourceRefValue $SourceRefValue `
        -SourceCommit $SourceCommit `
        -AppendSourceCommit:$AppendSourceCommit

    Invoke-GitAnnotatedTag -TagName $TagName -Commitish $Commitish -Message $message
    $refSpec = "refs/tags/${TagName}:refs/tags/${TagName}"
    $leaseArg = ""
    if ((Get-TrimmedText $RemoteExpectedOid) -ne "") {
        $leaseArg = "--force-with-lease=refs/tags/${TagName}:${RemoteExpectedOid}"
    }

    if ($DryRun) {
        if ($leaseArg -ne "") {
            Write-Host "DRY-RUN: skip push -> $RemoteName $refSpec $leaseArg"
        } else {
            Write-Host "DRY-RUN: skip push -> $RemoteName $refSpec"
        }
        return
    }

    if ($leaseArg -ne "") {
        Invoke-Git -Args @("push", $leaseArg, $RemoteName, $refSpec) | Out-Null
    } else {
        Invoke-Git -Args @("push", $RemoteName, $refSpec) | Out-Null
    }
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $repoRoot

Write-Step "Validating repository state"
Invoke-Git -Args @("rev-parse", "--is-inside-work-tree") | Out-Null

if (-not $AllowDirtyWorkingTree) {
    Assert-CleanTrackedWorkingTree
}

$versionChoiceExplicit = $PSBoundParameters.ContainsKey("Version")
$Version = Resolve-VersionValue -ExplicitVersion $Version
if (-not $NonInteractive -and -not $versionChoiceExplicit) {
    Write-Step "Resolving publish version"
    $Version = Resolve-InteractiveVersionValue -DefaultVersion $Version
}
if ((Get-TrimmedText $PublicRemote) -eq "") {
    $PublicRemote = Resolve-DefaultPublicRemote
}
$committer = Resolve-CommitterIdentity -ExplicitName $CommitterName -ExplicitEmail $CommitterEmail
$resolvedExcludePaths = Resolve-ExcludePaths -Paths $ExcludePaths

$sourceCommit = Get-FirstOutputLine -Lines (Invoke-Git -Args @("rev-parse", "--verify", "$SourceRef`^{commit}")).Output
$sourceShort = Get-FirstOutputLine -Lines (Invoke-Git -Args @("rev-parse", "--short", $sourceCommit)).Output
$releaseBranch = "$ReleasePrefix$Version"
$tagName = "$TagPrefix$Version"
$publicUrl = Resolve-PublicRemoteUrl -RemoteOrUrl $PublicRemote

$releaseChoiceExplicit = $PSBoundParameters.ContainsKey("PublishRelease") -or $PSBoundParameters.ContainsKey("SkipRelease")
$tagChoiceExplicit = $PSBoundParameters.ContainsKey("PublishTag") -or $PSBoundParameters.ContainsKey("SkipTag")
$shouldPublishRelease = $false
$shouldPublishTag = $false

if (-not $NonInteractive -and -not $releaseChoiceExplicit -and -not $tagChoiceExplicit) {
    Write-Step "Collecting publish options"
    $scopeSelection = Resolve-InteractivePublishScope -ReleaseBranch $releaseBranch -TagName $tagName
    $shouldPublishRelease = [bool]$scopeSelection.PublishRelease
    $shouldPublishTag = [bool]$scopeSelection.PublishTag
} else {
    $shouldPublishRelease = Resolve-OptionalPublishChoice `
        -Name "release branch" `
        -Enable:$PublishRelease `
        -Disable:$SkipRelease `
        -NonInteractive:$NonInteractive `
        -Prompt "Publish release branch ${releaseBranch}?"

    $shouldPublishTag = Resolve-OptionalPublishChoice `
        -Name "tag" `
        -Enable:$PublishTag `
        -Disable:$SkipTag `
        -NonInteractive:$NonInteractive `
        -Prompt "Publish tag ${tagName}?"
}

$publishDryRun = [bool]$DryRun

Write-Host "Source ref: $SourceRef -> $sourceCommit"
Write-Host "Target branch: $TargetBranch"
Write-Host "Public remote: $publicUrl"
Write-Host "Release branch: $(if ($shouldPublishRelease) { $releaseBranch } else { 'skipped' })"
Write-Host "Tag: $(if ($shouldPublishTag) { $tagName } else { 'skipped' })"
Write-Host "Committer: $($committer.Name) <$($committer.Email)>"
Write-Host "Commit message: $(if ((Get-TrimmedText $CommitMessage) -ne "" -or (Get-TrimmedText $CommitMessageFile) -ne "") { 'custom' } else { 'default' })"
if ($resolvedExcludePaths.Count -gt 0) {
    Write-Host "Excluded paths: $($resolvedExcludePaths -join ', ')"
}
if ($publishDryRun) {
    Write-Host "Dry-run: enabled (no push will be performed)"
}

Write-Step "Checking remote refs"
$targetRemoteOid = Get-RemoteRefOid -RemoteUrl $publicUrl -RefName $TargetBranch -RefType heads
$releaseRemoteOid = ""
if ($shouldPublishRelease) {
    $releaseRemoteOid = Get-RemoteRefOid -RemoteUrl $publicUrl -RefName $releaseBranch -RefType heads
}
$tagRemoteOid = ""
if ($shouldPublishTag) {
    $tagRemoteOid = Get-RemoteRefOid -RemoteUrl $publicUrl -RefName $tagName -RefType tags
}

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$tempRoot = Join-Path $env:TEMP "ant-chrome-public-$Version-$timestamp"
$snapshotDir = Join-Path $tempRoot "snapshot"
$workRepoDir = Join-Path $tempRoot "workrepo"
$archivePath = Join-Path $tempRoot "snapshot.zip"

New-Item -ItemType Directory -Path $snapshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $workRepoDir -Force | Out-Null

try {
    Write-Step "Exporting source snapshot"
    Invoke-Git -Args @("archive", "--format=zip", "-o", $archivePath, $sourceCommit) | Out-Null
    Expand-Archive -LiteralPath $archivePath -DestinationPath $snapshotDir -Force
    if ($resolvedExcludePaths.Count -gt 0) {
        Write-Step "Removing excluded paths from snapshot"
        Remove-ExcludedPathsFromSnapshot -SnapshotDir $snapshotDir -Paths $resolvedExcludePaths
    }

    Write-Step "Applying sanitized public config"
    Apply-PublicConfigTemplate -SnapshotDir $snapshotDir

    if (-not $SkipRuntimeCheck) {
        Write-Step "Checking required runtime files in snapshot"
        $requiredFiles = @(
            "bin/xray.exe",
            "bin/sing-box.exe",
            "bin/linux-amd64/xray",
            "bin/linux-amd64/sing-box",
            "bin/linux-arm64/xray",
            "bin/linux-arm64/sing-box",
            "publish/runtime-manifest.json",
            "publish/runtime-sources.json"
        )
        foreach ($file in $requiredFiles) {
            $path = Join-Path $snapshotDir $file
            if (-not (Test-Path -LiteralPath $path)) {
                throw "Required runtime file missing in source snapshot: $file. Add it before publishing, or use -SkipRuntimeCheck."
            }
        }
    }

    Write-Step "Preparing temporary publish repository"
    Push-Location $workRepoDir
    try {
        Invoke-Git -Args @("init") | Out-Null
        Invoke-Git -Args @("remote", "add", "public", $publicUrl) | Out-Null
        Invoke-Git -Args @("config", "user.name", $committer.Name) | Out-Null
        Invoke-Git -Args @("config", "user.email", $committer.Email) | Out-Null

        if ((Get-TrimmedText $targetRemoteOid) -ne "") {
            Write-Step "Loading current public $TargetBranch"
            Invoke-Git -Args @("fetch", "--no-tags", "public", "refs/heads/${TargetBranch}:refs/remotes/public/$TargetBranch") | Out-Null
            Invoke-Git -Args @("checkout", "-B", $TargetBranch, "refs/remotes/public/$TargetBranch") | Out-Null
        } else {
            Write-Step "Creating initial public $TargetBranch"
            Invoke-Git -Args @("checkout", "--orphan", $TargetBranch) | Out-Null
        }

        Write-Step "Publishing snapshot to $TargetBranch (one aggregated commit)"
        Sync-SnapshotToRepo -SnapshotDir $snapshotDir -RepoDir $workRepoDir
        Invoke-Git -Args @("add", "-A") | Out-Null

        $targetMessage = Resolve-CommitMessage `
            -ExplicitMessage $CommitMessage `
            -ExplicitMessageFile $CommitMessageFile `
            -DefaultMessageFactory {
                Build-CommitMessage `
                    -Title "publish: $Version snapshot ($sourceShort)" `
                    -Channel $TargetBranch `
                    -VersionValue $Version `
                    -SourceRefValue $SourceRef `
                    -SourceCommit $sourceCommit `
                    -AppendSourceCommit:$IncludeSourceCommit
            }
        Invoke-GitCommit -Message $targetMessage -AllowEmpty

        $publishedCommit = Get-FirstOutputLine -Lines (Invoke-Git -Args @("rev-parse", "HEAD")).Output

        Publish-BranchRef -RemoteName "public" -BranchName $TargetBranch -Commitish $publishedCommit -RemoteExpectedOid $targetRemoteOid -DryRun:$publishDryRun

        if ($shouldPublishRelease) {
            Write-Step "Publishing release branch ref $releaseBranch"
            Publish-BranchRef -RemoteName "public" -BranchName $releaseBranch -Commitish $publishedCommit -RemoteExpectedOid $releaseRemoteOid -AllowRewrite -DryRun:$publishDryRun
        }

        if ($shouldPublishTag) {
            Write-Step "Publishing tag $tagName"
            Publish-TagRef `
                -RemoteName "public" `
                -TagName $tagName `
                -Commitish $publishedCommit `
                -VersionValue $Version `
                -SourceRefValue $SourceRef `
                -SourceCommit $sourceCommit `
                -RemoteExpectedOid $tagRemoteOid `
                -DryRun:$publishDryRun `
                -AppendSourceCommit:$IncludeSourceCommit
        }
    }
    finally {
        Pop-Location
    }

    Write-Step "Publish completed"
    Write-Host "Published source commit: $sourceCommit"
    Write-Host "Updated target branch: $TargetBranch (appends one aggregated commit per publish)"
    if ($shouldPublishRelease) {
        Write-Host "Updated release branch: $releaseBranch"
    }
    if ($shouldPublishTag) {
        Write-Host "Updated tag: $tagName"
    }
}
finally {
    if ($KeepTempDir) {
        Write-Host ""
        Write-Host "Temporary directory kept: $tempRoot"
    } else {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
