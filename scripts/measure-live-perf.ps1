# measure-live-perf.ps1 — 统计所有内核进程的内存与进程数,用于 live_perf 开/关对比。
#
# 用法:
#   1. 开 N 个直播窗口,稳定播放 5 分钟
#   2. 运行本脚本记录结果
#   3. 改 config.yaml 的 live_perf_enabled,重启全部实例,重复 1-2
#
# 关注两个指标:每实例内存均值、每实例进程均值。预期 A 级开启后内存 -30~40%、进程数明显下降。

$ErrorActionPreference = 'SilentlyContinue'

$procs = Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match '--user-data-dir' }
$total = 0
$count = 0
$byProfile = @{}

foreach ($p in $procs) {
    $proc = Get-Process -Id $p.ProcessId
    if (-not $proc) { continue }
    $ws = $proc.WorkingSet64
    $total += $ws
    $count++
    if ($p.CommandLine -match '--user-data-dir=([^\s"]+)') {
        $key = $Matches[1]
        $byProfile[$key] = [long]($byProfile[$key]) + $ws
    }
}

"===== 内核进程统计 ====="
"进程总数: $count"
"内存总量: {0:N0} MB" -f ($total / 1MB)
"实例数量: $($byProfile.Count)"

if ($byProfile.Count -gt 0) {
    "每实例内存均值: {0:N0} MB" -f ($total / 1MB / $byProfile.Count)
    "每实例进程均值: {0:N1}" -f ($count / $byProfile.Count)
    ""
    "===== 内存占用 Top 10 实例 ====="
    $byProfile.GetEnumerator() | Sort-Object Value -Descending | Select-Object -First 10 | ForEach-Object {
        "{0,8:N0} MB  {1}" -f ($_.Value / 1MB), (Split-Path $_.Key -Leaf)
    }
}
