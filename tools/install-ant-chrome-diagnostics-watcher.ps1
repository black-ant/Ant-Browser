[CmdletBinding()]
param(
    [string]$InstallDir = 'E:\software\Ant Browser',
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'

function Get-NormalizedPath {
    param([string]$Path)

    return [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
}
$InstallDir = Get-NormalizedPath $InstallDir
$sourcePath = Join-Path $PSScriptRoot 'ant-chrome-diagnostics-watcher.ps1'
$diagnosticRoot = Join-Path $InstallDir 'data\diagnostics'
$destinationPath = Join-Path $diagnosticRoot 'ant-chrome-diagnostics-watcher.ps1'
$taskName = 'Ant Browser Diagnostics Watcher'
$userId = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name

if ($Uninstall) {
    Stop-ScheduledTask -TaskName $taskName -TaskPath ([string]'\') -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $taskName -TaskPath ([string]'\') -Confirm:$false -ErrorAction SilentlyContinue
    Write-Output ('Removed task: ' + $taskName)
    exit 0
}

if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
    throw ('Diagnostics watcher is missing: ' + $sourcePath)
}

Stop-ScheduledTask -TaskName $taskName -TaskPath ([string]'\') -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName $taskName -TaskPath ([string]'\') -Confirm:$false -ErrorAction SilentlyContinue

New-Item -ItemType Directory -Path $diagnosticRoot -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $diagnosticRoot 'incidents') -Force | Out-Null
Copy-Item -LiteralPath $sourcePath -Destination $destinationPath -Force

$powershellPath = Join-Path $PSHOME 'powershell.exe'
$quote = [char]34
$arguments = '-NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -WindowStyle Hidden -File ' + $quote + $destinationPath + $quote + ' -InstallDir ' + $quote + $InstallDir + $quote
$action = New-ScheduledTaskAction -Execute $powershellPath -Argument $arguments -WorkingDirectory $diagnosticRoot
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $userId
$principal = New-ScheduledTaskPrincipal -UserId $userId -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -MultipleInstances IgnoreNew -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
$task = New-ScheduledTask -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description 'Read-only Ant Browser exit diagnostics; no process termination and no app restart.'
$taskRoot = [string]'\'
Register-ScheduledTask -TaskName $taskName -TaskPath $taskRoot -InputObject $task -Force | Out-Null
Start-ScheduledTask -TaskName $taskName -TaskPath $taskRoot

Write-Output ('Deployed: ' + $destinationPath)
Write-Output ('Registered and started task: ' + $taskName)
