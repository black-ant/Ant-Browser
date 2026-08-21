import type { BrowserProfile, BrowserTab } from '../types'
import { getBindings, getMockProfiles, nowISOString, setMockProfiles } from './runtime'

export async function startBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStart) {
    return (await bindings.BrowserInstanceStart(profileId)) || null
  }

  const nextProfiles = getMockProfiles().map((item) =>
    item.profileId === profileId
      ? {
          ...item,
          running: true,
          debugPort: 9222,
          debugReady: true,
          pid: Math.floor(Math.random() * 100000),
          runtimeWarning: '',
          lastStartAt: nowISOString(),
        }
      : item,
  )
  setMockProfiles(nextProfiles)
  return nextProfiles.find((item) => item.profileId === profileId) || null
}

export async function startBrowserInstanceDirect(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStartDirect) {
    return (await bindings.BrowserInstanceStartDirect(profileId)) || null
  }
  return startBrowserInstance(profileId)
}

export async function startBrowserInstanceByCode(code: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStartByCode) {
    return (await bindings.BrowserInstanceStartByCode(code)) || null
  }

  const normalized = code.trim().toUpperCase()
  const profile = getMockProfiles().find((item) => (item.launchCode || '').toUpperCase() === normalized)
  if (!profile) {
    throw new Error('launch code not found')
  }
  return startBrowserInstance(profile.profileId)
}

export async function openBrowserFingerprintCheck(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceOpenFingerprintCheck) {
    return (await bindings.BrowserInstanceOpenFingerprintCheck(profileId)) || null
  }
  return startBrowserInstance(profileId)
}

export async function stopBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStop) {
    return (await bindings.BrowserInstanceStop(profileId)) || null
  }

  const nextProfiles = getMockProfiles().map((item) =>
    item.profileId === profileId
      ? { ...item, running: false, debugReady: false, debugPort: 0, pid: 0, runtimeWarning: '', lastStopAt: nowISOString() }
      : item,
  )
  setMockProfiles(nextProfiles)
  return nextProfiles.find((item) => item.profileId === profileId) || null
}

export async function restartBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceRestart) {
    return (await bindings.BrowserInstanceRestart(profileId)) || null
  }
  await stopBrowserInstance(profileId)
  return startBrowserInstance(profileId)
}

export async function openBrowserUrl(profileId: string, targetUrl: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceOpenUrl) {
    return (await bindings.BrowserInstanceOpenUrl(profileId, targetUrl)) === true
  }
  return true
}

export async function fetchBrowserTabs(profileId: string): Promise<BrowserTab[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceGetTabs) {
    return (await bindings.BrowserInstanceGetTabs(profileId)) || []
  }
  return [
    { tabId: 'tab-1', title: '新标签页', url: 'about:blank', active: true },
    { tabId: 'tab-2', title: '示例站点', url: 'https://example.com', active: false },
  ]
}

export async function openUserDataDir(userDataDir: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenUserDataDir) {
    await bindings.OpenUserDataDir(userDataDir)
    return true
  }
  return false
}

export async function openUserDataRoot(): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenUserDataRoot) {
    await bindings.OpenUserDataRoot()
    return true
  }
  return false
}

export interface WindowTileResult {
  total: number
  tiled: number
  failed: number
  errors: string[]
}

// 一键平铺运行中实例的窗口；profileIds 为空表示全部运行中实例
export async function tileBrowserWindows(profileIds: string[]): Promise<WindowTileResult> {
  // 优先用 wails 运行时注入的 window.go（由后端反射生成，永远与当前二进制同步）；
  // 静态 wailsjs 绑定文件在 -skipbindings 构建下会滞后于新增 Go 方法，只作兜底。
  const goApp = (globalThis as any).go?.main?.App
  if (goApp?.BrowserWindowsTile) {
    return await goApp.BrowserWindowsTile(profileIds)
  }
  const bindings: any = await getBindings()
  if (bindings?.BrowserWindowsTile) {
    return await bindings.BrowserWindowsTile(profileIds)
  }
  const hasGo = !!goApp
  throw new Error(
    `当前环境不支持窗口平铺（运行中的程序版本过旧，请安装最新安装包后重试；go=${hasGo ? '1' : '0'}）`,
  )
}
