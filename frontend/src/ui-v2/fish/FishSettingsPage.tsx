import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Download, RefreshCw, Save, ShieldCheck } from 'lucide-react'
import { toast } from '../../shared/components'
import { themeConfigs, useTheme, type ThemeType } from '../../shared/theme'
import {
  automationProbeSystemNode,
  automationRuntimeSelfCheck,
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
  type AutomationNodeSource,
  type AutomationRuntimeCheck,
  type AutomationState,
  type AutomationSystemNodeProbe,
  type LaunchServerSettings,
} from '../../modules/settings/api'
import type { AppSettings } from '../../modules/settings/types'
import type { AutomationRuntimeProgress } from '../../modules/settings/progress'
import { FishConfirm, FishSwitch } from './FishFormPrimitives'
import './fish-modules.css'

type Tab = 'general' | 'automation' | 'backup' | 'advanced'
type Action = 'none' | 'runtime'

export function FishSettingsPage() {
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()
  const [tab, setTab] = useState<Tab>('general')
  const [settings, setSettings] = useState<AppSettings | null>(null)
  const [automation, setAutomation] = useState<AutomationState | null>(null)
  const [launch, setLaunch] = useState<LaunchServerSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [action, setAction] = useState<Action>('none')
  const [automationProgress, setAutomationProgress] = useState<AutomationRuntimeProgress | null>(null)
  const [nodePath, setNodePath] = useState('')
  const [nodeProbe, setNodeProbe] = useState<AutomationSystemNodeProbe | null>(null)
  const [runtimeCheck, setRuntimeCheck] = useState<AutomationRuntimeCheck | null>(null)
  const [resetSettingsOpen, setResetSettingsOpen] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [nextSettings, nextAutomation, nextLaunch] = await Promise.all([
        fetchSettings(),
        fetchAutomationState(),
        fetchLaunchServerSettings(),
      ])
      setSettings(nextSettings)
      setAutomation(nextAutomation)
      setLaunch(nextLaunch)
      setNodePath(nextAutomation.settings.systemNodePath || nextAutomation.status.systemNodePath || '')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '设置加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    const runtime = (window as Window & {
      runtime?: { EventsOn?: (name: string, cb: (payload: any) => void) => (() => void) | void }
    }).runtime
    const offRuntime = runtime?.EventsOn?.('automation:runtime:progress', (payload: any) => {
      const next: AutomationRuntimeProgress = {
        phase: String(payload?.phase || 'working'),
        progress: Number(payload?.progress) || 0,
        message: String(payload?.message || '正在准备自动化运行时…'),
        component: payload?.component,
      }
      setAutomationProgress(next)
      if (next.phase === 'done' || next.phase === 'error') {
        void fetchAutomationState().then(setAutomation).catch(() => {})
      }
    }) || (() => {})
    return () => offRuntime()
  }, [])

  const saveGeneral = async () => {
    if (!settings) return
    setSaving(true)
    try {
      const ok = await saveSettings(settings)
      if (!ok) throw new Error('保存失败')
      toast.success('通用设置已保存')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const resetGeneral = async () => {
    try {
      const next = await resetSettings()
      setSettings(next)
      setResetSettingsOpen(false)
      toast.success('已恢复默认设置')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '恢复默认设置失败')
    }
  }

  const saveAutomation = async (patch?: { enabled?: boolean; headless?: boolean }) => {
    if (!automation) return
    setSaving(true)
    try {
      const next = await saveAutomationSettings(
        patch?.enabled ?? automation.settings.enabled,
        patch?.headless ?? automation.settings.headlessDefault,
      )
      setAutomation(next)
      toast.success('自动化设置已保存')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const saveRuntime = async (source?: AutomationNodeSource) => {
    if (!automation) return
    setSaving(true)
    try {
      let next = await saveAutomationRuntimeSettings(source || automation.settings.nodeSource, nodePath)
      next = await saveAutomationScriptPackageSettings(automation.settings.allowTypeScriptBuild)
      setAutomation(next)
      toast.success('运行时设置已保存')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const installRuntime = async () => {
    setAction('runtime')
    setAutomationProgress({ phase: 'starting', progress: 0, message: '准备安装自动化运行时…' })
    try {
      setAutomation(await installAutomationRuntime())
      toast.success('自动化运行时安装完成')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '安装失败')
    } finally {
      setAction('none')
    }
  }

  const probe = async () => {
    try {
      const result = await automationProbeSystemNode(nodePath)
      setNodeProbe(result)
      result.ok ? toast.success(`检测到 Node ${result.version}`) : toast.warning('未检测到可用系统 Node')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '检测失败')
    }
  }

  const selfCheck = async () => {
    try {
      const result = await automationRuntimeSelfCheck()
      setRuntimeCheck(result)
      result.ok ? toast.success('运行时自检通过') : toast.warning('运行时自检未通过')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '自检失败')
    }
  }

  const saveLaunch = async () => {
    if (!launch) return
    setSaving(true)
    try {
      setLaunch(await saveLaunchServerSettings(launch.preferredPort))
      toast.success('Launch API 端口已保存')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (loading || !settings || !automation || !launch) {
    return <div className="fish-content"><div className="fish-data-empty"><strong>正在加载设置</strong></div></div>
  }

  return (
    <div className="fish-content fish-module-content">
      <section className="fish-module-hero">
        <div><span className="fish-eyebrow">设置</span><h1>应用设置</h1><p>主题、自动化、备份与运行参数。</p></div>
      </section>
      <section className="fish-module-strip">
        <div><span>当前主题</span><strong>{themeConfigs.find((item) => item.id === theme)?.name || theme}</strong></div>
        <div><span>自动化</span><strong>{automation.settings.enabled ? '开启' : '关闭'}</strong></div>
        <div><span>运行时</span><strong>{automation.status.ready ? 'Ready' : automation.status.installed ? 'Installed' : 'Not installed'}</strong></div>
        <div><span>Launch API</span><strong>{launch.ready ? `${launch.port}` : '未就绪'}</strong></div>
      </section>
      <div className="fish-module-toolbar">
        <div className="fish-segments">
          {([['general', '通用'], ['automation', '自动化'], ['backup', '备份与系统'], ['advanced', '高级']] as [Tab, string][]).map(([value, label]) => (
            <button className={tab === value ? 'active' : ''} key={value} onClick={() => setTab(value)}>{label}</button>
          ))}
        </div>
      </div>

      {tab === 'general' ? (
        <section className="fish-module-section fish-form-grid">
          <div className="fish-module-section-head">
            <div><strong>应用与外观</strong><span>保存在本地应用设置中。</span></div>
            <button className="fish-btn primary" disabled={saving} onClick={() => void saveGeneral()}><Save size={16} strokeWidth={1.8} />{saving ? '保存中' : '保存'}</button>
          </div>
          <div className="fish-form-grid two">
            <label className="fish-field"><span>应用名称</span><input className="fish-input" value={settings.appName} onChange={(event) => setSettings({ ...settings, appName: event.target.value })} /></label>
            <label className="fish-field"><span>语言</span><select className="fish-select" value={settings.language} onChange={(event) => setSettings({ ...settings, language: event.target.value })}><option value="zh-CN">简体中文</option><option value="en-US">English</option></select></label>
          </div>
          <label className="fish-field"><span>应用描述</span><textarea className="fish-textarea compact" rows={3} value={settings.appDescription} onChange={(event) => setSettings({ ...settings, appDescription: event.target.value })} /></label>
          <div className="fish-setting-list">
            {themeConfigs.map((item) => <div className="fish-setting-row" key={item.id}><div><strong>{item.name}</strong><span>{item.description}</span></div><button className={`fish-btn${theme === item.id ? ' active' : ''}`} onClick={() => setTheme(item.id as ThemeType)}>{theme === item.id ? '当前' : '使用'}</button></div>)}
          </div>
          <div className="fish-setting-row"><div><strong>通知</strong><span>显示应用通知与异常提醒。</span></div><FishSwitch checked={settings.enableNotifications} onChange={(value) => setSettings({ ...settings, enableNotifications: value })} /></div>
          <div className="fish-setting-row"><div><strong>自动保存</strong><span>允许编辑页周期性保存草稿。</span></div><FishSwitch checked={settings.enableAutoSave} onChange={(value) => setSettings({ ...settings, enableAutoSave: value })} /></div>
          {settings.enableAutoSave ? <label className="fish-field"><span>自动保存间隔（秒）</span><input className="fish-input" type="number" min={5} value={settings.autoSaveInterval} onChange={(event) => setSettings({ ...settings, autoSaveInterval: Number(event.target.value) || 30 })} /></label> : null}
          <button className="fish-btn" onClick={() => setResetSettingsOpen(true)}>恢复默认设置</button>
        </section>
      ) : null}

      {tab === 'automation' ? (
        <section className="fish-module-section fish-form-grid">
          <div className="fish-setting-row"><div><strong>启用自动化</strong><span>允许 Playwright CDP / Launch API 脚本运行。</span></div><FishSwitch checked={automation.settings.enabled} onChange={(value) => void saveAutomation({ enabled: value })} /></div>
          <div className="fish-setting-row"><div><strong>默认无头模式</strong><span>脚本未单独声明时使用 headless。</span></div><FishSwitch checked={automation.settings.headlessDefault} onChange={(value) => void saveAutomation({ headless: value })} /></div>
          <div className="fish-form-grid two">
            <label className="fish-field"><span>Node 来源</span><select className="fish-select" value={automation.settings.nodeSource} onChange={(event) => setAutomation({ ...automation, settings: { ...automation.settings, nodeSource: event.target.value } })}><option value="auto">自动</option><option value="system">系统 Node</option><option value="bundled">内置 Node</option></select></label>
            <label className="fish-field"><span>系统 Node 路径</span><input className="fish-input" value={nodePath} onChange={(event) => setNodePath(event.target.value)} placeholder="node.exe 或目录" /></label>
          </div>
          <div className="fish-setting-row"><div><strong>允许 TypeScript 构建</strong><span>导入脚本包时允许 TS 编译。</span></div><FishSwitch checked={automation.settings.allowTypeScriptBuild} onChange={(value) => setAutomation({ ...automation, settings: { ...automation.settings, allowTypeScriptBuild: value } })} /></div>
          <div className="fish-inline-actions">
            <button className="fish-btn primary" disabled={saving} onClick={() => void saveRuntime(automation.settings.nodeSource as AutomationNodeSource)}><Save size={16} strokeWidth={1.8} />保存运行时设置</button>
            <button className="fish-btn" onClick={() => void probe()}><RefreshCw size={16} strokeWidth={1.8} />检测系统 Node</button>
            <button className="fish-btn" onClick={() => void selfCheck()}><ShieldCheck size={16} strokeWidth={1.8} />运行时自检</button>
            <button className="fish-btn" disabled={action === 'runtime'} onClick={() => void installRuntime()}><Download size={16} strokeWidth={1.8} />{action === 'runtime' ? '安装中' : '安装运行时'}</button>
          </div>
          {nodeProbe ? <div className={`fish-inline-note${nodeProbe.ok ? ' success' : ' danger'}`}>{nodeProbe.ok ? `${nodeProbe.path} · ${nodeProbe.version}` : '系统 Node 不可用'}</div> : null}
          {runtimeCheck ? <div className={`fish-inline-note${runtimeCheck.ok ? ' success' : ' danger'}`}>{runtimeCheck.ok ? `Node ${runtimeCheck.nodeVersion} · Playwright ${runtimeCheck.playwrightVersion}` : '自动化运行时自检未通过'}</div> : null}
          {automationProgress ? <Progress progress={automationProgress.progress} message={automationProgress.message} /> : null}
          <div className="fish-module-section">
            <div className="fish-module-section-head"><div><strong>Launch API</strong><span>{launch.baseUrl}</span></div></div>
            <div className="fish-form-grid two">
              <label className="fish-field"><span>Host</span><input className="fish-input" readOnly value={launch.host} /></label>
              <label className="fish-field"><span>首选端口</span><input className="fish-input" type="number" min={1} max={65535} value={launch.preferredPort} onChange={(event) => setLaunch({ ...launch, preferredPort: Number(event.target.value) || 19876 })} /></label>
            </div>
            <button className="fish-btn" disabled={saving} onClick={() => void saveLaunch()}>保存端口</button>
          </div>
        </section>
      ) : null}

      {tab === 'backup' ? (
        <section className="fish-module-section fish-form-grid">
          <div className="fish-module-section-head">
            <div><strong>备份与恢复</strong><span>最新上游已经提供独立备份中心，统一管理本地、OpenList、S3、定时备份、恢复历史和冲突处理。</span></div>
          </div>
          <div className="fish-setting-row">
            <div><strong>打开备份中心</strong><span>避免在设置页维护第二套备份状态和导入流程。</span></div>
            <button className="fish-btn primary" type="button" onClick={() => navigate('/system/backup')}>进入备份中心</button>
          </div>
        </section>
      ) : null}

      {tab === 'advanced' ? (
        <section className="fish-module-section fish-form-grid">
          <div className="fish-module-section-head">
            <div><strong>高级运行参数</strong><span>这些设置主要影响应用自身资源与日志策略。</span></div>
            <button className="fish-btn primary" disabled={saving} onClick={() => void saveGeneral()}><Save size={16} strokeWidth={1.8} />保存</button>
          </div>
          <div className="fish-form-grid two">
            <label className="fish-field"><span>最大上传 MB</span><input className="fish-input" type="number" min={1} value={settings.maxUploadSize} onChange={(event) => setSettings({ ...settings, maxUploadSize: Number(event.target.value) || 10 })} /></label>
            <label className="fish-field"><span>Session Timeout 分钟</span><input className="fish-input" type="number" min={1} value={settings.sessionTimeout} onChange={(event) => setSettings({ ...settings, sessionTimeout: Number(event.target.value) || 30 })} /></label>
            <label className="fish-field"><span>日志级别</span><select className="fish-select" value={settings.logLevel} onChange={(event) => setSettings({ ...settings, logLevel: event.target.value as AppSettings['logLevel'] })}><option value="debug">debug</option><option value="info">info</option><option value="warn">warn</option><option value="error">error</option></select></label>
            <label className="fish-field"><span>最大内存 MB</span><input className="fish-input" type="number" min={128} value={settings.maxMemoryMB} onChange={(event) => setSettings({ ...settings, maxMemoryMB: Number(event.target.value) || 1024 })} /></label>
            <label className="fish-field"><span>GC Percent</span><input className="fish-input" type="number" min={10} value={settings.gcPercent} onChange={(event) => setSettings({ ...settings, gcPercent: Number(event.target.value) || 100 })} /></label>
          </div>
          <div className="fish-setting-row"><div><strong>缓存</strong><span>启用应用级缓存。</span></div><FishSwitch checked={settings.cacheEnabled} onChange={(value) => setSettings({ ...settings, cacheEnabled: value })} /></div>
        </section>
      ) : null}

      <FishConfirm
        open={resetSettingsOpen}
        title="恢复默认设置"
        content="将清除本地 AppSettings 并恢复默认值。"
        confirmText="恢复默认"
        danger
        onClose={() => setResetSettingsOpen(false)}
        onConfirm={() => void resetGeneral()}
      />
    </div>
  )
}

function Progress({ progress, message }: { progress: number; message: string }) {
  const normalized = Math.max(0, Math.min(100, progress))
  return <div className="fish-progress-card"><div><strong>{message}</strong><span>{normalized}%</span></div><div className="fish-progress-line"><span style={{ width: `${normalized}%` }} /></div></div>
}
