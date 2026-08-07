import { useEffect, useMemo, useState } from 'react'
import { Button, Modal, toast } from '../../../shared/components'
import type { BrowserExtension, BrowserProfile } from '../types'
import {
  fetchBrowserExtensions,
  fetchBrowserProfileExtensionSettings,
  saveBrowserProfileExtensionSettings,
} from '../api/extensions'

interface ProfileExtensionModalProps {
  open: boolean
  profile: BrowserProfile | null
  onClose: () => void
}

export function ProfileExtensionModal({ open, profile, onClose }: ProfileExtensionModalProps) {
  const [extensions, setExtensions] = useState<BrowserExtension[]>([])
  const [configured, setConfigured] = useState(false)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds])

  useEffect(() => {
    if (!open || !profile) return
    setLoading(true)
    Promise.all([
      fetchBrowserExtensions(),
      fetchBrowserProfileExtensionSettings(profile.profileId),
    ]).then(([extensionItems, settings]) => {
      setExtensions(extensionItems)
      setConfigured(settings.configured)
      setSelectedIds(settings.extensionIds)
    }).catch((error: any) => {
      toast.error(error?.message || '加载实例插件配置失败')
    }).finally(() => setLoading(false))
  }, [open, profile])

  const toggleExtension = (extensionId: string, checked: boolean) => {
    setSelectedIds((current) => {
      if (checked) return current.includes(extensionId) ? current : [...current, extensionId]
      return current.filter((item) => item !== extensionId)
    })
  }

  const handleSave = async () => {
    if (!profile) return
    setSaving(true)
    try {
      await saveBrowserProfileExtensionSettings(profile.profileId, selectedIds, configured)
      toast.success('实例插件配置已保存，下次启动时写入')
      onClose()
    } catch (error: any) {
      toast.error(error?.message || '保存实例插件配置失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={profile ? `插件配置：${profile.profileName}` : '插件配置'}
      width="640px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} loading={saving} disabled={loading}>保存</Button>
        </>
      )}
    >
      <div className="space-y-3">
        <label className="flex items-center justify-between rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-3 py-2">
          <span className="text-sm text-[var(--color-text-primary)]">手动设置本实例</span>
          <input
            type="checkbox"
            checked={configured}
            onChange={(event) => setConfigured(event.target.checked)}
            className="h-4 w-4 rounded accent-[var(--color-accent)]"
          />
        </label>

        {!configured ? (
          <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-5 text-sm text-[var(--color-text-muted)]">
            当前实例继承“默认安装”插件。打开手动设置后，只写入下方勾选的插件。
          </div>
        ) : null}

        <div className="max-h-[360px] space-y-2 overflow-auto pr-1">
          {extensions.map((extension) => (
            <label key={extension.extensionId} className={`flex items-start gap-3 rounded-xl border border-[var(--color-border-default)] px-3 py-2 ${configured ? 'bg-[var(--color-bg-surface)]' : 'bg-[var(--color-bg-muted)] opacity-70'}`}>
              <input
                type="checkbox"
                disabled={!configured}
                checked={selectedSet.has(extension.extensionId)}
                onChange={(event) => toggleExtension(extension.extensionId, event.target.checked)}
                className="mt-1 h-4 w-4 shrink-0 rounded accent-[var(--color-accent)]"
              />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                  <span>{extension.name || extension.extensionId}</span>
                  {extension.version ? <span className="text-xs font-normal text-[var(--color-text-muted)]">v{extension.version}</span> : null}
                  {!extension.enabled ? <span className="rounded bg-[var(--color-bg-muted)] px-1.5 py-0.5 text-xs text-[var(--color-text-muted)]">全局停用</span> : null}
                </div>
                <div className="mt-1 break-all font-mono text-xs text-[var(--color-text-muted)]">{extension.extensionId}</div>
              </div>
            </label>
          ))}

          {extensions.length === 0 ? (
            <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-6 text-center text-sm text-[var(--color-text-muted)]">
              还没有安装插件，请先到插件包管理页面导入。
            </div>
          ) : null}
        </div>
      </div>
    </Modal>
  )
}
