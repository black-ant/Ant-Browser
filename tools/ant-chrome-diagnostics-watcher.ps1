[CmdletBinding()]
param(
    [string]$InstallDir = 'E:\software\Ant Browser',
    [int]$PollMilliseconds = 2000,
    [int]$FileSampleSeconds = 30,
    [int]$EventWindowMinutes = 5,
    [int]$TailLines = 250,
    [switch]$Once
)

$ErrorActionPreference = 'Continue'

function Get-NormalizedPath {
    param([string]$Path)

    try {
        return [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
    } catch {
        return $Path.TrimEnd('\')
    }
}

function Convert-ToIsoTime {
    param([object]$Value)

    if ($null -eq $Value) {
        return $null
    }

    if ($Value -is [datetime]) {
        return ([datetime]$Value).ToString('o')
    }

    try {
        return ([System.Management.ManagementDateTimeConverter]::ToDateTime([string]$Value)).ToString('o')
    } catch {
        try {
            return (Get-Date $Value).ToString('o')
        } catch {
            return [string]$Value
        }
    }
}

function Ensure-Directory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Get-WatcherMutexName {
    param([string]$Path)

    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($Path.ToLowerInvariant())
        $hash = $sha256.ComputeHash($bytes)
        $suffix = ([System.BitConverter]::ToString($hash)).Replace('-', '').Substring(0, 16)
        return 'Local\AntBrowserDiagnosticsWatcher-' + $suffix
    } finally {
        $sha256.Dispose()
    }
}

function Write-Record {
    param(
        [string]$Event,
        [object]$Data
    )

    $record = [ordered]@{
        time = (Get-Date).ToString('o')
        event = $Event
        watcher_pid = $PID
    }

    if ($null -ne $Data) {
        foreach ($key in $Data.Keys) {
            $record[$key] = $Data[$key]
        }
    }

    try {
        ($record | ConvertTo-Json -Depth 12 -Compress) | Add-Content -LiteralPath $watcherLogPath -Encoding UTF8
    } catch {
    }
}

function Write-JsonFile {
    param(
        [string]$Path,
        [object]$Value
    )

    try {
        ($Value | ConvertTo-Json -Depth 12) | Set-Content -LiteralPath $Path -Encoding UTF8
    } catch {
        try {
            ('diagnostic write failed: ' + $_.Exception.Message) | Set-Content -LiteralPath $Path -Encoding UTF8
        } catch {
        }
    }
}

function Get-FileSnapshot {
    param([string]$Path)

    $snapshot = [ordered]@{
        path = $Path
        exists = $false
        length = $null
        last_write_time = $null
        sha256 = $null
        file_version = $null
        product_version = $null
    }

    try {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
            return $snapshot
        }

        $item = Get-Item -LiteralPath $Path -ErrorAction Stop
        $snapshot.exists = $true
        $snapshot.length = [int64]$item.Length
        $snapshot.last_write_time = $item.LastWriteTime.ToString('o')

        try {
            $hash = Get-FileHash -LiteralPath $Path -Algorithm SHA256 -ErrorAction Stop
            $snapshot.sha256 = $hash.Hash
        } catch {
        }

        try {
            $version = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
            $snapshot.file_version = $version.FileVersion
            $snapshot.product_version = $version.ProductVersion
        } catch {
        }
    } catch {
        $snapshot.error = $_.Exception.Message
    }

    return $snapshot
}

function Get-FileSnapshots {
    return [ordered]@{
        application = Get-FileSnapshot -Path $appPath
        uninstaller = Get-FileSnapshot -Path $uninstallerPath
    }
}

function Get-TargetProcessSnapshot {
    $processes = @(Get-Process -Name 'ant-chrome' -ErrorAction SilentlyContinue)
    foreach ($process in $processes) {
        $path = $null
        try {
            $path = $process.Path
        } catch {
        }

        if (-not $path -or -not [string]::Equals((Get-NormalizedPath $path), $appPath, [System.StringComparison]::OrdinalIgnoreCase)) {
            continue
        }

        try {
            $snapshot = Get-CimInstance -ClassName Win32_Process -Filter ('ProcessId=' + [int]$process.Id) -ErrorAction SilentlyContinue
            if ($snapshot) {
                return $snapshot
            }
        } catch {
        }

        return [pscustomobject]@{
            ProcessId = [int]$process.Id
            ParentProcessId = $null
            Name = 'ant-chrome.exe'
            ExecutablePath = $path
            CreationDate = $process.StartTime
        }
    }

    return $null
}

function New-ProcessObservation {
    param([object]$Snapshot)

    $processHandle = $null
    try {
        $processHandle = [System.Diagnostics.Process]::GetProcessById([int]$Snapshot.ProcessId)
    } catch {
    }

    return [ordered]@{
        pid = [int]$Snapshot.ProcessId
        parent_pid = [int]$Snapshot.ParentProcessId
        name = [string]$Snapshot.Name
        path = [string]$Snapshot.ExecutablePath
        started_at = Convert-ToIsoTime $Snapshot.CreationDate
        process_handle = $processHandle
        start_files = Get-FileSnapshots
    }
}

function Get-ObservedExitInfo {
    param([object]$Observation)

    $exitInfo = [ordered]@{
        observed_at = (Get-Date).ToString('o')
        available = $false
        code = $null
        error = $null
    }

    if ($null -eq $Observation.process_handle) {
        $exitInfo.error = 'process handle was not available'
        return $exitInfo
    }

    try {
        $Observation.process_handle.Refresh()
        if ($Observation.process_handle.HasExited) {
            $exitInfo.available = $true
            $exitInfo.code = [int]$Observation.process_handle.ExitCode
        } else {
            $exitInfo.error = 'process was not reported as exited when inspected'
        }
    } catch {
        $exitInfo.error = $_.Exception.Message
    }

    return $exitInfo
}

function Get-ProcessKind {
    param([string]$Name)

    $value = $Name.ToLowerInvariant()
    if ($value -eq 'msedgewebview2.exe') {
        return 'webview2'
    }
    if ($value -eq 'xray.exe' -or $value -eq 'sing-box.exe' -or $value -eq 'mihomo.exe') {
        return 'proxy'
    }
    return 'other'
}

function New-ProcessRow {
    param([object]$Process)

    return [ordered]@{
        pid = [int]$Process.ProcessId
        parent_pid = [int]$Process.ParentProcessId
        name = [string]$Process.Name
        kind = Get-ProcessKind -Name ([string]$Process.Name)
        path = [string]$Process.ExecutablePath
        created_at = Convert-ToIsoTime $Process.CreationDate
    }
}

function Get-DescendantProcessSnapshot {
    param([int]$RootPid)

    $allProcesses = @()
    try {
        $allProcesses = @(Get-CimInstance -ClassName Win32_Process -ErrorAction SilentlyContinue)
    } catch {
        return [ordered]@{
            root_pid = $RootPid
            processes = @()
            error = $_.Exception.Message
        }
    }

    $childrenByParent = @{}
    foreach ($process in $allProcesses) {
        $parentKey = [string]([int]$process.ParentProcessId)
        if (-not $childrenByParent.ContainsKey($parentKey)) {
            $childrenByParent[$parentKey] = New-Object System.Collections.Generic.List[object]
        }
        $childrenByParent[$parentKey].Add($process)
    }

    $queue = New-Object 'System.Collections.Generic.Queue[int]'
    $seen = New-Object 'System.Collections.Generic.HashSet[int]'
    $queue.Enqueue($RootPid)
    $rows = New-Object System.Collections.Generic.List[object]

    while ($queue.Count -gt 0) {
        $parentPid = $queue.Dequeue()
        $parentKey = [string]$parentPid
        if (-not $childrenByParent.ContainsKey($parentKey)) {
            continue
        }

        foreach ($child in $childrenByParent[$parentKey]) {
            $childPid = [int]$child.ProcessId
            if (-not $seen.Add($childPid)) {
                continue
            }
            $rows.Add((New-ProcessRow -Process $child))
            $queue.Enqueue($childPid)
        }
    }

    return [ordered]@{
        root_pid = $RootPid
        processes = $rows.ToArray()
    }
}

function Copy-TailLog {
    param(
        [string]$SourcePath,
        [string]$DestinationPath
    )

    try {
        if (Test-Path -LiteralPath $SourcePath -PathType Leaf) {
            $content = @(Get-Content -LiteralPath $SourcePath -Tail $TailLines -ErrorAction Stop)
            if ($content.Count -gt 0) {
                $content | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
            } else {
                '' | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
            }
        } else {
            ('missing: ' + $SourcePath) | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
        }
    } catch {
        ('read failed: ' + $_.Exception.Message) | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
    }
}

function Copy-WailsTailLog {
    param([string]$DestinationPath)

    try {
        if (-not (Test-Path -LiteralPath $lifecycleLogPath -PathType Leaf)) {
            ('missing: ' + $lifecycleLogPath) | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
            return
        }

        $pattern = 'event\s*:\s*wails\.log'
        $content = @(Get-Content -LiteralPath $lifecycleLogPath -Tail ($TailLines * 4) -ErrorAction Stop | Where-Object { $_ -match $pattern } | Select-Object -Last $TailLines)
        if ($content.Count -gt 0) {
            $content | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
        } else {
            'no wails.log events found in app-lifecycle.log' | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
        }
    } catch {
        ('read failed: ' + $_.Exception.Message) | Set-Content -LiteralPath $DestinationPath -Encoding UTF8
    }
}

function Get-EventRecord {
    param([object]$Event)

    $message = ''
    try {
        $message = [string]$Event.Message
    } catch {
    }
    if ($message.Length -gt 8000) {
        $message = $message.Substring(0, 8000)
    }

    return [ordered]@{
        log_name = [string]$Event.LogName
        time = Convert-ToIsoTime $Event.TimeCreated
        id = [int]$Event.Id
        level = [string]$Event.LevelDisplayName
        provider = [string]$Event.ProviderName
        message = $message
    }
}

function Get-RelatedEvents {
    param([datetime]$EndTime)

    $from = $EndTime.ToUniversalTime().AddMinutes(-1 * $EventWindowMinutes)
    $to = $EndTime.ToUniversalTime().AddSeconds(10)
    $records = New-Object System.Collections.Generic.List[object]
    $errors = New-Object System.Collections.Generic.List[object]
    $logNames = @(
        'Application',
        'Microsoft-Windows-WER-Diagnostics/Operational',
        'Microsoft-Windows-TaskScheduler/Operational',
        'Microsoft-Windows-EdgeUpdate/Operational'
    )

    foreach ($logName in $logNames) {
        try {
            Get-WinEvent -ListLog $logName -ErrorAction Stop | Out-Null
        } catch {
            $errors.Add([ordered]@{ log_name = $logName; error = 'log unavailable' })
            continue
        }

        try {
            $events = @(Get-WinEvent -FilterHashtable @{ LogName = $logName; StartTime = $from; EndTime = $to } -ErrorAction Stop)
        } catch {
            $errors.Add([ordered]@{ log_name = $logName; error = $_.Exception.Message })
            continue
        }

        foreach ($event in $events) {
            $message = [string]$event.Message
            $include = $false

            if ($logName -eq 'Application') {
                $include = ($event.ProviderName -match 'Application Error|Windows Error Reporting|\.NET Runtime|Application Hang|SideBySide') -or ($event.Id -in @(1000, 1001, 1002, 1026))
            } elseif ($logName -match 'TaskScheduler') {
                $include = $message -match 'Ant Browser|ant-chrome|Diagnostics Watcher|AntLaunch'
            } else {
                $include = $true
            }

            if ($include) {
                $records.Add((Get-EventRecord -Event $event))
            }

            if ($records.Count -ge 300) {
                break
            }
        }

        if ($records.Count -ge 300) {
            break
        }
    }

    return [ordered]@{
        from = $from.ToString('o')
        to = $to.ToString('o')
        events = $records.ToArray()
        errors = $errors.ToArray()
    }
}

function Get-DumpSnapshot {
    $rows = New-Object System.Collections.Generic.List[object]

    try {
        if (Test-Path -LiteralPath $dumpDir -PathType Container) {
            foreach ($file in @(Get-ChildItem -LiteralPath $dumpDir -File -ErrorAction SilentlyContinue)) {
                $rows.Add([ordered]@{
                    name = $file.Name
                    length = [int64]$file.Length
                    last_write_time = $file.LastWriteTime.ToString('o')
                })
            }
        }
    } catch {
        return [ordered]@{ files = @(); error = $_.Exception.Message }
    }

    return [ordered]@{ files = $rows.ToArray() }
}

function New-Incident {
    param(
        [object]$Observation,
        [object]$ExitInfo
    )

    $incidentName = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssfffZ') + '-pid' + $Observation.pid
    $incidentDir = Join-Path $incidentRoot $incidentName
    Ensure-Directory -Path $incidentDir

    Write-Record -Event 'process.exit.detected' -Data @{
        app_pid = $Observation.pid
        parent_pid = $Observation.parent_pid
        exit_code_available = $ExitInfo.available
        exit_code = $ExitInfo.code
        incident_dir = $incidentDir
    }

    $endTime = Get-Date
    $currentFiles = Get-FileSnapshots
    $tree = Get-DescendantProcessSnapshot -RootPid $Observation.pid
    $events = Get-RelatedEvents -EndTime ([datetime]$ExitInfo.observed_at)
    $dumps = Get-DumpSnapshot

    Write-JsonFile -Path (Join-Path $incidentDir 'file-snapshot.json') -Value ([ordered]@{
        at = $endTime.ToString('o')
        application = $currentFiles.application
        uninstaller = $currentFiles.uninstaller
        start = $Observation.start_files
    })
    Write-JsonFile -Path (Join-Path $incidentDir 'process-tree.json') -Value ([ordered]@{
        app = [ordered]@{
            pid = $Observation.pid
            parent_pid = $Observation.parent_pid
            name = $Observation.name
            path = $Observation.path
            started_at = $Observation.started_at
            observed_exit_at = $ExitInfo.observed_at
        }
        descendants = $tree
    })
    Write-JsonFile -Path (Join-Path $incidentDir 'windows-events.json') -Value $events
    Write-JsonFile -Path (Join-Path $incidentDir 'wer-dumps.json') -Value $dumps

    Copy-TailLog -SourcePath $lifecycleLogPath -DestinationPath (Join-Path $incidentDir 'app-lifecycle.log.tail')
    Copy-TailLog -SourcePath $supervisorLogPath -DestinationPath (Join-Path $incidentDir 'process-supervisor.log.tail')
    Copy-WailsTailLog -DestinationPath (Join-Path $incidentDir 'wails.log.tail')
    Copy-TailLog -SourcePath $watcherLogPath -DestinationPath (Join-Path $incidentDir 'watcher.log.tail')

    $manifest = [ordered]@{
        schema_version = 1
        incident_id = $incidentName
        created_at = $endTime.ToString('o')
        collection_mode = 'read-only'
        app = [ordered]@{
            pid = $Observation.pid
            parent_pid = $Observation.parent_pid
            name = $Observation.name
            path = $Observation.path
            started_at = $Observation.started_at
            observed_exit_at = $ExitInfo.observed_at
        }
        exit = $ExitInfo
        files = 'file-snapshot.json'
        process_tree = 'process-tree.json'
        windows_events = 'windows-events.json'
        wer_dumps = 'wer-dumps.json'
        log_tails = @(
            'app-lifecycle.log.tail',
            'process-supervisor.log.tail',
            'wails.log.tail',
            'watcher.log.tail'
        )
    }
    Write-JsonFile -Path (Join-Path $incidentDir 'manifest.json') -Value $manifest
    return $incidentDir
}

if ($PollMilliseconds -lt 500) {
    $PollMilliseconds = 500
}
if ($FileSampleSeconds -lt 5) {
    $FileSampleSeconds = 5
}
if ($EventWindowMinutes -lt 1) {
    $EventWindowMinutes = 1
}
if ($TailLines -lt 20) {
    $TailLines = 20
}

$InstallDir = Get-NormalizedPath $InstallDir
$appPath = Join-Path $InstallDir 'ant-chrome.exe'
$uninstallerPath = Join-Path $InstallDir 'Uninstall.exe'
$dataDir = Join-Path $InstallDir 'data'
$diagnosticRoot = Join-Path $dataDir 'diagnostics'
$incidentRoot = Join-Path $diagnosticRoot 'incidents'
$dumpDir = Join-Path $diagnosticRoot 'dumps'
$logsDir = Join-Path $dataDir 'logs'
$watcherLogPath = Join-Path $diagnosticRoot 'watcher.log'
$statePath = Join-Path $diagnosticRoot 'watcher-state.json'
$lifecycleLogPath = Join-Path $logsDir 'app-lifecycle.log'
$supervisorLogPath = Join-Path $logsDir 'process-supervisor.log'
$wailsLogPath = Join-Path $logsDir 'wails.log'

Ensure-Directory -Path $diagnosticRoot
Ensure-Directory -Path $incidentRoot

$mutex = New-Object System.Threading.Mutex($false, (Get-WatcherMutexName -Path $appPath))
$mutexOwned = $false
try {
    try {
        $mutexOwned = $mutex.WaitOne(0)
    } catch [System.Threading.AbandonedMutexException] {
        $mutexOwned = $true
    }

    if (-not $mutexOwned) {
        exit 0
    }

    Write-Record -Event 'watcher.start' -Data @{
        mode = 'read-only'
        app = $appPath
        poll_milliseconds = $PollMilliseconds
        task_safe = $true
        can_terminate_processes = $false
    }

    $lastFileSnapshot = Get-FileSnapshots
    $lastFileSampleAt = Get-Date
    Write-Record -Event 'files.snapshot' -Data $lastFileSnapshot

    $observation = $null
    $waitingLogged = $false
    $lastIncident = $null
    Write-JsonFile -Path $statePath -Value ([ordered]@{
        watcher_pid = $PID
        app = $appPath
        observed_pid = $null
        observed_started_at = $null
        last_incident = $null
        updated_at = (Get-Date).ToString('o')
    })

    while ($true) {
        $target = Get-TargetProcessSnapshot

        if ($null -eq $observation -and $null -eq $target) {
            if (-not $waitingLogged) {
                Write-Record -Event 'app.waiting' -Data @{ app = $appPath }
                $waitingLogged = $true
            }
        } elseif ($null -eq $observation -and $null -ne $target) {
            $observation = New-ProcessObservation -Snapshot $target
            $waitingLogged = $false
            Write-Record -Event 'app.started' -Data @{
                app_pid = $observation.pid
                parent_pid = $observation.parent_pid
                started_at = $observation.started_at
            }
            Write-JsonFile -Path $statePath -Value ([ordered]@{
                watcher_pid = $PID
                app = $appPath
                observed_pid = $observation.pid
                observed_started_at = $observation.started_at
                last_incident = $lastIncident
                updated_at = (Get-Date).ToString('o')
            })
        } elseif ($null -ne $observation -and ($null -eq $target -or [int]$target.ProcessId -ne [int]$observation.pid)) {
            $exitInfo = Get-ObservedExitInfo -Observation $observation
            $lastIncident = New-Incident -Observation $observation -ExitInfo $exitInfo
            Write-Record -Event 'incident.completed' -Data @{
                app_pid = $observation.pid
                incident_dir = $lastIncident
                exit_code_available = $exitInfo.available
                exit_code = $exitInfo.code
            }
            $observation = $null
            Write-JsonFile -Path $statePath -Value ([ordered]@{
                watcher_pid = $PID
                app = $appPath
                observed_pid = $null
                observed_started_at = $null
                last_incident = $lastIncident
                updated_at = (Get-Date).ToString('o')
            })

            if ($null -ne $target) {
                $observation = New-ProcessObservation -Snapshot $target
                Write-Record -Event 'app.started' -Data @{
                    app_pid = $observation.pid
                    parent_pid = $observation.parent_pid
                    started_at = $observation.started_at
                }
            }
        }

        $now = Get-Date
        if (($now - $lastFileSampleAt).TotalSeconds -ge $FileSampleSeconds) {
            $currentFileSnapshot = Get-FileSnapshots
            $previousJson = $lastFileSnapshot | ConvertTo-Json -Depth 8 -Compress
            $currentJson = $currentFileSnapshot | ConvertTo-Json -Depth 8 -Compress
            if ($previousJson -ne $currentJson) {
                Write-Record -Event 'files.changed' -Data @{
                    before = $lastFileSnapshot
                    after = $currentFileSnapshot
                }
                $lastFileSnapshot = $currentFileSnapshot
            }
            $lastFileSampleAt = $now
        }

        if ($Once) {
            break
        }

        Start-Sleep -Milliseconds $PollMilliseconds
    }
} finally {
    Write-Record -Event 'watcher.stop' -Data @{ mode = 'read-only' }
    if ($mutexOwned) {
        try {
            $mutex.ReleaseMutex()
        } catch {
        }
    }
    $mutex.Dispose()
}
