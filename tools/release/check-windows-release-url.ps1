param(
    [Parameter(Mandatory = $true)]
    [string]$ManifestUrl,

    [string]$Target = "windows-amd64"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-PackageUrl {
    param(
        [Parameter(Mandatory = $true)]
        [string]$BaseUrl,

        [Parameter(Mandatory = $true)]
        [string]$PackageUrl
    )

    if ($PackageUrl -match "^[a-zA-Z][a-zA-Z0-9+.-]*://") {
        return $PackageUrl
    }

    $baseUri = [Uri]::new($BaseUrl)
    return ([Uri]::new($baseUri, $PackageUrl)).AbsoluteUri
}

function Get-Sha256 {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Invoke-Download {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Url,

        [Parameter(Mandatory = $true)]
        [string]$OutputPath
    )

    Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $OutputPath -TimeoutSec 120
}

Write-Host "[INFO] checking release manifest: $ManifestUrl"
$manifestResponse = Invoke-WebRequest -UseBasicParsing -Uri $ManifestUrl -TimeoutSec 30
$manifestContent = [string]$manifestResponse.Content
$manifestContent = $manifestContent.TrimStart([char[]]@([char]0xFEFF, [char]0xEF, [char]0xBB, [char]0xBF))
$manifest = $manifestContent | ConvertFrom-Json

if ([int]$manifest.schemaVersion -ne 1) {
    throw "unexpected manifest schemaVersion: $($manifest.schemaVersion)"
}

$packages = @($manifest.packages)
if ($packages.Count -eq 0) {
    throw "manifest has no packages"
}

$package = $packages | Where-Object {
    ([string]$_.target).Trim().ToLowerInvariant() -eq $Target.ToLowerInvariant()
} | Select-Object -First 1

if ($null -eq $package) {
    throw "manifest does not contain target package: $Target"
}

if ([string]$package.payloadType -ne "full") {
    throw "unexpected payloadType for $Target`: $($package.payloadType)"
}

$expectedSha = ([string]$package.sha256).Trim().ToLowerInvariant()
if ($expectedSha -notmatch "^[0-9a-f]{64}$") {
    throw "invalid sha256 for $Target`: $($package.sha256)"
}

$packageRef = ([string]$package.url).Trim()
if ($packageRef -eq "") {
    throw "package url is empty for $Target"
}

$packageUrl = Resolve-PackageUrl -BaseUrl $ManifestUrl -PackageUrl $packageRef
$tempFile = Join-Path ([System.IO.Path]::GetTempPath()) ("maka-browser-release-{0}-{1}.zip" -f $Target, ([Guid]::NewGuid().ToString("N")))

try {
    Write-Host "[INFO] downloading package: $packageUrl"
    Invoke-Download -Url $packageUrl -OutputPath $tempFile

    $actualSha = Get-Sha256 -Path $tempFile
    if ($actualSha -ne $expectedSha) {
        throw "package sha256 mismatch for $Target`: expected $expectedSha, got $actualSha"
    }

    $actualSize = (Get-Item -LiteralPath $tempFile).Length
    $hasSize = $package.PSObject.Properties.Name -contains "size"
    if ($hasSize -and $null -ne $package.size -and [int64]$package.size -ne $actualSize) {
        throw "package size mismatch for $Target`: expected $($package.size), got $actualSize"
    }

    Write-Host "[OK] release URL preflight passed"
    Write-Host "[OK] manifest: $ManifestUrl"
    Write-Host "[OK] package: $packageUrl"
    Write-Host "[OK] sha256: $actualSha"
}
finally {
    Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
}
