import { useEffect, useMemo, useState } from 'react'
import { ChevronUp, ChevronDown } from 'lucide-react'

// 关键字内联行（从 BrowserListPage 提取）：超出两行时折叠并提供展开。
export function KeywordInlineRow({ keywords }: { keywords: string[] }) {
  const [expanded, setExpanded] = useState(false)
  const cRef = (useMemo(() => ({ current: null as HTMLDivElement | null }), []) as unknown) as React.MutableRefObject<HTMLDivElement | null>
  const [isOverflowing, setIsOverflowing] = useState(false)

  useEffect(() => {
    if (cRef.current) {
      setIsOverflowing(cRef.current.scrollHeight > 36)
    }
  }, [keywords])

  if (!keywords?.length) {
    return <span className="text-xs text-[var(--color-text-muted)] italic">暂无关键字</span>
  }

  return (
    <div className="flex items-start gap-4 w-full">
      <div
        ref={cRef}
        className={`flex flex-wrap gap-2 flex-1 transition-all duration-300 ${expanded ? '' : 'overflow-hidden max-h-[32px]'}`}
      >
        {keywords.map((kw, i) => (
          <span
            key={i}
            className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs
              bg-[var(--color-bg-surface)] border border-[var(--color-border-default)]
              text-[var(--color-text-secondary)] max-w-[200px]"
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
