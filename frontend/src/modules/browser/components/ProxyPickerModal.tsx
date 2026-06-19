import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Check, Loader2, Pencil, Plus, Search, Trash2, Wifi, X } from 'lucide-react'
import { Button, ConfirmModal, FormItem, Input, Modal, Textarea, toast } from '../../../shared/components'
import type { BrowserProxy } from '../types'
import { browserProxyBatchTestSpeed, browserProxyTestSpeed, fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies } from '../api'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { ProxyImportModal } from './ProxyImportModal'

interface ProxyPickerModalProps {
  open: boolean
  currentProxyId: string
  onSelect: (proxy: BrowserProxy) => void
  onClose: () => void
  onProxyListUpdated?: (proxies: BrowserProxy[]) => void
  onProxyDeleted?: (deletedProxyId: string, nextProxies: BrowserProxy[]) => void
}

type SpeedResult = { ok: boolean; latencyMs: number; error: string }

type ChainSocksHop = {
  protocol?: string
  server?: string
  port?: number
  username?: string
  password?: string
}

type ChainSocksConfig = {
  localPort?: number
  first?: ChainSocksHop
  second?: ChainSocksHop
}

interface ChainHopForm {
  server: string
  port: string
  username: string
  password: string
}

interface ChainEditForm {
  proxyName: string
  localPort: string
  first: ChainHopForm
  second: ChainHopForm
}

const INITIAL_CHAIN_EDIT_FORM: ChainEditForm = {
  proxyName: '',
  localPort: '',
  first: { server: '', port: '', username: '', password: '' },
  second: { server: '', port: '', username: '', password: '' },
}

const LOCAL_PROXY_ID = '__local__'

function parseChainSocks5Config(proxyConfig: string): ChainSocksConfig | null {
  const cfg = proxyConfig.trim()
  if (!cfg.toLowerCase().startsWith(CHAIN_SOCKS5_PREFIX)) {
    return null
  }
  const encoded = cfg.slice(CHAIN_SOCKS5_PREFIX.length)
  if (!encoded) {
    return null
  }

  const normalizeHop = (raw: unknown): ChainSocksHop | null => {
    if (!raw || typeof raw !== 'object') return null
    const hop = raw as Record<string, unknown>
    const protocol = String(hop.protocol || '').trim().toLowerCase()
    if (protocol && protocol !== 'socks5') return null

    const server = String(hop.server || '').trim()
    if (!server) return null

    const portVal = Number(hop.port || 0)
    if (!Number.isInteger(portVal) || portVal < 1 || portVal > 65535) return null

    const username = String(hop.username || '').trim()
    const password = hop.password === undefined || hop.password === null ? '' : String(hop.password)
    if (password && !username) return null

    return {
      protocol: 'socks5',
      server,
      port: portVal,
      username: username || undefined,
      password: password || undefined,
    }
  }

  try {
    const decoded = decodeURIComponent(encoded)
    const parsed = JSON.parse(decoded) as Record<string, unknown>
    const first = normalizeHop(parsed.first)
    const second = normalizeHop(parsed.second)
    if (!first || !second) return null

    const localPortRaw = parsed.localPort
    const localPortNum = localPortRaw === undefined || localPortRaw === null || localPortRaw === ''
      ? 0
      : Number(localPortRaw)
    if (!Number.isInteger(localPortNum) || localPortNum < 0 || localPortNum > 65535) return null

    return {
      first,
      second,
      localPort: localPortNum > 0 ? localPortNum : undefined,
    }
  } catch {
    return null
  }
}

function toChainEditForm(proxyName: string, cfg: ChainSocksConfig): ChainEditForm {
  return {
    proxyName,
    localPort: cfg.localPort ? String(cfg.localPort) : '',
    first: {
      server: cfg.first?.server || '',
      port: cfg.first?.port ? String(cfg.first.port) : '',
      username: cfg.first?.username || '',
      password: cfg.first?.password || '',
    },
    second: {
      server: cfg.second?.server || '',
      port: cfg.second?.port ? String(cfg.second.port) : '',
      username: cfg.second?.username || '',
      password: cfg.second?.password || '',
    },
  }
}

function buildChainProxyConfig(form: ChainEditForm): string {
  const parseHop = (label: string, hop: ChainHopForm): ChainSocksHop => {
    const server = hop.server.trim()
    if (!server) {
      throw new Error(`请输入${label}代理地址`)
    }
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(server)) {
      throw new Error(`${label}代理地址只需要填写主机名或 IP，不需要协议头`)
    }

    const portInput = hop.port.trim()
    if (!portInput) {
      throw new Error(`请输入${label}代理端口`)
    }
    if (!/^\d+$/.test(portInput)) {
      throw new Error(`${label}代理端口必须为数字`)
    }

    const port = Number(portInput)
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error(`${label}代理端口必须在 1-65535 之间`)
    }

    const username = hop.username.trim()
    const password = hop.password
    if (password && !username) {
      throw new Error(`${label}填写密码时请同时填写账号`)
    }

    return {
      protocol: 'socks5',
      server,
      port,
      username: username || undefined,
      password: password || undefined,
    }
  }

  const localPortInput = form.localPort.trim()
  if (localPortInput && !/^\d+$/.test(localPortInput)) {
    throw new Error('本地监听端口必须为数字')
  }
  const localPort = localPortInput ? Number(localPortInput) : 0
  if (localPortInput && (!Number.isInteger(localPort) || localPort < 1 || localPort > 65535)) {
    throw new Error('本地监听端口必须在 1-65535 之间')
  }

  const payload: ChainSocksConfig = {
    first: parseHop('第一层', form.first),
    second: parseHop('第二层', form.second),
    localPort: localPort > 0 ? localPort : undefined,
  }

  const encodedPayload = encodeURIComponent(JSON.stringify(payload))
  return `${CHAIN_SOCKS5_PREFIX}${encodedPayload}`
}
const ALL_GROUP = '__all__'
const DIRECT_PROXY_ID = '__direct__'
const SPEED_RESULT_EVENT = 'proxy:speed:result'
const BATCH_TEST_CONCURRENCY = 20
const CHAIN_SOCKS5_PREFIX = 'chain+socks5://'

function formatProxyConfigForDisplay(proxyConfig: string): string {
  const raw = (proxyConfig || '').trim()
  if (!raw || !raw.toLowerCase().startsWith(CHAIN_SOCKS5_PREFIX)) {
    return raw
  }

  const encoded = raw.slice(CHAIN_SOCKS5_PREFIX.length)
  if (!encoded) return raw

  try {
    const decoded = decodeURIComponent(encoded)
    const parsed = JSON.parse(decoded) as ChainSocksConfig
    const firstServer = (parsed.first?.server || '').trim()
    const secondServer = (parsed.second?.server || '').trim()
    if (!firstServer || !secondServer) return raw
    return `${firstServer} -> ${secondServer}`
  } catch {
    return raw
  }
}


export function ProxyPickerModal({ open, currentProxyId, onSelect, onClose, onProxyListUpdated, onProxyDeleted }: ProxyPickerModalProps) {
  const [groups, setGroups] = useState<string[]>([])
  const [allProxies, setAllProxies] = useState<BrowserProxy[]>([])
  const [selectedGroup, setSelectedGroup] = useState<string>(ALL_GROUP)
  const [search, setSearch] = useState('')
  const [onlyAvailable, setOnlyAvailable] = useState(false)
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
    setOnlyAvailable(false)
    setSpeedMap({})
    setTestingIds(new Set())
    setEditingProxy(null)
    setDeleteCandidate(null)
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

    // 仅可用：保留实测成功（ok 且延迟>=0）或直连的代理
    if (onlyAvailable) {
      list = list.filter(proxy => {
        if (proxy.proxyConfig === 'direct://') return true
        const latest = speedMap[proxy.proxyId]
        const result = latest || (proxy.lastTestedAt
          ? { ok: proxy.lastTestOk ?? false, latencyMs: proxy.lastLatencyMs ?? -1 }
          : undefined)
        return !!result?.ok && result.latencyMs >= 0
      })
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
  }, [selectedGroup, search, onlyAvailable, allProxies, speedMap])

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
        setSpeedMap(prev => ({
          ...prev,
          [proxyId]: { ok: result.ok, latencyMs: result.latencyMs, error: result.error },
        }))
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
      setSpeedMap(prev => ({
        ...prev,
        [data.proxyId]: { ok: data.ok, latencyMs: data.latencyMs, error: data.error },
      }))
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
              next[result.proxyId] = { ok: result.ok, latencyMs: result.latencyMs, error: result.error }
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
    onSelect(selected)
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
        if (updated) onSelect(updated)
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
    if (proxy.proxyId === DIRECT_PROXY_ID || proxy.proxyId === LOCAL_PROXY_ID) return
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
          <span className="font-semibold text-[var(--color-text-primary)]">从代理池选择</span>
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
                onClick={() => setOnlyAvailable(v => !v)}
                title="仅显示实测可用或直连的代理"
                className={`shrink-0 flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border transition-colors ${
                  onlyAvailable
                    ? 'border-[var(--color-primary)] text-[var(--color-primary)] bg-[var(--color-primary)]/10'
                    : 'border-[var(--color-border)] text-[var(--color-text-secondary)] hover:text-[var(--color-primary)] hover:border-[var(--color-primary)]'
                }`}
              >
                仅可用
              </button>
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
                    onSelect={() => { onSelect(item.proxy); onClose() }}
                    onTest={e => testOne(item.proxy.proxyId, e)}
                    onEdit={e => handleEditClick(item.proxy, e)}
                    onDelete={e => handleDeleteClick(item.proxy, e)}
                  />
                ))
              )}
            </div>
          </div>
        </div>

        <div className="px-5 py-3 border-t border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
          共 {displayProxies.length} 条，点击行即选中
        </div>
      </div>

      <ProxyImportModal
        open={importOpen}
        onClose={() => setImportOpen(false)}
        existingProxies={allProxies}
        groups={groups}
        onImported={handleImported}
      />

      <Modal
        open={!!editingProxy}
        onClose={closeEditModal}
        title="编辑代理"
        width="520px"
        footer={
          <>
            <Button variant="secondary" onClick={closeEditModal} disabled={savingEdit}>取消</Button>
            <Button onClick={handleSaveEdit} loading={savingEdit}>保存</Button>
          </>
        }
      >
        <div className="space-y-3">
          <FormItem label="代理名称" required>
            <Input
              value={chainEditMode ? chainEditForm.proxyName : editName}
              onChange={e => {
                if (chainEditMode) {
                  setChainEditForm(prev => ({ ...prev, proxyName: e.target.value }))
                } else {
                  setEditName(e.target.value)
                }
              }}
              placeholder="例如：香港节点"
            />
          </FormItem>

          <FormItem label="分组名称（可选）">
            <Input value={editGroup} onChange={e => setEditGroup(e.target.value)} placeholder="例如：香港、美国" />
          </FormItem>

          {chainEditMode ? (
            <div className="space-y-3 rounded-md border border-[var(--color-border)] p-3">
              <FormItem label="本地监听端口（可选）">
                <Input
                  type="number"
                  min={1}
                  max={65535}
                  value={chainEditForm.localPort}
                  onChange={e => setChainEditForm(prev => ({ ...prev, localPort: e.target.value }))}
                  placeholder="留空自动分配"
                />
              </FormItem>

              <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
                <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第一层 SOCKS5</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormItem label="代理地址" required>
                    <Input value={chainEditForm.first.server} onChange={e => updateChainHop('first', 'server', e.target.value)} />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input type="number" min={1} max={65535} value={chainEditForm.first.port} onChange={e => updateChainHop('first', 'port', e.target.value)} />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input value={chainEditForm.first.username} onChange={e => updateChainHop('first', 'username', e.target.value)} />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input type="password" value={chainEditForm.first.password} onChange={e => updateChainHop('first', 'password', e.target.value)} />
                  </FormItem>
                </div>
              </div>

              <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
                <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第二层 SOCKS5</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormItem label="代理地址" required>
                    <Input value={chainEditForm.second.server} onChange={e => updateChainHop('second', 'server', e.target.value)} />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input type="number" min={1} max={65535} value={chainEditForm.second.port} onChange={e => updateChainHop('second', 'port', e.target.value)} />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input value={chainEditForm.second.username} onChange={e => updateChainHop('second', 'username', e.target.value)} />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input type="password" value={chainEditForm.second.password} onChange={e => updateChainHop('second', 'password', e.target.value)} />
                  </FormItem>
                </div>
              </div>
            </div>
          ) : (
            <FormItem label="代理配置" required>
              <Textarea
                value={editConfig}
                onChange={e => setEditConfig(e.target.value)}
                rows={6}
                placeholder="支持 http://、https://、socks5://、chain+socks5://"
              />
            </FormItem>
          )}

          <FormItem label="DNS 服务器（可选）">
            <Textarea
              value={editDnsServers}
              onChange={e => setEditDnsServers(e.target.value)}
              rows={4}
              placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`}
            />
          </FormItem>
        </div>
      </Modal>

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

function GroupItem({ label, active, count, onClick }: { label: string; active: boolean; count: number; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left px-3 py-2 text-sm flex items-center justify-between gap-2 transition-colors ${
        active
          ? 'bg-[var(--color-primary)]/10 text-[var(--color-primary)] font-medium'
          : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]'
      }`}
    >
      <span className="truncate">{label}</span>
      <span className="text-xs opacity-60 shrink-0">{count}</span>
    </button>
  )
}

interface ProxyRowProps {
  proxy: BrowserProxy
  selected: boolean
  testing: boolean
  speedResult?: SpeedResult
  displayConfig: string
  onSelect: () => void
  onTest: (e: React.MouseEvent) => void
  onEdit: (e: React.MouseEvent) => void
  onDelete: (e: React.MouseEvent) => void
}

function SpeedBadge({ testing, result }: { testing: boolean; result?: SpeedResult }) {
  if (testing) return <Loader2 className="w-3.5 h-3.5 animate-spin text-[var(--color-text-muted)] shrink-0" />
  if (!result) return null
  if (!result.ok) return <span className="text-xs text-red-500 shrink-0">失败</span>
  const color = result.latencyMs < 200 ? 'text-green-500' : result.latencyMs < 500 ? 'text-yellow-500' : 'text-red-500'
  return <span className={`text-xs font-medium shrink-0 ${color}`}>{result.latencyMs}ms</span>
}

function ProxyRow({ proxy, selected, testing, speedResult, displayConfig, onSelect, onTest, onEdit, onDelete }: ProxyRowProps) {
  const isDirect = proxy.proxyId === DIRECT_PROXY_ID
  const isLocal = proxy.proxyId === LOCAL_PROXY_ID
  const disableDelete = isDirect || isLocal

  return (
    <div
      onClick={onSelect}
      className={`w-full px-4 py-2.5 flex items-center gap-3 cursor-pointer transition-colors border-b border-[var(--color-border)]/40 last:border-0 overflow-hidden ${
        selected ? 'bg-[var(--color-primary)]/10' : 'hover:bg-[var(--color-bg-hover)]'
      }`}
    >
      <div className="flex-1 min-w-0 overflow-hidden">
        <div className="text-sm font-medium text-[var(--color-text-primary)] truncate">
          {proxy.proxyName || proxy.proxyId}
          {proxy.groupName && <span className="ml-2 text-xs text-[var(--color-primary)]/70 font-normal">[{proxy.groupName}]</span>}
        </div>
        <div className="text-xs text-[var(--color-text-muted)] truncate mt-0.5 w-0 min-w-full">
          {displayConfig}
        </div>
      </div>
      <SpeedBadge testing={testing} result={speedResult} />
      <button
        onClick={onTest}
        disabled={testing}
        title="测速"
        className="shrink-0 p-1 rounded text-[var(--color-text-muted)] hover:text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 disabled:opacity-40 transition-colors"
      >
        <Wifi className="w-3.5 h-3.5" />
      </button>
      <button
        onClick={onEdit}
        disabled={isDirect}
        title={isDirect ? '直连不可编辑' : '编辑代理'}
        className="shrink-0 p-1 rounded text-[var(--color-text-muted)] hover:text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
      >
        <Pencil className="w-3.5 h-3.5" />
      </button>
      <button
        onClick={onDelete}
        disabled={disableDelete}
        title={isDirect ? '直连不可删除' : isLocal ? '本地代理不可删除' : '删除代理'}
        className="shrink-0 p-1 rounded text-[var(--color-text-muted)] hover:text-red-500 hover:bg-red-500/10 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
      {selected && <Check className="w-4 h-4 text-[var(--color-primary)] shrink-0" />}
    </div>
  )
}
