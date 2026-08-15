import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ChevronDown, ChevronUp, FolderOpen, HelpCircle, Layers, ShieldCheck } from 'lucide-react'
import { Button, Card, ConfirmModal, FormItem, Input, Modal, Select, Textarea, toast } from '../../../shared/components'
import type { BrowserCore, BrowserFingerprintCapabilityReport, BrowserFingerprintCapabilityRow, BrowserFingerprintCheckResult, BrowserProfileInput, BrowserProxy, BrowserGroup, ProxyLocationResolveResult } from '../types'
import { browserProxyResolveLocation, checkBrowserProfileFingerprint, createBrowserProfile, fetchAllTags, fetchBrowserCores, fetchBrowserProfileFingerprintMatrix, fetchBrowserProfiles, fetchBrowserProxies, fetchBrowserSettings, fetchGroups, openBrowserFingerprintCheck, openUserDataDir, regenerateBrowserProfileFingerprint, updateBrowserProfile, validateBrowserProfileFingerprint, validateProxyConfig, type FingerprintValidationResult } from '../api'
import { FingerprintPanel } from '../components/FingerprintPanel'
import { applyLocaleToFingerprintArgs, validateFingerprintArgs, withAdaptiveDefaultWindowSize } from '../utils/fingerprintSerializer'
import { TagInput } from '../components/TagInput'
import { GroupSelector } from '../components/GroupSelector'
import { ProxyPickerModal } from '../components/ProxyPickerModal'

const fallbackLowLaunchArgs = ['--disable-sync', '--no-first-run']
const directProxyID = '__direct__'
const RESTORE_LAST_SESSION_OPTIONS = [
  { value: '', label: '跟随内核默认' },
  { value: 'enabled', label: '开启：恢复历史标签' },
  { value: 'disabled', label: '关闭：不恢复历史标签' },
]
type ProxySourceMode = 'pool' | 'local'
type BrowserProfileEditForm = BrowserProfileInput & { lastLaunchArgs?: string[] }

function normalizeLaunchArgs(args: string[]): string[] {
  return (args || []).map(item => item.trim()).filter(Boolean)
}

function resolveDefaultLaunchArgs(args: string[]): string[] {
  const normalized = normalizeLaunchArgs(args)
  return normalized.length > 0 ? normalized : fallbackLowLaunchArgs
}

function joinValues(values?: string[]): string {
  return values?.length ? values.join(', ') : '-'
}

function displayValue(value: unknown): string {
  if (value === undefined || value === null || value === '') return '-'
  if (Array.isArray(value)) return joinValues(value)
  return String(value)
}

type FingerprintMatchMode = 'exact' | 'contains' | 'platform' | 'browser-version' | 'platform-version' | 'display'
type FingerprintMatchStatus = 'match' | 'version_compatible' | 'mismatch' | 'not_configured' | 'observe'

function normalizeFingerprintPlatformForMatch(value: unknown): string {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (!normalized) return ''
  if (['windows', 'win', 'win32', 'win64', 'wince'].includes(normalized)) return 'windows'
  if (normalized.startsWith('linux') || normalized === 'x11') return 'linux'
  if (normalized === 'mac' || normalized === 'macos' || normalized.includes('mac')) return 'macos'
  return normalized
}

function versionParts(value: unknown): number[] {
  const match = String(value ?? '').replace(/_/g, '.').match(/\d+(?:\.\d+)*/)
  return match ? match[0].split('.').map(item => Number.parseInt(item, 10)).filter(Number.isFinite) : []
}

function versionList(value: unknown, patterns: RegExp[]): number[][] {
  const text = String(value ?? '').replace(/_/g, '.')
  return patterns.flatMap(pattern => Array.from(text.matchAll(pattern), match => versionParts(match[1])).filter(parts => parts.length > 0))
}

function sameVersionPrefix(left: number[], right: number[]): boolean {
  if (!left.length || !right.length) return false
  const size = Math.min(left.length, right.length)
  return left.slice(0, size).every((part, index) => part === right[index])
}

function matchBrowserVersionStatus(expected: unknown, actual: unknown): FingerprintMatchStatus {
  if (String(actual ?? '').includes(String(expected))) return 'match'
  const expectedParts = versionParts(expected)
  if (!expectedParts.length) return 'mismatch'
  const versions = versionList(actual, [/(?:Chrome|Chromium|Edg|OPR|Vivaldi)\/([0-9]+(?:[._][0-9]+)*)/g, /"version"\s*:\s*"([0-9]+(?:[._][0-9]+)*)"/g])
  return versions.some(actualParts => actualParts[0] === expectedParts[0]) ? 'version_compatible' : 'mismatch'
}

function matchPlatformVersionStatus(expected: unknown, actual: unknown): FingerprintMatchStatus {
  if (String(actual ?? '').includes(String(expected))) return 'match'
  const expectedParts = versionParts(expected)
  if (!expectedParts.length) return 'mismatch'
  const versions = versionList(actual, [/Windows NT\s+([0-9]+(?:[._][0-9]+)*)/g, /Mac OS X\s+([0-9]+(?:[._][0-9]+)*)/g, /Android\s+([0-9]+(?:[._][0-9]+)*)/g, /(?:CPU (?:iPhone )?OS|iPhone OS)\s+([0-9]+(?:[._][0-9]+)*)/g, /"platformVersion"\s*:\s*"([0-9]+(?:[._][0-9]+)*)"/g])
  return versions.some(actualParts => sameVersionPrefix(expectedParts, actualParts)) ? 'version_compatible' : 'mismatch'
}

function matchStatus(expected: unknown, actual: unknown, mode: FingerprintMatchMode = 'exact'): FingerprintMatchStatus {
  if (mode === 'display') return 'observe'
  if (expected === undefined || expected === null || expected === '') return 'not_configured'
  if (mode === 'platform') {
    const expectedPlatform = normalizeFingerprintPlatformForMatch(expected)
    const actualPlatform = normalizeFingerprintPlatformForMatch(actual)
    return expectedPlatform && expectedPlatform === actualPlatform ? 'match' : 'mismatch'
  }
  if (mode === 'browser-version') return matchBrowserVersionStatus(expected, actual)
  if (mode === 'platform-version') return matchPlatformVersionStatus(expected, actual)
  if (Array.isArray(actual)) {
    const expectedItems = String(expected).split(',').map(item => item.trim()).filter(Boolean)
    const actualItems = actual.map(item => String(item).trim()).filter(Boolean)
    return expectedItems.length > 0 && expectedItems.every((item, index) => actualItems[index] === item) ? 'match' : 'mismatch'
  }
  if (mode === 'contains') return String(actual).includes(String(expected)) ? 'match' : 'mismatch'
  return String(expected) === String(actual) ? 'match' : 'mismatch'
}

function FingerprintCheckRow({ label, expected, actual, mode }: { label: string; expected?: unknown; actual: unknown; mode?: FingerprintMatchMode }) {
  const status = matchStatus(expected, actual, mode)
  const statusText = status === 'match' ? '一致' : status === 'version_compatible' ? '口径匹配' : status === 'mismatch' ? '不一致' : status === 'not_configured' ? '未设期望' : '观察值'
  const statusClass = status === 'match' || status === 'version_compatible'
    ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
    : status === 'mismatch'
      ? 'bg-amber-50 text-amber-700 border-amber-200'
      : 'bg-slate-50 text-slate-600 border-slate-200'
  return (
    <div className="grid grid-cols-[120px_minmax(0,1fr)_minmax(0,1fr)_70px] gap-3 items-start py-2 border-b border-[var(--color-border)] last:border-b-0 text-sm">
      <div className="text-[var(--color-text-muted)]">{label}</div>
      <div className="font-mono text-xs break-all text-[var(--color-text-secondary)]">{displayValue(expected)}</div>
      <div className="font-mono text-xs break-all text-[var(--color-text-primary)]">{displayValue(actual)}</div>
      <div><span className={`inline-flex px-2 py-0.5 rounded border text-xs ${statusClass}`}>{statusText}</span></div>
    </div>
  )
}

function FingerprintDisplayRow({ label, actual }: { label: string; actual: unknown }) {
  return <FingerprintCheckRow label={label} actual={actual} mode="display" />
}


const matrixStatusLabels: Record<string, string> = {
  kept: '保留',
  injected: '补齐',
  inferred: '补齐',
  converted: '转换',
  removed: '清理',
  disabled: '关闭',
  overridden: '已覆盖',
  not_effective: '实测无效',
  pending: '待启动',
  kept_legacy: '旧版保留',
  kept_unconfirmed: '保守保留',
  kept_unknown: '原样保留',
}

function matrixStatusClass(status: string): string {
  if (status === 'removed' || status === 'not_effective') return 'bg-red-50 text-red-700 border-red-200'
  if (status === 'disabled') return 'bg-slate-50 text-slate-600 border-slate-200'
  if (status === 'overridden') return 'bg-slate-50 text-slate-600 border-slate-200'
  if (status === 'converted' || status === 'injected' || status === 'inferred') return 'bg-blue-50 text-blue-700 border-blue-200'
  if (status === 'kept_unconfirmed' || status === 'kept_unknown' || status === 'pending') return 'bg-amber-50 text-amber-700 border-amber-200'
  return 'bg-emerald-50 text-emerald-700 border-emerald-200'
}

function FingerprintMatrixRow({ row }: { row: BrowserFingerprintCapabilityRow }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-[120px_88px_minmax(0,1fr)] gap-2 px-3 py-2 border-b border-[var(--color-border)] last:border-b-0 text-sm">
      <div className="font-medium text-[var(--color-text-primary)]">{row.capability}</div>
      <div>
        <span className={`inline-flex px-2 py-0.5 rounded border text-xs ${matrixStatusClass(row.status)}`}>
          {matrixStatusLabels[row.status] || row.status}
        </span>
      </div>
      <div className="min-w-0 space-y-1">
        <div className="text-xs text-[var(--color-text-secondary)]">{row.action || '-'}</div>
        {(row.inputArg || row.runtimeArg) && (
          <div className="font-mono text-xs break-all text-[var(--color-text-muted)]">
            {row.inputArg && <span>配置：{row.inputArg}</span>}
            {row.inputArg && row.runtimeArg && row.inputArg !== row.runtimeArg && <span className="mx-1">→</span>}
            {row.runtimeArg && row.inputArg !== row.runtimeArg && <span>运行：{row.runtimeArg}</span>}
          </div>
        )}
        {row.note && <div className="text-xs text-[var(--color-text-muted)]">{row.note}</div>}
      </div>
    </div>
  )
}

function FingerprintMatrixReport({ report }: { report: BrowserFingerprintCapabilityReport | null }) {
  if (!report) {
    return (
      <div className="mt-4 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-text-muted)]">
        正在读取指纹适配矩阵
      </div>
    )
  }
  const versionText = report.chromeVersion ? `Chrome ${report.chromeVersion}` : '内核版本未知'
  const rows = report.rows || []
  return (
    <div className="mt-4 rounded-lg border border-[var(--color-border)] overflow-hidden">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-1 px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-hover)]">
        <div className="text-sm font-medium text-[var(--color-text-primary)]">版本适配矩阵</div>
        <div className="text-xs text-[var(--color-text-muted)]">
          {versionText} · 配置 {report.rawArgs?.length || 0} 项 · 运行 {report.launchArgs?.length || 0} 项
        </div>
      </div>
      {report.warnings?.length > 0 && (
        <div className="px-3 py-2 border-b border-[var(--color-border)] bg-amber-50 text-xs text-amber-700 space-y-1">
          {report.warnings.map((warning, index) => <div key={`${warning}-${index}`}>{warning}</div>)}
        </div>
      )}
      {rows.length > 0 ? (
        <div>
          {rows.map((row, index) => <FingerprintMatrixRow key={`${row.capability}-${row.inputArg}-${index}`} row={row} />)}
        </div>
      ) : (
        <div className="px-3 py-2 text-sm text-[var(--color-text-muted)]">没有需要转换的指纹参数</div>
      )}
    </div>
  )
}

function resolvePoolProxySelection(
  proxyId: string,
  proxyConfig: string,
  proxies: BrowserProxy[],
): { mode: ProxySourceMode; proxyId: string; proxyConfig: string } {
  const normalizedProxyId = proxyId.trim()
  if (normalizedProxyId) {
    const matchedByID = proxies.find((proxy) => proxy.proxyId.trim() === normalizedProxyId)
    if (matchedByID?.proxyId) {
      return { mode: 'pool', proxyId: matchedByID.proxyId, proxyConfig: '' }
    }
  }

  const rawProxyConfig = proxyConfig.trim()
  const normalizedConfig = rawProxyConfig.toLowerCase()
  if (normalizedConfig) {
    const matchedByConfig = proxies.find((proxy) => (proxy.proxyConfig || '').trim().toLowerCase() === normalizedConfig)
    if (matchedByConfig?.proxyId) {
      return { mode: 'pool', proxyId: matchedByConfig.proxyId, proxyConfig: '' }
    }
    return { mode: 'local', proxyId: '', proxyConfig: rawProxyConfig }
  }

  const directProxy = proxies.find((proxy) => proxy.proxyId === directProxyID)
  return { mode: 'pool', proxyId: directProxy?.proxyId || '', proxyConfig: '' }
}

export function BrowserEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isCreate = id === 'new'
  const [formData, setFormData] = useState<BrowserProfileEditForm>({
    profileName: '',
    userDataDir: '',
    coreId: '',
    restoreLastSession: '',
    fingerprintArgs: [],
    proxyId: directProxyID,
    proxyConfig: '',
    memoryLimitMb: 0,
    launchArgs: [],
    lastLaunchArgs: [],
    tags: [],
    keywords: [],
    groupId: '',
  })
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroup[]>([])
  const [launchArgsText, setLaunchArgsText] = useState('')
  const [allTags, setAllTags] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [proxyPickerOpen, setProxyPickerOpen] = useState(false)
  const [proxyMode, setProxyMode] = useState<ProxySourceMode>('pool')
  const [isDirty, setIsDirty] = useState(false)
  const [leaveConfirm, setLeaveConfirm] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [locationResolving, setLocationResolving] = useState(false)
  const [locationResult, setLocationResult] = useState<ProxyLocationResolveResult | null>(null)
  const [fpValidation, setFpValidation] = useState<FingerprintValidationResult | null>(null)
  const [fpBusy, setFpBusy] = useState(false)
  const [fingerprintChecking, setFingerprintChecking] = useState(false)
  const [fingerprintPageOpening, setFingerprintPageOpening] = useState(false)
  const [fingerprintCheckResult, setFingerprintCheckResult] = useState<BrowserFingerprintCheckResult | null>(null)
  const [fingerprintCheckOpen, setFingerprintCheckOpen] = useState(false)
  const [fingerprintMatrix, setFingerprintMatrix] = useState<BrowserFingerprintCapabilityReport | null>(null)
  const [fingerprintMatrixOpen, setFingerprintMatrixOpen] = useState(false)
  const [launchArgsOpen, setLaunchArgsOpen] = useState(false)
  const [launchArgsHelpOpen, setLaunchArgsHelpOpen] = useState(false)

  useEffect(() => {
    const loadData = async () => {
      const [coreList, proxyList, tagList, groupList, settings] = await Promise.all([
        fetchBrowserCores(),
        fetchBrowserProxies(),
        fetchAllTags(),
        fetchGroups(),
        fetchBrowserSettings(),
      ])
      const resolvedDefaultLaunchArgs = resolveDefaultLaunchArgs(settings.defaultLaunchArgs || [])
      setCores(coreList)
      setProxies(proxyList)
      setAllTags(tagList)
      setGroups(groupList)

      if (isCreate) {
        const resolved = resolvePoolProxySelection('', '', proxyList)
        setProxyMode('pool')
        setFormData((prev) => ({
          ...prev,
          proxyId: resolved.proxyId || directProxyID,
          proxyConfig: '',
          restoreLastSession: '',
          fingerprintArgs: withAdaptiveDefaultWindowSize(settings.defaultFingerprintArgs || []),
        }))
        setLaunchArgsText(resolvedDefaultLaunchArgs.join('\n'))
        return
      }
      const list = await fetchBrowserProfiles()
      const current = list.find(item => item.profileId === id)
      if (!current) return
      const currentLaunchArgs = normalizeLaunchArgs(current.launchArgs)
      const normalizedCoreId = !current.coreId || current.coreId.toLowerCase() === 'default'
        ? ''
        : current.coreId
      const resolvedProxy = resolvePoolProxySelection(current.proxyId || '', current.proxyConfig || '', proxyList)
      setProxyMode(resolvedProxy.mode)
      setFormData({
        profileName: current.profileName,
        userDataDir: current.userDataDir,
        coreId: normalizedCoreId,
        restoreLastSession: current.restoreLastSession || '',
        fingerprintArgs: current.fingerprintArgs,
        proxyId: resolvedProxy.proxyId,
        proxyConfig: resolvedProxy.proxyConfig,
        memoryLimitMb: current.memoryLimitMb || 0,
        launchArgs: currentLaunchArgs,
        lastLaunchArgs: current.lastLaunchArgs || [],
        tags: current.tags,
        keywords: current.keywords || [],
        groupId: current.groupId || '',
      })
      setLaunchArgsText(currentLaunchArgs.join('\n'))
    }
    loadData()
  }, [id, isCreate])

  const fingerprintArgsKey = formData.fingerprintArgs.join('\n')

  useEffect(() => {
    let cancelled = false
    const loadMatrix = async () => {
      try {
        const report = await fetchBrowserProfileFingerprintMatrix(isCreate ? '' : id || '', formData.coreId, formData.fingerprintArgs)
        if (!cancelled) setFingerprintMatrix(report)
      } catch {
        if (!cancelled) setFingerprintMatrix(null)
      }
    }
    loadMatrix()
    return () => { cancelled = true }
  }, [id, isCreate, formData.coreId, fingerprintArgsKey])

  const handleChange = (field: keyof BrowserProfileInput, value: string | string[] | number) => {
    setIsDirty(true)
    setFormData(prev => {
      if (field === 'proxyId') {
        return { ...prev, proxyId: typeof value === 'string' ? value : '' }
      }
      return { ...prev, [field]: value }
    })
  }

  const handleProxyModeChange = (mode: ProxySourceMode) => {
    setIsDirty(true)
    setProxyMode(mode)
    if (mode === 'pool') {
      setFormData((prev) => {
        if (prev.proxyId.trim()) {
          return prev
        }
        const directProxy = proxies.find((proxy) => proxy.proxyId === directProxyID)
        return {
          ...prev,
          proxyId: directProxy?.proxyId || '',
        }
      })
    }
  }

  const handleSave = async () => {
    const resolvedProxyId = proxyMode === 'pool' ? (formData.proxyId || '').trim() : ''
    const resolvedProxyConfig = proxyMode === 'local' ? (formData.proxyConfig || '').trim() : ''
    if (proxyMode === 'local' && !resolvedProxyConfig) {
      setSaveError('请输入本地代理地址')
      return
    }

    const payload: BrowserProfileInput = {
      profileName: formData.profileName,
      userDataDir: formData.userDataDir,
      coreId: formData.coreId,
      restoreLastSession: formData.restoreLastSession || '',
      fingerprintArgs: formData.fingerprintArgs,
      tags: formData.tags,
      keywords: formData.keywords,
      groupId: formData.groupId,
      proxyId: resolvedProxyId,
      proxyConfig: resolvedProxyConfig,
      memoryLimitMb: Math.max(0, Math.floor(Number(formData.memoryLimitMb) || 0)),
      launchArgs: normalizeLaunchArgs(launchArgsText.split('\n')),
    }
    const fingerprintValidation = validateFingerprintArgs(payload.fingerprintArgs)
    if (!fingerprintValidation.valid) {
      setSaveError(fingerprintValidation.issues.filter(issue => issue.level === 'error').map(issue => issue.message).join('\n'))
      return
    }
    if (proxyMode === 'pool' && !resolvedProxyId) {
      payload.proxyId = directProxyID
      payload.proxyConfig = ''
    }

    setSaving(true)
    try {
      const validation = await validateProxyConfig(payload.proxyConfig, payload.proxyId)
      if (!validation.supported) {
        setSaveError(validation.errorMsg || '代理配置无效')
        return
      }
      if (isCreate) {
        await createBrowserProfile(payload)
        toast.success('配置已创建')
      } else if (id) {
        await updateBrowserProfile(id, payload)
        toast.success('配置已更新')
      }
      setIsDirty(false)
      navigate('/browser/list')
    } catch (error: any) {
      setSaveError(typeof error === 'string' ? error : error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleBack = () => {
    if (isDirty) { setLeaveConfirm(true) } else { navigate('/browser/list') }
  }

  const handleApplyProxyLocation = async () => {
    if (proxyMode !== 'pool' || !formData.proxyId || formData.proxyId === directProxyID) {
      toast.error('请选择代理池中的非直连节点')
      return
    }
    setLocationResolving(true)
    setLocationResult(null)
    try {
      const result = await browserProxyResolveLocation(formData.proxyId)
      setLocationResult(result)
      if (!result.ok || !result.lang || !result.timezone) {
        toast.error(result.error || '无法根据代理 IP 匹配定位')
        return
      }
      const nextArgs = applyLocaleToFingerprintArgs(formData.fingerprintArgs, result.lang, result.timezone)
      handleChange('fingerprintArgs', nextArgs)
      toast.success(`已设置 ${result.lang} / ${result.timezone}`)
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '代理定位失败')
    } finally {
      setLocationResolving(false)
    }
  }

  const handleValidateFingerprint = async () => {
    if (isCreate || !id) return
    setFpBusy(true)
    try {
      setFpValidation(await validateBrowserProfileFingerprint(id))
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '校验失败')
    } finally {
      setFpBusy(false)
    }
  }

  const handleRegenerateFingerprint = async () => {
    if (isCreate || !id) {
      toast.error('请先保存实例后再重新生成身份')
      return
    }
    setFpBusy(true)
    try {
      const profile = await regenerateBrowserProfileFingerprint(id)
      if (profile?.fingerprintArgs) {
        handleChange('fingerprintArgs', profile.fingerprintArgs)
      }
      setFpValidation(await validateBrowserProfileFingerprint(id))
      toast.success('已重新生成唯一自洽指纹身份')
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '重新生成失败')
    } finally {
      setFpBusy(false)
    }
  }

  const handleFingerprintCheck = async () => {
    if (isCreate || !id) {
      toast.warning('请先保存实例，再启动后自测')
      return
    }
    if (isDirty) {
      toast.warning('当前有未保存修改，请先保存后再自测')
      return
    }
    setFingerprintChecking(true)
    try {
      const result = await checkBrowserProfileFingerprint(id)
      setFingerprintCheckResult(result)
      setFingerprintCheckOpen(true)
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '指纹自测失败')
    } finally {
      setFingerprintChecking(false)
    }
  }

  const handleOpenFingerprintPage = async () => {
    if (isCreate || !id) {
      toast.warning('请先保存实例，再打开检测页')
      return
    }
    if (isDirty) {
      toast.warning('当前有未保存修改，请先保存后再检测')
      return
    }
    setFingerprintPageOpening(true)
    try {
      const profile = await openBrowserFingerprintCheck(id)
      if (profile?.lastLaunchArgs) {
        setFormData(prev => ({ ...prev, lastLaunchArgs: profile.lastLaunchArgs || [] }))
      }
      toast.success('已在目标浏览器打开指纹检测页')
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '打开指纹检测页失败')
    } finally {
      setFingerprintPageOpening(false)
    }
  }

  const defaultCore = cores.find(c => c.isDefault)
  const selectedPoolProxy = proxies.find((proxy) => proxy.proxyId === formData.proxyId)

  const handleOpenUserDataDir = async () => {
    if (!formData.userDataDir.trim()) {
      toast.error('请先输入用户数据目录')
      return
    }
    try {
      await openUserDataDir(formData.userDataDir)
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '打开目录失败')
    }
  }

  const handleProxyListUpdated = (nextProxies: BrowserProxy[]) => {
    setProxies(nextProxies)
  }

  const handleProxyDeleted = (deletedProxyId: string, nextProxies: BrowserProxy[]) => {
    setProxies(nextProxies)
    if (formData.proxyId !== deletedProxyId) {
      return
    }

    const fallbackProxy = nextProxies.find((proxy) => proxy.proxyId === directProxyID)
    if (fallbackProxy) {
      handleChange('proxyId', fallbackProxy.proxyId)
      return
    }

    handleChange('proxyId', '')
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{isCreate ? '新建配置' : '编辑配置'}</h1>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleBack}>返回列表</Button>
          <Button size="sm" onClick={handleSave} loading={saving}>保存配置</Button>
        </div>
      </div>

      <Card title="基础配置">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="配置名称" required>
            <Input value={formData.profileName} onChange={e => handleChange('profileName', e.target.value)} placeholder="请输入配置名称" />
          </FormItem>
          <FormItem label="用户数据目录（留空自动生成）">
            <div className="flex gap-2">
              <Input
                value={formData.userDataDir}
                onChange={e => handleChange('userDataDir', e.target.value)}
                placeholder="留空自动生成"
                className="flex-1"
              />
              <Button variant="secondary" size="sm" onClick={handleOpenUserDataDir} title="在资源管理器中打开">
                <FolderOpen className="w-4 h-4" />
              </Button>
            </div>
          </FormItem>
          <FormItem label="内核">
            <Select
              value={formData.coreId}
              onChange={e => handleChange('coreId', e.target.value)}
              options={
                cores.length > 0 ? [
                  { value: '', label: defaultCore ? `使用默认 (${defaultCore.coreName})` : '使用默认内核' },
                  ...cores.map(c => ({ value: c.coreId, label: c.coreName })),
                ] : [
                  { value: '', label: '暂无内核，请添加内核' }
                ]
              }
            />
          </FormItem>
          <FormItem label="历史标签">
            <Select
              value={formData.restoreLastSession || ''}
              onChange={e => handleChange('restoreLastSession', e.target.value)}
              options={RESTORE_LAST_SESSION_OPTIONS}
            />
          </FormItem>
          <FormItem label="最大内存 MB">
            <Input
              type="number"
              min="0"
              step="128"
              value={String(formData.memoryLimitMb || 0)}
              onChange={e => handleChange('memoryLimitMb', Math.max(0, Math.floor(Number(e.target.value) || 0)))}
              placeholder="0 表示不限制"
            />
          </FormItem>
          <FormItem label="标签">
            <TagInput
              value={formData.tags}
              onChange={tags => handleChange('tags', tags)}
              suggestions={allTags}
              placeholder="输入标签后按回车，支持从已有标签选择"
            />
          </FormItem>
          <FormItem label="分组">
            <GroupSelector
              groups={groups}
              value={formData.groupId || ''}
              onChange={groupId => handleChange('groupId', groupId)}
              placeholder="未分组"
              className="w-full"
            />
          </FormItem>
        </div>
      </Card>

      <Card title="代理与定位">
        <div className="grid grid-cols-1 lg:grid-cols-[220px_minmax(0,1fr)] gap-4 items-start">
          <FormItem label="代理来源">
            <Select
              value={proxyMode}
              onChange={e => handleProxyModeChange(e.target.value as ProxySourceMode)}
              options={[
                { value: 'pool', label: '代理池' },
                { value: 'local', label: '本地代理' },
              ]}
            />
          </FormItem>
          {proxyMode === 'pool' ? (
            <FormItem label="代理地址选择">
              <div className="flex flex-col sm:flex-row gap-2">
                <Select
                  value={formData.proxyId}
                  onChange={e => { handleChange('proxyId', e.target.value); setLocationResult(null) }}
                  options={
                    proxies.length > 0
                      ? proxies.map(p => ({ value: p.proxyId, label: p.proxyName || p.proxyId }))
                      : [{ value: '', label: '暂无代理，请先到代理池创建' }]
                  }
                  className="flex-1 min-w-0"
                />
                <Button variant="secondary" size="sm" className="shrink-0" onClick={() => setProxyPickerOpen(true)} title="按分组选择代理">
                  <Layers className="w-4 h-4" />
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  className="shrink-0"
                  onClick={handleApplyProxyLocation}
                  loading={locationResolving}
                  disabled={!formData.proxyId || formData.proxyId === directProxyID}
                >
                  按代理匹配定位
                </Button>
              </div>
              {locationResult && (
                <div className="mt-2 text-xs text-[var(--color-text-muted)]">
                  {locationResult.ok
                    ? `出口 ${locationResult.ip || '-'} · ${[locationResult.country, locationResult.region, locationResult.city].filter(Boolean).join(' / ') || '-'} · ${locationResult.lang} · ${locationResult.timezone}`
                    : locationResult.error || '未匹配到定位'}
                </div>
              )}
            </FormItem>
          ) : (
            <FormItem label="本地代理地址" hint="支持 http://、https://、socks5://">
              <Input
                value={formData.proxyConfig}
                onChange={e => handleChange('proxyConfig', e.target.value)}
                placeholder="http://127.0.0.1:7890"
              />
            </FormItem>
          )}
        </div>
        <p className="text-xs text-[var(--color-text-muted)] mt-2">
          {proxyMode === 'pool'
            ? `当前使用代理池节点${selectedPoolProxy?.proxyName ? `：${selectedPoolProxy.proxyName}` : '。'}`
            : '本地代理不会进入代理池，只对当前实例保存生效。'}
        </p>
      </Card>

      <ProxyPickerModal
        open={proxyPickerOpen}
        currentProxyId={formData.proxyId}
        onSelect={proxy => { handleChange('proxyId', proxy.proxyId); setLocationResult(null) }}
        onProxyListUpdated={handleProxyListUpdated}
        onProxyDeleted={handleProxyDeleted}
        onClose={() => setProxyPickerOpen(false)}
      />

      <Card
        title="指纹配置"
        actions={(
          <>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setFingerprintMatrixOpen(true)}
              title="查看版本适配矩阵"
              aria-label="查看版本适配矩阵"
            >
              <HelpCircle className="w-4 h-4" />
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleOpenFingerprintPage}
              loading={fingerprintPageOpening}
              disabled={isCreate}
            >
              浏览器内检测
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleFingerprintCheck}
              loading={fingerprintChecking}
              disabled={isCreate}
            >
              <ShieldCheck className="w-4 h-4" />
              自测当前实例
            </Button>
          </>
        )}
      >
        {!isCreate && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12, flexWrap: 'wrap' }}>
            <Button onClick={handleRegenerateFingerprint} disabled={fpBusy}>重新生成唯一身份</Button>
            <Button onClick={handleValidateFingerprint} disabled={fpBusy}>校验一致性</Button>
            {fpValidation && (
              <span
                style={{
                  fontSize: 12,
                  padding: '2px 10px',
                  borderRadius: 12,
                  background: fpValidation.ok ? 'rgba(34,197,94,0.15)' : 'rgba(234,179,8,0.15)',
                  color: fpValidation.ok ? '#16a34a' : '#b45309',
                }}
              >
                {fpValidation.ok
                  ? '✓ 指纹自洽'
                  : `⚠ ${fpValidation.issues.length} 项：${fpValidation.issues.map(i => i.message).join('；')}`}
              </span>
            )}
          </div>
        )}
        <FingerprintPanel
          value={formData.fingerprintArgs}
          onChange={args => handleChange('fingerprintArgs', args)}
        />
      </Card>

      <Card padding="none">
        <button
          type="button"
          className="w-full flex items-center justify-between px-5 py-4 text-left hover:bg-[var(--color-bg-hover)] transition-colors"
          onClick={() => setLaunchArgsOpen(current => !current)}
        >
          <span className="inline-flex items-center gap-2 text-sm font-semibold text-[var(--color-text-primary)]">
            <span>高级启动参数</span>
            <span
              role="button"
              tabIndex={0}
              className="inline-flex h-5 w-5 items-center justify-center rounded-full text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
              onClick={event => {
                event.stopPropagation()
                setLaunchArgsHelpOpen(true)
              }}
              onKeyDown={event => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  event.stopPropagation()
                  setLaunchArgsHelpOpen(true)
                }
              }}
              aria-label="查看高级启动参数说明"
            >
              <HelpCircle className="h-4 w-4" />
            </span>
          </span>
          {launchArgsOpen ? <ChevronUp className="w-4 h-4 text-[var(--color-text-muted)]" /> : <ChevronDown className="w-4 h-4 text-[var(--color-text-muted)]" />}
        </button>
        {launchArgsOpen && (
          <div className="space-y-3 px-5 pb-5 border-t border-[var(--color-border-muted)] pt-4">
            <Textarea
              value={launchArgsText}
              onChange={e => { setLaunchArgsText(e.target.value); setIsDirty(true) }}
              rows={6}
              placeholder="--disable-sync"
            />
            {!isCreate && formData.lastLaunchArgs && formData.lastLaunchArgs.length > 0 && (
              <div className="rounded-lg border border-[var(--color-border)] overflow-hidden">
                <div className="px-3 py-2 text-xs font-medium text-[var(--color-text-muted)] border-b border-[var(--color-border)]">上次实际启动参数</div>
                <Textarea
                  value={formData.lastLaunchArgs.join('\n')}
                  readOnly
                  rows={6}
                  className="border-0 rounded-none bg-[var(--color-bg-hover)] font-mono text-xs"
                />
              </div>
            )}
          </div>
        )}
      </Card>

      <Modal
        open={launchArgsHelpOpen}
        onClose={() => setLaunchArgsHelpOpen(false)}
        title="高级启动参数说明"
        width="920px"
        footer={<Button variant="secondary" onClick={() => setLaunchArgsHelpOpen(false)}>关闭</Button>}
      >
        <div className="rounded-lg border border-[var(--color-border)] overflow-hidden">
          <div className="grid grid-cols-[220px_minmax(0,1fr)_minmax(180px,0.7fr)] gap-3 px-3 py-2 text-xs font-medium text-[var(--color-text-muted)] border-b border-[var(--color-border)] bg-[var(--color-bg-hover)]">
            <div>参数</div>
            <div>含义</div>
            <div>示例</div>
          </div>
          {[
            ['--disable-sync', '关闭 Chrome 同步，减少账号和同步服务干扰。', '--disable-sync'],
            ['--no-first-run', '跳过首次运行向导，启动更干净。', '--no-first-run'],
            ['--start-maximized', '启动后最大化窗口；会影响窗口大小检测口径。', '--start-maximized'],
            ['--disable-background-networking', '减少后台网络请求；可能影响部分 Chrome 服务。', '--disable-background-networking'],
            ['--disable-features=<list>', '关闭指定 Chromium Feature；仅在明确知道影响时使用。', '--disable-features=Translate'],
            ['--enable-features=<list>', '开启指定 Chromium Feature；仅在明确知道影响时使用。', '--enable-features=NetworkService'],
            ['不建议手填', '指纹、代理、用户数据目录、调试端口属于托管参数，会由页面配置和后端启动流程生成。', '--user-data-dir / --proxy-server / --remote-debugging-port'],
            ['上次实际启动参数', '只读记录，展示最终启动 Chrome 的完整参数，可用于排查后端自动补齐内容。', '下方只读区域'],
          ].map(([arg, meaning, example]) => (
            <div key={arg} className="grid grid-cols-[220px_minmax(0,1fr)_minmax(180px,0.7fr)] gap-3 px-3 py-2 text-xs border-b last:border-b-0 border-[var(--color-border)]">
              <code className="break-all text-[var(--color-text-primary)]">{arg}</code>
              <div className="text-[var(--color-text-secondary)]">{meaning}</div>
              <code className="break-all text-[var(--color-text-muted)]">{example}</code>
            </div>
          ))}
        </div>
      </Modal>

      <ConfirmModal
        open={leaveConfirm}
        onClose={() => setLeaveConfirm(false)}
        onConfirm={() => navigate('/browser/list')}
        title="放弃未保存的更改？"
        content="当前页面有未保存的修改，离开后将丢失这些更改。"
        confirmText="放弃并离开"
        cancelText="继续编辑"
        danger
      />

      <Modal
        open={fingerprintCheckOpen}
        onClose={() => setFingerprintCheckOpen(false)}
        title="指纹自测结果"
        width="860px"
      >
        {fingerprintCheckResult ? (
          <div className="space-y-4">
            <div className="grid grid-cols-[120px_minmax(0,1fr)_minmax(0,1fr)_70px] gap-3 text-xs font-medium text-[var(--color-text-muted)] border-b border-[var(--color-border)] pb-2">
              <div>项目</div>
              <div>配置值</div>
              <div>实际值</div>
              <div>状态</div>
            </div>
            <div>
              <FingerprintCheckRow label="语言" expected={fingerprintCheckResult.expected.language} actual={fingerprintCheckResult.runtime.language} />
              <FingerprintCheckRow label="语言列表" expected={fingerprintCheckResult.expected.acceptLanguage} actual={fingerprintCheckResult.runtime.languages} />
              <FingerprintCheckRow label="时区" expected={fingerprintCheckResult.expected.timezone} actual={fingerprintCheckResult.runtime.timezone} />
              <FingerprintCheckRow label="CPU 核心" expected={fingerprintCheckResult.expected.hardwareConcurrency} actual={fingerprintCheckResult.runtime.hardwareConcurrency} />
              <FingerprintDisplayRow label="设备内存" actual={fingerprintCheckResult.runtime.deviceMemory} />
              <FingerprintDisplayRow label="颜色深度" actual={fingerprintCheckResult.runtime.colorDepth} />
              <FingerprintDisplayRow label="触控点" actual={fingerprintCheckResult.runtime.maxTouchPoints} />
              <FingerprintDisplayRow label="Do Not Track" actual={fingerprintCheckResult.runtime.doNotTrack} />
              <FingerprintCheckRow label="窗口大小" expected={fingerprintCheckResult.expected.windowSize} actual={`${fingerprintCheckResult.runtime.outerWidth},${fingerprintCheckResult.runtime.outerHeight}`} />
              <FingerprintCheckRow label="平台" expected={fingerprintCheckResult.expected.platform} actual={fingerprintCheckResult.runtime.platform} mode="platform" />
              <FingerprintCheckRow label="品牌版本" expected={fingerprintCheckResult.expected.brandVersion} actual={fingerprintCheckResult.runtime.userAgent} mode="browser-version" />
              <FingerprintCheckRow label="系统版本" expected={fingerprintCheckResult.expected.platformVersion} actual={fingerprintCheckResult.runtime.userAgent} mode="platform-version" />
              <FingerprintCheckRow label="UA" expected={fingerprintCheckResult.expected.brand} actual={fingerprintCheckResult.runtime.userAgent} mode="contains" />
              <FingerprintDisplayRow label="UA Data" actual={fingerprintCheckResult.runtime.userAgentData} />
              <FingerprintCheckRow label="Webdriver" expected={false} actual={fingerprintCheckResult.runtime.webdriver} />
              <FingerprintDisplayRow label="屏幕" actual={`${fingerprintCheckResult.runtime.screenWidth}x${fingerprintCheckResult.runtime.screenHeight} depth ${fingerprintCheckResult.runtime.colorDepth} @ ${fingerprintCheckResult.runtime.devicePixelRatio}`} />
              <FingerprintDisplayRow label="WebGL Vendor" actual={fingerprintCheckResult.runtime.webglVendor} />
              <FingerprintDisplayRow label="WebGL Renderer" actual={fingerprintCheckResult.runtime.webglRenderer} />
              <FingerprintDisplayRow label="Canvas Hash" actual={fingerprintCheckResult.runtime.canvasHash} />
              <FingerprintDisplayRow label="Audio Hash" actual={fingerprintCheckResult.runtime.audioHash} />
              <FingerprintDisplayRow label="ClientRects Hash" actual={fingerprintCheckResult.runtime.clientRectsHash} />
              <FingerprintDisplayRow label="媒体设备数量" actual={fingerprintCheckResult.runtime.mediaDeviceCount} />
              <FingerprintDisplayRow label="Canvas 噪声" actual={fingerprintCheckResult.expected.canvasNoise ? '已配置' : '未配置'} />
              <FingerprintDisplayRow label="Audio 噪声" actual="不作为期望" />
              <FingerprintDisplayRow label="ClientRects 噪声" actual={fingerprintCheckResult.expected.clientRectsNoise ? '已配置' : '未配置'} />
              <FingerprintDisplayRow label="字体列表" actual="不作为期望" />
              <FingerprintDisplayRow label="插件" actual={fingerprintCheckResult.runtime.plugins} />
              <FingerprintDisplayRow label="种子" actual={fingerprintCheckResult.expected.seed || '未显式设置'} />
              <FingerprintDisplayRow label="排除伪装" actual={fingerprintCheckResult.expected.disableSpoofing || '无'} />
              <FingerprintDisplayRow label="WebRTC" actual={fingerprintCheckResult.expected.webrtcPolicy || '未显式设置'} />
            </div>
          </div>
        ) : (
          <div className="text-sm text-[var(--color-text-muted)]">暂无自测结果</div>
        )}
      </Modal>

      <Modal
        open={fingerprintMatrixOpen}
        onClose={() => setFingerprintMatrixOpen(false)}
        title="版本适配矩阵"
        width="980px"
        footer={<Button variant="secondary" onClick={() => setFingerprintMatrixOpen(false)}>关闭</Button>}
      >
        <FingerprintMatrixReport report={fingerprintMatrix} />
      </Modal>

      <Modal
        open={!!saveError}
        onClose={() => setSaveError('')}
        title="保存失败"
        width="420px"
        footer={<Button onClick={() => setSaveError('')}>知道了</Button>}
      >
        <div className="text-[var(--color-text-secondary)]">{saveError}</div>
      </Modal>
    </div>
  )
}
