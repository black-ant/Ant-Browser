import { spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
export const frontendDir = resolve(scriptDir, '..')
export const repoRoot = resolve(frontendDir, '..')

const frontendDirLower = frontendDir.toLowerCase()
const repoRootLower = repoRoot.toLowerCase()
const defaultPreferredPort = 5218
const maxCandidateCount = 20
const processInspectionFilter = "Name = 'node.exe' OR Name = 'cmd.exe' OR Name = 'npm.exe' OR Name = 'esbuild.exe' OR Name = 'wails.exe'"

function runPowerShell(command, cwd = repoRoot) {
  return spawnSync('powershell.exe', ['-NoProfile', '-Command', command], {
    cwd,
    encoding: 'utf8',
    timeout: 12000,
  })
}

function normalizeProcessChain(proc) {
  const chain = Array.isArray(proc?.chain) ? proc.chain : []
  return [
    String(proc?.commandLine || ''),
    ...chain.map((item) => String(item?.commandLine || '')),
  ]
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
}

function collectProcessesByPowerShell(filterCommand) {
  const result = runPowerShell(filterCommand)

  if (result.error) {
    throw new Error(result.error.message || 'failed to inspect processes')
  }

  if (result.status !== 0) {
    throw new Error(result.stderr?.trim() || 'failed to inspect processes')
  }

  const text = result.stdout.trim()
  if (!text) {
    return []
  }

  const parsed = JSON.parse(text)
  return Array.isArray(parsed) ? parsed : [parsed]
}

export function resolveRequestedPort(rawPort, fallbackPort = defaultPreferredPort) {
  const parsed = Number.parseInt(String(rawPort || '').trim(), 10)
  if (Number.isInteger(parsed) && parsed > 0 && parsed <= 65535) {
    return parsed
  }
  return fallbackPort
}

export function listListeners(port) {
  const items = collectProcessesByPowerShell(`
$port = ${port}
$items = @()
$allProcs = Get-CimInstance Win32_Process
$procMap = @{}
foreach ($item in $allProcs) { $procMap[[string]$item.ProcessId] = $item }
$conns = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
foreach ($conn in $conns) {
  $proc = $procMap[[string]$conn.OwningProcess]
  if (-not $proc) { continue }

  $chain = @()
  $current = $proc
  $visited = @{}
  $depth = 0
  while ($current -and $depth -lt 8 -and -not $visited.ContainsKey([string]$current.ProcessId)) {
    $visited[[string]$current.ProcessId] = $true
    $chain += [PSCustomObject]@{
      pid = [int]$current.ProcessId
      parentProcessId = [int]$current.ParentProcessId
      name = [string]$current.Name
      commandLine = [string]$current.CommandLine
    }
    if ($current.ParentProcessId -le 0) { break }
    $current = $procMap[[string]$current.ParentProcessId]
    $depth++
  }

  $items += [PSCustomObject]@{
    pid = [int]$proc.ProcessId
    parentProcessId = [int]$proc.ParentProcessId
    name = [string]$proc.Name
    commandLine = [string]$proc.CommandLine
    chain = $chain
  }
}
$items | ConvertTo-Json -Compress -Depth 6
`)
  return items
}

export function summarizeProcess(proc) {
  const name = proc?.name || 'unknown'
  const pid = proc?.pid || 0
  const commandLine = String(proc?.commandLine || '').trim()
  if (!commandLine) {
    return `${name} (PID ${pid})`
  }
  return `${name} (PID ${pid}) ${commandLine}`
}

export function isProjectDevProcess(proc) {
  const lines = normalizeProcessChain(proc)
  const hasProjectPath = lines.some((line) => line.includes(frontendDirLower) || line.includes(repoRootLower))
  const hasDevWatcher = lines.some((line) => line.includes('scripts/dev-watcher.mjs'))
  const hasRawDev = lines.some((line) => line.includes('npm run dev:raw'))
  const hasVite = lines.some((line) => line.includes('vite/bin/vite.js') || line.includes('\\vite\\bin\\vite.js'))
  const hasNpmDev = lines.some((line) => line.includes('npm run dev'))
  const hasEnsureNative = lines.some((line) => line.includes('ensure-rollup-native.mjs'))
  const hasWailsDev = lines.some((line) => line.includes('wails dev'))
  const hasEsbuild = lines.some((line) => line.includes('esbuild.exe') || line.endsWith('\\esbuild'))
  const hasDevMarker = hasDevWatcher || hasRawDev || hasVite || hasNpmDev || hasEnsureNative || hasWailsDev || hasEsbuild

  return hasProjectPath && hasDevMarker
}

export function killProcessTree(pid) {
  if (!pid || pid <= 0) {
    return true
  }

  const killed = spawnSync('taskkill.exe', ['/F', '/T', '/PID', String(pid)], {
    cwd: repoRoot,
    stdio: 'ignore',
    timeout: 8000,
  })
  if (killed.error) {
    return false
  }
  return killed.status === 0
}

export function listProjectDevProcesses() {
  return collectProcessesByPowerShell(`
$items = @()
$allProcs = Get-CimInstance Win32_Process
$procMap = @{}
foreach ($item in $allProcs) { $procMap[[string]$item.ProcessId] = $item }
$targetNames = @('node.exe', 'cmd.exe', 'npm.exe', 'esbuild.exe', 'wails.exe')
$procs = $allProcs | Where-Object { $targetNames -contains $_.Name }
foreach ($proc in $procs) {
  $chain = @()
  $current = $proc
  $visited = @{}
  $depth = 0
  while ($current -and $depth -lt 8 -and -not $visited.ContainsKey([string]$current.ProcessId)) {
    $visited[[string]$current.ProcessId] = $true
    $chain += [PSCustomObject]@{
      pid = [int]$current.ProcessId
      parentProcessId = [int]$current.ParentProcessId
      name = [string]$current.Name
      commandLine = [string]$current.CommandLine
    }
    if ($current.ParentProcessId -le 0) { break }
    $current = $procMap[[string]$current.ParentProcessId]
    $depth++
  }

  $items += [PSCustomObject]@{
    pid = [int]$proc.ProcessId
    parentProcessId = [int]$proc.ParentProcessId
    name = [string]$proc.Name
    commandLine = [string]$proc.CommandLine
    chain = $chain
  }
}
$items | ConvertTo-Json -Compress -Depth 6
`)
}

function selectRootProcesses(processes) {
  const pidSet = new Set(processes.map((proc) => proc.pid))
  return processes.filter((proc) => !pidSet.has(proc.parentProcessId))
}

function waitForPortToClear(port, timeoutMs = 2500) {
  const startedAt = Date.now()
  while (Date.now() - startedAt < timeoutMs) {
    if (listListeners(port).length === 0) {
      return true
    }
    runPowerShell('Start-Sleep -Milliseconds 150')
  }
  return listListeners(port).length === 0
}

export function cleanupProjectListeners(port, logger = console.error) {
  const listeners = listListeners(port)
  let cleaned = false

  for (const proc of listeners) {
    if (!isProjectDevProcess(proc)) {
      continue
    }

    cleaned = true
    logger(`[dev] cleaning stale project dev process on ${port}: ${summarizeProcess(proc)}`)
    if (!killProcessTree(proc.pid)) {
      throw new Error(`failed to kill stale dev process ${proc.pid} on port ${port}`)
    }
  }

  if (cleaned && !waitForPortToClear(port)) {
    throw new Error(`port ${port} is still occupied after cleaning stale project dev processes`)
  }

  return cleaned
}

export function cleanupProjectDevProcesses(logger = console.error) {
  const processes = listProjectDevProcesses().filter(isProjectDevProcess)
  const roots = selectRootProcesses(processes)

  for (const proc of roots) {
    logger(`[dev] cleaning stale project dev process: ${summarizeProcess(proc)}`)
    if (!killProcessTree(proc.pid)) {
      throw new Error(`failed to kill stale project dev process ${proc.pid}`)
    }
  }

  return roots.length > 0
}

export function resolveFrontendDevPort(preferredPort = defaultPreferredPort, logger = console.error, allowFallback = true) {
  const requestedPort = resolveRequestedPort(preferredPort)

  cleanupProjectListeners(requestedPort, logger)
  let listeners = listListeners(requestedPort)
  if (listeners.length === 0) {
    return {
      port: requestedPort,
      preferredPort: requestedPort,
      reusedPreferredPort: true,
      reason: 'preferred-port-available',
    }
  }

  logger(`[dev] preferred frontend port ${requestedPort} is occupied by: ${listeners.map(summarizeProcess).join('; ')}`)
  if (!allowFallback) {
    throw new Error(`preferred frontend port ${requestedPort} is occupied`)
  }

  for (let offset = 1; offset <= maxCandidateCount; offset++) {
    const candidatePort = requestedPort + offset
    cleanupProjectListeners(candidatePort, logger)
    listeners = listListeners(candidatePort)
    if (listeners.length === 0) {
      logger(`[dev] switching frontend dev server from ${requestedPort} to ${candidatePort}`)
      return {
        port: candidatePort,
        preferredPort: requestedPort,
        reusedPreferredPort: false,
        reason: 'fallback-port-selected',
      }
    }
  }

  throw new Error(`failed to find a free frontend dev port in range ${requestedPort}-${requestedPort + maxCandidateCount}`)
}

function parseCliArgs(argv) {
  const args = argv.slice(2)
  const command = args[0] || 'resolve'
  let preferredPort = defaultPreferredPort

  for (let index = 1; index < args.length; index++) {
    const item = args[index]
    if (item === '--preferred' && index + 1 < args.length) {
      preferredPort = resolveRequestedPort(args[index + 1], defaultPreferredPort)
      index++
    }
  }

  return { command, preferredPort }
}

function runCli() {
  const { command, preferredPort } = parseCliArgs(process.argv)
  if (command === 'cleanup') {
    cleanupProjectDevProcesses()
    return
  }
  if (command !== 'resolve') {
    throw new Error(`unsupported command: ${command}`)
  }

  const result = resolveFrontendDevPort(preferredPort)
  process.stdout.write(`${result.port}\n`)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    runCli()
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    process.stderr.write(`${message}\n`)
    process.exit(1)
  }
}
