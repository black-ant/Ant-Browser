import { useCallback, useMemo, useState, type MouseEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Archive,
  CheckSquare2,
  Chrome,
  Copy,
  Download,
  FolderOpen,
  Gauge,
  KeyRound,
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
  SlidersHorizontal,
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

export function FishBrowserListPage() {
  const navigate = useNavigate()
  const { viewMode, setViewMode, filters, setFilters } = useBrowserListViewState()
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [selectionMode, setSelectionMode] = useState(false)
  const [filterOpen, setFilterOpen] = useState(false)
  const [manageOpen, setManageOpen] = useState(false)
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
  const errorCount = useMemo(() => profiles.filter((profile) => Boolean(profile.lastError)).length, [profiles])
  const selectedProfiles = useMemo(() => profiles.filter((profile) => selectedIds.has(profile.profileId)), [profiles, selectedIds])
  const searchValue = filters.keyword || filters.kwSearch
  const hasAdvancedFilter = Boolean(filters.groupId || filters.proxyId || filters.coreId || filters.tags.size)

  const updateStatus = (status: '' | 'running' | 'stopped') => setFilters({ ...filters, status })
  const clearAdvancedFilters = () => setFilters({ ...filters, groupId: '', proxyId: '', coreId: '', tags: new Set() })
  const closeTransient = () => { setRowMenu(null); setFilterOpen(false); setManageOpen(false) }

  const toggleSelected = (profileId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current)
      next.has(profileId) ? next.delete(profileId) : next.add(profileId)
      return next
    })
  }

  const leaveSelectionMode = () => {
    setSelectionMode(false)
    setSelectedIds(new Set())
  }

  const handleBatchStart = async () => {
    const targets = filteredProfiles.filter((profile) => selectedIds.has(profile.profileId) && !profile.running)
    if (targets.length === 0) return
    setBulkBusy(true)
    try { for (const profile of targets) await handleStart(profile.profileId) } finally { setBulkBusy(false) }
  }

  const handleBatchStop = async () => {
    const targets = filteredProfiles.filter((profile) => selectedIds.has(profile.profileId) && profile.running)
    if (targets.length === 0) return
    setBulkBusy(true)
    try { for (const profile of targets) await handleStop(profile.profileId) } finally { setBulkBusy(false) }
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
    } finally { setBulkBusy(false) }
  }

  const confirmBatchDelete = async () => {
    if (selectedIds.size === 0) return
    setBulkBusy(true)
    try {
      for (const profileId of selectedIds) await deleteBrowserProfile(profileId)
      toast.success(`已将 ${selectedIds.size} 个实例移入回收站`)
      leaveSelectionMode()
      setBatchDeleteOpen(false)
      await loadProfiles()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '批量删除失败')
    } finally { setBulkBusy(false) }
  }

  const handleImport = async () => {
    setManageOpen(false)
    try {
      const result = await importBrowserProfilePackage()
      if (!result.cancelled) {
        toast.success(result.message || `已导入 ${result.importedCount} 个实例`)
        await loadProfiles()
      }
    } catch (error) { toast.error(error instanceof Error ? error.message : '导入实例失败') }
  }

  const handleOpenDataDir = async (profile: BrowserProfile) => {
    try {
      if (!profile.userDataDir.trim()) { toast.warning('该实例没有用户数据目录'); return }
      const opened = await openUserDataDir(profile.userDataDir)
      if (!opened) toast.warning('当前环境不支持打开数据目录')
    } catch (error) { toast.error(error instanceof Error ? error.message : '打开数据目录失败') }
  }

  const handleExportProfile = async (profile: BrowserProfile) => {
    if (profile.running) { toast.error(`请先停止实例再导出：${profile.profileName}`); return }
    try {
      const result = await exportBrowserProfilePackage([profile.profileId])
      if (!result.cancelled) toast.success(result.message || '实例已导出')
    } catch (error) { toast.error(error instanceof Error ? error.message : '导出实例失败') }
  }

  const handleSpeedTest = async (profile: BrowserProfile) => {
    if (!profile.proxyId) { toast.warning('当前实例没有绑定代理'); return }
    try {
      const result = await browserProxyTestSpeed(profile.proxyId)
      toast.success(result.ok ? `代理延迟 ${result.latencyMs} ms` : result.error || '代理测速失败')
    } catch (error) { toast.error(error instanceof Error ? error.message : '代理测速失败') }
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
      setSelectedIds((current) => { const next = new Set(current); next.delete(profileId); return next })
      await loadProfiles()
    } catch (error) { toast.error(error instanceof Error ? error.message : '删除实例失败') }
  }

  const openRowMenu = (event: MouseEvent, profile: BrowserProfile) => {
    event.stopPropagation()
    const rect = event.currentTarget.getBoundingClientRect()
    setFilterOpen(false)
    setManageOpen(false)
    setRowMenu({ profile, x: Math.min(rect.left, window.innerWidth - 210), y: Math.min(rect.bottom + 5, window.innerHeight - 360) })
  }

  const runMenuAction = async (action: string, profile: BrowserProfile) => {
    setRowMenu(null)
    if (action === 'edit') navigate(`/browser/edit/${profile.profileId}`)
    if (action === 'copy') navigate(`/browser/copy/${profile.profileId}`)
    if (action === 'restart') await handleRestart(profile.profileId)
    if (action === 'stop') await handleStop(profile.profileId)
    if (action === 'folder') await handleOpenDataDir(profile)
    if (action === 'export') await handleExportProfile(profile)
    if (action === 'speed') await handleSpeedTest(profile)
    if (action === 'proxy') setProxyPickerProfile(profile)
    if (action === 'keywords') setKeywordsProfile(profile)
    if (action === 'extensions') setExtensionsProfile(profile)
    if (action === 'code') setLaunchCodeProfile(profile)
    if (action === 'delete') setDeleteProfile(profile)
  }

  const renderProfile = (profile: BrowserProfile, index: number, card = false) => {
    const selected = selectedIds.has(profile.profileId)
    const status = getProfileStatus(profile)
    const starting = isProfileStarting(profile.profileId)
    const stopping = isProfileStopping(profile.profileId)
    const groupName = profile.groupId ? groupNameMap.get(profile.groupId) || profile.groupId : '未分组'
    const proxyName = profile.proxyId ? proxyNameMap.get(profile.proxyId) || profile.proxyId : profile.proxyConfig ? '自定义代理' : '直连（不走代理）'
    const statusClass = profile.lastError ? ' error' : starting || stopping ? ' warn' : profile.running ? '' : ' off'
    const fingerprintLabel = profile.fingerprintArgs?.length ? `${profile.fingerprintArgs.length} 项指纹参数` : getProfileCoreLabel(profile)
    const actions = (
      <div className="fish-row-actions">
        <button className={`fish-mini-btn fish-row-launch${profile.running ? ' running' : ''}`} disabled={starting || stopping} type="button" onClick={() => void (profile.running ? handleStop(profile.profileId) : handleStart(profile.profileId))}>{starting ? '启动中' : stopping ? '停止中' : profile.running ? '停止' : '启动'}</button>
        <button className="fish-mini-icon" type="button" title="快速详情" onClick={() => setDetailProfile(profile)}><PanelRightOpen size={16} strokeWidth={1.8} /></button>
        <button className="fish-mini-icon" type="button" title="更多操作" onClick={(event) => openRowMenu(event, profile)}><MoreHorizontal size={16} strokeWidth={1.8} /></button>
      </div>
    )

    if (card) {
      return (
        <article className="fish-profile-card" key={profile.profileId}>
          <div className="fish-card-top">
            {selectionMode ? <button className={`fish-select-btn${selected ? ' selected' : ''}`} type="button" onClick={() => toggleSelected(profile.profileId)}>{selected ? <CheckSquare2 size={16} strokeWidth={1.8} /> : <Square size={16} strokeWidth={1.8} />}</button> : null}
            <div className="fish-profile-main"><span className="fish-profile-icon">{String(index + 1).padStart(2, '0')}<span className="fish-browser-dot"><Chrome size={9} strokeWidth={1.8} /></span></span><div className="fish-profile-copy"><div className="fish-profile-title"><strong>{profile.profileName}</strong>{groupName !== '未分组' ? <span className="fish-pill">{groupName}</span> : null}</div><small>{profile.launchCode || profile.profileId}</small></div></div>
            <span className="fish-spacer" />{actions}
          </div>
          <div className="fish-card-meta">
            <div><span>状态</span><strong><span className={`fish-status${statusClass}`}>{status.label}</span></strong></div>
            <div><span>代理</span><strong>{proxyName}</strong></div>
            <div><span>指纹 / 内核</span><strong>{fingerprintLabel}</strong></div>
            <div><span>最近使用</span><strong>{formatUpdatedAt(profile.lastStartAt || profile.updatedAt)}</strong></div>
          </div>
          <div className="fish-tag-row">{(profile.tags || []).slice(0, 4).map((tag, tagIndex) => <span className={`fish-pill${tagIndex === 0 ? ' blue' : ''}`} key={tag}>{tag}</span>)}</div>
        </article>
      )
    }

    return (
      <div className={`fish-profile-row${selectionMode ? ' selecting' : ''}`} key={profile.profileId}>
        {selectionMode ? <button className={`fish-select-btn${selected ? ' selected' : ''}`} type="button" onClick={() => toggleSelected(profile.profileId)}>{selected ? <CheckSquare2 size={16} strokeWidth={1.8} /> : <Square size={16} strokeWidth={1.8} />}</button> : null}
        <div className="fish-profile-main">
          <span className="fish-profile-icon">{String(index + 1).padStart(2, '0')}<span className="fish-browser-dot"><Chrome size={9} strokeWidth={1.8} /></span></span>
          <div className="fish-profile-copy"><div className="fish-profile-title"><strong>{profile.profileName}</strong>{groupName !== '未分组' ? <span className="fish-pill">{groupName}</span> : null}</div><small>{profile.launchCode || profile.profileId}</small>{profile.tags?.length ? <div className="fish-tag-row">{profile.tags.slice(0, 2).map((tag, tagIndex) => <span className={`fish-pill${tagIndex === 0 ? ' blue' : ''}`} key={tag}>{tag}</span>)}</div> : null}</div>
        </div>
        <div><div className="fish-label">状态</div><div className="fish-value"><span className={`fish-status${statusClass}`}>{status.label}</span></div></div>
        <div><div className="fish-label">代理</div><div className="fish-value">{proxyName}</div></div>
        <div><div className="fish-label">指纹 / 内核</div><div className="fish-value">{fingerprintLabel}</div></div>
        <div><div className="fish-label">最近使用</div><div className="fish-value">{formatUpdatedAt(profile.lastStartAt || profile.updatedAt)}</div></div>
        {actions}
      </div>
    )
  }

  return (
    <div className="fish-content" onClick={closeTransient}>
      <section className="fish-stats-line">
        <div className="fish-stat"><span>全部环境</span><strong>{profiles.length}</strong></div>
        <div className="fish-stat"><span>运行中</span><strong>{runningCount}</strong></div>
        <div className="fish-stat"><span>可用代理</span><strong>{proxies.length}</strong></div>
        <div className="fish-stat"><span>异常环境</span><strong>{errorCount}</strong></div>
      </section>

      <div className="fish-toolbar">
        <label className="fish-search"><Search size={16} strokeWidth={1.8} /><input value={searchValue} onChange={(event) => setFilters({ ...filters, keyword: event.target.value, kwSearch: '' })} placeholder="搜索环境、分组、代理或标签" /></label>
        <button className={`fish-chip${filters.status === '' && !filters.groupId ? ' active' : ''}`} type="button" onClick={(event) => { event.stopPropagation(); setFilters({ ...filters, status: '', groupId: '' }) }}>全部</button>
        <button className={`fish-chip${filters.status === 'running' ? ' active' : ''}`} type="button" onClick={(event) => { event.stopPropagation(); updateStatus('running') }}>运行中</button>
        <button className={`fish-chip${filters.status === 'stopped' ? ' active' : ''}`} type="button" onClick={(event) => { event.stopPropagation(); updateStatus('stopped') }}>已停止</button>
        {groups.slice(0, 2).map((group) => <button className={`fish-chip${filters.groupId === group.groupId ? ' active' : ''}`} type="button" key={group.groupId} onClick={(event) => { event.stopPropagation(); setFilters({ ...filters, groupId: filters.groupId === group.groupId ? '' : group.groupId }) }}>{group.groupName}</button>)}
        <span className="fish-spacer" />
        <button className={`fish-btn${hasAdvancedFilter ? ' soft' : ''}`} type="button" onClick={(event) => { event.stopPropagation(); setManageOpen(false); setFilterOpen((value) => !value) }}><SlidersHorizontal size={16} strokeWidth={1.8} />筛选</button>
        <button className="fish-btn" type="button" onClick={(event) => { event.stopPropagation(); void loadProfiles() }}><RefreshCw size={16} strokeWidth={1.8} />同步</button>
        <button className="fish-icon-btn" type="button" title="更多管理" onClick={(event) => { event.stopPropagation(); setFilterOpen(false); setManageOpen((value) => !value) }}><MoreHorizontal size={16} strokeWidth={1.8} /></button>

        {filterOpen ? (
          <div className="fish-menu fish-filter-panel" onClick={(event) => event.stopPropagation()}>
            <div className="fish-filter-grid">
              <div className="fish-field"><label>分组</label><select className="fish-select" value={filters.groupId} onChange={(event) => setFilters({ ...filters, groupId: event.target.value })}><option value="">全部分组</option><option value="__ungrouped__">未分组</option>{groups.map((group) => <option value={group.groupId} key={group.groupId}>{group.groupName}</option>)}</select></div>
              <div className="fish-field"><label>代理</label><select className="fish-select" value={filters.proxyId} onChange={(event) => setFilters({ ...filters, proxyId: event.target.value })}><option value="">全部代理</option><option value="__none__">无代理</option>{proxies.map((proxy) => <option value={proxy.proxyId} key={proxy.proxyId}>{proxy.proxyName || proxy.proxyId}</option>)}</select></div>
              <div className="fish-field"><label>内核</label><select className="fish-select" value={filters.coreId} onChange={(event) => setFilters({ ...filters, coreId: event.target.value })}><option value="">全部内核</option>{cores.map((core) => <option value={core.coreId} key={core.coreId}>{core.coreName}</option>)}</select></div>
              <div className="fish-field"><label>视图</label><div className="fish-segment"><button className={viewMode === 'table' ? 'active' : ''} type="button" onClick={() => setViewMode('table')}><List size={16} strokeWidth={1.8} />列表</button><button className={viewMode === 'card' ? 'active' : ''} type="button" onClick={() => setViewMode('card')}><LayoutGrid size={16} strokeWidth={1.8} />卡片</button></div></div>
            </div>
            {hasAdvancedFilter ? <button className="fish-btn" type="button" style={{ marginTop: 10 }} onClick={clearAdvancedFilters}><X size={16} strokeWidth={1.8} />清除高级筛选</button> : null}
          </div>
        ) : null}

        {manageOpen ? (
          <div className="fish-menu" style={{ right: 0, top: 46 }} onClick={(event) => event.stopPropagation()}>
            <button className="fish-menu-item" type="button" onClick={() => { setSelectionMode(true); setManageOpen(false) }}><CheckSquare2 size={16} strokeWidth={1.8} />批量操作</button>
            <button className="fish-menu-item" type="button" onClick={() => void handleImport()}><Upload size={16} strokeWidth={1.8} />导入环境</button>
            <button className="fish-menu-item" type="button" onClick={() => { setBackupOpen(true); setManageOpen(false) }}><Archive size={16} strokeWidth={1.8} />备份与恢复</button>
            <button className="fish-menu-item" type="button" onClick={() => { setTrashOpen(true); setManageOpen(false) }}><Trash2 size={16} strokeWidth={1.8} />回收站</button>
          </div>
        ) : null}
      </div>

      {selectionMode ? (
        <div className="fish-batch" onClick={(event) => event.stopPropagation()}>
          <strong>已选择 {selectedIds.size} 个环境</strong>
          <button className="fish-btn" disabled={bulkBusy || selectedIds.size === 0} type="button" onClick={() => void handleBatchStart()}><Play size={16} strokeWidth={1.8} />启动</button>
          <button className="fish-btn" disabled={bulkBusy || selectedIds.size === 0} type="button" onClick={() => void handleBatchStop()}><StopCircle size={16} strokeWidth={1.8} />停止</button>
          <button className="fish-btn" disabled={bulkBusy || selectedIds.size === 0} type="button" onClick={() => void handleBatchExport()}><Download size={16} strokeWidth={1.8} />导出</button>
          <button className="fish-btn" disabled={bulkBusy || selectedIds.size === 0} type="button" onClick={() => setBackupOpen(true)}><Archive size={16} strokeWidth={1.8} />备份</button>
          <span className="fish-spacer" />
          <button className="fish-btn danger" disabled={bulkBusy || selectedIds.size === 0} type="button" onClick={() => setBatchDeleteOpen(true)}><Trash2 size={16} strokeWidth={1.8} />移到回收站</button>
          <button className="fish-mini-icon" type="button" title="退出批量操作" onClick={leaveSelectionMode}><X size={16} strokeWidth={1.8} /></button>
        </div>
      ) : null}

      <div className="fish-section-title"><h2>浏览器环境</h2><p>{filteredProfiles.length} 个环境</p></div>
      {loading ? <div className="fish-empty"><strong>正在读取环境</strong><span>从本地浏览器配置和运行状态加载。</span></div> : filteredProfiles.length === 0 ? <div className="fish-empty"><strong>没有匹配的环境</strong><span>调整搜索或筛选条件后再试。</span></div> : viewMode === 'card' ? <div className="fish-card-grid">{filteredProfiles.map((profile, index) => renderProfile(profile, index, true))}</div> : <div className="fish-profile-list">{filteredProfiles.map((profile, index) => renderProfile(profile, index))}</div>}

      {rowMenu ? (
        <div className="fish-menu fixed" style={{ left: rowMenu.x, top: rowMenu.y }} onClick={(event) => event.stopPropagation()}>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('edit', rowMenu.profile)}><Pencil size={16} strokeWidth={1.8} />编辑</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('restart', rowMenu.profile)}><RotateCw size={16} strokeWidth={1.8} />重新启动</button>
          {rowMenu.profile.running ? <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('stop', rowMenu.profile)}><StopCircle size={16} strokeWidth={1.8} />停止环境</button> : null}
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('copy', rowMenu.profile)}><Copy size={16} strokeWidth={1.8} />复制环境</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('code', rowMenu.profile)}><KeyRound size={16} strokeWidth={1.8} />快捷打开码</button>
          <div className="fish-menu-sep" />
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('keywords', rowMenu.profile)}><KeyRound size={16} strokeWidth={1.8} />关键字管理</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('extensions', rowMenu.profile)}><Puzzle size={16} strokeWidth={1.8} />实例扩展</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('proxy', rowMenu.profile)}><Route size={16} strokeWidth={1.8} />切换代理</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('speed', rowMenu.profile)}><Gauge size={16} strokeWidth={1.8} />代理测速</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('folder', rowMenu.profile)}><FolderOpen size={16} strokeWidth={1.8} />打开数据目录</button>
          <button className="fish-menu-item" type="button" onClick={() => void runMenuAction('export', rowMenu.profile)}><Download size={16} strokeWidth={1.8} />导出环境</button>
          <div className="fish-menu-sep" />
          <button className="fish-menu-item danger" type="button" onClick={() => void runMenuAction('delete', rowMenu.profile)}><Trash2 size={16} strokeWidth={1.8} />移到回收站</button>
        </div>
      ) : null}

      {detailProfile ? (
        <aside className="fish-drawer">
          <div className="fish-drawer-head"><strong>{detailProfile.profileName}</strong><span className="fish-spacer" /><button className="fish-mini-icon" type="button" onClick={() => setDetailProfile(null)}><X size={16} strokeWidth={1.8} /></button></div>
          <div className="fish-drawer-body">
            <section className="fish-drawer-section"><h4>环境信息</h4><div className="fish-kv"><span>ID</span><span>{detailProfile.profileId}</span></div><div className="fish-kv"><span>分组</span><span>{detailProfile.groupId ? groupNameMap.get(detailProfile.groupId) || detailProfile.groupId : '未分组'}</span></div><div className="fish-kv"><span>状态</span><span>{getProfileStatus(detailProfile).label}</span></div></section>
            <section className="fish-drawer-section"><h4>网络</h4><div className="fish-kv"><span>代理</span><span>{detailProfile.proxyId ? proxyNameMap.get(detailProfile.proxyId) || detailProfile.proxyId : 'DIRECT'}</span></div><div className="fish-kv"><span>PID</span><span>{detailProfile.pid || '—'}</span></div><div className="fish-kv"><span>调试端口</span><span>{detailProfile.debugPort || '—'}</span></div></section>
            <section className="fish-drawer-section"><h4>指纹</h4><div className="fish-kv"><span>内核</span><span>{getProfileCoreLabel(detailProfile)}</span></div><div className="fish-kv"><span>参数</span><span>{detailProfile.fingerprintArgs?.length || 0} 项</span></div><div className="fish-kv"><span>快捷码</span><span>{detailProfile.launchCode || '—'}</span></div></section>
            <section className="fish-drawer-section"><h4>数据</h4><div className="fish-kv"><span>数据目录</span><span>{detailProfile.userDataDir || '—'}</span></div><div className="fish-kv"><span>最近使用</span><span>{formatUpdatedAt(detailProfile.lastStartAt || detailProfile.updatedAt)}</span></div></section>
            <button className="fish-btn primary" style={{ width: '100%', marginTop: 14 }} type="button" onClick={() => navigate(`/browser/detail/${detailProfile.profileId}`)}>打开完整详情</button>
          </div>
        </aside>
      ) : null}

      {proxyErrorModal ? <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>代理不可用</strong></div><div className="ui-v2-modal-body"><p>{proxyErrorMsg || '当前代理配置无法启动。'}</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" onClick={() => { setProxyErrorModal(false); setPendingStartId(null) }}>取消</button><button className="ui-v2-btn primary" type="button" onClick={() => pendingStartId && void handleStartDirect(pendingStartId)}>使用直连启动</button></div></div></div> : null}
      {deleteProfile ? <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>移到回收站</strong></div><div className="ui-v2-modal-body"><p>确定将「{deleteProfile.profileName}」移到回收站？3 天内可以恢复。</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" onClick={() => setDeleteProfile(null)}>取消</button><button className="ui-v2-btn danger" type="button" onClick={() => void confirmDelete()}>确认删除</button></div></div></div> : null}
      {batchDeleteOpen ? <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>批量移到回收站</strong></div><div className="ui-v2-modal-body"><p>确定将选中的 {selectedIds.size} 个实例移到回收站？3 天内可以恢复。</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" disabled={bulkBusy} onClick={() => setBatchDeleteOpen(false)}>取消</button><button className="ui-v2-btn danger" type="button" disabled={bulkBusy} onClick={() => void confirmBatchDelete()}>{bulkBusy ? '删除中' : '确认删除'}</button></div></div></div> : null}

      {keywordsProfile ? <KeywordsV2Modal profile={keywordsProfile} onClose={() => setKeywordsProfile(null)} onChanged={loadProfiles} /> : null}
      {extensionsProfile ? <ProfileExtensionsV2Modal profile={extensionsProfile} onClose={() => setExtensionsProfile(null)} /> : null}
      {launchCodeProfile ? <LaunchCodeV2Modal profile={launchCodeProfile} onClose={() => setLaunchCodeProfile(null)} onChanged={loadProfiles} /> : null}
      {proxyPickerProfile ? <ProxyPickerV2Modal profile={proxyPickerProfile} proxies={proxies} onSelect={(proxy) => saveProfileProxy(proxyPickerProfile, proxy)} onClose={() => setProxyPickerProfile(null)} /> : null}
      {trashOpen ? <TrashV2Modal onClose={() => setTrashOpen(false)} onChanged={loadProfiles} /> : null}
      {backupOpen ? <BackupV2Modal selectedProfiles={selectedProfiles} runningCount={runningCount} onClose={() => setBackupOpen(false)} onChanged={loadProfiles} /> : null}
      {opError ? <div className="ui-v2-backdrop"><div className="ui-v2-modal"><div className="ui-v2-modal-head"><strong>操作失败</strong></div><div className="ui-v2-modal-body"><p>{opError}</p></div><div className="ui-v2-modal-foot"><button className="ui-v2-btn primary" type="button" onClick={() => setOpError('')}>知道了</button></div></div></div> : null}
    </div>
  )
}
