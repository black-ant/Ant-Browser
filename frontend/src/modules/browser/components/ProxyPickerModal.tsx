import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { GitBranch, Plus, Search, Wifi, X } from 'lucide-react'
import { ConfirmModal, Switch, toast } from '../../../shared/components'
import type { BrowserProxy } from '../types'
import { browserProxyBatchTestSpeed, browserProxyTestSpeed, fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies } from '../api'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { ProxyImportModal } from './ProxyImportModal'
import { ProxyEditModal } from './ProxyPickerModal.edit'
import { GroupItem, ProxyRow } from './ProxyPickerModal.rows'
import { ALL_GROUP, BATCH_TEST_CONCURRENCY, DIRECT_PROXY_ID, INITIAL_CHAIN_EDIT_FORM, SPEED_RESULT_EVENT, buildChainProxyConfig, formatProxyConfigForDisplay, parseChainSocks5Config, toChainEditForm, type ChainEditForm, type ChainHopForm, type SpeedResult } from './ProxyPickerModal.helpers'

interface ProxyPickerModalProps {
  open: boolean
  currentProxyId: string
  title?: string
  profileRunning?: boolean
  onSelect: (proxy: BrowserProxy, options?: { force: boolean }) => void
  onClose: () => void
  onOpenRouting?: () => void
  onProxyListUpdated?: (proxies: BrowserProxy[]) => void
  onProxyDeleted?: (deletedProxyId: string, nextProxies: BrowserProxy[]) => void
  onProxyTested?: (proxyId: string, result: SpeedResult) => void
}

export function ProxyPickerModal({ open, currentProxyId, title = '从代理池选择', profileRunning = false, onSelect, onClose, onOpenRouting, onProxyListUpdated, onProxyDeleted, onProxyTested }: ProxyPickerModalProps) {
  const [groups, setGroups] = useState<string[]>([])
  const [allProxies, setAllProxies] = useState<BrowserProxy[]>([])
  const [selectedGroup, setSelectedGroup] = useState<string>(ALL_GROUP)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [speedMap, setSpeedMap] = useState<Record<string, SpeedResult>>({})
  const [testingIds, setTestingIds] = useState<Set<string>>(new Set())
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [editName, setEditName] = useState('')
  const [editConfig, setEditConfig] = useState('')
  const [editGroup, setEditGroup] = useState('')
  const [editDnsServers, setEditDnsServers] = useState('')
  const [chainEditMode, setChainEditMode] = useState(false)
  const [chainEditForm, setChainEditForm] = useState<ChainEditForm>(INITIAL_CHAIN_EDIT_FORM)
  const [savingEdit, setSavingEdit] = useState(false)
  const [deleteCandidate, setDeleteCandidate] = useState<BrowserProxy | null>(null)
  const [forceDisconnect, setForceDisconnect] = useState(false)
  const abortRef = useRef(false)

  const loadData = async () => {
    setLoading(true)
    try {
      const [groupList, proxyList] = await Promise.all([
        fetchBrowserProxyGroups(),
        fetchBrowserProxies(),
      ])
      setGroups(groupList)
      setAllProxies(proxyList)
      onProxyListUpdated?.(proxyList)
      const initMap: Record<string, SpeedResult> = {}
      proxyList.forEach(proxy => {
        if (proxy.lastTestedAt) {
          initMap[proxy.proxyId] = {
            ok: proxy.lastTestOk ?? false,
            latencyMs: proxy.lastLatencyMs ?? -1,
            error: '',
          }
        }
      })
      setSpeedMap(initMap)
      return proxyList
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!open) return
    setSelectedGroup(ALL_GROUP)
    setSearch('')
    setSpeedMap({})
    setTestingIds(new Set())
    setEditingProxy(null)
    setDeleteCandidate(null)
    setForceDisconnect(false)
    abortRef.current = false
    void loadData()
    return () => { abortRef.current = true }
  }, [open])

  const displayProxies = useMemo(() => {
    let list = allProxies
    if (selectedGroup !== ALL_GROUP) {
      list = list.filter(proxy => proxy.groupName === selectedGroup)
    }
    if (search.trim()) {
      const query = search.trim().toLowerCase()
      list = list.filter(proxy =>
        (proxy.proxyName || '').toLowerCase().includes(query) ||
        (proxy.proxyConfig || '').toLowerCase().includes(query)
      )
    }

    const getSortTuple = (proxy: BrowserProxy): [number, number, string] => {
      const latest = speedMap[proxy.proxyId]
      const history = proxy.lastTestedAt
        ? { ok: proxy.lastTestOk ?? false, latencyMs: proxy.lastLatencyMs ?? -1 }
        : undefined
      const result = latest || history

      if (result?.ok && result.latencyMs >= 0) {
        return [0, result.latencyMs, proxy.proxyName || '']
      }
      if (proxy.proxyConfig === 'direct://') {
        return [2, Number.MAX_SAFE_INTEGER, proxy.proxyName || '']
      }
      if (result && !result.ok) {
        return [3, Number.MAX_SAFE_INTEGER, proxy.proxyName || '']
      }
      return [4, Number.MAX_SAFE_INTEGER, proxy.proxyName || '']
    }

    return [...list]
      .sort((a, b) => {
        const [rankA, latencyA, nameA] = getSortTuple(a)
        const [rankB, latencyB, nameB] = getSortTuple(b)
        if (rankA !== rankB) return rankA - rankB
        if (latencyA !== latencyB) return latencyA - latencyB
        return nameA.localeCompare(nameB, 'zh-CN')
      })
      .map(proxy => ({
        proxy,
        displayConfig: formatProxyConfigForDisplay(proxy.proxyConfig),
      }))
  }, [selectedGroup, search, allProxies, speedMap])

  const groupCounts = useMemo(() => {
    const counts = new Map<string, number>()
    allProxies.forEach(proxy => {
      const key = proxy.groupName || ''
      counts.set(key, (counts.get(key) || 0) + 1)
    })
    return counts
  }, [allProxies])

  const testOne = async (proxyId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (testingIds.has(proxyId)) return
    setTestingIds(prev => new Set(prev).add(proxyId))
    try {
      const result = await browserProxyTestSpeed(proxyId)
      if (!abortRef.current) {
        const nextResult = { ok: result.ok, latencyMs: result.latencyMs, error: result.error }
        setSpeedMap(prev => ({
          ...prev,
          [proxyId]: nextResult,
        }))
        onProxyTested?.(proxyId, nextResult)
      }
    } finally {
      setTestingIds(prev => {
        const next = new Set(prev)
        next.delete(proxyId)
        return next
      })
    }
  }

  const testAll = async () => {
    const ids = displayProxies.map(item => item.proxy.proxyId).filter(id => id !== DIRECT_PROXY_ID)
    if (ids.length === 0) return

    abortRef.current = false
    setTestingIds(new Set(ids))
    const idSet = new Set(ids)

    const off = EventsOn(SPEED_RESULT_EVENT, (data: { proxyId: string; ok: boolean; latencyMs: number; error: string }) => {
      if (abortRef.current || !idSet.has(data.proxyId)) return
      const nextResult = { ok: data.ok, latencyMs: data.latencyMs, error: data.error }
      setSpeedMap(prev => ({
        ...prev,
        [data.proxyId]: nextResult,
      }))
      onProxyTested?.(data.proxyId, nextResult)
      setTestingIds(prev => {
        const next = new Set(prev)
        next.delete(data.proxyId)
        return next
      })
    })

    try {
      const results = await browserProxyBatchTestSpeed(ids, BATCH_TEST_CONCURRENCY)
      if (!abortRef.current) {
        setSpeedMap(prev => {
          const next = { ...prev }
          let changed = false
          results.forEach(result => {
            if (!idSet.has(result.proxyId)) return
            const current = next[result.proxyId]
            if (
              !current ||
              current.ok !== result.ok ||
              current.latencyMs !== result.latencyMs ||
              current.error !== result.error
            ) {
              const nextResult = { ok: result.ok, latencyMs: result.latencyMs, error: result.error }
              next[result.proxyId] = nextResult
              onProxyTested?.(result.proxyId, nextResult)
              changed = true
            }
          })
          return changed ? next : prev
        })
      }
    } finally {
      off()
      setTestingIds(prev => {
        const next = new Set(prev)
        ids.forEach(id => next.delete(id))
        return next
      })
    }
  }

  const handleImported = async (newProxies: BrowserProxy[]) => {
    const refreshed = await loadData()
    const targetProxyId = newProxies[newProxies.length - 1]?.proxyId
    if (!targetProxyId) return
    const selected = refreshed.find(proxy => proxy.proxyId === targetProxyId)
    if (!selected) return
    onSelect(selected, { force: profileRunning && forceDisconnect })
    onClose()
  }

  const handleEditClick = (proxy: BrowserProxy, e: React.MouseEvent) => {
    e.stopPropagation()
    if (proxy.proxyId === DIRECT_PROXY_ID) return
    setEditingProxy(proxy)
    setEditName(proxy.proxyName || '')
    setEditConfig(proxy.proxyConfig || '')
    setEditGroup(proxy.groupName || '')
    setEditDnsServers(proxy.dnsServers || '')

    const chainCfg = parseChainSocks5Config(proxy.proxyConfig || '')
    if (chainCfg) {
      setChainEditMode(true)
      setChainEditForm(toChainEditForm(proxy.proxyName || '', chainCfg))
    } else {
      setChainEditMode(false)
      setChainEditForm(INITIAL_CHAIN_EDIT_FORM)
    }
  }

  const updateChainHop = (hop: 'first' | 'second', field: keyof ChainHopForm, value: string) => {
    setChainEditForm(prev => ({
      ...prev,
      [hop]: {
        ...prev[hop],
        [field]: value,
      },
    }))
  }

  const closeEditModal = () => {
    setEditingProxy(null)
    setSavingEdit(false)
  }

  const handleSaveEdit = async () => {
    if (!editingProxy) return
    const nextName = chainEditMode ? chainEditForm.proxyName.trim() : editName.trim()
    if (!nextName) {
      toast.error('请输入代理名称')
      return
    }

    let nextConfig = editConfig.trim()
    if (chainEditMode) {
      try {
        nextConfig = buildChainProxyConfig(chainEditForm)
      } catch (error: any) {
        toast.error(error?.message || '链式代理配置无效')
        return
      }
    }

    const nextProxies = allProxies.map(item =>
      item.proxyId === editingProxy.proxyId
        ? {
            ...item,
            proxyName: nextName,
            proxyConfig: nextConfig,
            groupName: editGroup.trim() || undefined,
            dnsServers: editDnsServers.trim() || undefined,
          }
        : item
    )

    setSavingEdit(true)
    try {
      await saveBrowserProxies(nextProxies)
      setAllProxies(nextProxies)
      onProxyListUpdated?.(nextProxies)
      if (editingProxy.proxyId === currentProxyId) {
        const updated = nextProxies.find(item => item.proxyId === currentProxyId)
        if (updated) onSelect(updated, { force: profileRunning && forceDisconnect })
      }
      toast.success('代理已更新')
      closeEditModal()
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSavingEdit(false)
    }
  }

  const handleDeleteClick = (proxy: BrowserProxy, e: React.MouseEvent) => {
    e.stopPropagation()
    if (proxy.proxyId === DIRECT_PROXY_ID) return
    setDeleteCandidate(proxy)
  }

  const handleDeleteConfirm = async () => {
    if (!deleteCandidate) return
    const nextProxies = allProxies.filter(item => item.proxyId !== deleteCandidate.proxyId)
    try {
      await saveBrowserProxies(nextProxies)
      setAllProxies(nextProxies)
      onProxyListUpdated?.(nextProxies)
      onProxyDeleted?.(deleteCandidate.proxyId, nextProxies)
      toast.success('代理已删除')
      setDeleteCandidate(null)
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
  }

  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center" onClick={onClose}>
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" />
      <div
        className="relative bg-[var(--color-bg-elevated)] border border-[var(--color-border)] rounded-xl shadow-2xl w-[720px] max-h-[580px] flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--color-border)]">
          <span className="font-semibold text-[var(--color-text-primary)]">{title}</span>
          <button onClick={onClose} className="text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex flex-1 min-h-0">
          <div className="w-44 border-r border-[var(--color-border)] flex flex-col py-2 overflow-y-auto shrink-0 bg-[var(--color-bg-muted)]">
            <GroupItem label="全部" active={selectedGroup === ALL_GROUP} count={allProxies.length} onClick={() => setSelectedGroup(ALL_GROUP)} />
            {groups.map(groupName => (
              <GroupItem
                key={groupName}
                label={groupName}
                active={selectedGroup === groupName}
                count={groupCounts.get(groupName) || 0}
                onClick={() => setSelectedGroup(groupName)}
              />
            ))}
            {groups.length === 0 && <p className="text-xs text-[var(--color-text-muted)] px-3 py-2">暂无分组</p>}
          </div>

          <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
            <div className="px-3 py-2 border-b border-[var(--color-border)] flex gap-2 items-center">
              <div className="relative flex-1">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[var(--color-text-muted)]" />
                <input
                  type="text"
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  placeholder="搜索代理名称或配置..."
                  className="w-full pl-8 pr-3 py-1.5 text-sm bg-[var(--color-bg-input)] border border-[var(--color-border)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none focus:border-[var(--color-primary)]"
                />
              </div>
              <button
                onClick={() => setImportOpen(true)}
                className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-[var(--color-border)] text-[var(--color-text-secondary)] hover:text-[var(--color-primary)] hover:border-[var(--color-primary)] transition-colors"
              >
                <Plus className="w-3.5 h-3.5" />
                导入代理
              </button>
              <button
                onClick={testAll}
                disabled={testingIds.size > 0 || displayProxies.length === 0}
                className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-[var(--color-border)] text-[var(--color-text-secondary)] hover:text-[var(--color-primary)] hover:border-[var(--color-primary)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                <Wifi className="w-3.5 h-3.5" />
                全部测速
              </button>
            </div>

            <div className="flex-1 overflow-y-auto">
              {loading ? (
                <div className="flex items-center justify-center h-24 text-sm text-[var(--color-text-muted)]">加载中...</div>
              ) : displayProxies.length === 0 ? (
                <div className="flex items-center justify-center h-24 text-sm text-[var(--color-text-muted)]">暂无代理</div>
              ) : (
                displayProxies.map(item => (
                  <ProxyRow
                    key={item.proxy.proxyId}
                    proxy={item.proxy}
                    selected={item.proxy.proxyId === currentProxyId}
                    testing={testingIds.has(item.proxy.proxyId)}
                    speedResult={speedMap[item.proxy.proxyId]}
                    displayConfig={item.displayConfig}
                    onSelect={() => { onSelect(item.proxy, { force: profileRunning && forceDisconnect }); onClose() }}
                    onTest={e => testOne(item.proxy.proxyId, e)}
                    onEdit={e => handleEditClick(item.proxy, e)}
                    onDelete={e => handleDeleteClick(item.proxy, e)}
                  />
                ))
              )}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3 border-t border-[var(--color-border)] px-5 py-3 text-xs text-[var(--color-text-muted)]">
          <span>共 {displayProxies.length} 条，点击行即选中</span>
          {profileRunning && (
            <label className="ml-auto flex items-center gap-2 text-[var(--color-text-secondary)]">
              <Switch checked={forceDisconnect} onChange={setForceDisconnect} />
              <span>立即断开旧连接</span>
            </label>
          )}
          {onOpenRouting && (
            <button
              type="button"
              onClick={() => { onOpenRouting(); onClose() }}
              className={`${profileRunning ? '' : 'ml-auto'} flex items-center gap-1.5 rounded-lg border border-[var(--color-border)] px-2.5 py-1.5 text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-primary)] hover:text-[var(--color-primary)]`}
            >
              <GitBranch className="h-3.5 w-3.5" />
              分流规则
            </button>
          )}
        </div>
      </div>

      <ProxyImportModal
        open={importOpen}
        onClose={() => setImportOpen(false)}
        existingProxies={allProxies}
        groups={groups}
        onImported={handleImported}
      />

      <ProxyEditModal
        open={!!editingProxy}
        chainEditMode={chainEditMode}
        editName={editName}
        editConfig={editConfig}
        editGroup={editGroup}
        editDnsServers={editDnsServers}
        chainEditForm={chainEditForm}
        saving={savingEdit}
        setEditName={setEditName}
        setEditConfig={setEditConfig}
        setEditGroup={setEditGroup}
        setEditDnsServers={setEditDnsServers}
        setChainEditForm={setChainEditForm}
        updateChainHop={updateChainHop}
        onClose={closeEditModal}
        onSave={handleSaveEdit}
      />
      <ConfirmModal
        open={!!deleteCandidate}
        onClose={() => setDeleteCandidate(null)}
        onConfirm={handleDeleteConfirm}
        title="删除代理"
        content={`确认删除代理「${deleteCandidate?.proxyName || ''}」？`}
        confirmText="确认删除"
        cancelText="取消"
        danger
      />
    </div>,
    document.body
  )
}
