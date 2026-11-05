import { useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'

interface UseProxyDeleteFlowOptions {
  proxies: BrowserProxy[]
  saveProxies: (list: BrowserProxy[]) => Promise<void>
  removeSelectedId: (proxyId: string) => void
}

export function useProxyDeleteFlow({ proxies, saveProxies, removeSelectedId }: UseProxyDeleteFlowOptions) {
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const handleDeleteClick = (proxyId: string) => {
    setDeletingId(proxyId)
    setDeleteConfirmOpen(true)
  }

  const handleDeleteConfirm = async () => {
    if (!deletingId) return
    try {
      const newProxies = proxies.filter(p => p.proxyId !== deletingId)
      await saveProxies(newProxies)
      removeSelectedId(deletingId)
      toast.success('代理已删除')
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
    setDeletingId(null)
  }

  return {
    deleteConfirmOpen,
    setDeleteConfirmOpen,
    handleDeleteClick,
    handleDeleteConfirm,
  }
}
