import { useEffect, useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import { BUILTIN_PROXY_IDS, type ProxyDisplayInfo } from './helpers'

interface UseProxySelectionOptions {
  proxies: BrowserProxy[]
  currentPageList: ProxyDisplayInfo[]
  saveProxies: (list: BrowserProxy[]) => Promise<void>
}

export function useProxySelection({ proxies, currentPageList, saveProxies }: UseProxySelectionOptions) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false)

  const selectablePageItems = currentPageList.filter(p => !BUILTIN_PROXY_IDS.has(p.proxyId))
  const allCurrentPageSelected = selectablePageItems.length > 0 && selectablePageItems.every(p => selectedIds.has(p.proxyId))
  const someCurrentPageSelected = selectablePageItems.some(p => selectedIds.has(p.proxyId))
  const selectedCount = selectedIds.size

  useEffect(() => {
    const validIds = new Set(proxies.map(proxy => proxy.proxyId))
    setSelectedIds(prev => {
      const next = new Set(Array.from(prev).filter(proxyId => validIds.has(proxyId)))
      return next.size === prev.size ? prev : next
    })
  }, [proxies])

  const handleToggleAll = () => {
    if (allCurrentPageSelected) {
      setSelectedIds(prev => {
        const next = new Set(prev)
        selectablePageItems.forEach(p => next.delete(p.proxyId))
        return next
      })
    } else {
      setSelectedIds(prev => {
        const next = new Set(prev)
        selectablePageItems.forEach(p => next.add(p.proxyId))
        return next
      })
    }
  }

  const handleToggleOne = (proxyId: string) => {
    if (BUILTIN_PROXY_IDS.has(proxyId)) return
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

  const removeSelectedIds = (proxyIds: Iterable<string>) => {
    const removedIds = new Set(proxyIds)
    setSelectedIds(prev => {
      const next = new Set(prev)
      removedIds.forEach(proxyId => next.delete(proxyId))
      return next
    })
  }

  return {
    selectedIds,
    selectedCount,
    allCurrentPageSelected,
    someCurrentPageSelected,
    batchDeleteConfirmOpen,
    setBatchDeleteConfirmOpen,
    handleToggleAll,
    handleToggleOne,
    handleBatchDeleteConfirm,
    removeSelectedId,
    removeSelectedIds,
  }
}
