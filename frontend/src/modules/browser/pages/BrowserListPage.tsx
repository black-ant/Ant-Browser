import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { CheckCircle, Edit2, ExternalLink, FileText, Plus, Sliders, Star, Trash2, XCircle, Gift } from 'lucide-react'
import { Button, Card, FormItem, Input, Modal, Table, Textarea, toast, useConfirm } from '../../../shared/components'
import { fetchDashboardStats, redeemCDKey, redeemGithubStar, reloadConfig } from '../../dashboard/api'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserCore, BrowserCoreInput, BrowserProfile, BrowserProxy, BrowserSettings, BrowserGroupWithCount } from '../types'
import { InstanceFilterBar, EMPTY_FILTERS } from '../components/InstanceFilterBar'
import type { InstanceFilters } from '../components/InstanceFilterBar'
import { KeywordsModal } from '../components/KeywordsModal'
import { EventsOn, BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { PROJECT_GITHUB_URL } from '../../../config/links'
import { resolveActionErrorMessage, resolveActionFeedback } from '../utils/actionErrors'
import { runWithConcurrency } from '../utils/concurrency'
import { resolveProfileStatus } from './browser-list/helpers'
import { ProfileCard } from './browser-list/ProfileCard'
import { StatusOverview } from './browser-list/StatusOverview'
import { EnhancedToolbar } from './browser-list/EnhancedToolbar'
import { EnhancedBatchToolbar } from './browser-list/EnhancedBatchToolbar'
import { createEnhancedColumns } from './browser-list/enhancedColumns'
import {
  copyBrowserProfile,
  deleteBrowserCore,
  deleteBrowserProfile,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchGroups,
  restartBrowserInstance,
  saveBrowserCore,
  saveBrowserSettings,
  setDefaultBrowserCore,
  startBrowserInstance,
  stopBrowserInstance,
  validateBrowserCorePath,
  validateProxyConfig,
} from '../api'

// 批量操作的前端并发上限。真正的启动并发由后端启动队列限流；
// 这里只是避免一次性向后端发起过多 RPC 调用。
const BATCH_OP_CONCURRENCY = 8

export function BrowserListPage() {
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])

  // 视图模式
  const [viewMode, setViewMode] = useState<'card' | 'table'>(() => {
    return (localStorage.getItem('browser:viewMode') as 'card' | 'table') || 'table'
  })

  // 勾选状态
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)

  // 筛选状态（从 localStorage 恢复）
  const [filters, setFilters] = useState<InstanceFilters>(() => {
    try {
      const saved = localStorage.getItem('browser:filters')
      if (saved) {
        const parsed = JSON.parse(saved)
        return { ...EMPTY_FILTERS, ...parsed, tags: new Set(parsed.tags || []) }
      }
    } catch { /* ignore */ }
    return EMPTY_FILTERS
  })

  // 新增：搜索和筛选面板状态
  const [searchQuery, setSearchQuery] = useState('')
  const [filterPanelOpen, setFilterPanelOpen] = useState(false)
  const [refreshing, setRefreshing] = useState(false)

  // 持久化筛选状态
  useEffect(() => {
    const serializable = { ...filters, tags: Array.from(filters.tags) }
    localStorage.setItem('browser:filters', JSON.stringify(serializable))
  }, [filters])

  useEffect(() => {
    localStorage.setItem('browser:viewMode', viewMode)
  }, [viewMode])

  // 代理不支持弹窗
  const [proxyErrorModal, setProxyErrorModal] = useState(false)
  const [proxyErrorMsg, setProxyErrorMsg] = useState('')
  const [opError, setOpError] = useState('')
  const [pendingStartId, setPendingStartId] = useState<string | null>(null)
  const [startingIds, setStartingIds] = useState<Set<string>>(new Set())
  const [stoppingIds, setStoppingIds] = useState<Set<string>>(new Set())
  const profilesRef = useRef<BrowserProfile[]>([])
  const silentRefreshInFlightRef = useRef(false)

  // 关键字弹窗
  const [kwModal, setKwModal] = useState<{ open: boolean; profile: BrowserProfile | null }>({ open: false, profile: null })

  const openKwModal = (profile: BrowserProfile) => setKwModal({ open: true, profile })
  const closeKwModal = () => setKwModal({ open: false, profile: null })

  // 复制弹窗
  const [copyModal, setCopyModal] = useState<{ open: boolean; profile: BrowserProfile | null }>({ open: false, profile: null })
  const [copyName, setCopyName] = useState('')
  const [copying, setCopying] = useState(false)

  const openCopyModal = (profile: BrowserProfile) => {
    setCopyName(profile.profileName + ' (副本)')
    setCopyModal({ open: true, profile })
  }
  const closeCopyModal = () => {
    setCopyModal({ open: false, profile: null })
    setCopyName('')
  }

  // 基础配置弹窗
  const [settingsModalOpen, setSettingsModalOpen] = useState(false)
  const [settings, setSettings] = useState<BrowserSettings>({
    userDataRoot: 'data',
    defaultFingerprintArgs: [],
    defaultLaunchArgs: [],
    defaultProxy: '',
    startReadyTimeoutMs: 3000,
    startStableWindowMs: 1200,
  })
  const [fingerprintText, setFingerprintText] = useState('')
  const [launchText, setLaunchText] = useState('')
  const [savingSettings, setSavingSettings] = useState(false)

  // 内核管理
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [coreModalOpen, setCoreModalOpen] = useState(false)
  const [coreForm, setCoreForm] = useState<BrowserCoreInput>({ coreId: '', coreName: '', corePath: '', isDefault: false })
  const [coreValidation, setCoreValidation] = useState<{ valid: boolean; message: string } | null>(null)
  const [savingCore, setSavingCore] = useState(false)

  // 扩容管理
  const [expandModalOpen, setExpandModalOpen] = useState(false)
  const [cdKey, setCdKey] = useState('')
  const [redeeming, setRedeeming] = useState(false)
  const [maxProfileLimit, setMaxProfileLimit] = useState(100)

  const updatePendingIds = (
    setter: React.Dispatch<React.SetStateAction<Set<string>>>,
    profileId: string,
    active: boolean
  ) => {
    setter(prev => {
      const next = new Set(prev)
      if (active) {
        next.add(profileId)
      } else {
        next.delete(profileId)
      }
      return next
    })
  }

  const replaceProfilesState = (items: BrowserProfile[]) => {
    profilesRef.current = items
    setProfiles(items)
  }

  const updateProfilesState = (updater: (items: BrowserProfile[]) => BrowserProfile[]) => {
    const next = updater(profilesRef.current)
    profilesRef.current = next
    setProfiles(next)
  }

  const mergeProfileState = (profile: BrowserProfile | null | undefined) => {
    if (!profile) return
    updateProfilesState(prev => prev.map(item => (
      item.profileId === profile.profileId ? { ...item, ...profile } : item
    )))
  }

  // patchProfileRuntime 按 profileId 增量合并部分运行时字段（用于事件驱动更新，避免全量拉取）
  const patchProfileRuntime = (profileId: string, patch: Partial<BrowserProfile>) => {
    if (!profileId) return
    updateProfilesState(prev => prev.map(item => (
      item.profileId === profileId ? { ...item, ...patch } : item
    )))
  }

  // runtimePatchFromEvent 从生命周期事件载荷提取已知运行时字段
  const runtimePatchFromEvent = (payload: any): Partial<BrowserProfile> => {
    const patch: Partial<BrowserProfile> = {}
    if (typeof payload?.running === 'boolean') patch.running = payload.running
    if (typeof payload?.status === 'string') patch.status = payload.status
    if (typeof payload?.debugReady === 'boolean') patch.debugReady = payload.debugReady
    if (typeof payload?.debugPort === 'number') patch.debugPort = payload.debugPort
    if (typeof payload?.pid === 'number') patch.pid = payload.pid
    if (typeof payload?.runtimeWarning === 'string') patch.runtimeWarning = payload.runtimeWarning
    return patch
  }

  const syncProfiles = (items: BrowserProfile[], syncRuntimeState: boolean) => {
    if (syncRuntimeState) {
      const previousById = new Map(profilesRef.current.map(item => [item.profileId, item]))
      const newlyRunning = items.find(item => item.running && !previousById.get(item.profileId)?.running)
      if (newlyRunning) {
        updatePendingIds(setStartingIds, newlyRunning.profileId, false)
        updatePendingIds(setStoppingIds, newlyRunning.profileId, false)
      }
      items.forEach(item => {
        if (!item.running && previousById.get(item.profileId)?.running) {
          updatePendingIds(setStartingIds, item.profileId, false)
          updatePendingIds(setStoppingIds, item.profileId, false)
        }
      })
    }
    replaceProfilesState(items)
  }

  const loadProfiles = async ({ silent = false, syncRuntimeState = false }: { silent?: boolean; syncRuntimeState?: boolean } = {}) => {
    if (silent && silentRefreshInFlightRef.current) {
      return profilesRef.current
    }
    if (!silent) {
      setLoading(true)
    } else {
      silentRefreshInFlightRef.current = true
    }
    try {
      const items = await fetchBrowserProfiles()
      syncProfiles(items, syncRuntimeState)
      return items
    } finally {
      if (silent) {
        silentRefreshInFlightRef.current = false
      } else {
        setLoading(false)
      }
    }
  }

  const loadGroups = async () => {
    setGroups(await fetchGroups())
  }

  const loadSettings = async () => {
    const data = await fetchBrowserSettings()
    setSettings(data)
    setFingerprintText((data.defaultFingerprintArgs || []).join('\n'))
    setLaunchText((data.defaultLaunchArgs || []).join('\n'))
  }

  const loadCores = async () => {
    setCores(await fetchBrowserCores())
  }

  const loadQuota = async () => {
    try {
      await reloadConfig()
      const stats = await fetchDashboardStats()
      setMaxProfileLimit(stats.maxProfileLimit || 100)
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    void loadProfiles()
    loadGroups()
    loadQuota()
    fetchBrowserProxies().then(setProxies)
    fetchBrowserCores().then(setCores)

    // 监听浏览器实例生命周期事件，按事件载荷增量更新单个实例状态（不再全量拉取）
    const offStarted = EventsOn('browser:instance:started', (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (!profileId) return
      updatePendingIds(setStartingIds, profileId, false)
      updatePendingIds(setStoppingIds, profileId, false)
      patchProfileRuntime(profileId, runtimePatchFromEvent(payload))
    })
    const offUpdated = EventsOn('browser:instance:updated', (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (!profileId) return
      patchProfileRuntime(profileId, runtimePatchFromEvent(payload))
    })
    const offStopped = EventsOn('browser:instance:stopped', (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (!profileId) return
      updatePendingIds(setStartingIds, profileId, false)
      updatePendingIds(setStoppingIds, profileId, false)
      patchProfileRuntime(profileId, { running: false, status: 'stopped', debugReady: false, debugPort: 0, pid: 0, runtimeWarning: '' })
    })
    const offCrashed = EventsOn('browser:instance:crashed', (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (!profileId) return
      updatePendingIds(setStartingIds, profileId, false)
      updatePendingIds(setStoppingIds, profileId, false)
      const lastError = typeof payload?.error === 'string' ? payload.error : undefined
      patchProfileRuntime(profileId, { running: false, status: 'crashed', debugReady: false, debugPort: 0, pid: 0, ...(lastError ? { lastError } : {}) })
    })

    // 兜底对账轮询：事件驱动为主，轮询频率放宽到 5s，仅用于补偿漏掉的事件与外部变更
    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void loadProfiles({ silent: true, syncRuntimeState: true })
    }, 5000)

    return () => {
      window.clearInterval(timer)
      offStarted?.()
      offUpdated?.()
      offStopped?.()
      offCrashed?.()
    }
  }, [])

  const runningCount = useMemo(() => profiles.filter(p => p.running).length, [profiles])

  // 状态统计
  const statusStats = useMemo(() => {
    const starting = Array.from(startingIds).length
    const error = profiles.filter(p => p.lastError).length
    const proxyError = profiles.filter(p => p.runtimeWarning && p.runtimeWarning.includes('代理')).length

    return {
      total: profiles.length,
      running: runningCount,
      starting,
      error,
      proxyError,
    }
  }, [profiles, runningCount, startingIds])

  const allTags = useMemo(() => {
    const set = new Set<string>()
    profiles.forEach(p => p.tags?.forEach(t => set.add(t)))
    return Array.from(set).sort()
  }, [profiles])

  const defaultCore = useMemo(() => {
    return cores.find(core => core.isDefault) || cores[0] || null
  }, [cores])

  const resolveProfileCore = (profile: BrowserProfile) => {
    const coreId = (profile.coreId || '').trim()
    if (coreId && !/^default$/i.test(coreId)) {
      return cores.find(core => core.coreId === coreId) || null
    }
    return defaultCore
  }

  const getProfileCoreLabel = (profile: BrowserProfile) => {
    const resolvedCore = resolveProfileCore(profile)
    if (resolvedCore) {
      return resolvedCore.coreName
    }

    const coreId = (profile.coreId || '').trim()
    if (!coreId || /^default$/i.test(coreId)) {
      return '使用默认内核'
    }
    return coreId
  }

  const isProfileStarting = (profileId: string) => startingIds.has(profileId)
  const isProfileStopping = (profileId: string) => stoppingIds.has(profileId)
  const isProfileBusy = (profileId: string) => isProfileStarting(profileId) || isProfileStopping(profileId)

  const getProfileStatus = (profile: BrowserProfile) => (
    resolveProfileStatus(profile.running, profile.debugReady, isProfileStarting(profile.profileId), isProfileStopping(profile.profileId), profile.status)
  )

  const filteredProfiles = useMemo(() => {
    const naturalCompare = (a: string, b: string): number => {
      const re = /(\d+)|(\D+)/g
      const partsA = a.match(re) || []
      const partsB = b.match(re) || []
      for (let i = 0; i < Math.max(partsA.length, partsB.length); i++) {
        if (i >= partsA.length) return -1
        if (i >= partsB.length) return 1
        const pa = partsA[i], pb = partsB[i]
        const na = Number(pa), nb = Number(pb)
        if (!isNaN(na) && !isNaN(nb)) {
          if (na !== nb) return na - nb
        } else {
          const cmp = pa.localeCompare(pb, 'zh-CN')
          if (cmp !== 0) return cmp
        }
      }
      return 0
    }

    let result = profiles

    // 搜索过滤
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      result = result.filter(p =>
        p.profileName.toLowerCase().includes(query) ||
        p.tags?.some(tag => tag.toLowerCase().includes(query)) ||
        p.keywords?.some(kw => kw.toLowerCase().includes(query))
      )
    }

    // 原有筛选逻辑
    result = result.filter(p => {
      // 分组筛选
      if (filters.groupId === '__ungrouped__' && p.groupId) return false
      if (filters.groupId && filters.groupId !== '__ungrouped__' && p.groupId !== filters.groupId) return false

      if (filters.keyword && !p.profileName.toLowerCase().includes(filters.keyword.toLowerCase())) return false
      if (filters.status === 'running' && !p.running) return false
      if (filters.status === 'stopped' && p.running) return false
      if (filters.proxyId === '__none__' && (p.proxyId || p.proxyConfig)) return false
      if (filters.proxyId && filters.proxyId !== '__none__' && p.proxyId !== filters.proxyId) return false
      if (filters.coreId) {
        const effectiveCore = resolveProfileCore(p)
        if (!effectiveCore || effectiveCore.coreId !== filters.coreId) return false
      }
      if (filters.tags.size > 0 && !p.tags?.some(t => filters.tags.has(t))) return false
      if (filters.kwSearch) {
        const q = filters.kwSearch.toLowerCase()
        const hit = p.keywords?.some(v => v.toLowerCase().includes(q))
        if (!hit) return false
      }
      return true
    })

    return result.sort((a, b) => naturalCompare(a.profileName, b.profileName))
  }, [profiles, searchQuery, filters, defaultCore, cores])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await loadProfiles()
    } finally {
      setRefreshing(false)
    }
  }

  const handleStart = async (profileId: string) => {
    const profile = profiles.find(p => p.profileId === profileId)
    updatePendingIds(setStartingIds, profileId, true)
    try {
      if (profile) {
        const result = await validateProxyConfig(profile.proxyConfig || '', profile.proxyId || '')
        if (!result.supported) {
          setProxyErrorMsg(result.errorMsg)
          setPendingStartId(profileId)
          setProxyErrorModal(true)
          return
        }
      }

      const startedProfile = await startBrowserInstance(profileId)
      mergeProfileState(startedProfile)
      if (startedProfile?.running && !startedProfile.debugReady && startedProfile.runtimeWarning) {
        toast.warning(startedProfile.runtimeWarning)
      } else {
        toast.success(`实例已启动${startedProfile?.profileName ? `：${startedProfile.profileName}` : ''}`)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '实例启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStartingIds, profileId, false)
    }
  }

  const handleStop = async (profileId: string) => {
    updatePendingIds(setStoppingIds, profileId, true)
    try {
      const stoppedProfile = await stopBrowserInstance(profileId)
      mergeProfileState(stoppedProfile)
      toast.success('实例已停止')
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      toast.error(resolveActionErrorMessage(error, '实例停止失败'))
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStoppingIds, profileId, false)
    }
  }

  const handleRestart = async (profileId: string) => {
    updatePendingIds(setStoppingIds, profileId, true)
    try {
      const restartedProfile = await restartBrowserInstance(profileId)
      mergeProfileState(restartedProfile)
      toast.success(`实例已重启${restartedProfile?.profileName ? `：${restartedProfile.profileName}` : ''}`)
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '实例重启失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        setOpError(feedback.message)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStoppingIds, profileId, false)
    }
  }

  const handleDelete = async (profileId: string) => {
    await deleteBrowserProfile(profileId)
    toast.success('配置已删除')
    loadProfiles()
  }

  // 批量操作
  const toggleSelect = (profileId: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      next.has(profileId) ? next.delete(profileId) : next.add(profileId)
      return next
    })
  }



  const handleSelectAll = () => {
    setSelectedIds(new Set(filteredProfiles.map(p => p.profileId)))
  }

  const handleDeselectAll = () => {
    setSelectedIds(new Set())
  }

  const handleBatchStart = async () => {
    const ids = Array.from(selectedIds).filter(id => {
      const profile = profiles.find(p => p.profileId === id)
      return profile && !profile.running
    })
    if (ids.length === 0) return
    setBatchLoading(true)
    // 立即将所有待启动实例标记为启动中（含尚在前端队列等待的），状态变化清晰
    ids.forEach(id => updatePendingIds(setStartingIds, id, true))
    let success = 0, pending = 0, failed = 0
    const pendingMessages: string[] = []
    const failureMessages: string[] = []
    await runWithConcurrency(ids, BATCH_OP_CONCURRENCY, async (id) => {
      const profile = profiles.find(p => p.profileId === id)
      try {
        const startedProfile = await startBrowserInstance(id)
        mergeProfileState(startedProfile)
        success++
      } catch (error: any) {
        const feedback = resolveActionFeedback(error, '实例启动失败')
        if (feedback.pendingAttach) {
          pending++
          pendingMessages.push(`${profile?.profileName ?? id}：${feedback.message}`)
        } else {
          failed++
          failureMessages.push(`${profile?.profileName ?? id}：${feedback.message}`)
        }
      } finally {
        updatePendingIds(setStartingIds, id, false)
      }
    })
    setBatchLoading(false)
    const summary = [`成功 ${success}`]
    if (pending > 0) summary.push(`待接管 ${pending}`)
    if (failed > 0) summary.push(`失败 ${failed}`)
    toast.success(`批量启动完成：${summary.join('，')}`)
    if (pendingMessages.length > 0) {
      const preview = pendingMessages.slice(0, 3)
      const more = pendingMessages.length > preview.length ? `\n另有 ${pendingMessages.length - preview.length} 个实例已打开窗口，仍在后台接管。` : ''
      toast.warning(`以下实例已打开窗口，仍在后台接管：\n${preview.join('\n')}${more}`)
    }
    if (failureMessages.length > 0) {
      const preview = failureMessages.slice(0, 3)
      const more = failureMessages.length > preview.length ? `\n另有 ${failureMessages.length - preview.length} 个实例启动失败，请逐个检查。` : ''
      toast.error(`以下实例启动失败：\n${preview.join('\n')}${more}`)
    }
    loadProfiles()
  }

  const handleBatchStop = async () => {
    const ids = Array.from(selectedIds).filter(id => {
      const profile = profiles.find(p => p.profileId === id)
      return profile && profile.running
    })
    if (ids.length === 0) return
    setBatchLoading(true)
    ids.forEach(id => updatePendingIds(setStoppingIds, id, true))
    let success = 0, failed = 0
    await runWithConcurrency(ids, BATCH_OP_CONCURRENCY, async (id) => {
      try {
        const stoppedProfile = await stopBrowserInstance(id)
        mergeProfileState(stoppedProfile)
        success++
      } catch {
        failed++
      } finally {
        updatePendingIds(setStoppingIds, id, false)
      }
    })
    setBatchLoading(false)
    toast.success(`批量停止完成：成功 ${success}${failed > 0 ? `，失败 ${failed}` : ''}`)
    loadProfiles()
  }

  const handleBatchDelete = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return
    if (!(await confirm({ content: `确定删除选中的 ${ids.length} 个实例？`, danger: true }))) return
    setBatchLoading(true)
    for (const id of ids) {
      await deleteBrowserProfile(id)
    }
    setBatchLoading(false)
    setSelectedIds(new Set())
    toast.success(`已删除 ${ids.length} 个实例`)
    loadProfiles()
  }

  const handleCopy = async (profileId: string) => {
    if (!copyModal.profile) return
    setCopying(true)
    try {
      await copyBrowserProfile(profileId, copyName)
      toast.success('实例已复制')
      closeCopyModal()
      loadProfiles()
    } catch (error: any) {
      closeCopyModal()
      setOpError(typeof error === 'string' ? error : error?.message || '复制失败')
    } finally {
      setCopying(false)
    }
  }

  const handleOpenSettings = async () => {
    await Promise.all([loadSettings(), loadCores()])
    setSettingsModalOpen(true)
  }

  const handleSaveSettings = async () => {
    setSavingSettings(true)
    try {
      await saveBrowserSettings({
        ...settings,
        defaultFingerprintArgs: fingerprintText.split('\n').map(s => s.trim()).filter(Boolean),
        defaultLaunchArgs: launchText.split('\n').map(s => s.trim()).filter(Boolean),
      })
      toast.success('配置已保存')
      setSettingsModalOpen(false)
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSavingSettings(false)
    }
  }

  // 内核管理
  const handleOpenCoreModal = (core?: BrowserCore) => {
    setCoreForm(core ? { ...core } : { coreId: '', coreName: '', corePath: '', isDefault: false })
    setCoreValidation(null)
    setCoreModalOpen(true)
  }

  const handleValidateCorePath = async () => {
    if (!coreForm.corePath.trim()) {
      setCoreValidation({ valid: false, message: '请输入路径' })
      return
    }
    const result = await validateBrowserCorePath(coreForm.corePath)
    setCoreValidation(result)
  }

  const handleSaveCore = async () => {
    if (!coreForm.coreName.trim()) {
      toast.error('请输入内核名称')
      return
    }
    if (!coreForm.corePath.trim()) {
      toast.error('请输入内核路径')
      return
    }
    setSavingCore(true)
    try {
      await saveBrowserCore(coreForm)
      toast.success('内核已保存')
      setCoreModalOpen(false)
      loadCores()
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSavingCore(false)
    }
  }

  const handleDeleteCore = async (coreId: string) => {
    if (cores.length <= 1) {
      toast.error('至少保留一个内核')
      return
    }
    await deleteBrowserCore(coreId)
    toast.success('内核已删除')
    loadCores()
  }

  const handleSetDefaultCore = async (coreId: string) => {
    await setDefaultBrowserCore(coreId)
    toast.success('已设为默认')
    loadCores()
  }

  const handleRedeem = async () => {
    if (!cdKey.trim()) return
    setRedeeming(true)
    const result = await redeemCDKey(cdKey.trim())
    setRedeeming(false)
    if (result.success) {
      toast.success('兑换成功！此名额已到账')
      setCdKey('')
      loadQuota()
    } else {
      toast.error(result.message || '兑换失败')
    }
  }

  const handleClaimStarGift = async () => {
    setRedeeming(true)
    const starRes = await redeemGithubStar()
    setRedeeming(false)
    if (starRes.success) {
      toast.success('感谢您的支持！已额外赠送 50 个永久额度！')
      setCdKey('')
      loadQuota()
    } else {
      toast.error(starRes.message || '领取失败')
    }
  }

  const handleOpenGithubStarGift = async () => {
    BrowserOpenURL(PROJECT_GITHUB_URL)
    await handleClaimStarGift()
  }

  // 增强的表格列配置
  const enhancedColumns = useMemo(() => createEnhancedColumns({
    proxies,
    getProfileCoreLabel,
    getProfileStatus,
    isProfileStarting,
    isProfileStopping,
    isProfileBusy,
    onStart: handleStart,
    onStop: handleStop,
    onRestart: handleRestart,
    onKeywords: openKwModal,
    onCopy: openCopyModal,
    onDelete: handleDelete,
    onRefresh: loadProfiles,
    onToggleSelect: toggleSelect,
    selectedIds,
    onSelectAll: handleSelectAll,
    onDeselectAll: handleDeselectAll,
    totalCount: filteredProfiles.length,
  }), [proxies, selectedIds, filteredProfiles.length, startingIds, stoppingIds])

  const coreColumns: TableColumn<BrowserCore>[] = [
    { key: 'coreName', title: '名称' },
    { key: 'corePath', title: '路径' },
    {
      key: 'isDefault',
      title: '默认',
      render: (value) => value ? <Star className="w-4 h-4 text-yellow-500 fill-yellow-500" /> : null,
    },
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      render: (_, record) => (
        <div className="flex justify-end gap-1">
          {!record.isDefault && (
            <Button size="sm" variant="ghost" onClick={() => handleSetDefaultCore(record.coreId)} title="设为默认"><Star className="w-4 h-4" /></Button>
          )}
          <Button size="sm" variant="ghost" onClick={() => handleOpenCoreModal(record)} title="编辑"><Edit2 className="w-4 h-4" /></Button>
          <Button size="sm" variant="ghost" onClick={() => handleDeleteCore(record.coreId)} title="删除"><Trash2 className="w-4 h-4" /></Button>
        </div>
      ),
    },
  ]

  return (
    <div className="flex flex-col h-screen bg-[var(--color-bg-base)]">
      {confirmDialog}

      {/* 顶部标题栏 */}
      <header className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例管理</h1>
            <p className="text-sm text-[var(--color-text-muted)] mt-0.5">
              共 {profiles.length} 个实例
              {filteredProfiles.length !== profiles.length && (
                <span className="text-[var(--color-accent)]"> · 筛选后 {filteredProfiles.length} 个</span>
              )}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={handleOpenSettings}>
              <Sliders className="w-4 h-4" />
              基础配置
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => { setCdKey(''); setExpandModalOpen(true); loadQuota() }}
            >
              <Gift className="w-4 h-4" />
              扩容
            </Button>
          </div>
        </div>
      </header>

      {/* 状态概览 */}
      <div className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
        <StatusOverview {...statusStats} />
      </div>

      {/* 工具栏 */}
      <EnhancedToolbar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        onRefresh={handleRefresh}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        filterActive={filterPanelOpen}
        onToggleFilter={() => setFilterPanelOpen(prev => !prev)}
        refreshing={refreshing}
      />

      {/* 筛选面板（可折叠）*/}
      {filterPanelOpen && (
        <div className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)] animate-fade-in">
          <InstanceFilterBar
            filters={filters}
            onChange={setFilters}
            proxies={proxies}
            cores={cores}
            allTags={allTags}
            groups={groups}
          />
        </div>
      )}

      {/* 批量操作工具栏 */}
      {selectedIds.size > 0 && (
        <div className="flex-shrink-0 px-6 py-3 bg-[var(--color-bg-base)]">
          <EnhancedBatchToolbar
            selectedCount={selectedIds.size}
            totalCount={filteredProfiles.length}
            onSelectAll={handleSelectAll}
            onDeselectAll={handleDeselectAll}
            onBatchStart={handleBatchStart}
            onBatchStop={handleBatchStop}
            onBatchDelete={handleBatchDelete}
            batchLoading={batchLoading}
          />
        </div>
      )}

      {/* 主内容区 - 表格 */}
      <main className="flex-1 overflow-hidden">
        <div className="h-full overflow-auto px-6 py-4">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <div className="flex flex-col items-center gap-3">
                <div className="w-12 h-12 rounded-full bg-[var(--color-bg-muted)] flex items-center justify-center">
                  <div className="w-6 h-6 border-3 border-[var(--color-accent)] border-t-transparent rounded-full animate-spin" />
                </div>
                <span className="text-sm font-medium text-[var(--color-text-secondary)]">加载中...</span>
              </div>
            </div>
          ) : filteredProfiles.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="flex flex-col items-center gap-4 max-w-md text-center">
                <div className="w-16 h-16 rounded-2xl bg-[var(--color-bg-muted)] flex items-center justify-center">
                  <FileText className="w-8 h-8 text-[var(--color-text-muted)]" />
                </div>
                <div>
                  <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-1">
                    {profiles.length === 0 ? '还没有实例' : '未找到匹配的实例'}
                  </h3>
                  <p className="text-sm text-[var(--color-text-muted)] mb-4">
                    {profiles.length === 0
                      ? '创建第一个浏览器实例开始管理多账号环境'
                      : '尝试调整搜索或筛选条件'}
                  </p>
                  {profiles.length === 0 ? (
                    <Link to="/browser/create">
                      <Button>
                        <Plus className="w-4 h-4" />
                        新建实例
                      </Button>
                    </Link>
                  ) : (
                    <Button
                      variant="secondary"
                      onClick={() => { setSearchQuery(''); setFilters(EMPTY_FILTERS) }}
                    >
                      <XCircle className="w-4 h-4" />
                      清空筛选
                    </Button>
                  )}
                </div>
              </div>
            </div>
          ) : viewMode === 'table' ? (
            <div className="bg-[var(--color-bg-surface)] rounded-xl border border-[var(--color-border-default)] overflow-hidden">
              <Table
                columns={enhancedColumns}
                data={filteredProfiles}
                rowKey="profileId"
                stickyHeader
                maxHeight="calc(100vh - 400px)"
              />
            </div>
          ) : (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {filteredProfiles.map((record) => {
                const core = resolveProfileCore(record)
                const proxy = proxies.find(p => p.proxyId === record.proxyId)
                return (
                  <ProfileCard
                    key={record.profileId}
                    record={record}
                    selected={selectedIds.has(record.profileId)}
                    coreLabel={core?.coreName || getProfileCoreLabel(record)}
                    proxyName={proxy?.proxyName || record.proxyId || '-'}
                    status={getProfileStatus(record)}
                    isStarting={isProfileStarting(record.profileId)}
                    isStopping={isProfileStopping(record.profileId)}
                    isBusy={isProfileBusy(record.profileId)}
                    onToggleSelect={toggleSelect}
                    onStart={handleStart}
                    onStop={handleStop}
                    onRestart={handleRestart}
                    onKeywords={openKwModal}
                    onCopy={openCopyModal}
                    onDelete={handleDelete}
                    onRefreshCode={loadProfiles}
                  />
                )
              })}
            </div>
          )}
        </div>
      </main>

      {/* 基础配置弹窗 */}
      <Modal open={settingsModalOpen} onClose={() => setSettingsModalOpen(false)} title="基础配置" width="700px"
        footer={<><Button variant="secondary" onClick={() => setSettingsModalOpen(false)}>取消</Button><Button onClick={handleSaveSettings} loading={savingSettings}>保存</Button></>}>
        <div className="space-y-6">
          {/* 内核管理 */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-[var(--color-text-primary)]">内核管理</span>
              <div className="flex gap-2">
                <Button size="sm" onClick={() => handleOpenCoreModal()}><Plus className="w-4 h-4" />新增内核</Button>
              </div>
            </div>
            <Card padding="none">
              <Table columns={coreColumns} data={cores} rowKey="coreId" />
            </Card>
          </div>

          {/* 其他设置 */}
          <FormItem label="用户数据根目录">
            <Input value={settings.userDataRoot} onChange={e => setSettings(prev => ({ ...prev, userDataRoot: e.target.value }))} placeholder="data" />
          </FormItem>
          <FormItem label="默认指纹参数（每行一个）">
            <Textarea value={fingerprintText} onChange={e => setFingerprintText(e.target.value)} rows={3} placeholder="--fingerprint-brand=Chrome" />
          </FormItem>
          <FormItem label="默认启动参数（每行一个）">
            <Textarea value={launchText} onChange={e => setLaunchText(e.target.value)} rows={3} placeholder="--disable-sync" />
          </FormItem>
          <FormItem label="默认代理">
            <Input value={settings.defaultProxy} onChange={e => setSettings(prev => ({ ...prev, defaultProxy: e.target.value }))} placeholder="http://127.0.0.1:7890" />
          </FormItem>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <FormItem label="启动就绪超时（毫秒）" hint="默认 3000，慢机器可调到 5000-10000">
              <Input
                type="number"
                min={1000}
                step={500}
                value={settings.startReadyTimeoutMs}
                onChange={e => setSettings(prev => ({ ...prev, startReadyTimeoutMs: Math.max(1000, Number(e.target.value) || 3000) }))}
                placeholder="3000"
              />
            </FormItem>
            <FormItem label="启动稳定窗口（毫秒）" hint="建议 1200-3000">
              <Input
                type="number"
                min={0}
                step={100}
                value={settings.startStableWindowMs}
                onChange={e => setSettings(prev => ({ ...prev, startStableWindowMs: Math.max(0, Number(e.target.value) || 1200) }))}
                placeholder="1200"
              />
            </FormItem>
          </div>
        </div>
      </Modal>

      {/* 内核编辑弹窗 */}
      <Modal open={coreModalOpen} onClose={() => setCoreModalOpen(false)} title={coreForm.coreId ? '编辑内核' : '新增内核'} width="500px"
        footer={<><Button variant="secondary" onClick={() => setCoreModalOpen(false)}>取消</Button><Button onClick={handleSaveCore} loading={savingCore}>保存</Button></>}>
        <div className="space-y-4">
          <FormItem label="内核名称" required>
            <Input value={coreForm.coreName} onChange={e => setCoreForm(prev => ({ ...prev, coreName: e.target.value }))} placeholder="Chrome 142" />
          </FormItem>
          <FormItem label="内核路径" required>
            <div className="flex gap-2">
              <Input value={coreForm.corePath} onChange={e => { setCoreForm(prev => ({ ...prev, corePath: e.target.value })); setCoreValidation(null) }} placeholder="chrome 或 D:/browsers/chrome-120" className="flex-1" />
              <Button variant="secondary" onClick={handleValidateCorePath}>验证</Button>
            </div>
            {coreValidation && (
              <div className={`flex items-center gap-1 mt-1 text-sm ${coreValidation.valid ? 'text-green-600' : 'text-red-600'}`}>
                {coreValidation.valid ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                {coreValidation.message}
              </div>
            )}
          </FormItem>
        </div>
      </Modal>

      {/* 代理不支持弹窗 */}
      <Modal
        open={proxyErrorModal}
        onClose={() => { setProxyErrorModal(false); setPendingStartId(null) }}
        title="代理链路不可用"
        width="420px"
        footer={
          <>
            <Button variant="secondary" onClick={() => { setProxyErrorModal(false); setPendingStartId(null) }}>取消</Button>
            {pendingStartId && (
              <Link to={`/browser/edit/${pendingStartId}`}>
                <Button onClick={() => setProxyErrorModal(false)}>去修改代理</Button>
              </Link>
            )}
          </>
        }
      >
        <div className="space-y-3">
          <div className="flex items-start gap-3 p-3 rounded-lg bg-[var(--color-bg-secondary)]">
            <XCircle className="w-5 h-5 text-red-500 mt-0.5 shrink-0" />
            <p className="text-sm text-[var(--color-text-primary)]">{proxyErrorMsg}</p>
          </div>
          <p className="text-sm text-[var(--color-text-muted)]">请前往编辑页面重新选择可用链路；如果是订阅导入，先刷新订阅并确认该节点仍存在。</p>
        </div>
      </Modal>

      {/* 关键字弹窗 */}
      {kwModal.profile && (
        <KeywordsModal
          open={kwModal.open}
          profileId={kwModal.profile.profileId}
          profileName={kwModal.profile.profileName}
          initialKeywords={kwModal.profile.keywords || []}
          onClose={closeKwModal}
          onSaved={(keywords) => {
            updateProfilesState(prev => prev.map(p =>
              p.profileId === kwModal.profile!.profileId ? { ...p, keywords } : p
            ))
          }}
        />
      )}

      {/* 扩容弹窗 */}
      <Modal
        open={expandModalOpen}
        onClose={() => setExpandModalOpen(false)}
        title="实例扩容系统"
        width="480px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setExpandModalOpen(false)}>关闭</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="bg-[var(--color-bg-secondary)] p-4 rounded-lg flex items-center justify-between border border-[var(--color-border-default)]">
            <div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">当前使用情况</p>
              <p className="text-xs text-[var(--color-text-muted)] mt-1">每个配置都需要消耗 1 个实例额度</p>
            </div>
            <div className="text-right">
              <span className={`text-2xl font-semibold ${profiles.length >= maxProfileLimit ? 'text-red-500' : 'text-[var(--color-success)]'}`}>
                {profiles.length}
              </span>
              <span className="text-sm text-[var(--color-text-muted)] ml-1">/ {maxProfileLimit}</span>
            </div>
          </div>

          <div className="pt-2 border-t border-[var(--color-border-muted)]">
            <label className="block text-sm font-medium text-[var(--color-text-primary)] mb-2">使用兑换码扩容</label>
            <div className="flex gap-2">
              <Input
                value={cdKey}
                onChange={e => setCdKey(e.target.value)}
                placeholder="输入兑换码 (如 ANT-...)"
                onKeyDown={e => e.key === 'Enter' && handleRedeem()}
                className="flex-1"
              />
              <Button onClick={handleRedeem} loading={redeeming} disabled={!cdKey.trim()}>
                进行兑换
              </Button>
            </div>
          </div>

          <div className="mt-4 p-3 bg-blue-500/10 border border-blue-500/20 rounded-lg">
            <div className="flex items-center justify-between gap-4">
              <p className="text-sm text-[var(--color-text-primary)]">点亮 GitHub Star 后，可再获赠 50 个永久额度</p>
              <button
                type="button"
                className="shrink-0 rounded-full p-2 text-[var(--color-accent)] transition-colors hover:bg-[var(--color-accent)]/10 disabled:opacity-50"
                onClick={handleOpenGithubStarGift}
                disabled={redeeming}
                title="打开 GitHub 并领取赠送"
                aria-label="打开 GitHub 并领取赠送"
              >
                <ExternalLink className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </Modal>

      {/* 复制实例弹窗 */}
      <Modal
        open={copyModal.open}
        onClose={closeCopyModal}
        title="复制实例"
        width="420px"
        footer={
          <>
            <Button variant="secondary" onClick={closeCopyModal}>取消</Button>
            <Button onClick={() => copyModal.profile && handleCopy(copyModal.profile.profileId)} loading={copying}>确认复制</Button>
          </>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-[var(--color-text-muted)]">
            复制实例将保留原有的代理、内核、启动参数、标签等配置，但会生成新的指纹种子。
          </p>
          <FormItem label="新实例名称" required>
            <Input
              value={copyName}
              onChange={e => setCopyName(e.target.value)}
              placeholder="请输入新实例名称"
              autoFocus
            />
          </FormItem>
        </div>
      </Modal>

      {/* 操作错误弹窗 */}
      <Modal
        open={!!opError}
        onClose={() => setOpError('')}
        title="操作失败"
        width="420px"
        footer={<Button onClick={() => setOpError('')}>知道了</Button>}
      >
        <div className="text-[var(--color-text-secondary)] whitespace-pre-line">{opError}</div>
      </Modal>
    </div>
  )
}
