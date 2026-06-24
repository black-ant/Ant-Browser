import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { FolderOpen, Layers, Package } from 'lucide-react'
import { Button, Card, ConfirmModal, FormItem, Input, Modal, Select, Textarea, toast } from '../../../shared/components'
import type { BrowserCore, BrowserProfileInput, BrowserProxy, BrowserGroup } from '../types'
import {
  createBrowserProfile,
  fetchAllTags,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchExtensions,
  fetchGroups,
  openUserDataDir,
  setExtensionProfiles,
  updateBrowserProfile,
} from '../api'
import type { BrowserExtension } from '../api'
import { FingerprintPanel } from '../components/FingerprintPanel'
import { randomFingerprintSeed } from '../utils/fingerprintSerializer'
import { TagInput } from '../components/TagInput'
import { GroupSelector } from '../components/GroupSelector'
import { ProxyPickerModal } from '../components/ProxyPickerModal'
import { AccountSelector } from './browser-create-v2/AccountSelector'
import { ConfigSummary } from './browser-edit/ConfigSummary'

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
  const [formData, setFormData] = useState<BrowserProfileInput>({
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
  const [isDirty, setIsDirty] = useState(false)
  const [leaveConfirm, setLeaveConfirm] = useState(false)
  const [saveError, setSaveError] = useState('')

  useEffect(() => {
    const loadData = async () => {
      setExtensionsLoadError('')
      const [coreList, proxyList, tagList, groupList, settings, extensionList] = await Promise.all([
        fetchBrowserCores(),
        fetchBrowserProxies(),
        fetchAllTags(),
        fetchGroups(),
        fetchBrowserSettings(),
        fetchExtensions().catch((error) => {
          setExtensionsLoadError(getErrorMessage(error, '扩展列表加载失败'))
          return [] as BrowserExtension[]
        }),
      ])
      const resolvedDefaultLaunchArgs = resolveDefaultLaunchArgs(settings.defaultLaunchArgs || [])
      setCores(coreList)
      setProxies(proxyList)
      setAllTags(tagList)
      setGroups(groupList)
      setExtensions(extensionList)

      if (isCreate) {
        setLaunchArgsText(resolvedDefaultLaunchArgs.join('\n'))
        setSelectedExtensionIds([])
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
        profileName: current.profileName,
        userDataDir: current.userDataDir,
        coreId: normalizedCoreId,
        fingerprintArgs: current.fingerprintArgs,
        proxyId: current.proxyId,
        proxyConfig: current.proxyConfig,
        launchArgs: currentLaunchArgs,
        tags: current.tags,
        keywords: current.keywords || [],
        groupId: current.groupId || '',
        accountIds: current.accountIds || [],
      })
      setLaunchArgsText(currentLaunchArgs.join('\n'))
    }
    loadData()
  }, [id, isCreate])

  const handleChange = (field: keyof BrowserProfileInput, value: string | string[]) => {
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
    const payload: BrowserProfileInput = {
      ...formData,
      launchArgs: normalizeLaunchArgs(launchArgsText.split('\n')),
    }
    try {
      if (isCreate) {
        const profile = await createBrowserProfile(payload)
        if (selectedExtensionIds.length > 0) {
          if (!profile?.profileId) {
            throw new Error('配置已创建，但未返回实例 ID，无法绑定扩展')
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

      <Card title="基础信息" subtitle="实例与配置名称">
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

      <Card title="账号关联" subtitle="关联平台账号以同步 Cookie">
        <FormItem label="关联的平台账号">
          <AccountSelector
            selectedIds={formData.accountIds || []}
            onChange={accountIds => handleChange('accountIds', accountIds)}
          />
          <p className="mt-2 text-xs text-[var(--color-text-muted)]">
            💡 提示：关联账号后，可在实例启动时自动导入账号的 Cookie
          </p>
        </FormItem>
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

      <Card title="配置摘要" subtitle="当前配置的关键信息与风险提示">
        <ConfigSummary
          formData={formData}
          proxy={proxies.find(p => p.proxyId === formData.proxyId)}
          extensions={extensions}
          selectedExtensionIds={selectedExtensionIds}
          accountCount={formData.accountIds?.length || 0}
        />
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
