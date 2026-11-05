import { createBrowserProfile, deleteBrowserProfile } from './profiles'
import { fetchLaunchServerInfo } from './launch'
import { getBindings, getGoApp, getMockProfiles, nowISOString } from './runtime'
import { startBrowserInstance, stopBrowserInstance } from './instances'

export interface AutomationDemoResult {
  ok: boolean
  status: number
  method: string
  path: string
  baseUrl: string
  requestedAt: string
  error: string
  requestedCode: string
  profileId: string
  profileName: string
  launchCode: string
  cdpUrl: string
  debugPort: number
  created: boolean
  launched: boolean
  deleted: boolean
  stoppedBeforeDelete: boolean
  stopError: string
  authHeader: string
  response: Record<string, any>
}

export interface AutomationDemoCreateOptions {
  profileName?: string
  launchCode?: string
  startUrl?: string
  launchArgs?: string[]
  skipDefaultStartUrls?: boolean
  autoLaunch?: boolean
}

function normalizeAutomationDemoResult(raw: any, fallback: Partial<AutomationDemoResult> = {}): AutomationDemoResult {
  const response =
    raw?.response && typeof raw.response === 'object' && !Array.isArray(raw.response)
      ? (raw.response as Record<string, any>)
      : (fallback.response || {})

  const status = Number(raw?.status ?? fallback.status ?? 200) || 0
  const ok =
    raw?.ok !== undefined
      ? !!raw.ok
      : (fallback.ok !== undefined ? !!fallback.ok : status >= 200 && status < 300 && response.ok !== false)

  return {
    ok,
    status,
    method: String(raw?.method || fallback.method || 'GET'),
    path: String(raw?.path || fallback.path || ''),
    baseUrl: String(raw?.baseUrl || fallback.baseUrl || ''),
    requestedAt: String(raw?.requestedAt || fallback.requestedAt || nowISOString()),
    error: String(raw?.error || fallback.error || ''),
    requestedCode: String(raw?.requestedCode || fallback.requestedCode || ''),
    profileId: String(raw?.profileId || fallback.profileId || ''),
    profileName: String(raw?.profileName || fallback.profileName || ''),
    launchCode: String(raw?.launchCode || fallback.launchCode || ''),
    cdpUrl: String(raw?.cdpUrl || fallback.cdpUrl || ''),
    debugPort: Number(raw?.debugPort ?? fallback.debugPort ?? 0) || 0,
    created: raw?.created !== undefined ? !!raw.created : !!fallback.created,
    launched: raw?.launched !== undefined ? !!raw.launched : !!fallback.launched,
    deleted: raw?.deleted !== undefined ? !!raw.deleted : !!fallback.deleted,
    stoppedBeforeDelete: raw?.stoppedBeforeDelete !== undefined ? !!raw.stoppedBeforeDelete : !!fallback.stoppedBeforeDelete,
    stopError: String(raw?.stopError || fallback.stopError || ''),
    authHeader: String(raw?.authHeader || fallback.authHeader || ''),
    response,
  }
}

async function callAutomationDemoBinding(methodName: string, args: any[] = []): Promise<any | null> {
  const bindings: any = await getBindings()
  if (typeof bindings?.[methodName] === 'function') {
    return await bindings[methodName](...args)
  }

  const goApp = getGoApp()
  if (typeof goApp?.[methodName] === 'function') {
    return await goApp[methodName](...args)
  }

  return null
}

function nextMockDemoCode(): string {
  const token = Date.now().toString(36).replace(/[^a-z0-9]/gi, '').toUpperCase().slice(-6).padStart(6, '0')
  return `DEMO_${token}`
}

function normalizeAutomationDemoCreateOptions(input: AutomationDemoCreateOptions = {}): AutomationDemoCreateOptions {
  const launchArgs = Array.isArray(input.launchArgs)
    ? input.launchArgs
        .map((item) => String(item || '').trim())
        .filter(Boolean)
    : []

  return {
    profileName: String(input.profileName || '').trim(),
    launchCode: String(input.launchCode || '').trim().toUpperCase(),
    startUrl: String(input.startUrl || '').trim(),
    launchArgs,
    skipDefaultStartUrls: input.skipDefaultStartUrls === true,
    autoLaunch: input.autoLaunch === true,
  }
}

export async function automationDemoHealthCheck(): Promise<AutomationDemoResult> {
  const raw = await callAutomationDemoBinding('AutomationDemoHealthCheck')
  if (raw !== null) {
    return normalizeAutomationDemoResult(raw, {
      method: 'GET',
      path: '/api/health',
      response: { ok: true },
    })
  }

  const info = await fetchLaunchServerInfo()
  return normalizeAutomationDemoResult({
    ok: true,
    status: 200,
    method: 'GET',
    path: '/api/health',
    baseUrl: info.baseUrl,
    response: { ok: true },
  })
}

export async function automationDemoCreateProfile(options: AutomationDemoCreateOptions = {}): Promise<AutomationDemoResult> {
  const normalizedOptions = normalizeAutomationDemoCreateOptions(options)

  let raw = null
  if (
    normalizedOptions.profileName ||
    normalizedOptions.launchCode ||
    normalizedOptions.startUrl ||
    normalizedOptions.launchArgs?.length ||
    normalizedOptions.skipDefaultStartUrls ||
    normalizedOptions.autoLaunch
  ) {
    raw = await callAutomationDemoBinding('AutomationDemoCreateProfileWithOptions', [JSON.stringify(normalizedOptions)])
  }
  if (raw === null) {
    raw = await callAutomationDemoBinding('AutomationDemoCreateProfile')
  }
  if (raw !== null) {
    return normalizeAutomationDemoResult(raw, {
      method: 'POST',
      path: '/api/profiles',
    })
  }

  const launchCode = normalizedOptions.launchCode || nextMockDemoCode()
  const profileName = normalizedOptions.profileName || `自动化 Demo ${launchCode}`
  const profile = await createBrowserProfile({
    profileName,
    userDataDir: `automation-demo-${launchCode.toLowerCase()}`,
    coreId: '',
    fingerprintArgs: [],
    proxyId: '',
    proxyConfig: '',
    memoryLimitMb: 0,
    launchArgs: normalizedOptions.launchArgs || [],
    tags: ['自动化', 'Demo'],
    keywords: ['automation-demo', 'launch-api-demo'],
    groupId: '',
  })
  if (!profile) {
    return normalizeAutomationDemoResult({
      ok: false,
      status: 500,
      method: 'POST',
      path: '/api/profiles',
      error: 'mock create profile failed',
      response: { ok: false, error: 'mock create profile failed' },
    })
  }

  profile.launchCode = launchCode
  let debugPort = 0
  let cdpUrl = ''
  let launched = false
  if (normalizedOptions.autoLaunch) {
    const started = await startBrowserInstance(profile.profileId)
    debugPort = started?.debugPort || 9222
    cdpUrl = `http://127.0.0.1:${debugPort}`
    launched = !!started
  }

  return normalizeAutomationDemoResult({
    ok: true,
    status: 201,
    method: 'POST',
    path: '/api/profiles',
    profileId: profile.profileId,
    profileName,
    launchCode,
    created: true,
    launched,
    debugPort,
    cdpUrl,
    response: {
      ok: true,
      created: true,
      profileId: profile.profileId,
      profileName,
      launchCode,
      launched,
      debugPort,
      cdpUrl,
      profile,
    },
  })
}

export async function automationDemoLaunchProfile(code: string): Promise<AutomationDemoResult> {
  const normalizedCode = code.trim().toUpperCase()
  const raw = await callAutomationDemoBinding('AutomationDemoLaunchProfile', [normalizedCode])
  if (raw !== null) {
    return normalizeAutomationDemoResult(raw, {
      method: 'POST',
      path: '/api/launch',
      requestedCode: normalizedCode,
    })
  }

  const profile = getMockProfiles().find((item) => (item.launchCode || '').trim().toUpperCase() === normalizedCode)
  if (!profile) {
    return normalizeAutomationDemoResult({
      ok: false,
      status: 404,
      method: 'POST',
      path: '/api/launch',
      requestedCode: normalizedCode,
      error: 'launch code not found',
      response: { ok: false, error: 'launch code not found' },
    })
  }

  const started = await startBrowserInstance(profile.profileId)
  const debugPort = started?.debugPort || 9222
  return normalizeAutomationDemoResult({
    ok: true,
    status: 200,
    method: 'POST',
    path: '/api/launch',
    requestedCode: normalizedCode,
    profileId: profile.profileId,
    profileName: profile.profileName,
    launchCode: normalizedCode,
    cdpUrl: `http://127.0.0.1:${debugPort}`,
    debugPort,
    launched: true,
    response: {
      ok: true,
      profileId: profile.profileId,
      profileName: profile.profileName,
      launchCode: normalizedCode,
      cdpUrl: `http://127.0.0.1:${debugPort}`,
      debugPort,
      launched: true,
    },
  })
}

export async function automationDemoDeleteProfile(profileId: string): Promise<AutomationDemoResult> {
  const normalizedProfileID = profileId.trim()
  const raw = await callAutomationDemoBinding('AutomationDemoDeleteProfile', [normalizedProfileID])
  if (raw !== null) {
    return normalizeAutomationDemoResult(raw, {
      method: 'DELETE',
      path: `/api/profiles/${normalizedProfileID}`,
      profileId: normalizedProfileID,
    })
  }

  const profile = getMockProfiles().find((item) => item.profileId === normalizedProfileID)
  if (!profile) {
    return normalizeAutomationDemoResult({
      ok: false,
      status: 404,
      method: 'DELETE',
      path: `/api/profiles/${normalizedProfileID}`,
      profileId: normalizedProfileID,
      error: 'profile not found',
      response: { ok: false, error: 'profile not found' },
    })
  }

  let stoppedBeforeDelete = false
  if (profile.running) {
    await stopBrowserInstance(normalizedProfileID)
    stoppedBeforeDelete = true
  }
  await deleteBrowserProfile(normalizedProfileID)

  return normalizeAutomationDemoResult({
    ok: true,
    status: 200,
    method: 'DELETE',
    path: `/api/profiles/${normalizedProfileID}`,
    profileId: normalizedProfileID,
    profileName: profile.profileName,
    launchCode: profile.launchCode || '',
    deleted: true,
    stoppedBeforeDelete,
    response: {
      ok: true,
      deleted: true,
      profileId: normalizedProfileID,
      profileName: profile.profileName,
      launchCode: profile.launchCode || '',
    },
  })
}
