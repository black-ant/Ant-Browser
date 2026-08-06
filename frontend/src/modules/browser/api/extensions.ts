import type { BrowserExtension, BrowserExtensionLookupResult, BrowserProfileExtensionSettings } from '../types'
import { getBindings, getGoApp } from './runtime'

export interface BrowserExtensionManualInstallGuide {
  extensionId: string
  storeUrl: string
  downloadUrl: string
  downloadDir: string
  fileName: string
}

export interface BrowserExtensionManualDownloadFile {
  fileName: string
  filePath: string
  sizeBytes: number
  updatedAt: string
}

function normalizeExtension(payload: any): BrowserExtension {
  return {
    extensionId: String(payload?.extensionId || ''),
    name: String(payload?.name || ''),
    version: String(payload?.version || ''),
    description: String(payload?.description || ''),
    iconDataUrl: String(payload?.iconDataUrl || ''),
    manifestJson: String(payload?.manifestJson || ''),
    sourceUrl: String(payload?.sourceUrl || ''),
    installDir: String(payload?.installDir || ''),
    installMode: String(payload?.installMode || 'persistent'),
    packagePath: String(payload?.packagePath || ''),
    packageHash: String(payload?.packageHash || ''),
    enabled: payload?.enabled !== false,
    installedAt: String(payload?.installedAt || ''),
    updatedAt: String(payload?.updatedAt || ''),
  }
}

function normalizeLookup(payload: any): BrowserExtensionLookupResult {
  return {
    extensionId: String(payload?.extensionId || ''),
    name: String(payload?.name || ''),
    version: String(payload?.version || ''),
    description: String(payload?.description || ''),
    storeUrl: String(payload?.storeUrl || ''),
    installable: payload?.installable === true,
    message: String(payload?.message || ''),
  }
}

function normalizeManualInstallGuide(payload: any): BrowserExtensionManualInstallGuide {
  return {
    extensionId: String(payload?.extensionId || ''),
    storeUrl: String(payload?.storeUrl || ''),
    downloadUrl: String(payload?.downloadUrl || ''),
    downloadDir: String(payload?.downloadDir || ''),
    fileName: String(payload?.fileName || ''),
  }
}

function normalizeManualDownloadFile(payload: any): BrowserExtensionManualDownloadFile {
  return {
    fileName: String(payload?.fileName || ''),
    filePath: String(payload?.filePath || ''),
    sizeBytes: Number(payload?.sizeBytes || 0),
    updatedAt: String(payload?.updatedAt || ''),
  }
}

export async function fetchBrowserExtensions(): Promise<BrowserExtension[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionList) {
    const result = await bindings.BrowserExtensionList()
    return Array.isArray(result) ? result.map(normalizeExtension) : []
  }
  return []
}

export async function lookupBrowserExtension(query: string, proxyConfig = '', useProxy = false): Promise<BrowserExtensionLookupResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionLookupWithProxy) {
    return normalizeLookup(await bindings.BrowserExtensionLookupWithProxy({ query, useProxy, proxyConfig }))
  }
  if (useProxy) throw new Error('当前后端版本不支持插件下载代理，请重启或更新应用')
  if (bindings?.BrowserExtensionLookup) {
    return normalizeLookup(await bindings.BrowserExtensionLookup(query))
  }
  throw new Error('当前环境不支持插件查询')
}

export async function installBrowserExtension(query: string, proxyConfig = '', useProxy = false): Promise<BrowserExtension> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionInstallWithProxy) {
    return normalizeExtension(await bindings.BrowserExtensionInstallWithProxy({ query, useProxy, proxyConfig }))
  }
  if (useProxy) throw new Error('当前后端版本不支持插件下载代理，请重启或更新应用')
  if (bindings?.BrowserExtensionInstall) {
    return normalizeExtension(await bindings.BrowserExtensionInstall(query))
  }
  throw new Error('当前环境不支持插件安装')
}

export async function getBrowserExtensionManualInstallGuide(query: string): Promise<BrowserExtensionManualInstallGuide> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionManualInstallGuide) {
    return normalizeManualInstallGuide(await bindings.BrowserExtensionManualInstallGuide(query))
  }
  const goApp = getGoApp()
  if (goApp?.BrowserExtensionManualInstallGuide) {
    return normalizeManualInstallGuide(await goApp.BrowserExtensionManualInstallGuide(query))
  }
  throw new Error('当前环境不支持手动安装指南')
}

export async function openBrowserExtensionManualDownloadDir(): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionOpenManualDownloadDir) {
    await bindings.BrowserExtensionOpenManualDownloadDir()
    return
  }
  const goApp = getGoApp()
  if (goApp?.BrowserExtensionOpenManualDownloadDir) {
    await goApp.BrowserExtensionOpenManualDownloadDir()
    return
  }
  throw new Error('当前环境不支持打开下载目录')
}

export async function listBrowserExtensionManualDownloadFiles(): Promise<BrowserExtensionManualDownloadFile[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionListManualDownloadFiles) {
    const result = await bindings.BrowserExtensionListManualDownloadFiles()
    return Array.isArray(result) ? result.map(normalizeManualDownloadFile) : []
  }
  const goApp = getGoApp()
  if (goApp?.BrowserExtensionListManualDownloadFiles) {
    const result = await goApp.BrowserExtensionListManualDownloadFiles()
    return Array.isArray(result) ? result.map(normalizeManualDownloadFile) : []
  }
  return []
}

export async function installBrowserExtensionManualDownloadFile(fileName: string): Promise<BrowserExtension> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionInstallManualDownloadFile) {
    return normalizeExtension(await bindings.BrowserExtensionInstallManualDownloadFile(fileName))
  }
  const goApp = getGoApp()
  if (goApp?.BrowserExtensionInstallManualDownloadFile) {
    return normalizeExtension(await goApp.BrowserExtensionInstallManualDownloadFile(fileName))
  }
  throw new Error('当前环境不支持导入手动下载插件包')
}

export async function installBrowserExtensionLocalFile(): Promise<BrowserExtension> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionInstallLocalFile) {
    return normalizeExtension(await bindings.BrowserExtensionInstallLocalFile())
  }
  throw new Error('当前环境不支持本地插件包导入')
}

export async function installBrowserExtensionLocalDirectory(): Promise<BrowserExtension> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionInstallLocalDirectory) {
    return normalizeExtension(await bindings.BrowserExtensionInstallLocalDirectory())
  }
  throw new Error('当前环境不支持本地插件目录导入')
}

export async function setBrowserExtensionEnabled(extensionId: string, enabled: boolean): Promise<BrowserExtension> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionSetEnabled) {
    return normalizeExtension(await bindings.BrowserExtensionSetEnabled(extensionId, enabled))
  }
  throw new Error('当前环境不支持插件状态切换')
}

export async function deleteBrowserExtension(extensionId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionDelete) {
    await bindings.BrowserExtensionDelete(extensionId)
    return
  }
  throw new Error('当前环境不支持插件删除')
}

function normalizeProfileSettings(payload: any): BrowserProfileExtensionSettings {
  return {
    profileId: String(payload?.profileId || ''),
    configured: payload?.configured === true,
    extensionIds: Array.isArray(payload?.extensionIds) ? payload.extensionIds.map((item: unknown) => String(item || '')).filter(Boolean) : [],
    updatedAt: String(payload?.updatedAt || ''),
  }
}

export async function fetchBrowserProfileExtensionSettings(profileId: string): Promise<BrowserProfileExtensionSettings> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileExtensionGet) {
    return normalizeProfileSettings(await bindings.BrowserProfileExtensionGet(profileId))
  }
  throw new Error('当前环境不支持实例插件配置')
}

export async function saveBrowserProfileExtensionSettings(profileId: string, extensionIds: string[], configured: boolean): Promise<BrowserProfileExtensionSettings> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileExtensionSave) {
    return normalizeProfileSettings(await bindings.BrowserProfileExtensionSave(profileId, extensionIds, configured))
  }
  throw new Error('当前环境不支持保存实例插件配置')
}
