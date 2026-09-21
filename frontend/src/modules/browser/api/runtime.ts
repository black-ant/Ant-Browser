import type { BrowserCore, BrowserProfile, BrowserProxy, BrowserSettings } from '../types'

export async function getBindings() {
  if (!(globalThis as any).go?.main?.App) {
    return null
  }
  try {
    return await import('../../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

export function getGoApp(): any {
  return (globalThis as any).go?.main?.App ?? null
}

export function nowISOString(): string {
  return new Date().toISOString()
}

export function createDefaultBrowserSettings(): BrowserSettings {
  return {
    userDataRoot: 'data',
    defaultFingerprintArgs: [],
    defaultLaunchArgs: [],
    defaultStartUrls: [],
    lightStartEnabled: true,
    restoreLastSession: true,
    startReadyTimeoutMs: 3000,
    startStableWindowMs: 1200,
    defaultConnectorType: 'xray',
  }
}

let mockProfiles: BrowserProfile[] = [
  {
    profileId: 'mock-1',
    profileName: '默认指纹配置',
    userDataDir: 'data/default',
    coreId: 'default',
    fingerprintArgs: ['--fingerprint-brand=Chrome', '--fingerprint-platform=windows'],
    proxyId: '',
    proxyConfig: '',
    memoryLimitMb: 0,
    launchArgs: ['--disable-features=Translate'],
    tags: ['默认'],
    keywords: [],
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: nowISOString(),
    updatedAt: nowISOString(),
  },
]

let mockCores: BrowserCore[] = []
let mockProxies: BrowserProxy[] = []

export function getMockProfiles(): BrowserProfile[] {
  return mockProfiles
}

export function setMockProfiles(next: BrowserProfile[]): void {
  mockProfiles = next
}

export function getMockCores(): BrowserCore[] {
  return mockCores
}

export function setMockCores(next: BrowserCore[]): void {
  mockCores = next
}

export function getMockProxies(): BrowserProxy[] {
  return mockProxies
}

export function setMockProxies(next: BrowserProxy[]): void {
  mockProxies = next
}
