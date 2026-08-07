import { useEffect, useMemo, useState } from 'react'
import { ExternalLink, Search } from 'lucide-react'
import { Button, Input, Modal, toast } from '../../../shared/components'
import type { BrowserExtension, BrowserGroupWithCount, BrowserProfile, BrowserProfileExtensionSettings } from '../types'
import { fetchBrowserProfileExtensionSettings, saveBrowserProfileExtensionSettings, type BrowserExtensionManualDownloadFile, type BrowserExtensionManualInstallGuide } from '../api/extensions'
import { fetchGroups } from '../api/groups'
import { fetchBrowserProfiles } from '../api/profiles'
import { extensionHistoryActionLabel, formatExtensionTime, sameStringSet, type ExtensionHistoryRecord } from './extensionManagementUtils'

const UNGROUPED_PROFILE_GROUP_ID = '__ungrouped__'

export interface ExtensionProfileLimitModalProps {
  open: boolean
  extension: BrowserExtension | null
  allExtensions: BrowserExtension[]
  onClose: () => void
}

export function ExtensionProfileLimitModal({ open, extension, allExtensions, onClose }: ExtensionProfileLimitModalProps) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [settingsByProfile, setSettingsByProfile] = useState<Record<string, BrowserProfileExtensionSettings>>({})
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds])
  const defaultExtensionIds = useMemo(
    () => allExtensions.filter((item) => item.enabled && item.defaultInstall).map((item) => item.extensionId),
    [allExtensions],
  )
  const groupNameMap = useMemo(() => {
    const map = new Map<string, string>()
    groups.forEach((group) => map.set(group.groupId, group.groupName))
    return map
  }, [groups])

  const profileGroups = useMemo(() => {
    const buckets = new Map<string, BrowserProfile[]>()
    profiles.forEach((profile) => {
      const groupId = (profile.groupId || '').trim() || UNGROUPED_PROFILE_GROUP_ID
      if (!buckets.has(groupId)) buckets.set(groupId, [])
      buckets.get(groupId)!.push(profile)
    })

    const sections = groups
      .filter((group) => buckets.has(group.groupId))
      .sort((a, b) => a.sortOrder - b.sortOrder || a.groupName.localeCompare(b.groupName))
      .map((group) => ({
        groupId: group.groupId,
        groupName: group.groupName,
        profiles: buckets.get(group.groupId) || [],
      }))

    for (const [groupId, items] of buckets.entries()) {
      if (groupId === UNGROUPED_PROFILE_GROUP_ID) continue
      if (!sections.some((section) => section.groupId === groupId)) {
        sections.push({
          groupId,
          groupName: groupNameMap.get(groupId) || `分组 ${groupId}`,
          profiles: items,
        })
      }
    }

    if (buckets.has(UNGROUPED_PROFILE_GROUP_ID)) {
      sections.push({
        groupId: UNGROUPED_PROFILE_GROUP_ID,
        groupName: '未分组',
        profiles: buckets.get(UNGROUPED_PROFILE_GROUP_ID) || [],
      })
    }

    return sections
  }, [profiles, groups, groupNameMap])

  useEffect(() => {
    if (!open || !extension) return
    setLoading(true)
    Promise.all([fetchBrowserProfiles(), fetchGroups()]).then(async ([profileItems, groupItems]) => {
      const profileSettings = await Promise.all(profileItems.map(async (profile) => ({
        profile,
        settings: await fetchBrowserProfileExtensionSettings(profile.profileId),
      })))
      const settingsMap: Record<string, BrowserProfileExtensionSettings> = {}
      profileSettings.forEach(({ profile, settings }) => {
        settingsMap[profile.profileId] = settings
      })
      setProfiles(profileItems)
      setGroups(groupItems)
      setSettingsByProfile(settingsMap)
      setSelectedIds(profileItems
        .filter((profile) => {
          const settings = settingsMap[profile.profileId]
          return settings?.configured ? settings.extensionIds.includes(extension.extensionId) : extension.defaultInstall
        })
        .map((profile) => profile.profileId))
    }).catch((error: any) => {
      toast.error(error?.message || '加载实例限制失败')
    }).finally(() => setLoading(false))
  }, [open, extension])

  const toggleProfile = (profileId: string, checked: boolean) => {
    setSelectedIds((current) => {
      if (checked) return current.includes(profileId) ? current : [...current, profileId]
      return current.filter((item) => item !== profileId)
    })
  }

  const toggleGroup = (profileIds: string[], checked: boolean) => {
    setSelectedIds((current) => {
      const next = new Set(current)
      profileIds.forEach((profileId) => {
        if (checked) next.add(profileId)
        else next.delete(profileId)
      })
      return Array.from(next)
    })
  }

  const handleSave = async () => {
    if (!extension) return
    setSaving(true)
    try {
      const selected = new Set(selectedIds)
      const saveTasks = profiles.map((profile) => {
        const current = settingsByProfile[profile.profileId]
        const baseIds = current?.configured ? current.extensionIds : defaultExtensionIds
        const nextIds = selected.has(profile.profileId)
          ? Array.from(new Set([...baseIds, extension.extensionId]))
          : baseIds.filter((extensionId) => extensionId !== extension.extensionId)

        if (!current?.configured && sameStringSet(baseIds, nextIds)) return null
        if (current?.configured && sameStringSet(current.extensionIds, nextIds)) return null
        return saveBrowserProfileExtensionSettings(profile.profileId, nextIds, true)
      }).filter((task): task is Promise<BrowserProfileExtensionSettings> => task !== null)

      if (saveTasks.length > 0) await Promise.all(saveTasks)
      toast.success('实例插件设置已保存，下次启动时写入')
      onClose()
    } catch (error: any) {
      toast.error(error?.message || '保存实例限制失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={extension ? `设置实例：${extension.name || extension.extensionId}` : '设置实例'}
      width="680px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} loading={saving} disabled={loading || !extension}>保存</Button>
        </>
      )}
    >
      {loading ? (
        <div className="py-8 text-center text-sm text-[var(--color-text-muted)]">正在加载实例...</div>
      ) : (
        <div className="space-y-3">
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-3 py-2 text-sm text-[var(--color-text-secondary)]">
            勾选的实例会写入此插件；未勾选的实例会按默认安装状态或本实例的其他设置处理。
          </div>
          <div className="max-h-[420px] space-y-3 overflow-auto pr-1">
            {profileGroups.map((group) => {
              const groupProfileIds = group.profiles.map((profile) => profile.profileId)
              const selectedCount = groupProfileIds.filter((profileId) => selectedSet.has(profileId)).length
              const allSelected = groupProfileIds.length > 0 && selectedCount === groupProfileIds.length

              return (
                <section key={group.groupId} className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
                  <div className="flex items-center justify-between gap-3 border-b border-[var(--color-border-muted)] px-3 py-2">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{group.groupName}</div>
                      <div className="text-xs text-[var(--color-text-muted)]">已选 {selectedCount} / {groupProfileIds.length}</div>
                    </div>
                    <Button size="sm" variant="ghost" onClick={() => toggleGroup(groupProfileIds, !allSelected)}>
                      {allSelected ? '取消本组' : '选择本组'}
                    </Button>
                  </div>
                  <div className="divide-y divide-[var(--color-border-muted)]">
                    {group.profiles.map((profile) => (
                      <label key={profile.profileId} className="flex items-start gap-3 px-3 py-2">
                        <input
                          type="checkbox"
                          checked={selectedSet.has(profile.profileId)}
                          onChange={(event) => toggleProfile(profile.profileId, event.target.checked)}
                          className="mt-1 h-4 w-4 shrink-0 rounded accent-[var(--color-accent)]"
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                            <span>{profile.profileName || profile.profileId}</span>
                            {profile.running ? <span className="rounded bg-green-50 px-1.5 py-0.5 text-xs text-green-700">运行中</span> : null}
                            {settingsByProfile[profile.profileId]?.configured ? <span className="rounded bg-[var(--color-bg-muted)] px-1.5 py-0.5 text-xs font-normal text-[var(--color-text-muted)]">手动设置</span> : null}
                          </div>
                          <div className="mt-1 break-all font-mono text-xs text-[var(--color-text-muted)]">{profile.profileId}</div>
                        </div>
                      </label>
                    ))}
                  </div>
                </section>
              )
            })}

            {profiles.length === 0 ? (
              <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-8 text-center text-sm text-[var(--color-text-muted)]">
                暂无实例
              </div>
            ) : null}
          </div>
        </div>
      )}
    </Modal>
  )
}

export interface ExtensionHistoryModalProps {
  open: boolean
  records: ExtensionHistoryRecord[]
  onClose: () => void
  onPick: (record: ExtensionHistoryRecord) => void
  onClear: () => void
}

export function ExtensionHistoryModal({ open, records, onClose, onPick, onClear }: ExtensionHistoryModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="插件历史"
      width="760px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClear} disabled={records.length === 0}>清空历史</Button>
          <Button onClick={onClose}>关闭</Button>
        </>
      )}
    >
      <div className="max-h-[520px] overflow-y-auto">
        {records.length === 0 ? (
          <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-8 text-center text-sm text-[var(--color-text-muted)]">
            暂无历史记录
          </div>
        ) : (
          <div className="space-y-2">
            {records.map((record) => (
              <div key={record.id} className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-3">
                <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-full bg-[var(--color-bg-muted)] px-2 py-0.5 text-xs text-[var(--color-text-muted)]">{extensionHistoryActionLabel(record.action)}</span>
                      <span className={record.ok ? 'text-xs text-green-600' : 'text-xs text-red-500'}>{record.ok ? '成功' : '失败'}</span>
                      <span className="text-xs text-[var(--color-text-muted)]">{formatExtensionTime(record.createdAt)}</span>
                    </div>
                    <div className="mt-1 truncate text-sm font-medium text-[var(--color-text-primary)]">{record.name || record.extensionId || record.query}</div>
                    {record.extensionId ? <div className="mt-1 break-all font-mono text-xs text-[var(--color-text-muted)]">{record.extensionId}</div> : null}
                    {record.proxyLabel ? <div className="mt-1 text-xs text-[var(--color-text-muted)]">{record.proxyLabel}</div> : null}
                    {record.message ? <div className="mt-1 line-clamp-2 text-xs text-[var(--color-text-muted)]">{record.message}</div> : null}
                  </div>
                  <div className="flex shrink-0 gap-2">
                    <Button type="button" size="sm" variant="secondary" onClick={() => onPick(record)}>
                      <Search className="h-4 w-4" />
                      使用
                    </Button>
                    {record.storeUrl ? (
                      <Button type="button" size="sm" variant="secondary" onClick={() => window.open(record.storeUrl, '_blank', 'noopener,noreferrer')}>
                        <ExternalLink className="h-4 w-4" />
                        商店页
                      </Button>
                    ) : null}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </Modal>
  )
}

export interface ManualInstallModalProps {
  open: boolean
  guide: BrowserExtensionManualInstallGuide | null
  files: BrowserExtensionManualDownloadFile[]
  loading: boolean
  fileLoading: boolean
  importingFileName: string
  onClose: () => void
  onOpenDownloadDir: () => void
  onRefreshFiles: () => void
  onImportFile: (fileName: string) => void
  onImportDirectory: () => void
}

function formatFileSize(sizeBytes: number): string {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) return '0 B'
  if (sizeBytes < 1024) return `${sizeBytes} B`
  if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KB`
  return `${(sizeBytes / 1024 / 1024).toFixed(1)} MB`
}

export function ManualInstallModal({ open, guide, files, loading, fileLoading, importingFileName, onClose, onOpenDownloadDir, onRefreshFiles, onImportFile, onImportDirectory }: ManualInstallModalProps) {
  const copyText = async (value: string, label: string) => {
    if (!value) return
    await navigator.clipboard?.writeText(value)
    toast.success(`${label}已复制`)
  }

  return (
    <Modal open={open} onClose={onClose} title="手动安装插件" width="720px">
      {loading ? (
        <div className="py-8 text-center text-sm text-[var(--color-text-muted)]">正在生成安装信息...</div>
      ) : guide ? (
        <div className="space-y-4">
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] p-3">
            <div className="text-xs text-[var(--color-text-muted)]">插件 ID</div>
            <div className="mt-1 break-all font-mono text-sm text-[var(--color-text-primary)]">{guide.extensionId}</div>
          </div>

          <div className="space-y-2">
            <div className="text-sm font-medium text-[var(--color-text-primary)]">1. 复制下载地址，用浏览器或下载器下载 CRX</div>
            <div className="flex items-center gap-2">
              <Input value={guide.downloadUrl} readOnly className="min-w-0 flex-1 font-mono text-xs" />
              <Button type="button" variant="secondary" className="shrink-0 whitespace-nowrap px-3" onClick={() => copyText(guide.downloadUrl, '下载地址')}>复制</Button>
            </div>
          </div>

          <div className="space-y-2">
            <div className="text-sm font-medium text-[var(--color-text-primary)]">2. 推荐保存到这个文件夹</div>
            <div className="flex items-center gap-2">
              <Input value={`${guide.downloadDir}\${guide.fileName}`} readOnly className="min-w-0 flex-1 font-mono text-xs" />
              <Button type="button" variant="secondary" className="shrink-0 whitespace-nowrap px-3" onClick={() => copyText(guide.downloadDir, '文件夹路径')}>复制</Button>
              <Button type="button" variant="secondary" className="shrink-0 whitespace-nowrap px-3" onClick={onOpenDownloadDir}>打开</Button>
            </div>
          </div>

          <div className="space-y-2 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-3">
            <div className="flex items-center justify-between gap-2">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">3. 选择下载目录里的插件包导入</div>
              <Button type="button" size="sm" variant="secondary" onClick={onRefreshFiles} loading={fileLoading}>扫描目录</Button>
            </div>
            <div className="text-xs text-[var(--color-text-muted)]">只扫描上面文件夹里的 `.crx` / `.zip` 文件；如果你已经解压成插件目录，点“导入目录”。</div>
            <div className="max-h-52 overflow-y-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-muted)]">
              {files.length === 0 ? (
                <div className="px-3 py-5 text-center text-xs text-[var(--color-text-muted)]">目录里还没有可导入的 `.crx` / `.zip` 文件</div>
              ) : files.map((file) => (
                <div key={file.fileName} className="flex items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-3 py-2 last:border-b-0">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{file.fileName}</div>
                    <div className="mt-0.5 text-xs text-[var(--color-text-muted)]">{formatFileSize(file.sizeBytes)} · {file.updatedAt}</div>
                  </div>
                  <Button type="button" size="sm" onClick={() => onImportFile(file.fileName)} loading={importingFileName === file.fileName}>导入</Button>
                </div>
              ))}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button type="button" size="sm" variant="secondary" onClick={onImportDirectory}>导入目录</Button>
              {guide.storeUrl ? (
                <Button type="button" size="sm" variant="secondary" onClick={() => window.open(guide.storeUrl, '_blank', 'noopener,noreferrer')}>商店页</Button>
              ) : null}
            </div>
          </div>
        </div>
      ) : (
        <div className="py-8 text-center text-sm text-[var(--color-text-muted)]">请输入插件 ID 或 Chrome Web Store 链接</div>
      )}
    </Modal>
  )
}

export interface DownloadDirectoryInstallModalProps {
  open: boolean
  files: BrowserExtensionManualDownloadFile[]
  fileLoading: boolean
  importingFileName: string
  onClose: () => void
  onOpenDownloadDir: () => void
  onRefreshFiles: () => void
  onImportFile: (fileName: string) => void
}

export function DownloadDirectoryInstallModal({ open, files, fileLoading, importingFileName, onClose, onOpenDownloadDir, onRefreshFiles, onImportFile }: DownloadDirectoryInstallModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="从下载目录安装"
      width="680px"
      footer={(
        <>
          <Button variant="secondary" onClick={onOpenDownloadDir}>打开目录</Button>
          <Button variant="secondary" onClick={onRefreshFiles} loading={fileLoading}>重新扫描</Button>
          <Button onClick={onClose}>关闭</Button>
        </>
      )}
    >
      <div className="max-h-[420px] overflow-y-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-muted)]">
        {fileLoading ? (
          <div className="px-3 py-8 text-center text-sm text-[var(--color-text-muted)]">正在扫描下载目录...</div>
        ) : files.length === 0 ? (
          <div className="px-3 py-8 text-center text-sm text-[var(--color-text-muted)]">没有找到可导入的 `.crx` / `.zip` 文件</div>
        ) : files.map((file) => (
          <div key={file.fileName} className="flex items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-3 py-2 last:border-b-0">
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{file.fileName}</div>
              <div className="mt-0.5 text-xs text-[var(--color-text-muted)]">{formatFileSize(file.sizeBytes)} · {file.updatedAt}</div>
            </div>
            <Button type="button" size="sm" onClick={() => onImportFile(file.fileName)} loading={importingFileName === file.fileName}>导入</Button>
          </div>
        ))}
      </div>
    </Modal>
  )
}
