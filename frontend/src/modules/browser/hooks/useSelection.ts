import { useCallback, useState } from 'react'

// useSelection 统一的多选集合状态（基于 Set<string>），替换各页面重复的 selectedIds 逻辑。
export function useSelection() {
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const toggle = useCallback((id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const set = useCallback((id: string, on: boolean) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })
  }, [])

  const selectAll = useCallback((ids: string[]) => setSelected(new Set(ids)), [])
  const clear = useCallback(() => setSelected(new Set()), [])
  const has = useCallback((id: string) => selected.has(id), [selected])

  return { selected, setSelected, toggle, set, selectAll, clear, has }
}
