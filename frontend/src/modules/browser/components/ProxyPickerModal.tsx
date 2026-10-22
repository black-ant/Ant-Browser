import { useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { Search, X } from 'lucide-react'
import type { BrowserProxy, BrowserProxyVirtualGroup } from '../types'
import { fetchBrowserProxies, fetchBrowserProxyGroups, fetchBrowserProxyVirtualGroups } from '../api'
import { GroupItem, ProxyRow } from './ProxyPickerModal.rows'
import { ALL_GROUP, DIRECT_PROXY_ID, formatProxyConfigForDisplay } from './ProxyPickerModal.helpers'

const DIRECT_GROUP = '__direct_proxy__'
const VIRTUAL_GROUP_PREFIX = '__virtual_proxy_group__:'

function getVirtualGroupKey(group: BrowserProxyVirtualGroup) {
  return `${VIRTUAL_GROUP_PREFIX}${group.groupId.trim() || group.groupName.trim().toLowerCase()}`
}
const BUILTIN_DIRECT_PROXY: BrowserProxy = {
  proxyId: DIRECT_PROXY_ID,
  proxyName: '\u76f4\u8fde\uff08\u4e0d\u8d70\u4ee3\u7406\uff09',
  proxyConfig: 'direct://',
}

function ensureBuiltinDirectProxy(proxies: BrowserProxy[]): BrowserProxy[] {
  if (proxies.some(proxy => proxy.proxyId === DIRECT_PROXY_ID)) {
    return proxies
  }
  return [BUILTIN_DIRECT_PROXY, ...proxies]
}

interface ProxyPickerModalProps {
  open: boolean
  currentProxyId: string
  title?: string
  showVirtualGroups?: boolean
  onSelect: (proxy: BrowserProxy) => void
  onClose: () => void
  onProxyListUpdated?: (proxies: BrowserProxy[]) => void
}

export function ProxyPickerModal({ open, currentProxyId, title = '从代理池选择', showVirtualGroups = false, onSelect, onClose, onProxyListUpdated }: ProxyPickerModalProps) {
  const [groups, setGroups] = useState<string[]>([])
  const [virtualGroups, setVirtualGroups] = useState<BrowserProxyVirtualGroup[]>([])
  const [allProxies, setAllProxies] = useState<BrowserProxy[]>([])
  const [selectedGroup, setSelectedGroup] = useState<string>(ALL_GROUP)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)

  const loadData = async () => {
    setLoading(true)
    try {
      const [groupList, virtualGroupList, proxyList] = await Promise.all([
        fetchBrowserProxyGroups(),
        fetchBrowserProxyVirtualGroups(),
        fetchBrowserProxies(),
      ])
      const nextProxies = ensureBuiltinDirectProxy(proxyList)
      setGroups(groupList)
      setVirtualGroups(virtualGroupList)
      setAllProxies(nextProxies)
      onProxyListUpdated?.(nextProxies)
      return nextProxies
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!open) return
    setSelectedGroup(ALL_GROUP)
    setSearch('')
    void loadData()
  }, [open])

  const virtualGroupMembers = useMemo(() => {
    const result = new Map<string, Set<string>>()
    virtualGroups.forEach(group => {
      const memberIDs = new Set((group.memberProxyIds || []).map(proxyId => proxyId.trim().toLowerCase()).filter(Boolean))
      const memberGroupNames = new Set((group.memberGroupNames || []).map(groupName => groupName.trim().toLowerCase()).filter(Boolean))
      allProxies.forEach(proxy => {
        if (memberGroupNames.has((proxy.groupName || '').trim().toLowerCase())) {
          memberIDs.add(proxy.proxyId.trim().toLowerCase())
        }
      })
      result.set(getVirtualGroupKey(group), memberIDs)
    })
    return result
  }, [allProxies, virtualGroups])

  const displayProxies = useMemo(() => {
    let list = allProxies
    if (selectedGroup === DIRECT_GROUP) {
      list = list.filter(proxy => proxy.proxyId === DIRECT_PROXY_ID)
    } else if (selectedGroup.startsWith(VIRTUAL_GROUP_PREFIX)) {
      const memberIDs = virtualGroupMembers.get(selectedGroup) || new Set<string>()
      list = list.filter(proxy => memberIDs.has(proxy.proxyId.trim().toLowerCase()))
    } else if (selectedGroup !== ALL_GROUP) {
      list = list.filter(proxy => proxy.groupName === selectedGroup)
    }
    if (search.trim()) {
      const query = search.trim().toLowerCase()
      list = list.filter(proxy =>
        (proxy.proxyName || '').toLowerCase().includes(query) ||
        (proxy.proxyConfig || '').toLowerCase().includes(query)
      )
    }

    return list.map(proxy => ({
        proxy,
        displayConfig: formatProxyConfigForDisplay(proxy.proxyConfig),
      }))
  }, [selectedGroup, search, allProxies, virtualGroupMembers])

  const groupCounts = useMemo(() => {
    const counts = new Map<string, number>()
    allProxies.forEach(proxy => {
      const key = proxy.groupName || ''
      counts.set(key, (counts.get(key) || 0) + 1)
    })
    return counts
  }, [allProxies])

  const virtualGroupCounts = useMemo(() => new Map(
    Array.from(virtualGroupMembers.entries()).map(([groupKey, memberIDs]) => [
      groupKey,
      allProxies.filter(proxy => memberIDs.has(proxy.proxyId.trim().toLowerCase())).length,
    ]),
  ), [allProxies, virtualGroupMembers])

  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center" onClick={onClose}>
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" />
      <div
        className="relative bg-[var(--color-bg-elevated)] border border-[var(--color-border)] rounded-xl shadow-2xl w-[720px] max-h-[580px] flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
          <span className="font-semibold text-[var(--color-text-primary)]">{title}</span>
          <button onClick={onClose} className="text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex flex-1 min-h-0">
          <div className="w-44 border-r border-[var(--color-border)] flex flex-col py-2 overflow-y-auto shrink-0 bg-[var(--color-bg-muted)]">
            <GroupItem label="全部" active={selectedGroup === ALL_GROUP} count={allProxies.length} onClick={() => setSelectedGroup(ALL_GROUP)} />
            <GroupItem
              label={'\u4e0d\u4f7f\u7528\u4ee3\u7406'}
              active={selectedGroup === DIRECT_GROUP}
              count={allProxies.some(proxy => proxy.proxyId === DIRECT_PROXY_ID) ? 1 : 0}
              onClick={() => {
                setSelectedGroup(DIRECT_GROUP)
                setSearch('')
              }}
            />
            {groups.map(groupName => (
              <GroupItem
                key={groupName}
                label={groupName}
                active={selectedGroup === groupName}
                count={groupCounts.get(groupName) || 0}
                onClick={() => setSelectedGroup(groupName)}
              />
            ))}
            {showVirtualGroups && virtualGroups.map(group => {
              const groupKey = getVirtualGroupKey(group)
              return (
                <GroupItem
                  key={groupKey}
                  label={`虚拟：${group.groupName}`}
                  active={selectedGroup === groupKey}
                  count={virtualGroupCounts.get(groupKey) || 0}
                  onClick={() => setSelectedGroup(groupKey)}
                />
              )
            })}
            {groups.length === 0 && (!showVirtualGroups || virtualGroups.length === 0) && (
              <p className="px-3 py-2 text-xs text-[var(--color-text-muted)]">暂无分组</p>
            )}
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
                    displayConfig={item.displayConfig}
                    onSelect={() => { onSelect(item.proxy); onClose() }}
                  />
                ))
              )}
            </div>
          </div>
        </div>

        <div className="px-4 py-2.5 border-t border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
          共 {displayProxies.length} 条，点击行即选中
        </div>
      </div>

    </div>,
    document.body
  )
}
