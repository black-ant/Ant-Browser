import { useCallback, useEffect, useMemo, useState } from 'react'
import { CheckSquare2, ChevronDown, Gauge, Globe2, MoreHorizontal, Pencil, Plus, RefreshCw, Route, Search, Settings2, ShieldCheck, SlidersHorizontal, Trash2, X } from 'lucide-react'
import type { SortOrder } from '../../shared/components/Table'
import { toast } from '../../shared/components'
import type { BrowserProxy, ProxyIPHealthResult } from '../../modules/browser/types'
import { fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies } from '../../modules/browser/api'
import {
  BUILTIN_PROXY_IDS,
  buildChainImportCandidate,
  buildDirectImportCandidate,
  createInitialChainImportForm,
  ensureBuiltinProxies,
  sourceHostLabel,
  toChainImportForm,
  toDirectImportForm,
  toDisplayList,
  INITIAL_DIRECT_IMPORT_FORM,
  type ChainImportForm,
  type DirectImportForm,
  type ProxyDisplayInfo,
} from '../../modules/browser/pages/proxyPool/helpers'
import { useProxyChecks } from '../../modules/browser/pages/proxyPool/useProxyChecks'
import { useProxyImportFlow } from '../../modules/browser/pages/proxyPool/useProxyImportFlow'
import { useProxySelection } from '../../modules/browser/pages/proxyPool/useProxySelection'
import { useProxyCheckSettingsModal } from '../../modules/browser/pages/proxyPool/useProxyCheckSettingsModal'
import { useProxyGlobalRefreshConfig } from '../../modules/browser/pages/proxyPool/useProxyGlobalRefreshConfig'
import { useProxySourceRefresh } from '../../modules/browser/pages/proxyPool/useProxySourceRefresh'
import { useProxyDeleteFlow } from '../../modules/browser/pages/proxyPool/useProxyDeleteFlow'
import { useProxyCoreDownload } from '../../modules/browser/pages/proxyPool/useProxyCoreDownload'
import { useProxyPoolFilter } from '../../modules/browser/pages/proxyPool/useProxyPoolFilter'
import { FishConfirm, FishSwitch } from './FishFormPrimitives'
import { FishProxyCheckSettingsModal, FishProxyCoreModal, FishProxyEditModal, FishProxyGuideModal, FishProxyHealthModal, FishProxyImportModal, FishProxyPreviewModal, type FishProxyEditFormValue } from './FishProxyPoolModals'
import './fish-proxy.css'

function latencyLabel(value: number | undefined) {
  if (value === undefined) return '未测试'
  if (value === -1) return '测试中'
  if (value === -2) return '超时'
  if (value === -3) return '不支持'
  if (value === -4) return '失败'
  return `${value} ms`
}

export function FishProxyPoolPage() {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [displayList, setDisplayList] = useState<ProxyDisplayInfo[]>([])
  const [groups, setGroups] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterProtocol, setFilterProtocol] = useState('all')
  const [filterGroup, setFilterGroup] = useState('all')
  const [filterAvailableOnly, setFilterAvailableOnly] = useState(false)
  const [sortColumn, setSortColumn] = useState('')
  const [sortOrder, setSortOrder] = useState<SortOrder>(undefined)
  const [filterOpen, setFilterOpen] = useState(false)
  const [toolsOpen, setToolsOpen] = useState(false)
  const [selectionMode, setSelectionMode] = useState(false)
  const [rowMenuId, setRowMenuId] = useState<string | null>(null)
  const [usageGuideOpen, setUsageGuideOpen] = useState(false)
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [chainEditMode, setChainEditMode] = useState(false)
  const [chainEditForm, setChainEditForm] = useState<ChainImportForm>(() => createInitialChainImportForm())
  const [directEditMode, setDirectEditMode] = useState(false)
  const [directEditForm, setDirectEditForm] = useState<DirectImportForm>({ ...INITIAL_DIRECT_IMPORT_FORM })
  const [editForm, setEditForm] = useState<FishProxyEditFormValue>({ proxyName: '', proxyConfig: '', preferredKernel: 'auto', dnsServers: '', groupName: '' })
  const [saving, setSaving] = useState(false)

  const saveProxies = useCallback(async (list: BrowserProxy[]) => {
    await saveBrowserProxies(list)
    setProxies(list)
    setDisplayList(toDisplayList(list))
    setGroups(await fetchBrowserProxyGroups())
  }, [])

  const { globalAutoRefreshEnabled, setGlobalAutoRefreshEnabled, globalRefreshInterval, globalRefreshIntervalM, setGlobalRefreshIntervalM } = useProxyGlobalRefreshConfig()
  const checks = useProxyChecks({ proxies })
  const importFlow = useProxyImportFlow({ proxies, globalAutoRefreshEnabled, globalRefreshInterval, saveProxies })
  const sourceRefresh = useProxySourceRefresh({ proxies, globalAutoRefreshEnabled, globalRefreshInterval, saveProxies })
  const checkSettingsFlow = useProxyCheckSettingsModal()
  const coreFlow = useProxyCoreDownload()

  const loadProxies = useCallback(async () => {
    setLoading(true)
    try {
      const [list, groupList] = await Promise.all([fetchBrowserProxies(), fetchBrowserProxyGroups()])
      const finalList = await ensureBuiltinProxies(list)
      setProxies(finalList)
      setDisplayList(toDisplayList(finalList))
      setGroups(groupList)
    } catch (error) { toast.error(error instanceof Error ? error.message : '加载代理失败') } finally { setLoading(false) }
  }, [])

  useEffect(() => { void loadProxies(); void coreFlow.loadBrowserSettings() }, [loadProxies, coreFlow.loadBrowserSettings])

  const { protocolOptions, filteredList } = useProxyPoolFilter({ displayList, filterProtocol, filterKeyword, filterGroup, filterAvailableOnly, sortColumn, sortOrder, latencyMap: checks.latencyMap, ipHealthMap: checks.ipHealthMap })
  const selection = useProxySelection({ proxies, filteredList, saveProxies })
  const deleteFlow = useProxyDeleteFlow({ proxies, saveProxies, removeSelectedId: selection.removeSelectedId })

  const nonDirect = displayList.filter((item) => item.proxyConfig !== 'direct://')
  const healthyCount = nonDirect.filter((item) => (checks.latencyMap[item.proxyId] ?? -1) >= 0 || checks.ipHealthMap[item.proxyId]?.ok).length
  const latencyValues = nonDirect.map((item) => checks.latencyMap[item.proxyId]).filter((value): value is number => typeof value === 'number' && value >= 0)
  const averageLatency = latencyValues.length ? Math.round(latencyValues.reduce((sum, value) => sum + value, 0) / latencyValues.length) : 0
  const sourceCount = useMemo(() => new Set(displayList.map((item) => item.sourceId).filter(Boolean)).size, [displayList])
  const hasActiveFilters = !!filterKeyword || filterProtocol !== 'all' || filterGroup !== 'all' || filterAvailableOnly || !!sortColumn

  const handleEdit = (record: ProxyDisplayInfo) => {
    const proxy = proxies.find((item) => item.proxyId === record.proxyId)
    if (!proxy || BUILTIN_PROXY_IDS.has(proxy.proxyId)) return
    setEditingProxy(proxy)
    setEditForm({ proxyName: proxy.proxyName, proxyConfig: proxy.proxyConfig, preferredKernel: proxy.preferredKernel || 'auto', dnsServers: proxy.dnsServers || '', groupName: proxy.groupName || '' })
    const chain = toChainImportForm(proxy.proxyName, proxy.proxyConfig)
    const direct = chain ? null : toDirectImportForm(proxy.proxyName, proxy.proxyConfig)
    setChainEditMode(Boolean(chain)); setChainEditForm(chain || createInitialChainImportForm())
    setDirectEditMode(Boolean(direct)); setDirectEditForm(direct || { ...INITIAL_DIRECT_IMPORT_FORM })
    setEditModalOpen(true)
  }

  const updateChainEditHop = (hop: 'first' | 'second', field: keyof ChainImportForm['first'], value: string) => setChainEditForm((current) => ({ ...current, [hop]: { ...current[hop], [field]: value } }))

  const handleSaveProxy = async () => {
    if (!editingProxy) return
    let proxyName = editForm.proxyName.trim()
    let proxyConfig = editForm.proxyConfig
    try {
      if (chainEditMode) { const candidate = buildChainImportCandidate(chainEditForm); proxyName = candidate.proxyName; proxyConfig = candidate.proxyConfig }
      else if (directEditMode) { const candidate = buildDirectImportCandidate(directEditForm); proxyName = candidate.proxyName; proxyConfig = candidate.proxyConfig }
      else if (!proxyName) throw new Error('请输入代理名称')
    } catch (error) { toast.error(error instanceof Error ? error.message : '代理配置无效'); return }
    let preferredKernel = editForm.preferredKernel
    if (chainEditMode) {
      const hy2Chain = chainEditForm.firstMode === 'node' && chainEditForm.firstNodeProtocol === 'hysteria2'
      const allowedKernel = hy2Chain ? 'sing-box' : 'xray'
      if (preferredKernel !== 'auto' && preferredKernel !== allowedKernel) preferredKernel = 'auto'
    }
    setSaving(true)
    try {
      await saveProxies(proxies.map((proxy) => proxy.proxyId === editingProxy.proxyId ? { ...proxy, proxyName, proxyConfig, preferredKernel: preferredKernel === 'auto' ? undefined : preferredKernel, dnsServers: editForm.dnsServers.trim() || undefined, groupName: editForm.groupName.trim() || undefined } : proxy))
      setEditModalOpen(false); toast.success('代理已更新')
    } catch (error) { toast.error(error instanceof Error ? error.message : '保存失败') } finally { setSaving(false) }
  }

  const clearFilters = () => { setFilterKeyword(''); setFilterProtocol('all'); setFilterGroup('all'); setFilterAvailableOnly(false); setSortColumn(''); setSortOrder(undefined) }
  const chooseSort = (value: string) => { if (!value) { setSortColumn(''); setSortOrder(undefined); return } const [column, order] = value.split(':'); setSortColumn(column); setSortOrder(order as SortOrder) }

  return <div className="fish-content fish-proxy-content">
    <section className="fish-proxy-hero"><div><span className="fish-eyebrow">代理池</span><h1>代理池</h1><p>管理代理、订阅、测速与 IP 健康。</p></div><button className="fish-btn primary" type="button" onClick={() => importFlow.setImportModalOpen(true)}><Plus size={16} strokeWidth={1.8} />导入代理</button></section>

    <section className="fish-proxy-health-strip"><div><span>代理总数</span><strong>{displayList.length}</strong></div><div><span>当前可用</span><strong>{healthyCount}</strong></div><div><span>平均延迟</span><strong>{averageLatency ? `${averageLatency} ms` : '—'}</strong></div><div><span>订阅来源</span><strong>{sourceCount}</strong></div></section>

    <section className="fish-proxy-toolbar">
      <div className="fish-search fish-proxy-search"><Search size={16} strokeWidth={1.8} /><input value={filterKeyword} onChange={(e) => setFilterKeyword(e.target.value)} placeholder="搜索代理名称或服务器" /></div>
      <div className="fish-proxy-popover-wrap"><button className={`fish-btn${hasActiveFilters ? ' active' : ''}`} type="button" onClick={() => { setFilterOpen((value) => !value); setToolsOpen(false) }}><SlidersHorizontal size={16} strokeWidth={1.8} />筛选{hasActiveFilters ? ' · 已启用' : ''}<ChevronDown size={16} strokeWidth={1.8} /></button>{filterOpen ? <div className="fish-proxy-popover filter"><div className="fish-form-grid two"><label className="fish-field"><span>协议</span><select className="fish-select" value={filterProtocol} onChange={(e) => setFilterProtocol(e.target.value)}>{protocolOptions.map((protocol) => <option key={protocol} value={protocol}>{protocol === 'all' ? '全部协议' : protocol.toUpperCase()}</option>)}</select></label><label className="fish-field"><span>分组</span><select className="fish-select" value={filterGroup} onChange={(e) => setFilterGroup(e.target.value)}><option value="all">全部分组</option>{groups.map((group) => <option key={group} value={group}>{group}</option>)}</select></label><label className="fish-field"><span>排序</span><select className="fish-select" value={sortColumn && sortOrder ? `${sortColumn}:${sortOrder}` : ''} onChange={(e) => chooseSort(e.target.value)}><option value="">默认顺序</option><option value="proxyName:asc">名称 A → Z</option><option value="latency:asc">延迟低 → 高</option><option value="latency:desc">延迟高 → 低</option></select></label><div className="fish-setting-row compact"><div><strong>只显示可用</strong><span>测速成功或 IP 健康通过</span></div><FishSwitch checked={filterAvailableOnly} onChange={setFilterAvailableOnly} /></div></div><div className="fish-inline-actions"><button className="fish-btn" type="button" onClick={clearFilters}><X size={16} strokeWidth={1.8} />清除</button></div></div> : null}</div>
      <button className="fish-btn" type="button" disabled={!filteredList.length || checks.testingAll} onClick={() => void checks.handleTestAll(filteredList)}><Gauge size={16} strokeWidth={1.8} />{checks.testingAll ? '测速中' : '测试全部'}</button>
      <button className="fish-btn" type="button" disabled={!filteredList.length || checks.checkingAllIPHealth} onClick={() => void checks.handleCheckAllIPHealth(filteredList)}><ShieldCheck size={16} strokeWidth={1.8} />{checks.checkingAllIPHealth ? '检测中' : 'IP 健康'}</button>
      <button className="fish-btn" type="button" onClick={() => void loadProxies()}><RefreshCw size={16} strokeWidth={1.8} />刷新</button>
      <span className="fish-spacer" />
      {selectionMode ? <><span className="fish-proxy-selected">已选 {selection.selectedCount}</span><button className="fish-btn" type="button" onClick={selection.handleToggleAll}><CheckSquare2 size={16} strokeWidth={1.8} />{selection.allFilteredSelected ? '取消全选' : '全选'}</button>{selection.selectedCount ? <button className="fish-btn danger" type="button" onClick={() => selection.setBatchDeleteConfirmOpen(true)}><Trash2 size={16} strokeWidth={1.8} />删除所选</button> : null}<button className="fish-btn" type="button" onClick={() => setSelectionMode(false)}>完成</button></> : null}
      <div className="fish-proxy-popover-wrap"><button className="fish-btn icon-only" type="button" aria-label="更多工具" onClick={() => { setToolsOpen((value) => !value); setFilterOpen(false) }}><MoreHorizontal size={18} strokeWidth={1.8} /></button>{toolsOpen ? <div className="fish-proxy-popover tools"><button type="button" onClick={() => { setSelectionMode(true); setToolsOpen(false) }}><CheckSquare2 size={16} strokeWidth={1.8} />批量选择</button><button type="button" disabled={!sourceRefresh.hasURLImportSources} onClick={() => { void sourceRefresh.handleRefreshAllSources(false); setToolsOpen(false) }}><RefreshCw size={16} strokeWidth={1.8} />刷新订阅</button><button type="button" onClick={() => { void checkSettingsFlow.openCheckSettings(); setToolsOpen(false) }}><Settings2 size={16} strokeWidth={1.8} />检测设置</button><button type="button" onClick={() => { coreFlow.openCoreDownload(); setToolsOpen(false) }}><Route size={16} strokeWidth={1.8} />代理内核</button><button type="button" onClick={() => { setUsageGuideOpen(true); setToolsOpen(false) }}><Globe2 size={16} strokeWidth={1.8} />使用说明</button><div className="fish-proxy-auto-row"><div><strong>订阅自动刷新</strong><span>{globalRefreshInterval} 分钟</span></div><FishSwitch checked={globalAutoRefreshEnabled} onChange={setGlobalAutoRefreshEnabled} /></div>{globalAutoRefreshEnabled ? <label className="fish-field compact"><span>刷新间隔（分钟）</span><input className="fish-input" type="number" min={5} max={1440} value={globalRefreshIntervalM} onChange={(e) => setGlobalRefreshIntervalM(e.target.value)} /></label> : null}</div> : null}</div>
    </section>

    <section className={`fish-proxy-list${selectionMode ? ' selecting' : ''}`}>
      {loading ? <div className="fish-data-empty"><strong>正在读取代理</strong><span>加载代理池与订阅信息。</span></div> : filteredList.length ? filteredList.map((record) => {
        const builtin = BUILTIN_PROXY_IDS.has(record.proxyId)
        const health: ProxyIPHealthResult | undefined = checks.ipHealthMap[record.proxyId]
        const latency = checks.latencyMap[record.proxyId]
        const sourceLabel = record.sourceUrl ? sourceHostLabel(record.sourceUrl) : '手动添加'
        return <article className="fish-proxy-row" key={record.proxyId}>
          {selectionMode ? <input className="fish-row-check" type="checkbox" disabled={builtin} checked={selection.selectedIds.has(record.proxyId)} onChange={() => selection.handleToggleOne(record.proxyId)} /> : null}
          <div className="fish-proxy-name"><span className={`fish-proxy-type-mark ${record.type.toLowerCase()}`}><Route size={16} strokeWidth={1.8} /></span><div><strong>{record.proxyName}</strong><small>{record.type.toUpperCase()} · {record.server || 'DIRECT'}{record.port ? `:${record.port}` : ''}</small></div></div>
          <div className="fish-proxy-meta"><span>{record.groupName || '未分组'}</span><small>{sourceLabel}</small></div>
          <div className={`fish-proxy-latency${typeof latency === 'number' && latency >= 0 && latency < 250 ? ' good' : ''}`}><strong>{record.proxyConfig === 'direct://' ? 'DIRECT' : latencyLabel(latency)}</strong><small>{checks.latencyEngineMap[record.proxyId] || '延迟'}</small></div>
          <div className={`fish-proxy-health${health?.ok ? ' good' : health && !health.ok ? ' bad' : ''}`}><strong>{record.proxyConfig === 'direct://' ? '不适用' : checks.checkingIPHealthIds.has(record.proxyId) ? '检测中' : health?.ok ? health.ip || '健康' : health ? '检测失败' : '未检测'}</strong><small>{health?.ok ? `${health.isResidential ? '住宅' : '机房'} · fraud ${health.fraudScore}` : health?.error || 'IP 健康'}</small></div>
          <div className="fish-proxy-actions"><button className="fish-mini-action" type="button" disabled={record.proxyConfig === 'direct://' || latency === -1} onClick={() => void checks.handleTestOne(record)}><Gauge size={16} strokeWidth={1.8} />测速</button><button className="fish-mini-action" type="button" disabled={builtin} onClick={() => handleEdit(record)}><Pencil size={16} strokeWidth={1.8} />编辑</button><div className="fish-row-menu-wrap"><button className="fish-mini-icon" type="button" onClick={() => setRowMenuId(rowMenuId === record.proxyId ? null : record.proxyId)}><MoreHorizontal size={16} strokeWidth={1.8} /></button>{rowMenuId === record.proxyId ? <div className="fish-context-menu proxy-row-menu">{record.sourceId && record.sourceUrl ? <button type="button" onClick={() => { void sourceRefresh.refreshSingleSource(record.sourceId, false); setRowMenuId(null) }}><RefreshCw size={16} strokeWidth={1.8} />刷新订阅</button> : null}<button type="button" disabled={record.proxyConfig === 'direct://'} onClick={() => { void checks.handleCheckOneIPHealth(record); setRowMenuId(null) }}><ShieldCheck size={16} strokeWidth={1.8} />IP 健康</button>{health ? <button type="button" onClick={() => { checks.openIPHealthDetail(record.proxyId); setRowMenuId(null) }}><Globe2 size={16} strokeWidth={1.8} />原始结果</button> : null}<button className="danger" type="button" disabled={builtin} onClick={() => { deleteFlow.handleDeleteClick(record.proxyId); setRowMenuId(null) }}><Trash2 size={16} strokeWidth={1.8} />删除</button></div> : null}</div></div>
        </article>
      }) : <div className="fish-data-empty"><strong>没有匹配的代理</strong><span>{hasActiveFilters ? '调整筛选条件，或清除筛选。' : '点击“导入代理”添加节点。'}</span></div>}
    </section>

    <FishProxyImportModal open={importFlow.importModalOpen} groups={groups} proxies={proxies} mode={importFlow.importMode} url={importFlow.importUrl} fetchProxyId={importFlow.importFetchProxyId} resolvedUrl={importFlow.importResolvedUrl} text={importFlow.importText} dns={importFlow.importDnsServers} prefix={importFlow.importNamePrefix} group={importFlow.importGroupName} chainText={importFlow.chainImportText} directText={importFlow.directImportText} chainForm={importFlow.chainImportForm} directForm={importFlow.directImportForm} fetching={importFlow.fetchingImportUrl} canParse={importFlow.canParseImport} onClose={() => importFlow.setImportModalOpen(false)} onParse={importFlow.handleParseImport} onFetchURL={importFlow.handleFetchImportURL} onModeChange={importFlow.handleImportModeChange} onUrlChange={importFlow.handleImportUrlChange} onFetchProxyChange={importFlow.setImportFetchProxyId} onTextChange={importFlow.setImportText} onDnsChange={importFlow.setImportDnsServers} onPrefixChange={importFlow.setImportNamePrefix} onGroupChange={importFlow.setImportGroupName} onChainTextChange={importFlow.setChainImportText} onDirectTextChange={importFlow.setDirectImportText} onApplyChainJSON={importFlow.handleApplyChainJSON} onApplyDirectText={importFlow.handleApplyDirectText} onChainFormChange={(patch) => importFlow.setChainImportForm((current) => ({ ...current, ...patch }))} onChainHopChange={importFlow.updateChainImportHop} onDirectFormChange={(patch) => importFlow.setDirectImportForm((current) => ({ ...current, ...patch }))} onFillChainTemplate={importFlow.handleFillChainTemplate} onCopyChainTemplate={() => void importFlow.handleCopyChainTemplate()} onFillDirectTemplate={importFlow.handleFillDirectTemplate} onCopyDirectTemplate={() => void importFlow.handleCopyDirectTemplate()} />
    <FishProxyPreviewModal open={importFlow.previewModalOpen} list={importFlow.previewList} removedCount={importFlow.removedPreviewProxyNames.length} importing={importFlow.importing} onClose={() => importFlow.setPreviewModalOpen(false)} onBack={() => { importFlow.setPreviewModalOpen(false); importFlow.setImportModalOpen(true) }} onConfirm={() => void importFlow.handleConfirmImport()} onRemove={importFlow.handleRemovePreviewProxy} />
    <FishProxyEditModal open={editModalOpen} saving={saving} groups={groups} proxies={proxies} form={editForm} chainMode={chainEditMode} chainForm={chainEditForm} directMode={directEditMode} directForm={directEditForm} onClose={() => setEditModalOpen(false)} onSave={() => void handleSaveProxy()} onChange={(patch) => setEditForm((current) => ({ ...current, ...patch }))} onChainFormChange={(patch) => setChainEditForm((current) => ({ ...current, ...patch }))} onDirectFormChange={(patch) => setDirectEditForm((current) => ({ ...current, ...patch }))} onChainHopChange={updateChainEditHop} />
    <FishProxyHealthModal open={checks.ipHealthDetailOpen} detail={checks.currentIPHealthDetail} onClose={() => checks.setIPHealthDetailOpen(false)} />
    <FishProxyCheckSettingsModal open={checkSettingsFlow.checkSettingsOpen} settings={checkSettingsFlow.checkSettings} targetsText={checkSettingsFlow.checkTargetsText} saving={checkSettingsFlow.savingCheckSettings} onClose={() => checkSettingsFlow.setCheckSettingsOpen(false)} onSave={() => void checkSettingsFlow.saveCheckSettings()} onSettingsChange={checkSettingsFlow.setCheckSettings} onTargetsTextChange={checkSettingsFlow.setCheckTargetsText} />
    <FishProxyCoreModal open={coreFlow.coreDownloadOpen} core={coreFlow.coreDownloadType} goos={coreFlow.coreDownloadGOOS} goarch={coreFlow.coreDownloadGOARCH} downloadProxy={coreFlow.coreDownloadProxy} progress={coreFlow.coreDownloadProgress} status={coreFlow.downloadCoreStatus} statusLoading={coreFlow.downloadCoreStatusLoading} onCoreChange={coreFlow.setCoreDownloadType} onGOOSChange={coreFlow.setCoreDownloadGOOS} onGOARCHChange={coreFlow.setCoreDownloadGOARCH} onDownloadProxyChange={coreFlow.setCoreDownloadProxy} onClose={coreFlow.closeCoreDownload} onStart={() => void coreFlow.handleStartCoreDownload()} />
    <FishProxyGuideModal open={usageGuideOpen} onClose={() => setUsageGuideOpen(false)} />
    <FishConfirm open={deleteFlow.deleteConfirmOpen} title="删除代理" content="确定删除这个代理吗？此操作不可恢复。" confirmText="删除" danger onClose={() => deleteFlow.setDeleteConfirmOpen(false)} onConfirm={() => { void deleteFlow.handleDeleteConfirm(); deleteFlow.setDeleteConfirmOpen(false) }} />
    <FishConfirm open={selection.batchDeleteConfirmOpen} title="批量删除代理" content={`确定删除选中的 ${selection.selectedCount} 个代理吗？此操作不可恢复。`} confirmText="删除" danger onClose={() => selection.setBatchDeleteConfirmOpen(false)} onConfirm={() => { void selection.handleBatchDeleteConfirm(); selection.setBatchDeleteConfirmOpen(false); setSelectionMode(false) }} />
  </div>
}
