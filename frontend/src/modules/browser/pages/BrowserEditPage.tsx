import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { FolderOpen, Layers, Package, Plus, RefreshCw, X } from 'lucide-react'
import { Button, Card, ConfirmModal, FormItem, Input, Modal, Select, Textarea, toast } from '../../../shared/components'
import type { BrowserCore, BrowserGroup, BrowserProfileInput, BrowserProxy, BrowserTemplate, CreateWindowFormState } from '../types'
import {
  createBrowserProfile,
  fetchAccounts,
  fetchAllTags,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchExtensions,
  fetchGroups,
  fetchTemplates,
  createTemplate,
  openUserDataDir,
  setExtensionProfiles,
  updateBrowserProfile,
} from '../api'
import type { BrowserAccount, BrowserExtension } from '../api'
import { FingerprintPanel } from '../components/FingerprintPanel'
import { randomFingerprintSeed } from '../utils/fingerprintSerializer'
import { TagInput } from '../components/TagInput'
import { GroupSelector } from '../components/GroupSelector'
import { ProxyPickerModal } from '../components/ProxyPickerModal'
import { ProxyImportModal } from '../components/ProxyImportModal'
import { AccountPickerModal } from '../components/AccountPickerModal'
import { AccountAddDrawer } from '../components/AccountAddDrawer'
import { platformIcon } from '../config/platformPresets'
import { BrowserCreateWorkstationPage } from './BrowserCreateWorkstationPage'
import { createWindowFormToProfileInput, restoreCreateWindowFormState, restoreFormStateFromTemplate } from '../utils/createWindowConverter'

const fallbackLowLaunchArgs = ['--disable-sync', '--no-first-run']
const BROWSER_LIST_ROUTE = '/browser/list'
const DIRECT_PROXY_ID = '__direct__'

function normalizeLaunchArgs(args: string[]): string[] {
  return (args || []).map(item => item.trim()).filter(Boolean)
}

function resolveDefaultLaunchArgs(args: string[]): string[] {
  const normalized = normalizeLaunchArgs(args)
  return normalized.length > 0 ? normalized : fallbackLowLaunchArgs
}

function isLoadableExtension(extension: BrowserExtension): boolean {
  return (
    extension.enabled !== false &&
    (extension.sourceType || 'local').toLowerCase() === 'local' &&
    Boolean(extension.extensionPath?.trim())
  )
}

function uniqueProfileIds(profileIds: string[]): string[] {
  return Array.from(new Set(profileIds.map(id => id.trim()).filter(Boolean)))
}

function getErrorMessage(error: unknown, fallback: string): string {
  return typeof error === 'string' ? error : (error as Error)?.message || fallback
}

export function BrowserEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isCreate = id === 'new'
  const [formData, setFormData] = useState<CreateWindowFormState>({
    profileName: '',
    userDataDir: '',
    coreId: '',
    fingerprintArgs: [],
    proxyId: '',
    proxyConfig: '',
    launchArgs: [],
    tags: [],
    keywords: [],
    groupId: '',
    accountIds: [],
  })
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroup[]>([])
  const [extensions, setExtensions] = useState<BrowserExtension[]>([])
  const [selectedExtensionIds, setSelectedExtensionIds] = useState<string[]>([])
  const [extensionsLoadError, setExtensionsLoadError] = useState('')
  const [launchArgsText, setLaunchArgsText] = useState('')
  const [allTags, setAllTags] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [proxyPickerOpen, setProxyPickerOpen] = useState(false)
  const [proxyImportOpen, setProxyImportOpen] = useState(false)
  const [isDirty, setIsDirty] = useState(false)
  const [leaveConfirm, setLeaveConfirm] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [templates, setTemplates] = useState<BrowserTemplate[]>([])
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [accounts, setAccounts] = useState<BrowserAccount[]>([])
  const [accountPickerOpen, setAccountPickerOpen] = useState(false)
  const [accountAddOpen, setAccountAddOpen] = useState(false)

  useEffect(() => {
    const loadData = async () => {
      setExtensionsLoadError('')
      const [coreList, proxyList, tagList, groupList, settings, extensionList, accountList] = await Promise.all([
        fetchBrowserCores(),
        fetchBrowserProxies(),
        fetchAllTags(),
        fetchGroups(),
        fetchBrowserSettings(),
        fetchExtensions().catch((error) => {
          setExtensionsLoadError(getErrorMessage(error, '扩展列表加载失败'))
          return [] as BrowserExtension[]
        }),
        fetchAccounts().catch(() => [] as BrowserAccount[]),
      ])
      const resolvedDefaultLaunchArgs = resolveDefaultLaunchArgs(settings.defaultLaunchArgs || [])
      setCores(coreList)
      setProxies(proxyList)
      setAllTags(tagList)
      setGroups(groupList)
      setExtensions(extensionList)
      setAccounts(accountList)

      if (isCreate) {
        setLaunchArgsText(resolvedDefaultLaunchArgs.join('\n'))
        setSelectedExtensionIds([])
        fetchTemplates().then(setTemplates).catch(() => setTemplates([]))
        // 新建：预填后端默认指纹（含 WebRTC 防泄露策略）并生成唯一随机种子
        const defaultFp = normalizeLaunchArgs(settings.defaultFingerprintArgs || [])
        setFormData(prev => ({
          ...prev,
          fingerprintArgs: [...defaultFp, `--fingerprint=${randomFingerprintSeed()}`],
        }))
        return
      }
      const list = await fetchBrowserProfiles()
      const current = list.find(item => item.profileId === id)
      if (!current) return
      const currentLaunchArgs = normalizeLaunchArgs(current.launchArgs)
      const normalizedCoreId = !current.coreId || current.coreId.toLowerCase() === 'default'
        ? ''
        : current.coreId
      setSelectedExtensionIds(extensionList
        .filter(extension => (
          isLoadableExtension(extension) &&
          (extension.boundProfileIds || []).includes(current.profileId)
        ))
        .map(extension => extension.extensionId)
      )
      setFormData({
        ...restoreCreateWindowFormState(current),
        coreId: normalizedCoreId,
        launchArgs: currentLaunchArgs,
      })
      setLaunchArgsText(currentLaunchArgs.join('\n'))
    }
    loadData()
  }, [id, isCreate])

  const handleChange = (field: keyof CreateWindowFormState, value: string | string[] | boolean) => {
    setIsDirty(true)
    setFormData(prev => ({ ...prev, [field]: value }))
  }

  const handleExtensionToggle = (extensionId: string) => {
    setIsDirty(true)
    setSelectedExtensionIds(prev => (
      prev.includes(extensionId)
        ? prev.filter(id => id !== extensionId)
        : [...prev, extensionId]
    ))
  }

  const persistExtensionBindings = async (profileId: string) => {
    const selected = new Set(selectedExtensionIds)
    const loadableExtensions = extensions.filter(isLoadableExtension)
    const changes = loadableExtensions.flatMap(extension => {
      const currentProfileIds = uniqueProfileIds(extension.boundProfileIds || [])
      const isBound = currentProfileIds.includes(profileId)
      const shouldBind = selected.has(extension.extensionId)
      if (isBound === shouldBind) return []

      const nextProfileIds = shouldBind
        ? uniqueProfileIds([...currentProfileIds, profileId])
        : currentProfileIds.filter(item => item !== profileId)

      return [{ extensionId: extension.extensionId, nextProfileIds }]
    })

    if (changes.length === 0) return

    await Promise.all(changes.map(change => (
      setExtensionProfiles(change.extensionId, change.nextProfileIds)
    )))

    const nextByExtensionId = new Map(changes.map(change => [change.extensionId, change.nextProfileIds]))
    setExtensions(prev => prev.map(extension => (
      nextByExtensionId.has(extension.extensionId)
        ? { ...extension, boundProfileIds: nextByExtensionId.get(extension.extensionId) || [] }
        : extension
    )))
  }

  const handleSave = async () => {
    setSaving(true)
    setSaveError('')
    const convertedPayload = createWindowFormToProfileInput(formData, { launchArgsText, cores, selectedExtensionIds })
    const payload: BrowserProfileInput = isCreate
      ? convertedPayload
      : {
          profileName: formData.profileName,
          userDataDir: formData.userDataDir,
          coreId: formData.coreId,
          fingerprintArgs: formData.fingerprintArgs,
          proxyId: formData.proxyId,
          proxyConfig: formData.proxyConfig,
          launchArgs: normalizeLaunchArgs(launchArgsText.split('\n')),
          tags: formData.tags || [],
          keywords: formData.keywords || [],
          groupId: formData.groupId || '',
          launchCode: formData.launchCode || '',
          accountIds: formData.accountIds || [],
          profileConfig: convertedPayload.profileConfig,
        }
    try {
      if (isCreate) {
        const profile = await createBrowserProfile(payload)
        if (selectedExtensionIds.length > 0) {
          if (!profile?.profileId) {
            throw new Error('配置已创建，但未返回窗口 ID，无法绑定扩展')
          }
          try {
            await persistExtensionBindings(profile.profileId)
          } catch (error) {
            setSaveError(`配置已创建，但扩展绑定失败：${getErrorMessage(error, '更新绑定失败')}`)
            return
          }
        }
        toast.success('配置已创建')
      } else if (id) {
        await updateBrowserProfile(id, payload)
        try {
          await persistExtensionBindings(id)
        } catch (error) {
          setSaveError(`配置已更新，但扩展绑定失败：${getErrorMessage(error, '更新绑定失败')}`)
          return
        }
        toast.success('配置已更新')
      }
      setIsDirty(false)
      navigate(BROWSER_LIST_ROUTE)
    } catch (error: any) {
      setSaveError(getErrorMessage(error, '保存失败'))
    } finally {
      setSaving(false)
    }
  }

  const handleBack = () => {
    if (isDirty) { setLeaveConfirm(true) } else { navigate(BROWSER_LIST_ROUTE) }
  }

  const defaultCore = cores.find(c => c.isDefault)
  const proxyGroupNames = Array.from(new Set(proxies.map(p => (p.groupName || '').trim()).filter(Boolean)))
  const loadableExtensions = extensions.filter(isLoadableExtension)
  const selectedLoadableExtensionCount = selectedExtensionIds.filter(extensionId => (
    loadableExtensions.some(extension => extension.extensionId === extensionId)
  )).length

  const handleOpenUserDataDir = async () => {
    if (!formData.userDataDir.trim()) {
      toast.error('请先输入用户数据目录')
      return
    }
    try {
      await openUserDataDir(formData.userDataDir)
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '打开目录失败')
    }
  }

  const handleProxyListUpdated = (nextProxies: BrowserProxy[]) => {
    setProxies(nextProxies)
  }

  const handleProxyDeleted = (deletedProxyId: string, nextProxies: BrowserProxy[]) => {
    setProxies(nextProxies)
    if (formData.proxyId === deletedProxyId) {
      const fallbackProxy = nextProxies.find(item => item.proxyId === DIRECT_PROXY_ID)
      if (fallbackProxy) {
        handleChange('proxyId', DIRECT_PROXY_ID)
      } else {
        handleChange('proxyId', '')
      }
    }
  }

  const handleProxySelected = (proxy: BrowserProxy) => {
    handleChange('proxyId', proxy.proxyId)
    setProxies(prev => prev.some(item => item.proxyId === proxy.proxyId) ? prev : [...prev, proxy])
  }

  // 导入抽屉成功：刷新代理列表，并自动选中最后导入的代理。
  const handleProxyImported = async (newProxies: BrowserProxy[]) => {
    const refreshed = await fetchBrowserProxies().catch(() => [] as BrowserProxy[])
    setProxies(refreshed)
    setProxyImportOpen(false)
    const target = newProxies[newProxies.length - 1]
    if (target) {
      handleChange('proxyId', target.proxyId)
    }
  }

  const reloadAccounts = async (): Promise<BrowserAccount[]> => {
    const list = await fetchAccounts().catch(() => [] as BrowserAccount[])
    setAccounts(list)
    return list
  }

  // 选择抽屉确认：用勾选结果覆盖已关联账号集合。
  const handleAccountsSelected = (ids: string[]) => {
    handleChange('accountIds', ids)
  }

  // 添加抽屉创建成功：刷新列表，并自动关联新账号（保存并关闭时）。
  const handleAccountCreated = async (account: BrowserAccount, autoSelect: boolean) => {
    await reloadAccounts()
    if (autoSelect) {
      const next = Array.from(new Set([...(formData.accountIds || []), account.accountId]))
      handleChange('accountIds', next)
    }
  }

  // 从模板恢复创建页配置（不覆盖窗口身份字段：名称/数据目录/账号等由当前表单保留）。
  const handleApplyTemplate = (templateId: string) => {
    if (!templateId) {
      // “不使用模板”：仅清空已选模板标识，不动当前已填写的配置。
      setSelectedTemplateId('')
      return
    }
    const template = templates.find(item => item.templateId === templateId)
    if (!template) return
    const patch = restoreFormStateFromTemplate(template.profileConfig, formData)
    const restored = { ...formData, ...patch }
    setFormData(restored)
    setLaunchArgsText(normalizeLaunchArgs(restored.launchArgs).join('\n'))
    setSelectedTemplateId(templateId)
    setIsDirty(true)
    toast.success(`已套用模板「${template.templateName}」`)
  }

  // 保存当前创建页配置为新模板。
  const handleSaveAsTemplate = async (name: string) => {
    const templateName = name.trim()
    if (!templateName) {
      toast.error('请输入模板名称')
      return
    }
    const payload = createWindowFormToProfileInput(formData, { launchArgsText, cores, selectedExtensionIds })
    try {
      const created = await createTemplate({ templateName, profileConfig: payload.profileConfig || '{}' })
      if (created) {
        setTemplates(prev => [...prev, created])
        toast.success(`模板「${templateName}」已保存`)
      }
    } catch (error) {
      toast.error(getErrorMessage(error, '保存模板失败'))
    }
  }

  if (isCreate) {
    return (
      <>
        <BrowserCreateWorkstationPage
          formData={formData}
          cores={cores}
          proxies={proxies}
          groups={groups}
          allTags={allTags}
          extensionsLoadError={extensionsLoadError}
          loadableExtensions={loadableExtensions}
          selectedExtensionIds={selectedExtensionIds}
          selectedLoadableExtensionCount={selectedLoadableExtensionCount}
          launchArgsText={launchArgsText}
          saving={saving}
          onBack={handleBack}
          onSave={handleSave}
          onChange={handleChange}
          onExtensionToggle={handleExtensionToggle}
          onLaunchArgsTextChange={(value) => {
            setLaunchArgsText(value)
            setIsDirty(true)
          }}
          onOpenProxyPicker={() => setProxyPickerOpen(true)}
          onOpenProxyImport={() => setProxyImportOpen(true)}
          accounts={accounts}
          onOpenAccountPicker={() => setAccountPickerOpen(true)}
          onOpenAccountAdd={() => setAccountAddOpen(true)}
          templates={templates.map(t => ({ templateId: t.templateId, templateName: t.templateName }))}
          selectedTemplateId={selectedTemplateId}
          onApplyTemplate={handleApplyTemplate}
          onSaveAsTemplate={handleSaveAsTemplate}
        />

        <ProxyPickerModal
          open={proxyPickerOpen}
          currentProxyId={formData.proxyId}
          onSelect={handleProxySelected}
          onProxyListUpdated={handleProxyListUpdated}
          onProxyDeleted={handleProxyDeleted}
          onClose={() => setProxyPickerOpen(false)}
        />

        <ProxyImportModal
          open={proxyImportOpen}
          existingProxies={proxies}
          groups={proxyGroupNames}
          onImported={handleProxyImported}
          onClose={() => setProxyImportOpen(false)}
        />

        <AccountPickerModal
          open={accountPickerOpen}
          selectedIds={formData.accountIds || []}
          onConfirm={handleAccountsSelected}
          onClose={() => setAccountPickerOpen(false)}
        />

        <AccountAddDrawer
          open={accountAddOpen}
          onCreated={handleAccountCreated}
          onClose={() => setAccountAddOpen(false)}
        />

        <ConfirmModal
          open={leaveConfirm}
          onClose={() => setLeaveConfirm(false)}
          onConfirm={() => navigate(BROWSER_LIST_ROUTE)}
          title="放弃未保存的更改？"
          content="当前页面有未保存的修改，离开后将丢失这些更改。"
          confirmText="放弃并离开"
          cancelText="继续编辑"
          danger
        />

        <Modal
          open={!!saveError}
          onClose={() => setSaveError('')}
          title="保存失败"
          width="420px"
          footer={<Button onClick={() => setSaveError('')}>知道了</Button>}
        >
          <div className="text-[var(--color-text-secondary)]">{saveError}</div>
        </Modal>
      </>
    )
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{isCreate ? '新建配置' : '编辑配置'}</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">完善指纹与启动参数</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleBack}>返回列表</Button>
          <Button size="sm" onClick={handleSave} loading={saving}>保存配置</Button>
        </div>
      </div>

      <Card title="基础信息" subtitle="窗口与配置名称">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="配置名称" required>
            <Input value={formData.profileName} onChange={e => handleChange('profileName', e.target.value)} placeholder="请输入配置名称" />
          </FormItem>
          <FormItem label="用户数据目录（留空自动生成）">
            <div className="flex gap-2">
              <Input
                value={formData.userDataDir}
                onChange={e => handleChange('userDataDir', e.target.value)}
                placeholder="留空自动生成"
                className="flex-1"
              />
              <Button variant="secondary" size="sm" onClick={handleOpenUserDataDir} title="在资源管理器中打开">
                <FolderOpen className="w-4 h-4" />
              </Button>
            </div>
          </FormItem>
          <FormItem label="内核">
            <Select
              value={formData.coreId}
              onChange={e => handleChange('coreId', e.target.value)}
              options={
                cores.length > 0 ? [
                  { value: '', label: defaultCore ? `使用默认 (${defaultCore.coreName})` : '使用默认内核' },
                  ...cores.map(c => ({ value: c.coreId, label: c.coreName })),
                ] : [
                  { value: '', label: '暂无内核，请添加内核' }
                ]
              }
            />
          </FormItem>
          <FormItem label="标签">
            <TagInput
              value={formData.tags}
              onChange={tags => handleChange('tags', tags)}
              suggestions={allTags}
              placeholder="输入标签后按回车，支持从已有标签选择"
            />
          </FormItem>
          <FormItem label="分组">
            <GroupSelector
              groups={groups}
              value={formData.groupId || ''}
              onChange={groupId => handleChange('groupId', groupId)}
              placeholder="未分组"
              className="w-full"
            />
          </FormItem>
        </div>
      </Card>

      <Card title="代理配置" subtitle="选择代理池中的代理或手动输入">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="代理池选择">
            <div className="flex gap-2">
              <Select
                value={formData.proxyId}
                onChange={e => handleChange('proxyId', e.target.value)}
                options={[
                  { value: '', label: '不使用代理池' },
                  ...proxies.map(p => ({ value: p.proxyId, label: p.proxyName || p.proxyId })),
                ]}
                className="flex-1"
              />
              <Button variant="secondary" size="sm" onClick={() => setProxyPickerOpen(true)} title="按分组选择代理">
                <Layers className="w-4 h-4" />
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setProxyImportOpen(true)} title="导入 / 添加代理">
                <Plus className="w-4 h-4" />
              </Button>
            </div>
          </FormItem>
          <FormItem label="手动代理配置">
            <Input
              value={formData.proxyConfig}
              onChange={e => handleChange('proxyConfig', e.target.value)}
              placeholder="http://127.0.0.1:7890"
              disabled={!!formData.proxyId}
            />
          </FormItem>
        </div>
        {formData.proxyId && (
          <p className="text-xs text-[var(--color-text-muted)] mt-2">已选择代理池代理，手动配置将被忽略</p>
        )}
      </Card>

      <ProxyPickerModal
        open={proxyPickerOpen}
        currentProxyId={formData.proxyId}
        onSelect={proxy => {
          handleChange('proxyId', proxy.proxyId)
          setProxies(prev => prev.some(item => item.proxyId === proxy.proxyId) ? prev : [...prev, proxy])
        }}
        onProxyListUpdated={handleProxyListUpdated}
        onProxyDeleted={handleProxyDeleted}
        onClose={() => setProxyPickerOpen(false)}
      />

      <ProxyImportModal
        open={proxyImportOpen}
        existingProxies={proxies}
        groups={proxyGroupNames}
        onImported={handleProxyImported}
        onClose={() => setProxyImportOpen(false)}
      />

      <AccountPickerModal
        open={accountPickerOpen}
        selectedIds={formData.accountIds || []}
        onConfirm={handleAccountsSelected}
        onClose={() => setAccountPickerOpen(false)}
      />

      <AccountAddDrawer
        open={accountAddOpen}
        onCreated={handleAccountCreated}
        onClose={() => setAccountAddOpen(false)}
      />

      <Card
        title="账号关联"
        subtitle="关联平台账号以同步 Cookie"
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => setAccountPickerOpen(true)}>
              <RefreshCw className="w-4 h-4" />选择
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setAccountAddOpen(true)}>
              <Plus className="w-4 h-4" />添加
            </Button>
          </>
        }
      >
        <SelectedAccountChips
          accountIds={formData.accountIds || []}
          accounts={accounts}
          onRemove={accountId => handleChange('accountIds', (formData.accountIds || []).filter(id => id !== accountId))}
        />
        <p className="mt-2 text-xs text-[var(--color-text-muted)]">
          💡 提示：关联账号后，可在窗口启动时自动导入账号的 Cookie
        </p>
      </Card>

      <Card
        title="启动扩展"
        subtitle="绑定本地启用扩展"
        actions={loadableExtensions.length > 0 && (
          <span className="text-xs text-[var(--color-text-muted)]">
            已选 {selectedLoadableExtensionCount} / {loadableExtensions.length}
          </span>
        )}
      >
        <div className="space-y-3">
          {extensionsLoadError && (
            <div className="rounded-lg border border-[var(--color-error)]/40 bg-[var(--color-error)]/10 px-3 py-2 text-sm text-[var(--color-error)]">
              {extensionsLoadError}
            </div>
          )}

          {loadableExtensions.length > 0 ? (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
              {loadableExtensions.map(extension => {
                const checked = selectedExtensionIds.includes(extension.extensionId)
                return (
                  <label
                    key={extension.extensionId}
                    className={`flex items-start gap-3 rounded-lg border p-3 cursor-pointer transition-colors ${
                      checked
                        ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                        : 'border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] hover:border-[var(--color-border-strong)]'
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => handleExtensionToggle(extension.extensionId)}
                      className="mt-1 h-4 w-4 accent-[var(--color-accent)]"
                    />
                    <Package className={`mt-0.5 h-4 w-4 flex-shrink-0 ${checked ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted)]'}`} />
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-medium text-[var(--color-text-primary)] truncate">
                        {extension.extensionName || '未命名扩展'}
                      </span>
                      <span className="block text-xs text-[var(--color-text-muted)] truncate" title={extension.extensionPath}>
                        {extension.version ? `v${extension.version} · ` : ''}{extension.extensionPath}
                      </span>
                    </span>
                  </label>
                )
              })}
            </div>
          ) : (
            <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-4 py-5 text-sm text-[var(--color-text-muted)]">
              暂无可加载的本地启用扩展。请先在扩展管理中添加本地解压目录并启用。
            </div>
          )}

          <p className="text-xs text-[var(--color-text-muted)]">
            这里只显示"本地 + 已启用 + 有扩展路径"的扩展
          </p>
        </div>
      </Card>

      <Card title="指纹配置" subtitle="配置浏览器指纹参数">
        <FingerprintPanel
          value={formData.fingerprintArgs}
          onChange={args => handleChange('fingerprintArgs', args)}
        />
      </Card>

      <Card title="启动参数" subtitle={isCreate ? '新建时默认填入轻量参数模板，直接改这里即可' : '每行一个参数'}>
        <div className="space-y-2">
          <Textarea
            value={launchArgsText}
            onChange={e => { setLaunchArgsText(e.target.value); setIsDirty(true) }}
            rows={6}
            placeholder="--disable-sync"
          />
          {isCreate && (
            <p className="text-xs text-[var(--color-text-muted)]">这里默认就是轻量参数模板；需要更复杂的参数，直接在此基础上修改。</p>
          )}
        </div>
      </Card>

      <ConfirmModal
        open={leaveConfirm}
        onClose={() => setLeaveConfirm(false)}
        onConfirm={() => navigate(BROWSER_LIST_ROUTE)}
        title="放弃未保存的更改？"
        content="当前页面有未保存的修改，离开后将丢失这些更改。"
        confirmText="放弃并离开"
        cancelText="继续编辑"
        danger
      />

      <Modal
        open={!!saveError}
        onClose={() => setSaveError('')}
        title="保存失败"
        width="420px"
        footer={<Button onClick={() => setSaveError('')}>知道了</Button>}
      >
        <div className="text-[var(--color-text-secondary)]">{saveError}</div>
      </Modal>
    </div>
  )
}

// 已关联账号的可移除标签组
function SelectedAccountChips({
  accountIds,
  accounts,
  onRemove,
}: {
  accountIds: string[]
  accounts: BrowserAccount[]
  onRemove: (accountId: string) => void
}) {
  const selected = accountIds
    .map(id => accounts.find(acc => acc.accountId === id))
    .filter((acc): acc is BrowserAccount => !!acc)

  if (selected.length === 0) {
    return (
      <p className="text-sm text-[var(--color-text-muted)]">未关联平台账号，点击右上角「选择」或「添加」。</p>
    )
  }

  return (
    <div className="flex flex-wrap gap-2">
      {selected.map(acc => (
        <span
          key={acc.accountId}
          className="inline-flex items-center gap-1.5 rounded-full border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] py-1 pl-2.5 pr-1.5 text-xs text-[var(--color-text-secondary)]"
        >
          <span className="text-sm leading-none">{platformIcon(acc.platform)}</span>
          <span className="max-w-[180px] truncate">{acc.accountName || acc.username || acc.email || acc.accountId}</span>
          <button
            type="button"
            onClick={() => onRemove(acc.accountId)}
            className="flex h-4 w-4 items-center justify-center rounded-full text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-border-default)] hover:text-[var(--color-text-primary)]"
            aria-label="移除"
          >
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
    </div>
  )
}
