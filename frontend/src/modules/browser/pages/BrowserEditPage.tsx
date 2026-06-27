import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Button, ConfirmModal, Modal, toast } from '../../../shared/components'
import type { BrowserCore, BrowserGroup, BrowserProxy, BrowserTemplate, CreateWindowFormState } from '../types'
import {
  createBrowserProfile,
  fetchAccounts,
  fetchAllTags,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchCoreExtendedInfo,
  fetchExtensions,
  fetchGroups,
  fetchTemplates,
  createTemplate,
  setExtensionProfiles,
  updateBrowserProfile,
} from '../api'
import type { BrowserAccount, BrowserExtension } from '../api'
import { randomFingerprintSeed } from '../utils/fingerprintSerializer'
import { ProxyPickerModal } from '../components/ProxyPickerModal'
import { ProxyImportModal } from '../components/ProxyImportModal'
import { AccountPickerModal } from '../components/AccountPickerModal'
import { AccountAddDrawer } from '../components/AccountAddDrawer'
import { BrowserCreateWorkstationPage } from './BrowserCreateWorkstationPage'
import { createWindowFormToProfileInput, restoreCreateWindowFormState, restoreFormStateFromTemplate } from '../utils/createWindowConverter'

const fallbackLowLaunchArgs = ['--disable-sync', '--no-first-run']
const BROWSER_LIST_ROUTE = '/browser/list'
const DIRECT_PROXY_ID = '__direct__'

// 取当前屏幕可用尺寸，供转换器把九宫格窗口位置换算成 --window-position=x,y。
function currentScreenSize(): { width: number; height: number } | undefined {
  if (typeof window === 'undefined' || !window.screen) return undefined
  const width = window.screen.availWidth || window.screen.width
  const height = window.screen.availHeight || window.screen.height
  return width > 0 && height > 0 ? { width, height } : undefined
}

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
  const [coreVersions, setCoreVersions] = useState<Record<string, string>>({})
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
      const [coreList, coreInfoList, proxyList, tagList, groupList, settings, extensionList, accountList] = await Promise.all([
        fetchBrowserCores(),
        fetchCoreExtendedInfo().catch(() => []),
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
      const versionMap = Object.fromEntries(
        coreInfoList
          .map(info => [info.coreId, info.chromeVersion?.trim() || ''])
          .filter(([, version]) => Boolean(version))
      )
      setCores(coreList)
      setCoreVersions(versionMap)
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
      fetchTemplates().then(setTemplates).catch(() => setTemplates([]))
      // 复用创建页富表单回显：restoreCreateWindowFormState 已把受管参数剥离，
      // launchArgs/launchArgsText 只保留非受管的手填参数，避免重新保存时压制控件新值。
      const restored = restoreCreateWindowFormState(current)
      setFormData({ ...restored, coreId: normalizedCoreId })
      setLaunchArgsText(normalizeLaunchArgs(restored.launchArgs).join('\n'))
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
    const payload = createWindowFormToProfileInput(formData, { launchArgsText, cores, coreVersions, selectedExtensionIds, screen: currentScreenSize() })
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

  const proxyGroupNames = Array.from(new Set(proxies.map(p => (p.groupName || '').trim()).filter(Boolean)))
  const loadableExtensions = extensions.filter(isLoadableExtension)
  const selectedLoadableExtensionCount = selectedExtensionIds.filter(extensionId => (
    loadableExtensions.some(extension => extension.extensionId === extensionId)
  )).length

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
    const payload = createWindowFormToProfileInput(formData, { launchArgsText, cores, coreVersions, selectedExtensionIds, screen: currentScreenSize() })
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

  return (
    <>
      <BrowserCreateWorkstationPage
          mode={isCreate ? 'create' : 'edit'}
          formData={formData}
          cores={cores}
          coreVersions={coreVersions}
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
