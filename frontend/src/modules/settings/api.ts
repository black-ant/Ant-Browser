// Settings 模块 API
import type { AppSettings } from './types'
import { defaultSettings } from './types'

// 本地存储 key
const SETTINGS_KEY = 'app_settings'

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

export interface AutomationSettings {
  enabled: boolean
  installPolicy: string
  runtimeVersion: string
  headlessDefault: boolean
  keepRuntimeOnDisable: boolean
  allowTypeScriptBuild: boolean
  artifactsDir: string
  nodeSource: string
  systemNodePath: string
  nodeVersion: string
  playwrightVersion: string
}

export interface AutomationRuntimeStatus {
  installed: boolean
  ready: boolean
  installing: boolean
  lastError: string
  runtimeDir: string
  nodePath: string
  nodeSource: string
  nodeResolution: string
  systemNodeDetected: boolean
  systemNodePath: string
  systemNodeError: string
  nodeVersion: string
  playwrightVersion: string
}

export interface AutomationState {
  settings: AutomationSettings
  status: AutomationRuntimeStatus
}

export type AutomationNodeSource = 'auto' | 'system' | 'bundled'

export interface AutomationRuntimeCheck {
  ok: boolean
  nodeSource: string
  nodeVersion: string
  playwrightVersion: string
}

export interface AutomationSystemNodeProbe {
  ok: boolean
  path: string
  version: string
}

export interface LaunchServerSettings {
  host: string
  port: number
  preferredPort: number
  baseUrl: string
  ready: boolean
}

export const defaultAutomationState: AutomationState = {
  settings: {
    enabled: false,
    installPolicy: 'on_demand',
    runtimeVersion: 'node-22.15.1-playwright-core-1.59.0',
    headlessDefault: false,
    keepRuntimeOnDisable: true,
    allowTypeScriptBuild: false,
    artifactsDir: 'data/automation/artifacts',
    nodeSource: 'auto',
    systemNodePath: '',
    nodeVersion: '22.15.1',
    playwrightVersion: '1.59.0',
  },
  status: {
    installed: false,
    ready: false,
    installing: false,
    lastError: '',
    runtimeDir: '',
    nodePath: '',
    nodeSource: 'auto',
    nodeResolution: '',
    systemNodeDetected: false,
    systemNodePath: '',
    systemNodeError: '',
    nodeVersion: '22.15.1',
    playwrightVersion: '1.59.0',
  },
}

export interface BackupActionResult {
  cancelled?: boolean
  message?: string
  zipPath?: string
  resetFirst?: boolean
  imported?: number
  skipped?: number
  conflicts?: number
  includedEntries?: number
  skippedEntries?: number
  fileCount?: number
  partial?: boolean
  componentTotal?: number
  componentSuccess?: number
  componentFailed?: number
  failedComponents?: Array<{
    componentId?: string
    componentName?: string
    error?: string
  }>
}

// 获取设置
export async function fetchSettings(): Promise<AppSettings> {
  try {
    const stored = localStorage.getItem(SETTINGS_KEY)
    if (stored) {
      return { ...defaultSettings, ...JSON.parse(stored) }
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  }
  return defaultSettings
}

// 保存设置
export async function saveSettings(settings: AppSettings): Promise<boolean> {
  try {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
    return true
  } catch (error) {
    console.error('Failed to save settings:', error)
    return false
  }
}

// 重置设置
export async function resetSettings(): Promise<AppSettings> {
  localStorage.removeItem(SETTINGS_KEY)
  return defaultSettings
}

export async function initializeSystemData(): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupInitializeSystem) {
    return { cancelled: false, message: '当前环境不支持后端初始化接口' }
  }
  return (await bindings.BackupInitializeSystem()) || {}
}

export async function exportSystemConfig(): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupExportPackage) {
    return { cancelled: false, message: '当前环境不支持后端导出接口' }
  }
  return (await bindings.BackupExportPackage()) || {}
}

export async function importSystemConfig(resetFirst: boolean): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupImportPackage) {
    return { cancelled: false, message: '当前环境不支持后端导入接口' }
  }
  return (await bindings.BackupImportPackage(resetFirst)) || {}
}

export async function fetchAutomationState(): Promise<AutomationState> {
  const bindings: any = await getBindings()
  if (!bindings?.GetAutomationState) {
    return defaultAutomationState
  }
  const raw = (await bindings.GetAutomationState()) || {}
  return {
    settings: {
      ...defaultAutomationState.settings,
      ...(raw.settings || {}),
    },
    status: {
      ...defaultAutomationState.status,
      ...(raw.status || {}),
    },
  }
}

export async function saveAutomationSettings(enabled: boolean, headlessDefault: boolean): Promise<AutomationState> {
  const bindings: any = await getBindings()
  if (!bindings?.SaveAutomationSettings) {
    return {
      ...defaultAutomationState,
      settings: {
        ...defaultAutomationState.settings,
        enabled,
        headlessDefault,
      },
    }
  }
  const raw = (await bindings.SaveAutomationSettings(enabled, headlessDefault)) || {}
  return {
    settings: {
      ...defaultAutomationState.settings,
      ...(raw.settings || {}),
    },
    status: {
      ...defaultAutomationState.status,
      ...(raw.status || {}),
    },
  }
}

export async function saveAutomationRuntimeSettings(
  nodeSource: AutomationNodeSource | string,
  systemNodePath: string
): Promise<AutomationState> {
  const bindings: any = await getBindings()
  if (!bindings?.SaveAutomationRuntimeSettings) {
    return {
      ...defaultAutomationState,
      settings: {
        ...defaultAutomationState.settings,
        nodeSource: String(nodeSource || defaultAutomationState.settings.nodeSource),
        systemNodePath: String(systemNodePath || '').trim(),
      },
    }
  }
  const raw = (await bindings.SaveAutomationRuntimeSettings(nodeSource, systemNodePath)) || {}
  return {
    settings: {
      ...defaultAutomationState.settings,
      ...(raw.settings || {}),
    },
    status: {
      ...defaultAutomationState.status,
      ...(raw.status || {}),
    },
  }
}

export async function saveAutomationScriptPackageSettings(
  allowTypeScriptBuild: boolean
): Promise<AutomationState> {
  const bindings: any = await getBindings()
  if (!bindings?.SaveAutomationScriptPackageSettings) {
    return {
      ...defaultAutomationState,
      settings: {
        ...defaultAutomationState.settings,
        allowTypeScriptBuild,
      },
    }
  }
  const raw = (await bindings.SaveAutomationScriptPackageSettings(allowTypeScriptBuild)) || {}
  return {
    settings: {
      ...defaultAutomationState.settings,
      ...(raw.settings || {}),
    },
    status: {
      ...defaultAutomationState.status,
      ...(raw.status || {}),
    },
  }
}

export async function installAutomationRuntime(): Promise<AutomationState> {
  const bindings: any = await getBindings()
  if (!bindings?.InstallAutomationRuntime) {
    return defaultAutomationState
  }
  const raw = (await bindings.InstallAutomationRuntime()) || {}
  return {
    settings: {
      ...defaultAutomationState.settings,
      ...(raw.settings || {}),
    },
    status: {
      ...defaultAutomationState.status,
      ...(raw.status || {}),
    },
  }
}

export async function automationProbeSystemNode(systemNodePath: string): Promise<AutomationSystemNodeProbe> {
  const bindings: any = await getBindings()
  if (!bindings?.AutomationProbeSystemNode) {
    return { ok: false, path: '', version: '' }
  }
  return (await bindings.AutomationProbeSystemNode(systemNodePath)) || { ok: false, path: '', version: '' }
}

export async function automationRuntimeSelfCheck(): Promise<AutomationRuntimeCheck> {
  const bindings: any = await getBindings()
  if (!bindings?.AutomationRuntimeSelfCheck) {
    return { ok: false, nodeSource: '', nodeVersion: '', playwrightVersion: '' }
  }
  return (await bindings.AutomationRuntimeSelfCheck()) || { ok: false, nodeSource: '', nodeVersion: '', playwrightVersion: '' }
}

function normalizeLaunchServerSettings(payload: any): LaunchServerSettings {
  const host = String(payload?.host || '127.0.0.1')
  const port = Number(payload?.port) || 0
  const preferredPort = Number(payload?.preferredPort) || port || 19876
  const effectivePort = port > 0 ? port : preferredPort
  return {
    host,
    port: effectivePort,
    preferredPort,
    baseUrl: String(payload?.baseUrl || (effectivePort > 0 ? `http://${host}:${effectivePort}` : '')),
    ready: !!payload?.ready && port > 0,
  }
}

export async function fetchLaunchServerSettings(): Promise<LaunchServerSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.GetLaunchServerInfo) {
    return normalizeLaunchServerSettings(null)
  }
  return normalizeLaunchServerSettings(await bindings.GetLaunchServerInfo())
}

export async function saveLaunchServerSettings(port: number): Promise<LaunchServerSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.SaveLaunchServerSettings) {
    return normalizeLaunchServerSettings({ port, preferredPort: port, ready: false })
  }
  return normalizeLaunchServerSettings(await bindings.SaveLaunchServerSettings(port))
}
