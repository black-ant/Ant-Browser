import { useState } from 'react'
import { CheckCircle, Edit2, Plus, Star, Trash2, XCircle } from 'lucide-react'
import { Button, Card, FormItem, Input, Modal, Table, Textarea, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserCore, BrowserCoreInput, BrowserSettings } from '../types'
import { FrontProxySettings } from './FrontProxySettings'
import {
  deleteBrowserCore,
  saveBrowserCore,
  saveBrowserSettings,
  setDefaultBrowserCore,
  validateBrowserCorePath,
} from '../api'

interface BrowserSettingsModalProps {
  open: boolean
  onClose: () => void
  settings: BrowserSettings
  cores: BrowserCore[]
  onCoresChange: (cores: BrowserCore[]) => void
}

export function BrowserSettingsModal({ open, onClose, settings: initSettings, cores, onCoresChange }: BrowserSettingsModalProps) {
  const [settings, setSettings] = useState<BrowserSettings>(initSettings)
  const [fingerprintText, setFingerprintText] = useState((initSettings.defaultFingerprintArgs || []).join('\n'))
  const [launchText, setLaunchText] = useState((initSettings.defaultLaunchArgs || []).join('\n'))
  const [saving, setSaving] = useState(false)

  // 内核编辑弹窗
  const [coreModalOpen, setCoreModalOpen] = useState(false)
  const [coreForm, setCoreForm] = useState<BrowserCoreInput>({ coreId: '', coreName: '', corePath: '', isDefault: false })
  const [coreValidation, setCoreValidation] = useState<{ valid: boolean; message: string } | null>(null)
  const [savingCore, setSavingCore] = useState(false)

  const handleSave = async () => {
    setSaving(true)
    try {
      await saveBrowserSettings({
        ...settings,
        defaultFingerprintArgs: fingerprintText.split('\n').map(s => s.trim()).filter(Boolean),
        defaultLaunchArgs: launchText.split('\n').map(s => s.trim()).filter(Boolean),
      })
      toast.success('配置已保存')
      onClose()
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleOpenCoreModal = (core?: BrowserCore) => {
    setCoreForm(core ? { ...core } : { coreId: '', coreName: '', corePath: '', isDefault: false })
    setCoreValidation(null)
    setCoreModalOpen(true)
  }

  const handleValidateCorePath = async () => {
    if (!coreForm.corePath.trim()) { setCoreValidation({ valid: false, message: '请输入路径' }); return }
    setCoreValidation(await validateBrowserCorePath(coreForm.corePath))
  }

  const handleSaveCore = async () => {
    if (!coreForm.coreName.trim()) { toast.error('请输入内核名称'); return }
    if (!coreForm.corePath.trim()) { toast.error('请输入内核路径'); return }
    setSavingCore(true)
    try {
      await saveBrowserCore(coreForm)
      toast.success('内核已保存')
      setCoreModalOpen(false)
      // 刷新 cores 列表
      const { fetchBrowserCores } = await import('../api')
      onCoresChange(await fetchBrowserCores())
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSavingCore(false)
    }
  }

  const handleDeleteCore = async (coreId: string) => {
    if (cores.length <= 1) { toast.error('至少保留一个内核'); return }
    await deleteBrowserCore(coreId)
    toast.success('内核已删除')
    const { fetchBrowserCores } = await import('../api')
    onCoresChange(await fetchBrowserCores())
  }

  const handleSetDefaultCore = async (coreId: string) => {
    await setDefaultBrowserCore(coreId)
    toast.success('已设为默认')
    const { fetchBrowserCores } = await import('../api')
    onCoresChange(await fetchBrowserCores())
  }

  const coreColumns: TableColumn<BrowserCore>[] = [
    { key: 'coreName', title: '名称' },
    { key: 'corePath', title: '路径' },
    { key: 'isDefault', title: '默认', render: (v) => v ? <Star className="w-4 h-4 text-yellow-500 fill-yellow-500" /> : null },
    {
      key: 'actions', title: '操作', align: 'right',
      render: (_, record) => (
        <div className="flex justify-end gap-1">
          {!record.isDefault && <Button size="sm" variant="ghost" onClick={() => handleSetDefaultCore(record.coreId)} title="设为默认"><Star className="w-4 h-4" /></Button>}
          <Button size="sm" variant="ghost" onClick={() => handleOpenCoreModal(record)} title="编辑"><Edit2 className="w-4 h-4" /></Button>
          <Button size="sm" variant="ghost" onClick={() => handleDeleteCore(record.coreId)} title="删除"><Trash2 className="w-4 h-4" /></Button>
        </div>
      ),
    },
  ]

  return (
    <>
      <Modal open={open} onClose={onClose} title="基础配置" width="700px"
        footer={<><Button variant="secondary" onClick={onClose}>取消</Button><Button onClick={handleSave} loading={saving}>保存</Button></>}>
        <div className="space-y-6">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-[var(--color-text-primary)]">内核管理</span>
              <Button size="sm" onClick={() => handleOpenCoreModal()}><Plus className="w-4 h-4" />新增内核</Button>
            </div>
            <Card padding="none"><Table columns={coreColumns} data={cores} rowKey="coreId" /></Card>
          </div>
          <FormItem label="用户数据根目录">
            <Input value={settings.userDataRoot} onChange={e => setSettings(p => ({ ...p, userDataRoot: e.target.value }))} placeholder="data" />
          </FormItem>
          <FormItem label="默认指纹参数（每行一个）">
            <Textarea value={fingerprintText} onChange={e => setFingerprintText(e.target.value)} rows={3} placeholder="--fingerprint-brand=Chrome" />
          </FormItem>
          <FormItem label="默认启动参数（每行一个）">
            <Textarea value={launchText} onChange={e => setLaunchText(e.target.value)} rows={3} placeholder="--disable-sync" />
          </FormItem>
          <FormItem label="默认代理">
            <Input value={settings.defaultProxy} onChange={e => setSettings(p => ({ ...p, defaultProxy: e.target.value }))} placeholder="http://127.0.0.1:7890" />
          </FormItem>

          <FrontProxySettings
            enabled={settings.frontProxyEnabled}
            auto={settings.frontProxyAuto}
            addr={settings.frontProxyAddr}
            onChange={patch => setSettings(p => ({ ...p, ...patch }))}
          />
        </div>
      </Modal>

      <Modal open={coreModalOpen} onClose={() => setCoreModalOpen(false)} title={coreForm.coreId ? '编辑内核' : '新增内核'} width="500px"
        footer={<><Button variant="secondary" onClick={() => setCoreModalOpen(false)}>取消</Button><Button onClick={handleSaveCore} loading={savingCore}>保存</Button></>}>
        <div className="space-y-4">
          <FormItem label="内核名称" required>
            <Input value={coreForm.coreName} onChange={e => setCoreForm(p => ({ ...p, coreName: e.target.value }))} placeholder="Chrome 142" />
          </FormItem>
          <FormItem label="内核路径" required>
            <div className="flex gap-2">
              <Input value={coreForm.corePath} onChange={e => { setCoreForm(p => ({ ...p, corePath: e.target.value })); setCoreValidation(null) }} placeholder="chrome 或 D:/browsers/chrome-120" className="flex-1" />
              <Button variant="secondary" onClick={handleValidateCorePath}>验证</Button>
            </div>
            {coreValidation && (
              <div className={`flex items-center gap-1 mt-1 text-sm ${coreValidation.valid ? 'text-green-600' : 'text-red-600'}`}>
                {coreValidation.valid ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                {coreValidation.message}
              </div>
            )}
          </FormItem>
        </div>
      </Modal>
    </>
  )
}
