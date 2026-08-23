import { useCallback, useEffect, useState } from 'react'
import { ConfirmModal, toast } from '../../../shared/components'
import type { SortOrder } from '../../../shared/components/Table'
import type { BrowserProxy, ProxyIPHealthResult } from '../types'
import { fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies } from '../api'
import {
  buildChainImportCandidate,
  buildDirectImportCandidate,
  createInitialChainImportForm,
  INITIAL_DIRECT_IMPORT_FORM,
  ensureBuiltinProxies,
  parseProxyInfo,
  toChainImportForm,
  toDirectImportForm,
  toDisplayList,
  type ChainImportForm,
  type DirectImportForm,
  type ProxyDisplayInfo,
} from './proxyPool/helpers'

import {
  ProxyPoolEditModal,
  ProxyPoolIPHealthDetailModal,
  ProxyPoolImportModal,
  ProxyPoolPreviewModal,
  type ProxyEditFormValue,
} from './proxyPool/ProxyPoolModals'
import { ProxyPoolHeader } from './proxyPool/ProxyPoolHeader'
import { ProxyPoolTableCard } from './proxyPool/ProxyPoolTableCard'
import { ProxyPoolCheckSettingsModal } from './proxyPool/ProxyPoolCheckSettingsModal'
import { ProxyPoolUsageGuideModal } from './proxyPool/ProxyPoolUsageGuideModal'
import { ProxyCoreDownloadModal } from './proxyPool/ProxyCoreDownloadModal'
import { useProxySourceRefresh } from './proxyPool/useProxySourceRefresh'
import { useProxyImportFlow } from './proxyPool/useProxyImportFlow'
import { useProxyChecks } from './proxyPool/useProxyChecks'
import { useProxySelection } from './proxyPool/useProxySelection'
import { useProxyCheckSettingsModal } from './proxyPool/useProxyCheckSettingsModal'
import { useProxyGlobalRefreshConfig } from './proxyPool/useProxyGlobalRefreshConfig'
import { useProxyDeleteFlow } from './proxyPool/useProxyDeleteFlow'
import { useProxyCoreDownload } from './proxyPool/useProxyCoreDownload'
import { useProxyPoolFilter } from './proxyPool/useProxyPoolFilter'

type KernelOption = { value: string; label: string }

function resolveChainKernelOptions(form: ChainImportForm, proxies: BrowserProxy[]): KernelOption[] {
  const front = proxies.find(proxy => proxy.proxyId === form.frontProxyId)
  const protocol = front ? parseProxyInfo(front.proxyConfig).type.toLowerCase() : ''
  const automatic: KernelOption = { value: 'auto', label: '自动（推荐）' }
  if (['hysteria', 'hysteria2', 'tuic', 'anytls'].includes(protocol)) {
    return [automatic, { value: 'sing-box', label: 'sing-box' }]
  }
  if (['mieru', 'wireguard'].includes(protocol)) {
    return [automatic, { value: 'mihomo', label: 'Mihomo' }]
  }
  return [automatic, { value: 'xray', label: 'Xray' }]
}

export function ProxyPoolPage() {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [displayList, setDisplayList] = useState<ProxyDisplayInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [usageGuideOpen, setUsageGuideOpen] = useState(false)
  const {
    coreDownloadOpen,
    coreDownloadType,
    setCoreDownloadType,
    coreDownloadGOOS,
    setCoreDownloadGOOS,
    coreDownloadGOARCH,
    setCoreDownloadGOARCH,
    coreDownloadProxy,
    setCoreDownloadProxy,
    coreDownloadProgress,
    currentCoreStatus,
    downloadCoreStatus,
    downloadCoreStatusLoading,
    loadBrowserSettings,
    handleStartCoreDownload,
    openCoreDownload,
    closeCoreDownload,
  } = useProxyCoreDownload()
  const [groups, setGroups] = useState<string[]>([])

  const [filterProtocol, setFilterProtocol] = useState<string>('all')
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterGroup, setFilterGroup] = useState<string>('all')
  const [filterAvailableOnly, setFilterAvailableOnly] = useState(false)
  const [sortColumn, setSortColumn] = useState<string>('') // 默认不排序
  const [sortOrder, setSortOrder] = useState<SortOrder>(undefined)

  const {
    checkSettingsOpen,
    setCheckSettingsOpen,
    checkSettings,
    setCheckSettings,
    checkTargetsText,
    setCheckTargetsText,
    savingCheckSettings,
    openCheckSettings,
    saveCheckSettings,
  } = useProxyCheckSettingsModal()

  const {
    globalAutoRefreshEnabled,
    setGlobalAutoRefreshEnabled,
    globalRefreshInterval,
    globalRefreshIntervalM,
    setGlobalRefreshIntervalM,
  } = useProxyGlobalRefreshConfig()

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [chainEditMode, setChainEditMode] = useState(false)
  const [chainEditForm, setChainEditForm] = useState<ChainImportForm>(() => createInitialChainImportForm())
  const [directEditMode, setDirectEditMode] = useState(false)
  const [directEditForm, setDirectEditForm] = useState<DirectImportForm>({ ...INITIAL_DIRECT_IMPORT_FORM })
  const [editForm, setEditForm] = useState<ProxyEditFormValue>({
    proxyName: '',
    proxyConfig: '',
    preferredKernel: 'auto',
    dnsServers: '',
    groupName: '',
  })
  const [saving, setSaving] = useState(false)
  const saveProxies = useCallback(async (list: BrowserProxy[]) => {
    await saveBrowserProxies(list)
    setProxies(list)
    setDisplayList(toDisplayList(list))
    const grps = await fetchBrowserProxyGroups()
    setGroups(grps)
  }, [])

  const {
    importModalOpen, setImportModalOpen, importMode, importUrl, importFetchProxyId, importResolvedUrl, importText,
    importDnsServers, importNamePrefix, importGroupName, chainImportText, directImportText,
    chainImportForm, directImportForm, previewModalOpen, setPreviewModalOpen, previewList, removedPreviewProxyNames,
    importing, fetchingImportUrl, canParseImport, setImportText, setImportDnsServers,
    setImportNamePrefix, setImportGroupName, setImportFetchProxyId, setChainImportText, setDirectImportText,
    setChainImportForm, setDirectImportForm, handleRemovePreviewProxy, updateChainImportLanding,
    handleImportModeChange, handleFillChainTemplate, handleFillDirectTemplate, handleCopyChainTemplate,
    handleCopyDirectTemplate, handleApplyChainJSON, handleApplyDirectText, handleImportUrlChange,
    handleFetchImportURL, handleParseImport, handleConfirmImport,
  } = useProxyImportFlow({
    proxies,
    globalAutoRefreshEnabled,
    globalRefreshInterval,
    saveProxies,
  })

  const {
    hasURLImportSources,
    refreshingAllSources,
    refreshingSourceIds,
    refreshSingleSource,
    handleRefreshAllSources,
  } = useProxySourceRefresh({
    proxies,
    globalAutoRefreshEnabled,
    globalRefreshInterval,
    saveProxies,
  })

  const {
    latencyMap,
    latencyEngineMap,
    latencyErrorMap,
    testingAll,
    ipHealthMap,
    checkingIPHealthIds,
    checkingAllIPHealth,
    ipHealthDetailOpen,
    setIPHealthDetailOpen,
    currentIPHealthDetail,
    setLatencyMap,
    setLatencyEngineMap,
    setIPHealthMap,
    handleTestOne,
    handleTestAll,
    handleCheckOneIPHealth,
    handleCheckAllIPHealth,
    openIPHealthDetail,
  } = useProxyChecks({ proxies })

  const loadProxies = useCallback(async () => {
    setLoading(true)
    try {
      const [list, groupList] = await Promise.all([
        fetchBrowserProxies(),
        fetchBrowserProxyGroups(),
      ])
      const finalList = await ensureBuiltinProxies(list)
      setProxies(finalList)
      setDisplayList(toDisplayList(finalList))
      setGroups(groupList)

      setLatencyMap(prev => {
        const validIds = new Set(finalList.map(p => p.proxyId))
        const next: Record<string, number> = {}
        Object.entries(prev).forEach(([proxyId, latency]) => {
          if (validIds.has(proxyId)) next[proxyId] = latency
        })
        return next
      })

      setLatencyEngineMap(prev => {
        const validIds = new Set(finalList.map(p => p.proxyId))
        const next: Record<string, string> = {}
        Object.entries(prev).forEach(([proxyId, engine]) => {
          if (validIds.has(proxyId)) next[proxyId] = engine
        })
        return next
      })

      setIPHealthMap(prev => {
        const validIds = new Set(finalList.map(p => p.proxyId))
        const next: Record<string, ProxyIPHealthResult> = {}
        Object.entries(prev).forEach(([proxyId, health]) => {
          if (validIds.has(proxyId)) next[proxyId] = health
        })
        return next
      })
    } catch (error: any) {
      toast.error(error?.message || '加载代理失败')
    } finally {
      setLoading(false)
    }
  }, [setIPHealthMap, setLatencyEngineMap, setLatencyMap])

  useEffect(() => {
    void loadProxies()
    void loadBrowserSettings()
  }, [loadProxies, loadBrowserSettings])

  const { protocolOptions, filteredList } = useProxyPoolFilter({
    displayList,
    filterProtocol,
    filterKeyword,
    filterGroup,
    filterAvailableOnly,
    sortColumn,
    sortOrder,
    latencyMap,
    ipHealthMap,
  })

  const {
    selectedIds,
    selectedCount,
    allFilteredSelected,
    someFilteredSelected,
    batchDeleteConfirmOpen,
    setBatchDeleteConfirmOpen,
    handleToggleAll,
    handleToggleOne,
    handleBatchDeleteConfirm,
    removeSelectedId,
  } = useProxySelection({ proxies, filteredList, saveProxies })

  const updateChainEditLanding = (field: keyof ChainImportForm['landing'], value: string) => {
    setChainEditForm(prev => ({
      ...prev,
      landing: {
        ...prev.landing,
        [field]: value,
      },
    }))
  }

  const handleEdit = (record: ProxyDisplayInfo) => {
    const proxy = proxies.find(p => p.proxyId === record.proxyId)
    if (proxy) {
      setEditingProxy(proxy)
      const nextChainForm = toChainImportForm(proxy.proxyName, proxy.proxyConfig)
      const allowedKernels = nextChainForm
        ? resolveChainKernelOptions(nextChainForm, proxies).map(option => option.value)
        : []
      const preferredKernel = proxy.preferredKernel || 'auto'
      setEditForm({
        proxyName: proxy.proxyName,
        proxyConfig: proxy.proxyConfig,
        preferredKernel: nextChainForm && !allowedKernels.includes(preferredKernel) ? 'auto' : preferredKernel,
        dnsServers: proxy.dnsServers || '',
        groupName: proxy.groupName || '',
      })
      const nextDirectForm = nextChainForm ? null : toDirectImportForm(proxy.proxyName, proxy.proxyConfig)
      if (nextChainForm) {
        setChainEditMode(true)
        setChainEditForm(nextChainForm)
        setDirectEditMode(false)
        setDirectEditForm({ ...INITIAL_DIRECT_IMPORT_FORM })
      } else if (nextDirectForm) {
        setChainEditMode(false)
        setChainEditForm(createInitialChainImportForm())
        setDirectEditMode(true)
        setDirectEditForm(nextDirectForm)
      } else {
        setChainEditMode(false)
        setChainEditForm(createInitialChainImportForm())
        setDirectEditMode(false)
        setDirectEditForm({ ...INITIAL_DIRECT_IMPORT_FORM })
      }
      setEditModalOpen(true)
    }
  }

  const handleSaveProxy = async () => {
    if (!editingProxy) return

    let nextProxyName = editForm.proxyName.trim()
    let nextProxyConfig = editForm.proxyConfig
    if (chainEditMode) {
      try {
        const candidate = buildChainImportCandidate(chainEditForm)
        nextProxyName = candidate.proxyName
        nextProxyConfig = candidate.proxyConfig
      } catch (error: any) {
        toast.error(error?.message || '链式代理配置无效')
        return
      }
    } else if (directEditMode) {
      try {
        const candidate = buildDirectImportCandidate(directEditForm)
        nextProxyName = candidate.proxyName
        nextProxyConfig = candidate.proxyConfig
      } catch (error: any) {
        toast.error(error?.message || '代理配置无效')
        return
      }
    } else if (!nextProxyName) {
      toast.error('请输入代理名称')
      return
    }

    setSaving(true)
    try {
      const newProxies = proxies.map(p =>
        p.proxyId === editingProxy.proxyId
          ? {
            ...p,
            proxyName: nextProxyName,
            proxyConfig: nextProxyConfig,
            preferredKernel: editForm.preferredKernel === 'auto' ? undefined : editForm.preferredKernel,
            dnsServers: editForm.dnsServers.trim() || undefined,
            groupName: editForm.groupName.trim() || undefined,
          }
          : p
      )
      await saveProxies(newProxies)
      setEditModalOpen(false)
      toast.success('代理已更新')
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }
  const {
    deleteConfirmOpen,
    setDeleteConfirmOpen,
    handleDeleteClick,
    handleDeleteConfirm,
  } = useProxyDeleteFlow({ proxies, saveProxies, removeSelectedId })
  return (
    <div className="space-y-5 animate-fade-in">
      <ProxyPoolHeader
        checkingAllIPHealth={checkingAllIPHealth}
        currentConnectorStatus={currentCoreStatus?.message || '未知'}
        hasURLImportSources={hasURLImportSources}
        onCheckAllIPHealth={() => void handleCheckAllIPHealth(filteredList)}
        onOpenSettings={() => void openCheckSettings()}
        onOpenUsageGuide={() => setUsageGuideOpen(true)}
        onOpenImport={() => setImportModalOpen(true)}
        onOpenCoreDownload={openCoreDownload}
        onRefreshAllSources={() => void handleRefreshAllSources(false)}
        onTestAll={() => void handleTestAll(filteredList)}
        refreshingAllSources={refreshingAllSources}
        testingAll={testingAll}
        totalCount={filteredList.length}
      />

      <ProxyPoolTableCard
        allFilteredSelected={allFilteredSelected}
        checkingIPHealthIds={checkingIPHealthIds}
        data={filteredList}
        filterGroup={filterGroup}
        filterKeyword={filterKeyword}
        filterProtocol={filterProtocol}
        filterAvailableOnly={filterAvailableOnly}
        globalAutoRefreshEnabled={globalAutoRefreshEnabled}
        globalRefreshInterval={globalRefreshInterval}
        globalRefreshIntervalM={globalRefreshIntervalM}
        groups={groups}
        ipHealthMap={ipHealthMap}
        latencyMap={latencyMap}
        latencyEngineMap={latencyEngineMap}
        latencyErrorMap={latencyErrorMap}
        loading={loading}
        onCheckOneIPHealth={(record) => void handleCheckOneIPHealth(record)}
        onClearFilters={() => {
          setFilterProtocol('all')
          setFilterKeyword('')
          setFilterGroup('all')
          setFilterAvailableOnly(false)
        }}
        onDelete={handleDeleteClick}
        onEdit={handleEdit}
        onFilterGroupChange={setFilterGroup}
        onFilterKeywordChange={setFilterKeyword}
        onFilterProtocolChange={setFilterProtocol}
        onFilterAvailableOnlyChange={setFilterAvailableOnly}
        onGlobalAutoRefreshEnabledChange={setGlobalAutoRefreshEnabled}
        onGlobalRefreshIntervalMChange={setGlobalRefreshIntervalM}
        onOpenBatchDelete={() => setBatchDeleteConfirmOpen(true)}
        onOpenIPHealthDetail={openIPHealthDetail}
        onRefreshSingleSource={(sourceId) => void refreshSingleSource(sourceId, false)}
        onSort={({ column, order }) => {
          setSortColumn(column)
          setSortOrder(order)
        }}
        onTestOne={(record) => void handleTestOne(record)}
        onToggleAll={handleToggleAll}
        onToggleOne={handleToggleOne}
        protocolOptions={protocolOptions}
        refreshingSourceIds={refreshingSourceIds}
        selectedCount={selectedCount}
        selectedIds={selectedIds}
        someFilteredSelected={someFilteredSelected}
        sortColumn={sortColumn}
        sortOrder={sortOrder}
      />

      <ProxyPoolUsageGuideModal
        open={usageGuideOpen}
        onClose={() => setUsageGuideOpen(false)}
      />

      <ProxyPoolImportModal
        open={importModalOpen}
        groups={groups}
        importMode={importMode}
        importUrl={importUrl}
        importFetchProxyId={importFetchProxyId}
        importResolvedUrl={importResolvedUrl}
        importText={importText}
        importDnsServers={importDnsServers}
        importNamePrefix={importNamePrefix}
        importGroupName={importGroupName}
        chainImportText={chainImportText}
        directImportText={directImportText}
        chainImportForm={chainImportForm}
        directImportForm={directImportForm}
        fetchingImportUrl={fetchingImportUrl}
        fetchProxyOptions={proxies.filter(proxy => {
          const config = proxy.proxyConfig.trim().toLowerCase()
          return !!config && !config.startsWith('direct://') && !config.startsWith('chain+proxy://') && !config.startsWith('chain+socks5://')
        })}
        canParseImport={canParseImport}
        onClose={() => setImportModalOpen(false)}
        onParse={handleParseImport}
        onFetchImportUrl={handleFetchImportURL}
        onImportModeChange={handleImportModeChange}
        onImportUrlChange={handleImportUrlChange}
        onImportFetchProxyIdChange={setImportFetchProxyId}
        onImportTextChange={setImportText}
        onImportDnsServersChange={setImportDnsServers}
        onImportNamePrefixChange={setImportNamePrefix}
        onImportGroupNameChange={setImportGroupName}
        onChainImportTextChange={setChainImportText}
        onDirectImportTextChange={setDirectImportText}
        onApplyChainJSON={handleApplyChainJSON}
        onApplyDirectText={handleApplyDirectText}
        onChainImportFormChange={(patch) => setChainImportForm((prev) => ({ ...prev, ...patch }))}
        onChainImportLandingChange={updateChainImportLanding}
        onFillChainTemplate={handleFillChainTemplate}
        onCopyChainTemplate={() => void handleCopyChainTemplate()}
        onFillDirectTemplate={handleFillDirectTemplate}
        onCopyDirectTemplate={() => void handleCopyDirectTemplate()}
        onDirectImportFormChange={(patch) => setDirectImportForm((prev) => ({ ...prev, ...patch }))}
      />

      <ProxyPoolPreviewModal
        open={previewModalOpen}
        importMode={importMode}
        importDnsServers={importDnsServers}
        previewList={previewList}
        removedPreviewProxyNames={removedPreviewProxyNames}
        importing={importing}
        onClose={() => setPreviewModalOpen(false)}
        onBack={() => {
          setPreviewModalOpen(false)
          setImportModalOpen(true)
        }}
        onConfirm={handleConfirmImport}
        onRemoveProxy={handleRemovePreviewProxy}
      />

      <ProxyPoolEditModal
        open={editModalOpen}
        saving={saving}
        groups={groups}
        editForm={editForm}
        chainEditMode={chainEditMode}
        chainEditForm={chainEditForm}
        chainKernelOptions={resolveChainKernelOptions(chainEditForm, proxies)}
        frontProxyOptions={proxies.filter(proxy => proxy.proxyId !== editingProxy?.proxyId && !proxy.proxyConfig.trim().toLowerCase().startsWith('direct://') && !proxy.proxyConfig.trim().toLowerCase().startsWith('chain+'))}
        directEditMode={directEditMode}
        directEditForm={directEditForm}
        onClose={() => setEditModalOpen(false)}
        onSave={handleSaveProxy}
        onChange={(patch) => setEditForm((prev) => ({ ...prev, ...patch }))}
        onChainEditFormChange={(patch) => setChainEditForm((prev) => ({ ...prev, ...patch }))}
        onDirectEditFormChange={(patch) => setDirectEditForm((prev) => ({ ...prev, ...patch }))}
        onChainEditLandingChange={updateChainEditLanding}
      />

      <ProxyPoolIPHealthDetailModal
        open={ipHealthDetailOpen}
        detail={currentIPHealthDetail}
        onClose={() => setIPHealthDetailOpen(false)}
      />

      <ProxyPoolCheckSettingsModal
        open={checkSettingsOpen}
        checkSettings={checkSettings}
        checkTargetsText={checkTargetsText}
        saving={savingCheckSettings}
        onClose={() => setCheckSettingsOpen(false)}
        onSave={saveCheckSettings}
        onCheckSettingsChange={setCheckSettings}
        onCheckTargetsTextChange={setCheckTargetsText}
      />

      <ProxyCoreDownloadModal
        open={coreDownloadOpen}
        core={coreDownloadType}
        goos={coreDownloadGOOS}
        goarch={coreDownloadGOARCH}
        downloadProxy={coreDownloadProxy}
        progress={coreDownloadProgress}
        status={downloadCoreStatus}
        statusLoading={downloadCoreStatusLoading}
        onCoreChange={setCoreDownloadType}
        onGOOSChange={setCoreDownloadGOOS}
        onGOARCHChange={setCoreDownloadGOARCH}
        onDownloadProxyChange={setCoreDownloadProxy}
        onClose={closeCoreDownload}
        onStart={handleStartCoreDownload}
      />

      <ConfirmModal open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)} onConfirm={handleDeleteConfirm}
        title="确认删除" content="确定要删除这个代理吗？此操作不可恢复。" confirmText="删除" danger />

      <ConfirmModal open={batchDeleteConfirmOpen} onClose={() => setBatchDeleteConfirmOpen(false)} onConfirm={handleBatchDeleteConfirm}
        title="批量删除" content={`确定要删除选中的 ${selectedCount} 个代理吗？此操作不可恢复。`} confirmText="删除" danger />
    </div>
  )
}
