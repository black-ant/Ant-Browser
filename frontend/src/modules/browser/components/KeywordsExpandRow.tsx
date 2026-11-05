import { useState, useRef, useEffect } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'

interface Props {
  keywords: string[]
  colSpan: number
}

export function KeywordsExpandRow({ keywords, colSpan }: Props) {
  const [expanded, setExpanded] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const [isOverflowing, setIsOverflowing] = useState(false)

  useEffect(() => {
    if (containerRef.current) {
      // 检查内容实际高度是否超过了 1 行的高度 (约 32px)
      setIsOverflowing(containerRef.current.scrollHeight > 36)
    }
  }, [keywords])

  return (
    <tr>
      <td
        colSpan={colSpan}
        className="px-6 py-3 bg-[var(--color-bg-muted)]/30 border-b border-[var(--color-border-muted)]"
      >
        {!keywords?.length ? (
          <span className="text-xs text-[var(--color-text-muted)]">-</span>
        ) : (
          <div className="flex items-start gap-4 min-w-0">
            <div
              ref={containerRef}
              className={`flex flex-wrap gap-2 flex-1 min-w-0 transition-all duration-300 ${expanded ? '' : 'overflow-hidden max-h-[32px]'}`}
            >
              {keywords.map((kw, i) => (
                <span
                  key={i}
                  className="inline-flex max-w-full min-w-0 items-center gap-1.5 px-2.5 py-1 rounded-md text-xs
                    bg-[var(--color-bg-surface)] border border-[var(--color-border-default)]
                    text-[var(--color-text-secondary)]"
                  title={kw}
                >
                  <span className="text-[var(--color-text-muted)] font-mono shrink-0">{i + 1}.</span>
                  <span className="truncate">{kw}</span>
                </span>
              ))}
            </div>
            {isOverflowing && (
              <button
                onClick={() => setExpanded(!expanded)}
                className="shrink-0 flex items-center gap-1 text-xs text-[var(--color-accent)] hover:underline mt-1 focus:outline-none"
              >
                {expanded ? (
                  <>收回 <ChevronUp className="w-3.5 h-3.5" /></>
                ) : (
                  <>展开详情 <ChevronDown className="w-3.5 h-3.5" /></>
                )}
              </button>
            )}
          </div>
        )}
      </td>
    </tr>
  )
}

