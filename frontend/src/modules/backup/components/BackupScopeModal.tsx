import { useEffect, useMemo, useState } from 'react'
import { Search } from 'lucide-react'

import { Button, Input, Modal, Select } from '../../../shared/components'
import { fetchBrowserProfiles } from '../../browser/api/profiles'
import type { BrowserProfile } from '../../browser/types'

interface BackupScopeModalProps {
  open: boolean
  onClose: () => void
  onConfirm: (profileIds: string[]) => void
  initialProfileIds?: string[]
}

type BackupScope = 'full' | 'profiles'
type ProfileSort = 'createdAtAsc' | 'createdAtDesc' | 'profileNameAsc' | 'profileNameDesc'

const PROFILE_SORT_OPTIONS = [
  { value: 'createdAtDesc', label: '创建时间 从晚到早' },
  { value: 'createdAtAsc', label: '创建时间 从早到晚' },
  { value: 'profileNameAsc', label: '名称 A 到 Z' },
  { value: 'profileNameDesc', label: '名称 Z 到 A' },
]

function getProfileDisplayName(profile: BrowserProfile) {
  return profile.profileName.trim() || profile.profileId
}

function compareProfileName(left: BrowserProfile, right: BrowserProfile) {
  return getProfileDisplayName(left).localeCompare(getProfileDisplayName(right), 'zh-CN', {
    numeric: true,
    sensitivity: 'base',
  }) || left.profileId.localeCompare(right.profileId)
}

function compareProfileCreatedAt(left: BrowserProfile, right: BrowserProfile, descending: boolean) {
  const leftTime = Date.parse(left.createdAt)
  const rightTime = Date.parse(right.createdAt)
  const leftValid = Number.isFinite(leftTime)
  const rightValid = Number.isFinite(rightTime)

  if (leftValid !== rightValid) return leftValid ? -1 : 1
  if (leftValid && rightValid && leftTime !== rightTime) {
    return descending ? rightTime - leftTime : leftTime - rightTime
  }
  return compareProfileName(left, right)
}

export function BackupScopeModal({
  open,
  onClose,
  onConfirm,
  initialProfileIds = [],
}: BackupScopeModalProps) {
  const [scope, setScope] = useState<BackupScope>('full')
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [profileQuery, setProfileQuery] = useState('')
  const [profileSort, setProfileSort] = useState<ProfileSort>('createdAtDesc')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    const selected = new Set(initialProfileIds.filter(Boolean))
    setScope(selected.size > 0 ? 'profiles' : 'full')
    setSelectedIds(selected)
    setProfileQuery('')
    setProfileSort('createdAtDesc')
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
  const filteredProfiles = useMemo(() => {
    const query = profileQuery.trim().toLocaleLowerCase()
    if (!query) return profiles

    return profiles.filter(profile => (
      profile.profileName.toLocaleLowerCase().includes(query)
      || profile.profileId.toLocaleLowerCase().includes(query)
    ))
  }, [profileQuery, profiles])
  const sortedProfiles = useMemo(() => {
    const sorted = [...filteredProfiles]
    sorted.sort((left, right) => {
      if (profileSort === 'createdAtAsc') return compareProfileCreatedAt(left, right, false)
      if (profileSort === 'createdAtDesc') return compareProfileCreatedAt(left, right, true)
      const nameComparison = compareProfileName(left, right)
      return profileSort === 'profileNameAsc' ? nameComparison : -nameComparison
    })
    return sorted
  }, [filteredProfiles, profileSort])
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
      width="720px"
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
          </label>
        </div>

        {scope === 'profiles' && (
          <div className="rounded-lg border border-[var(--color-border-default)]">
            {!loading && profiles.length > 0 && (
              <div className="flex items-center gap-2 border-b border-[var(--color-border-muted)] p-2">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={() => toggleAll()}
                  disabled={selectableProfiles.length === 0}
                  aria-label={allSelected ? '取消全选实例' : '全选实例'}
                  title={allSelected ? '取消全选' : '全选'}
                  className="h-4 w-4 shrink-0 accent-[var(--color-accent)]"
                />
                <div className="relative min-w-0 flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]" />
                  <Input
                    aria-label="搜索实例"
                    value={profileQuery}
                    onChange={event => setProfileQuery(event.target.value)}
                    placeholder="搜索实例名称或 ID"
                    className="w-full pl-9"
                  />
                </div>
                <span className='shrink-0 text-xs text-[var(--color-text-muted)]'>已选 {selectedIds.size}</span>
                <div className='flex shrink-0 items-center gap-1.5'>
                  <span className='text-xs text-[var(--color-text-muted)]'>排序</span>
                  <Select
                    aria-label="实例排序"
                    value={profileSort}
                    onChange={event => setProfileSort(event.target.value as ProfileSort)}
                    options={PROFILE_SORT_OPTIONS}
                    className="w-[176px]"
                  />
                </div>
              </div>
            )}
            <div className="max-h-80 overflow-y-auto p-2">
              {loading && <p className="px-2 py-5 text-center text-sm text-[var(--color-text-muted)]">读取实例中...</p>}
              {!loading && profiles.length === 0 && <p className="px-2 py-5 text-center text-sm text-[var(--color-text-muted)]">暂无可备份实例</p>}
              {!loading && profiles.length > 0 && (
                sortedProfiles.length > 0 ? (
                  <div className="space-y-1">
                    {sortedProfiles.map(profile => {
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
                ) : (
                  <p className="px-2 py-5 text-center text-sm text-[var(--color-text-muted)]">没有匹配的实例</p>
                )
              )}
            </div>
          </div>
        )}

        {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
      </div>
    </Modal>
  )
}
