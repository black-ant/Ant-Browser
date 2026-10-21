import { useMemo } from 'react'
import type { SortOrder } from '../../../../shared/components/Table'
import type { BrowserProxyVirtualGroup, ProxyIPHealthResult } from '../../types'
import type { ProxyDisplayInfo } from './helpers'
import { isProxySpeedResultFresh } from './storage'

interface UseProxyPoolFilterOptions {
  displayList: ProxyDisplayInfo[]
  filterProtocol: string
  filterKeyword: string
  filterGroup: string
  filterAvailableOnly: boolean
  virtualGroups: BrowserProxyVirtualGroup[]
  sortColumn: string
  sortOrder: SortOrder
  latencyMap: Record<string, number>
  latencyFingerprintMap: Record<string, string>
  latencyTestedAtMap: Record<string, string>
  now: number
  speedResultMaxAgeMs: number
  ipHealthMap: Record<string, ProxyIPHealthResult>
}

const compareText = (a: string, b: string) => a.localeCompare(b, 'zh-CN')

function getLatencySortTuple(
  latencyMap: Record<string, number>,
  latencyFingerprintMap: Record<string, string>,
  latencyTestedAtMap: Record<string, string>,
  proxy: ProxyDisplayInfo,
  now: number,
  speedResultMaxAgeMs: number,
): [number, number] {
  const latency = hasCurrentSpeedResult(
    latencyMap,
    latencyFingerprintMap,
    latencyTestedAtMap,
    proxy,
    now,
    speedResultMaxAgeMs,
  ) ? latencyMap[proxy.proxyId] : undefined
  if (latency === undefined) return [5, Number.MAX_SAFE_INTEGER]
  if (latency === -1) return [1, Number.MAX_SAFE_INTEGER]
  if (latency === -2) return [2, Number.MAX_SAFE_INTEGER]
  if (latency === -3) return [3, Number.MAX_SAFE_INTEGER]
  if (latency === -4) return [4, Number.MAX_SAFE_INTEGER]
  return [0, latency]
}

function isProxyAvailable(
  proxyId: string,
  latencyMap: Record<string, number>,
  latencyFingerprintMap: Record<string, string>,
  latencyTestedAtMap: Record<string, string>,
  proxy: ProxyDisplayInfo,
  now: number,
  speedResultMaxAgeMs: number,
  ipHealthMap: Record<string, ProxyIPHealthResult>,
) {
  const latency = hasCurrentSpeedResult(
    latencyMap,
    latencyFingerprintMap,
    latencyTestedAtMap,
    proxy,
    now,
    speedResultMaxAgeMs,
  ) ? latencyMap[proxyId] : undefined
  return (typeof latency === 'number' && latency >= 0) || Boolean(ipHealthMap[proxyId]?.ok)
}

function hasCurrentSpeedResult(
  latencyMap: Record<string, number>,
  latencyFingerprintMap: Record<string, string>,
  latencyTestedAtMap: Record<string, string>,
  proxy: ProxyDisplayInfo,
  now: number,
  speedResultMaxAgeMs: number,
) {
  const fingerprint = (proxy.speedTestFingerprint || '').trim()
  const cachedFingerprint = (latencyFingerprintMap[proxy.proxyId] || '').trim()
  return Boolean(fingerprint)
    && fingerprint === cachedFingerprint
    && isProxySpeedResultFresh(latencyTestedAtMap[proxy.proxyId], now, speedResultMaxAgeMs)
    && typeof latencyMap[proxy.proxyId] === 'number'
}

function compareByColumn(
  latencyMap: Record<string, number>,
  latencyFingerprintMap: Record<string, string>,
  latencyTestedAtMap: Record<string, string>,
  a: ProxyDisplayInfo,
  b: ProxyDisplayInfo,
  column: string,
  now: number,
  speedResultMaxAgeMs: number,
) {
  switch (column) {
    case 'proxyName':
      return compareText(a.proxyName || '', b.proxyName || '')
    case 'groupName':
      return compareText(a.groupName || '', b.groupName || '')
    case 'type':
      return compareText(a.type || '', b.type || '')
    case 'server':
      return compareText(a.server || '', b.server || '')
    case 'port':
      return (a.port || 0) - (b.port || 0)
    case 'latency': {
      const [rankA, valA] = getLatencySortTuple(latencyMap, latencyFingerprintMap, latencyTestedAtMap, a, now, speedResultMaxAgeMs)
      const [rankB, valB] = getLatencySortTuple(latencyMap, latencyFingerprintMap, latencyTestedAtMap, b, now, speedResultMaxAgeMs)
      if (rankA !== rankB) return rankA - rankB
      if (valA !== valB) return valA - valB
      return compareText(a.proxyName || '', b.proxyName || '')
    }
    default:
      return 0
  }
}

export function useProxyPoolFilter(options: UseProxyPoolFilterOptions) {
  const {
    displayList,
    filterProtocol,
    filterKeyword,
    filterGroup,
    filterAvailableOnly,
    virtualGroups,
    sortColumn,
    sortOrder,
    latencyMap,
    latencyFingerprintMap,
    latencyTestedAtMap,
    now,
    speedResultMaxAgeMs,
    ipHealthMap,
  } = options

  const protocolOptions = useMemo(
    () => ['all', ...Array.from(new Set(displayList.map(p => p.type).filter(t => t !== '-')))],
    [displayList],
  )

  const filteredList = useMemo(() => {
    const keyword = filterKeyword.toLowerCase()
    const selectedVirtualGroup = virtualGroups.find(group => {
      const key = group.groupId.trim() || group.groupName.trim().toLowerCase()
      return filterGroup === `__virtual_proxy_group__:${key}`
    })
    const virtualMemberIDs = new Set((selectedVirtualGroup?.memberProxyIds || []).map(proxyId => proxyId.trim().toLowerCase()))
    const virtualMemberGroupNames = new Set((selectedVirtualGroup?.memberGroupNames || []).map(groupName => groupName.trim().toLowerCase()))
    const filtered = displayList.filter(p => {
      const matchProtocol = filterProtocol === 'all' || p.type === filterProtocol
      const matchKeyword = !keyword || p.proxyName.toLowerCase().includes(keyword) || p.server.toLowerCase().includes(keyword)
      const matchGroup = selectedVirtualGroup
        ? p.proxyConfig !== 'direct://'
          && (virtualMemberIDs.has(p.proxyId.trim().toLowerCase()) || virtualMemberGroupNames.has(p.groupName.trim().toLowerCase()))
        : filterGroup === 'all' || p.groupName === filterGroup
      const matchAvailable = !filterAvailableOnly || isProxyAvailable(
        p.proxyId,
        latencyMap,
        latencyFingerprintMap,
        latencyTestedAtMap,
        p,
        now,
        speedResultMaxAgeMs,
        ipHealthMap,
      )
      return matchProtocol && matchKeyword && matchGroup && matchAvailable
    })

    if (!sortColumn || !sortOrder) return filtered
    return [...filtered].sort((a, b) => {
      const cmp = compareByColumn(latencyMap, latencyFingerprintMap, latencyTestedAtMap, a, b, sortColumn, now, speedResultMaxAgeMs)
      return sortOrder === 'asc' ? cmp : -cmp
    })
  }, [
    displayList,
    filterProtocol,
    filterKeyword,
    filterGroup,
    filterAvailableOnly,
    sortColumn,
    sortOrder,
    latencyMap,
    latencyFingerprintMap,
    latencyTestedAtMap,
    now,
    speedResultMaxAgeMs,
    ipHealthMap,
    virtualGroups,
  ])

  return { protocolOptions, filteredList }
}
