import { useEffect, useMemo, useState } from 'react'

import { Button, Card, Input, Switch, Table } from '../../../../shared/components'
import type { SortOrder, TableColumn } from '../../../../shared/components/Table'
import type { ProxyIPHealthResult } from '../../types'

import { BUILTIN_PROXY_IDS, sourceHostLabel, type ProxyDisplayInfo } from './helpers'
import { BrowserEditSelect } from '../../components/BrowserEditSelect'

interface ProxyPoolTableCardProps {
  allFilteredSelected: boolean
  checkingIPHealthIds: Set<string>
  data: ProxyDisplayInfo[]
  filterGroup: string
  filterKeyword: string
  filterProtocol: string
  filterAvailableOnly: boolean
  globalAutoRefreshEnabled: boolean
  globalRefreshInterval: number
  globalRefreshIntervalM: string
  groups: string[]
  virtualGroupOptions: Array<{ value: string; label: string }>
  ipHealthMap: Record<string, ProxyIPHealthResult>
  loading: boolean
  onCheckOneIPHealth: (record: ProxyDisplayInfo) => void
  onClearFilters: () => void
  onDelete: (proxyId: string) => void
  onEdit: (record: ProxyDisplayInfo) => void
  onFilterGroupChange: (nextValue: string) => void
  onFilterKeywordChange: (nextValue: string) => void
  onFilterProtocolChange: (nextValue: string) => void
  onFilterAvailableOnlyChange: (checked: boolean) => void
  onGlobalAutoRefreshEnabledChange: (checked: boolean) => void
  onGlobalRefreshIntervalMChange: (nextValue: string) => void
  onOpenBatchDelete: () => void
  onOpenIPHealthDetail: (proxyId: string) => void
  onRefreshSingleSource: (sourceId: string) => void
  onSort: (next: { column: string; order: SortOrder }) => void
  onTestOne: (record: ProxyDisplayInfo) => void
  onToggleAll: () => void
  onToggleOne: (proxyId: string) => void
  protocolOptions: string[]
  refreshingSourceIds: Set<string>
  selectedCount: number
  selectedIds: Set<string>
  someFilteredSelected: boolean
  sortColumn: string
  sortOrder: SortOrder
  latencyMap: Record<string, number>
  latencyEngineMap: Record<string, string>
  latencyErrorMap: Record<string, string>
}

export function ProxyPoolTableCard({
  allFilteredSelected,
  checkingIPHealthIds,
  data,
  filterGroup,
  filterKeyword,
  filterProtocol,
  filterAvailableOnly,
  globalAutoRefreshEnabled,
  globalRefreshInterval,
  globalRefreshIntervalM,
  groups,
  virtualGroupOptions,
  ipHealthMap,
  loading,
  onCheckOneIPHealth,
  onClearFilters,
  onDelete,
  onEdit,
  onFilterGroupChange,
  onFilterKeywordChange,
  onFilterProtocolChange,
  onFilterAvailableOnlyChange,
  onGlobalAutoRefreshEnabledChange,
  onGlobalRefreshIntervalMChange,
  onOpenBatchDelete,
  onOpenIPHealthDetail,
  onRefreshSingleSource,
  onSort,
  onTestOne,
  onToggleAll,
  onToggleOne,
  protocolOptions,
  refreshingSourceIds,
  selectedCount,
  selectedIds,
  someFilteredSelected,
  sortColumn,
  sortOrder,
  latencyMap,
  latencyEngineMap,
  latencyErrorMap,
}: ProxyPoolTableCardProps) {
  const hasActiveFilters = filterProtocol !== 'all' || !!filterKeyword || filterGroup !== 'all' || filterAvailableOnly
  const [openMoreProxyId, setOpenMoreProxyId] = useState<string | null>(null)
  const [moreMenuPlacement, setMoreMenuPlacement] = useState<'up' | 'down'>('down')

  useEffect(() => {
    const closeMoreMenu = () => {
      setOpenMoreProxyId(null)
      setMoreMenuPlacement('down')
    }
    document.addEventListener('click', closeMoreMenu)
    return () => document.removeEventListener('click', closeMoreMenu)
  }, [])

  const resolveMoreMenuPlacement = (button: HTMLElement, menuHeight: number) => {
    const buttonRect = button.getBoundingClientRect()
    const scrollParent = button.closest('.overflow-auto')
    const scrollParentRect = scrollParent?.getBoundingClientRect()
    const boundaryTop = Math.max(0, scrollParentRect?.top ?? 0)
    const boundaryBottom = Math.min(window.innerHeight, scrollParentRect?.bottom ?? window.innerHeight)
    const availableAbove = buttonRect.top - boundaryTop
    const availableBelow = boundaryBottom - buttonRect.bottom
    return availableBelow < menuHeight && availableAbove > availableBelow ? 'up' : 'down'
  }

  const renderLatency = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    const value = latencyMap[record.proxyId]
    if (value === undefined) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (value === -1) return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">测试中...</span>
    const error = latencyErrorMap[record.proxyId] || ''
    if (value === -2) return <span className="text-red-500 text-xs" title={error || '测速超时'}>超时</span>
    if (value === -3) return <span className="text-gray-400 text-xs" title={error || '协议不支持'}>不支持</span>
    if (value === -4) return <span className="text-red-500 text-xs" title={error || '测速失败'}>失败</span>
    const color = value < 200 ? 'text-green-500' : value < 500 ? 'text-yellow-500' : 'text-red-500'
    return <span className={`text-xs font-medium ${color}`}>{value} ms</span>
  }

  const renderLatencyEngine = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    const value = latencyMap[record.proxyId]
    if (value === undefined) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (value === -1) return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">-</span>
    return latencyEngineMap[record.proxyId]
      ? <span className="text-xs text-[var(--color-text-secondary)] whitespace-nowrap">{latencyEngineMap[record.proxyId]}</span>
      : <span className="text-[var(--color-text-muted)] text-xs">-</span>
  }

  const renderIPHealth = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    if (checkingIPHealthIds.has(record.proxyId)) {
      return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">检测中...</span>
    }

    const result = ipHealthMap[record.proxyId]
    if (!result) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (!result.ok) {
      return (
        <div className="flex items-center gap-2">
          <span className="text-xs text-red-500 truncate max-w-[120px]" title={result.error || '检测失败'}>失败</span>
          <Button size="sm" variant="ghost" onClick={(event) => { event.stopPropagation(); onOpenIPHealthDetail(record.proxyId) }}>原始</Button>
        </div>
      )
    }

    const location = [result.country, result.region, result.city].filter(Boolean).join(' / ')
    return (
      <div className="flex items-center gap-2 min-w-0">
        <div className="min-w-0">
          <div className="text-xs text-[var(--color-text-primary)] truncate">{result.ip || '-'}</div>
          <div className="text-[11px] text-[var(--color-text-muted)] truncate">
            {`fraud ${result.fraudScore} | ${result.isResidential ? '住宅' : '机房'}${location ? ` | ${location}` : ''}`}
          </div>
        </div>
        <Button size="sm" variant="ghost" onClick={(event) => { event.stopPropagation(); onOpenIPHealthDetail(record.proxyId) }}>原始</Button>
      </div>
    )
  }

  const columns = useMemo<TableColumn<ProxyDisplayInfo>[]>(() => [
    {
      key: 'checkbox',
      title: '',
      width: '40px',
      render: (_, record) => (
        <input
          type="checkbox"
          checked={selectedIds.has(record.proxyId)}
          disabled={BUILTIN_PROXY_IDS.has(record.proxyId)}
          onChange={() => onToggleOne(record.proxyId)}
          onClick={event => event.stopPropagation()}
          className="w-4 h-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)] cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
        />
      ),
    },
    { key: 'proxyName', title: '代理名称', width: '180px', sortable: true },
    {
      key: 'groupName',
      title: '分组',
      width: '100px',
      sortable: true,
      render: (value) => value ? <span className="px-1.5 py-0.5 text-xs rounded bg-[var(--color-accent)]/10 text-[var(--color-accent)]">{value}</span> : '-',
    },
    {
      key: 'source',
      title: '来源',
      width: '180px',
      render: (_, record) => {
        if (!record.sourceUrl) return '-'
        const host = sourceHostLabel(record.sourceUrl)
        return (
          <div className="text-xs leading-5">
            <div className="text-[var(--color-text-primary)] truncate" title={record.sourceUrl}>{host}</div>
            <div className="text-[var(--color-text-muted)]">
              {globalAutoRefreshEnabled ? `自动刷新 ${globalRefreshInterval} 分钟（全局）` : '手动刷新'}
            </div>
          </div>
        )
      },
    },
    { key: 'type', title: '类型', width: '90px', sortable: true },
    { key: 'server', title: '服务器', width: '180px', sortable: true },
    { key: 'port', title: '端口', width: '80px', sortable: true, render: (value) => value || '-' },
    {
      key: 'latency',
      title: '延迟',
      width: '90px',
      sortable: true,
      render: (_, record) => renderLatency(record),
    },
    {
      key: 'latencyEngine',
      title: '测速类型',
      width: '90px',
      render: (_, record) => renderLatencyEngine(record),
    },
    {
      key: 'ipHealth',
      title: (
        <div className="leading-tight">
          <div>IP健康</div>
          <div className="mt-0.5 text-[10px] font-normal text-[var(--color-text-muted)]">
            仅供参考
          </div>
        </div>
      ),
      width: '280px',
      render: (_, record) => renderIPHealth(record),
    },
    {
      key: 'actions',
      title: '操作',
      width: '190px',
      render: (_, record) => {
        const isBuiltin = BUILTIN_PROXY_IDS.has(record.proxyId)
        const sourceId = record.sourceId || ''
        const hasSource = !!sourceId && !!record.sourceUrl
        const moreOpen = openMoreProxyId === record.proxyId
        const closeMore = () => setOpenMoreProxyId(null)
        return (
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={(event) => { event.stopPropagation(); closeMore(); onTestOne(record) }}
              loading={latencyMap[record.proxyId] === -1}
              disabled={record.proxyConfig === 'direct://'}
            >
              测速
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={isBuiltin}
              title={isBuiltin ? '内置代理不可编辑' : undefined}
              onClick={(event) => {
                event.stopPropagation()
                closeMore()
                if (!isBuiltin) onEdit(record)
              }}
            >
              编辑
            </Button>
            <div className="relative" onClick={(event) => event.stopPropagation()}>
              <Button
                size="sm"
                variant="secondary"
                aria-expanded={moreOpen}
                onClick={(event) => {
                  event.stopPropagation()
                  if (moreOpen) {
                    closeMore()
                    return
                  }
                  setMoreMenuPlacement(resolveMoreMenuPlacement(event.currentTarget, hasSource ? 132 : 96))
                  setOpenMoreProxyId(record.proxyId)
                }}
              >
                更多
              </Button>
              {moreOpen && (
                <div className={`absolute right-0 z-30 w-28 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-1 shadow-lg ${moreMenuPlacement === 'up' ? 'bottom-9' : 'top-9'}`}>
                  {hasSource && (
                    <Button
                      size="sm"
                      variant="ghost"
                      className="w-full justify-start"
                      onClick={(event) => { event.stopPropagation(); closeMore(); onRefreshSingleSource(sourceId) }}
                      loading={refreshingSourceIds.has(sourceId)}
                    >
                      刷新订阅
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="ghost"
                    className="w-full justify-start"
                    onClick={(event) => { event.stopPropagation(); closeMore(); onCheckOneIPHealth(record) }}
                    loading={checkingIPHealthIds.has(record.proxyId)}
                    disabled={record.proxyConfig === 'direct://'}
                  >
                    IP健康
                  </Button>
                  <Button
                    size="sm"
                    variant="danger"
                    className="w-full justify-start"
                    disabled={isBuiltin}
                    title={isBuiltin ? '内置代理不可删除' : undefined}
                    onClick={(event) => {
                      event.stopPropagation()
                      closeMore()
                      if (!isBuiltin) onDelete(record.proxyId)
                    }}
                  >
                    删除
                  </Button>
                </div>
              )}
            </div>
          </div>
        )
      },
    },
  ], [
    checkingIPHealthIds,
    globalAutoRefreshEnabled,
    globalRefreshInterval,
    ipHealthMap,
    latencyMap,
    latencyEngineMap,
    onCheckOneIPHealth,
    onDelete,
    onEdit,
    onOpenIPHealthDetail,
    onRefreshSingleSource,
    openMoreProxyId,
    moreMenuPlacement,
    onTestOne,
    onToggleOne,
    refreshingSourceIds,
    selectedIds,
  ])

  return (
    <Card>
      <div className="flex items-center gap-3 mb-4">
        <Input
          value={filterKeyword}
          onChange={event => onFilterKeywordChange(event.target.value)}
          placeholder="搜索名称或服务器..."
          style={{ width: '220px' }}
        />
        <BrowserEditSelect
          label="代理协议筛选"
          value={filterProtocol}
          onChange={onFilterProtocolChange}
          options={protocolOptions.map(protocol => ({
            value: protocol,
            label: protocol === 'all' ? '全部协议' : protocol.toUpperCase(),
          }))}
          className="w-40 shrink-0"
        />
        <BrowserEditSelect
          label="代理分组筛选"
          value={filterGroup}
          onChange={onFilterGroupChange}
          options={[
            { value: 'all', label: '全部分组' },
            ...groups.map(group => ({ value: group, label: group })),
            ...virtualGroupOptions,
          ]}
          className="w-48 shrink-0"
        />
        {hasActiveFilters && (
          <Button size="sm" variant="ghost" onClick={onClearFilters}>清除筛选</Button>
        )}
        <label className="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)] cursor-pointer select-none">
          <input
            type="checkbox"
            checked={filterAvailableOnly}
            onChange={event => onFilterAvailableOnlyChange(event.target.checked)}
            className="w-4 h-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)] cursor-pointer"
          />
          只展示可用
        </label>
        <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-2 py-1.5">
          <span className="text-xs text-[var(--color-text-muted)]">全局自动刷新</span>
          <Switch
            checked={globalAutoRefreshEnabled}
            onChange={onGlobalAutoRefreshEnabledChange}
          />
          <Input
            type="number"
            min={5}
            max={1440}
            value={globalRefreshIntervalM}
            onChange={event => onGlobalRefreshIntervalMChange(event.target.value)}
            className="w-24"
            disabled={!globalAutoRefreshEnabled}
          />
          <span className="text-xs text-[var(--color-text-muted)]">分钟</span>
        </div>
        <div className="flex-1" />
        {data.length > 0 && (
          <label className="flex items-center gap-1.5 text-sm text-[var(--color-text-muted)] cursor-pointer select-none">
            <input
              type="checkbox"
              checked={allFilteredSelected}
              ref={(element) => {
                if (element) {
                  element.indeterminate = someFilteredSelected && !allFilteredSelected
                }
              }}
              onChange={onToggleAll}
              className="w-4 h-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)] cursor-pointer"
            />
            全选
          </label>
        )}
        {selectedCount > 0 && (
          <Button size="sm" variant="danger" onClick={onOpenBatchDelete}>
            删除所选 ({selectedCount})
          </Button>
        )}
      </div>
      <Table
        columns={columns}
        data={data}
        rowKey="proxyId"
        loading={loading}
        emptyText="暂无代理配置，点击上方按钮添加或导入"
        sortColumn={sortColumn}
        sortOrder={sortOrder}
        onSort={onSort}
      />
    </Card>
  )
}
