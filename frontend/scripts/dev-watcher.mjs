import { spawn, spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const frontendDir = resolve(scriptDir, '..')
const defaultVitePort = 5218
const defaultMaxOldSpaceSizeMb = 512
const defaultMaxSemiSpaceSizeMb = 16
const defaultRssWarnMb = 384
const defaultRssHardLimitMb = 0
const defaultRssHardLimitHits = 3
const defaultRssAutoRestart = false
const defaultRssRestartDelayMs = 1500
const defaultRssRestartMaxCount = 3
const defaultRssRestartWindowMs = 300000
const defaultMemoryPollMs = 3000
const nodeExecutable = process.execPath
const ensureNativeScript = resolve(frontendDir, 'scripts', 'ensure-rollup-native.mjs')
const viteEntry = resolve(frontendDir, 'node_modules', 'vite', 'bin', 'vite.js')

function ensureNativeRuntime(env) {
  const result = spawnSync(nodeExecutable, [ensureNativeScript], {
    cwd: frontendDir,
    stdio: 'inherit',
    env,
  })

  if (result.error) {
    throw result.error
  }
  if ((result.status ?? 0) !== 0) {
    throw new Error(`ensure native failed with exit code ${result.status ?? 1}`)
  }
}

function resolveRequestedPort(rawPort, fallbackPort = defaultVitePort) {
  const parsed = Number.parseInt(String(rawPort || '').trim(), 10)
  if (Number.isInteger(parsed) && parsed > 0 && parsed <= 65535) {
    return parsed
  }
  return fallbackPort
}

function resolvePositiveInteger(rawValue, fallbackValue) {
  const parsed = Number.parseInt(String(rawValue || '').trim(), 10)
  if (Number.isInteger(parsed) && parsed > 0) {
    return parsed
  }
  return fallbackValue
}

function resolveNonNegativeInteger(rawValue, fallbackValue) {
  const raw = String(rawValue ?? '').trim()
  if (!raw) {
    return fallbackValue
  }

  const parsed = Number.parseInt(raw, 10)
  if (Number.isInteger(parsed) && parsed >= 0) {
    return parsed
  }
  return fallbackValue
}

function resolveBoolean(rawValue, fallbackValue) {
  const raw = String(rawValue ?? '').trim().toLowerCase()
  if (!raw) {
    return fallbackValue
  }
  if (raw === '1' || raw === 'true' || raw === 'yes' || raw === 'on') {
    return true
  }
  if (raw === '0' || raw === 'false' || raw === 'no' || raw === 'off') {
    return false
  }
  return fallbackValue
}

function resolveNodeArgs(env) {
  const maxOldSpaceSizeMb = resolvePositiveInteger(
    env.FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB,
    defaultMaxOldSpaceSizeMb,
  )
  const maxSemiSpaceSizeMb = resolvePositiveInteger(
    env.FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB,
    defaultMaxSemiSpaceSizeMb,
  )

  const args = [`--max-old-space-size=${maxOldSpaceSizeMb}`]
  if (maxSemiSpaceSizeMb > 0) {
    args.push(`--max-semi-space-size=${maxSemiSpaceSizeMb}`)
  }
  if (String(env.FRONTEND_NODE_HEAP_SNAPSHOT || '').trim() === '1') {
    args.push('--heapsnapshot-near-heap-limit=2')
  }
  args.push(viteEntry)

  return {
    args,
    maxOldSpaceSizeMb,
    maxSemiSpaceSizeMb,
  }
}

function killProcessTree(pid) {
  if (!pid || pid <= 0) {
    return
  }

  if (process.platform === 'win32') {
    spawnSync('taskkill.exe', ['/F', '/T', '/PID', String(pid)], {
      stdio: 'ignore',
    })
    return
  }

  try {
    process.kill(pid, 'SIGTERM')
  } catch {}
}

function readProcessRssMb(pid) {
  if (!pid || pid <= 0) {
    return 0
  }

  if (process.platform === 'win32') {
    const result = spawnSync(
      'powershell.exe',
      [
        '-NoProfile',
        '-Command',
        `$proc = Get-Process -Id ${pid} -ErrorAction SilentlyContinue; if ($proc) { [math]::Round($proc.WorkingSet64 / 1MB, 0) }`,
      ],
      {
        cwd: frontendDir,
        encoding: 'utf8',
      },
    )

    if (result.status !== 0) {
      return 0
    }

    return resolvePositiveInteger(result.stdout, 0)
  }

  const result = spawnSync('ps', ['-o', 'rss=', '-p', String(pid)], {
    cwd: frontendDir,
    encoding: 'utf8',
  })
  if (result.status !== 0) {
    return 0
  }

  const rssKb = resolvePositiveInteger(result.stdout, 0)
  return Math.round(rssKb / 1024)
}

function startMemoryWatcher(child, env, onHardLimitReached) {
  const rssWarnMb = resolvePositiveInteger(env.FRONTEND_NODE_RSS_WARN_MB, defaultRssWarnMb)
  const rssHardLimitMb = resolveNonNegativeInteger(
    env.FRONTEND_NODE_RSS_HARD_LIMIT_MB,
    defaultRssHardLimitMb,
  )
  const rssHardLimitHits = resolvePositiveInteger(
    env.FRONTEND_NODE_RSS_HARD_LIMIT_HITS,
    defaultRssHardLimitHits,
  )
  const pollMs = resolvePositiveInteger(env.FRONTEND_NODE_MEMORY_POLL_MS, defaultMemoryPollMs)
  let warnedAtMb = 0
  let overHardLimitHits = 0

  const timer = setInterval(() => {
    if (!child.pid || child.exitCode !== null) {
      return
    }

    const rssMb = readProcessRssMb(child.pid)
    if (rssMb <= 0) {
      return
    }

    if (rssMb >= rssWarnMb && (warnedAtMb === 0 || Math.abs(rssMb - warnedAtMb) >= 128)) {
      warnedAtMb = rssMb
      console.warn(`[dev] vite RSS is ${rssMb} MB (warning threshold: ${rssWarnMb} MB)`)
    }

    if (rssHardLimitMb > 0 && rssMb >= rssHardLimitMb) {
      overHardLimitHits += 1

      if (overHardLimitHits >= rssHardLimitHits) {
        console.error(
          `[dev] vite RSS reached ${rssMb} MB, exceeding hard limit ${rssHardLimitMb} MB for ${overHardLimitHits}/${rssHardLimitHits} checks. stopping Vite child.`,
        )
        try {
          onHardLimitReached?.({
            rssMb,
            rssHardLimitMb,
            hits: overHardLimitHits,
            requiredHits: rssHardLimitHits,
          })
        } catch {}
        killProcessTree(child.pid)
      } else {
        console.warn(
          `[dev] vite RSS reached ${rssMb} MB (hard limit ${rssHardLimitMb} MB), hit ${overHardLimitHits}/${rssHardLimitHits}. waiting before taking action.`,
        )
      }
      return
    }

    overHardLimitHits = 0
  }, pollMs)

  timer.unref?.()
  return timer
}

function main() {
  const requestedPort = resolveRequestedPort(process.env.FRONTEND_PORT, defaultVitePort)

  const childEnv = {
    ...process.env,
    FRONTEND_PORT: String(requestedPort),
  }

  ensureNativeRuntime(childEnv)

  const nodeArgs = resolveNodeArgs(childEnv)
  const rssHardLimitMb = resolveNonNegativeInteger(
    childEnv.FRONTEND_NODE_RSS_HARD_LIMIT_MB,
    defaultRssHardLimitMb,
  )
  const rssHardLimitHits = resolvePositiveInteger(
    childEnv.FRONTEND_NODE_RSS_HARD_LIMIT_HITS,
    defaultRssHardLimitHits,
  )
  const rssAutoRestartEnabled = resolveBoolean(
    childEnv.FRONTEND_NODE_RSS_AUTO_RESTART,
    defaultRssAutoRestart,
  )
  const rssRestartDelayMs = resolvePositiveInteger(
    childEnv.FRONTEND_NODE_RSS_RESTART_DELAY_MS,
    defaultRssRestartDelayMs,
  )
  const rssRestartMaxCount = resolvePositiveInteger(
    childEnv.FRONTEND_NODE_RSS_RESTART_MAX_COUNT,
    defaultRssRestartMaxCount,
  )
  const rssRestartWindowMs = resolvePositiveInteger(
    childEnv.FRONTEND_NODE_RSS_RESTART_WINDOW_MS,
    defaultRssRestartWindowMs,
  )
  const hardLimitDisplay = rssHardLimitMb > 0 ? `${rssHardLimitMb} MB` : 'disabled'
  const restartDisplay = rssAutoRestartEnabled
    ? `on(${rssRestartMaxCount}/${rssRestartWindowMs}ms delay=${rssRestartDelayMs}ms)`
    : 'off'

  console.log(
    `[dev] starting Vite on http://127.0.0.1:${requestedPort} with --max-old-space-size=${nodeArgs.maxOldSpaceSizeMb} MB --max-semi-space-size=${nodeArgs.maxSemiSpaceSizeMb} MB --rss-hard-limit=${hardLimitDisplay} --rss-hard-limit-hits=${rssHardLimitHits} --rss-auto-restart=${restartDisplay}`,
  )

  let shuttingDown = false
  let child = null
  let memoryWatcher = null
  let childKilledByRssLimit = false
  let restartTimer = null
  let rssRestartTimestamps = []

  const clearRestartTimer = () => {
    if (restartTimer) {
      clearTimeout(restartTimer)
      restartTimer = null
    }
  }

  const clearMemoryWatcher = () => {
    if (memoryWatcher) {
      clearInterval(memoryWatcher)
      memoryWatcher = null
    }
  }

  const canRestartAfterRssLimit = () => {
    if (!rssAutoRestartEnabled) {
      return false
    }

    const now = Date.now()
    rssRestartTimestamps = rssRestartTimestamps.filter((timestamp) => now - timestamp <= rssRestartWindowMs)
    if (rssRestartTimestamps.length >= rssRestartMaxCount) {
      return false
    }

    rssRestartTimestamps.push(now)
    return true
  }

  const shutdown = (exitCode = 0) => {
    if (shuttingDown) {
      return
    }

    shuttingDown = true
    clearRestartTimer()
    clearMemoryWatcher()

    if (child && child.pid && child.exitCode === null) {
      killProcessTree(child.pid)
    }

    process.exit(exitCode)
  }

  const launchViteChild = () => {
    childKilledByRssLimit = false
    child = spawn(nodeExecutable, nodeArgs.args, {
      cwd: frontendDir,
      // Keep stdin open for Vite. In detached Windows launches the inherited
      // stdin can close immediately, which makes Vite exit cleanly after boot.
      stdio: ['pipe', 'inherit', 'inherit'],
      env: childEnv,
    })

    memoryWatcher = startMemoryWatcher(child, childEnv, () => {
      childKilledByRssLimit = true
    })

    child.on('error', (error) => {
      console.error(`[dev] failed to start Vite: ${error instanceof Error ? error.message : String(error)}`)
      shutdown(1)
    })

    child.on('exit', (code, signal) => {
      clearMemoryWatcher()
      child = null

      if (shuttingDown) {
        return
      }

      if (childKilledByRssLimit) {
        if (!canRestartAfterRssLimit()) {
          console.error(
            `[dev] vite exceeded RSS hard limit repeatedly and auto restart budget is exhausted (${rssRestartMaxCount} times / ${rssRestartWindowMs}ms).`,
          )
          process.exit(1)
          return
        }

        console.warn(`[dev] restarting Vite after RSS hard-limit stop in ${rssRestartDelayMs}ms...`)
        restartTimer = setTimeout(() => {
          restartTimer = null
          if (!shuttingDown) {
            launchViteChild()
          }
        }, rssRestartDelayMs)
        restartTimer.unref?.()
        return
      }

      if (signal) {
        console.error(`[dev] vite exited with signal ${signal}`)
        process.exit(1)
        return
      }

      process.exit(code ?? 0)
    })
  }

  const handleSignal = (signal) => {
    console.log(`[dev] received ${signal}, stopping Vite...`)
    shutdown(0)
  }

  process.on('SIGINT', handleSignal)
  process.on('SIGTERM', handleSignal)
  process.on('exit', () => {
    clearRestartTimer()
    clearMemoryWatcher()

    if (child && child.pid && child.exitCode === null) {
      killProcessTree(child.pid)
    }
  })

  launchViteChild()
}

try {
  main()
} catch (error) {
  console.error(`[dev] ${error instanceof Error ? error.message : String(error)}`)
  process.exit(1)
}
