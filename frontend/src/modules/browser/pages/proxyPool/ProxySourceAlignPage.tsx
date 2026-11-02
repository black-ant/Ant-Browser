import { useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowLeft, RefreshCw } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Card, Select, Table, toast } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { BrowserProxy } from '../../types'
import { fetchBrowserProxies, fetchClashImportFromURL, saveBrowserProxies } from '../../api'
import {
  nextProxyID,
  parseClashImportText,
  proxyToYaml,
  resolveImportedProxyName,
} from './helpers'
import {
  applySourceAlignment,
  buildSourceAlignment,
  collectURLImportSources,
  configEndpoint,
  sourceHostLabel,
  type AlignSubNode,
} from './helpers'
import type { URLImportSourceMeta } from './helpers.types'
import { readSourceAlignRecord, writeSourceAlignRecord } from './storage'

function formatRefreshTime(ms: number): string {
  const date = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// 对齐工作台（独立页面）：订阅改名后的人工核对与合并。
// 三方核对 = 已对齐（同名沿用旧 ID）/ 未对齐旧节点（可手动配对）/ 订阅新增。
// 应用 = 配对的旧行吸收新节点（proxyId 不变，实例绑定零操作），未配对旧行保持异常。
export function ProxySourceAlignPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [poolLoading, setPoolLoading] = useState(true)
  const [sourceId, setSourceId] = useState(searchParams.get('sourceId') || '')

  const loadPool = useCallback(async () => {
    try {
      setProxies(await fetchBrowserProxies())
    } finally {
      setPoolLoading(false)
    }
  }, [])
  useEffect(() => {
    void loadPool()
  }, [loadPool])

  const sources = useMemo(() => collectURLImportSources(proxies), [proxies])
  const source: URLImportSourceMeta | null =
    sources.find(item => item.sourceId === sourceId) || null

  useEffect(() => {
    // 未指定或指定的源已不存在时，回落到第一个源，并把选择同步进 URL。
    if (sources.length === 0) return
    if (!sources.some(item => item.sourceId === sourceId)) {
      setSourceId(sources[0].sourceId)
      return
    }
    if (searchParams.get('sourceId') !== sourceId) {
      setSearchParams(new URLSearchParams({ sourceId }), { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sources, sourceId])

  const sourceRows = useMemo(
    () => (source ? proxies.filter(proxy => (proxy.sourceId || '').trim() === source.sourceId) : []),
    [proxies, source],
  )

  const [subNodes, setSubNodes] = useState<AlignSubNode[]>([])
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [retryTick, setRetryTick] = useState(0)
  const [pairs, setPairs] = useState<Record<string, AlignSubNode | null>>({})
  const [applying, setApplying] = useState(false)

  useEffect(() => {
    setSubNodes([])
    setPairs({})
    setFetchError('')
    if (!source) return
    let cancelled = false
    setFetching(true)
    ;(async () => {
      try {
        const result = await fetchClashImportFromURL(source.sourceUrl, source.sourceFetchProxyId || '')
        if (cancelled) return
        const parsed = parseClashImportText(result.content || '')
        const nodes: AlignSubNode[] = parsed.map((proxy, index) => {
          const config = proxyToYaml(proxy)
          return {
            poolName: resolveImportedProxyName(proxy, index, source.sourceNamePrefix || ''),
            config,
            endpoint: configEndpoint(config),
          }
        })
        if (nodes.length === 0) throw new Error('订阅内容未解析到可用节点')
        const align = buildSourceAlignment(sourceRows, nodes)
        const initPairs: Record<string, AlignSubNode | null> = {}
        align.remainingOld.forEach(row => {
          const suggested = align.suggestions[row.proxyId]
          initPairs[row.proxyId] = suggested ? nodes.find(node => node.poolName === suggested) || null : null
        })
        setSubNodes(nodes)
        setPairs(initPairs)
      } catch (e: any) {
        if (!cancelled) setFetchError(e?.message || '订阅拉取失败')
      } finally {
        if (!cancelled) setFetching(false)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source?.sourceId, retryTick])

  const alignment = useMemo(() => buildSourceAlignment(sourceRows, subNodes), [sourceRows, subNodes])
  const pairedNewNames = useMemo(
    () => new Set(Object.values(pairs).filter(Boolean).map(node => node!.poolName)),
    [pairs],
  )
  const addedPreview = useMemo(
    () => alignment.remainingNew.filter(node => !pairedNewNames.has(node.poolName)),
    [alignment.remainingNew, pairedNewNames],
  )
  const pairCount = Object.values(pairs).filter(Boolean).length
  const keptMissing = Math.max(0, alignment.remainingOld.length - pairCount)
  const lastRecord = readSourceAlignRecord(source?.sourceId || '')

  // 左右对应表：左列历史节点（固定），右列对应最新节点（已对齐的固定展示，未对齐的下拉选择）。
  // 未处理的排前面，其余按名字排序。
  interface AlignTableRow {
    key: string
    oldName: string
    oldEndpoint: string
    missing: boolean
    locked: boolean
    sub: AlignSubNode | null
  }
  const alignTableRows = useMemo<AlignTableRow[]>(() => {
    const alignedByOld = new Map(alignment.aligned.map(item => [item.row.proxyId, item.sub]))
    const rows: AlignTableRow[] = sourceRows.map(row => {
      const lockedSub = alignedByOld.get(row.proxyId)
      return {
        key: row.proxyId,
        oldName: row.proxyName || row.proxyId,
        oldEndpoint: configEndpoint(row.proxyConfig),
        missing: !!row.sourceMissing,
        locked: !!lockedSub,
        sub: lockedSub || pairs[row.proxyId] || null,
      }
    })
    rows.sort((a, b) => Number(a.locked) - Number(b.locked) || a.oldName.localeCompare(b.oldName))
    return rows
  }, [alignment.aligned, sourceRows, pairs])

  const alignColumns: TableColumn<AlignTableRow>[] = [
    {
      key: 'old',
      title: '历史节点（固定）',
      render: (_, row) => (
        <div className='min-w-0'>
          <div className='flex items-center gap-1.5'>
            <span className='truncate text-xs text-[var(--color-text-primary)]' title={row.oldName}>{row.oldName}</span>
            {row.missing && (
              <span className='shrink-0 px-1.5 py-0.5 text-[11px] rounded bg-red-500/10 text-red-500'>异常</span>
            )}
          </div>
          <div className='truncate text-[11px] text-[var(--color-text-muted)]'>{row.oldEndpoint || '配置缺少端点信息'}</div>
        </div>
      ),
    },
    {
      key: 'new',
      title: '对应最新节点',
      render: (_, row) => {
        if (row.locked) {
          return (
            <div className='min-w-0'>
              <div className='truncate text-xs text-[var(--color-text-primary)]' title={row.sub?.poolName}>{row.sub?.poolName}</div>
              <div className='truncate text-[11px] text-[var(--color-text-muted)]'>{row.sub?.endpoint}</div>
            </div>
          )
        }
        const chosen = row.sub
        return (
          <Select
            className='w-64'
            value={chosen ? chosen.poolName : ''}
            onChange={(event) => {
              const value = event.target.value
              setPairs(prev => ({
                ...prev,
                [row.key]: alignment.remainingNew.find(node => node.poolName === value) || null,
              }))
            }}
            options={[
              { value: '', label: '不合并（保持异常）' },
              ...alignment.remainingNew
                .filter(node => node.poolName === chosen?.poolName || !pairedNewNames.has(node.poolName))
                .map(node => ({ value: node.poolName, label: node.poolName })),
            ]}
          />
        )
      },
    },
    {
      key: 'status',
      title: '状态',
      width: '96px',
      render: (_, row) => {
        if (row.locked) {
          return <span className='inline-block px-1.5 py-0.5 text-[11px] rounded bg-emerald-500/10 text-emerald-600'>已对齐</span>
        }
        if (row.sub) {
          return <span className='inline-block px-1.5 py-0.5 text-[11px] rounded bg-amber-500/10 text-amber-600'>待合并</span>
        }
        if (row.missing) {
          return <span className='inline-block px-1.5 py-0.5 text-[11px] rounded bg-red-500/10 text-red-500'>保持异常</span>
        }
        return <span className='inline-block px-1.5 py-0.5 text-[11px] rounded bg-[var(--color-bg-muted)] text-[var(--color-text-muted)]'>将标记异常</span>
      },
    },
  ]
  const missingSinceTimes = Array.from(
    new Set(sourceRows.filter(row => row.sourceMissing).map(row => row.sourceMissingSince || '').filter(Boolean)),
  )

  const handleApply = async () => {
    if (!source) return
    setApplying(true)
    try {
      const manualPairs: Record<string, AlignSubNode> = {}
      Object.entries(pairs).forEach(([oldId, sub]) => {
        if (sub) manualPairs[oldId] = sub
      })
      const nowISO = new Date().toISOString()
      const next = applySourceAlignment(proxies, source, alignment, manualPairs, nowISO, nextProxyID)
      await saveBrowserProxies(next)
      setProxies(next)
      const nameOf = (proxyId: string) => sourceRows.find(row => row.proxyId === proxyId)?.proxyName || proxyId
      writeSourceAlignRecord(source.sourceId, {
        at: nowISO,
        pairs: [
          ...alignment.aligned.map(item => ({ oldName: item.row.proxyName || '', newName: item.sub.poolName })),
          ...Object.entries(manualPairs).map(([oldId, sub]) => ({ oldName: nameOf(oldId), newName: sub.poolName })),
        ],
        added: addedPreview.length,
        keptMissing,
      })
      toast.success(`对齐已应用：合并 ${pairCount} 对、新增 ${addedPreview.length} 个、异常保留 ${keptMissing} 个`)
    } catch (e: any) {
      toast.error(e?.message || '应用对齐失败')
    } finally {
      setApplying(false)
    }
  }

  return (
    <div className='space-y-4 animate-fade-in'>
      <Card padding='none' className='shadow-[var(--shadow-sm)]'>
        <div className='flex items-center justify-between gap-3 px-4 py-3'>
          <div className='flex min-w-0 items-center gap-2'>
            <Button
              size='sm'
              variant='ghost'
              onClick={() => navigate('/browser/proxy-pool')}
              title='返回代理池'
              aria-label='返回代理池'
            >
              <ArrowLeft className='h-4 w-4' />
            </Button>
            <h1 className='text-xl font-semibold text-[var(--color-text-primary)]'>对齐工作台</h1>
            <span className='hidden text-xs text-[var(--color-text-muted)] lg:inline'>
              订阅改名后的人工核对与合并：旧节点 ID 保留，实例绑定不动
            </span>
          </div>
          {sources.length > 0 && (
            <Select
              className='w-64'
              value={source?.sourceId || ''}
              onChange={(event) => setSourceId(event.target.value)}
              options={sources.map(item => ({ value: item.sourceId, label: sourceHostLabel(item.sourceUrl) }))}
            />
          )}
        </div>
      </Card>

      {poolLoading ? (
        <Card padding='none'>
          <div className='px-4 py-10 text-center text-sm text-[var(--color-text-muted)]'>正在加载代理池…</div>
        </Card>
      ) : sources.length === 0 ? (
        <Card padding='none'>
          <div className='px-4 py-10 text-center text-sm text-[var(--color-text-muted)]'>
            暂无 URL 订阅源，请先在代理池「导入代理」中导入订阅
          </div>
        </Card>
      ) : !source ? (
        <Card padding='none'>
          <div className='px-4 py-10 text-center text-sm text-[var(--color-text-muted)]'>未找到指定订阅源，请从右上角切换</div>
        </Card>
      ) : (
        <Card padding='none'>
          <div className='space-y-3 p-4'>
            {lastRecord && (
              <p className='text-xs text-[var(--color-text-muted)]'>
                上次对齐记录：{new Date(lastRecord.at).toLocaleString()} · 对齐 {lastRecord.pairs.length} 对 · 新增 {lastRecord.added} 个 · 异常保留 {lastRecord.keptMissing} 个
              </p>
            )}
            {fetching && <p className='text-sm text-[var(--color-text-muted)]'>正在拉取订阅最新内容…</p>}
            {fetchError && (
              <div className='flex items-center justify-between rounded-lg border border-[var(--color-error)] bg-red-500/5 px-3 py-2'>
                <span className='text-sm text-red-500'>{fetchError}</span>
                <Button size='sm' variant='secondary' onClick={() => setRetryTick(tick => tick + 1)}>
                  <RefreshCw className='h-3.5 w-3.5' />重试
                </Button>
              </div>
            )}
            {!fetching && !fetchError && (
              <>
                <div className='flex items-center gap-3 text-xs text-[var(--color-text-muted)]'>
                  <span>已对齐 {alignment.aligned.length}</span>
                  <span>待处理旧节点 {alignment.remainingOld.length}</span>
                  <span>订阅新增 {addedPreview.length}</span>
                  {missingSinceTimes.length === 1 && missingSinceTimes[0] && Number.isFinite(Date.parse(missingSinceTimes[0]))
                    ? <span>异常自 {formatRefreshTime(Date.parse(missingSinceTimes[0]))}</span>
                    : null}
                </div>
                <Table
                  columns={alignColumns}
                  data={alignTableRows}
                  rowKey='key'
                  maxHeight='420px'
                  tableMinWidth='720px'
                  emptyText='该订阅源暂无节点'
                />
                {addedPreview.length > 0 && (
                  <div className='space-y-1'>
                    <p className='text-xs font-medium text-[var(--color-text-primary)]'>订阅新增（{addedPreview.length}）— 没有历史对应，将作为新节点加入</p>
                    <div className='max-h-40 overflow-y-auto rounded border border-[var(--color-border-muted)] divide-y divide-[var(--color-border-muted)]'>
                      {addedPreview.map(node => (
                        <div key={node.poolName} className='flex items-center justify-between gap-3 px-2.5 py-1.5 text-xs'>
                          <span className='min-w-0 truncate text-[var(--color-text-primary)]'>{node.poolName}</span>
                          <span className='shrink-0 text-[var(--color-text-muted)]'>{node.endpoint}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                <p className='text-[11px] text-[var(--color-text-muted)]'>
                  左列为池内历史节点（固定不变）；右列通过下拉指定对应的订阅最新节点。合并 = 旧节点沿用原 ID、名字与配置更新为新版（实例绑定不动）；未合并旧节点保持异常；确认前不会改动任何数据。
                </p>
              </>
            )}
            <div className='flex items-center justify-end gap-2 border-t border-[var(--color-border-muted)] pt-3'>
              <Button variant='secondary' onClick={() => navigate('/browser/proxy-pool')}>返回代理池</Button>
              <Button onClick={() => void handleApply()} loading={applying} disabled={fetching || !!fetchError || !source}>
                应用对齐
              </Button>
            </div>
          </div>
        </Card>
      )}
    </div>
  )
}
