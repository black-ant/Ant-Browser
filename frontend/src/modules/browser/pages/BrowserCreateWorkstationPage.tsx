import { useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import clsx from 'clsx'
import {
  Apple,
  ArrowLeft,
  ChevronDown,
  Chrome,
  Database,
  FileText,
  Flame,
  HelpCircle,
  LayoutTemplate,
  Link2,
  Monitor,
  Package,
  Paperclip,
  Plus,
  RefreshCw,
  Settings,
  Shield,
  Smartphone,
  Tag,
  Terminal,
  Wand2,
  X,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Button, Input, Modal } from '../../../shared/components'
import type { BrowserAccount, BrowserExtension } from '../api'
import { GroupSelector } from '../components/GroupSelector'
import { platformIcon } from '../config/platformPresets'
import { TagInput } from '../components/TagInput'
import type { BrowserCore, BrowserGroup, BrowserProxy, CreateWindowFormState } from '../types'
import { randomFingerprintSeed } from '../utils/fingerprintSerializer'

type CreateMode = 'single' | 'batch' | 'import'
type SectionKey = 'urls' | 'basic' | 'advanced' | 'preferences'
type SummaryItem = { label: string; value: ReactNode; muted?: boolean }

interface BrowserCreateWorkstationPageProps {
  formData: CreateWindowFormState
  cores: BrowserCore[]
  proxies: BrowserProxy[]
  groups: BrowserGroup[]
  allTags: string[]
  extensionsLoadError: string
  loadableExtensions: BrowserExtension[]
  selectedExtensionIds: string[]
  selectedLoadableExtensionCount: number
  launchArgsText: string
  saving: boolean
  onBack: () => void
  onSave: () => void
  onChange: (field: keyof CreateWindowFormState, value: string | string[] | boolean) => void
  onExtensionToggle: (extensionId: string) => void
  onLaunchArgsTextChange: (value: string) => void
  onOpenProxyPicker: () => void
  onOpenProxyImport: () => void
  accounts: BrowserAccount[]
  onOpenAccountPicker: () => void
  onOpenAccountAdd: () => void
  templates: BrowserTemplateOption[]
  selectedTemplateId: string
  onApplyTemplate: (templateId: string) => void
  onSaveAsTemplate: (name: string) => void
}

export interface BrowserTemplateOption {
  templateId: string
  templateName: string
}

const controlClass = [
  'h-9 w-full rounded-md border border-[#c7d1dd] bg-white px-3 text-sm text-[#111827]',
  'placeholder:text-[#9aa5b1] outline-none transition-colors',
  'focus:border-[#168fff] focus:ring-2 focus:ring-[#168fff]/10',
].join(' ')

const textAreaClass = [
  'w-full resize-none rounded-md border border-[#c7d1dd] bg-white px-3 py-2 text-sm text-[#111827]',
  'placeholder:text-[#9aa5b1] outline-none transition-colors',
  'focus:border-[#168fff] focus:ring-2 focus:ring-[#168fff]/10',
].join(' ')

const tabs: { key: CreateMode; label: string }[] = [
  { key: 'single', label: '单个创建' },
  { key: 'batch', label: '批量创建' },
  { key: 'import', label: '窗口导入' },
]

const systems: { value: string; label: string; Icon: LucideIcon; className?: string }[] = [
  { value: 'windows', label: 'Windows', Icon: Monitor, className: 'text-[#168fff]' },
  { value: 'macos', label: 'macOS', Icon: Apple, className: 'text-[#111827]' },
  { value: 'linux', label: 'Linux', Icon: Terminal, className: 'text-[#eab308]' },
  { value: 'android', label: 'Android', Icon: Smartphone, className: 'text-[#52c41a]' },
  { value: 'ios', label: 'iOS', Icon: Apple, className: 'text-[#a8b0b9]' },
]

const browserTypes = [
  { value: 'chrome', label: 'Chrome', Icon: Chrome },
  { value: 'firefox', label: 'Firefox', Icon: Flame },
]

const userAgents = [
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36',
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36',
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36',
]

// 一键配置预设：每项是一组创建页表单补丁，套用时合并到当前表单（保留窗口身份字段）。
// 仅设置内核可生效的字段（语言/时区/UA/搜索引擎/WebRTC/字体/媒体开关等）。
const quickPresets: { label: string; patch: Partial<CreateWindowFormState> }[] = [
  {
    label: '通用',
    patch: {
      language: 'auto', uiLanguage: 'auto', timezone: 'auto',
      searchEngine: 'google', webrtc: 'disabled', fontFingerprint: 'random',
      audio: true, image: true, video: true,
    },
  },
  {
    label: '海外社媒',
    patch: {
      language: 'en-US', uiLanguage: 'en-US', timezone: 'America/New_York',
      searchEngine: 'google', webrtc: 'disabled', fontFingerprint: 'random',
      userAgent: userAgents[0],
    },
  },
  {
    label: '跨境电商',
    patch: {
      language: 'en-US', uiLanguage: 'en-US', timezone: 'America/Los_Angeles',
      searchEngine: 'google', webrtc: 'disabled', fontFingerprint: 'random',
      image: true, video: true,
    },
  },
  {
    label: 'AI 专用',
    patch: {
      language: 'en-US', uiLanguage: 'en-US', timezone: 'America/Los_Angeles',
      searchEngine: 'google', webrtc: 'disabled', fontFingerprint: 'random',
      googleLogin: 'off',
    },
  },
  {
    label: 'TikTok',
    patch: {
      language: 'en-US', uiLanguage: 'en-US', timezone: 'America/Los_Angeles',
      searchEngine: 'google', webrtc: 'disabled', fontFingerprint: 'random',
      audio: true, video: true,
    },
  },
  {
    label: 'Reddit',
    patch: {
      language: 'en-US', uiLanguage: 'en-US', timezone: 'America/New_York',
      searchEngine: 'duckduckgo', webrtc: 'disabled', fontFingerprint: 'random',
    },
  },
]
const webglProfiles = [
  {
    vendor: 'Google Inc. (AMD)',
    renderer: 'ANGLE (AMD, AMD Radeon (TM) R9 200 Series Direct3D11 vs_5_0 ps_5_0, D3D11-27.20.14501.28009)',
  },
  {
    vendor: 'Google Inc. (Intel)',
    renderer: 'ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)',
  },
  {
    vendor: 'Google Inc. (NVIDIA)',
    renderer: 'ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 Direct3D11 vs_5_0 ps_5_0, D3D11)',
  },
]
const deviceNameSamples = ['DESKTOP-TN92C12E', 'DESKTOP-K4M8Q1P', 'LAPTOP-7A3NDQ5M', 'WORKSTATION-0928']
const macAddressSamples = ['1A-76-07-3B-E3-56', '4C-2F-8A-19-D5-0B', 'A8-3E-7C-42-91-F0', '60-45-BD-77-2C-18']
const unsupportedTitle = '暂未支持启动生效'
const kernelUnsupportedTitle = '当前内核暂不支持该项指纹伪装'
const languageOptions = [
  { value: 'auto', label: '基于 IP 匹配' },
  { value: 'zh-CN', label: '中文 (zh-CN)' },
  { value: 'en-US', label: 'English (en-US)' },
  { value: 'en-GB', label: 'English (en-GB)' },
  { value: 'ja-JP', label: '日本語 (ja-JP)' },
  { value: 'ko-KR', label: '한국어 (ko-KR)' },
  { value: 'fr-FR', label: 'Français (fr-FR)' },
  { value: 'de-DE', label: 'Deutsch (de-DE)' },
]
const timezoneOptions = [
  { value: 'auto', label: '基于 IP 匹配' },
  { value: 'Asia/Shanghai', label: 'Asia/Shanghai' },
  { value: 'Asia/Tokyo', label: 'Asia/Tokyo' },
  { value: 'Asia/Singapore', label: 'Asia/Singapore' },
  { value: 'America/New_York', label: 'America/New_York' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles' },
  { value: 'Europe/London', label: 'Europe/London' },
  { value: 'Europe/Paris', label: 'Europe/Paris' },
  { value: 'Australia/Sydney', label: 'Australia/Sydney' },
]

function pickRandom<T>(items: T[]): T {
  return items[Math.floor(Math.random() * items.length)]
}

function replaceSwitchArg(args: string[] | undefined, prefix: string, value: string): string[] {
  const nextArg = `${prefix}${value}`
  let replaced = false
  const out: string[] = []
  for (const raw of args || []) {
    const arg = raw.trim()
    if (!arg) continue
    if (arg.startsWith(prefix)) {
      if (!replaced) {
        out.push(nextArg)
        replaced = true
      }
      continue
    }
    out.push(arg)
  }
  if (!replaced) out.push(nextArg)
  return out
}

function displayByMode(value: string | undefined, fallback = '随机') {
  if (!value || value === 'random') return fallback
  if (value === 'auto') return '基于 IP 匹配'
  if (value === 'real') return '真实'
  if (value === 'disabled') return '禁用'
  if (value === 'disable_non_proxied_udp') return '禁止'
  if (value === 'system') return '跟随系统'
  if (value === 'allow') return '允许'
  if (value === 'deny') return '禁止'
  if (value === 'custom') return '自定义'
  return value
}

function SummaryRow({ item }: { item: SummaryItem }) {
  return (
    <div className={clsx('grid grid-cols-[104px_minmax(0,1fr)] gap-3 text-xs leading-5', item.muted && 'opacity-45')}>
      <dt className="font-semibold text-[#0064d8]">{item.label}</dt>
      <dd className="min-w-0 break-words font-semibold text-[#03234a]">{item.value}</dd>
    </div>
  )
}

function DevicePreview({
  system,
  systemLabel,
}: {
  system: string
  systemLabel: string
}) {
  const SystemIcon = systems.find(item => item.value === system)?.Icon || Monitor

  return (
    <div className="relative h-[132px] overflow-hidden rounded-md bg-gradient-to-b from-[#eaf7ff] to-[#f5f3ff]">
      <div className="absolute left-1/2 top-4 flex -translate-x-1/2 items-center">
        <span className="flex h-6 w-6 items-center justify-center rounded border border-[#b9cfed] bg-white text-[#168fff] shadow-sm">
          <SystemIcon className="h-3.5 w-3.5" />
        </span>
        <span className="h-px w-[82px] bg-[#bfd1e5]" />
        <span className="flex h-6 w-6 items-center justify-center rounded border border-[#b9cfed] bg-white text-[#64748b] shadow-sm">
          <Plus className="h-3.5 w-3.5" />
        </span>
      </div>
      <div className="absolute bottom-0 left-1/2 h-[78px] w-[164px] -translate-x-1/2 rounded-t-sm border-[4px] border-[#1f2937] bg-[#91c4ed] shadow-md">
        <div className="absolute inset-0 overflow-hidden bg-[#d8f2ff]">
          <div className="absolute -left-6 top-2 h-24 w-44 rounded-full bg-[#42a5ff] blur-[10px]" />
          <div className="absolute left-6 top-4 h-28 w-48 rotate-[-18deg] rounded-full bg-[#1677ff] blur-[8px]" />
          <div className="absolute left-16 top-9 h-24 w-44 rotate-[22deg] rounded-full bg-[#65c3ff] blur-[8px]" />
          <div className="absolute left-10 top-12 h-16 w-32 rounded-full border-[10px] border-[#e5fbff]/70" />
        </div>
      </div>
      <span className="sr-only">{systemLabel}</span>
    </div>
  )
}

function QuickConfigPanel({ onRandomize, onApplyPreset }: { onRandomize: () => void; onApplyPreset: (label: string) => void }) {
  return (
    <section className="relative rounded-xl bg-[#f0efff] p-4">
      <div className="absolute right-3 top-2 rounded-full bg-[#12d789] px-3 py-1 text-xs font-bold text-white">
        提高 5 倍稳定性
      </div>
      <div className="mb-3 flex items-center gap-2 pr-28 text-sm font-bold text-[#111827]">
        <Wand2 className="h-4 w-4 text-[#f59e0b]" />
        一键配置
      </div>
      <div className="flex flex-wrap gap-2">
        {quickPresets.map(preset => (
          <button
            key={preset.label}
            type="button"
            onClick={() => onApplyPreset(preset.label)}
            title={`套用「${preset.label}」预设`}
            className="h-8 rounded-md border border-[#b8c5d5] bg-white px-3 text-sm font-semibold text-[#111827] shadow-sm transition-colors hover:border-[#168fff] hover:text-[#168fff]"
          >
            {preset.label}
          </button>
        ))}
      </div>
      <button
        type="button"
        onClick={onRandomize}
        className="mt-3 inline-flex h-8 items-center gap-2 rounded-md border border-[#b8c5d5] bg-white px-3 text-sm font-semibold text-[#111827] shadow-sm transition-colors hover:border-[#168fff] hover:text-[#168fff]"
      >
        <RefreshCw className="h-4 w-4" />
        随机
      </button>
    </section>
  )
}

function SummaryPanel({
  system,
  systemLabel,
  summaryItems,
  onCollapse,
  onRandomize,
  onApplyPreset,
}: {
  system: string
  systemLabel: string
  summaryItems: SummaryItem[]
  onCollapse: () => void
  onRandomize: () => void
  onApplyPreset: (label: string) => void
}) {
  return (
    <aside className="w-full min-w-0 shrink-0 xl:w-[376px]">
      <div className="flex h-[calc(100vh-13rem)] min-h-[480px] flex-col pr-1 xl:sticky xl:top-0">
        <QuickConfigPanel onRandomize={onRandomize} onApplyPreset={onApplyPreset} />

        <section className="mt-3 flex min-h-0 flex-1 flex-col rounded-xl border border-[#cbd5e1] bg-white p-4 shadow-md">
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-bold text-[#111827]">概要</h3>
          <button
            type="button"
            onClick={onCollapse}
            className="flex h-6 w-6 items-center justify-center rounded border border-[#cbd5e1] text-[#111827] transition-colors hover:border-[#168fff] hover:text-[#168fff]"
            title="收起"
            aria-label="收起概要"
          >
            <FileText className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          <DevicePreview system={system} systemLabel={systemLabel} />
          <dl className="mt-4 space-y-2.5">
            {summaryItems.map(item => (
              <SummaryRow key={item.label} item={item} />
            ))}
          </dl>
        </div>
        <button
          type="button"
          onClick={onRandomize}
          className="mt-4 flex h-10 shrink-0 items-center justify-center gap-2 rounded-md border border-[#168fff] bg-white text-sm font-semibold text-[#168fff] transition-colors hover:bg-[#edf7ff]"
        >
          <RefreshCw className="h-4 w-4" />
          一键随机指纹
        </button>
        </section>
      </div>
    </aside>
  )
}

function FieldRow({
  label,
  children,
  alignStart = false,
}: {
  label: string
  children: ReactNode
  alignStart?: boolean
}) {
  return (
    <div className="grid grid-cols-[112px_minmax(0,1fr)] items-start gap-3">
      <label className={clsx('text-right text-sm text-[#111827]', alignStart ? 'pt-2' : 'pt-1.5')}>
        {label}
      </label>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

function SelectedAccounts({
  accountIds,
  accounts,
  onRemove,
}: {
  accountIds: string[]
  accounts: BrowserAccount[]
  onRemove: (accountId: string) => void
}) {
  const selected = accountIds
    .map(id => accounts.find(acc => acc.accountId === id))
    .filter((acc): acc is BrowserAccount => !!acc)

  if (selected.length === 0) {
    return (
      <p className="text-xs text-[#94a3b8]">未关联平台账号，点击右上角「选择」或「添加」。</p>
    )
  }

  return (
    <div className="flex flex-wrap gap-2">
      {selected.map(acc => (
        <span
          key={acc.accountId}
          className="inline-flex items-center gap-1.5 rounded-full border border-[#cbd5e1] bg-[#f8fafc] py-1 pl-2.5 pr-1.5 text-xs text-[#334155]"
        >
          <span className="text-sm leading-none">{platformIcon(acc.platform)}</span>
          <span className="max-w-[180px] truncate">{acc.accountName || acc.username || acc.email || acc.accountId}</span>
          <button
            type="button"
            onClick={() => onRemove(acc.accountId)}
            className="flex h-4 w-4 items-center justify-center rounded-full text-[#94a3b8] transition-colors hover:bg-[#e2e8f0] hover:text-[#475569]"
            aria-label="移除"
          >
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
    </div>
  )
}

function SectionDivider({
  title,
  children,
  actions,
}: {
  title: string
  children?: ReactNode
  actions?: ReactNode
}) {
  return (
    <section className="border-t border-dashed border-[#cbd5e1] py-5">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <h3 className="text-base font-semibold text-[#111827]">{title}</h3>
        </div>
        {actions && <div className="flex shrink-0 items-center gap-5 text-sm text-[#111827]">{actions}</div>}
      </div>
      {children && <div className="mt-3">{children}</div>}
    </section>
  )
}

function InlineAction({
  icon: Icon,
  children,
  onClick,
  disabled = false,
  title,
}: {
  icon: LucideIcon
  children: ReactNode
  onClick?: () => void
  disabled?: boolean
  title?: string
}) {
  const isDisabled = disabled || !onClick
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={isDisabled}
      title={title || (isDisabled ? '暂未实现' : undefined)}
      className="inline-flex h-8 items-center gap-1.5 text-sm font-medium text-[#111827] transition-colors hover:text-[#168fff] disabled:cursor-not-allowed disabled:text-[#94a3b8] disabled:hover:text-[#94a3b8]"
    >
      <Icon className="h-4 w-4" />
      {children}
    </button>
  )
}

function CollapsibleRow({
  id,
  title,
  icon: Icon,
  open,
  onToggle,
  children,
}: {
  id: SectionKey
  title: string
  icon: LucideIcon
  open: boolean
  onToggle: (id: SectionKey) => void
  children: ReactNode
}) {
  return (
    <section className="border-t border-dashed border-[#cbd5e1]">
      <button
        type="button"
        onClick={() => onToggle(id)}
        className="flex w-full items-center gap-3 py-5 text-left text-[#111827] transition-colors hover:text-[#168fff]"
      >
        <span className="flex h-5 w-5 items-center justify-center rounded border border-[#aeb8c4] bg-white">
          <Plus className={clsx('h-3.5 w-3.5 transition-transform', open && 'rotate-45')} />
        </span>
        <Icon className="h-4 w-4 text-[#64748b]" />
        <span className="text-base font-semibold">{title}</span>
      </button>
      {open && <div className="pb-5 pl-8">{children}</div>}
    </section>
  )
}

function FooterButton({ icon: Icon, children }: { icon: LucideIcon; children: ReactNode }) {
  return (
    <button
      type="button"
      disabled
      title="暂未实现"
      className="inline-flex h-9 items-center gap-2 rounded-md border border-[#cbd5e1] bg-white px-3 text-sm font-medium text-[#111827] shadow-sm transition-colors disabled:cursor-not-allowed disabled:border-[#e2e8f0] disabled:text-[#94a3b8] disabled:shadow-none"
    >
      <Icon className="h-4 w-4 text-[#64748b]" />
      {children}
    </button>
  )
}

function SettingRow({
  label,
  children,
  info = false,
  alignStart = false,
}: {
  label: string
  children: ReactNode
  info?: boolean
  alignStart?: boolean
}) {
  return (
    <div className={clsx('grid grid-cols-[116px_minmax(0,1fr)] gap-2', alignStart ? 'items-start' : 'items-center')}>
      <div className={clsx('flex justify-end gap-1 text-sm text-[#111827]', alignStart ? 'pt-2' : 'items-center')}>
        <span>{label}</span>
        {info && <HelpCircle className="h-3.5 w-3.5 text-[#b8c2cc]" />}
      </div>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

function SegmentedControl({
  value,
  options,
  onChange,
  className,
  disabled = false,
  title,
}: {
  value: string
  options: { value: string; label: string }[]
  onChange: (value: string) => void
  className?: string
  disabled?: boolean
  title?: string
}) {
  return (
    <div
      className={clsx('inline-flex max-w-full overflow-visible rounded-md bg-[#edf2f7] p-1', disabled && 'opacity-60', className)}
      style={{ width: 'max-content', minWidth: 'max-content' }}
      title={title}
    >
      {options.map(option => {
        const active = value === option.value
        return (
          <button
            key={option.value}
            type="button"
            disabled={disabled}
            onClick={() => onChange(option.value)}
            className={clsx(
              'h-8 min-w-[58px] shrink-0 whitespace-nowrap rounded border px-3 text-xs font-medium transition-colors',
              active
                ? 'border border-[#b7c5d3] bg-white text-[#111827] shadow-sm'
                : 'border-transparent text-[#334155] hover:text-[#168fff]',
              disabled && 'cursor-not-allowed hover:text-[#334155]',
            )}
          >
            {option.label}
          </button>
        )
      })}
    </div>
  )
}

function RandomizedInput({
  value,
  onChange,
  onRandom,
  placeholder,
  multiline = false,
  disabled = false,
  title,
}: {
  value: string
  onChange: (value: string) => void
  onRandom: () => void
  placeholder?: string
  multiline?: boolean
  disabled?: boolean
  title?: string
}) {
  const inputClass = multiline
    ? clsx(textAreaClass, 'min-h-[74px] pr-10', disabled && 'cursor-not-allowed bg-[#f8fafc] text-[#94a3b8]')
    : clsx(controlClass, 'pr-10', disabled && 'cursor-not-allowed bg-[#f8fafc] text-[#94a3b8]')

  return (
    <div className="relative w-full max-w-[590px]" title={title}>
      {multiline ? (
        <textarea
          value={value}
          disabled={disabled}
          onChange={event => onChange(event.target.value)}
          rows={3}
          placeholder={placeholder}
          className={inputClass}
        />
      ) : (
        <input
          value={value}
          disabled={disabled}
          onChange={event => onChange(event.target.value)}
          placeholder={placeholder}
          className={inputClass}
        />
      )}
      <button
        type="button"
        disabled={disabled}
        onClick={onRandom}
        className="absolute right-2 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded text-[#64748b] transition-colors hover:bg-[#edf2f7] hover:text-[#168fff] disabled:cursor-not-allowed disabled:text-[#94a3b8] disabled:hover:bg-transparent"
        title={disabled ? title || unsupportedTitle : '随机生成'}
        aria-label="随机生成"
      >
        <RefreshCw className="h-3.5 w-3.5" />
      </button>
    </div>
  )
}

function SelectControl({
  value,
  options,
  onChange,
  className,
  disabled = false,
  title,
}: {
  value: string
  options: { value: string; label: string }[]
  onChange: (value: string) => void
  className?: string
  disabled?: boolean
  title?: string
}) {
  return (
    <div className={clsx('relative w-full max-w-[590px]', className)} title={title}>
      <select
        value={value}
        disabled={disabled}
        onChange={event => onChange(event.target.value)}
        className={clsx(controlClass, 'appearance-none pr-9', disabled && 'cursor-not-allowed bg-[#f8fafc] text-[#94a3b8]')}
      >
        {options.map(option => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#64748b]" />
    </div>
  )
}

function DimensionInput({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className="inline-flex h-9 overflow-hidden rounded-md border border-[#c7d1dd] bg-white">
      <span className="flex w-10 items-center justify-center border-r border-[#dbe3ec] text-sm text-[#111827]">{label}</span>
      <input
        value={value}
        onChange={event => onChange(event.target.value)}
        className="h-full w-24 border-0 px-3 text-sm text-[#111827] outline-none"
      />
    </label>
  )
}

function WindowPositionPicker({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}) {
  const current = value || 'top-left'

  return (
    <div className="grid h-[138px] w-[230px] grid-cols-3 grid-rows-3 border border-dashed border-[#c7d1dd] bg-[#f1f6fb]">
      {['top-left', 'top', 'top-right', 'left', 'center', 'right', 'bottom-left', 'bottom', 'bottom-right'].map(position => {
        const active = current === position
        return (
          <button
            key={position}
            type="button"
            onClick={() => onChange(position)}
            className={clsx(
              'relative border border-dashed border-[#c7d1dd] transition-colors hover:bg-white/70',
              active && 'bg-white ring-2 ring-inset ring-[#168fff]',
            )}
            aria-label={position}
          >
            {active && (
              <span className="absolute left-1 top-1 h-[34px] w-[66px] rounded-sm border-2 border-[#168fff] bg-white">
                <span className="absolute right-1 top-1 flex gap-0.5">
                  <span className="h-1 w-1 rounded-full bg-[#168fff]" />
                  <span className="h-1 w-1 rounded-full bg-[#168fff]" />
                  <span className="h-1 w-1 rounded-full bg-[#168fff]" />
                </span>
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}

export function BrowserCreateWorkstationPage({
  formData,
  cores,
  proxies,
  groups,
  allTags,
  extensionsLoadError,
  loadableExtensions,
  selectedExtensionIds,
  selectedLoadableExtensionCount,
  launchArgsText,
  saving,
  onBack,
  onSave,
  onChange,
  onExtensionToggle,
  onLaunchArgsTextChange,
  onOpenProxyPicker,
  onOpenProxyImport,
  accounts,
  onOpenAccountPicker,
  onOpenAccountAdd,
  templates,
  selectedTemplateId,
  onApplyTemplate,
  onSaveAsTemplate,
}: BrowserCreateWorkstationPageProps) {
  const [mode, setMode] = useState<CreateMode>('single')
  const [summaryPanelOpen, setSummaryPanelOpen] = useState(false)
  const [saveTemplateOpen, setSaveTemplateOpen] = useState(false)
  const [notesOpen, setNotesOpen] = useState(false)
  const [templateNameDraft, setTemplateNameDraft] = useState('')
  const cookieFileInputRef = useRef<HTMLInputElement>(null)

  const handleCookieFileSelected = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = '' // 允许重复选择同一文件
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      const text = typeof reader.result === 'string' ? reader.result.trim() : ''
      if (text) onChange('cookies', text)
    }
    reader.readAsText(file)
  }
  const [openSections, setOpenSections] = useState<Record<SectionKey, boolean>>({
    urls: false,
    basic: false,
    advanced: false,
    preferences: false,
  })

  const selectedProxy = useMemo(
    () => proxies.find(proxy => proxy.proxyId === formData.proxyId),
    [proxies, formData.proxyId],
  )
  const defaultCore = cores.find(core => core.isDefault)
  const system = formData.system || 'windows'
  const browserCore = formData.browserCore || 'chrome'
  const browserVersion = formData.browserVersion || defaultCore?.coreName || 'RoxyChrome 149'
  const systemVersion = formData.systemVersion || 'Windows 11'
  const userAgent = formData.userAgent || userAgents[0]
  const systemLabel = systems.find(item => item.value === system)?.label || systemVersion
  const browserCoreLabel = browserTypes.find(item => item.value === browserCore)?.label || browserCore
  const summaryItems = useMemo<SummaryItem[]>(() => [
    { label: '操作系统', value: systemLabel },
    { label: '内核类型', value: browserCoreLabel },
    { label: 'User Agent', value: userAgent },
    { label: '语言', value: displayByMode(formData.language, '基于 IP 匹配') },
    { label: '时区', value: displayByMode(formData.timezone, '基于 IP 匹配') },
    { label: '地理位置提示', value: displayByMode(formData.geolocationDisplay || 'allow') },
    { label: '地理位置', value: displayByMode(formData.geolocation, '基于 IP 匹配'), muted: !formData.geolocation },
    { label: '声音', value: formData.audio === false ? '关闭' : '开启' },
    { label: '图片', value: formData.image === false ? '关闭' : '开启' },
    { label: '视频', value: formData.video === false ? '关闭' : '开启' },
    { label: '窗口大小', value: formData.resolution === 'fullscreen' ? '全屏' : `${formData.windowWidth || '1000'} x ${formData.windowHeight || '1000'}` },
    { label: '窗口位置', value: displayByMode(formData.windowPosition || 'top-left') },
    { label: '搜索引擎', value: displayByMode(formData.searchEngine || 'Google') },
    { label: '分辨率', value: displayByMode(formData.resolution || 'custom', '自定义') },
    { label: '字体指纹', value: formData.fontFingerprint === 'system' ? '跟随系统' : '随机' },
    { label: 'WebRTC', value: displayByMode(formData.webrtc || 'disable_non_proxied_udp') },
    { label: 'WebGL 图像', value: displayByMode(formData.webgl, '随机') },
    { label: 'WebGL Info', value: displayByMode(formData.webgl, '随机') },
    { label: 'Canvas', value: displayByMode(formData.canvas, '随机') },
    { label: 'AudioContext', value: displayByMode(formData.audioContext, '随机') },
    { label: 'Speech Voices', value: displayByMode(formData.speechVoices, '随机') },
    { label: 'Do Not Track', value: displayByMode(formData.doNotTrack || 'on', '开启') },
    { label: 'Client Rects', value: displayByMode(formData.clientRects, '随机') },
    { label: '媒体设备', value: displayByMode(formData.mediaDevices, '随机') },
    { label: '设备名称', value: displayByMode(formData.deviceName, '随机') },
    { label: 'MAC地址', value: displayByMode(formData.macAddress || 'custom') },
    { label: '硬件并发数', value: formData.hardwareConcurrency ? `${formData.hardwareConcurrency}核` : '12核' },
    { label: '设备内存', value: formData.deviceMemory || '8G' },
    { label: 'SSL指纹设置', value: displayByMode(formData.sslFingerprint || 'disabled') },
    { label: '端口扫描保护', value: displayByMode(formData.portScanProtection || 'on', '开启') },
    { label: '硬件加速模式', value: displayByMode(formData.hardwareAcceleration || 'on', '开启') },
  ], [browserCoreLabel, formData, systemLabel, userAgent])

  const toggleSection = (id: SectionKey) => {
    setOpenSections(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const handleRandomFingerprint = () => {
    const webgl = pickRandom(webglProfiles)
    onChange('userAgent', pickRandom(userAgents))
    onChange('hardwareConcurrency', pickRandom(['4', '6', '8', '12', '16']))
    onChange('deviceMemory', pickRandom(['4', '8', '16']))
    onChange('webglVendor', webgl.vendor)
    onChange('webglRenderer', webgl.renderer)
    onChange('deviceNameValue', pickRandom(deviceNameSamples))
    onChange('macAddressValue', pickRandom(macAddressSamples))
    onChange('fingerprintArgs', replaceSwitchArg(formData.fingerprintArgs, '--fingerprint=', randomFingerprintSeed()))
  }

  // 套用一键预设：把预设补丁逐项写入表单（仅覆盖预设涉及的字段）。
  const handleApplyPreset = (label: string) => {
    const preset = quickPresets.find(item => item.label === label)
    if (!preset) return
    Object.entries(preset.patch).forEach(([field, value]) => {
      onChange(field as keyof CreateWindowFormState, value as string | boolean)
    })
  }

  return (
    <div className="-m-5 flex h-[calc(100vh-3.5rem)] min-h-[720px] flex-col bg-white text-[#111827]">
      <header className="shrink-0 border-b border-[#e5eaf0] bg-white">
        <div className="flex h-11 items-center px-6">
          <button
            type="button"
            onClick={onBack}
            className="inline-flex items-center gap-3 text-base font-semibold text-[#111827] transition-colors hover:text-[#168fff]"
          >
            <ArrowLeft className="h-5 w-5" />
            创建窗口
          </button>
        </div>
        <nav className="flex h-9 items-end gap-8 px-6">
          {tabs.map(tab => (
            <button
              key={tab.key}
              type="button"
              disabled={tab.key !== 'single'}
              title={tab.key !== 'single' ? unsupportedTitle : undefined}
              onClick={() => setMode(tab.key)}
              className={clsx(
                'relative h-full px-1 text-sm font-medium transition-colors',
                mode === tab.key ? 'text-[#168fff]' : 'text-[#4b5563] hover:text-[#111827]',
                tab.key !== 'single' && 'cursor-not-allowed text-[#94a3b8] hover:text-[#94a3b8]',
              )}
            >
              {tab.label}
              {mode === tab.key && <span className="absolute inset-x-0 bottom-0 h-0.5 bg-[#168fff]" />}
            </button>
          ))}
        </nav>
      </header>

      <div className="relative min-h-0 flex-1 overflow-auto bg-white">
        <div
          className={clsx(
            'mx-auto grid w-full gap-6 px-5 pb-28 pt-7 xl:gap-8',
            summaryPanelOpen ? 'max-w-[1180px] xl:grid-cols-[minmax(0,760px)_376px]' : 'max-w-[860px]',
          )}
        >
        <main className="min-w-0">
          <div className="relative mb-5 flex items-center justify-center">
            <h2 className="text-base font-semibold text-[#111827]">窗口信息</h2>
            <div className="absolute right-0 flex items-center gap-5">
              <div className="relative">
                <select
                  value={selectedTemplateId}
                  onChange={event => onApplyTemplate(event.target.value)}
                  title={templates.length === 0 ? '暂无模板，可在下方“存为新模板”创建' : '从已保存模板套用配置'}
                  className="h-8 cursor-pointer appearance-none rounded-md border border-[#cbd5e1] bg-white pl-8 pr-7 text-sm font-semibold text-[#111827] outline-none transition-colors hover:border-[#168fff] focus:border-[#168fff]"
                >
                  <option value="">不使用模板</option>
                  {templates.map(template => (
                    <option key={template.templateId} value={template.templateId}>
                      {template.templateName}
                    </option>
                  ))}
                </select>
                <LayoutTemplate className="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-[#64748b]" />
                <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-[#64748b]" />
              </div>
              <button
                type="button"
                onClick={() => setSummaryPanelOpen(open => !open)}
                className="flex h-12 w-12 items-center justify-center rounded-xl border border-[#cbd5e1] bg-white text-[#111827] shadow-md transition-colors hover:border-[#168fff] hover:text-[#168fff]"
                title={summaryPanelOpen ? '收起概要' : '展开概要'}
                aria-label={summaryPanelOpen ? '收起概要' : '展开概要'}
              >
                <FileText className="h-5 w-5" />
              </button>
            </div>
          </div>

          <div className="space-y-4">
            <FieldRow label="窗口名称">
              <input
                value={formData.profileName}
                onChange={event => onChange('profileName', event.target.value)}
                placeholder="未命名窗口"
                className={controlClass}
              />
            </FieldRow>

            <FieldRow label="分组">
              <GroupSelector
                groups={groups}
                value={formData.groupId || ''}
                onChange={groupId => onChange('groupId', groupId)}
                placeholder="未分组"
                className="w-full"
              />
            </FieldRow>

            <FieldRow label="标签" alignStart>
              <TagInput
                value={formData.tags || []}
                onChange={tags => onChange('tags', tags)}
                suggestions={allTags}
                placeholder="输入标签后按回车"
              />
            </FieldRow>

            <FieldRow label="系统">
              <div className="grid grid-cols-[270px_minmax(0,1fr)] gap-2">
                <div className="grid h-9 grid-cols-5 overflow-hidden rounded-md border border-[#c7d1dd] bg-white">
                  {systems.map(item => {
                    const Icon = item.Icon
                    const active = system === item.value
                    return (
                      <button
                        key={item.value}
                        type="button"
                        onClick={() => onChange('system', item.value)}
                        className={clsx(
                          'flex items-center justify-center border-r border-[#e2e8f0] last:border-r-0 transition-colors',
                          active ? 'bg-[#edf7ff] ring-2 ring-inset ring-[#168fff]' : 'hover:bg-[#f8fafc]',
                        )}
                        title={item.label}
                        aria-label={item.label}
                      >
                        <Icon className={clsx('h-4 w-4', item.className)} />
                      </button>
                    )
                  })}
                </div>
                <div className="relative">
                  <select
                    value={systemVersion}
                    onChange={event => onChange('systemVersion', event.target.value)}
                    className={clsx(controlClass, 'appearance-none pr-9')}
                  >
                    <option>Windows 11</option>
                    <option>Windows 10</option>
                    <option>macOS 14</option>
                    <option>Ubuntu 22.04</option>
                    <option>Android 13</option>
                  </select>
                  <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#64748b]" />
                </div>
              </div>
            </FieldRow>

            <FieldRow label="内核">
              <div className="grid grid-cols-[270px_minmax(0,1fr)] gap-2">
                <div className="grid h-9 grid-cols-2 overflow-hidden rounded-md border border-[#c7d1dd] bg-white">
                  {browserTypes.map(item => {
                    const Icon = item.Icon
                    const active = browserCore === item.value
                    return (
                      <button
                        key={item.value}
                        type="button"
                        onClick={() => onChange('browserCore', item.value)}
                        className={clsx(
                          'inline-flex items-center justify-center gap-2 border-r border-[#e2e8f0] text-sm font-semibold last:border-r-0 transition-colors',
                          active ? 'bg-[#edf7ff] text-[#168fff] ring-2 ring-inset ring-[#168fff]' : 'text-[#111827] hover:bg-[#f8fafc]',
                        )}
                      >
                        <Icon className="h-4 w-4" />
                        {item.label}
                      </button>
                    )
                  })}
                </div>
                <div className="relative">
                  <select
                    value={browserVersion}
                    onChange={event => onChange('browserVersion', event.target.value)}
                    className={clsx(controlClass, 'appearance-none pr-9')}
                  >
                    <option>{defaultCore?.coreName || 'RoxyChrome 149'}</option>
                    {cores.map(core => (
                      <option key={core.coreId} value={core.coreName}>
                        {core.coreName}
                      </option>
                    ))}
                    <option>Chrome 149</option>
                    <option>Firefox 128</option>
                  </select>
                  <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#64748b]" />
                </div>
              </div>
            </FieldRow>

            <FieldRow label="User-Agent">
              <div className="grid grid-cols-[minmax(0,1fr)_72px] gap-2">
                <input
                  value={userAgent}
                  onChange={event => onChange('userAgent', event.target.value)}
                  className={controlClass}
                />
                <button
                  type="button"
                  onClick={handleRandomFingerprint}
                  className="inline-flex h-9 items-center justify-center gap-1.5 rounded-md border border-[#c7d1dd] bg-white text-sm font-semibold text-[#111827] transition-colors hover:border-[#168fff] hover:text-[#168fff]"
                >
                  <RefreshCw className="h-4 w-4" />
                  随机
                </button>
              </div>
            </FieldRow>

            <FieldRow label="Cookies" alignStart>
              <div className="relative">
                <textarea
                  value={formData.cookies || ''}
                  onChange={event => onChange('cookies', event.target.value)}
                  rows={4}
                  placeholder="支持格式: JSON、Netscape、Name=Value&#10;Name=Value 未写 domain 时，将按行从下方平台账号的网址自动填充"
                  className={clsx(textAreaClass, 'pr-9')}
                />
                <input
                  ref={cookieFileInputRef}
                  type="file"
                  accept=".json,.txt,application/json,text/plain"
                  className="hidden"
                  onChange={handleCookieFileSelected}
                />
                <button
                  type="button"
                  onClick={() => cookieFileInputRef.current?.click()}
                  className="absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded-md text-[#64748b] transition-colors hover:bg-[#edf2f7] hover:text-[#168fff]"
                  title="从文件导入 Cookie（JSON / Netscape / Name=Value）"
                  aria-label="从文件导入 Cookie"
                >
                  <Paperclip className="h-4 w-4" />
                </button>
              </div>
            </FieldRow>
          </div>

          <div className="mt-4">
            <SectionDivider
              title="代理 IP"
              actions={
                <>
                  <InlineAction icon={RefreshCw} onClick={onOpenProxyPicker}>选择</InlineAction>
                  <InlineAction icon={Plus} onClick={onOpenProxyImport}>添加</InlineAction>
                </>
              }
            >
              <p className="mt-0.5 flex items-center gap-1.5 text-xs text-[#475569]">
                本机网络环境: SG/Singapore(54.151.163.131)
                <HelpCircle className="h-3.5 w-3.5 text-[#94a3b8]" />
              </p>
              {(selectedProxy || formData.proxyConfig) && (
                <p className="mt-2 text-xs text-[#168fff]">
                  当前代理: {selectedProxy?.proxyName || selectedProxy?.proxyId || formData.proxyConfig}
                </p>
              )}
            </SectionDivider>

            <SectionDivider
              title="平台账号"
              actions={
                <>
                  <InlineAction icon={RefreshCw} onClick={onOpenAccountPicker}>选择</InlineAction>
                  <InlineAction icon={Plus} onClick={onOpenAccountAdd}>添加</InlineAction>
                </>
              }
            >
              <SelectedAccounts
                accountIds={formData.accountIds || []}
                accounts={accounts}
                onRemove={accountId => onChange('accountIds', (formData.accountIds || []).filter(id => id !== accountId))}
              />
            </SectionDivider>

            <SectionDivider
              title="启动扩展"
              actions={loadableExtensions.length > 0 && (
                <span className="text-xs font-medium text-[#64748b]">
                  已选 {selectedLoadableExtensionCount} / {loadableExtensions.length}
                </span>
              )}
            >
              <div className="space-y-3">
                {extensionsLoadError && (
                  <div className="rounded-md border border-[#ef4444]/40 bg-[#fef2f2] px-3 py-2 text-sm text-[#b91c1c]">
                    {extensionsLoadError}
                  </div>
                )}

                {loadableExtensions.length > 0 ? (
                  <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                    {loadableExtensions.map(extension => {
                      const checked = selectedExtensionIds.includes(extension.extensionId)
                      return (
                        <label
                          key={extension.extensionId}
                          className={clsx(
                            'flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors',
                            checked
                              ? 'border-[#168fff] bg-[#edf7ff]'
                              : 'border-[#cbd5e1] bg-white hover:border-[#94a3b8]',
                          )}
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => onExtensionToggle(extension.extensionId)}
                            className="mt-1 h-4 w-4 accent-[#168fff]"
                          />
                          <Package className={clsx('mt-0.5 h-4 w-4 shrink-0', checked ? 'text-[#168fff]' : 'text-[#64748b]')} />
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-[#111827]">
                              {extension.extensionName || '未命名扩展'}
                            </span>
                            <span className="block truncate text-xs text-[#64748b]" title={extension.extensionPath}>
                              {extension.version ? `v${extension.version} · ` : ''}{extension.extensionPath}
                            </span>
                          </span>
                        </label>
                      )
                    })}
                  </div>
                ) : (
                  <div className="rounded-md border border-[#cbd5e1] bg-[#f8fafc] px-4 py-5 text-sm text-[#64748b]">
                    暂无可加载的本地启用扩展。请先在扩展管理中添加本地解压目录并启用。
                  </div>
                )}
              </div>
            </SectionDivider>

            <CollapsibleRow
              id="urls"
              title="URLs"
              icon={Link2}
              open={openSections.urls}
              onToggle={toggleSection}
            >
              <textarea
                value={formData.urls || ''}
                onChange={event => onChange('urls', event.target.value)}
                rows={3}
                placeholder="每行一个启动网址"
                className={textAreaClass}
              />
            </CollapsibleRow>

            <CollapsibleRow
              id="basic"
              title="基础设置"
              icon={Settings}
              open={openSections.basic}
              onToggle={toggleSection}
            >
              <div className="space-y-4">
                <SettingRow label="语言" info>
                  <SelectControl
                    value={formData.language || 'auto'}
                    options={languageOptions}
                    onChange={value => onChange('language', value)}
                  />
                </SettingRow>

                <SettingRow label="界面语言" info>
                  <SelectControl
                    value={formData.uiLanguage || 'auto'}
                    options={languageOptions}
                    onChange={value => onChange('uiLanguage', value)}
                  />
                </SettingRow>

                <SettingRow label="时区">
                  <SelectControl
                    value={formData.timezone || 'auto'}
                    options={timezoneOptions}
                    onChange={value => onChange('timezone', value)}
                  />
                </SettingRow>

                <SettingRow label="地理位置提示">
                  <SegmentedControl
                    value={formData.geolocationDisplay || 'allow'}
                    options={[
                      { value: 'ask', label: '询问' },
                      { value: 'allow', label: '允许' },
                      { value: 'deny', label: '禁止' },
                    ]}
                    onChange={value => onChange('geolocationDisplay', value)}
                    className="w-[178px]"
                  />
                </SettingRow>

                <SettingRow label="地理位置">
                  <SegmentedControl
                    value={formData.geolocation || 'auto'}
                    options={[
                      { value: 'auto', label: '基于 IP 匹配' },
                    ]}
                    onChange={value => onChange('geolocation', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="声音">
                  <SegmentedControl
                    value={formData.audio === false ? 'off' : 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('audio', value === 'on')}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="图片">
                  <SegmentedControl
                    value={formData.image === false ? 'off' : 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('image', value === 'on')}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="视频">
                  <SegmentedControl
                    value={formData.video === false ? 'off' : 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('video', value === 'on')}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="窗口大小">
                  <div className="space-y-3">
                    <SegmentedControl
                      value={formData.resolution || 'custom'}
                      options={[
                        { value: 'custom', label: '自定义' },
                        { value: 'fullscreen', label: '全屏' },
                      ]}
                      onChange={value => onChange('resolution', value)}
                      className="w-[132px]"
                    />
                    <div className="flex flex-wrap gap-2">
                      <DimensionInput
                        label="宽"
                        value={formData.windowWidth || '1000'}
                        onChange={value => onChange('windowWidth', value)}
                      />
                      <DimensionInput
                        label="高"
                        value={formData.windowHeight || '1000'}
                        onChange={value => onChange('windowHeight', value)}
                      />
                    </div>
                  </div>
                </SettingRow>

                <SettingRow label="窗口位置" info>
                  <WindowPositionPicker
                    value={formData.windowPosition || 'top-left'}
                    onChange={value => onChange('windowPosition', value)}
                  />
                </SettingRow>

                <SettingRow label="搜索引擎">
                  <div className="relative w-full max-w-[590px]">
                    <select
                      value={formData.searchEngine || 'google'}
                      onChange={event => onChange('searchEngine', event.target.value)}
                      className={clsx(controlClass, 'appearance-none pl-9 pr-9')}
                    >
                      <option value="google">Google</option>
                      <option value="bing">Bing</option>
                      <option value="duckduckgo">DuckDuckGo</option>
                      <option value="baidu">Baidu</option>
                    </select>
                    <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm font-bold text-[#4285f4]">G</span>
                    <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#64748b]" />
                  </div>
                </SettingRow>
              </div>
            </CollapsibleRow>

            <CollapsibleRow
              id="advanced"
              title="高级指纹设置"
              icon={Shield}
              open={openSections.advanced}
              onToggle={toggleSection}
            >
              <div className="space-y-4">
                <SettingRow label="分辨率">
                  <SegmentedControl
                    value={formData.screenResolution || 'system'}
                    options={[
                      { value: 'system', label: '跟随系统' },
                      { value: 'custom', label: '自定义' },
                    ]}
                    onChange={value => onChange('screenResolution', value)}
                    className="w-[158px]"
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="字体指纹">
                  <SegmentedControl
                    value={formData.fontFingerprint || 'random'}
                    options={[
                      { value: 'random', label: '随机' },
                      { value: 'system', label: '跟随系统' },
                    ]}
                    onChange={value => onChange('fontFingerprint', value)}
                    className="w-[146px]"
                  />
                </SettingRow>

                <SettingRow label="WebRTC">
                  <SegmentedControl
                    value={formData.webrtc || 'disabled'}
                    options={[
                      { value: 'replace', label: '替换' },
                      { value: 'real', label: '真实' },
                      { value: 'disabled', label: '禁止' },
                    ]}
                    onChange={value => onChange('webrtc', value)}
                    className="w-[178px]"
                  />
                </SettingRow>

                <SettingRow label="WebGL 图像">
                  <SegmentedControl
                    value={formData.webgl || 'random'}
                    options={[
                      { value: 'random', label: '随机' },
                      { value: 'real', label: '真实' },
                    ]}
                    onChange={value => onChange('webgl', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="WebGL Info">
                  <SegmentedControl
                    value={formData.webglInfo || 'random'}
                    options={[
                      { value: 'real', label: '真实' },
                      { value: 'random', label: '随机' },
                      { value: 'custom', label: '自定义' },
                    ]}
                    onChange={value => onChange('webglInfo', value)}
                    className="w-[192px]"
                  />
                </SettingRow>

                <SettingRow label="WebGL 厂商">
                  <RandomizedInput
                    value={formData.webglVendor || webglProfiles[0].vendor}
                    onChange={value => onChange('webglVendor', value)}
                    onRandom={() => {
                      const webgl = pickRandom(webglProfiles)
                      onChange('webglVendor', webgl.vendor)
                      onChange('webglRenderer', webgl.renderer)
                    }}
                  />
                </SettingRow>

                <SettingRow label="WebGL 渲染" alignStart>
                  <RandomizedInput
                    value={formData.webglRenderer || webglProfiles[0].renderer}
                    onChange={value => onChange('webglRenderer', value)}
                    onRandom={() => {
                      const webgl = pickRandom(webglProfiles)
                      onChange('webglVendor', webgl.vendor)
                      onChange('webglRenderer', webgl.renderer)
                    }}
                    multiline
                  />
                </SettingRow>

                <SettingRow label="WebGpu">
                  <SegmentedControl
                    value={formData.webgpu || 'match'}
                    options={[
                      { value: 'match', label: '基于WebGL匹配' },
                      { value: 'real', label: '真实' },
                      { value: 'disabled', label: '禁止' },
                    ]}
                    onChange={value => onChange('webgpu', value)}
                    className="w-auto"
                  />
                </SettingRow>

                {[
                  ['Canvas', 'canvas'],
                  ['AudioContext', 'audioContext'],
                  ['Client Rects', 'clientRects'],
                ].map(([label, field]) => (
                  <SettingRow key={field} label={label}>
                    <SegmentedControl
                      value={(formData[field as keyof CreateWindowFormState] as string) || 'random'}
                      options={[
                        { value: 'random', label: '随机' },
                        { value: 'real', label: '真实' },
                      ]}
                      onChange={value => onChange(field as keyof CreateWindowFormState, value)}
                      className="w-[120px]"
                    />
                  </SettingRow>
                ))}

                {[
                  ['Speech Voices', 'speechVoices'],
                  ['媒体设备', 'mediaDevices'],
                  ['设备名称', 'deviceName'],
                ].map(([label, field]) => (
                  <SettingRow key={field} label={label}>
                    <SegmentedControl
                      value={(formData[field as keyof CreateWindowFormState] as string) || 'random'}
                      options={[
                        { value: 'random', label: '随机' },
                        { value: 'real', label: '真实' },
                      ]}
                      onChange={value => onChange(field as keyof CreateWindowFormState, value)}
                      className="w-[120px]"
                      disabled
                      title={kernelUnsupportedTitle}
                    />
                  </SettingRow>
                ))}

                <SettingRow label="Do Not Track">
                  <SegmentedControl
                    value={formData.doNotTrack || 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('doNotTrack', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="设备名称">
                  <RandomizedInput
                    value={formData.deviceNameValue || deviceNameSamples[0]}
                    onChange={value => onChange('deviceNameValue', value)}
                    onRandom={() => onChange('deviceNameValue', pickRandom(deviceNameSamples))}
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="MAC地址">
                  <SegmentedControl
                    value={formData.macAddress || 'custom'}
                    options={[
                      { value: 'real', label: '真实' },
                      { value: 'custom', label: '自定义' },
                    ]}
                    onChange={value => onChange('macAddress', value)}
                    className="w-[132px]"
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="MAC地址">
                  <RandomizedInput
                    value={formData.macAddressValue || macAddressSamples[0]}
                    onChange={value => onChange('macAddressValue', value)}
                    onRandom={() => onChange('macAddressValue', pickRandom(macAddressSamples))}
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="硬件并发数">
                  <SelectControl
                    value={formData.hardwareConcurrency || '12'}
                    options={['2', '4', '6', '8', '12', '16', '24'].map(value => ({ value, label: `${value}核` }))}
                    onChange={value => onChange('hardwareConcurrency', value)}
                  />
                </SettingRow>

                <SettingRow label="设备内存">
                  <SelectControl
                    value={formData.deviceMemory || '8'}
                    options={['2', '4', '8', '16', '32'].map(value => ({ value, label: `${value}G` }))}
                    onChange={value => onChange('deviceMemory', value)}
                  />
                </SettingRow>

                <SettingRow label="SSL指纹设置">
                  <SegmentedControl
                    value={formData.sslFingerprint || 'disabled'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'disabled', label: '关闭' },
                    ]}
                    onChange={value => onChange('sslFingerprint', value)}
                    className="w-[120px]"
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="端口扫描保护">
                  <SegmentedControl
                    value={formData.portScanProtection || 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('portScanProtection', value)}
                    className="w-[120px]"
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="扫描白名单">
                  <input
                    value={formData.portScanAllowList || ''}
                    disabled
                    title={kernelUnsupportedTitle}
                    onChange={event => onChange('portScanAllowList', event.target.value)}
                    placeholder="允许被网站扫描的端口，多个用英文逗号隔开"
                    className={clsx(controlClass, 'max-w-[590px] cursor-not-allowed bg-[#f8fafc] text-[#94a3b8]')}
                  />
                </SettingRow>

                <SettingRow label="硬件加速模式">
                  <SegmentedControl
                    value={formData.hardwareAcceleration || 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('hardwareAcceleration', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="禁用沙盒">
                  <SegmentedControl
                    value={formData.sandbox || 'off'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('sandbox', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="启动参数">
                  <input
                    value={launchArgsText}
                    onChange={event => onLaunchArgsTextChange(event.target.value)}
                    placeholder="浏览器启动参数,如 --mute 多个参数以英文分号分隔"
                    className={clsx(controlClass, 'max-w-[590px]')}
                  />
                </SettingRow>
              </div>
            </CollapsibleRow>

            <CollapsibleRow
              id="preferences"
              title="偏好设置"
              icon={Wand2}
              open={openSections.preferences}
              onToggle={toggleSection}
            >
              <div className="space-y-4">
                {[
                  { label: '写入默认书签', field: 'customBookmarks', defaultOn: false },
                  { label: '启动浏览器前删除缓存文件', field: 'clearCacheBeforeStart', defaultOn: false },
                  { label: '启动浏览器前删除Cookie', field: 'clearCookiesBeforeStart', defaultOn: false },
                  { label: '启动浏览器前删除Local Storage', field: 'clearLocalStorageBeforeStart', defaultOn: false },
                  { label: '启动浏览器时随机指纹', field: 'randomFingerprintOnStart', defaultOn: false },
                ].map(item => {
                  const field = item.field as keyof CreateWindowFormState
                  const rawValue = formData[field]
                  const isOn = typeof rawValue === 'boolean' ? rawValue : item.defaultOn

                  return (
                    <SettingRow key={item.field} label={item.label}>
                      <SegmentedControl
                        value={isOn ? 'on' : 'off'}
                        options={[
                          { value: 'on', label: '开启' },
                          { value: 'off', label: '关闭' },
                        ]}
                        onChange={value => onChange(field, value === 'on')}
                        className="w-[120px]"
                      />
                    </SettingRow>
                  )
                })}

                {[
                  { label: '同步书签', field: 'syncBookmarks', defaultOn: false },
                  { label: '同步历史记录', field: 'syncHistory', defaultOn: false },
                  { label: '同步标签页', field: 'syncTabs', defaultOn: true },
                  { label: '同步Cookie', field: 'syncCookies', defaultOn: true },
                  { label: '同步扩展应用程序', field: 'syncExtensions', defaultOn: false },
                  { label: '同步已保存的用户名密码', field: 'syncPasswords', defaultOn: true },
                  { label: '同步IndexedDB', field: 'syncIndexedDB', defaultOn: false },
                  { label: '同步Local Storage', field: 'syncLocalStorage', defaultOn: true },
                  { label: '同步Session Storage', field: 'syncSessionStorage', defaultOn: false },
                  { label: '弹出保存密码提示', field: 'passwordPrompt', defaultOn: true },
                ].map(item => {
                  const field = item.field as keyof CreateWindowFormState
                  const rawValue = formData[field]
                  const isOn = typeof rawValue === 'boolean' ? rawValue : item.defaultOn

                  return (
                    <SettingRow key={item.field} label={item.label}>
                      <SegmentedControl
                        value={isOn ? 'on' : 'off'}
                        options={[
                          { value: 'on', label: '开启' },
                          { value: 'off', label: '关闭' },
                        ]}
                        onChange={value => onChange(field, value === 'on')}
                        className="w-[120px]"
                        disabled
                        title={unsupportedTitle}
                      />
                    </SettingRow>
                  )
                })}

                {/* 启动门控：启动前探测出口 IP，按规则中止打开（后端 applyProfileStartGate 落地）。 */}
                {[
                  { label: '网络不通停止打开', field: 'keepNetworkOn', defaultOn: false },
                  { label: 'IP发生变化停止打开', field: 'stopOnIpChange', defaultOn: false },
                  { label: 'IP对应国家/地区发生改变停止打开', field: 'stopOnIpRegionChange', defaultOn: false },
                ].map(item => {
                  const field = item.field as keyof CreateWindowFormState
                  const rawValue = formData[field]
                  const isOn = typeof rawValue === 'boolean' ? rawValue : item.defaultOn

                  return (
                    <SettingRow key={item.field} label={item.label}>
                      <SegmentedControl
                        value={isOn ? 'on' : 'off'}
                        options={[
                          { value: 'on', label: '开启' },
                          { value: 'off', label: '关闭' },
                        ]}
                        onChange={value => onChange(field, value === 'on')}
                        className="w-[120px]"
                      />
                    </SettingRow>
                  )
                })}

                <SettingRow label="打开工作台">
                  <SegmentedControl
                    value={formData.openWorkbench || 'follow'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                      { value: 'follow', label: '跟随软件设置' },
                    ]}
                    onChange={value => onChange('openWorkbench', value)}
                    className="w-auto"
                    disabled
                    title={kernelUnsupportedTitle}
                  />
                </SettingRow>

                <SettingRow label="IP 变化提醒" info>
                  <SegmentedControl
                    value={formData.ipChangeReminder === 'on' ? 'on' : 'off'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('ipChangeReminder', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="是否开启谷歌登录" info>
                  <SegmentedControl
                    value={formData.googleLogin === 'off' ? 'off' : 'on'}
                    options={[
                      { value: 'on', label: '开启' },
                      { value: 'off', label: '关闭' },
                    ]}
                    onChange={value => onChange('googleLogin', value)}
                    className="w-[120px]"
                  />
                </SettingRow>

                <SettingRow label="网址访问黑名单" info alignStart>
                  <textarea
                    value={formData.websiteAccessBlacklist || ''}
                    disabled
                    title={unsupportedTitle}
                    onChange={event => onChange('websiteAccessBlacklist', event.target.value)}
                    rows={3}
                    placeholder="每行一个 URL，换行以添加多个"
                    className={clsx(textAreaClass, 'min-h-[74px] max-w-[590px] cursor-not-allowed bg-[#f8fafc] text-[#94a3b8]')}
                  />
                </SettingRow>

                <SettingRow label="网址访问白名单" info alignStart>
                  <textarea
                    value={formData.websiteAccessWhitelist || ''}
                    onChange={event => onChange('websiteAccessWhitelist', event.target.value)}
                    rows={3}
                    placeholder="每行一个 URL/域名；填写后仅放行匹配项，其余全部拦截"
                    className={clsx(textAreaClass, 'min-h-[74px] max-w-[590px]')}
                  />
                </SettingRow>
              </div>
            </CollapsibleRow>
          </div>
        </main>
        {summaryPanelOpen && (
          <SummaryPanel
            system={system}
            systemLabel={systemLabel}
            summaryItems={summaryItems}
            onCollapse={() => setSummaryPanelOpen(false)}
            onRandomize={handleRandomFingerprint}
            onApplyPreset={handleApplyPreset}
          />
        )}
        {!summaryPanelOpen && (
          <div className="absolute left-[calc(50%+450px)] top-7 hidden xl:block">
            <button
              type="button"
              onClick={() => setSummaryPanelOpen(true)}
              className="group relative flex h-14 w-12 flex-col items-center justify-center gap-1 rounded-xl border border-[#d7e0ea] bg-white text-[#111827] shadow-md transition-colors hover:border-[#168fff] hover:text-[#168fff]"
              title="展开"
              aria-label="展开概要"
            >
              <span className="pointer-events-none absolute -left-2 top-2 rounded bg-[#334155] px-2 py-1 text-xs font-semibold text-white opacity-0 shadow-md transition-opacity group-hover:opacity-100">
                展开
              </span>
              <FileText className="h-4 w-4" />
              <span className="text-[11px] font-semibold">概要</span>
            </button>
          </div>
        )}
        </div>
      </div>

      <footer className="shrink-0 border-t border-[#e5eaf0] bg-white">
        <div className={clsx('mx-auto flex h-[58px] w-full items-center justify-between px-5', summaryPanelOpen ? 'max-w-[1180px]' : 'max-w-[860px]')}>
          <div className="flex items-center gap-2">
            <FooterButton icon={Database}>默认项目</FooterButton>
            <FooterButton icon={Tag}>标签</FooterButton>
            <button
              type="button"
              onClick={() => setNotesOpen(true)}
              className={clsx(
                'inline-flex h-9 items-center gap-2 rounded-md border px-3 text-sm font-medium shadow-sm transition-colors',
                formData.notes?.trim()
                  ? 'border-[#168fff] bg-[#edf7ff] text-[#168fff]'
                  : 'border-[#cbd5e1] bg-white text-[#111827] hover:border-[#168fff] hover:text-[#168fff]',
              )}
            >
              <FileText className="h-4 w-4" />
              备注{formData.notes?.trim() ? ' ●' : ''}
            </button>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => { setTemplateNameDraft(formData.profileName?.trim() || ''); setSaveTemplateOpen(true) }}
              className="inline-flex h-9 items-center gap-3 rounded-md border border-[#cbd5e1] bg-white px-4 text-sm font-semibold text-[#111827] shadow-sm transition-colors hover:border-[#168fff] hover:text-[#168fff]"
            >
              存为新模板
            </button>
            <button
              type="button"
              onClick={onSave}
              disabled={saving}
              className="inline-flex h-9 items-center justify-center rounded-md bg-[#168fff] px-5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-[#0f7edf] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {saving ? '创建中...' : '创建窗口'}
            </button>
          </div>
        </div>
      </footer>

      <Modal
        open={saveTemplateOpen}
        onClose={() => setSaveTemplateOpen(false)}
        title="存为新模板"
        width="420px"
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setSaveTemplateOpen(false)}>取消</Button>
            <Button
              onClick={() => {
                onSaveAsTemplate(templateNameDraft)
                setSaveTemplateOpen(false)
              }}
              disabled={!templateNameDraft.trim()}
            >
              保存模板
            </Button>
          </div>
        }
      >
        <div className="space-y-2">
          <p className="text-sm text-[#475569]">将当前窗口配置（指纹、启动参数、偏好等）保存为可复用的创建模板。</p>
          <Input
            value={templateNameDraft}
            onChange={e => setTemplateNameDraft(e.target.value)}
            placeholder="请输入模板名称"
            autoFocus
          />
        </div>
      </Modal>

      <Modal
        open={notesOpen}
        onClose={() => setNotesOpen(false)}
        title="窗口备注"
        width="460px"
        footer={<Button onClick={() => setNotesOpen(false)}>完成</Button>}
      >
        <textarea
          value={formData.notes || ''}
          onChange={e => onChange('notes', e.target.value)}
          rows={6}
          placeholder="为该窗口添加备注（仅本地保存，不影响启动）"
          className={clsx(textAreaClass, 'min-h-[120px]')}
        />
      </Modal>
    </div>
  )
}
