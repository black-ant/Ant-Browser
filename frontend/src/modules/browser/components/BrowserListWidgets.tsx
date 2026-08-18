import { useEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronUp, Copy, Download, Pencil, Play, RefreshCw, Square, Trash2 } from 'lucide-react'

import { Button, toast } from '../../../shared/components'
import { regenerateBrowserProfileCode, setBrowserProfileCode } from '../api'

interface BatchToolbarProps {
  selectedCount: number
  totalCount: number
  onSelectAll: () => void
  onDeselectAll: () => void
  onBatchStart: () => void
  onBatchStop: () => void
  onBatchExport: () => void
  onBatchDelete: () => void
  batchLoading: boolean
  exporting?: boolean
}

export function BatchToolbar({
  selectedCount,
  totalCount,
  onSelectAll,
  onDeselectAll,
  onBatchStart,
  onBatchStop,
  onBatchExport,
  onBatchDelete,
  batchLoading,
  exporting = false,
}: BatchToolbarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="flex items-center gap-3 px-4 py-2.5 bg-[var(--color-accent)]/10 border border-[var(--color-accent)]/20 rounded-lg">
      <span className="text-sm font-medium text-[var(--color-accent)]">已选 {selectedCount} / {totalCount}</span>
      <div className="flex gap-1.5 ml-auto">
        <Button size="sm" variant="ghost" onClick={onSelectAll}>全选</Button>
        <Button size="sm" variant="ghost" onClick={onDeselectAll}>取消</Button>
        <Button size="sm" onClick={onBatchStart} loading={batchLoading} title="批量启动">
          <Play className="w-3.5 h-3.5" />启动
        </Button>
        <Button size="sm" variant="secondary" onClick={onBatchStop} loading={batchLoading} title="批量停止">
          <Square className="w-3.5 h-3.5" />停止
        </Button>
        <Button size="sm" variant="secondary" onClick={onBatchExport} loading={exporting} title="导出实例">
          <Download className="w-3.5 h-3.5" />导出
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={onBatchDelete}
          title="批量删除"
          className="text-red-500 hover:text-red-600"
        >
          <Trash2 className="w-3.5 h-3.5" />删除
        </Button>
      </div>
    </div>
  )
}

interface LaunchCodeCellProps {
  profileId: string
  code: string
  onRefresh: () => void
}

export function LaunchCodeCell({ profileId, code, onRefresh }: LaunchCodeCellProps) {
  const [loading, setLoading] = useState(false)

  const handleCopy = () => {
    if (!code) return
    navigator.clipboard.writeText(code).then(() => toast.success('已复制快捷码'))
  }

  const handleRegenerate = async () => {
    setLoading(true)
    try {
      await regenerateBrowserProfileCode(profileId)
      onRefresh()
      toast.success('快捷码已重新生成')
    } catch {
      toast.error('重新生成失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCustomCode = async () => {
    const next = prompt('请输入自定义 Code（4-32位，仅支持字母/数字/_/-）', code || '')
    if (next == null) return

    const value = next.trim()
    if (!value) {
      toast.error('Code 不能为空')
      return
    }

    setLoading(true)
    try {
      const applied = await setBrowserProfileCode(profileId, value)
      onRefresh()
      toast.success(`Code 已更新为 ${applied}`)
    } catch (error: any) {
      toast.error(error?.message || '设置自定义 Code 失败')
    } finally {
      setLoading(false)
    }
  }

  if (!code) {
    return <span className="text-[var(--color-text-muted)] text-xs">-</span>
  }

  return (
    <div className="flex items-center gap-1">
      <code className="text-xs font-mono bg-[var(--color-bg-secondary)] px-1.5 py-0.5 rounded text-[var(--color-accent)]">{code}</code>
      <button onClick={handleCopy} className="p-0.5 hover:text-[var(--color-accent)] text-[var(--color-text-muted)] transition-colors" title="复制">
        <Copy className="w-3 h-3" />
      </button>
      <button onClick={handleRegenerate} disabled={loading} className="p-0.5 hover:text-[var(--color-accent)] text-[var(--color-text-muted)] transition-colors disabled:opacity-50" title="重新生成">
        <RefreshCw className="w-3 h-3" />
      </button>
      <button onClick={handleCustomCode} disabled={loading} className="p-0.5 hover:text-[var(--color-accent)] text-[var(--color-text-muted)] transition-colors disabled:opacity-50" title="自定义">
        <Pencil className="w-3 h-3" />
      </button>
    </div>
  )
}

interface KeywordInlineRowProps {
  keywords: string[]
}

export function KeywordInlineRow({ keywords }: KeywordInlineRowProps) {
  const [expanded, setExpanded] = useState(false)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [isOverflowing, setIsOverflowing] = useState(false)

  const handleCopyKeyword = async (keyword: string) => {
    try {
      await navigator.clipboard.writeText(keyword)
      toast.success('关键字已复制')
    } catch {
      toast.error('复制失败')
    }
  }

  useEffect(() => {
    if (containerRef.current) {
      setIsOverflowing(containerRef.current.scrollHeight > 36)
    }
  }, [keywords])

  if (!keywords?.length) {
    return <span className="text-xs text-[var(--color-text-muted)]">-</span>
  }

  return (
    <div className="flex items-start gap-4 w-full min-w-0">
      <div
        ref={containerRef}
        className={`flex flex-wrap gap-2 flex-1 min-w-0 transition-all duration-300 ${expanded ? '' : 'overflow-hidden max-h-[32px]'}`}
      >
        {keywords.map((keyword, index) => (
          <button
            type="button"
            key={index}
            className="inline-flex max-w-full min-w-0 items-center gap-1.5 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-2.5 py-1 text-left text-xs text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-accent)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
            title={`点击复制：${keyword}`}
            onClick={() => { void handleCopyKeyword(keyword) }}
          >
            <span className="text-[var(--color-text-muted)] font-mono shrink-0">{index + 1}.</span>
            <span className="truncate">{keyword}</span>
          </button>
        ))}
      </div>
      {isOverflowing && (
        <button
          onClick={() => setExpanded((prev) => !prev)}
          className="shrink-0 flex items-center gap-1 text-xs font-medium text-[var(--color-accent)] hover:text-indigo-400 mt-1 focus:outline-none"
        >
          {expanded ? (
            <>收回 <ChevronUp className="w-3.5 h-3.5" /></>
          ) : (
            <>展开详情 <ChevronDown className="w-3.5 h-3.5" /></>
          )}
        </button>
      )}
    </div>
  )
}
