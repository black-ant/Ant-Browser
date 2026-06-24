import { useState, useEffect, useRef } from 'react'
import { ArrowLeft, Download, Upload, Save } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { Button, ConfirmModal } from '../../../shared/components'
import { LeftPanel } from './browser-create-v2/LeftPanel'
import { RightPanel } from './browser-create-v2/RightPanel'
import { DEFAULT_CONFIG } from './browser-create-v2/types'
import type { FingerprintProfileConfig } from './browser-create-v2/types'
import { validateConfig, hasErrors } from './browser-create-v2/validation'
import type { ValidationError } from './browser-create-v2/validation'
import { convertToProfileInput, normalizeConfig } from './browser-create-v2/converter'
import {
  createBrowserProfile as createBrowserProfileAPI,
  fetchAllTags,
  fetchBrowserCores,
  fetchBrowserSettings,
  fetchBrowserProxies,
  fetchExtensions,
  fetchGroups,
  setExtensionProfiles,
} from '../api'
import type { BrowserExtension } from '../api'
import type { BrowserCore, BrowserGroupWithCount, BrowserSettings, BrowserProxy } from '../types'
import { saveDraft, loadDraft, clearDraft, hasDraft, getDraftTimestamp, formatTimeSince } from '../../../services/draftService'

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

export function BrowserCreatePageV2() {
  const navigate = useNavigate()

  // 核心状态：指纹配置
  const [config, setConfig] = useState<FingerprintProfileConfig>(DEFAULT_CONFIG)

  // 真实数据加载
  const [settings, setSettings] = useState<BrowserSettings | null>(null)
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [allTags, setAllTags] = useState<string[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [extensions, setExtensions] = useState<BrowserExtension[]>([])
  const [isLoading, setIsLoading] = useState(true)

  // 选择的内核、分组
  const [selectedCoreId, setSelectedCoreId] = useState<string>('')
  const [selectedGroupId, setSelectedGroupId] = useState<string>('')

  // 验证错误和加载状态
  const [errors, setErrors] = useState<ValidationError[]>([])
  const [isSaving, setIsSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [showDraftPrompt, setShowDraftPrompt] = useState(false)
  const [draftTimestamp, setDraftTimestamp] = useState<Date | null>(null)

  // Dirty 状态和离开确认
  const [isDirty, setIsDirty] = useState(false)
  const [showLeaveConfirm, setShowLeaveConfirm] = useState(false)
  const pendingNavigate = useRef<string | null>(null)

  // 初始化：加载数据
  useEffect(() => {
    const loadData = async () => {
      setIsLoading(true)
      try {
        const [browserSettings, browserCores, browserProxies, tagList, groupList, extensionList] = await Promise.all([
          fetchBrowserSettings(),
          fetchBrowserCores(),
          fetchBrowserProxies(),
          fetchAllTags(),
          fetchGroups(),
          fetchExtensions().catch((error) => {
            console.error('加载扩展列表失败:', error)
            return [] as BrowserExtension[]
          }),
        ])
        setSettings(browserSettings)
        setCores(browserCores)
        setProxies(browserProxies)
        setAllTags(tagList)
        setGroups(groupList)
        setExtensions(extensionList)

        // 检查是否有草稿
        if (hasDraft()) {
          const timestamp = getDraftTimestamp()
          setDraftTimestamp(timestamp)
          setShowDraftPrompt(true)
        }
      } catch (error) {
        console.error('加载数据失败:', error)
        setSaveError('加载数据失败，请刷新页面重试')
      } finally {
        setIsLoading(false)
      }
    }

    loadData()
  }, [])

  // 自动保存草稿（仅当有修改时）
  useEffect(() => {
    if (!isDirty) return  // 没有修改，不保存空草稿

    const intervalId = setInterval(() => {
      saveDraft(config, { selectedCoreId, selectedGroupId })
    }, 30000)

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isDirty) {
        saveDraft(config, { selectedCoreId, selectedGroupId })
        e.preventDefault()
        e.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', handleBeforeUnload)

    return () => {
      clearInterval(intervalId)
      window.removeEventListener('beforeunload', handleBeforeUnload)
    }
  }, [config, isDirty, selectedCoreId, selectedGroupId])

  // 恢复草稿
  const handleRestoreDraft = () => {
    const draft = loadDraft()
    if (draft) {
      // 使用 normalizeConfig 确保数据完整
      const normalized = normalizeConfig(draft.config)
      setConfig(normalized)
      setSelectedCoreId(draft.selectedCoreId || '')
      setSelectedGroupId(draft.selectedGroupId || '')
      setShowDraftPrompt(false)
      setIsDirty(true)
    }
  }

  // 忽略草稿
  const handleIgnoreDraft = () => {
    clearDraft()
    setShowDraftPrompt(false)
  }

  // 更新配置的辅助函数（标记为 dirty）
  const updateConfig = (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => {
    setConfig(updater)
    setIsDirty(true)
    setErrors([])
    setSaveError(null)
  }

  const handleCoreChange = (coreId: string) => {
    setSelectedCoreId(coreId)
    setIsDirty(true)
  }

  const handleGroupChange = (groupId: string) => {
    setSelectedGroupId(groupId)
    setIsDirty(true)
  }

  const bindSelectedExtensions = async (profileId: string) => {
    const selectedIds = new Set(config.preferences.extensionIds)
    const selectedExtensions = extensions.filter(extension => (
      selectedIds.has(extension.extensionId) && isLoadableExtension(extension)
    ))

    if (selectedExtensions.length === 0) return

    await Promise.all(selectedExtensions.map(extension => (
      setExtensionProfiles(
        extension.extensionId,
        uniqueProfileIds([...(extension.boundProfileIds || []), profileId])
      )
    )))
  }

  // 保存配置
  const handleSave = async () => {
    // 验证
    const selectedCore = selectedCoreId
      ? cores.find(core => core.coreId === selectedCoreId)
      : cores.find(core => core.isDefault)
    const validationErrors = validateConfig(config, { selectedCore })
    if (hasErrors(validationErrors)) {
      setErrors(validationErrors)
      setSaveError(validationErrors[0]?.message || '配置校验失败，请检查表单内容')
      window.scrollTo({ top: 0, behavior: 'smooth' })
      return
    }

    setIsSaving(true)
    setSaveError(null)

    try {
      // 转换配置为后端格式（传入真实数据）
      const profileInput = convertToProfileInput(config, {
        defaultFingerprintArgs: settings?.defaultFingerprintArgs || [],
        defaultLaunchArgs: settings?.defaultLaunchArgs || [],
        coreId: selectedCoreId,
        groupId: selectedGroupId,
      })

      // 调用 Wails API 保存配置
      const profile = await createBrowserProfileAPI(profileInput)

      if (profile) {
        try {
          await bindSelectedExtensions(profile.profileId)
        } catch (bindError) {
          console.error('绑定扩展失败:', bindError)
          clearDraft()
          setIsDirty(false)
          setSaveError(`实例已创建（${profile.profileName || profile.profileId}），但扩展绑定失败：${bindError instanceof Error ? bindError.message : '请到扩展管理页手动绑定'}`)
          return
        }

        // 清除草稿和 dirty 状态
        clearDraft()
        setIsDirty(false)
        // 保存成功，跳转到列表页
        navigate('/browser/list')
      } else {
        throw new Error('保存失败：未返回配置数据')
      }
    } catch (error) {
      console.error('保存失败:', error)
      setSaveError(error instanceof Error ? error.message : '保存失败，请重试')
    } finally {
      setIsSaving(false)
    }
  }

  // 导出配置
  const handleExport = () => {
    const dataStr = JSON.stringify(config, null, 2)
    const dataBlob = new Blob([dataStr], { type: 'application/json' })
    const url = URL.createObjectURL(dataBlob)
    const link = document.createElement('a')
    link.href = url
    link.download = `fingerprint-config-${Date.now()}.json`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  // 导入配置（带完整校验）
  const handleImport = () => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.json'
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0]
      if (!file) return

      const reader = new FileReader()
      reader.onload = (event) => {
        try {
          const json = event.target?.result as string
          const parsed = JSON.parse(json)

          // 使用 normalizeConfig 进行完整校验和默认值合并
          const normalized = normalizeConfig(parsed, { rejectUnsupportedBrowserCore: true })

          setConfig(normalized)
          setIsDirty(true)
          setErrors([])
          setSaveError(null)
        } catch (error) {
          console.error('导入失败:', error)
          setSaveError('导入失败：' + (error instanceof Error ? error.message : '配置文件格式不正确'))
        }
      }
      reader.readAsText(file)
    }
    input.click()
  }

  // 返回列表（带离开确认）
  const handleBack = () => {
    if (isDirty) {
      pendingNavigate.current = '/browser/list'
      setShowLeaveConfirm(true)
    } else {
      navigate('/browser/list')
    }
  }

  // 确认离开
  const handleConfirmLeave = () => {
    if (pendingNavigate.current) {
      setIsDirty(false)
      navigate(pendingNavigate.current)
    }
    setShowLeaveConfirm(false)
  }

  // 取消离开
  const handleCancelLeave = () => {
    pendingNavigate.current = null
    setShowLeaveConfirm(false)
  }

  if (isLoading) {
    return (
      <div className="browser-create-shell min-h-screen bg-[var(--color-bg-base)] flex items-center justify-center">
        <div className="text-[var(--color-text-secondary)]">加载中...</div>
      </div>
    )
  }

  return (
    <div className="browser-create-shell min-h-screen bg-[var(--color-bg-base)]">
      {/* 顶部导航栏 */}
      <div className="browser-create-topbar bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)] backdrop-blur-sm">
        <div className="max-w-[1600px] mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <button
                onClick={handleBack}
                className="flex items-center gap-2 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
              >
                <ArrowLeft className="w-5 h-5" />
                <span className="text-sm font-medium">返回列表</span>
              </button>
            </div>

            {/* 操作按钮组 */}
            <div className="flex items-center gap-3">
              <button
                onClick={handleImport}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] bg-[var(--color-bg-elevated)] border border-[var(--color-border-default)] rounded-lg hover:bg-[var(--color-bg-muted)] hover:border-[var(--color-border-strong)] transition-all"
              >
                <Upload className="w-4 h-4" />
                导入配置
              </button>
              <button
                onClick={handleExport}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] bg-[var(--color-bg-elevated)] border border-[var(--color-border-default)] rounded-lg hover:bg-[var(--color-bg-muted)] hover:border-[var(--color-border-strong)] transition-all"
              >
                <Download className="w-4 h-4" />
                导出配置
              </button>
              <Button
                size="lg"
                variant="primary"
                onClick={handleSave}
                disabled={isSaving}
              >
                {isSaving ? (
                  <>
                    <Save className="w-4 h-4 animate-spin" />
                    保存中...
                  </>
                ) : (
                  '保存并创建'
                )}
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* 主体内容区 */}
      <div className="browser-create-layout max-w-[1600px] mx-auto px-6 py-6">
        {/* 草稿恢复提示 */}
        {showDraftPrompt && (
          <div className="browser-create-alert browser-create-alert-info mb-6 p-4 bg-[rgba(59,130,246,0.10)] border border-[rgba(59,130,246,0.30)] rounded-lg flex items-center justify-between backdrop-blur-sm">
            <div>
              <h4 className="text-sm font-semibold text-[var(--color-primary)] mb-1">
                发现未保存的草稿
              </h4>
              <p className="text-sm text-[var(--color-text-secondary)]">
                {draftTimestamp && `上次编辑时间：${formatTimeSince(draftTimestamp)}`}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={handleIgnoreDraft}
                className="px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] rounded transition-colors"
              >
                忽略
              </button>
              <button
                onClick={handleRestoreDraft}
                className="px-4 py-2 text-sm font-medium text-white bg-[var(--color-primary)] hover:bg-[#2563eb] rounded transition-colors"
              >
                恢复草稿
              </button>
            </div>
          </div>
        )}

        {/* 错误提示 */}
        {saveError && (
          <div className="browser-create-alert browser-create-alert-error mb-6 p-4 bg-[rgba(239,68,68,0.10)] border border-[rgba(239,68,68,0.30)] rounded-lg backdrop-blur-sm">
            <h4 className="text-sm font-semibold text-[var(--color-error)] mb-2">
              保存失败
            </h4>
            <p className="text-sm text-[var(--color-text-secondary)]">{saveError}</p>
          </div>
        )}

        {/* 主体：左右布局 */}
        <div className="browser-create-grid grid grid-cols-1 lg:grid-cols-3 gap-6">
          <>
            {/* 左侧：配置表单 */}
            <div className="lg:col-span-2 min-w-0">
              <LeftPanel
                config={config}
                updateConfig={updateConfig}
                errors={errors}
                cores={cores}
                proxies={proxies}
                allTags={allTags}
                groups={groups}
                extensions={extensions}
                selectedCoreId={selectedCoreId}
                selectedGroupId={selectedGroupId}
                onCoreChange={handleCoreChange}
                onGroupChange={handleGroupChange}
              />
            </div>

            {/* 右侧：预览和快捷配置 */}
            <div className="lg:col-span-1 min-w-0">
              <RightPanel
                config={config}
                updateConfig={updateConfig}
                cores={cores}
                selectedCoreId={selectedCoreId}
              />
            </div>
          </>
        </div>
      </div>

      {/* 离开确认弹窗 */}
      {showLeaveConfirm && (
        <ConfirmModal
          open={showLeaveConfirm}
          onClose={handleCancelLeave}
          onConfirm={handleConfirmLeave}
          title="确认离开"
          content="您有未保存的修改，确定要离开吗？"
          confirmText="离开"
          cancelText="取消"
          danger={true}
        />
      )}
    </div>
  )
}
