import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Button, Card, FormItem, Select, toast } from '../../../shared/components'
import { BrowserProfileCopyForm } from '../components/BrowserProfileCopyForm'
import { createBrowserProfileCopyOptions, isBrowserProfileCopyOptionsValid } from '../copyOptions'
import { buildBrowserProfileCopyName } from '../copyName'
import type { BrowserProfile, BrowserProfileCopyOptions } from '../types'
import { copyBrowserProfile, fetchBrowserProfiles } from '../api'

export function BrowserCopyPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [sourceId, setSourceId] = useState(id || '')
  const [targetName, setTargetName] = useState('')
  const [copyOptions, setCopyOptions] = useState<BrowserProfileCopyOptions>(() => createBrowserProfileCopyOptions())
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const loadProfiles = async () => {
      const list = await fetchBrowserProfiles()
      setProfiles(list)
      if (!sourceId && list.length > 0) {
        setSourceId(list[0].profileId)
      }
    }
    loadProfiles()
  }, [])

  const sourceProfile = profiles.find(item => item.profileId === sourceId)

  useEffect(() => {
    if (!sourceProfile) {
      return
    }
    setTargetName(buildBrowserProfileCopyName(sourceProfile.profileName))
  }, [sourceProfile?.profileId, sourceProfile?.profileName])

  const handleCopy = async () => {
    if (!sourceProfile || !targetName.trim()) {
      toast.error('请填写目标名称')
      return
    }
    if (!isBrowserProfileCopyOptionsValid(copyOptions)) {
      toast.error('请至少勾选一个自动化指纹项')
      return
    }
    setSaving(true)
    try {
      await copyBrowserProfile(sourceProfile.profileId, targetName.trim(), copyOptions)
      toast.success('配置已复制')
      navigate('/browser/list')
    } catch (error: any) {
      toast.error(error?.message || '复制失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4 animate-fade-in">
      <Card padding="none" className="shadow-[var(--shadow-sm)]">
        <div className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">配置复制</h1>
          <div className="flex flex-wrap gap-2 sm:justify-end">
            <Button variant="secondary" size="sm" onClick={() => navigate('/browser/list')}>返回列表</Button>
            <Button size="sm" onClick={handleCopy} loading={saving} disabled={!targetName.trim() || !isBrowserProfileCopyOptionsValid(copyOptions)}>生成配置</Button>
          </div>
        </div>
      </Card>

      <Card title="复制设置">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <FormItem label="源配置">
            <Select
              value={sourceId}
              onChange={e => setSourceId(e.target.value)}
              options={profiles.map(item => ({ value: item.profileId, label: item.profileName }))}
            />
          </FormItem>
        </div>
        <div className="mt-4">
          <BrowserProfileCopyForm
            copyName={targetName}
            copyOptions={copyOptions}
            onCopyNameChange={setTargetName}
            onCopyOptionsChange={setCopyOptions}
          />
        </div>
      </Card>
    </div>
  )
}
