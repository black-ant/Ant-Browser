import { useEffect, useMemo, useState } from 'react'
import { Button, Input, Modal, Select, Switch, Table } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { BrowserProxy } from '../../types'
import { normalizeRefreshIntervalM, parseTimestampMs, sourceHostLabel } from './helpers.sources'
import type { URLImportSourceMeta } from './helpers.types'
import { ProxySourceAbnormalModal } from './ProxySourceAbnormalModal'

interface ProxySourceRefreshModalProps {
  open: boolean
  sources: URLImportSourceMeta[]
  proxies: BrowserProxy[]
  refreshingAll: boolean
  refreshingSourceIds: Set<string>
  /** 拉取代理下拉候选（测速成功节点，与导入弹窗同源） */
  fetchProxyOptions: BrowserProxy[]
  onClose: () => void
  onRefreshAll: () => void
  onRefreshOne: (sourceId: string) => void
  /** 保存订阅源配置（自动更新开关/间隔/拉取代理），成功后由父级重载池 */
  onSaveSourceConfig: (sourceId: string, autoRefresh: boolean, intervalM: number, fetchProxyId: string) => Promise<void>
  /** 异常对齐弹窗应用后由父级重载代理池 */
  onReloadProxies: () => Promise<void>
}

interface SourceManagerRow extends URLImportSourceMeta {
  nodeCount: number
  missingCount: number
  lastMs: number
}

interface SourceConfigDraft {
  sourceId: string
  label: string
  autoRefresh: boolean
  intervalM: string
  fetchProxyId: string
}

function formatRefreshTime(ms: number): string {
  const date = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatRelativeAge(ms: number): string {
  const diffMin = Math.floor((Date.now() - ms) / 60000)
  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin} 分钟前`
  const hours = Math.floor(diffMin / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}

// 分钟数转白话时长：≥60 分钟自动换算成小时，缩短文案减少换行
function formatMinutes(m: number): string {
  if (m >= 60 && m % 60 === 0) return `${m / 60} 小时`
  if (m > 60) return `${Math.floor(m / 60)} 小时 ${m % 60} 分`
  return `${m} 分钟`
}

// 订阅更新管理：集中展示每个 URL 订阅源的上次更新时间、自动更新计划，
// 支持编辑每源的开关/间隔/拉取代理，并提供单源/全部手动刷新入口
// （刷新与保存都走 useProxySourceRefresh / 后端 BrowserProxyUpdateSourceConfig 链路）。
export function ProxySourceRefreshModal({
  open,
  sources,
  proxies,
  refreshingAll,
  refreshingSourceIds,
  fetchProxyOptions,
  onClose,
  onRefreshAll,
  onRefreshOne,
  onSaveSourceConfig,
  onReloadProxies,
}: ProxySourceRefreshModalProps) {
  // 相对时间（x 分钟前/分钟后）随时间流逝会过期，弹窗打开期间每 30 秒重算一次。
  const [, setNowTick] = useState(0)
  useEffect(() => {
    if (!open) return
    const timer = window.setInterval(() => setNowTick(value => value + 1), 30_000)
    return () => window.clearInterval(timer)
  }, [open])

  const [draft, setDraft] = useState<SourceConfigDraft | null>(null)
  const [savingConfig, setSavingConfig] = useState(false)
  const [abnormalSourceId, setAbnormalSourceId] = useState<string | null>(null)
  useEffect(() => {
    // 弹窗关闭或源列表变化后，若编辑中的源已不存在则退出编辑态。
    if (!draft) return
    if (!open || !sources.some(item => item.sourceId === draft.sourceId)) {
      setDraft(null)
    }
  }, [open, sources, draft])
  useEffect(() => {
    // 异常对齐弹窗同口径：源消失或弹窗关闭即收起。
    if (!abnormalSourceId) return
    if (!open || !sources.some(item => item.sourceId === abnormalSourceId)) {
      setAbnormalSourceId(null)
    }
  }, [open, sources, abnormalSourceId])

  const rows = useMemo<SourceManagerRow[]>(() => {
    return sources.map((meta) => {
      const sourceProxies = proxies.filter(proxy => (proxy.sourceId || '').trim() === meta.sourceId)
      return {
        ...meta,
        nodeCount: sourceProxies.length,
        missingCount: sourceProxies.filter(proxy => proxy.sourceMissing).length,
        lastMs: parseTimestampMs(meta.sourceLastRefreshAt),
      }
    }).sort((a, b) => b.lastMs - a.lastMs)
  }, [sources, proxies])

  const abnormalSource = abnormalSourceId
    ? rows.find(row => row.sourceId === abnormalSourceId) || null
    : null

  const columns: TableColumn<SourceManagerRow>[] = [
    {
      key: 'source',
      title: '订阅源',
      width: '230px',
      render: (_, row) => (
        <div className="min-w-0">
          <div className="truncate text-[13px] text-[var(--color-text-primary)]" title={row.sourceUrl}>
            {sourceHostLabel(row.sourceUrl)}
          </div>
          {(row.sourceGroupName || row.sourceNamePrefix) && (
            <div className="mt-0.5 truncate text-xs text-[var(--color-text-muted)]">
              {[
                row.sourceGroupName ? `分组：${row.sourceGroupName}` : '',
                row.sourceNamePrefix ? `前缀：${row.sourceNamePrefix}` : '',
              ].filter(Boolean).join(' · ')}
            </div>
          )}
        </div>
      ),
    },
    {
      key: 'nodeCount',
      title: '节点数',
      width: '105px',
      render: (_, row) => (
        <div className="flex items-center gap-1.5 whitespace-nowrap">
          <span className="text-[13px] text-[var(--color-text-primary)]">{row.nodeCount} 个</span>
          {row.missingCount > 0 && (
            <button
              type="button"
              onClick={() => setAbnormalSourceId(prev => (prev === row.sourceId ? null : row.sourceId))}
              title="点击查看异常节点详情并对齐"
              className={`px-1.5 py-0.5 text-[11px] rounded cursor-pointer transition-colors ${
                abnormalSourceId === row.sourceId
                  ? 'bg-red-500/25 text-red-600'
                  : 'bg-red-500/10 text-red-500 hover:bg-red-500/20'
              }`}
            >
              异常 {row.missingCount}
            </button>
          )}
        </div>
      ),
    },
    {
      key: 'autoRefresh',
      title: '自动更新',
      width: '185px',
      render: (_, row) => {
        if (!row.sourceAutoRefresh) {
          return (
            <span className="inline-block px-1.5 py-0.5 text-[11px] rounded bg-[var(--color-bg-muted)] text-[var(--color-text-muted)]">
              已关闭
            </span>
          )
        }
        const intervalM = row.sourceRefreshIntervalM > 0 ? row.sourceRefreshIntervalM : 60
        let nextText = '待首次刷新'
        if (row.lastMs > 0) {
          const dueMs = row.lastMs + intervalM * 60_000
          nextText = dueMs <= Date.now()
            ? '已到期，即将自动刷新'
            : `约 ${formatMinutes(Math.max(1, Math.ceil((dueMs - Date.now()) / 60_000)))}后`
        }
        return (
          <div>
            <div className="flex items-center gap-1.5 whitespace-nowrap">
              <span className="inline-block px-1.5 py-0.5 text-[11px] rounded bg-emerald-500/10 text-emerald-600">
                已开启
              </span>
              <span className="text-xs text-[var(--color-text-muted)]">每 {formatMinutes(intervalM)}</span>
            </div>
            <div className="mt-0.5 text-xs text-[var(--color-text-muted)] whitespace-nowrap">{nextText}</div>
          </div>
        )
      },
    },
    {
      key: 'lastRefreshAt',
      title: '上次更新',
      width: '160px',
      render: (_, row) => {
        // 明确状态：失败记录优先展示，未刷新过的源也不允许留空。
        if (row.sourceLastRefreshError) {
          return (
            <div className="min-w-0">
              <span className="inline-block px-1.5 py-0.5 text-[11px] rounded bg-red-500/10 text-red-500">
                上次刷新失败
              </span>
              <div
                className="mt-0.5 truncate text-xs text-[var(--color-text-muted)]"
                title={row.sourceLastRefreshError}
              >
                {row.sourceLastRefreshError}
              </div>
            </div>
          )
        }
        if (row.lastMs <= 0) {
          return <span className="text-[13px] text-[var(--color-text-muted)]">未刷新</span>
        }
        return (
          <div>
            <div className="text-[13px] text-[var(--color-text-primary)]">{formatRefreshTime(row.lastMs)}</div>
            <div className="mt-0.5 text-xs text-[var(--color-text-muted)]">{formatRelativeAge(row.lastMs)}</div>
          </div>
        )
      },
    },
    {
      key: 'actions',
      title: '操作',
      width: '160px',
      render: (_, row) => (
        <div className="flex items-center gap-1.5">
          <Button
            size="sm"
            variant={draft?.sourceId === row.sourceId ? 'primary' : 'secondary'}
            className="whitespace-nowrap"
            onClick={() => {
              if (draft?.sourceId === row.sourceId) {
                setDraft(null)
                return
              }
              setDraft({
                sourceId: row.sourceId,
                label: sourceHostLabel(row.sourceUrl),
                autoRefresh: row.sourceAutoRefresh,
                intervalM: String(row.sourceRefreshIntervalM > 0 ? row.sourceRefreshIntervalM : 60),
                fetchProxyId: row.sourceFetchProxyId,
              })
            }}
          >
            {draft?.sourceId === row.sourceId ? '编辑中' : '编辑'}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            className="whitespace-nowrap"
            loading={refreshingSourceIds.has(row.sourceId)}
            disabled={refreshingAll}
            onClick={() => onRefreshOne(row.sourceId)}
          >
            立即刷新
          </Button>
        </div>
      ),
    },
  ]

  const handleSaveDraft = async () => {
    if (!draft) return
    const intervalM = draft.autoRefresh ? normalizeRefreshIntervalM(Number(draft.intervalM || 0)) : 0
    setSavingConfig(true)
    try {
      await onSaveSourceConfig(draft.sourceId, draft.autoRefresh, intervalM, draft.fetchProxyId.trim())
      setDraft(null)
    } finally {
      setSavingConfig(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="订阅更新管理"
      width="1040px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>关闭</Button>
          <Button
            onClick={onRefreshAll}
            loading={refreshingAll}
            disabled={rows.length === 0}
          >
            全部刷新
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        {draft && (
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] p-3 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-[var(--color-text-primary)]">
                编辑订阅源：<span className="truncate" title={draft.sourceId}>{draft.label}</span>
              </span>
              <Button size="sm" variant="ghost" onClick={() => setDraft(null)}>取消编辑</Button>
            </div>
            <div className="flex flex-wrap items-end gap-x-4 gap-y-2">
              <div className="flex items-center gap-2 pb-1.5">
                <span className="text-xs text-[var(--color-text-muted)]">自动更新</span>
                <Switch
                  checked={draft.autoRefresh}
                  onChange={(checked) => setDraft(prev => prev ? { ...prev, autoRefresh: checked } : prev)}
                />
              </div>
              {draft.autoRefresh && (
                <label className="flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
                  刷新间隔（分钟，5-1440）
                  <Input
                    type="number"
                    min={5}
                    max={1440}
                    value={draft.intervalM}
                    onChange={(event) => setDraft(prev => prev ? { ...prev, intervalM: event.target.value } : prev)}
                    className="w-24"
                  />
                </label>
              )}
              <label className="flex flex-col gap-1 text-xs text-[var(--color-text-muted)]">
                拉取代理（订阅失效或被墙时选择）
                <Select
                  value={draft.fetchProxyId}
                  onChange={(event) => setDraft(prev => prev ? { ...prev, fetchProxyId: event.target.value } : prev)}
                  options={[
                    { value: '', label: '直连（不走代理）' },
                    ...fetchProxyOptions.map(proxy => ({
                      value: proxy.proxyId,
                      label: `${proxy.proxyName}${typeof proxy.lastLatencyMs === 'number' && proxy.lastLatencyMs > 0 ? ` · ${proxy.lastLatencyMs}ms` : ''}`,
                    })),
                  ]}
                  className="min-w-[220px]"
                />
              </label>
              <Button
                size="sm"
                onClick={() => void handleSaveDraft()}
                loading={savingConfig}
              >
                保存
              </Button>
            </div>
            <p className="text-[11px] text-[var(--color-text-muted)]">
              拉取代理用于本源所有定时/手动刷新请求；所选节点被删除后刷新会失败，请及时回来更换。
            </p>
          </div>
        )}
        <Table
          columns={columns}
          data={rows}
          rowKey="sourceId"
          maxHeight="420px"
          emptyText="暂无 URL 订阅源，请先在「导入代理」中导入订阅"
        />
      </div>
      {abnormalSource && (
        <ProxySourceAbnormalModal
          open={!!abnormalSourceId}
          source={abnormalSource}
          proxies={proxies}
          onClose={() => setAbnormalSourceId(null)}
          onApplied={onReloadProxies}
        />
      )}
    </Modal>
  )
}
