import { useEffect, useState } from 'react'
import { Plus, Trash2, Edit2, RefreshCw, Link as LinkIcon, ExternalLink, Star, Users, Package, Download, CheckCircle2, XCircle } from 'lucide-react'
import { Badge, Button, Card, FormItem, Input, Modal, Switch, Table, Textarea, toast, Select, useConfirm } from '../../../shared/components'
import { requireFields } from '../../../shared/utils/validate'
import { useBrowserProfiles } from '../hooks/useBrowserProfiles'
import type { TableColumn } from '../../../shared/components/Table'
import {
  fetchExtensions,
  createExtension,
  updateExtension,
  deleteExtension,
  toggleExtension,
  setExtensionProfiles,
  validateExtensionPath,
  type BrowserExtension,
  type BrowserExtensionInput,
  type ExtensionValidateResult,
} from '../api'

type TabType = 'installed' | 'store'

const LEGACY_STORAGE_KEY = 'ant-browser:extensions'

const SOURCE_OPTIONS = [
  { value: 'local', label: '本地（解压目录）' },
  { value: 'store', label: '商店' },
  { value: 'builtin', label: '内置推荐' },
]

function sourceBadge(sourceType: string) {
  const s = (sourceType || 'local').toLowerCase()
  if (s === 'store') return <Badge variant="success">商店</Badge>
  if (s === 'builtin') return <Badge variant="info">内置推荐</Badge>
  return <Badge variant="default">本地</Badge>
}

// 扩展商店推荐（内置推荐，仅引用：商店扩展无法经 --load-extension 加载）
const POPULAR_EXTENSIONS = [
  { id: 'cjpalhdlnbpafiamejdnhcphjbkeiagm', name: 'uBlock Origin', description: '高效的请求过滤工具：占用极低的内存和CPU', category: '生产工具', rating: 4.8, users: '10,000,000+' },
  { id: 'nngceckbapebfimnlniiiahkandclblb', name: 'Bitwarden', description: '适合个人、团队与商业组织使用的安全且免费的密码管理器', category: '生产工具', rating: 4.7, users: '3,000,000+' },
  { id: 'eimadpbcbfnmbkopoojfekhnkhdbieeh', name: 'Dark Reader', description: '适用于任何网站的黑暗主题，关爱眼睛', category: '无障碍', rating: 4.6, users: '5,000,000+' },
  { id: 'dhdgffkkebhmkfjojejmpbldmpobfkfo', name: 'Tampermonkey', description: '世界上最流行的用户脚本管理器', category: '生产工具', rating: 4.7, users: '10,000,000+' },
]

export function ExtensionManagementPage() {
  const { confirm, dialog: confirmDialog } = useConfirm()
  const { profiles, reload: reloadProfiles } = useBrowserProfiles(false)
  const [extensions, setExtensions] = useState<BrowserExtension[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingExtension, setEditingExtension] = useState<BrowserExtension | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [bindModalOpen, setBindModalOpen] = useState(false)
  const [bindingExtensionId, setBindingExtensionId] = useState('')
  const [activeTab, setActiveTab] = useState<TabType>('installed')
  const [storeSearchQuery, setStoreSearchQuery] = useState('')
  const [saving, setSaving] = useState(false)
  const [pathCheck, setPathCheck] = useState<ExtensionValidateResult | null>(null)

  const [formData, setFormData] = useState({
    extensionName: '',
    version: '',
    description: '',
    extensionPath: '',
    sourceType: 'local',
  })

  useEffect(() => {
    void init()
  }, [])

  const init = async () => {
    await migrateLegacyExtensions()
    await loadExtensions()
    await loadProfiles()
  }

  // 一次性把旧 localStorage 扩展迁移到后端（仅当后端为空且本地有数据）
  const migrateLegacyExtensions = async () => {
    try {
      const raw = localStorage.getItem(LEGACY_STORAGE_KEY)
      if (!raw) return
      const legacy = JSON.parse(raw)
      if (!Array.isArray(legacy) || legacy.length === 0) {
        localStorage.removeItem(LEGACY_STORAGE_KEY)
        return
      }
      const existing = await fetchExtensions()
      if (existing.length > 0) {
        localStorage.removeItem(LEGACY_STORAGE_KEY)
        return
      }
      let migrated = 0
      for (const e of legacy) {
        const input: BrowserExtensionInput = {
          extensionName: e.extensionName || '未命名扩展',
          extensionPath: e.path || e.extensionPath || '',
          version: e.version || '',
          enabled: e.enabled ?? true,
          boundProfileIds: e.profileIds || e.boundProfileIds || [],
          sourceType: e.source === 'google' ? 'store' : (e.sourceType || 'local'),
          sourceUrl: e.sourceUrl || '',
          description: e.description || '',
        }
        try {
          await createExtension(input)
          migrated++
        } catch {
          // 无效路径等：跳过，不阻断迁移
        }
      }
      localStorage.removeItem(LEGACY_STORAGE_KEY)
      if (migrated > 0) toast.success(`已从本地迁移 ${migrated} 个扩展到数据库`)
    } catch {
      // ignore
    }
  }

  const loadExtensions = async () => {
    setLoading(true)
    try {
      setExtensions(await fetchExtensions())
    } catch (error: any) {
      toast.error(error?.message || '加载扩展列表失败')
    } finally {
      setLoading(false)
    }
  }

  const loadProfiles = async () => {
    try {
      await reloadProfiles()
    } catch (error: any) {
      console.error('加载浏览器窗口失败:', error)
    }
  }

  const handleOpenModal = (extension?: BrowserExtension) => {
    setPathCheck(null)
    if (extension) {
      setEditingExtension(extension)
      setFormData({
        extensionName: extension.extensionName,
        version: extension.version,
        description: extension.description,
        extensionPath: extension.extensionPath,
        sourceType: extension.sourceType || 'local',
      })
    } else {
      setEditingExtension(null)
      setFormData({ extensionName: '', version: '', description: '', extensionPath: '', sourceType: 'local' })
    }
    setModalOpen(true)
  }

  const handleCloseModal = () => {
    setModalOpen(false)
    setEditingExtension(null)
  }

  // 校验扩展目录并回填名称/版本
  const handleCheckPath = async () => {
    if (!formData.extensionPath.trim()) {
      setPathCheck(null)
      return
    }
    const res = await validateExtensionPath(formData.extensionPath)
    setPathCheck(res)
    if (res.valid) {
      setFormData(prev => ({
        ...prev,
        extensionName: prev.extensionName || res.name,
        version: prev.version || res.version,
      }))
    }
  }

  const handleSave = async () => {
    const validationError = requireFields([
      { value: formData.extensionName, label: '扩展名称' },
      { value: formData.extensionPath, label: '扩展路径' },
    ])
    if (validationError) {
      toast.error(validationError)
      return
    }
    setSaving(true)
    try {
      const base = editingExtension
      const input: BrowserExtensionInput = {
        extensionName: formData.extensionName,
        extensionPath: formData.extensionPath,
        version: formData.version,
        enabled: base ? base.enabled : true,
        boundProfileIds: base ? base.boundProfileIds : [],
        sourceType: formData.sourceType,
        sourceUrl: base?.sourceUrl || '',
        description: formData.description,
      }
      if (editingExtension) {
        const updated = await updateExtension(editingExtension.extensionId, input)
        if (updated) {
          setExtensions(extensions.map(e => e.extensionId === editingExtension.extensionId ? updated : e))
          toast.success('扩展更新成功')
        }
      } else {
        const created = await createExtension(input)
        if (created) {
          setExtensions([created, ...extensions])
          toast.success('扩展添加成功')
        }
      }
      handleCloseModal()
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (extensionId: string) => {
    if (!(await confirm({ content: '确定要删除这个扩展吗？', danger: true }))) return
    try {
      if (await deleteExtension(extensionId)) {
        setExtensions(extensions.filter(e => e.extensionId !== extensionId))
        toast.success('删除成功')
      }
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
  }

  const handleToggleEnabled = async (ext: BrowserExtension) => {
    const next = !ext.enabled
    try {
      await toggleExtension(ext.extensionId, next)
      setExtensions(extensions.map(e => e.extensionId === ext.extensionId ? { ...e, enabled: next } : e))
    } catch (error: any) {
      toast.error(error?.message || '切换状态失败')
    }
  }

  const handleOpenBindModal = (extensionId: string) => {
    setBindingExtensionId(extensionId)
    setBindModalOpen(true)
  }

  const handleToggleProfileBinding = async (profileId: string) => {
    const ext = extensions.find(e => e.extensionId === bindingExtensionId)
    if (!ext) return
    const next = ext.boundProfileIds.includes(profileId)
      ? ext.boundProfileIds.filter(id => id !== profileId)
      : [...ext.boundProfileIds, profileId]
    // 乐观更新
    setExtensions(extensions.map(e => e.extensionId === bindingExtensionId ? { ...e, boundProfileIds: next } : e))
    try {
      await setExtensionProfiles(bindingExtensionId, next)
    } catch (error: any) {
      toast.error(error?.message || '更新绑定失败')
      await loadExtensions()
    }
  }

  const filteredExtensions = extensions.filter(ext =>
    ext.extensionName.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const filteredStoreExtensions = POPULAR_EXTENSIONS.filter(ext =>
    ext.name.toLowerCase().includes(storeSearchQuery.toLowerCase()) ||
    ext.description.toLowerCase().includes(storeSearchQuery.toLowerCase())
  )

  const handleOpenInWebStore = (extensionId: string) => {
    window.open(`https://chrome.google.com/webstore/detail/${extensionId}`, '_blank')
  }
  const handleCopyId = (extensionId: string) => {
    navigator.clipboard.writeText(extensionId)
    toast.success('扩展ID已复制到剪贴板')
  }

  const columns: TableColumn<BrowserExtension>[] = [
    {
      key: 'extensionName',
      title: '扩展信息',
      render: (_, record) => (
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center flex-shrink-0">
            <Download className="w-5 h-5 text-blue-600" />
          </div>
          <div className="min-w-0">
            <div className="font-medium text-[var(--color-text-primary)] truncate">{record.extensionName}</div>
            <div className="text-xs text-[var(--color-text-muted)] truncate" title={record.extensionPath}>v{record.version || '未知'} · {record.extensionPath}</div>
          </div>
        </div>
      ),
    },
    { key: 'sourceType', title: '来源', width: '110px', render: (value) => sourceBadge(value as string) },
    {
      key: 'enabled',
      title: '启用',
      width: '80px',
      render: (_, record) => <Switch checked={record.enabled} onChange={() => handleToggleEnabled(record)} />,
    },
    {
      key: 'boundProfileIds',
      title: '已绑定窗口',
      width: '120px',
      render: (value) => {
        const count = (value as string[]).length
        return count > 0 ? <Badge>{count} 个窗口</Badge> : <span className="text-sm text-[var(--color-text-muted)]">未绑定</span>
      },
    },
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      width: '140px',
      render: (_, record) => (
        <div className="flex justify-end gap-1">
          <Button size="sm" variant="ghost" onClick={() => handleOpenBindModal(record.extensionId)} title="绑定窗口"><LinkIcon className="w-3.5 h-3.5" /></Button>
          <Button size="sm" variant="ghost" onClick={() => handleOpenModal(record)} title="编辑"><Edit2 className="w-3.5 h-3.5" /></Button>
          <Button size="sm" variant="ghost" onClick={() => handleDelete(record.extensionId)} title="删除"><Trash2 className="w-3.5 h-3.5 text-red-500" /></Button>
        </div>
      ),
    },
  ]

  return (
    <div className="overflow-auto p-5 space-y-5 animate-fade-in h-full">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">扩展管理</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">本地扩展绑定到窗口后，启动浏览器会自动通过 --load-extension 加载</p>
        </div>
        <div className="flex gap-2">
          {activeTab === 'installed' && (
            <>
              <Button variant="secondary" size="sm" onClick={loadExtensions} loading={loading}><RefreshCw className="w-4 h-4" />刷新</Button>
              <Button size="sm" onClick={() => handleOpenModal()}><Plus className="w-4 h-4" />添加扩展</Button>
            </>
          )}
        </div>
      </div>

      <Card padding="none">
        <div className="flex border-b border-[var(--color-border-default)]">
          <button className={`px-6 py-3 text-sm font-medium transition-colors ${activeTab === 'installed' ? 'text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}`} onClick={() => setActiveTab('installed')}>已安装扩展</button>
          <button className={`px-6 py-3 text-sm font-medium transition-colors ${activeTab === 'store' ? 'text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]' : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'}`} onClick={() => setActiveTab('store')}>扩展推荐</button>
        </div>
      </Card>

      {activeTab === 'installed' && (
        <>
          <Card padding="md">
            <Input value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} placeholder="搜索扩展名称..." />
          </Card>
          <Card padding="none">
            <div className="overflow-auto" style={{ maxHeight: 'calc(100vh - 460px)' }}>
              {loading ? (
                <div className="py-20 flex flex-col items-center justify-center gap-3">
                  <div className="w-8 h-8 border-4 border-[var(--color-border-default)] border-t-[var(--color-accent)] rounded-full animate-spin"></div>
                  <p className="text-sm text-[var(--color-text-muted)]">加载中...</p>
                </div>
              ) : filteredExtensions.length === 0 ? (
                <div className="py-20 flex flex-col items-center justify-center gap-4">
                  <div className="w-16 h-16 rounded-2xl bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center">
                    <Download className="w-8 h-8 text-blue-600" />
                  </div>
                  <div className="text-center">
                    <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-1">{searchQuery ? '未找到扩展' : '还没有扩展'}</h3>
                    <p className="text-sm text-[var(--color-text-muted)] mb-4">{searchQuery ? '尝试其他搜索关键词' : '添加解压后的本地扩展目录并绑定到窗口'}</p>
                    {!searchQuery && <Button size="sm" onClick={() => handleOpenModal()}><Plus className="w-4 h-4" />添加扩展</Button>}
                  </div>
                </div>
              ) : (
                <Table columns={columns} data={filteredExtensions} rowKey="extensionId" />
              )}
            </div>
          </Card>
          <div className="text-sm text-[var(--color-text-muted)]">共 {filteredExtensions.length} 个扩展</div>
        </>
      )}

      {activeTab === 'store' && (
        <>
          <Card padding="md">
            <Input value={storeSearchQuery} onChange={(e) => setStoreSearchQuery(e.target.value)} placeholder="搜索扩展名称..." />
          </Card>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filteredStoreExtensions.map(ext => (
              <Card key={ext.id} padding="md" className="hover:shadow-lg transition-shadow">
                <div className="space-y-3">
                  <div className="flex items-start gap-3">
                    <div className="w-12 h-12 rounded-lg bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center flex-shrink-0">
                      <Package className="w-6 h-6 text-blue-600" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <h3 className="font-medium text-[var(--color-text-primary)] truncate">{ext.name}</h3>
                      <Badge variant="default" className="mt-1">{ext.category}</Badge>
                    </div>
                  </div>
                  <p className="text-sm text-[var(--color-text-secondary)] line-clamp-2 min-h-[2.5rem]">{ext.description}</p>
                  <div className="flex items-center gap-4 text-xs text-[var(--color-text-muted)]">
                    <div className="flex items-center gap-1"><Star className="w-3.5 h-3.5 text-yellow-500 fill-yellow-500" /><span>{ext.rating}</span></div>
                    <div className="flex items-center gap-1"><Users className="w-3.5 h-3.5" /><span>{ext.users}</span></div>
                  </div>
                  <div className="flex gap-2 pt-2">
                    <Button size="sm" variant="secondary" className="flex-1" onClick={() => handleOpenInWebStore(ext.id)}><ExternalLink className="w-3.5 h-3.5" />在商店中打开</Button>
                    <Button size="sm" variant="secondary" onClick={() => handleCopyId(ext.id)} title="复制扩展ID">ID</Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
          {filteredStoreExtensions.length === 0 && (
            <div className="py-20 text-center">
              <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-50 dark:bg-gray-900/30 flex items-center justify-center"><Package className="w-8 h-8 text-gray-400" /></div>
              <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-1">未找到扩展</h3>
              <p className="text-sm text-[var(--color-text-muted)]">尝试其他搜索关键词</p>
            </div>
          )}
        </>
      )}

      {/* 扩展编辑弹窗 */}
      <Modal open={modalOpen} onClose={handleCloseModal} title={editingExtension ? '编辑扩展' : '添加扩展'} width="600px">
        <div className="space-y-4 py-4">
          <FormItem label="扩展来源" required>
            <Select value={formData.sourceType} onChange={e => setFormData(prev => ({ ...prev, sourceType: e.target.value }))} options={SOURCE_OPTIONS} />
            {formData.sourceType !== 'local' && (
              <p className="text-xs text-[var(--color-text-muted)] mt-1">仅本地（解压目录）扩展会在启动时自动加载；商店/内置推荐仅作记录。</p>
            )}
          </FormItem>
          <FormItem label="扩展路径" required>
            <div className="flex gap-2">
              <Input value={formData.extensionPath} onChange={e => setFormData(prev => ({ ...prev, extensionPath: e.target.value }))} onBlur={handleCheckPath} placeholder="C:\\Extensions\\uBlock-Origin" />
              <Button variant="secondary" onClick={handleCheckPath}>校验</Button>
            </div>
            <p className="text-xs text-[var(--color-text-muted)] mt-1">解压后的 Chrome 扩展文件夹路径（需含 manifest.json）</p>
            {pathCheck && (
              <div className={`flex items-center gap-1.5 mt-1.5 text-xs ${pathCheck.valid ? 'text-green-600' : 'text-red-500'}`}>
                {pathCheck.valid ? <CheckCircle2 className="w-3.5 h-3.5" /> : <XCircle className="w-3.5 h-3.5" />}
                <span>{pathCheck.valid ? `有效：${pathCheck.name || ''} ${pathCheck.version ? 'v' + pathCheck.version : ''}` : pathCheck.message}</span>
              </div>
            )}
          </FormItem>
          <FormItem label="扩展名称" required>
            <Input value={formData.extensionName} onChange={e => setFormData(prev => ({ ...prev, extensionName: e.target.value }))} placeholder="例如：uBlock Origin" />
          </FormItem>
          <FormItem label="版本">
            <Input value={formData.version} onChange={e => setFormData(prev => ({ ...prev, version: e.target.value }))} placeholder="例如：1.50.0" />
          </FormItem>
          <FormItem label="描述">
            <Textarea rows={3} value={formData.description} onChange={e => setFormData(prev => ({ ...prev, description: e.target.value }))} placeholder="扩展的功能描述" />
          </FormItem>
        </div>
        <div className="flex justify-end gap-2 pt-4 border-t border-[var(--color-border-default)]">
          <Button variant="secondary" onClick={handleCloseModal}>取消</Button>
          <Button onClick={handleSave} loading={saving}>保存</Button>
        </div>
      </Modal>

      {/* 绑定窗口弹窗 */}
      <Modal open={bindModalOpen} onClose={() => setBindModalOpen(false)} title="绑定浏览器窗口" width="600px">
        <div className="space-y-4 py-4">
          <p className="text-sm text-[var(--color-text-secondary)]">选择要绑定此扩展的浏览器窗口；绑定后，下次启动这些窗口时会自动加载该扩展。</p>
          <div className="space-y-2 max-h-[400px] overflow-auto">
            {profiles.map(profile => {
              const ext = extensions.find(e => e.extensionId === bindingExtensionId)
              const isBinding = ext?.boundProfileIds.includes(profile.profileId) || false
              return (
                <div key={profile.profileId} className="flex items-center justify-between p-3 border border-[var(--color-border-default)] rounded-lg hover:bg-[var(--color-bg-muted)] transition-colors">
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-[var(--color-text-primary)]">{profile.profileName}</div>
                    <div className="text-xs text-[var(--color-text-muted)] truncate">{profile.profileId}</div>
                  </div>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" checked={isBinding} onChange={() => handleToggleProfileBinding(profile.profileId)} className="w-4 h-4" />
                    <span className="text-sm text-[var(--color-text-secondary)]">{isBinding ? '已绑定' : '未绑定'}</span>
                  </label>
                </div>
              )
            })}
          </div>
        </div>
        <div className="flex justify-end gap-2 pt-4 border-t border-[var(--color-border-default)]">
          <Button onClick={() => setBindModalOpen(false)}>完成</Button>
        </div>
      </Modal>
    </div>
  )
}
