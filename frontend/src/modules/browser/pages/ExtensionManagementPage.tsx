import { useEffect, useMemo, useState } from 'react'
import { toast } from '../../../shared/components'
import type { BrowserExtension, BrowserExtensionLookupResult, BrowserProxy } from '../types'
import {
  deleteBrowserExtension,
  fetchBrowserExtensions,
  getBrowserExtensionManualInstallGuide,
  installBrowserExtension,
  installBrowserExtensionLocalDirectory,
  installBrowserExtensionLocalFile,
  installBrowserExtensionManualDownloadFile,
  listBrowserExtensionManualDownloadFiles,
  lookupBrowserExtension,
  openBrowserExtensionManualDownloadDir,
  setBrowserExtensionEnabled,
  setBrowserExtensionDefaultInstall,
  type BrowserExtensionManualDownloadFile,
  type BrowserExtensionManualInstallGuide,
} from '../api/extensions'
import { fetchBrowserProxies } from '../api/proxies'
import { ProxyPickerModal } from '../components/ProxyPickerModal'
import { ExtensionInstallCard, ExtensionManagementHeader, InstalledExtensionsList } from './ExtensionManagementCards'
import { DownloadDirectoryInstallModal, ExtensionHistoryModal, ExtensionProfileLimitModal, ManualInstallModal } from './ExtensionManagementModals'
import { EXTENSION_HISTORY_LIMIT, buildChromeWebStoreQueryURL, createExtensionHistoryRecord, extensionStoreURL, loadExtensionDownloadProxyPreference, loadExtensionHistory, saveExtensionDownloadProxyPreference, saveExtensionHistory, type ExtensionHistoryRecord } from './extensionManagementUtils'

export function ExtensionManagementPage() {
  const [items, setItems] = useState<BrowserExtension[]>([])
  const [query, setQuery] = useState('')
  const [lookup, setLookup] = useState<BrowserExtensionLookupResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [querying, setQuerying] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [importing, setImporting] = useState<'none' | 'file' | 'directory'>('none')
  const [updatingId, setUpdatingId] = useState('')
  const [busyId, setBusyId] = useState('')
  const [busyAction, setBusyAction] = useState<'toggle' | 'delete' | 'default' | ''>('')
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [useProxy, setUseProxy] = useState(false)
  const [selectedProxyId, setSelectedProxyId] = useState('')
  const [proxyModalOpen, setProxyModalOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyRecords, setHistoryRecords] = useState<ExtensionHistoryRecord[]>([])
  const [manualOpen, setManualOpen] = useState(false)
  const [manualLoading, setManualLoading] = useState(false)
  const [manualGuide, setManualGuide] = useState<BrowserExtensionManualInstallGuide | null>(null)
  const [manualFiles, setManualFiles] = useState<BrowserExtensionManualDownloadFile[]>([])
  const [manualFileLoading, setManualFileLoading] = useState(false)
  const [manualImportingFileName, setManualImportingFileName] = useState('')
  const [downloadDirOpen, setDownloadDirOpen] = useState(false)
  const [lastLookupProxyLabel, setLastLookupProxyLabel] = useState('')
  const [limitExtension, setLimitExtension] = useState<BrowserExtension | null>(null)

  const installedIds = useMemo(() => new Set(items.map((item) => item.extensionId)), [items])
  const selectedProxy = useMemo(
    () => proxies.find((proxy) => proxy.proxyId === selectedProxyId),
    [proxies, selectedProxyId],
  )
  const downloadProxyConfig = useProxy ? selectedProxy?.proxyConfig || '' : ''

  const refresh = async () => {
    setLoading(true)
    try {
      setItems(await fetchBrowserExtensions())
    } catch (error: any) {
      toast.error(error?.message || '加载插件失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
    setHistoryRecords(loadExtensionHistory())
  }, [])

  useEffect(() => {
    fetchBrowserProxies().then((items) => {
      const preference = loadExtensionDownloadProxyPreference()
      const directProxyId = items.find((item) => item.proxyConfig === 'direct://')?.proxyId || ''
      const restoredProxy = preference.proxyId ? items.find((item) => item.proxyId === preference.proxyId) : undefined
      setProxies(items)
      if (preference.useProxy && restoredProxy && restoredProxy.proxyConfig !== 'direct://') {
        setUseProxy(true)
        setSelectedProxyId(restoredProxy.proxyId)
      } else {
        setUseProxy(false)
        setSelectedProxyId(directProxyId)
      }
    }).catch(() => {
      setProxies([])
    })
  }, [])

  const appendHistory = (input: Omit<ExtensionHistoryRecord, 'id' | 'createdAt'>) => {
    const record = createExtensionHistoryRecord(input)
    setHistoryRecords((current) => {
      const next = [record, ...current].slice(0, EXTENSION_HISTORY_LIMIT)
      saveExtensionHistory(next)
      return next
    })
  }

  const currentProxyLabel = () => (useProxy && selectedProxy ? `使用代理：${selectedProxy.proxyName || selectedProxy.proxyId}` : '直连')

  const handleLookup = async () => {
    const value = query.trim()
    if (!value) {
      toast.warning('请输入插件 ID 或 Chrome Web Store 链接')
      return
    }
    if (useProxy && !downloadProxyConfig) {
      toast.warning('请先选择可用的下载代理')
      return
    }
    setQuerying(true)
    try {
      const result = await lookupBrowserExtension(value, downloadProxyConfig, useProxy)
      setLookup(result)
      const proxyLabel = currentProxyLabel()
      setLastLookupProxyLabel(useProxy && selectedProxy ? proxyLabel : '直连查询')
      appendHistory({
        action: 'lookup',
        query: value,
        extensionId: result.extensionId || '',
        name: result.name || '',
        version: result.version || '',
        storeUrl: result.storeUrl || '',
        proxyLabel,
        ok: result.installable,
        message: result.message || '',
      })
    } catch (error: any) {
      setLookup(null)
      appendHistory({
        action: 'lookup',
        query: value,
        extensionId: '',
        name: '',
        version: '',
        storeUrl: buildChromeWebStoreQueryURL(value),
        proxyLabel: currentProxyLabel(),
        ok: false,
        message: error?.message || '查询插件失败',
      })
      toast.error(error?.message || '查询插件失败')
    } finally {
      setQuerying(false)
    }
  }

  const handleOpenWebStoreQuery = () => {
    window.open(buildChromeWebStoreQueryURL(query), '_blank', 'noopener,noreferrer')
  }

  const refreshManualFiles = async () => {
    setManualFileLoading(true)
    try {
      setManualFiles(await listBrowserExtensionManualDownloadFiles())
    } catch (error: any) {
      toast.error(error?.message || '扫描下载目录失败')
    } finally {
      setManualFileLoading(false)
    }
  }

  const handleOpenManualInstall = async () => {
    const target = lookup?.extensionId || query.trim()
    if (!target) {
      toast.warning('请输入插件 ID 或 Chrome Web Store 链接')
      return
    }
    setManualOpen(true)
    setManualLoading(true)
    try {
      setManualGuide(await getBrowserExtensionManualInstallGuide(target))
      void refreshManualFiles()
    } catch (error: any) {
      setManualGuide(null)
      toast.error(error?.message || '生成手动安装信息失败')
    } finally {
      setManualLoading(false)
    }
  }

  const handleOpenManualDownloadDir = async () => {
    try {
      await openBrowserExtensionManualDownloadDir()
    } catch (error: any) {
      toast.error(error?.message || '打开下载目录失败')
    }
  }

  const handleOpenDownloadDirectoryInstall = async () => {
    setDownloadDirOpen(true)
    try {
      await openBrowserExtensionManualDownloadDir()
      await refreshManualFiles()
    } catch (error: any) {
      toast.error(error?.message || '打开下载目录失败')
    }
  }

  const handleImportManualFile = async (fileName: string) => {
    setManualImportingFileName(fileName)
    try {
      const installed = await installBrowserExtensionManualDownloadFile(fileName)
      appendHistory({
        action: 'import',
        query: fileName,
        extensionId: installed.extensionId || '',
        name: installed.name || '',
        version: installed.version || '',
        storeUrl: installed.sourceUrl || '',
        proxyLabel: '手动下载目录',
        ok: true,
        message: '导入成功',
      })
      toast.success(`已导入 ${installed.name || installed.extensionId}`)
      setManualOpen(false)
      setDownloadDirOpen(false)
      await refresh()
    } catch (error: any) {
      appendHistory({
        action: 'import',
        query: fileName,
        extensionId: '',
        name: '',
        version: '',
        storeUrl: '',
        proxyLabel: '手动下载目录',
        ok: false,
        message: error?.message || '导入插件失败',
      })
      toast.error(error?.message || '导入插件失败')
    } finally {
      setManualImportingFileName('')
    }
  }

  const handleInstall = async () => {
    const target = lookup?.extensionId || query.trim()
    if (!target) return
    if (useProxy && !downloadProxyConfig) {
      toast.warning('请先选择可用的下载代理')
      return
    }
    setInstalling(true)
    try {
      const installed = await installBrowserExtension(target, downloadProxyConfig, useProxy)
      appendHistory({
        action: 'install',
        query: target,
        extensionId: installed.extensionId || target,
        name: installed.name || '',
        version: installed.version || '',
        storeUrl: installed.sourceUrl || buildChromeWebStoreQueryURL(target),
        proxyLabel: currentProxyLabel(),
        ok: true,
        message: '安装成功',
      })
      toast.success(`已安装 ${installed.name || installed.extensionId}`)
      setLookup(null)
      setLastLookupProxyLabel('')
      setQuery('')
      await refresh()
    } catch (error: any) {
      appendHistory({
        action: 'install',
        query: target,
        extensionId: lookup?.extensionId || '',
        name: lookup?.name || '',
        version: lookup?.version || '',
        storeUrl: lookup?.storeUrl || buildChromeWebStoreQueryURL(target),
        proxyLabel: currentProxyLabel(),
        ok: false,
        message: error?.message || '安装插件失败',
      })
      toast.error(error?.message || '安装插件失败')
    } finally {
      setInstalling(false)
    }
  }

  const handleImportLocal = async (mode: 'file' | 'directory') => {
    setImporting(mode)
    try {
      const installed = mode === 'file'
        ? await installBrowserExtensionLocalFile()
        : await installBrowserExtensionLocalDirectory()
      appendHistory({
        action: 'import',
        query: mode === 'file' ? '本地插件包' : '本地插件目录',
        extensionId: installed.extensionId || '',
        name: installed.name || '',
        version: installed.version || '',
        storeUrl: installed.sourceUrl || '',
        proxyLabel: '本地导入',
        ok: true,
        message: '导入成功',
      })
      toast.success(`已导入 ${installed.name || installed.extensionId}`)
      await refresh()
    } catch (error: any) {
      const message = error?.message || '导入插件失败'
      if (!message.includes('已取消')) {
        appendHistory({
          action: 'import',
          query: mode === 'file' ? '本地插件包' : '本地插件目录',
          extensionId: '',
          name: '',
          version: '',
          storeUrl: '',
          proxyLabel: '本地导入',
          ok: false,
          message,
        })
      }
      if (!message.includes('已取消')) toast.error(message)
    } finally {
      setImporting('none')
    }
  }

  const handleUpdateExtension = async (item: BrowserExtension) => {
    if (useProxy && !downloadProxyConfig) {
      toast.warning('请先选择可用的下载代理')
      return
    }
    setUpdatingId(item.extensionId)
    try {
      const installed = await installBrowserExtension(item.extensionId, downloadProxyConfig, useProxy)
      appendHistory({
        action: 'install',
        query: item.extensionId,
        extensionId: installed.extensionId || item.extensionId,
        name: installed.name || item.name || '',
        version: installed.version || '',
        storeUrl: installed.sourceUrl || buildChromeWebStoreQueryURL(item.extensionId),
        proxyLabel: currentProxyLabel(),
        ok: true,
        message: item.version && installed.version && item.version !== installed.version ? `已更新 ${item.version} → ${installed.version}` : '更新完成',
      })
      toast.success(item.version && installed.version && item.version !== installed.version
        ? `已更新到 v${installed.version}`
        : '插件已重新安装')
      await refresh()
    } catch (error: any) {
      appendHistory({
        action: 'install',
        query: item.extensionId,
        extensionId: item.extensionId,
        name: item.name || '',
        version: item.version || '',
        storeUrl: extensionStoreURL(item) || buildChromeWebStoreQueryURL(item.extensionId),
        proxyLabel: currentProxyLabel(),
        ok: false,
        message: error?.message || '更新插件失败',
      })
      toast.error(error?.message || '更新插件失败')
    } finally {
      setUpdatingId('')
    }
  }

  const handleToggle = async (item: BrowserExtension) => {
    setBusyId(item.extensionId)
    setBusyAction('toggle')
    try {
      const updated = await setBrowserExtensionEnabled(item.extensionId, !item.enabled)
      setItems((current) => current.map((entry) => entry.extensionId === updated.extensionId ? updated : entry))
    } catch (error: any) {
      toast.error(error?.message || '更新插件状态失败')
    } finally {
      setBusyId('')
      setBusyAction('')
    }
  }

  const handleDelete = async (item: BrowserExtension) => {
    if (!window.confirm(`删除插件「${item.name || item.extensionId}」？`)) return
    setBusyId(item.extensionId)
    setBusyAction('delete')
    try {
      await deleteBrowserExtension(item.extensionId)
      setItems((current) => current.filter((entry) => entry.extensionId !== item.extensionId))
      toast.success('插件已删除')
    } catch (error: any) {
      toast.error(error?.message || '删除插件失败')
    } finally {
      setBusyId('')
      setBusyAction('')
    }
  }

  const handleDefaultInstallToggle = async (item: BrowserExtension) => {
    setBusyId(item.extensionId)
    setBusyAction('default')
    try {
      const updated = await setBrowserExtensionDefaultInstall(item.extensionId, !item.defaultInstall)
      setItems((current) => current.map((entry) => entry.extensionId === updated.extensionId ? updated : entry))
      toast.success(updated.defaultInstall ? '已开启默认安装，未单独配置的实例下次启动会安装' : '已关闭默认安装，未单独配置的实例下次启动会移除')
    } catch (error: any) {
      toast.error(error?.message || '更新默认安装状态失败')
    } finally {
      setBusyId('')
      setBusyAction('')
    }
  }

  const handlePickHistory = (record: ExtensionHistoryRecord) => {
    setQuery(record.extensionId || record.query)
    setLookup(null)
    setLastLookupProxyLabel('')
    setHistoryOpen(false)
  }

  const handleClearHistory = () => {
    setHistoryRecords([])
    saveExtensionHistory([])
    toast.success('历史已清空')
  }

  const proxyButtonText = useProxy
    ? `下载代理：${selectedProxy?.proxyName || selectedProxyId || '未选择'}`
    : '下载代理'

  return (
    <div className="space-y-4 animate-fade-in">
      <ExtensionManagementHeader
        proxyButtonText={proxyButtonText}
        loading={loading}
        importing={importing}
        downloadDirectoryLoading={manualFileLoading && downloadDirOpen}
        onOpenProxy={() => setProxyModalOpen(true)}
        onOpenHistory={() => setHistoryOpen(true)}
        onImportFile={() => void handleImportLocal('file')}
        onImportDirectory={() => void handleImportLocal('directory')}
        onOpenDownloadDirectory={() => void handleOpenDownloadDirectoryInstall()}
        onRefresh={refresh}
      />

      <ProxyPickerModal
        open={proxyModalOpen}
        title="选择下载代理"
        currentProxyId={useProxy ? selectedProxyId : proxies.find((item) => item.proxyConfig === 'direct://')?.proxyId || ''}
        onClose={() => setProxyModalOpen(false)}
        onSelect={(proxy) => {
          setProxies((current) => {
            if (current.some((item) => item.proxyId === proxy.proxyId)) return current
            return [...current, proxy]
          })
          setSelectedProxyId(proxy.proxyId)
          const nextUseProxy = proxy.proxyConfig !== 'direct://'
          setUseProxy(nextUseProxy)
          saveExtensionDownloadProxyPreference({ useProxy: nextUseProxy, proxyId: proxy.proxyId })
        }}
        onProxyListUpdated={(nextProxies) => {
          setProxies(nextProxies)
        }}
        onProxyDeleted={(deletedProxyId, nextProxies) => {
          setProxies(nextProxies)
          if (deletedProxyId === selectedProxyId) {
            const directProxyId = nextProxies.find((item) => item.proxyConfig === 'direct://')?.proxyId || ''
            setUseProxy(false)
            setSelectedProxyId(directProxyId)
            saveExtensionDownloadProxyPreference({ useProxy: false, proxyId: directProxyId })
          }
        }}
        onProxyTested={(proxyId, result) => {
          const testedAt = new Date().toISOString()
          setProxies((current) => current.map((proxy) => (
            proxy.proxyId === proxyId
              ? { ...proxy, lastTestOk: result.ok, lastLatencyMs: result.ok ? result.latencyMs : -1, lastTestedAt: testedAt }
              : proxy
          )))
        }}
      />

      <ExtensionHistoryModal
        open={historyOpen}
        records={historyRecords}
        onClose={() => setHistoryOpen(false)}
        onPick={handlePickHistory}
        onClear={handleClearHistory}
      />

      <ExtensionProfileLimitModal
        open={!!limitExtension}
        extension={limitExtension}
        allExtensions={items}
        onClose={() => setLimitExtension(null)}
      />

      <ManualInstallModal
        open={manualOpen}
        guide={manualGuide}
        files={manualFiles}
        loading={manualLoading}
        fileLoading={manualFileLoading}
        importingFileName={manualImportingFileName}
        onClose={() => setManualOpen(false)}
        onOpenDownloadDir={handleOpenManualDownloadDir}
        onRefreshFiles={() => void refreshManualFiles()}
        onImportFile={(fileName) => void handleImportManualFile(fileName)}
        onImportDirectory={() => {
          setManualOpen(false)
          void handleImportLocal('directory')
        }}
      />

      <DownloadDirectoryInstallModal
        open={downloadDirOpen}
        files={manualFiles}
        fileLoading={manualFileLoading}
        importingFileName={manualImportingFileName}
        onClose={() => setDownloadDirOpen(false)}
        onOpenDownloadDir={handleOpenManualDownloadDir}
        onRefreshFiles={() => void refreshManualFiles()}
        onImportFile={(fileName) => void handleImportManualFile(fileName)}
      />

      <ExtensionInstallCard
        query={query}
        lookup={lookup}
        querying={querying}
        installing={installing}
        useProxy={useProxy}
        selectedProxy={selectedProxy}
        installedIds={installedIds}
        lastLookupProxyLabel={lastLookupProxyLabel}
        onQueryChange={setQuery}
        onLookup={() => void handleLookup()}
        onOpenWebStoreQuery={handleOpenWebStoreQuery}
        onOpenManualInstall={handleOpenManualInstall}
        onOpenProxy={() => setProxyModalOpen(true)}
        onInstall={() => void handleInstall()}
      />

      <InstalledExtensionsList
        items={items}
        busyId={busyId}
        busyAction={busyAction}
        updatingId={updatingId}
        onRestrictProfiles={setLimitExtension}
        onDefaultInstallToggle={(target) => void handleDefaultInstallToggle(target)}
        onUpdate={(target) => void handleUpdateExtension(target)}
        onToggle={(target) => void handleToggle(target)}
        onDelete={(target) => void handleDelete(target)}
      />
    </div>

  )
}
