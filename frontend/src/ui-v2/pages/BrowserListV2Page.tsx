import { useCallback, useMemo, useState, type MouseEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Archive,
  ArrowLeft,
  CheckSquare2,
  Copy,
  Download,
  FolderOpen,
  Gauge,
  Key,
  LayoutGrid,
  List,
  MoreHorizontal,
  PanelRightOpen,
  Pencil,
  Play,
  Puzzle,
  RefreshCw,
  RotateCw,
  Route,
  Search,
  Square,
  StopCircle,
  Trash2,
  Upload,
  X,
} from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserCore, BrowserProfile, BrowserProxy } from '../../modules/browser/types'
import {
  browserProxyTestSpeed,
  deleteBrowserProfile,
  exportBrowserProfilePackage,
  fetchBrowserCores,
  importBrowserProfilePackage,
  openUserDataDir,
  updateBrowserProfile,
} from '../../modules/browser/api'
import { EMPTY_FILTERS } from '../../modules/browser/components/InstanceFilterBar'
import { useBrowserListData } from '../../modules/browser/pages/browserList/useBrowserListData'
import { useBrowserListDerived, useBrowserListViewState } from '../../modules/browser/pages/browserList/useBrowserListViewState'
import { useBrowserProfileActions } from '../../modules/browser/pages/browserList/useBrowserProfileActions'
import {
  BackupV2Modal,
  KeywordsV2Modal,
  LaunchCodeV2Modal,
  ProfileExtensionsV2Modal,
  ProxyPickerV2Modal,
  TrashV2Modal,
} from '../components/BrowserListV2Modals'

function formatUpdatedAt(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function BrowserListV2Page() {
  const navigate = useNavigate()
  const { viewMode, setViewMode, filters, setFilters } = useBrowserListViewState()
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [detailProfile, setDetailProfile] = useState<BrowserProfile | null>(null)
  const [rowMenu, setRowMenu] = useState<{ profile: BrowserProfile; x: number; y: number } | null>(null)
  const [deleteProfile, setDeleteProfile] = useState<BrowserProfile | null>(null)
  const [proxyErrorModal, setProxyErrorModal] = useState(false)
  const [proxyErrorMsg, setProxyErrorMsg] = useState('')
  const [pendingStartId, setPendingStartId] = useState<string | null>(null)
  const [opError, setOpError] = useState('')
  const [bulkBusy, setBulkBusy] = useState(false)
  const [keywordsProfile, setKeywordsProfile] = useState<BrowserProfile | null>(null)
  const [extensionsProfile, setExtensionsProfile] = useState<BrowserProfile | null>(null)
  const [proxyPickerProfile, setProxyPickerProfile] = useState<BrowserProfile | null>(null)
  const [launchCodeProfile, setLaunchCodeProfile] = useState<BrowserProfile | null>(null)
  const [trashOpen, setTrashOpen] = useState(false)
  const [backupOpen, setBackupOpen] = useState(false)
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false)

  const loadCores = useCallback(() => {
    void fetchBrowserCores().then(setCores).catch(() => setCores([]))
  }, [])

  const {
    profiles,
    loading,
    proxies,
    groups,
    startingIds,
    stoppingIds,
    setStartingIds,
    setStoppingIds,
    updatePendingIds,
    mergeProfileState,
    loadProfiles,
  } = useBrowserListData({ loadCores })

  const {
    runningCount,
    allTags,
    filteredProfiles,
    getProfileCoreLabel,
    isProfileStarting,
    isProfileStopping,
    getProfileStatus,
  } = useBrowserListDerived(profiles, cores, filters, startingIds, stoppingIds)

  const { handleStart, handleStartDirect, handleStop, handleRestart } = useBrowserProfileActions({
    profiles,
    setProxyErrorModal,
    setProxyErrorMsg,
    setPendingStartId,
    setOpError,
    setStartingIds,
    setStoppingIds,
    updatePendingIds,
    mergeProfileState,
    loadProfiles,
  })

  const groupNameMap = useMemo(() => new Map(groups.map((group) => [group.groupId, group.groupName])), [groups])
  const proxyNameMap = useMemo(() => new Map(proxies.map((proxy) => [proxy.proxyId, proxy.proxyName || proxy.proxyId])), [proxies])
  const stoppedCount = profiles.length - runningCount
  const searchValue = filters.keyword || filters.kwSearch
  const hasAdvancedFilter = Boolean(filters.groupId || filters.proxyId || filters.coreId || filters.tags.size)
  const selectedProfiles = useMemo(() => profiles.filter((profile) => selectedIds.has(profile.profileId)), [profiles, selectedIds])

  const updateStatus = (status: '' | 'running' | 'stopped') => setFilters({ ...filters, status })
  const clearFilters = () => setFilters({ ...EMPTY_FILTERS, tags: new Set() })

  const toggleSelected = (profileId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current)
      next.has(profileId) ? next.delete(profileId) : next.add(profileId)
      return next
    })
  }

  const handleBatchStart = async () => {
    const targets = filteredProfiles.filter((profile) => selectedIds.has(profile.profileId) && !profile.running)
    if (targets.length === 0) return
    setBulkBusy(true)
    try {
      for (const profile of targets) await handleStart(profile.profileId)
    } finally {
      setBulkBusy(false)
    }
  }

  const handleBatchStop = async () => {
    const targets = filteredProfiles.filter((profile) => selectedIds.has(profile.profileId) && profile.running)
    if (targets.length === 0) return
    setBulkBusy(true)
    try {
      for (const profile of targets) await handleStop(profile.profileId)
    } finally {
      setBulkBusy(false)
    }
  }

  const handleBatchExport = async () => {
    if (selectedIds.size === 0) return
    const running = selectedProfiles.filter((profile) => profile.running)
    if (running.length > 0) {
      toast.error(`请先停止实例再导出：${running.slice(0, 3).map((profile) => profile.profileName).join('、')}${running.length > 3 ? ' 等' : ''}`)
      return
    }
    setBulkBusy(true)
    try {
      const result = await exportBrowserProfilePackage(Array.from(selectedIds))
      if (!result.cancelled) toast.success(result.message || `已导出 ${result.profileCount} 个实例`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '批量导出失败')
    } finally {
      setBulkBusy(false)
    }
  }

  const confirmBatchDelete = async () => {
    if (selectedIds.size === 0) return
    setBulkBusy(true)
    try {
      for (const profileId of selectedIds) await deleteBrowserProfile(profileId)
      toast.success(`已将 ${selectedIds.size} 个实例移入回收站`)
      setSelectedIds(new Set())
      setBatchDeleteOpen(false)
      await loadProfiles()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '批量删除失败')
    } finally {
      setBulkBusy(false)
    }
  }

  const handleImport = async () => {
    try {
      const result = await importBrowserProfilePackage()
      if (!result.cancelled) {
        toast.success(result.message || `已导入 ${result.importedCount} 个实例`)
        await loadProfiles()
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '导入实例失败')
    }
  }

  const handleOpenDataDir = async (profile: BrowserProfile) => {
    try {
      if (!profile.userDataDir.trim()) {
        toast.warning('该实例没有用户数据目录')
        return
      }
      const opened = await openUserDataDir(profile.userDataDir)
      if (!opened) toast.warning('当前环境不支持打开数据目录')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '打开数据目录失败')
    }
  }

  const handleExportProfile = async (profile: BrowserProfile) => {
    if (profile.running) {
      toast.error(`请先停止实例再导出：${profile.profileName}`)
      return
    }
    try {
      const result = await exportBrowserProfilePackage([profile.profileId])
      if (!result.cancelled) toast.success(result.message || '实例已导出')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '导出实例失败')
    }
  }

  const handleSpeedTest = async (profile: BrowserProfile) => {
    if (!profile.proxyId) {
      toast.warning('当前实例没有绑定代理')
      return
    }
    try {
      const result = await browserProxyTestSpeed(profile.proxyId)
      toast.success(result.ok ? `代理延迟 ${result.latencyMs} ms` : result.error || '代理测速失败')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '代理测速失败')
    }
  }

  const saveProfileProxy = async (profile: BrowserProfile, proxy: BrowserProxy) => {
    try {
      const updated = await updateBrowserProfile(profile.profileId, {
        profileName: profile.profileName,
        userDataDir: profile.userDataDir,
        coreId: profile.coreId,
        restoreLastSession: profile.restoreLastSession,
        fingerprintArgs: profile.fingerprintArgs,
        proxyId: proxy.proxyId,
        proxyConfig: '',
        memoryLimitMb: profile.memoryLimitMb || 0,
        launchArgs: profile.launchArgs,
        tags: profile.tags,
        keywords: profile.keywords || [],
        groupId: profile.groupId || '',
      })
      mergeProfileState(updated || { ...profile, proxyId: proxy.proxyId, proxyConfig: '' })
      toast.success('代理已切换')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '切换代理失败')
      throw error
    }
  }

  const confirmDelete = async () => {
    if (!deleteProfile) return
    const profileId = deleteProfile.profileId
    try {
      await deleteBrowserProfile(profileId)
      toast.success('实例已移入回收站')
      setDeleteProfile(null)
      setDetailProfile(null)
      setSelectedIds((current) => {
        const next = new Set(current)
        next.delete(profileId)
        return next
      })
      await loadProfiles()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除实例失败')
    }
  }

  const openRowMenu = (event: MouseEvent, profile: BrowserProfile) => {
    event.stopPropagation()
    const rect = event.currentTarget.getBoundingClientRect()
    setRowMenu({ profile, x: Math.min(rect.left, window.innerWidth - 210), y: Math.min(rect.bottom + 5, window.innerHeight - 360) })
  }

  const runMenuAction = async (action: string, profile: BrowserProfile) => {
    setRowMenu(null)
    if (action === 'edit') navigate(`/browser/edit/${profile.profileId}`)
    if (action === 'copy') navigate(`/browser/copy/${profile.profileId}`)
    if (action === 'restart') await handleRestart(profile.profileId)
    if (action === 'folder') await handleOpenDataDir(profile)
    if (action === 'export') await handleExportProfile(profile)
    if (action === 'speed') await handleSpeedTest(profile)
    if (action === 'proxy') setProxyPickerProfile(profile)
    if (action === 'keywords') setKeywordsProfile(profile)
    if (action === 'extensions') setExtensionsProfile(profile)
    if (action === 'code') setLaunchCodeProfile(profile)
    if (action === 'delete') setDeleteProfile(profile)
  }

  return (
    <div className="ui-v2-content" onClick={() => setRowMenu(null)}>
      <section className="ui-v2-hero">
        <h1>每个浏览器实例，都保持独立、清晰、可控。</h1>
        <p>这里已经接入真实实例、代理、内核、分组和运行状态；旧版列表仍保留作为完整功能回退。</p>
      </section>

      <section className="ui-v2-stats">
        <div className="ui-v2-stat"><span>全部实例</span><strong>{profiles.length}</strong></div>
        <div className="ui-v2-stat"><span>运行中</span><strong>{runningCount}</strong></div>
        <div className="ui-v2-stat"><span>已停止</span><strong>{stoppedCount}</strong></div>
        <div className="ui-v2-stat"><span>当前筛选</span><strong>{filteredProfiles.length}</strong></div>
      </section>

      <div className="ui-v2-toolbar">
        <label className="ui-v2-search">
          <Search size={16} strokeWidth={1.8} />
          <input value={searchValue} onChange={(event) => setFilters({ ...filters, keyword: event.target.value, kwSearch: '' })} placeholder="搜索实例、快捷码、代理或标签" />
        </label>
        <button className={`ui-v2-chip${filters.status === '' ? ' active' : ''}`} type="button" onClick={() => updateStatus('')}>全部</button>
        <button className={`ui-v2-chip${filters.status === 'running' ? ' active' : ''}`} type="button" onClick={() => updateStatus('running')}>运行中</button>
        <button className={`ui-v2-chip${filters.status === 'stopped' ? ' active' : ''}`} type="button" onClick={() => updateStatus('stopped')}>已停止</button>
        {filters.tags.size > 0 ? <button className="ui-v2-chip active" type="button" onClick={() => setFilters({ ...filters, tags: new Set() })}>标签 {filters.tags.size}</button> : null}
        <span className="ui-v2-spacer" />
        <select className="ui-v2-select" value={filters.groupId} onChange={(event) => setFilters({ ...filters, groupId: event.target.value })} title="分组筛选">
          <option value="">全部分组</option><option value="__ungrouped__">未分组</option>
          {groups.map((group) => <option key={group.groupId} value={group.groupId}>{group.groupName}</option>)}
        </select>
        <select className="ui-v2-select" value={filters.proxyId} onChange={(event) => setFilters({ ...filters, proxyId: event.target.value })} title="代理筛选">
          <option value="">全部代理</option><option value="__none__">无代理</option>
          {proxies.map((proxy) => <option key={proxy.proxyId} value={proxy.proxyId}>{proxy.proxyName || proxy.proxyId}</option>)}
        </select>
        <select className="ui-v2-select" value={filters.coreId} onChange={(event) => setFilters({ ...filters, coreId: event.target.value })} title="内核筛选">
          <option value="">全部内核</option>{cores.map((core) => <option key={core.coreId} value={core.coreId}>{core.coreName}</option>)}
        </select>
        {hasAdvancedFilter ? <button className="ui-v2-icon-btn" type="button" title="清除筛选" onClick={clearFilters}><X size={16} strokeWidth={1.8} /></button> : null}
        <button className="ui-v2-btn" type="button" onClick={() => void loadProfiles()}><RefreshCw size={16} strokeWidth={1.8} />刷新</button>
      </div>

      <div className="ui-v2-toolbar ui-v2-secondary-toolbar">
        <button className="ui-v2-btn" type="button" onClick={() => void handleImport()}><Upload size={16} strokeWidth={1.8} />导入实例</button>
        <button className="ui-v2-btn" type="button" onClick={() => setBackupOpen(true)}><Archive size={16} strokeWidth={1.8} />备份与恢复</button>
        <button className="ui-v2-btn" type="button" onClick={() => setTrashOpen(true)}><Trash2 size={16} strokeWidth={1.8} />回收站</button>
        <span className="ui-v2-spacer" />
        <div className="ui-v2-view-toggle"><button className={viewMode === 'card' ? 'active' : ''} type="button" title="卡片视图" onClick={() => setViewMode('card')}><LayoutGrid size={16} strokeWidth={1.8} /></button><button className={viewMode === 'table' ? 'active' : ''} type="button" title="列表视图" onClick={() => setViewMode('table')}><List size={16} strokeWidth={1.8} /></button></div>
        <button className="ui-v2-btn soft" type="button" onClick={() => navigate('/browser/list-legacy')}><ArrowLeft size={16} strokeWidth={1.8} />返回完整旧版</button>
      </div>

      {selectedIds.size > 0 ? (
        <div className="ui-v2-toolbar batch">
          <strong>已选择 {selectedIds.size} 个实例</strong>
          <button className="ui-v2-btn" disabled={bulkBusy} type="button" onClick={() => void handleBatchStart()}><Play size={16} strokeWidth={1.8} />启动</button>
          <button className="ui-v2-btn" disabled={bulkBusy} type="button" onClick={() => void handleBatchStop()}><StopCircle size={16} strokeWidth={1.8} />停止</button>
          <button className="ui-v2-btn" disabled={bulkBusy} type="button" onClick={() => void handleBatchExport()}><Download size={16} strokeWidth={1.8} />导出</button>
          <button className="ui-v2-btn" disabled={bulkBusy} type="button" onClick={() => setBackupOpen(true)}><Archive size={16} strokeWidth={1.8} />备份</button>
          <button className="ui-v2-btn danger" disabled={bulkBusy} type="button" onClick={() => setBatchDeleteOpen(true)}><Trash2 size={16} strokeWidth={1.8} />删除</button>
          <span className="ui-v2-spacer" />
          <button className="ui-v2-mini-icon" type="button" title="取消选择" onClick={() => setSelectedIds(new Set())}><X size={16} strokeWidth={1.8} /></button>
        </div>
      ) : null}

      <div className="ui-v2-section-title">
        <h2>浏览器实例</h2>
        <p>{filteredProfiles.length} 个实例 · {allTags.length} 个标签</p>
      </div>

      <div className={`ui-v2-profile-list${viewMode === 'card' ? ' card-mode' : ''}`}>
        {loading ? (
          <div className="ui-v2-empty"><strong>正在读取实例</strong><span>从本地浏览器配置和运行状态加载。</span></div>
        ) : filteredProfiles.length === 0 ? (
          <div className="ui-v2-empty"><strong>没有匹配的实例</strong><span>调整搜索或筛选条件后再试。</span></div>
        ) : filteredProfiles.map((profile, index) => {
          const selected = selectedIds.has(profile.profileId)
          const status = getProfileStatus(profile)
          const starting = isProfileStarting(profile.profileId)
          const stopping = isProfileStopping(profile.profileId)
          const groupName = profile.groupId ? groupNameMap.get(profile.groupId) || profile.groupId : '未分组'
          const proxyName = profile.proxyId ? proxyNameMap.get(profile.proxyId) || profile.proxyId : profile.proxyConfig ? '自定义代理' : '未绑定'
          const statusClass = profile.lastError ? ' error' : starting || stopping ? ' pending' : profile.running ? '' : ' off'
          return (
            <div className={`ui-v2-profile-row${viewMode === 'card' ? ' card' : ''}`} key={profile.profileId}>
              <button className={`ui-v2-check${selected ? ' selected' : ''}`} type="button" onClick={() => toggleSelected(profile.profileId)} title="选择实例">
                {selected ? <CheckSquare2 size={16} strokeWidth={1.8} /> : <Square size={16} strokeWidth={1.8} />}
              </button>
              <div className="ui-v2-profile-main">
                <span className="ui-v2-profile-icon">{String(index + 1).padStart(2, '0')}</span>
                <div className="ui-v2-profile-copy">
                  <strong>{profile.profileName}</strong>
                  <small>{profile.launchCode || profile.profileId} · {groupName}</small>
                  <div className="ui-v2-tag-row">
                    {(profile.tags || []).slice(0, 3).map((tag, tagIndex) => <span className={`ui-v2-pill${tagIndex === 0 ? ' blue' : ''}`} key={tag}>{tag}</span>)}
                  </div>
                </div>
              </div>
              <div><div className="ui-v2-label">状态</div><div className="ui-v2-value"><span className={`ui-v2-status${statusClass}`}>{status.label}</span></div></div>
              <div><div className="ui-v2-label">代理</div><div className="ui-v2-value">{proxyName}</div></div>
              <div className="ui-v2-core-col"><div className="ui-v2-label">内核</div><div className="ui-v2-value">{getProfileCoreLabel(profile)}</div></div>
              <div className="ui-v2-recent-col"><div className="ui-v2-label">最近启动</div><div className="ui-v2-value">{formatUpdatedAt(profile.lastStartAt || profile.updatedAt)}</div></div>
              <div className="ui-v2-row-actions">
                <button className={`ui-v2-mini-btn${profile.running ? ' soft' : ''}`} disabled={starting || stopping} type="button" onClick={() => void (profile.running ? handleStop(profile.profileId) : handleStart(profile.profileId))}>{starting ? '启动中' : stopping ? '停止中' : profile.running ? '停止' : '启动'}</button>
                <button className="ui-v2-mini-icon" type="button" title="快速详情" onClick={() => setDetailProfile(profile)}><PanelRightOpen size={16} strokeWidth={1.8} /></button>
                <button className="ui-v2-mini-icon" type="button" title="更多操作" onClick={(event) => openRowMenu(event, profile)}><MoreHorizontal size={16} strokeWidth={1.8} /></button>
              </div>
            </div>
          )
        })}
      </div>

      {rowMenu ? (
        <div className="ui-v2-menu fixed" style={{ left: rowMenu.x, top: rowMenu.y }} onClick={(event) => event.stopPropagation()}>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('edit', rowMenu.profile)}><Pencil size={16} strokeWidth={1.8} />编辑配置</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('restart', rowMenu.profile)}><RotateCw size={16} strokeWidth={1.8} />重新启动</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('copy', rowMenu.profile)}><Copy size={16} strokeWidth={1.8} />复制实例</button>
          <div className="ui-v2-menu-sep" />
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('folder', rowMenu.profile)}><FolderOpen size={16} strokeWidth={1.8} />打开数据目录</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('export', rowMenu.profile)}><Download size={16} strokeWidth={1.8} />导出实例</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('speed', rowMenu.profile)}><Gauge size={16} strokeWidth={1.8} />代理测速</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('proxy', rowMenu.profile)}><Route size={16} strokeWidth={1.8} />切换代理</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('keywords', rowMenu.profile)}><Key size={16} strokeWidth={1.8} />关键字</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('extensions', rowMenu.profile)}><Puzzle size={16} strokeWidth={1.8} />实例插件</button>
          <button className="ui-v2-menu-item" type="button" onClick={() => void runMenuAction('code', rowMenu.profile)}><Copy size={16} strokeWidth={1.8} />快捷打开码</button>
          <div className="ui-v2-menu-sep" />
          <button className="ui-v2-menu-item danger" type="button" onClick={() => void runMenuAction('delete', rowMenu.profile)}><Trash2 size={16} strokeWidth={1.8} />移到回收站</button>
        </div>
      ) : null}

      {detailProfile ? (
        <aside className="ui-v2-drawer">
          <div className="ui-v2-drawer-head"><strong>{detailProfile.profileName}</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" onClick={() => setDetailProfile(null)}><X size={16} strokeWidth={1.8} /></button></div>
          <div className="ui-v2-drawer-body">
            <section className="ui-v2-drawer-section"><h4>运行状态</h4><div className="ui-v2-kv"><span>状态</span><span>{getProfileStatus(detailProfile).label}</span></div><div className="ui-v2-kv"><span>PID</span><span>{detailProfile.pid || '—'}</span></div><div className="ui-v2-kv"><span>调试端口</span><span>{detailProfile.debugPort || '—'}</span></div></section>
            <section className="ui-v2-drawer-section"><h4>配置</h4><div className="ui-v2-kv"><span>快捷码</span><span>{detailProfile.launchCode || '—'}</span></div><div className="ui-v2-kv"><span>分组</span><span>{detailProfile.groupId ? groupNameMap.get(detailProfile.groupId) || detailProfile.groupId : '未分组'}</span></div><div className="ui-v2-kv"><span>代理</span><span>{detailProfile.proxyId ? proxyNameMap.get(detailProfile.proxyId) || detailProfile.proxyId : '未绑定'}</span></div><div className="ui-v2-kv"><span>内核</span><span>{getProfileCoreLabel(detailProfile)}</span></div></section>
            <section className="ui-v2-drawer-section"><h4>高级信息</h4><div className="ui-v2-kv"><span>Profile ID</span><span>{detailProfile.profileId}</span></div><div className="ui-v2-kv"><span>数据目录</span><span>{detailProfile.userDataDir}</span></div></section>
            <button className="ui-v2-btn primary" type="button" onClick={() => navigate(`/browser/detail/${detailProfile.profileId}`)}>打开完整详情</button>
          </div>
        </aside>
      ) : null}

      {proxyErrorModal ? (
        <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>代理不可用</strong></div><div className="ui-v2-modal-body"><p>{proxyErrorMsg || '当前代理配置无法启动。'}</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" onClick={() => { setProxyErrorModal(false); setPendingStartId(null) }}>取消</button><button className="ui-v2-btn primary" type="button" onClick={() => pendingStartId && void handleStartDirect(pendingStartId)}>使用直连启动</button></div></div></div>
      ) : null}

      {deleteProfile ? (
        <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>移到回收站</strong></div><div className="ui-v2-modal-body"><p>确定将「{deleteProfile.profileName}」移到回收站？3 天内可以恢复。</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" onClick={() => setDeleteProfile(null)}>取消</button><button className="ui-v2-btn danger" type="button" onClick={() => void confirmDelete()}>确认删除</button></div></div></div>
      ) : null}

      {batchDeleteOpen ? (
        <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>批量移到回收站</strong></div><div className="ui-v2-modal-body"><p>确定将选中的 {selectedIds.size} 个实例移到回收站？3 天内可以恢复。</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" disabled={bulkBusy} onClick={() => setBatchDeleteOpen(false)}>取消</button><button className="ui-v2-btn danger" type="button" disabled={bulkBusy} onClick={() => void confirmBatchDelete()}>{bulkBusy ? '删除中' : '确认删除'}</button></div></div></div>
      ) : null}

      {keywordsProfile ? <KeywordsV2Modal profile={keywordsProfile} onClose={() => setKeywordsProfile(null)} onChanged={loadProfiles} /> : null}
      {extensionsProfile ? <ProfileExtensionsV2Modal profile={extensionsProfile} onClose={() => setExtensionsProfile(null)} /> : null}
      {launchCodeProfile ? <LaunchCodeV2Modal profile={launchCodeProfile} onClose={() => setLaunchCodeProfile(null)} onChanged={loadProfiles} /> : null}
      {proxyPickerProfile ? <ProxyPickerV2Modal profile={proxyPickerProfile} proxies={proxies} onSelect={(proxy) => saveProfileProxy(proxyPickerProfile, proxy)} onClose={() => setProxyPickerProfile(null)} /> : null}
      {trashOpen ? <TrashV2Modal onClose={() => setTrashOpen(false)} onChanged={loadProfiles} /> : null}
      {backupOpen ? <BackupV2Modal selectedProfiles={selectedProfiles} runningCount={runningCount} onClose={() => setBackupOpen(false)} onChanged={loadProfiles} /> : null}

      {opError ? (
        <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>操作失败</strong></div><div className="ui-v2-modal-body"><p>{opError}</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn primary" type="button" onClick={() => setOpError('')}>知道了</button></div></div></div>
      ) : null}
    </div>
  )
}
