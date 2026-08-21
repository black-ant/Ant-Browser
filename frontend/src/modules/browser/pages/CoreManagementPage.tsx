import { useEffect, useState, useCallback } from 'react'
import { FolderOpen } from 'lucide-react'
import { Badge, Button, Card, ConfirmModal, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserCore, BrowserCoreInput, BrowserCoreValidateResult, BrowserSettings, BrowserCoreExtended, BrowserProxy } from '../types'
import { fetchBrowserCores, saveBrowserCore, deleteBrowserCore, setDefaultBrowserCore, validateBrowserCorePathForBackend, openCorePath, fetchBrowserSettings, saveBrowserSettings, fetchCoreExtendedInfo, scanBrowserCores, importLocalBrowserCore, BrowserCoreDownload, fetchBrowserProxies, redownloadBrowserCore } from '../api'
import { CORE_BACKEND_FINGERPRINT_CHROMIUM, coreBackendLabel, isCloakBackend, normalizeCoreBackend, parseCoreEnvInput } from '../utils/coreBackend'
import { Environment, EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime'
import { CoreDownloadModal } from './coreManagement/CoreDownloadModal'
import { CoreEditModal } from './coreManagement/CoreEditModal'
import { CoreSettingsCard } from './coreManagement/CoreSettingsCard'
import { CoreSettingsModal } from './coreManagement/CoreSettingsModal'
import { fetchCoreDownloadRecommendation } from './coreManagement/coreDownloadRecommendation'
import type { CoreDownloadRecommendation } from './coreManagement/coreDownloadRecommendation'
import type { CoreDisplayInfo, CoreDownloadForm, CoreDownloadProgress, CoreEditForm, CoreSettingsForm } from './coreManagement.types'

export function CoreManagementPage() {
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [displayList, setDisplayList] = useState<CoreDisplayInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [scanning, setScanning] = useState(false)
  const [importing, setImporting] = useState(false)

  // 全局设置状态
  const [settings, setSettings] = useState<BrowserSettings>({
    userDataRoot: '',
    defaultFingerprintArgs: [],
    defaultLaunchArgs: [],
    defaultStartUrls: [],
    lightStartEnabled: true,
    restoreLastSession: false,
    startReadyTimeoutMs: 3000,
    startStableWindowMs: 1200,
    defaultConnectorType: 'xray',
  })
  const [settingsModalOpen, setSettingsModalOpen] = useState(false)
  const [settingsForm, setSettingsForm] = useState<CoreSettingsForm>({
    userDataRoot: '',
    defaultFingerprintArgs: '',
    defaultLaunchArgs: '',
    defaultStartUrls: '',
    lightStartEnabled: true,
    restoreLastSession: false,
    startReadyTimeoutMs: 3000,
    startStableWindowMs: 1200,
  })
  const [savingSettings, setSavingSettings] = useState(false)

  // 编辑弹窗状态
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingCore, setEditingCore] = useState<BrowserCore | null>(null)
  const [editForm, setEditForm] = useState<CoreEditForm>({ coreName: '', corePath: '', coreBackend: CORE_BACKEND_FINGERPRINT_CHROMIUM, coreEnv: '' })
  const [saving, setSaving] = useState(false)
  const [pathValidating, setPathValidating] = useState(false)
  const [pathValidResult, setPathValidResult] = useState<BrowserCoreValidateResult | null>(null)

  // 删除确认状态
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deletingCore, setDeletingCore] = useState<CoreDisplayInfo | null>(null)

  // 内核下载
  const [downloadModalOpen, setDownloadModalOpen] = useState(false)
  const [downloadForm, setDownloadForm] = useState<CoreDownloadForm>({ name: '', url: '', proxyMode: 'system', proxyId: '', mode: 'download' })
  const [downloadProgress, setDownloadProgress] = useState<CoreDownloadProgress | null>(null)
  const [downloadRecommendation, setDownloadRecommendation] = useState<CoreDownloadRecommendation | null>(null)
  const [downloadRecommendationLoading, setDownloadRecommendationLoading] = useState(false)
  const [downloadRecommendationError, setDownloadRecommendationError] = useState('')
  const [importProgress, setImportProgress] = useState<CoreDownloadProgress | null>(null)
  const [proxies, setProxies] = useState<BrowserProxy[]>([])

  useEffect(() => {
    loadData()

    // 监听下载进度
    const onDownloadProgress = (data: { phase: string; progress: number; message: string }) => {
      setDownloadProgress(data)
      if (data.phase === 'done') {
        toast.success(data.message)
        setTimeout(() => {
          setDownloadModalOpen(false)
          setDownloadProgress(null)
          loadData() // 更新内核列表
        }, 1500)
      } else if (data.phase === 'error') {
        toast.error(data.message)
        setDownloadProgress(null) // 清理进度使其可以重新开始
      }
    }
    EventsOn('download:progress', onDownloadProgress)

    const onImportProgress = (data: { phase: string; progress: number; message: string }) => {
      setImportProgress(data)
      if (data.phase === 'done' || data.phase === 'error') {
        setTimeout(() => setImportProgress(null), 1200)
      }
    }
    EventsOn('core-import:progress', onImportProgress)

    return () => {
      EventsOff('download:progress')
      EventsOff('core-import:progress')
    }
  }, [])

  const loadData = async () => {
    setLoading(true)
    try {
      // 并行加载设置、内核列表和扩展信息
      const [settingsData, coreList, extendedInfo] = await Promise.all([
        fetchBrowserSettings(),
        fetchBrowserCores(),
        fetchCoreExtendedInfo(),
      ])

      setSettings(settingsData)
      setCores(coreList)

      // 创建扩展信息映射
      const extendedMap = new Map<string, BrowserCoreExtended>()
      extendedInfo.forEach(info => extendedMap.set(info.coreId, info))

      // 验证所有路径并合并扩展信息
      const displayInfoList: CoreDisplayInfo[] = await Promise.all(
        coreList.map(async (core) => {
          const coreBackend = normalizeCoreBackend(core.coreBackend)
          // 按内核后端校验：不同后端的可执行文件名不同
          const result = await validateBrowserCorePathForBackend(core.corePath, coreBackend)
          const extended = extendedMap.get(core.coreId)
          return {
            coreId: core.coreId,
            coreName: core.coreName,
            corePath: core.corePath,
            isDefault: core.isDefault,
            pathValid: result.valid,
            pathMessage: result.message,
            chromeVersion: extended?.chromeVersion || '',
            instanceCount: extended?.instanceCount || 0,
            coreBackend,
          }
        })
      )
      setDisplayList(displayInfoList)
    } finally {
      setLoading(false)
    }
  }

  // 防抖验证路径（按当前选择的内核后端校验）
  const validatePath = useCallback(async (path: string, coreBackend: string) => {
    if (!path.trim()) {
      setPathValidResult(null)
      return
    }
    setPathValidating(true)
    try {
      const result = await validateBrowserCorePathForBackend(path, normalizeCoreBackend(coreBackend))
      setPathValidResult(result)
    } finally {
      setPathValidating(false)
    }
  }, [])

  // 路径或后端变化时触发验证（防抖）
  useEffect(() => {
    fetchBrowserProxies().then(setProxies)
    const timer = setTimeout(() => {
      if (editModalOpen && editForm.corePath) {
        validatePath(editForm.corePath, editForm.coreBackend)
      }
    }, 500)
    return () => clearTimeout(timer)
  }, [editForm.corePath, editForm.coreBackend, editModalOpen, validatePath])

  // 表格列定义
  const columns: TableColumn<CoreDisplayInfo>[] = [
    { key: 'coreName', title: '内核名称', width: '150px' },
    {
      key: 'coreBackend',
      title: '内核后端',
      width: '150px',
      render: (val) => (
        <Badge variant={isCloakBackend(val as string) ? 'info' : 'default'}>
          {coreBackendLabel(val as string)}
        </Badge>
      ),
    },
    { key: 'corePath', title: '内核路径', width: '180px' },
    {
      key: 'chromeVersion',
      title: 'Chrome 版本',
      width: '130px',
      render: (val) => val || '-',
    },
    {
      key: 'instanceCount',
      title: '使用实例',
      width: '90px',
      render: (val) => <Badge variant="default">{val}</Badge>,
    },
    {
      key: 'isDefault',
      title: '默认',
      width: '70px',
      render: (val) => val ? <Badge variant="info">默认</Badge> : null,
    },
    {
      key: 'pathValid',
      title: '状态',
      width: '80px',
      render: (val) => (
        <Badge variant={val ? 'success' : 'error'}>
          {val ? '有效' : '无效'}
        </Badge>
      ),
    },
    {
      key: 'actions',
      title: '操作',
      width: '220px',
      render: (_, record) => (
        <div className="flex gap-2">
          <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleOpenPath(record.corePath) }} title="打开目录">
            <FolderOpen className="w-4 h-4" />
          </Button>
          <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleEdit(record) }}>
            编辑
          </Button>
          <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleRedownload(record) }}>
            重新下载
          </Button>
          {!record.isDefault && (
            <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); handleSetDefault(record.coreId) }}>
              设为默认
            </Button>
          )}
          <Button size="sm" variant="danger" onClick={(e) => { e.stopPropagation(); handleDeleteClick(record) }}>
            删除
          </Button>
        </div>
      ),
    },
  ]

  // 打开内核路径
  const handleOpenPath = async (corePath: string) => {
    try {
      await openCorePath(corePath)
    } catch (error: any) {
      toast.error(error?.message || '打开目录失败')
    }
  }

  // 扫描 chrome 目录，自动注册新内核
  const handleScan = async () => {
    setScanning(true)
    try {
      await scanBrowserCores()
      await loadData()
      toast.success('扫描完成')
    } catch (error: any) {
      toast.error(error?.message || '扫描失败')
    } finally {
      setScanning(false)
    }
  }

  const handleImportLocal = async () => {
    setImporting(true)
    setImportProgress({ phase: 'selecting', progress: 0, message: '请选择本地内核包...' })
    try {
      const imported = await importLocalBrowserCore()
      if (!imported) {
        setImportProgress(null)
        return
      }
      await loadData()
      toast.success(`已导入：${imported.coreName}`)
    } catch (error: any) {
      toast.error(error?.message || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  // 新增内核
  const handleAdd = () => {
    setEditingCore(null)
    setEditForm({ coreName: '', corePath: '', coreBackend: CORE_BACKEND_FINGERPRINT_CHROMIUM, coreEnv: '' })
    setPathValidResult(null)
    setEditModalOpen(true)
  }

  // 编辑内核
  const handleEdit = (record: CoreDisplayInfo) => {
    const core = cores.find(c => c.coreId === record.coreId)
    if (core) {
      setEditingCore(core)
      setEditForm({
        coreName: core.coreName,
        corePath: core.corePath,
        coreBackend: normalizeCoreBackend(core.coreBackend),
        coreEnv: (core.coreEnv || []).join('\n'),
      })
      setPathValidResult({ valid: record.pathValid, message: record.pathMessage })
      setEditModalOpen(true)
    }
  }

  const loadCoreDownloadRecommendation = async () => {
    setDownloadRecommendationLoading(true)
    setDownloadRecommendationError('')
    try {
      let runtimeEnv: { platform?: string; arch?: string } | null = null
      try {
        runtimeEnv = await Environment()
      } catch {
        runtimeEnv = null
      }
      const recommendation = await fetchCoreDownloadRecommendation(runtimeEnv)
      setDownloadRecommendation(recommendation)
      if (!recommendation) {
        setDownloadRecommendationError('当前环境没有可自动解压的 Release 压缩包')
        return
      }
      setDownloadForm(prev => ({
        ...prev,
        url: prev.url.trim() ? prev.url : recommendation.downloadUrl,
      }))
    } catch (error: any) {
      setDownloadRecommendation(null)
      setDownloadRecommendationError(error?.message || '获取推荐地址失败')
    } finally {
      setDownloadRecommendationLoading(false)
    }
  }

  const handleOpenDownload = () => {
    setDownloadForm({ name: '', url: '', proxyMode: 'system', proxyId: '', mode: 'download' })
    setDownloadProgress(null)
    setDownloadRecommendation(null)
    setDownloadRecommendationError('')
    setDownloadModalOpen(true)
    void loadCoreDownloadRecommendation()
  }

  const handleRedownload = (record: CoreDisplayInfo) => {
    setDownloadForm({ coreId: record.coreId, name: record.coreName, url: '', proxyMode: 'system', proxyId: '', mode: 'redownload' })
    setDownloadProgress(null)
    setDownloadRecommendation(null)
    setDownloadRecommendationError('')
    setDownloadModalOpen(true)
    void loadCoreDownloadRecommendation()
  }

  // 保存内核
  const handleSaveCore = async () => {
    if (!editForm.coreName.trim()) {
      toast.error('请输入内核名称')
      return
    }
    if (!editForm.corePath.trim()) {
      toast.error('请输入内核路径')
      return
    }
    setSaving(true)
    try {
      const input: BrowserCoreInput = {
        coreId: editingCore?.coreId || `core-${Date.now()}`,
        coreName: editForm.coreName.trim(),
        corePath: editForm.corePath.trim(),
        isDefault: editingCore?.isDefault || false,
        coreBackend: normalizeCoreBackend(editForm.coreBackend),
        coreEnv: parseCoreEnvInput(editForm.coreEnv),
      }
      await saveBrowserCore(input)
      await loadData()
      setEditModalOpen(false)
      toast.success(editingCore ? '内核已更新' : '内核已添加')
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  // 删除点击
  const handleDeleteClick = (record: CoreDisplayInfo) => {
    if (record.isDefault) {
      toast.warning('默认内核不能删除')
      return
    }
    setDeletingCore(record)
    setDeleteConfirmOpen(true)
  }

  // 确认删除
  const handleDeleteConfirm = async () => {
    if (!deletingCore) return
    try {
      await deleteBrowserCore(deletingCore.coreId)
      await loadData()
      toast.success('内核已删除')
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
    setDeletingCore(null)
  }

  // 设为默认
  const handleSetDefault = async (coreId: string) => {
    try {
      await setDefaultBrowserCore(coreId)
      await loadData()
      toast.success('已设为默认内核')
    } catch (error: any) {
      toast.error(error?.message || '设置失败')
    }
  }

  // 开始下载
  const handleStartDownloadCore = async () => {
    if (!downloadForm.name.trim() || !downloadForm.url.trim()) {
      toast.error('请输入名称和下载地址')
      return
    }
    if (downloadForm.mode === 'redownload' && !downloadForm.coreId) {
      toast.error('缺少内核ID')
      return
    }
    setDownloadProgress({ phase: 'starting', progress: 0, message: '准备下载...' })
    try {
      // 在这儿我们需要从 proxies 中寻找匹配到的代理设定，如果有则传过去的 url
      let targetProxy = ''
      if (downloadForm.proxyMode === 'system') {
        targetProxy = '__system__'
      } else if (downloadForm.proxyMode === 'direct') {
        targetProxy = '__direct__'
      } else {
        const proxyProfile = proxies.find(p => p.proxyId === downloadForm.proxyId)
        targetProxy = downloadForm.proxyId
        if (proxyProfile && proxyProfile.proxyConfig) {
          targetProxy = proxyProfile.proxyConfig
        }
      }

      if (downloadForm.mode === 'redownload') {
        await redownloadBrowserCore(downloadForm.coreId || '', downloadForm.url.trim(), targetProxy)
      } else {
        await BrowserCoreDownload(downloadForm.name.trim(), downloadForm.url.trim(), targetProxy)
      }
    } catch (err: any) {
      toast.error(err.message || '内部启动下载失败')
      setDownloadProgress(null)
    }
  }

  // 打开设置编辑弹窗
  const handleEditSettings = () => {
    setSettingsForm({
      userDataRoot: settings.userDataRoot,
      defaultFingerprintArgs: settings.defaultFingerprintArgs.join('\n'),
      defaultLaunchArgs: settings.defaultLaunchArgs.join('\n'),
      defaultStartUrls: settings.defaultStartUrls.join('\n'),
      lightStartEnabled: settings.lightStartEnabled,
      restoreLastSession: settings.restoreLastSession,
      startReadyTimeoutMs: settings.startReadyTimeoutMs,
      startStableWindowMs: settings.startStableWindowMs,
    })
    setSettingsModalOpen(true)
  }

  // 保存设置
  const handleSaveSettings = async () => {
    setSavingSettings(true)
    try {
      const newSettings: BrowserSettings = {
        userDataRoot: settingsForm.userDataRoot.trim(),
        defaultFingerprintArgs: settingsForm.defaultFingerprintArgs.split('\n').map(s => s.trim()).filter(Boolean),
        defaultLaunchArgs: settingsForm.defaultLaunchArgs.split('\n').map(s => s.trim()).filter(Boolean),
        defaultStartUrls: settingsForm.defaultStartUrls.split('\n').map(s => s.trim()).filter(Boolean),
        lightStartEnabled: settingsForm.lightStartEnabled,
        restoreLastSession: settingsForm.restoreLastSession,
        startReadyTimeoutMs: Math.max(1000, Number(settingsForm.startReadyTimeoutMs) || 3000),
        startStableWindowMs: Math.max(0, Number(settingsForm.startStableWindowMs) || 1200),
        defaultConnectorType: settings.defaultConnectorType || 'xray',
      }
      await saveBrowserSettings(newSettings)
      setSettings(newSettings)
      setSettingsModalOpen(false)
      toast.success('设置已保存')
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSavingSettings(false)
    }
  }


  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">内核管理</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">管理 Chrome 内核版本和全局设置</p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="secondary" onClick={handleOpenDownload}>下载内核</Button>
          <Button size="sm" variant="secondary" onClick={handleImportLocal} loading={importing}>导入本地</Button>
          <Button size="sm" variant="secondary" onClick={handleScan} loading={scanning}>扫描内核</Button>
          <Button size="sm" onClick={handleAdd}>新增内核</Button>
        </div>
      </div>

      {importProgress && (
        <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm">
          <span className="text-[var(--color-text-secondary)]">{importProgress.message}</span>
          <span className="text-[var(--color-text-muted)]">{Math.max(0, Math.min(100, importProgress.progress))}%</span>
        </div>
      )}

      <CoreSettingsCard settings={settings} onEdit={handleEditSettings} />

      {/* 内核列表卡片 */}
      <Card title="内核列表" subtitle="已配置的 Chrome 内核">
        <Table
          columns={columns}
          data={displayList}
          rowKey="coreId"
          loading={loading}
          emptyText="暂无内核，请添加内核"
        />
      </Card>

      <CoreSettingsModal
        open={settingsModalOpen}
        form={settingsForm}
        saving={savingSettings}
        setForm={setSettingsForm}
        onClose={() => setSettingsModalOpen(false)}
        onSave={handleSaveSettings}
      />

      <CoreEditModal
        open={editModalOpen}
        isEditing={Boolean(editingCore)}
        form={editForm}
        saving={saving}
        pathValidating={pathValidating}
        pathValidResult={pathValidResult}
        setForm={setEditForm}
        onClose={() => setEditModalOpen(false)}
        onSave={handleSaveCore}
      />

      {/* 删除确认弹窗 */}
      <ConfirmModal
        open={deleteConfirmOpen}
        onClose={() => setDeleteConfirmOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="确认删除"
        content={`确定要删除内核"${deletingCore?.coreName}"吗？此操作不可恢复。`}
        confirmText="删除"
        danger
      />

      <CoreDownloadModal
        open={downloadModalOpen}
        form={downloadForm}
        progress={downloadProgress}
        recommendation={downloadRecommendation}
        recommendationLoading={downloadRecommendationLoading}
        recommendationError={downloadRecommendationError}
        proxies={proxies}
        setForm={setDownloadForm}
        setProgress={setDownloadProgress}
        onRefreshRecommendation={loadCoreDownloadRecommendation}
        onClose={() => setDownloadModalOpen(false)}
        onStart={handleStartDownloadCore}
      />
    </div>
  )
}
