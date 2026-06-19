import { useEffect, useRef, useState } from 'react'
import { Plus, Trash2, Edit2, Cookie, Save, X, RefreshCw, Search, CheckSquare, Download, Upload, ArrowDownToLine, AlertTriangle } from 'lucide-react'
import { Badge, Button, Card, FormItem, Input, Modal, Textarea, toast, Select, useConfirm } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import {
  fetchBrowserProfiles,
  fetchAccounts,
  fetchAccount,
  createAccount,
  updateAccount,
  deleteAccount,
  saveAccountCookiesFromProfile,
  restoreAccountCookiesToProfile,
  exportAccounts,
  importAccounts,
  type BrowserAccount,
  type BrowserAccountInput,
} from '../api'
import { PLATFORM_PRESETS, platformIcon } from '../config/platformPresets'
import { requireFields } from '../../../shared/utils/validate'
import { CookieEditorModal } from '../components/CookieEditorModal'

type Account = BrowserAccount

export function AccountManagementPage() {
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [accounts, setAccounts] = useState<Account[]>([])
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingAccount, setEditingAccount] = useState<Account | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedAccounts, setSelectedAccounts] = useState<string[]>([])

  // Cookie 相关
  const [cookieModalOpen, setCookieModalOpen] = useState(false)
  const [currentAccountId, setCurrentAccountId] = useState('')
  const [editingCookies, setEditingCookies] = useState('')

  // 导入/导出
  const [exportModalOpen, setExportModalOpen] = useState(false)
  const [exportIncludeSecrets, setExportIncludeSecrets] = useState(false)
  const importInputRef = useRef<HTMLInputElement | null>(null)

  // 表单状态
  const [formData, setFormData] = useState({
    accountName: '',
    username: '',
    email: '',
    password: '',
    platform: 'other',
    platformUrl: '',
    platformIcon: '🌐',
    relatedProfileIds: [] as string[],
    notes: '',
  })

  useEffect(() => {
    loadAccounts()
    loadProfiles()
  }, [])

  const loadAccounts = async () => {
    setLoading(true)
    try {
      const data = await fetchAccounts()
      setAccounts(data)
    } catch (error: any) {
      toast.error(error?.message || '加载账号列表失败')
    } finally {
      setLoading(false)
    }
  }

  const loadProfiles = async () => {
    try {
      setProfiles(await fetchBrowserProfiles())
    } catch (error: any) {
      console.error('加载浏览器实例失败:', error)
    }
  }

  const handleOpenModal = (account?: Account) => {
    if (account) {
      setEditingAccount(account)
      setFormData({
        accountName: account.accountName,
        username: account.username,
        email: account.email,
        password: account.password,
        platform: account.platform || 'other',
        platformUrl: '',
        platformIcon: platformIcon(account.platform),
        relatedProfileIds: account.relatedProfileIds || [],
        notes: account.notes,
      })
    } else {
      setEditingAccount(null)
      setFormData({
        accountName: '', username: '', email: '', password: '',
        platform: 'other', platformUrl: '', platformIcon: '🌐',
        relatedProfileIds: [], notes: '',
      })
    }
    setModalOpen(true)
  }

  const handleCloseModal = () => {
    setModalOpen(false)
    setEditingAccount(null)
  }

  const handleSave = async () => {
    const validationError = requireFields([{ value: formData.accountName, label: '账号名称' }])
    if (validationError) {
      toast.error(validationError)
      return
    }
    try {
      const input: BrowserAccountInput = {
        accountName: formData.accountName,
        platform: formData.platform,
        username: formData.username,
        email: formData.email,
        password: formData.password,
        relatedProfileIds: formData.relatedProfileIds,
        notes: formData.notes,
        cookies: '', // Cookie 在 Cookie 编辑器/同步中单独处理
      }
      if (editingAccount) {
        // 保留已存 Cookie：更新走 update（cookies 传空会清空），故改名/关联用 update，但 cookies 用 setProfiles + 不动 cookies
        // 为避免清空已存 Cookie，这里关联用专用接口，其余字段用 update 但带回原 cookies。
        const full = await fetchAccount(editingAccount.accountId)
        input.cookies = full?.cookies || ''
        const updated = await updateAccount(editingAccount.accountId, input)
        if (updated) {
          setAccounts(accounts.map(a => a.accountId === editingAccount.accountId ? updated : a))
          toast.success('账号更新成功')
        }
      } else {
        const newAccount = await createAccount(input)
        if (newAccount) {
          setAccounts([newAccount, ...accounts])
          toast.success('账号添加成功')
        }
      }
      handleCloseModal()
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    }
  }

  const handleDelete = async (accountId: string) => {
    if (!(await confirm({ content: '确定要删除这个账号吗？', danger: true }))) return
    try {
      if (await deleteAccount(accountId)) {
        setAccounts(accounts.filter(a => a.accountId !== accountId))
        setSelectedAccounts(prev => prev.filter(id => id !== accountId))
        toast.success('删除成功')
      }
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
  }

  const handlePlatformChange = (platform: string) => {
    const preset = PLATFORM_PRESETS.find(p => p.value === platform)
    setFormData(prev => ({
      ...prev,
      platform,
      platformUrl: preset?.url || '',
      platformIcon: preset?.icon || '🌐',
    }))
  }

  const toggleRelatedProfile = (profileId: string) => {
    setFormData(prev => ({
      ...prev,
      relatedProfileIds: prev.relatedProfileIds.includes(profileId)
        ? prev.relatedProfileIds.filter(id => id !== profileId)
        : [...prev.relatedProfileIds, profileId],
    }))
  }

  const handleToggleSelect = (accountId: string) => {
    setSelectedAccounts(prev =>
      prev.includes(accountId) ? prev.filter(id => id !== accountId) : [...prev, accountId]
    )
  }

  const handleSelectAll = () => {
    if (selectedAccounts.length === filteredAccounts.length) setSelectedAccounts([])
    else setSelectedAccounts(filteredAccounts.map(a => a.accountId))
  }

  // 返回该账号第一个运行中且就绪的关联实例
  const runningRelatedProfile = (account: Account): BrowserProfile | null => {
    for (const id of account.relatedProfileIds || []) {
      const p = profiles.find(pp => pp.profileId === id)
      if (p && p.running && p.debugReady) return p
    }
    return null
  }

  // Cookie 编辑弹窗
  const handleOpenCookieModal = async (account: Account) => {
    try {
      const full = await fetchAccount(account.accountId)
      if (full) {
        setCurrentAccountId(account.accountId)
        setEditingCookies(full.cookies)
        setCookieModalOpen(true)
      }
    } catch (error: any) {
      toast.error(error?.message || '获取 Cookie 失败')
    }
  }

  const handleSaveCookies = async (cookieText: string) => {
    try {
      const account = accounts.find(a => a.accountId === currentAccountId)
      if (!account) return
      const input: BrowserAccountInput = {
        accountName: account.accountName,
        platform: account.platform,
        username: account.username,
        email: account.email,
        password: (await fetchAccount(currentAccountId))?.password || '',
        relatedProfileIds: account.relatedProfileIds || [],
        notes: account.notes,
        cookies: cookieText,
      }
      const updated = await updateAccount(currentAccountId, input)
      if (updated) {
        setAccounts(accounts.map(a => a.accountId === currentAccountId ? updated : a))
        toast.success('Cookie 保存成功')
        setCookieModalOpen(false)
      }
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    }
  }

  // 从运行中实例读取 Cookie 并保存（JSON，供回写/过期解析）
  const handleExtractCookies = async () => {
    const account = accounts.find(a => a.accountId === currentAccountId)
    if (!account) return
    const profile = runningRelatedProfile(account)
    if (!profile) {
      toast.error('请先关联一个运行中的实例')
      return
    }
    try {
      const count = await saveAccountCookiesFromProfile(currentAccountId, profile.profileId)
      const full = await fetchAccount(currentAccountId)
      if (full) setEditingCookies(full.cookies)
      await loadAccounts()
      toast.success(`已从实例读取并保存 ${count} 条 Cookie`)
    } catch (error: any) {
      toast.error(error?.message || '提取 Cookie 失败')
    }
  }

  // 回写 Cookie 到运行中实例
  const handleRestoreCookies = async (account: Account) => {
    if (!account.cookieCount) {
      toast.error('该账号未保存 Cookie')
      return
    }
    const profile = runningRelatedProfile(account)
    if (!profile) {
      toast.error('请先启动一个已关联的实例')
      return
    }
    try {
      const count = await restoreAccountCookiesToProfile(account.accountId, profile.profileId, true)
      toast.success(`已回写 ${count} 条 Cookie 到「${profile.profileName}」`)
    } catch (error: any) {
      toast.error(error?.message || '回写 Cookie 失败')
    }
  }

  const canExtractCookies = () => {
    const account = accounts.find(a => a.accountId === currentAccountId)
    return !!account && !!runningRelatedProfile(account)
  }

  // 导出
  const handleConfirmExport = async () => {
    try {
      const ids = selectedAccounts.length > 0 ? selectedAccounts : []
      const json = await exportAccounts(ids, exportIncludeSecrets)
      const blob = new Blob([json], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `accounts_${new Date().toISOString().slice(0, 10)}${exportIncludeSecrets ? '_secrets' : ''}.json`
      a.click()
      URL.revokeObjectURL(url)
      setExportModalOpen(false)
      toast.success(`已导出 ${ids.length > 0 ? ids.length : accounts.length} 个账号`)
    } catch (error: any) {
      toast.error(error?.message || '导出失败')
    }
  }

  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = '' // 允许重复选择同一文件
    if (!file) return
    try {
      const text = await file.text()
      const count = await importAccounts(text)
      await loadAccounts()
      toast.success(`成功导入 ${count} 个账号`)
    } catch (error: any) {
      toast.error(error?.message || '导入失败')
    }
  }

  const filteredAccounts = accounts.filter(acc =>
    acc.accountName.toLowerCase().includes(searchQuery.toLowerCase()) ||
    acc.username.toLowerCase().includes(searchQuery.toLowerCase()) ||
    acc.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
    acc.platform.toLowerCase().includes(searchQuery.toLowerCase())
  )

  return (
    <div className="overflow-auto p-5 space-y-5 animate-fade-in h-full">
      {confirmDialog}
      {/* 页头 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">平台账号</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">管理平台账号、关联实例并同步 Cookie</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => importInputRef.current?.click()}>
            <Upload className="w-4 h-4" />导入
          </Button>
          <Button variant="secondary" size="sm" onClick={() => { setExportIncludeSecrets(false); setExportModalOpen(true) }}>
            <Download className="w-4 h-4" />导出
          </Button>
          <Button variant="secondary" size="sm" onClick={loadAccounts} loading={loading}>
            <RefreshCw className="w-4 h-4" />刷新
          </Button>
          <Button size="sm" onClick={() => handleOpenModal()}>
            <Plus className="w-4 h-4" />添加账号
          </Button>
        </div>
      </div>
      <input ref={importInputRef} type="file" accept=".json,application/json" className="hidden" onChange={handleImportFile} />

      {/* 搜索栏 */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
          <Input value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} placeholder="搜索账号 / 用户名 / 邮箱 / 平台" className="pl-10" />
        </div>
      </div>

      {filteredAccounts.length === 0 ? (
        <Card padding="lg" className="flex flex-col items-center justify-center text-center min-h-[400px]">
          <div className="w-16 h-16 mb-4 rounded-2xl bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/30 dark:to-blue-800/20 flex items-center justify-center">
            <CheckSquare className="w-8 h-8 text-blue-600" />
          </div>
          <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">暂无平台账号</h3>
          <p className="text-sm text-[var(--color-text-muted)] mb-4 max-w-xs">添加平台账号以管理并与实例绑定、同步 Cookie</p>
          <Button size="sm" onClick={() => handleOpenModal()}><Plus className="w-4 h-4" />添加账号</Button>
        </Card>
      ) : (
        <Card padding="md">
          <div className="space-y-3">
            <div className="flex items-center gap-3 pb-3 border-b border-[var(--color-border-default)] text-sm font-medium text-[var(--color-text-muted)]">
              <div className="w-8">
                <input type="checkbox" checked={selectedAccounts.length === filteredAccounts.length && filteredAccounts.length > 0} onChange={handleSelectAll} className="w-4 h-4" />
              </div>
              <div className="flex-1">账号信息</div>
              <div className="w-28">Cookie</div>
              <div className="w-40">已关联实例</div>
              <div className="w-28 text-right">操作</div>
            </div>

            <div className="space-y-2">
              {filteredAccounts.map(account => {
                const related = (account.relatedProfileIds || [])
                  .map(id => profiles.find(p => p.profileId === id))
                  .filter((p): p is BrowserProfile => !!p)
                const isSelected = selectedAccounts.includes(account.accountId)
                return (
                  <div key={account.accountId} className="flex items-center gap-3 p-3 rounded-lg hover:bg-[var(--color-bg-muted)] transition-colors border border-transparent hover:border-[var(--color-border-default)]">
                    <div className="w-8">
                      <input type="checkbox" checked={isSelected} onChange={() => handleToggleSelect(account.accountId)} className="w-4 h-4" />
                    </div>
                    <div className="flex-1 flex items-center gap-3 min-w-0">
                      <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/30 dark:to-blue-800/20 flex items-center justify-center flex-shrink-0 text-xl">
                        {platformIcon(account.platform)}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="font-medium text-[var(--color-text-primary)] truncate">{account.accountName}</div>
                        <div className="text-sm text-[var(--color-text-secondary)] truncate">{account.email || account.username || account.platform}</div>
                      </div>
                    </div>
                    <div className="w-28"><CookieExpiryBadge account={account} /></div>
                    <div className="w-40 flex flex-wrap gap-1">
                      {related.length === 0 ? (
                        <span className="text-sm text-[var(--color-text-muted)]">未关联</span>
                      ) : (
                        <>
                          {related.slice(0, 2).map(p => (
                            <Badge key={p.profileId} variant={p.running ? 'success' : 'default'}>{p.profileName}</Badge>
                          ))}
                          {related.length > 2 && <Badge variant="default">+{related.length - 2}</Badge>}
                        </>
                      )}
                    </div>
                    <div className="w-28 flex justify-end gap-1">
                      <Button size="sm" variant="ghost" onClick={() => handleRestoreCookies(account)} title="回写 Cookie 到运行中的关联实例">
                        <ArrowDownToLine className="w-3.5 h-3.5" />
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => handleOpenCookieModal(account)} title="管理 Cookie">
                        <Cookie className="w-3.5 h-3.5" />
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => handleOpenModal(account)} title="编辑">
                        <Edit2 className="w-3.5 h-3.5" />
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => handleDelete(account.accountId)} title="删除">
                        <Trash2 className="w-3.5 h-3.5 text-red-500" />
                      </Button>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </Card>
      )}

      <div className="text-sm text-[var(--color-text-muted)]">共 {filteredAccounts.length} 个账号</div>

      {/* 账号编辑弹窗 */}
      <Modal open={modalOpen} onClose={handleCloseModal} title={editingAccount ? '编辑账号' : '添加账号'} width="600px">
        <div className="space-y-4 py-4">
          <FormItem label="账号名称" required>
            <Input value={formData.accountName} onChange={e => setFormData(prev => ({ ...prev, accountName: e.target.value }))} placeholder="例如：Twitter主号" />
          </FormItem>
          <FormItem label="平台" required>
            <Select value={formData.platform} onChange={e => handlePlatformChange(e.target.value)} options={PLATFORM_PRESETS.map(p => ({ value: p.value, label: p.label }))} />
          </FormItem>
          {formData.platform === 'other' && (
            <FormItem label="平台 URL">
              <Input value={formData.platformUrl} onChange={e => setFormData(prev => ({ ...prev, platformUrl: e.target.value }))} placeholder="https://example.com" />
            </FormItem>
          )}
          <div className="grid grid-cols-2 gap-3">
            <FormItem label="用户名">
              <Input value={formData.username} onChange={e => setFormData(prev => ({ ...prev, username: e.target.value }))} placeholder="username" />
            </FormItem>
            <FormItem label="邮箱">
              <Input value={formData.email} onChange={e => setFormData(prev => ({ ...prev, email: e.target.value }))} placeholder="user@example.com" />
            </FormItem>
          </div>
          <FormItem label="密码">
            <Input type="password" value={formData.password} onChange={e => setFormData(prev => ({ ...prev, password: e.target.value }))} placeholder="登录密码" />
          </FormItem>
          <FormItem label="关联实例（可多选）">
            {profiles.length === 0 ? (
              <span className="text-sm text-[var(--color-text-muted)]">暂无实例</span>
            ) : (
              <div className="max-h-40 overflow-y-auto rounded-md border border-[var(--color-border-default)] divide-y divide-[var(--color-border-default)]">
                {profiles.map(p => (
                  <label key={p.profileId} className="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-[var(--color-bg-muted)]">
                    <input type="checkbox" checked={formData.relatedProfileIds.includes(p.profileId)} onChange={() => toggleRelatedProfile(p.profileId)} className="w-4 h-4" />
                    <span className="text-sm text-[var(--color-text-primary)]">{p.profileName}</span>
                    {p.running && <Badge variant="success">运行中</Badge>}
                  </label>
                ))}
              </div>
            )}
          </FormItem>
          <FormItem label="备注">
            <Textarea rows={3} value={formData.notes} onChange={e => setFormData(prev => ({ ...prev, notes: e.target.value }))} placeholder="其他备注信息" />
          </FormItem>
        </div>
        <div className="flex justify-end gap-2 pt-4 border-t border-[var(--color-border-default)]">
          <Button variant="secondary" onClick={handleCloseModal}><X className="w-4 h-4" />取消</Button>
          <Button onClick={handleSave}><Save className="w-4 h-4" />保存</Button>
        </div>
      </Modal>

      {/* 导出选项弹窗 */}
      <Modal open={exportModalOpen} onClose={() => setExportModalOpen(false)} title="导出账号" width="460px">
        <div className="space-y-4 py-4">
          <p className="text-sm text-[var(--color-text-secondary)]">
            将导出{selectedAccounts.length > 0 ? `选中的 ${selectedAccounts.length}` : `全部 ${accounts.length}`} 个账号为 JSON 文件。
          </p>
          <label className="flex items-start gap-2 cursor-pointer">
            <input type="checkbox" checked={exportIncludeSecrets} onChange={e => setExportIncludeSecrets(e.target.checked)} className="w-4 h-4 mt-0.5" />
            <span className="text-sm text-[var(--color-text-primary)]">包含敏感数据（密码 / Cookie）</span>
          </label>
          {exportIncludeSecrets && (
            <div className="flex items-start gap-2 p-3 rounded-md bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800">
              <AlertTriangle className="w-4 h-4 text-red-500 mt-0.5 flex-shrink-0" />
              <span className="text-xs text-red-600 dark:text-red-400">
                导出文件将包含明文密码与 Cookie，任何获得该文件的人都能登录这些账号。请妥善保管，切勿分享或上传到不可信位置。
              </span>
            </div>
          )}
        </div>
        <div className="flex justify-end gap-2 pt-4 border-t border-[var(--color-border-default)]">
          <Button variant="secondary" onClick={() => setExportModalOpen(false)}><X className="w-4 h-4" />取消</Button>
          <Button onClick={handleConfirmExport}><Download className="w-4 h-4" />{exportIncludeSecrets ? '确认导出（含敏感）' : '导出'}</Button>
        </div>
      </Modal>

      {/* Cookie 编辑弹窗 */}
      <CookieEditorModal
        open={cookieModalOpen}
        onClose={() => setCookieModalOpen(false)}
        initialCookies={editingCookies}
        onSave={handleSaveCookies}
        onExtract={handleExtractCookies}
        canExtract={canExtractCookies()}
      />
    </div>
  )
}

function CookieExpiryBadge({ account }: { account: BrowserAccount }) {
  const count = account.cookieCount || 0
  const earliest = account.cookieEarliestExpiry || 0
  if (count === 0) return <span className="text-sm text-[var(--color-text-muted)]">无</span>
  if (earliest > 0) {
    const nowSec = Date.now() / 1000
    if (earliest < nowSec) return <Badge variant="error">已过期</Badge>
    if (earliest < nowSec + 7 * 86400) return <Badge variant="warning">{count} · 临期</Badge>
  }
  return <Badge variant="info">{count}</Badge>
}
