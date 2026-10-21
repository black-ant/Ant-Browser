import { useState } from 'react'
import { toast } from '../../../../shared/components'
import { createDefaultProxyCheckSettings, fetchProxyCheckSettings, saveProxyCheckSettings } from '../../api'
import { notifyProxySpeedResultSettingsChanged } from '../../proxySpeedFreshness'
import type { ProxyCheckSettings } from '../../types'

export function useProxyCheckSettingsModal() {
  const [checkSettingsOpen, setCheckSettingsOpen] = useState(false)
  const [checkSettings, setCheckSettings] = useState<ProxyCheckSettings>(() => createDefaultProxyCheckSettings())
  const [checkTargetsText, setCheckTargetsText] = useState('')
  const [savingCheckSettings, setSavingCheckSettings] = useState(false)

  const openCheckSettings = async () => {
    const settings = await fetchProxyCheckSettings()
    setCheckSettings(settings)
    setCheckTargetsText(JSON.stringify(settings.targets || [], null, 2))
    setCheckSettingsOpen(true)
  }

  const saveCheckSettings = async (): Promise<boolean> => {
    setSavingCheckSettings(true)
    try {
      const targets = JSON.parse(checkTargetsText || '[]')
      await saveProxyCheckSettings({ ...checkSettings, targets })
      notifyProxySpeedResultSettingsChanged()
      toast.success('检测设置已保存')
      setCheckSettingsOpen(false)
      return true
    } catch (error: any) {
      toast.error(error?.message || '检测设置保存失败')
      return false
    } finally {
      setSavingCheckSettings(false)
    }
  }

  return {
    checkSettingsOpen,
    setCheckSettingsOpen,
    checkSettings,
    setCheckSettings,
    checkTargetsText,
    setCheckTargetsText,
    savingCheckSettings,
    openCheckSettings,
    saveCheckSettings,
  }
}
