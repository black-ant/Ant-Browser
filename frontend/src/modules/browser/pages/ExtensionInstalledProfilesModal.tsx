import { useEffect, useState } from 'react'
import { Badge, Button, Modal, toast } from '../../../shared/components'
import type { BrowserExtension, BrowserProfile } from '../types'
import { fetchBrowserProfiles } from '../api/profiles'
import { fetchBrowserProfileExtensionSettings } from '../api/extensions'
import { startBrowserInstance, stopBrowserInstance } from '../api/instances'

interface ExtensionInstalledProfilesModalProps {
  open: boolean
  extension: BrowserExtension | null
  onClose: () => void
}

// 列出正在使用某个插件的实例，并支持直接启停。
export function ExtensionInstalledProfilesModal({ open, extension, onClose }: ExtensionInstalledProfilesModalProps) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(false)
  const [busyId, setBusyId] = useState('')

  useEffect(() => {
    if (!open || !extension) return
    setLoading(true)
    fetchBrowserProfiles().then(async (profileItems) => {
      const matched = await Promise.all(profileItems.map(async (profile) => {
        const settings = await fetchBrowserProfileExtensionSettings(profile.profileId).catch(() => null)
        const usesExtension = settings?.configured
          ? settings.extensionIds.includes(extension.extensionId)
          : extension.defaultInstall
        return usesExtension ? profile : null
      }))
      setProfiles(matched.filter((profile): profile is BrowserProfile => profile !== null))
    }).catch((error: any) => {
      toast.error(error?.message || '加载使用该插件的实例失败')
    }).finally(() => setLoading(false))
  }, [open, extension])

  const refreshProfiles = async () => {
    const items = await fetchBrowserProfiles().catch(() => [] as BrowserProfile[])
    setProfiles((current) => current.map((profile) => items.find((item) => item.profileId === profile.profileId) || profile))
  }

  const handleStart = async (profile: BrowserProfile) => {
    setBusyId(profile.profileId)
    try {
      const started = await startBrowserInstance(profile.profileId)
      setProfiles((current) => current.map((item) => (
        item.profileId === profile.profileId ? { ...item, running: true, ...(started || {}) } : item
      )))
      toast.success(`${profile.profileName} 已启动`)
    } catch (error: any) {
      toast.error(error?.message || '启动实例失败')
    } finally {
      setBusyId('')
      void refreshProfiles()
    }
  }

  const handleStop = async (profile: BrowserProfile) => {
    setBusyId(profile.profileId)
    try {
      await stopBrowserInstance(profile.profileId)
      setProfiles((current) => current.map((item) => (
        item.profileId === profile.profileId ? { ...item, running: false } : item
      )))
      toast.success(`${profile.profileName} 已停止`)
    } catch (error: any) {
      toast.error(error?.message || '停止实例失败')
    } finally {
      setBusyId('')
      void refreshProfiles()
    }
  }

  const runningCount = profiles.filter((profile) => profile.running).length

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={extension ? `打开实例：${extension.name || extension.extensionId}` : '打开实例'}
      width="640px"
      footer={<Button variant="secondary" onClick={onClose}>关闭</Button>}
    >
      {loading ? (
        <div className="py-8 text-center text-sm text-[var(--color-text-muted)]">正在加载实例...</div>
      ) : (
        <div className="space-y-3">
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-3 py-2 text-sm text-[var(--color-text-secondary)]">
            共 {profiles.length} 个实例使用该插件，其中 {runningCount} 个运行中。插件在实例启动时加载，改动配置后需重启实例生效。
          </div>

          {profiles.length === 0 ? (
            <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-8 text-center text-sm text-[var(--color-text-muted)]">
              还没有实例使用该插件。点「设置实例」把它分配给实例。
            </div>
          ) : (
            <div className="max-h-[380px] space-y-2 overflow-auto pr-1">
              {profiles.map((profile) => (
                <div key={profile.profileId} className="flex flex-col gap-2 rounded-xl border border-[var(--color-border-default)] px-3 py-2 md:flex-row md:items-center md:justify-between">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-medium text-[var(--color-text-primary)]">{profile.profileName || profile.profileId}</span>
                      <Badge variant={profile.running ? 'success' : 'default'} size="sm" dot>
                        {profile.running ? '运行中' : '已停止'}
                      </Badge>
                    </div>
                    <div className="mt-0.5 break-all font-mono text-xs text-[var(--color-text-muted)]">{profile.profileId}</div>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    {profile.running ? (
                      <Button size="sm" variant="secondary" onClick={() => void handleStop(profile)} loading={busyId === profile.profileId}>
                        停止
                      </Button>
                    ) : (
                      <Button size="sm" onClick={() => void handleStart(profile)} loading={busyId === profile.profileId}>
                        启动
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}
