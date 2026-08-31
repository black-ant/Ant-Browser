import { useEffect, useMemo, useState } from 'react'

import { Button, Modal } from '../../../shared/components'
import { fetchBrowserProfiles } from '../../browser/api/profiles'
import type { BrowserProfile } from '../../browser/types'

interface BackupScopeModalProps {
  open: boolean
  onClose: () => void
  onConfirm: (profileIds: string[]) => void
  initialProfileIds?: string[]
}

type BackupScope = 'full' | 'profiles'

export function BackupScopeModal({
  open,
  onClose,
  onConfirm,
  initialProfileIds = [],
}: BackupScopeModalProps) {
  const [scope, setScope] = useState<BackupScope>('full')
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    const selected = new Set(initialProfileIds.filter(Boolean))
    setScope(selected.size > 0 ? 'profiles' : 'full')
    setSelectedIds(selected)
    setError('')
    setLoading(true)

    let active = true
    void fetchBrowserProfiles()
      .then(items => {
        if (!active) return
        const available = items.filter(profile => !profile.deletedAt)
        setProfiles(available)
        setSelectedIds(current => new Set(
          Array.from(current).filter(id => available.some(profile => profile.profileId === id && !profile.running)),
        ))
      })
      .catch(fetchError => {
        if (active) {
          setProfiles([])
          setError(fetchError?.message || '读取实例列表失败')
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [initialProfileIds, open])

  const selectableProfiles = useMemo(
    () => profiles.filter(profile => !profile.running),
    [profiles],
  )
  const allSelected = selectableProfiles.length > 0 && selectableProfiles.every(profile => selectedIds.has(profile.profileId))

  const toggleProfile = (profileId: string) => {
    setError('')
    setSelectedIds(current => {
      const next = new Set(current)
      if (next.has(profileId)) next.delete(profileId)
      else next.add(profileId)
      return next
    })
  }

  const toggleAll = () => {
    setError('')
    if (allSelected) {
      setSelectedIds(new Set())
      return
    }
    setSelectedIds(new Set(selectableProfiles.map(profile => profile.profileId)))
  }

  const handleConfirm = () => {
    if (scope === 'full') {
      onConfirm([])
      return
    }
    if (selectedIds.size === 0) {
      setError('请选择至少一个实例')
      return
    }
    onConfirm(Array.from(selectedIds))
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="选择备份范围"
      width="560px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={handleConfirm}>继续选择渠道</Button>
        </>
      )}
    >
      <div className="space-y-3">
        <div className="grid gap-2 sm:grid-cols-2">
          <label
            className={`rounded-lg border px-3 py-3 text-left transition-colors ${scope === 'full' ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]' : 'border-[var(--color-border-default)] hover:border-[var(--color-accent)]'}`}
          >
            <span className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
              <input
                type="radio"
                name="backup-scope"
                checked={scope === 'full'}
                onChange={() => {
                  setScope('full')
                  setError('')
                }}
              />
              全量备份
            </span>
            <span className="mt-1 block text-xs text-[var(--color-text-muted)]">配置、数据、内核和日志</span>
          </label>
          <label
            className={`rounded-lg border px-3 py-3 text-left transition-colors ${scope === 'profiles' ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]' : 'border-[var(--color-border-default)] hover:border-[var(--color-accent)]'}`}
          >
            <span className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
              <input
                type="radio"
                name="backup-scope"
                checked={scope === 'profiles'}
                onChange={() => {
                  setScope('profiles')
                  setError('')
                }}
              />
              选择实例
            </span>
            <span className="mt-1 block text-xs text-[var(--color-text-muted)]">仅备份实例配置和用户数据</span>
          </label>
        </div>

        {scope === 'profiles' && (
          <div className="rounded-lg border border-[var(--color-border-default)]">
            <div className="flex items-center justify-between border-b border-[var(--color-border-muted)] px-3 py-2 text-xs">
              <button
                type="button"
                className="text-[var(--color-accent)] hover:underline disabled:cursor-not-allowed disabled:opacity-50"
                onClick={toggleAll}
                disabled={loading || selectableProfiles.length === 0}
              >
                {allSelected ? '取消全选' : '全选可备份实例'}
              </button>
              <span className="text-[var(--color-text-muted)]">已选 {selectedIds.size}</span>
            </div>
            <div className="max-h-64 overflow-y-auto p-2">
              {loading && <p className="px-2 py-5 text-center text-sm text-[var(--color-text-muted)]">读取实例中...</p>}
              {!loading && profiles.length === 0 && <p className="px-2 py-5 text-center text-sm text-[var(--color-text-muted)]">暂无可备份实例</p>}
              {!loading && profiles.length > 0 && (
                <div className="space-y-1">
                  {profiles.map(profile => {
                    const disabled = profile.running
                    return (
                      <label
                        key={profile.profileId}
                        className={`flex items-center gap-2 rounded-md px-2 py-2 text-sm ${disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer hover:bg-[var(--color-bg-muted)]'}`}
                      >
                        <input
                          type="checkbox"
                          checked={selectedIds.has(profile.profileId)}
                          onChange={() => toggleProfile(profile.profileId)}
                          disabled={disabled}
                          className="h-4 w-4 accent-[var(--color-accent)]"
                        />
                        <span className="min-w-0 flex-1 truncate text-[var(--color-text-primary)]">{profile.profileName || profile.profileId}</span>
                        {disabled && <span className="shrink-0 text-xs text-[var(--color-warning)]">运行中</span>}
                      </label>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        )}

        {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
      </div>
    </Modal>
  )
}
