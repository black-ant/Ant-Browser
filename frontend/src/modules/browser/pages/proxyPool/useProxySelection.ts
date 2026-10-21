import { useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import { isSelectableProxyDisplay, type ProxyDisplayInfo } from './helpers'

interface UseProxySelectionOptions {
  proxies: BrowserProxy[]
  filteredList: ProxyDisplayInfo[]
  saveProxies: (list: BrowserProxy[]) => Promise<void>
}

export function useProxySelection({ proxies, filteredList, saveProxies }: UseProxySelectionOptions) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false)

  const selectableFilteredList = filteredList.filter(isSelectableProxyDisplay)
  const allFilteredSelected = selectableFilteredList.length > 0 && selectableFilteredList.every(p => selectedIds.has(p.proxyId))
  const someFilteredSelected = selectableFilteredList.some(p => selectedIds.has(p.proxyId))
  const selectedCount = selectedIds.size

  const handleToggleAll = () => {
    if (allFilteredSelected) {
      setSelectedIds(prev => {
        const next = new Set(prev)
        selectableFilteredList.forEach(p => next.delete(p.proxyId))
        return next
      })
    } else {
      setSelectedIds(prev => {
        const next = new Set(prev)
        selectableFilteredList.forEach(p => next.add(p.proxyId))
        return next
      })
    }
  }

  const handleToggleOne = (proxyId: string) => {
    if (!filteredList.some(proxy => proxy.proxyId === proxyId && isSelectableProxyDisplay(proxy))) return
    setSelectedIds(prev => {
      const next = new Set(prev)
      next.has(proxyId) ? next.delete(proxyId) : next.add(proxyId)
      return next
    })
  }

  const handleBatchDeleteConfirm = async () => {
    try {
      const newProxies = proxies.filter(p => !selectedIds.has(p.proxyId))
      await saveProxies(newProxies)
      toast.success(`已删除 ${selectedIds.size} 个代理`)
      setSelectedIds(new Set())
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
  }

  const removeSelectedId = (proxyId: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      next.delete(proxyId)
      return next
    })
  }

  return {
    selectedIds,
    selectedCount,
    allFilteredSelected,
    someFilteredSelected,
    batchDeleteConfirmOpen,
    setBatchDeleteConfirmOpen,
    handleToggleAll,
    handleToggleOne,
    handleBatchDeleteConfirm,
    removeSelectedId,
  }
}
