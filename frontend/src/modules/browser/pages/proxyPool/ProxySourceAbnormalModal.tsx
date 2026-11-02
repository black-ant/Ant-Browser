import { useEffect, useMemo, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { Button, Modal, Select, Table, toast } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { BrowserProxy, SourceAlignRecord } from '../../types'
import { fetchClashImportFromURL, fetchSourceAlignRecords, saveBrowserProxies } from '../../api'
import type { URLImportSourceMeta } from './helpers.types'
import { nextProxyID, parseClashImportText, proxyToYaml, resolveImportedProxyName } from './helpers'
import { applySourceAlignment, buildSourceAlignment, configEndpoint, type AlignSubNode } from './helpers.sources'
import { readSourceAlignRecord, writeSourceAlignRecord } from './storage'

interface AbnormalRow {
  key: string
  oldName: string
  oldEndpoint: string
  since: string
  sub: AlignSubNode | null
}

interface ProxySourceAbnormalModalProps {
  open: boolean
  source: URLImportSourceMeta
  /** 全量代理池（应用合并用） */
  proxies: BrowserProxy[]
  onClose: () => void
  onApplied: () => void | Promise<void>
}

// 异常节点详情与对齐（独立弹窗）：
// 左右对应表格——左列历史异常节点（固定），右列通过下拉指定对应的订阅最新节点；
// 展示后端刷新自动对齐记录与手动对齐记录；应用 = 配对旧行吸收新节点（旧 ID 保留，实例绑定不动）。
export function ProxySourceAbnormalModal({
  open,
  source,
  proxies,
  onClose,
  onApplied,
}: ProxySourceAbnormalModalProps) {
  const sourceRows = useMemo(
    () => (source ? proxies.filter(proxy => (proxy.sourceId || '').trim() === source.sourceId) : []),
    [proxies, source],
  )
  const abnormalRows = useMemo(() => sourceRows.filter(row => row.sourceMissing), [sourceRows])

  const [subNodes, setSubNodes] = useState<AlignSubNode[]>([])
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [retryTick, setRetryTick] = useState(0)
  const [pairs, setPairs] = useState<Record<string, AlignSubNode | null>>({})
  const [applying, setApplying] = useState(false)
  const [autoRecords, setAutoRecords] = useState<SourceAlignRecord[]>([])

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
  const keptMissing = Math.max(0, abnormalRows.length - pairCount)
  const manualRecord = readSourceAlignRecord(source?.sourceId || '')

  useEffect(() => {
    setSubNodes([])
    setPairs({})
    setFetchError('')
    setAutoRecords([])
    if (!open || !source) return
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
        setSubNodes(nodes)
      } catch (e: any) {
        if (!cancelled) setFetchError(e?.message || '订阅拉取失败')
      } finally {
        if (!cancelled) setFetching(false)
      }
      try {
        const records = await fetchSourceAlignRecords(source.sourceId)
        if (!cancelled) setAutoRecords(records)
      } catch {
        // 记录读取失败不阻塞对齐流程
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, source?.sourceId, retryTick])

  const tableRows = useMemo<AbnormalRow[]>(
    () =>
      abnormalRows.map(row => ({
        key: row.proxyId,
        oldName: row.proxyName || row.proxyId,
        oldEndpoint: configEndpoint(row.proxyConfig),
        since: row.sourceMissingSince || '',
        sub: pairs[row.proxyId] || null,
      })),
    [abnormalRows, pairs],
  )

  const alignColumns: TableColumn<AbnormalRow>[] = [
    {
      key: 'old',
      title: '历史节点（固定）',
      render: (_, row) => (
        <div className='min-w-0'>
          <div className='truncate text-xs text-[var(--color-text-primary)]' title={row.oldName}>{row.oldName}</div>
          <div className='truncate text-[11px] text-[var(--color-text-muted)]'>{row.oldEndpoint || '配置缺少端点信息'}</div>
        </div>
      ),
    },
    {
      key: 'new',
      title: '对应最新节点',
      render: (_, row) => {
        const chosen = row.sub
        return (
          <Select
            className='w-64'
            value={chosen ? chosen.poolName : ''}
            onChange={(event) => {
              const value = event.target.value
              setPairs(prev => ({
                ...prev,
                [row.key]: subNodes.find(node => node.poolName === value) || null,
              }))
            }}
            options={[
              { value: '', label: '不合并（保持异常）' },
              ...subNodes
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
        if (row.sub) {
          return <span className='inline-block px-1.5 py-0.5 text-[11px] rounded bg-amber-500/10 text-amber-600'>待合并</span>
        }
        return <span className='inline-block px-1.5 py-0.5 text-[11px] rounded bg-red-500/10 text-red-500'>保持异常</span>
      },
    },
  ]

  const handleApply = async () => {
    if (!source) return
    setApplying(true)
    try {
      const manualPairs: Record<string, AlignSubNode> = {}
      Object.entries(pairs).forEach(([oldId, sub]) => {
        if (sub) manualPairs[oldId] = sub
      })
      const nowISO = new Date().toISOString()
      const alignmentFull = buildSourceAlignment(sourceRows, subNodes)
      const next = applySourceAlignment(proxies, source, alignmentFull, manualPairs, nowISO, nextProxyID)
      await saveBrowserProxies(next)
      const nameOf = (proxyId: string) => sourceRows.find(row => row.proxyId === proxyId)?.proxyName || proxyId
      writeSourceAlignRecord(source.sourceId, {
        at: nowISO,
        pairs: Object.entries(manualPairs).map(([oldId, sub]) => ({ oldName: nameOf(oldId), newName: sub.poolName })),
        added: addedPreview.length,
        keptMissing,
      })
      toast.success(`对齐已应用：合并 ${pairCount} 对、新增 ${addedPreview.length} 个、异常保留 ${keptMissing} 个`)
      await onApplied()
      onClose()
    } catch (e: any) {
      toast.error(e?.message || '应用对齐失败')
    } finally {
      setApplying(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`异常节点详情与对齐 — ${source.sourceUrl ? source.sourceUrl.replace(/^https?:\/\//, '').split('/')[0] : source.sourceId}`}
      width="900px"
      footer={
        <>
          <Button variant='secondary' onClick={onClose}>关闭</Button>
          <Button onClick={() => void handleApply()} loading={applying} disabled={fetching || !!fetchError}>
            应用对齐
          </Button>
        </>
      }
    >
      <div className='space-y-3'>
        {autoRecords.length > 0 && (
          <div className='text-xs text-[var(--color-text-muted)] space-y-0.5'>
            {autoRecords.map(record => (
              <p key={record.at}>
                刷新自动对齐 {new Date(record.at).toLocaleString()} · 对齐 {record.pairs?.length || 0} 对 · 新增 {record.added} 个 · 异常保留 {record.keptMissing} 个
              </p>
            ))}
          </div>
        )}
        {manualRecord && manualRecord.pairs.length > 0 && (
          <p className='text-xs text-[var(--color-text-muted)]'>
            手动对齐 {new Date(manualRecord.at).toLocaleString()} · 对齐 {manualRecord.pairs.length} 对 · 新增 {manualRecord.added} 个 · 异常保留 {manualRecord.keptMissing} 个
          </p>
        )}
        <div className='flex items-center gap-3 text-xs text-[var(--color-text-muted)]'>
          <span>异常节点 {abnormalRows.length}</span>
          <span>待合并 {pairCount}</span>
          <span>订阅新增 {addedPreview.length}</span>
        </div>
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
            {tableRows.length > 0 ? (
              <Table
                columns={alignColumns}
                data={tableRows}
                rowKey='key'
                maxHeight='320px'
                tableMinWidth='720px'
              />
            ) : (
              <p className='text-sm text-[var(--color-text-muted)]'>当前没有异常节点。</p>
            )}
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
              左列为池内历史异常节点（固定不变）；右列通过下拉指定对应的订阅最新节点。应用 = 配对的旧节点沿用原 ID、名字与配置更新为新版（实例绑定不动）；未配对旧节点保持异常；确认前不会改动任何数据。已对齐的节点不会出现在此表。
            </p>
          </>
        )}
      </div>
    </Modal>
  )
}
