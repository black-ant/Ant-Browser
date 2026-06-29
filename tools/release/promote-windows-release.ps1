param(
    [Parameter(Mandatory = $true)]
    [string]$ReleaseVersion,
    [string]$RemoteRoot = "/opt/1688shop/releases/windows"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Version = $ReleaseVersion

function Require-Env {
    param([string]$Name)

    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "required environment variable missing: $Name"
    }
    return $value.Trim()
}

function Invoke-Native {
    param(
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Protect-PrivateKeyFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ($env:OS -eq "Windows_NT") {
        $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
        $acl = Get-Acl -LiteralPath $Path
        $acl.SetAccessRuleProtection($true, $false)
        $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity, "FullControl", "Allow")
        $acl.SetAccessRule($rule)
        Set-Acl -LiteralPath $Path -AclObject $acl
        return
    }

    Invoke-Native -FilePath "chmod" -Arguments @("600", $Path)
}

$hostName = Require-Env "WINDOWS_RELEASE_SSH_HOST"
$port = Require-Env "WINDOWS_RELEASE_SSH_PORT"
$user = Require-Env "WINDOWS_RELEASE_SSH_USER"
$keyText = Require-Env "WINDOWS_RELEASE_SSH_KEY"

$tempKey = Join-Path $env:TEMP "windows-release-key-$([guid]::NewGuid().ToString('N')).pem"
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($tempKey, $keyText.Replace("`r`n", "`n"), $utf8NoBom)
Protect-PrivateKeyFile -Path $tempKey

try {
    $target = "$user@$hostName"
    $sshBaseArgs = @("-i", $tempKey, "-p", $port, "-o", "StrictHostKeyChecking=accept-new")
    $remoteScript = @"
set -eu
test_dir='$RemoteRoot/test/$Version'
stable_dir='$RemoteRoot/stable/$Version'
stable_alias='$RemoteRoot/stable/app-update-stable.json'
if [ ! -d "`$test_dir" ]; then
  echo "missing test release: `$test_dir" >&2
  exit 31
fi
if [ -e "`$stable_dir" ]; then
  echo "stable release already exists: `$stable_dir" >&2
  exit 32
fi
mkdir -p '$RemoteRoot/stable'
cp -a "`$test_dir" "`$stable_dir"
cd "`$stable_dir"
sha256sum MakaBrowser-$Version-windows-amd64.zip app-update-stable.json > promotion-sha256.txt
cp -f "`$`{stable_dir`}/app-update-stable.json" "`$stable_alias"
echo "[OK] promoted $Version to stable"
"@

    Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs + @($target, $remoteScript))
}
finally {
    Remove-Item -LiteralPath $tempKey -Force -ErrorAction SilentlyContinue
}
