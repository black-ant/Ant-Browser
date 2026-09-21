import { useCallback, useEffect, useMemo, useState } from 'react'
import { Download, FolderOpen, MoreHorizontal, Pencil, Plus, RefreshCw, ScanSearch, Search, Settings2, Star, Trash2, Upload } from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserCore, BrowserCoreExtended, BrowserCoreInput, BrowserCoreValidateResult, BrowserProxy, BrowserSettings } from '../../modules/browser/types'
import {
  BrowserCoreDownload,
  deleteBrowserCore,
  fetchBrowserCores,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchCoreExtendedInfo,
  importLocalBrowserCore,
  openCorePath,
  redownloadBrowserCore,
  saveBrowserCore,
  saveBrowserSettings,
  scanBrowserCores,
  setDefaultBrowserCore,
  validateBrowserCorePath,
} from '../../modules/browser/api'
import type { CoreDisplayInfo, CoreDownloadForm, CoreDownloadProgress, CoreEditForm, CoreSettingsForm } from '../../modules/browser/pages/coreManagement.types'
import { fetchCoreDownloadRecommendation, type CoreDownloadRecommendation } from '../../modules/browser/pages/coreManagement/coreDownloadRecommendation'
import { FishConfirm } from './FishFormPrimitives'
import { FishCoreDownloadModal, FishCoreEditModal, FishCoreSettingsModal } from './FishCoreModals'
import './fish-core.css'

const defaultSettings: BrowserSettings = {
  userDataRoot: '',
  defaultFingerprintArgs: [],
  defaultLaunchArgs: [],
  defaultStartUrls: [],
  lightStartEnabled: true,
  restoreLastSession: true,
  startReadyTimeoutMs: 3000,
  startStableWindowMs: 1200,
  defaultConnectorType: 'xray',
}

function compactList(values: string[]) {
  if (!values.length) return '—'
  return values.join(' · ')
}

export function FishCoreManagementPage() {
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [displayList, setDisplayList] = useState<CoreDisplayInfo[]>([])
  const [settings, setSettings] = useState<BrowserSettings>(defaultSettings)
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [loading, setLoading] = useState(true)
  const [keyword, setKeyword] = useState('')
  const [scanning, setScanning] = useState(false)
  const [importing, setImporting] = useState(false)
  const [importProgress, setImportProgress] = useState<CoreDownloadProgress | null>(null)
  const [rowMenuId, setRowMenuId] = useState<string | null>(null)

  const [settingsOpen, setSettingsOpen] = useState(false)
  const [settingsForm, setSettingsForm] = useState<CoreSettingsForm>({ userDataRoot: '', defaultFingerprintArgs: '', defaultLaunchArgs: '', defaultStartUrls: '', lightStartEnabled: true, restoreLastSession: true, startReadyTimeoutMs: 3000, startStableWindowMs: 1200 })
  const [savingSettings, setSavingSettings] = useState(false)

  const [editOpen, setEditOpen] = useState(false)
  const [editingCore, setEditingCore] = useState<BrowserCore | null>(null)
  const [editForm, setEditForm] = useState<CoreEditForm>({ coreName: '', corePath: '' })
  const [savingCore, setSavingCore] = useState(false)
  const [pathValidating, setPathValidating] = useState(false)
  const [pathValidation, setPathValidation] = useState<BrowserCoreValidateResult | null>(null)

  const [deleteCore, setDeleteCore] = useState<CoreDisplayInfo | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)

  const [downloadOpen, setDownloadOpen] = useState(false)
  const [downloadForm, setDownloadForm] = useState<CoreDownloadForm>({ name: '', url: '', proxyMode: 'system', proxyId: '', mode: 'download' })
  const [downloadProgress, setDownloadProgress] = useState<CoreDownloadProgress | null>(null)
  const [recommendation, setRecommendation] = useState<CoreDownloadRecommendation | null>(null)
  const [recommendationLoading, setRecommendationLoading] = useState(false)
  const [recommendationError, setRecommendationError] = useState('')

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [settingsData, coreList, extendedInfo, proxyList] = await Promise.all([fetchBrowserSettings(), fetchBrowserCores(), fetchCoreExtendedInfo(), fetchBrowserProxies()])
      setSettings(settingsData)
      setCores(coreList)
      setProxies(proxyList)
      const extendedMap = new Map<string, BrowserCoreExtended>(extendedInfo.map((item) => [item.coreId, item]))
      const rows = await Promise.all(coreList.map(async (core): Promise<CoreDisplayInfo> => {
        const validation = await validateBrowserCorePath(core.corePath)
        const extended = extendedMap.get(core.coreId)
        return { coreId: core.coreId, coreName: core.coreName, corePath: core.corePath, isDefault: core.isDefault, pathValid: validation.valid, pathMessage: validation.message, chromeVersion: extended?.chromeVersion || '', instanceCount: extended?.instanceCount || 0 }
      }))
      setDisplayList(rows)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '加载内核失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadData()
    const runtime = (window as Window & { runtime?: { EventsOn?: (event: string, callback: (payload: any) => void) => (() => void) | void } }).runtime
    const offDownload = runtime?.EventsOn?.('download:progress', (data: CoreDownloadProgress) => {
      setDownloadProgress(data)
      if (data.phase === 'done') {
        toast.success(data.message)
        window.setTimeout(() => { setDownloadOpen(false); setDownloadProgress(null); void loadData() }, 1500)
      } else if (data.phase === 'error') {
        toast.error(data.message)
        setDownloadProgress(null)
      }
    }) || (() => {})
    const offImport = runtime?.EventsOn?.('core-import:progress', (data: CoreDownloadProgress) => {
      setImportProgress(data)
      if (data.phase === 'done' || data.phase === 'error') window.setTimeout(() => setImportProgress(null), 1200)
    }) || (() => {})
    return () => { offDownload(); offImport() }
  }, [loadData])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      if (!editOpen || !editForm.corePath.trim()) { if (!editForm.corePath.trim()) setPathValidation(null); return }
      setPathValidating(true)
      void validateBrowserCorePath(editForm.corePath).then(setPathValidation).finally(() => setPathValidating(false))
    }, 500)
    return () => window.clearTimeout(timer)
  }, [editForm.corePath, editOpen])

  const filteredRows = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    if (!q) return displayList
    return displayList.filter((row) => `${row.coreName} ${row.corePath} ${row.chromeVersion}`.toLowerCase().includes(q))
  }, [displayList, keyword])

  const validCount = displayList.filter((row) => row.pathValid).length
  const instanceCount = displayList.reduce((sum, row) => sum + row.instanceCount, 0)
  const defaultCore = displayList.find((row) => row.isDefault)

  const openSettings = () => {
    setSettingsForm({ userDataRoot: settings.userDataRoot, defaultFingerprintArgs: settings.defaultFingerprintArgs.join('\n'), defaultLaunchArgs: settings.defaultLaunchArgs.join('\n'), defaultStartUrls: settings.defaultStartUrls.join('\n'), lightStartEnabled: settings.lightStartEnabled, restoreLastSession: settings.restoreLastSession, startReadyTimeoutMs: settings.startReadyTimeoutMs, startStableWindowMs: settings.startStableWindowMs })
    setSettingsOpen(true)
  }

  const saveSettings = async () => {
    setSavingSettings(true)
    try {
      const next: BrowserSettings = {
        userDataRoot: settingsForm.userDataRoot.trim(),
        defaultFingerprintArgs: settingsForm.defaultFingerprintArgs.split('\n').map((item) => item.trim()).filter(Boolean),
        defaultLaunchArgs: settingsForm.defaultLaunchArgs.split('\n').map((item) => item.trim()).filter(Boolean),
        defaultStartUrls: settingsForm.defaultStartUrls.split('\n').map((item) => item.trim()).filter(Boolean),
        lightStartEnabled: settingsForm.lightStartEnabled,
        restoreLastSession: settingsForm.restoreLastSession,
        startReadyTimeoutMs: Math.max(1000, Number(settingsForm.startReadyTimeoutMs) || 3000),
        startStableWindowMs: Math.max(0, Number(settingsForm.startStableWindowMs) || 1200),
        defaultConnectorType: settings.defaultConnectorType || 'xray',
      }
      await saveBrowserSettings(next)
      setSettings(next)
      setSettingsOpen(false)
      toast.success('全局浏览器基线已保存')
    } catch (error) { toast.error(error instanceof Error ? error.message : '保存失败') } finally { setSavingSettings(false) }
  }

  const openAdd = () => { setEditingCore(null); setEditForm({ coreName: '', corePath: '' }); setPathValidation(null); setEditOpen(true) }
  const openEdit = (row: CoreDisplayInfo) => {
    const core = cores.find((item) => item.coreId === row.coreId)
    if (!core) return
    setEditingCore(core)
    setEditForm({ coreName: core.coreName, corePath: core.corePath })
    setPathValidation({ valid: row.pathValid, message: row.pathMessage })
    setEditOpen(true)
  }

  const saveCore = async () => {
    if (!editForm.coreName.trim()) { toast.error('请输入内核名称'); return }
    if (!editForm.corePath.trim()) { toast.error('请输入内核路径'); return }
    setSavingCore(true)
    try {
      const input: BrowserCoreInput = { coreId: editingCore?.coreId || `core-${Date.now()}`, coreName: editForm.coreName.trim(), corePath: editForm.corePath.trim(), isDefault: editingCore?.isDefault || false }
      await saveBrowserCore(input)
      setEditOpen(false)
      await loadData()
      toast.success(editingCore ? '内核已更新' : '内核已添加')
    } catch (error) { toast.error(error instanceof Error ? error.message : '保存失败') } finally { setSavingCore(false) }
  }

  const handleScan = async () => {
    setScanning(true)
    try { await scanBrowserCores(); await loadData(); toast.success('扫描完成') }
    catch (error) { toast.error(error instanceof Error ? error.message : '扫描失败') }
    finally { setScanning(false) }
  }

  const handleImport = async () => {
    setImporting(true)
    setImportProgress({ phase: 'selecting', progress: 0, message: '请选择本地内核包…' })
    try {
      const imported = await importLocalBrowserCore()
      if (!imported) { setImportProgress(null); return }
      await loadData(); toast.success(`已导入：${imported.coreName}`)
    } catch (error) { toast.error(error instanceof Error ? error.message : '导入失败') }
    finally { setImporting(false) }
  }

  const setDefaultCore = async (coreId: string) => {
    try { await setDefaultBrowserCore(coreId); await loadData(); toast.success('已设为默认内核') }
    catch (error) { toast.error(error instanceof Error ? error.message : '设置失败') }
  }

  const confirmDelete = async () => {
    if (!deleteCore) return
    setDeleteBusy(true)
    try { await deleteBrowserCore(deleteCore.coreId); setDeleteCore(null); await loadData(); toast.success('内核已删除') }
    catch (error) { toast.error(error instanceof Error ? error.message : '删除失败') }
    finally { setDeleteBusy(false) }
  }

  const openPath = async (path: string) => {
    try { const opened = await openCorePath(path); if (!opened) toast.warning('当前环境不支持打开目录') }
    catch (error) { toast.error(error instanceof Error ? error.message : '打开目录失败') }
  }

  const loadRecommendation = async () => {
    setRecommendationLoading(true); setRecommendationError('')
    try {
      const runtime = (window as Window & { runtime?: { Environment?: () => Promise<{ platform?: string; arch?: string }> } }).runtime
      let env: { platform?: string; arch?: string } | null = null
      try { env = runtime?.Environment ? await runtime.Environment() : null } catch { env = null }
      const result = await fetchCoreDownloadRecommendation(env)
      setRecommendation(result)
      if (!result) { setRecommendationError('当前环境没有可自动解压的 Release 压缩包'); return }
      setDownloadForm((current) => ({ ...current, url: current.url.trim() ? current.url : result.downloadUrl }))
    } catch (error) { setRecommendation(null); setRecommendationError(error instanceof Error ? error.message : '获取推荐地址失败') }
    finally { setRecommendationLoading(false) }
  }

  const openDownload = () => {
    setDownloadForm({ name: '', url: '', proxyMode: 'system', proxyId: '', mode: 'download' })
    setDownloadProgress(null); setRecommendation(null); setRecommendationError(''); setDownloadOpen(true); void loadRecommendation()
  }
  const openRedownload = (row: CoreDisplayInfo) => {
    setDownloadForm({ coreId: row.coreId, name: row.coreName, url: '', proxyMode: 'system', proxyId: '', mode: 'redownload' })
    setDownloadProgress(null); setRecommendation(null); setRecommendationError(''); setDownloadOpen(true); void loadRecommendation()
  }

  const startDownload = async () => {
    if (!downloadForm.name.trim() || !downloadForm.url.trim()) { toast.error('请输入名称和下载地址'); return }
    if (downloadForm.mode === 'redownload' && !downloadForm.coreId) { toast.error('缺少内核 ID'); return }
    setDownloadProgress({ phase: 'starting', progress: 0, message: '准备下载…' })
    try {
      let targetProxy = downloadForm.proxyMode === 'system' ? '__system__' : downloadForm.proxyMode === 'direct' ? '__direct__' : downloadForm.proxyId
      if (downloadForm.proxyMode === 'custom') {
        const proxy = proxies.find((item) => item.proxyId === downloadForm.proxyId)
        if (proxy?.proxyConfig) targetProxy = proxy.proxyConfig
      }
      if (downloadForm.mode === 'redownload') await redownloadBrowserCore(downloadForm.coreId || '', downloadForm.url.trim(), targetProxy)
      else await BrowserCoreDownload(downloadForm.name.trim(), downloadForm.url.trim(), targetProxy)
    } catch (error) { toast.error(error instanceof Error ? error.message : '启动下载失败'); setDownloadProgress(null) }
  }

  return <div className="fish-content fish-core-content">
    <section className="fish-core-hero"><div><span className="fish-eyebrow">内核与指纹</span><h1>内核与指纹</h1><p>管理 Chrome 内核与浏览器默认基线。</p></div><button className="fish-btn primary" type="button" onClick={openDownload}><Download size={16} strokeWidth={1.8} />下载内核</button></section>

    <section className="fish-core-health-strip"><div><span>已配置内核</span><strong>{displayList.length}</strong></div><div><span>路径有效</span><strong>{validCount}</strong></div><div><span>默认内核</span><strong>{defaultCore?.coreName || '—'}</strong></div><div><span>使用实例</span><strong>{instanceCount}</strong></div></section>

    <section className="fish-core-baseline">
      <div className="fish-core-section-head"><div><span className="fish-eyebrow">全局浏览器基线</span><strong>实例未单独覆盖时使用这些默认值</strong></div><button className="fish-btn" type="button" onClick={openSettings}><Settings2 size={16} strokeWidth={1.8} />编辑基线</button></div>
      <div className="fish-core-baseline-grid"><div><span>用户数据根目录</span><strong>{settings.userDataRoot || '—'}</strong></div><div><span>轻启动</span><strong>{settings.lightStartEnabled ? '开启' : '关闭'}</strong></div><div><span>恢复历史标签</span><strong>{settings.restoreLastSession ? '开启' : '关闭'}</strong></div><div><span>就绪超时</span><strong>{settings.startReadyTimeoutMs} ms</strong></div><div><span>稳定窗口</span><strong>{settings.startStableWindowMs} ms</strong></div></div>
      <div className="fish-core-baseline-lines"><div><span>默认启动页面</span><strong>{compactList(settings.defaultStartUrls)}</strong></div><div><span>默认指纹参数</span><strong>{compactList(settings.defaultFingerprintArgs)}</strong></div><div><span>默认启动参数</span><strong>{compactList(settings.defaultLaunchArgs)}</strong></div></div>
    </section>

    {importProgress ? <div className="fish-core-import-progress"><span>{importProgress.message}</span><strong>{Math.max(0, Math.min(100, importProgress.progress))}%</strong></div> : null}

    <section className="fish-core-toolbar"><div className="fish-search fish-core-search"><Search size={16} strokeWidth={1.8} /><input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="搜索内核名称、版本或路径" /></div><button className="fish-btn" type="button" disabled={scanning} onClick={() => void handleScan()}><ScanSearch size={16} strokeWidth={1.8} />{scanning ? '扫描中' : '扫描内核'}</button><button className="fish-btn" type="button" disabled={importing} onClick={() => void handleImport()}><Upload size={16} strokeWidth={1.8} />{importing ? '导入中' : '导入本地'}</button><button className="fish-btn" type="button" onClick={() => void loadData()}><RefreshCw size={16} strokeWidth={1.8} />刷新</button><span className="fish-spacer" /><button className="fish-btn" type="button" onClick={openAdd}><Plus size={16} strokeWidth={1.8} />新增内核</button></section>

    <section className="fish-core-list">
      {loading ? <div className="fish-data-empty"><strong>正在读取内核</strong><span>验证路径并读取 Chrome 版本与实例占用。</span></div> : filteredRows.length ? filteredRows.map((row) => <article className="fish-core-row" key={row.coreId}>
        <div className="fish-core-name"><span className={`fish-core-mark${row.pathValid ? ' good' : ' bad'}`}><Star size={16} strokeWidth={1.8} /></span><div><strong>{row.coreName}{row.isDefault ? <em>默认</em> : null}</strong><small>{row.chromeVersion ? `Chrome ${row.chromeVersion}` : '版本未识别'}</small></div></div>
        <div className="fish-core-path"><strong>{row.corePath || '—'}</strong><small>{row.pathMessage || '内核路径'}</small></div>
        <div className={`fish-core-status${row.pathValid ? ' good' : ' bad'}`}><strong>{row.pathValid ? '路径有效' : '路径无效'}</strong><small>{row.instanceCount} 个实例使用</small></div>
        <div className="fish-core-actions"><button className="fish-mini-action" type="button" onClick={() => void openPath(row.corePath)}><FolderOpen size={16} strokeWidth={1.8} />目录</button><button className="fish-mini-action" type="button" onClick={() => openEdit(row)}><Pencil size={16} strokeWidth={1.8} />编辑</button><div className="fish-row-menu-wrap"><button className="fish-mini-icon" type="button" onClick={() => setRowMenuId(rowMenuId === row.coreId ? null : row.coreId)}><MoreHorizontal size={16} strokeWidth={1.8} /></button>{rowMenuId === row.coreId ? <div className="fish-context-menu fish-core-row-menu"><button type="button" onClick={() => { openRedownload(row); setRowMenuId(null) }}><Download size={16} strokeWidth={1.8} />重新下载</button>{!row.isDefault ? <button type="button" onClick={() => { void setDefaultCore(row.coreId); setRowMenuId(null) }}><Star size={16} strokeWidth={1.8} />设为默认</button> : null}<button className="danger" type="button" disabled={row.isDefault} onClick={() => { if (row.isDefault) toast.warning('默认内核不能删除'); else setDeleteCore(row); setRowMenuId(null) }}><Trash2 size={16} strokeWidth={1.8} />删除</button></div> : null}</div></div>
      </article>) : <div className="fish-data-empty"><strong>{keyword ? '没有匹配的内核' : '还没有内核'}</strong><span>{keyword ? '换一个关键词，或清空搜索。' : '可以扫描、导入、下载或手动新增 Chrome 内核。'}</span></div>}
    </section>

    <FishCoreSettingsModal open={settingsOpen} form={settingsForm} saving={savingSettings} setForm={setSettingsForm} onClose={() => setSettingsOpen(false)} onSave={() => void saveSettings()} />
    <FishCoreEditModal open={editOpen} editing={Boolean(editingCore)} form={editForm} saving={savingCore} validating={pathValidating} validation={pathValidation} setForm={setEditForm} onClose={() => setEditOpen(false)} onSave={() => void saveCore()} />
    <FishCoreDownloadModal open={downloadOpen} form={downloadForm} progress={downloadProgress} recommendation={recommendation} recommendationLoading={recommendationLoading} recommendationError={recommendationError} proxies={proxies} setForm={setDownloadForm} setProgress={setDownloadProgress} onRefreshRecommendation={() => void loadRecommendation()} onClose={() => setDownloadOpen(false)} onStart={() => void startDownload()} />
    <FishConfirm open={Boolean(deleteCore)} title="删除 Chrome 内核" content={`确定删除“${deleteCore?.coreName || ''}”吗？此操作不可恢复。`} confirmText="删除" danger busy={deleteBusy} onClose={() => setDeleteCore(null)} onConfirm={() => void confirmDelete()} />
  </div>
}
