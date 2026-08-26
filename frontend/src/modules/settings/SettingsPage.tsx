import { useEffect, useRef, useState } from 'react'
import { Save, RotateCcw } from 'lucide-react'
import { Card, Button, ThemeSwitcher, toast } from '../../shared/components'
import {
  fetchSettings,
  saveSettings,
  resetSettings,
  initializeSystemData,
  exportSystemConfig,
  importSystemConfig,
  fetchAutomationState,
  saveAutomationScriptPackageSettings,
  saveAutomationSettings,
  saveAutomationRuntimeSettings,
  installAutomationRuntime,
  automationProbeSystemNode,
  automationRuntimeSelfCheck,
  fetchLaunchServerSettings,
  saveLaunchServerSettings,
  defaultAutomationState,
} from './api'
import type { AppSettings } from './types'
import type { AutomationNodeSource, AutomationRuntimeCheck, AutomationState, AutomationSystemNodeProbe } from './api'
import { defaultSettings } from './types'
import { AutomationSettingsCard } from './components/AutomationSettingsCard'
import { BackupImportModal, BackupSettingsCard } from './components/BackupSettingsCard'
import { OpenListBackupModal } from './components/OpenListBackupModal'
import { ScheduledBackupModal } from './components/ScheduledBackupModal'
import { SettingsAdvancedCard, SettingsBasicFeatureCards } from './components/SettingsGeneralCards'
import type { AutomationRuntimeProgress, BackupExportLogItem, BackupExportProgress } from './progress'
import { useSettingsProgressEffects } from './hooks/useSettingsProgressEffects'
import {
  defaultScheduledBackupSettings,
  fetchScheduledBackupSettings,
} from './openListApi'
import type { ScheduledBackupSettings } from './openListApi'

export function SettingsPage() {
  const [settings, setSettings] = useState<AppSettings>(defaultSettings)
  const [automationState, setAutomationState] = useState<AutomationState>(defaultAutomationState)
  const [automationProgress, setAutomationProgress] = useState<AutomationRuntimeProgress | null>(null)
  const [automationBusy, setAutomationBusy] = useState<'none' | 'toggle' | 'probe' | 'runtime' | 'package' | 'install' | 'check'>('none')
  const [automationCheck, setAutomationCheck] = useState<AutomationRuntimeCheck | null>(null)
  const [automationProbe, setAutomationProbe] = useState<AutomationSystemNodeProbe | null>(null)
  const [automationNodeSourceDraft, setAutomationNodeSourceDraft] = useState<AutomationNodeSource>('auto')
  const [automationSystemNodePathDraft, setAutomationSystemNodePathDraft] = useState('')
  const [automationRuntimeDirty, setAutomationRuntimeDirty] = useState(false)
  const [launchServerPortDraft, setLaunchServerPortDraft] = useState('19876')
  const [launchServerBaseUrl, setLaunchServerBaseUrl] = useState('')
  const [launchServerReady, setLaunchServerReady] = useState(false)
  const [launchServerSaving, setLaunchServerSaving] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [hasChanges, setHasChanges] = useState(false)
  const [importModalOpen, setImportModalOpen] = useState(false)
  const [openListModalOpen, setOpenListModalOpen] = useState(false)
  const [openListModalMode, setOpenListModalMode] = useState<'backup' | 'history'>('backup')
  const [remoteBackupBusy, setRemoteBackupBusy] = useState(false)
  const [scheduledBackupModalOpen, setScheduledBackupModalOpen] = useState(false)
  const [scheduledBackupBusy, setScheduledBackupBusy] = useState(false)
  const [scheduledBackup, setScheduledBackup] = useState<ScheduledBackupSettings>(defaultScheduledBackupSettings)
  const [actionLoading, setActionLoading] = useState<'none' | 'init' | 'export' | 'import-reset' | 'import-merge'>('none')
  const [exportProgress, setExportProgress] = useState<BackupExportProgress | null>(null)
  const [importProgress, setImportProgress] = useState<BackupExportProgress | null>(null)
  const [exportLogs, setExportLogs] = useState<BackupExportLogItem[]>([])
  const exportLogsRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    loadSettings()
  }, [])

  useEffect(() => {
    const timer = window.setInterval(() => {
      void fetchScheduledBackupSettings().then(setScheduledBackup).catch(() => {})
    }, 30_000)
    return () => window.clearInterval(timer)
  }, [])

  useSettingsProgressEffects({
    actionLoading,
    exportLogs,
    exportLogsRef,
    importProgress,
    setAutomationProgress,
    setAutomationState,
    setExportLogs,
    setExportProgress,
    setImportProgress,
  })

  useEffect(() => {
    setAutomationNodeSourceDraft((automationState.settings.nodeSource || 'auto') as AutomationNodeSource)
    setAutomationSystemNodePathDraft(automationState.settings.systemNodePath || '')
    setAutomationProbe(null)
    setAutomationRuntimeDirty(false)
  }, [automationState.settings.nodeSource, automationState.settings.systemNodePath])

  const loadSettings = async () => {
    setLoading(true)
    try {
      const [data, automation, launchServer, scheduled] = await Promise.all([
        fetchSettings(),
        fetchAutomationState(),
        fetchLaunchServerSettings(),
        fetchScheduledBackupSettings(),
      ])
      setSettings(data)
      setAutomationState(automation)
      setLaunchServerPortDraft(String(launchServer.preferredPort || launchServer.port || 19876))
      setLaunchServerBaseUrl(launchServer.baseUrl)
      setLaunchServerReady(launchServer.ready)
      setScheduledBackup(scheduled)
    } finally {
      setLoading(false)
    }
  }

  const handleChange = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    setSettings(prev => ({ ...prev, [key]: value }))
    setHasChanges(true)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const success = await saveSettings(settings)
      if (success) {
        setHasChanges(false)
        toast.success('设置已保存')
      }
    } catch (error: any) {
      toast.error(error?.message || '保存失败，请检查配置')
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    if (confirm('确定要重置所有设置吗？')) {
      const data = await resetSettings()
      setSettings(data)
      setHasChanges(false)
    }
  }

  const handleAutomationEnabledChange = async (enabled: boolean) => {
    setAutomationBusy('toggle')
    setAutomationCheck(null)
    try {
      const next = await saveAutomationSettings(enabled, automationState.settings.headlessDefault)
      setAutomationState(next)
      if (!enabled) {
        setAutomationProgress(null)
        toast.success('自动化支持已关闭')
        return
      }
      if (!next.status.ready) {
        setAutomationProgress({
          phase: 'checking',
          progress: 0,
          message: '已开启自动化支持，正在准备运行时...',
        })
        toast.success('自动化支持已开启，正在准备运行时')
        return
      }
      toast.success('自动化支持已开启')
    } catch (error: any) {
      toast.error(error?.message || '自动化配置保存失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleAutomationHeadlessChange = async (headlessDefault: boolean) => {
    setAutomationBusy('toggle')
    try {
      const next = await saveAutomationSettings(automationState.settings.enabled, headlessDefault)
      setAutomationState(next)
      toast.success(headlessDefault ? '默认无头模式已开启' : '默认无头模式已关闭')
    } catch (error: any) {
      toast.error(error?.message || '自动化配置保存失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleAutomationRuntimeSettingsSave = async () => {
    setAutomationBusy('runtime')
    setAutomationCheck(null)
    try {
      const next = await saveAutomationRuntimeSettings(automationNodeSourceDraft, automationSystemNodePathDraft)
      setAutomationState(next)
      setAutomationRuntimeDirty(false)

      if (next.settings.enabled && next.status.installing) {
        setAutomationProgress({
          phase: 'checking',
          progress: 0,
          message: '运行时策略已保存，正在重新检查自动化运行时...',
        })
        toast.success('运行时策略已保存，正在重新检查')
        return
      }

      toast.success('运行时策略已保存')
    } catch (error: any) {
      toast.error(error?.message || '运行时策略保存失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleAutomationTypeScriptBuildChange = async (allowTypeScriptBuild: boolean) => {
    setAutomationBusy('package')
    try {
      const next = await saveAutomationScriptPackageSettings(allowTypeScriptBuild)
      setAutomationState(next)
      toast.success(allowTypeScriptBuild ? 'TypeScript 导入构建已开启' : 'TypeScript 导入构建已关闭')
    } catch (error: any) {
      toast.error(error?.message || '脚本包配置保存失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleAutomationProbeSystemNode = async () => {
    setAutomationBusy('probe')
    try {
      const result = await automationProbeSystemNode(automationSystemNodePathDraft)
      setAutomationProbe(result)
      toast.success(`系统 Node 可用：${result.version}`)
    } catch (error: any) {
      setAutomationProbe(null)
      toast.error(error?.message || '系统 Node 检测失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleAutomationInstall = async () => {
    setAutomationBusy('install')
    try {
      const next = await installAutomationRuntime()
      setAutomationState(next)
      setAutomationProgress({
        phase: 'checking',
        progress: 0,
        message: '正在准备自动化运行时...',
      })
      toast.success('已开始准备自动化运行时')
    } catch (error: any) {
      toast.error(error?.message || '启动自动化运行时安装失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleAutomationSelfCheck = async () => {
    setAutomationBusy('check')
    try {
      const result = await automationRuntimeSelfCheck()
      setAutomationCheck(result)
      if (result.ok) {
        toast.success(`自检通过：Node ${result.nodeVersion} / playwright-core ${result.playwrightVersion}`)
      } else {
        toast.warning('自检未通过')
      }
    } catch (error: any) {
      setAutomationCheck(null)
      toast.error(error?.message || '自动化运行时自检失败')
    } finally {
      setAutomationBusy('none')
    }
  }

  const handleLaunchServerPortSave = async () => {
    const port = Number(launchServerPortDraft)
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      toast.error('端口必须在 1-65535 之间')
      return
    }

    setLaunchServerSaving(true)
    try {
      const next = await saveLaunchServerSettings(port)
      setLaunchServerPortDraft(String(next.preferredPort || next.port || port))
      setLaunchServerBaseUrl(next.baseUrl)
      setLaunchServerReady(next.ready)
      toast.success(`本地 API 端口已保存：${next.port}`)
    } catch (error: any) {
      toast.error(error?.message || '本地 API 端口保存失败')
    } finally {
      setLaunchServerSaving(false)
    }
  }

  const handleInitializeSystem = async () => {
    if (!confirm('初始化会清空当前数据并恢复默认状态，是否继续？')) {
      return
    }
    setActionLoading('init')
    try {
      const res = await initializeSystemData()
      if (res.cancelled) {
        toast.info('已取消初始化')
        return
      }
      toast.success(res.message || '初始化完成')
    } catch (error: any) {
      toast.error(error?.message || '初始化失败')
    } finally {
      setActionLoading('none')
    }
  }

  const handleExportSystem = async () => {
    setActionLoading('export')
    setExportLogs([])
    setExportProgress({ phase: 'starting', progress: 0, message: '准备导出...' })
    try {
      const res = await exportSystemConfig()
      if (res.cancelled) {
        setExportProgress(null)
        setExportLogs([])
        toast.info('已取消导出')
        return
      }
      const fileHint = Number.isFinite(res.fileCount) && (res.fileCount || 0) > 0
        ? `，共 ${res.fileCount} 个文件`
        : ''
      const resultMessage = res.zipPath
        ? `导出完成：${res.zipPath}${fileHint}`
        : `${res.message || '导出完成'}${fileHint}`
      setExportProgress(prev => prev?.phase === 'done'
        ? prev
        : { phase: 'done', progress: 100, message: resultMessage })
      toast.success(resultMessage)
    } catch (error: any) {
      setExportProgress(prev => ({
        phase: 'error',
        progress: prev?.progress ?? 0,
        message: error?.message || '导出失败',
      }))
      setExportLogs(prev => {
        const timestamp = new Date().toLocaleTimeString('zh-CN', { hour12: false })
        const text = error?.message || '导出失败'
        const next = [...prev, { id: Date.now() + Math.floor(Math.random() * 1000), phase: 'error', time: timestamp, text }]
        return next.length > 120 ? next.slice(next.length - 120) : next
      })
      toast.error(error?.message || '导出失败')
    } finally {
      setActionLoading('none')
    }
  }

  const handleImportSystem = async (resetFirst: boolean) => {
    setActionLoading(resetFirst ? 'import-reset' : 'import-merge')
    setImportProgress({
      phase: 'starting',
      progress: 0,
      message: resetFirst ? '等待选择 ZIP 备份（清空后恢复）...' : '等待选择 ZIP 备份（合并导入）...',
    })
    try {
      const res = await importSystemConfig(resetFirst)
      if (res.cancelled) {
        setImportProgress(null)
        toast.info('已取消导入')
        return
      }
      const imported = res.imported ?? 0
      const skipped = res.skipped ?? 0
      const conflicts = res.conflicts ?? 0
      const componentFailed = Number.isFinite(res.componentFailed) ? Math.max(0, Math.round(res.componentFailed || 0)) : 0
      const componentTotal = Number.isFinite(res.componentTotal) ? Math.max(0, Math.round(res.componentTotal || 0)) : 0
      const failedComponents = Array.isArray(res.failedComponents) ? res.failedComponents : []

      if (res.partial || componentFailed > 0) {
        const moduleNames = failedComponents
          .map(item => (item?.componentName || item?.componentId || '').trim())
          .filter(Boolean)
        const moduleHint = moduleNames.length > 0
          ? `：${moduleNames.slice(0, 3).join('、')}${moduleNames.length > 3 ? ` 等 ${moduleNames.length} 个模块` : ''}`
          : ''
        if (componentTotal > 0) {
          const componentSuccess = Math.max(0, componentTotal - componentFailed)
          toast.warning(`导入完成（部分成功）：模块成功 ${componentSuccess}/${componentTotal}，异常 ${componentFailed}${moduleHint}`)
        } else {
          toast.warning(`导入完成（部分成功）：异常模块 ${componentFailed}${moduleHint}`)
        }
      } else {
      toast.success(`导入完成：导入 ${imported}，跳过 ${skipped}，冲突 ${conflicts}`)
      }
      setImportModalOpen(false)
      setImportProgress(null)
    } catch (error: any) {
      setImportProgress(prev => ({
        phase: 'error',
        progress: prev?.progress ?? 0,
        message: error?.message || '导入失败',
      }))
      toast.error(error?.message || '导入失败')
    } finally {
      setActionLoading('none')
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-6 h-6 border-2 border-[var(--color-border-default)] border-t-[var(--color-accent)] rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="space-y-6 w-full animate-fade-in">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">系统设置</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">配置应用的各项参数</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleReset}>
            <RotateCcw className="w-4 h-4" />
            重置
          </Button>
          <Button variant="danger" size="sm" onClick={handleSave} loading={saving} disabled={!hasChanges}>
            <Save className="w-4 h-4" />
            保存
          </Button>
        </div>
      </div>

      <BackupSettingsCard
        actionLoading={actionLoading}
        exportProgress={exportProgress}
        exportLogs={exportLogs}
        exportLogsRef={exportLogsRef}
        onInitialize={() => { void handleInitializeSystem() }}
        onExport={() => { void handleExportSystem() }}
        onOpenImport={() => {
          setImportProgress(null)
          setImportModalOpen(true)
        }}
        onOpenOpenListBackup={() => {
          setOpenListModalMode('backup')
          setOpenListModalOpen(true)
        }}
        onOpenOpenListHistory={() => {
          setOpenListModalMode('history')
          setOpenListModalOpen(true)
        }}
        onOpenScheduledBackup={() => setScheduledBackupModalOpen(true)}
        scheduledBackup={scheduledBackup}
        remoteBusy={remoteBackupBusy || scheduledBackupBusy}
      />

      {/* 主题设置 */}
      <Card title="主题设置" subtitle="选择您喜欢的界面主题">
        <ThemeSwitcher />
      </Card>

      {/* 基础设置 */}
      <SettingsBasicFeatureCards settings={settings} onChange={handleChange} />
      <AutomationSettingsCard
        automationState={automationState}
        automationProgress={automationProgress}
        automationBusy={automationBusy}
        automationCheck={automationCheck}
        automationProbe={automationProbe}
        automationNodeSourceDraft={automationNodeSourceDraft}
        automationSystemNodePathDraft={automationSystemNodePathDraft}
        automationRuntimeDirty={automationRuntimeDirty}
        launchServerPortDraft={launchServerPortDraft}
        launchServerBaseUrl={launchServerBaseUrl}
        launchServerReady={launchServerReady}
        launchServerSaving={launchServerSaving}
        onEnabledChange={handleAutomationEnabledChange}
        onHeadlessChange={handleAutomationHeadlessChange}
        onNodeSourceDraftChange={(value) => {
          setAutomationNodeSourceDraft(value)
          setAutomationProbe(null)
          setAutomationRuntimeDirty(true)
        }}
        onSystemNodePathDraftChange={(value) => {
          setAutomationSystemNodePathDraft(value)
          setAutomationProbe(null)
          setAutomationRuntimeDirty(true)
        }}
        onLaunchServerPortDraftChange={setLaunchServerPortDraft}
        onSaveLaunchServerPort={() => { void handleLaunchServerPortSave() }}
        onTypeScriptBuildChange={handleAutomationTypeScriptBuildChange}
        onProbeSystemNode={() => { void handleAutomationProbeSystemNode() }}
        onSaveRuntimeSettings={() => { void handleAutomationRuntimeSettingsSave() }}
        onInstall={() => { void handleAutomationInstall() }}
        onSelfCheck={() => { void handleAutomationSelfCheck() }}
      />

      {/* 高级设置 */}
      <SettingsAdvancedCard settings={settings} onChange={handleChange} />

      <BackupImportModal
        open={importModalOpen}
        actionLoading={actionLoading}
        importProgress={importProgress}
        onClose={() => {
          setImportModalOpen(false)
          setImportProgress(null)
        }}
        onImport={(resetFirst) => { void handleImportSystem(resetFirst) }}
      />

      <OpenListBackupModal
        open={openListModalOpen}
        mode={openListModalMode}
        onClose={() => setOpenListModalOpen(false)}
        onBusyChange={setRemoteBackupBusy}
      />

      <ScheduledBackupModal
        open={scheduledBackupModalOpen}
        onClose={() => setScheduledBackupModalOpen(false)}
        onSaved={setScheduledBackup}
        onBusyChange={setScheduledBackupBusy}
      />

    </div>
  )
}
