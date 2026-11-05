const MUTATION_METHOD_PATTERN = /(Create|Update|Delete|Save|Set|Import|Export|Install|Uninstall|Download|Start|Stop|Launch|Run|Clear|Move|Copy|Refresh|Sync|Validate|Toggle|Switch|Enable|Disable|Force|Quit|Open|Close|Apply|Bind|Unbind|Restart|Warmup|Probe|Check|Test)/
const SKIP_METHODS = new Set(['GetAppLogs', 'FrontendOperationLog'])

let installed = false

function stringifyError(error: unknown) {
  if (error instanceof Error) return error.message || String(error)
  if (typeof error === 'string') return error
  try {
    return JSON.stringify(error)
  } catch {
    return String(error)
  }
}

function shouldLogFailure(method: string) {
  return !SKIP_METHODS.has(method)
}

function recordOperation(level: 'debug' | 'info' | 'error', method: string, success: boolean, durationMs: number, message = '') {
  const app = (window as any)?.go?.main?.App
  const logger = app?.FrontendOperationLog
  if (typeof logger !== 'function') return
  try {
    void logger(level, method, success, Math.max(0, Math.round(durationMs)), String(message || '').slice(0, 1200))
  } catch {
    // Logging must never break user operations.
  }
}

function successLogLevel(method: string): 'debug' | 'info' | null {
  if (SKIP_METHODS.has(method)) return null
  return MUTATION_METHOD_PATTERN.test(method) ? 'info' : 'debug'
}

export function installWailsOperationLogger() {
  if (installed) return
  installed = true

  const app = (window as any)?.go?.main?.App
  if (!app || typeof app !== 'object') return

  Object.keys(app).forEach((method) => {
    const original = app[method]
    if (typeof original !== 'function' || SKIP_METHODS.has(method)) return

    app[method] = (...args: unknown[]) => {
      const startedAt = performance.now()
      try {
        const result = original(...args)
        if (result && typeof result.then === 'function') {
          return result.then((value: unknown) => {
            const level = successLogLevel(method)
            if (level) {
              recordOperation(level, method, true, performance.now() - startedAt)
            }
            return value
          }).catch((error: unknown) => {
            if (shouldLogFailure(method)) {
              recordOperation('error', method, false, performance.now() - startedAt, stringifyError(error))
            }
            throw error
          })
        }
        const level = successLogLevel(method)
        if (level) {
          recordOperation(level, method, true, performance.now() - startedAt)
        }
        return result
      } catch (error) {
        if (shouldLogFailure(method)) {
          recordOperation('error', method, false, performance.now() - startedAt, stringifyError(error))
        }
        throw error
      }
    }
  })
}
