import type { EnvironmentFailureItem, EnvironmentStatus, ReleaseUpdateState } from './types'

type RuntimeBindings = {
  GetAppConfig?: () => Promise<any>
  GetDesktopEnvironmentStatus?: () => Promise<any>
  RepairDesktopEnvironment?: () => Promise<any>
  CheckDesktopReleaseUpdate?: () => Promise<any>
  ApplyDesktopReleaseUpdate?: () => Promise<any>
  ExportDesktopEnvironmentDiagnostics?: () => Promise<string>
}

const devEnvironmentStatus: EnvironmentStatus = {
  state: 'pass',
  items: [],
}

const devReleaseUpdateState: ReleaseUpdateState = {
  kind: 'none',
  localAppVersion: '',
  remoteAppVersion: '',
  resourceVersion: '',
  manifestSource: '',
  manifestUrl: '',
}

async function getBindings(): Promise<RuntimeBindings | null> {
  const fallback = ((window as any)?.go?.main?.App as RuntimeBindings | undefined) ?? null
  if (!fallback) return null

  try {
    const module: any = await import('../../wailsjs/go/main/App')
    return {
      ...(fallback || {}),
      ...(module as RuntimeBindings),
    }
  } catch {
    return fallback
  }
}

function normalizeFailureItem(input: any): EnvironmentFailureItem {
  const severity = String(input?.severity || 'error') as EnvironmentFailureItem['severity']

  return {
    code: String(input?.code || ''),
    severity: severity === 'info' || severity === 'warning' || severity === 'error' ? severity : 'error',
    message: String(input?.message || ''),
    repairable: Boolean(input?.repairable),
    recommendedAction: String(input?.recommendedAction || ''),
    details:
      input?.details && typeof input.details === 'object'
        ? Object.fromEntries(Object.entries(input.details).map(([key, value]) => [String(key), String(value ?? '').trim()]))
        : {},
  }
}

function normalizeEnvironmentStatus(input: any): EnvironmentStatus {
  const state = String(input?.state || 'blocked') as EnvironmentStatus['state']
  const items = Array.isArray(input?.items) ? input.items.map(normalizeFailureItem) : []

  return {
    state: state === 'checking' || state === 'pass' || state === 'repairable' || state === 'blocked' ? state : 'blocked',
    items,
  }
}

function normalizeReleaseUpdateState(input: any): ReleaseUpdateState {
  const kind = String(input?.kind || 'none') as ReleaseUpdateState['kind']

  return {
    kind: kind === 'soft' || kind === 'required' || kind === 'none' ? kind : 'none',
    localAppVersion: String(input?.localAppVersion || ''),
    remoteAppVersion: String(input?.remoteAppVersion || ''),
    resourceVersion: String(input?.resourceVersion || ''),
    manifestSource: String(input?.manifestSource || ''),
    manifestUrl: String(input?.manifestUrl || ''),
  }
}

async function requireBinding<K extends keyof RuntimeBindings>(name: K): Promise<NonNullable<RuntimeBindings[K]>> {
  const bindings = await getBindings()
  const fn = bindings?.[name]
  if (!fn) {
    throw new Error(`当前环境缺少 ${String(name)} 绑定`)
  }
  return fn as NonNullable<RuntimeBindings[K]>
}

export async function getDesktopEnvironmentStatus(): Promise<EnvironmentStatus> {
  const bindings = await getBindings()
  const fn = bindings?.GetDesktopEnvironmentStatus
  if (!fn) return devEnvironmentStatus
  return normalizeEnvironmentStatus(await fn())
}

export async function getAppConfig(): Promise<{ name: string; version: string }> {
  const bindings = await getBindings()
  const fn = bindings?.GetAppConfig
  if (!fn) {
    return {
      name: 'Maka Browser',
      version: 'dev',
    }
  }
  const config = await fn()
  return {
    name: String(config?.name || '').trim(),
    version: String(config?.version || '').trim(),
  }
}

export async function repairDesktopEnvironment(): Promise<EnvironmentStatus> {
  const fn = await requireBinding('RepairDesktopEnvironment')
  return normalizeEnvironmentStatus(await fn())
}

export async function checkDesktopReleaseUpdate(): Promise<ReleaseUpdateState> {
  const bindings = await getBindings()
  const fn = bindings?.CheckDesktopReleaseUpdate
  if (!fn) return devReleaseUpdateState
  return normalizeReleaseUpdateState(await fn())
}

export async function applyDesktopReleaseUpdate(): Promise<ReleaseUpdateState> {
  const fn = await requireBinding('ApplyDesktopReleaseUpdate')
  return normalizeReleaseUpdateState(await fn())
}

export async function exportDesktopEnvironmentDiagnostics(): Promise<string> {
  const fn = await requireBinding('ExportDesktopEnvironmentDiagnostics')
  return String((await fn()) || '').trim()
}
