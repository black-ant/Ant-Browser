import type {
  BrowserProfileAutomationTarget,
  BrowserProfileCopyMode,
  BrowserProfileCopyOptions,
} from './types'

export interface BrowserProfileAutomationTargetOption {
  value: BrowserProfileAutomationTarget
  label: string
  detail: string
}

export const BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS: BrowserProfileAutomationTargetOption[] = [
  { value: 'seed', label: '指纹种子', detail: '生成新种子' },
  { value: 'identity', label: '浏览器身份', detail: '品牌 / 平台' },
  { value: 'locale', label: '语言与时区', detail: '语言 / 时区' },
  { value: 'screen', label: '窗口大小', detail: '浏览器窗口' },
  { value: 'hardware', label: '硬件参数', detail: 'CPU 核心数' },
  { value: 'network', label: '网络与伪装', detail: 'WebRTC / 排除伪装' },
]

export const BROWSER_PROFILE_AUTOMATION_TARGET_PREFIXES: Record<BrowserProfileAutomationTarget, string[]> = {
  seed: ['--fingerprint'],
  identity: ['--fingerprint-brand', '--fingerprint-brand-version', '--fingerprint-platform', '--fingerprint-platform-version'],
  locale: ['--lang', '--accept-lang', '--timezone'],
  screen: ['--window-size'],
  hardware: ['--fingerprint-hardware-concurrency'],
  render: ['--fingerprint'],
  fonts: ['--fingerprint'],
  network: ['--webrtc-ip-handling-policy', '--disable-non-proxied-udp', '--disable-spoofing'],
  devices: ['--fingerprint'],
}

const LEGACY_AUTOMATION_TARGET_ALIASES: Partial<Record<BrowserProfileAutomationTarget, BrowserProfileAutomationTarget>> = {
  render: 'seed',
  fonts: 'seed',
  devices: 'seed',
}

export const DEFAULT_BROWSER_PROFILE_AUTOMATION_TARGETS: BrowserProfileAutomationTarget[] = ['seed']

export function createBrowserProfileCopyOptions(
  mode: BrowserProfileCopyMode = 'auto_fingerprint',
  automationTargets: BrowserProfileAutomationTarget[] = DEFAULT_BROWSER_PROFILE_AUTOMATION_TARGETS,
): BrowserProfileCopyOptions {
  return {
    mode,
    automationTargets: dedupeAutomationTargets(automationTargets),
  }
}

export function dedupeAutomationTargets(targets: BrowserProfileAutomationTarget[]): BrowserProfileAutomationTarget[] {
  const allowed = new Set<BrowserProfileAutomationTarget>(
    BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS.map((item) => item.value),
  )
  const output: BrowserProfileAutomationTarget[] = []
  const seen = new Set<BrowserProfileAutomationTarget>()
  for (const target of targets) {
    const normalizedTarget = LEGACY_AUTOMATION_TARGET_ALIASES[target] || target
    if (!allowed.has(normalizedTarget) || seen.has(normalizedTarget)) {
      continue
    }
    seen.add(normalizedTarget)
    output.push(normalizedTarget)
  }
  return output
}

export function isBrowserProfileCopyOptionsValid(options: BrowserProfileCopyOptions): boolean {
  if (options.mode !== 'auto_fingerprint') {
    return true
  }
  return options.automationTargets.length > 0
}

export function applyBrowserProfileCopyOptionsToArgs(
  sourceArgs: string[],
  defaultArgs: string[],
  options: BrowserProfileCopyOptions,
): string[] {
  if (options.mode === 'regular') {
    return [...sourceArgs]
  }

  const baseArgs = sourceArgs.length > 0 ? sourceArgs : defaultArgs
  if (baseArgs.length === 0) {
    return []
  }

  const targets = options.automationTargets.length > 0
    ? dedupeAutomationTargets(options.automationTargets)
    : DEFAULT_BROWSER_PROFILE_AUTOMATION_TARGETS
  const targetSet = new Set<BrowserProfileAutomationTarget>(targets)
  const defaultArgsByKey = mapArgsByKey(defaultArgs)
  const outputArgs: string[] = []
  const outputKeys = new Set<string>()

  for (const arg of baseArgs) {
    const trimmed = arg.trim()
    if (!trimmed) {
      continue
    }

    const key = getArgKey(trimmed)
    if (!key) {
      outputArgs.push(trimmed)
      continue
    }

    const target = findAutomationTargetByKey(key)
    if (!target || !targetSet.has(target)) {
      outputArgs.push(trimmed)
      outputKeys.add(key)
      continue
    }

    const replacement = defaultArgsByKey.get(key)
    if (replacement && !outputKeys.has(key)) {
      outputArgs.push(replacement)
      outputKeys.add(key)
    }
  }

  for (const arg of defaultArgs) {
    const trimmed = arg.trim()
    if (!trimmed) {
      continue
    }
    const key = getArgKey(trimmed)
    if (!key) {
      continue
    }
    const target = findAutomationTargetByKey(key)
    if (!target || !targetSet.has(target) || outputKeys.has(key)) {
      continue
    }
    outputArgs.push(trimmed)
    outputKeys.add(key)
  }

  return outputArgs
}

function mapArgsByKey(args: string[]): Map<string, string> {
  const output = new Map<string, string>()
  for (const arg of args) {
    const trimmed = arg.trim()
    if (!trimmed) {
      continue
    }
    const key = getArgKey(trimmed)
    if (!key) {
      continue
    }
    output.set(key, trimmed)
  }
  return output
}

function getArgKey(arg: string): string | null {
  const trimmed = arg.trim()
  if (!trimmed.startsWith('--')) {
    return null
  }
  const eqIndex = trimmed.indexOf('=')
  return (eqIndex > 0 ? trimmed.slice(0, eqIndex) : trimmed).toLowerCase()
}

function findAutomationTargetByKey(key: string): BrowserProfileAutomationTarget | null {
  const normalizedKey = key.toLowerCase()
  for (const option of BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS) {
    if (BROWSER_PROFILE_AUTOMATION_TARGET_PREFIXES[option.value].includes(normalizedKey)) {
      return option.value
    }
  }
  return null
}
