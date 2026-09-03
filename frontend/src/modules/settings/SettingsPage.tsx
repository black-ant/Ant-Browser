import { useEffect, useState } from 'react'
import { RotateCcw, Save } from 'lucide-react'

import { Button, Card, ThemeSwitcher, toast } from '../../shared/components'
import { useTheme } from '../../shared/theme'
import {
  automationProbeSystemNode,
  automationRuntimeSelfCheck,
  defaultAutomationState,
  fetchAutomationState,
  fetchLaunchServerSettings,
  fetchSettings,
  installAutomationRuntime,
  resetSettings,
  saveAutomationRuntimeSettings,
  saveAutomationScriptPackageSettings,
  saveAutomationSettings,
  saveLaunchServerSettings,
  saveSettings,
} from './api'
import type { AutomationNodeSource, AutomationRuntimeCheck, AutomationState, AutomationSystemNodeProbe } from './api'
import { AutomationSettingsCard } from './components/AutomationSettingsCard'
import { useSettingsProgressEffects } from './hooks/useSettingsProgressEffects'
import { SettingsAdvancedCard, SettingsBasicFeatureCards } from './components/SettingsGeneralCards'
import { defaultSettings } from './types'
import type { AppSettings } from './types'
import type { AutomationRuntimeProgress } from './progress'

type AutomationBusyState = 'none' | 'toggle' | 'probe' | 'runtime' | 'package' | 'install' | 'check'

export function SettingsPage() {
  const { setTheme } = useTheme()
  const [settings, setSettings] = useState<AppSettings>(defaultSettings)
  const [automationState, setAutomationState] = useState<AutomationState>(defaultAutomationState)
  const [automationProgress, setAutomationProgress] = useState<AutomationRuntimeProgress | null>(null)
  const [automationBusy, setAutomationBusy] = useState<AutomationBusyState>('none')
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

  useEffect(() => {
    void loadSettings()
  }, [])

  useSettingsProgressEffects({
    setAutomationProgress,
    setAutomationState,
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
      const [data, automation, launchServer] = await Promise.all([
        fetchSettings(),
        fetchAutomationState(),
        fetchLaunchServerSettings(),
      ])
      setSettings(data)
      setAutomationState(automation)
      setLaunchServerPortDraft(String(launchServer.preferredPort || launchServer.port || 19876))
      setLaunchServerBaseUrl(launchServer.baseUrl)
      setLaunchServerReady(launchServer.ready)
      setHasChanges(false)
    } catch (error: any) {
      toast.error(error?.message || '系统设置加载失败')
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
    if (!confirm('确定要重置所有设置吗？将重置通用设置、自动化、备份渠道和 Launch Server；不会删除浏览器实例、代理或内核数据。')) return

    try {
      const nextSettings = await resetSettings()
      const [automation, launchServer] = await Promise.all([
        fetchAutomationState(),
        fetchLaunchServerSettings(),
      ])
      setSettings(nextSettings)
      setTheme('light')
      setAutomationState(automation)
      setAutomationProgress(null)
      setAutomationCheck(null)
      setAutomationProbe(null)
      setAutomationNodeSourceDraft((automation.settings.nodeSource || 'auto') as AutomationNodeSource)
      setAutomationSystemNodePathDraft(automation.settings.systemNodePath || '')
      setAutomationRuntimeDirty(false)
      setLaunchServerPortDraft(String(launchServer.preferredPort || launchServer.port || 19876))
      setLaunchServerBaseUrl(launchServer.baseUrl)
      setLaunchServerReady(launchServer.ready)
      setHasChanges(false)
      toast.success('设置已重置')
    } catch (error: any) {
      toast.error(error?.message || '设置重置失败')
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
        setAutomationProgress({ phase: 'checking', progress: 0, message: '已开启自动化支持，正在准备运行时...' })
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
        setAutomationProgress({ phase: 'checking', progress: 0, message: '运行时策略已保存，正在重新检查自动化运行时...' })
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
      setAutomationProgress({ phase: 'checking', progress: 0, message: '正在准备自动化运行时...' })
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

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--color-border-default)] border-t-[var(--color-accent)]" />
      </div>
    )
  }

  return (
    <div className="w-full space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">系统设置</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleReset}>
            <RotateCcw className="h-4 w-4" />
            重置
          </Button>
          <Button variant="danger" size="sm" onClick={handleSave} loading={saving} disabled={!hasChanges}>
            <Save className="h-4 w-4" />
            保存
          </Button>
        </div>
      </div>

      <Card title="主题">
        <ThemeSwitcher />
      </Card>
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
      <SettingsAdvancedCard settings={settings} onChange={handleChange} />
    </div>
  )
}
