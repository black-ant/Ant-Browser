import { useEffect, useState } from 'react'
import { RotateCcw, Save } from 'lucide-react'

import { Button, Card, ThemeSwitcher, toast } from '../../shared/components'
import { fetchSettings, resetSettings, saveSettings } from './api'
import { SettingsAdvancedCard, SettingsBasicFeatureCards } from './components/SettingsGeneralCards'
import { defaultSettings } from './types'
import type { AppSettings } from './types'

export function SettingsPage() {
  const [settings, setSettings] = useState<AppSettings>(defaultSettings)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [hasChanges, setHasChanges] = useState(false)

  useEffect(() => {
    void loadSettings()
  }, [])

  const loadSettings = async () => {
    setLoading(true)
    try {
      setSettings(await fetchSettings())
      setHasChanges(false)
    } catch (error: any) {
      toast.error(error?.message || '系统设置加载失败')
    } finally {
      setLoading(false)
    }
  }

  const handleChange = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    setSettings(prev => ({ ...prev, [key]: value }))
    setHasChanges(true)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const success = await saveSettings(settings)
      if (success) {
        setHasChanges(false)
        toast.success('设置已保存')
      }
    } catch (error: any) {
      toast.error(error?.message || '保存失败，请检查配置')
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    if (!confirm('确定要重置所有设置吗？')) return

    try {
      setSettings(await resetSettings())
      setHasChanges(false)
      toast.success('设置已重置')
    } catch (error: any) {
      toast.error(error?.message || '设置重置失败')
    }
  }

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--color-border-default)] border-t-[var(--color-accent)]" />
      </div>
    )
  }

  return (
    <div className="w-full space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">系统设置</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleReset}>
            <RotateCcw className="h-4 w-4" />
            重置
          </Button>
          <Button variant="danger" size="sm" onClick={handleSave} loading={saving} disabled={!hasChanges}>
            <Save className="h-4 w-4" />
            保存
          </Button>
        </div>
      </div>

      <Card title="主题">
        <ThemeSwitcher />
      </Card>
      <SettingsBasicFeatureCards settings={settings} onChange={handleChange} />
      <SettingsAdvancedCard settings={settings} onChange={handleChange} />
    </div>
  )
}
