import { getBindings } from './runtime'

// —— 身份池模板类型(对应后端 identity.PoolRecord)——
export interface IdentityScreen {
  width: number
  height: number
  dpr: number
  colorDepth: number
}

export interface IdentityPoolRecord {
  id?: string
  platform: string
  platformVersion?: string
  brandVersion: string
  uaFull: string
  hardwareConcurrency: number
  deviceMemory: number
  screen: IdentityScreen
  windowSize: string
  languages: string[]
  locale: string
  timezone: string
  weight: number
}

export interface IdentityIssue {
  field: string
  message: string
  severity: string
  fixable: boolean
}

export interface IdentityValidationResult {
  ok: boolean
  issues: IdentityIssue[]
}

export interface IdentityPoolProblem {
  id: string
  uaFull: string
  platform: string
  issues: IdentityIssue[]
}

export interface IdentityValidateAllReport {
  total: number
  okCount: number
  badCount: number
  problems: IdentityPoolProblem[]
}

export function emptyIdentityRecord(): IdentityPoolRecord {
  return {
    platform: 'windows',
    platformVersion: '',
    brandVersion: '',
    uaFull: '',
    hardwareConcurrency: 8,
    deviceMemory: 8,
    screen: { width: 1920, height: 1080, dpr: 1, colorDepth: 24 },
    windowSize: '1920,1040',
    languages: ['en-US', 'en'],
    locale: 'en-US',
    timezone: 'America/New_York',
    weight: 1,
  }
}

// —— 无绑定(浏览器预览)时的 mock ——
let mockPool: IdentityPoolRecord[] = [
  { id: 'mock-1', ...emptyIdentityRecord(), brandVersion: '147.0.0.0', uaFull: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36' },
]

export async function fetchIdentityPool(): Promise<IdentityPoolRecord[]> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolList) {
    return (await bindings.IdentityPoolList()) || []
  }
  return mockPool
}

export async function createIdentity(rec: IdentityPoolRecord): Promise<IdentityPoolRecord | null> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolCreate) {
    return (await bindings.IdentityPoolCreate(rec)) || null
  }
  const created = { ...rec, id: `mock-${Date.now()}` }
  mockPool = [...mockPool, created]
  return created
}

export async function updateIdentity(id: string, rec: IdentityPoolRecord): Promise<IdentityPoolRecord | null> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolUpdate) {
    return (await bindings.IdentityPoolUpdate(id, rec)) || null
  }
  const next = { ...rec, id }
  mockPool = mockPool.map(r => (r.id === id ? next : r))
  return next
}

export async function deleteIdentity(id: string): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolDelete) {
    await bindings.IdentityPoolDelete(id)
    return
  }
  mockPool = mockPool.filter(r => r.id !== id)
}

export async function validateIdentity(rec: IdentityPoolRecord): Promise<IdentityValidationResult | null> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolValidate) {
    return (await bindings.IdentityPoolValidate(rec)) || null
  }
  return { ok: true, issues: [] }
}

export async function validateAllIdentities(): Promise<IdentityValidateAllReport | null> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolValidateAll) {
    return (await bindings.IdentityPoolValidateAll()) || null
  }
  return { total: mockPool.length, okCount: mockPool.length, badCount: 0, problems: [] }
}

export async function restoreDefaultIdentities(): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.IdentityPoolRestoreDefaults) {
    await bindings.IdentityPoolRestoreDefaults()
  }
}
