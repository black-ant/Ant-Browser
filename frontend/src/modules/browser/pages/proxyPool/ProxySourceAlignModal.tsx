import { useEffect, useMemo, useState } from 'react'
import { Button, Modal, Select, toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import { fetchClashImportFromURL, saveBrowserProxies } from '../../api'
import type { URLImportSourceMeta } from './helpers.types'
import { nextProxyID, parseClashImportText, proxyToYaml, resolveImportedProxyName } from './helpers'
import {
  applySourceAlignment,
  buildSourceAlignment,
  configEndpoint,
  type AlignSubNode,
} from './helpers.sources'
import { readSourceAlignRecord, writeSourceAlignRecord } from './storage'

interface ProxySourceAlignModalProps {
  open: boolean
  source: URLImportSourceMeta
  sourceRows: BrowserProxy[]
  onClose: () => void
  onApplied: () => void | Promise<void>
}

// 对齐工作台：拉取订阅最新内容，与池内该源节点做"之前 / 最新 / 已对齐"三方核对，
// 未对齐的允许人工配对合并（旧 ID 保留，实例绑定不动），应用后记录本次对齐结果。
export function ProxySourceAlignModal({
  open,
  source,
  sourceRows,
  onClose,
  onApplied,
}: ProxySourceAlignModalProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [retryTick, setRetryTick] = useState(0)
  const [subNodes, setSubNodes] = useState<AlignSubNode[]>([])
  const [pairs, setPairs] = useState<Record<string, AlignSubNode | null>>({})
  const [applying, setApplying] = useState(false)

  const alignment = useMemo(() => buildSourceAlignment(sourceRows, subNodes), [sourceRows, subNodes])

  useEffect(() => {
    if (!open) {
      setSubNodes([])
      setPairs({})
      setError('')
      return
    }
    let cancelled = false
    setLoading(true)
    setError('')
    setSubNodes([])
    setPairs({})
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
        if (!cancelled) setError(e?.message || '订阅拉取失败')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, source.sourceId, retryTick])

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
  const lastRecord = useMemo(() => (open ? readSourceAlignRecord(source.sourceId) : null), [open, source.sourceId])

  const handleApply = async () => {
    setApplying(true)
    try {
      const manualPairs: Record<string, AlignSubNode> = {}
      Object.entries(pairs).forEach(([oldId, sub]) => {
        if (sub) manualPairs[oldId] = sub
      })
      const next = applySourceAlignment(sourceRows, source, alignment, manualPairs, new Date().toISOString(), nextProxyID)
      await saveBrowserProxies(next)
      const nameOf = (proxyId: string) => sourceRows.find(row => row.proxyId === proxyId)?.proxyName || proxyId
      writeSourceAlignRecord(source.sourceId, {
        at: new Date().toISOString(),
        pairs: [
          ...alignment.aligned.map(item => ({ oldName: item.row.proxyName || '', newName: item.sub.poolName })),
          ...Object.entries(manualPairs).map(([oldId, sub]) => ({ oldName: nameOf(oldId), newName: sub.poolName })),
        ],
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
      title={`对齐工作台 — ${source.sourceUrl ? source.sourceUrl.replace(/^https?:\/\//, '').split('/')[0] : source.sourceId}`}
      width="900px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={() => void handleApply()} loading={applying} disabled={loading || !!error}>
            应用对齐
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        {lastRecord && (
          <p className="text-xs text-[var(--color-text-muted)]">
            上次对齐记录：{new Date(lastRecord.at).toLocaleString()} · 对齐 {lastRecord.pairs.length} 对 · 新增 {lastRecord.added} 个 · 异常保留 {lastRecord.keptMissing} 个
          </p>
        )}
        {loading && <p className="text-sm text-[var(--color-text-muted)]">正在拉取订阅最新内容…</p>}
        {error && (
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-error)] bg-red-500/5 px-3 py-2">
            <span className="text-sm text-red-500">{error}</span>
            <Button size="sm" variant="secondary" onClick={() => setRetryTick(tick => tick + 1)}>重试</Button>
          </div>
        )}
        {!loading && !error && (
          <>
            <div className="flex items-center gap-3 text-xs text-[var(--color-text-muted)]">
              <span>已对齐 {alignment.aligned.length}</span>
              <span>待处理旧节点 {alignment.remainingOld.length}</span>
              <span>订阅新增 {addedPreview.length}</span>
            </div>
            {alignment.aligned.length > 0 && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-[var(--color-text-primary)]">已对齐（沿用旧节点 ID，配置更新为订阅最新）</p>
                <div className="max-h-40 overflow-y-auto rounded border border-[var(--color-border-muted)] divide-y divide-[var(--color-border-muted)]">
                  {alignment.aligned.map(item => (
                    <div key={item.row.proxyId} className="flex items-center justify-between gap-3 px-2.5 py-1.5 text-xs">
                      <span className="min-w-0 truncate text-[var(--color-text-primary)]">{item.row.proxyName}</span>
                      <span className="shrink-0 text-[var(--color-text-muted)]">{item.sub.endpoint}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
            {alignment.remainingOld.length > 0 && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-[var(--color-text-primary)]">
                  未对齐旧节点（{alignment.remainingOld.length}）— 自动按"服务器：端口：协议"给出建议，可手动调整
                </p>
                <div className="space-y-1.5">
                  {alignment.remainingOld.map(row => {
                    const endpoint = configEndpoint(row.proxyConfig)
                    const chosen = pairs[row.proxyId] || null
                    return (
                      <div key={row.proxyId} className="flex items-center gap-2 rounded border border-[var(--color-border-muted)] px-2.5 py-1.5">
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-xs text-[var(--color-text-primary)]" title={row.proxyName}>{row.proxyName}</div>
                          <div className="truncate text-[11px] text-[var(--color-text-muted)]">{endpoint || '配置缺少端点信息'}</div>
                        </div>
                        <Select
                          className="w-64 shrink-0"
                          value={chosen ? chosen.poolName : ''}
                          onChange={(event) => {
                            const value = event.target.value
                            setPairs(prev => ({
                              ...prev,
                              [row.proxyId]: alignment.remainingNew.find(node => node.poolName === value) || null,
                            }))
                          }}
                          options={[
                            { value: '', label: '不合并（保持异常）' },
                            ...alignment.remainingNew
                              .filter(node => node.poolName === chosen?.poolName || !pairedNewNames.has(node.poolName))
                              .map(node => ({ value: node.poolName, label: node.poolName })),
                          ]}
                        />
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
            {alignment.remainingOld.length === 0 && alignment.aligned.length > 0 && alignment.remainingNew.length === 0 && (
              <p className="text-sm text-[var(--color-text-muted)]">全部节点已对齐，无需处理。</p>
            )}
            {addedPreview.length > 0 && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-[var(--color-text-primary)]">订阅新增（{addedPreview.length}）— 将作为新节点加入</p>
                <div className="max-h-40 overflow-y-auto rounded border border-[var(--color-border-muted)] divide-y divide-[var(--color-border-muted)]">
                  {addedPreview.map(node => (
                    <div key={node.poolName} className="flex items-center justify-between gap-3 px-2.5 py-1.5 text-xs">
                      <span className="min-w-0 truncate text-[var(--color-text-primary)]">{node.poolName}</span>
                      <span className="shrink-0 text-[var(--color-text-muted)]">{node.endpoint}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
            <p className="text-[11px] text-[var(--color-text-muted)]">
              合并 = 旧节点沿用原 ID、名字与配置更新为新版（实例绑定不动）；未合并旧节点保持异常；确认前不会改动任何数据。
            </p>
          </>
        )}
      </div>
    </Modal>
  )
}
