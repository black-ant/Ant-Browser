import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { HelpCircle, Save } from 'lucide-react'

import { Badge, Button, Card, FormItem, Input, Progress, Select, Switch } from '../../../shared/components'

import type { AutomationNodeSource, AutomationRuntimeCheck, AutomationState, AutomationSystemNodeProbe } from '../api'
import type { AutomationRuntimeProgress } from '../progress'

type AutomationBusyState = 'none' | 'toggle' | 'probe' | 'runtime' | 'package' | 'install' | 'check'
type AutomationStatusVariant = 'default' | 'success' | 'error' | 'warning' | 'info'

const AUTOMATION_NODE_SOURCE_OPTIONS: Array<{ value: AutomationNodeSource; label: string }> = [
  { value: 'auto', label: 'auto · 优先系统 Node，失败回退内建' },
  { value: 'system', label: 'system · 强制系统 Node，不可用则报错' },
  { value: 'bundled', label: 'bundled · 总是使用内建 Node' },
]

interface AutomationSettingsCardProps {
  automationState: AutomationState
  automationProgress: AutomationRuntimeProgress | null
  automationBusy: AutomationBusyState
  automationCheck: AutomationRuntimeCheck | null
  automationProbe: AutomationSystemNodeProbe | null
  automationNodeSourceDraft: AutomationNodeSource
  automationSystemNodePathDraft: string
  automationRuntimeDirty: boolean
  launchServerPortDraft: string
  launchServerBaseUrl: string
  launchServerReady: boolean
  launchServerSaving: boolean
  onEnabledChange: (enabled: boolean) => void
  onHeadlessChange: (headlessDefault: boolean) => void
  onNodeSourceDraftChange: (value: AutomationNodeSource) => void
  onSystemNodePathDraftChange: (value: string) => void
  onLaunchServerPortDraftChange: (value: string) => void
  onSaveLaunchServerPort: () => void
  onTypeScriptBuildChange: (allowTypeScriptBuild: boolean) => void
  onProbeSystemNode: () => void
  onSaveRuntimeSettings: () => void
  onInstall: () => void
  onSelfCheck: () => void
}

type HelpTipPosition = {
  left: number
  top: number
}

function HelpTip({ label, text }: { label: string, text: string }) {
  const buttonRef = useRef<HTMLButtonElement | null>(null)
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState<HelpTipPosition | null>(null)

  const updatePosition = useCallback(() => {
    const button = buttonRef.current
    if (!button) return

    const rect = button.getBoundingClientRect()
    const tooltipWidth = 288
    const tooltipGap = 8
    const viewportPadding = 12
    const nextLeft = Math.min(
      Math.max(rect.left + rect.width / 2 - tooltipWidth / 2, viewportPadding),
      window.innerWidth - tooltipWidth - viewportPadding,
    )
    const bottomTop = rect.bottom + tooltipGap
    const topTop = rect.top - tooltipGap
    const nextTop = bottomTop + 96 > window.innerHeight
      ? Math.max(viewportPadding, topTop - 96)
      : bottomTop

    setPosition({ left: nextLeft, top: nextTop })
  }, [])

  const showTip = () => {
    updatePosition()
    setOpen(true)
  }

  const hideTip = () => {
    setOpen(false)
  }

  useEffect(() => {
    if (!open) return

    updatePosition()
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)

    return () => {
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
    }
  }, [open, updatePosition])

  return (
    <span className="inline-flex">
      <button
        ref={buttonRef}
        type="button"
        aria-label={label}
        aria-expanded={open}
        onMouseEnter={showTip}
        onMouseLeave={hideTip}
        onFocus={showTip}
        onBlur={hideTip}
        className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-muted)] transition-colors hover:border-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-border-default)]"
      >
        <HelpCircle className="h-3.5 w-3.5" />
      </button>
      {open && position && createPortal(
        <span
          role="tooltip"
          className="pointer-events-none fixed z-[9999] w-72 whitespace-normal rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2 text-xs font-normal leading-5 text-[var(--color-text-secondary)] shadow-[var(--shadow-md)]"
          style={{ left: position.left, top: position.top }}
        >
          {text}
        </span>,
        document.body,
      )}
    </span>
  )
}

function joinDetails(items: Array<[string, string | number | boolean | undefined]>) {
  return items
    .filter(([, value]) => value !== undefined && value !== '')
    .map(([label, value]) => `${label}: ${value}`)
    .join('\n')
}

function resolveAutomationStatus(state: AutomationState): {
  enabled: boolean
  ready: boolean
  installing: boolean
  statusLabel: string
  statusVariant: AutomationStatusVariant
  nodeSource: string
  nodeSourceLabel: string
  systemNodePath: string
  systemNodeLabel: string
} {
  const enabled = state.settings.enabled
  const ready = state.status.ready
  const installing = state.status.installing
  const statusLabel = installing
    ? '准备中'
    : ready
      ? '已就绪'
      : state.status.installed
        ? '已安装'
        : state.status.lastError
          ? '异常'
          : '未安装'
  const statusVariant = installing
    ? 'warning'
    : ready
      ? 'success'
      : state.status.lastError
        ? 'error'
        : 'default'
  const nodeSource = state.status.nodeSource || state.settings.nodeSource || 'auto'
  const nodeSourceLabel = nodeSource === 'system'
    ? 'system（系统 Node）'
    : nodeSource === 'bundled'
      ? 'bundled（内建 Node）'
      : 'auto（自动选择）'
  const systemNodePath = state.status.systemNodePath || state.settings.systemNodePath
  const systemNodeLabel = state.status.systemNodeDetected
    ? '已检测到'
    : systemNodePath
      ? '已配置，待验证'
      : '未检测到'

  return {
    enabled,
    ready,
    installing,
    statusLabel,
    statusVariant,
    nodeSource,
    nodeSourceLabel,
    systemNodePath,
    systemNodeLabel,
  }
}

export function AutomationSettingsCard({
  automationState,
  automationProgress,
  automationBusy,
  automationCheck,
  automationProbe,
  automationNodeSourceDraft,
  automationSystemNodePathDraft,
  automationRuntimeDirty,
  launchServerPortDraft,
  launchServerBaseUrl,
  launchServerReady,
  launchServerSaving,
  onEnabledChange,
  onHeadlessChange,
  onNodeSourceDraftChange,
  onSystemNodePathDraftChange,
  onLaunchServerPortDraftChange,
  onSaveLaunchServerPort,
  onTypeScriptBuildChange,
  onProbeSystemNode,
  onSaveRuntimeSettings,
  onInstall,
  onSelfCheck,
}: AutomationSettingsCardProps) {
  const {
    enabled,
    ready,
    installing,
    statusLabel,
    statusVariant,
    nodeSource,
    nodeSourceLabel,
    systemNodePath,
    systemNodeLabel,
  } = resolveAutomationStatus(automationState)
  const runtimeDetails = joinDetails([
    ['安装策略', automationState.settings.installPolicy],
    ['Runtime', automationState.settings.runtimeVersion],
    ['Node 来源', nodeSourceLabel],
    ['Node', automationState.status.nodeVersion || automationState.settings.nodeVersion],
    ['playwright-core', automationState.status.playwrightVersion || automationState.settings.playwrightVersion],
    ['TS 导入构建', automationState.settings.allowTypeScriptBuild ? 'enabled' : 'disabled'],
    ['系统 Node', systemNodeLabel],
    ['解析说明', automationState.status.nodeResolution],
    ['运行时目录', automationState.status.runtimeDir],
    ['Node 路径', automationState.status.nodePath],
    ['系统 Node 路径', systemNodePath],
    ['系统 Node 异常', automationState.status.systemNodeError],
    ['最近错误', automationState.status.lastError],
  ])
  const probeDetails = automationProbe
    ? joinDetails([
      ['版本', automationProbe.version],
      ['路径', automationProbe.path],
    ])
    : ''

  return (
    <Card
      title={(
        <div className="flex items-center gap-2">
          <span>自动化支持</span>
          <HelpTip
            label="自动化支持说明"
            text="首次启用时优先检测系统 Node，仅在需要时回退下载内建 Node，并准备私有 playwright-core。"
          />
        </div>
      )}
    >
      <div className="space-y-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <div className="flex items-center gap-2">
                <p className="text-sm font-medium text-[var(--color-text-primary)]">启用自动化支持</p>
                <HelpTip
                  label="启用自动化支持说明"
                  text="开启后应用会自动准备本地 automation runtime；关闭时不会卸载，后续再次启用可直接复用。"
                />
              </div>
              <Badge variant={statusVariant} size="sm" dot>{statusLabel}</Badge>
            </div>
          </div>
          <Switch
            checked={enabled}
            onChange={onEnabledChange}
            disabled={automationBusy === 'toggle'}
          />
        </div>

        <div className="h-px bg-[var(--color-border-muted)]" />

        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium text-[var(--color-text-primary)]">默认无头模式</p>
              <HelpTip
                label="默认无头模式说明"
                text="作为后续自动化任务的默认启动策略，首版先只保存配置，不直接改实例启动参数。"
              />
            </div>
          </div>
          <Switch
            checked={automationState.settings.headlessDefault}
            onChange={onHeadlessChange}
            disabled={automationBusy === 'toggle'}
          />
        </div>

        <div className="h-px bg-[var(--color-border-muted)]" />

        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,220px)_minmax(0,1fr)] gap-4 items-end">
          <FormItem
            label={(
              <div className="flex items-center gap-2">
                <span>本地 API 端口</span>
                <HelpTip
                  label="本地 API 端口说明"
                  text="自动化脚本和外部工具访问本地 Launch API / CDP 统一入口使用这个端口。"
                />
              </div>
            )}
          >
            <Input
              type="number"
              value={launchServerPortDraft}
              onChange={event => onLaunchServerPortDraftChange(event.target.value)}
              min={1}
              max={65535}
              step={1}
              disabled={launchServerSaving}
            />
          </FormItem>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="primary"
              className="min-w-[88px]"
              onClick={onSaveLaunchServerPort}
              loading={launchServerSaving}
            >
              <Save className="h-3.5 w-3.5" />
              保存
            </Button>
            <span className="text-xs text-[var(--color-text-muted)]">
              {launchServerReady ? launchServerBaseUrl : '服务未就绪'}
            </span>
          </div>
        </div>

        <div className="h-px bg-[var(--color-border-muted)]" />

        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,220px)_minmax(0,1fr)] gap-4">
          <FormItem
            label={(
              <div className="flex items-center gap-2">
                <span>Node 来源策略</span>
                <HelpTip
                  label="Node 来源策略说明"
                  text="auto 适合大多数环境；system 用于强制复用本机 Node；bundled 会忽略系统 Node，始终使用应用内建 runtime。"
                />
              </div>
            )}
          >
            <Select
              value={automationNodeSourceDraft}
              onChange={event => onNodeSourceDraftChange(event.target.value as AutomationNodeSource)}
              disabled={automationBusy !== 'none'}
              options={AUTOMATION_NODE_SOURCE_OPTIONS}
            />
          </FormItem>
          <FormItem
            label={(
              <div className="flex items-center gap-2">
                <span>系统 Node 路径</span>
                <HelpTip
                  label="系统 Node 路径说明"
                  text="留空则走 PATH。"
                />
              </div>
            )}
          >
            <Input
              value={automationSystemNodePathDraft}
              onChange={event => onSystemNodePathDraftChange(event.target.value)}
              placeholder="例如 C:\\Program Files\\nodejs\\node.exe"
              disabled={automationBusy !== 'none' || automationNodeSourceDraft === 'bundled'}
            />
          </FormItem>
        </div>

        <div className="h-px bg-[var(--color-border-muted)]" />

        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium text-[var(--color-text-primary)]">允许导入 TypeScript 脚本（实验）</p>
              <HelpTip
                label="TypeScript 导入说明"
                text="仅支持单入口、本地相对依赖，并会在导入时构建为 CommonJS；不支持外部 npm 依赖。"
              />
            </div>
          </div>
          <Switch
            checked={automationState.settings.allowTypeScriptBuild}
            onChange={onTypeScriptBuildChange}
            disabled={automationBusy !== 'none'}
          />
        </div>

        <div className="flex items-center justify-between gap-4 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <HelpTip
            label="运行策略说明"
            text="auto 适合大多数环境；system 用于强制复用本机 Node；bundled 会忽略系统 Node，始终使用应用内建 runtime。"
          />
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={onProbeSystemNode}
              loading={automationBusy === 'probe'}
              disabled={automationBusy !== 'none' || automationNodeSourceDraft === 'bundled'}
            >
              检测系统 Node
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={onSaveRuntimeSettings}
              loading={automationBusy === 'runtime' && automationRuntimeDirty}
              disabled={!automationRuntimeDirty || automationBusy !== 'none'}
            >
              保存运行时策略
            </Button>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
          <span>Runtime：<code>{automationState.settings.runtimeVersion}</code></span>
          <span>Node：<code>{automationState.status.nodeVersion || automationState.settings.nodeVersion}</code></span>
          <HelpTip label="运行时详情" text={runtimeDetails} />
          {probeDetails && <HelpTip label="系统 Node 检测结果" text={probeDetails} />}
        </div>

        {automationProgress && (
          <div className="rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-3 space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="text-[var(--color-text-secondary)]">{automationProgress.message}</span>
              <span className="text-[var(--color-text-muted)]">
                {automationProgress.component ? `${automationProgress.component} · ` : ''}
                {automationProgress.phase}
              </span>
            </div>
            <Progress
              percent={automationProgress.progress}
              size="sm"
              status={automationProgress.phase === 'error' ? 'error' : automationProgress.phase === 'done' ? 'success' : 'normal'}
            />
          </div>
        )}

        {automationCheck && (
          <div className="rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
            最近自检：<code>{automationCheck.nodeSource || nodeSource}</code> / Node <code>{automationCheck.nodeVersion}</code> / playwright-core <code>{automationCheck.playwrightVersion}</code>
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={onInstall}
            loading={automationBusy === 'install'}
            disabled={installing}
          >
            {automationState.status.installed ? '修复/重装运行时' : '立即准备运行时'}
          </Button>
          <Button
            size="sm"
            onClick={onSelfCheck}
            loading={automationBusy === 'check'}
            disabled={!ready}
          >
            运行自检
          </Button>
        </div>
      </div>
    </Card>
  )
}
