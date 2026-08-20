import { useState } from 'react'
import { toast } from '../../../shared/components'
import type { BrowserProfile, BrowserProfileCopyOptions, BrowserProxy } from '../types'
import { BrowserCoreEditorModal, BrowserListHeader, BrowserListSettingsModal } from '../components/BrowserListLayout'
import { BatchToolbar } from '../components/BrowserListWidgets'
import { BrowserProfilesPanel } from '../components/BrowserProfilesPanel'
import { BrowserBackupModal } from '../components/BrowserBackupModal'
import { ProxyPickerModal } from '../components/ProxyPickerModal'
import { ProxyRoutingModal } from '../components/ProxyRoutingModal'
import { ProfileExtensionModal } from '../components/ProfileExtensionModal'
import { createBrowserProfileCopyOptions, isBrowserProfileCopyOptionsValid } from '../copyOptions'
import { buildBrowserProfileCopyName } from '../copyName'
import { resolveActionFeedback } from '../utils/actionErrors'
import { BrowserListDialogs } from './browserList/BrowserListDialogs'
import { useBrowserListDerived, useBrowserListViewState } from './browserList/useBrowserListViewState'
import { useBrowserListSettings } from './browserList/useBrowserListSettings'
import { useBrowserListData } from './browserList/useBrowserListData'
import { useBrowserProfileActions } from './browserList/useBrowserProfileActions'
import { warmupProfileProxyBeforeStart } from '../utils/proxyWarmup'
import {
  copyBrowserProfile,
  deleteBrowserProfile,
  exportBrowserProfilePackage,
  fetchBrowserProfileTrash,
  importBrowserProfilePackage,
  permanentlyDeleteBrowserProfile,
  restoreBrowserProfile,
  startBrowserInstance,
  stopBrowserInstance,
  switchBrowserProfileProxy,
  openUserDataDir,
  exportFullBrowserBackup,
  importFullBrowserBackup,
} from '../api'

type BackupLoadingMode = 'none' | 'export' | 'import-merge' | 'import-reset'

const directProxyID = '__direct__'

export function BrowserListPage() {
  const {
    viewMode,
    setViewMode,
    filters,
    setFilters,
    headerCollapsed,
    setHeaderCollapsed,
  } = useBrowserListViewState()

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)
  const [profilePackageBusy, setProfilePackageBusy] = useState(false)
  const [backupModalOpen, setBackupModalOpen] = useState(false)
  const [backupLoadingMode, setBackupLoadingMode] = useState<BackupLoadingMode>('none')
  const [deleteConfirm, setDeleteConfirm] = useState<{
    open: boolean
    mode: 'single' | 'batch'
    profileId?: string
    profileName?: string
    count: number
  }>({ open: false, mode: 'single', count: 0 })

  // 代理不支持弹窗
  const [proxyErrorModal, setProxyErrorModal] = useState(false)
  const [proxyErrorMsg, setProxyErrorMsg] = useState('')
  const [opError, setOpError] = useState('')
  const [pendingStartId, setPendingStartId] = useState<string | null>(null)

  // 关键字弹窗
  const [kwModal, setKwModal] = useState<{ open: boolean; profile: BrowserProfile | null }>({ open: false, profile: null })

  const openKwModal = (profile: BrowserProfile) => setKwModal({ open: true, profile })
  const closeKwModal = () => setKwModal({ open: false, profile: null })

  const [extensionModal, setExtensionModal] = useState<{ open: boolean; profile: BrowserProfile | null }>({ open: false, profile: null })
  const openExtensionModal = (profile: BrowserProfile) => setExtensionModal({ open: true, profile })
  const closeExtensionModal = () => setExtensionModal({ open: false, profile: null })

  const [proxyPickerProfile, setProxyPickerProfile] = useState<BrowserProfile | null>(null)
  const [routingProfile, setRoutingProfile] = useState<BrowserProfile | null>(null)

  // 复制弹窗
  const [copyModal, setCopyModal] = useState<{ open: boolean; profile: BrowserProfile | null }>({ open: false, profile: null })
  const [copyName, setCopyName] = useState('')
  const [copyOptions, setCopyOptions] = useState<BrowserProfileCopyOptions>(() => createBrowserProfileCopyOptions())
  const [copying, setCopying] = useState(false)
  const [trashModalOpen, setTrashModalOpen] = useState(false)
  const [trashProfiles, setTrashProfiles] = useState<BrowserProfile[]>([])
  const [trashLoading, setTrashLoading] = useState(false)
  const [restoringId, setRestoringId] = useState('')
  const [permanentlyDeletingId, setPermanentlyDeletingId] = useState('')
  const [permanentDeleteConfirm, setPermanentDeleteConfirm] = useState<{ open: boolean; profile: BrowserProfile | null }>({
    open: false,
    profile: null,
  })

  const openCopyModal = (profile: BrowserProfile) => {
    setCopyName(buildBrowserProfileCopyName(profile.profileName))
    setCopyOptions(createBrowserProfileCopyOptions())
    setCopyModal({ open: true, profile })
  }
  const closeCopyModal = () => {
    setCopyModal({ open: false, profile: null })
    setCopyName('')
    setCopyOptions(createBrowserProfileCopyOptions())
  }
  const {
    settingsModalOpen,
    setSettingsModalOpen,
    settings,
    setSettings,
    fingerprintText,
    setFingerprintText,
    launchText,
    setLaunchText,
    startUrlsText,
    setStartUrlsText,
    savingSettings,
    cores,
    coreModalOpen,
    setCoreModalOpen,
    coreForm,
    setCoreForm,
    coreValidation,
    setCoreValidation,
    savingCore,
    loadCores,
    handleOpenSettings,
    handleSaveSettings,
    handleOpenCoreModal,
    handleValidateCorePath,
    handleSaveCore,
    handleDeleteCore,
    handleSetDefaultCore,
  } = useBrowserListSettings()
  const {
    profiles,
    loading,
    proxies,
    groups,
    startingIds,
    stoppingIds,
    setStartingIds,
    setStoppingIds,
    updatePendingIds,
    updateProfilesState,
    mergeProfileState,
    updateProxiesState,
    loadProfiles,
  } = useBrowserListData({ loadCores })
  const {
    runningCount,
    allTags,
    filteredProfiles,
    resolveProfileCore,
    getProfileCoreLabel,
    isProfileStarting,
    isProfileStopping,
    isProfileBusy,
    getProfileStatus,
  } = useBrowserListDerived(profiles, cores, filters, startingIds, stoppingIds)
  const {
    handleStart,
    handleStartDirect,
    handleStop,
    handleRestart,
  } = useBrowserProfileActions({
    profiles,
    setProxyErrorModal,
    setProxyErrorMsg,
    setPendingStartId,
    setOpError,
    setStartingIds,
    setStoppingIds,
    updatePendingIds,
    mergeProfileState,
    loadProfiles,
  })
  // 批量操作
  const toggleSelect = (profileId: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      next.has(profileId) ? next.delete(profileId) : next.add(profileId)
      return next
    })
  }



  const handleSelectAll = () => {
    setSelectedIds(new Set(filteredProfiles.map(p => p.profileId)))
  }

  const handleDeselectAll = () => {
    setSelectedIds(new Set())
  }

  const handleBatchStart = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return
    setBatchLoading(true)
    let success = 0, pending = 0, failed = 0
    const pendingMessages: string[] = []
    const failureMessages: string[] = []
    for (const id of ids) {
      const profile = profiles.find(p => p.profileId === id)
      if (!profile || profile.running) continue
      updatePendingIds(setStartingIds, id, true)
      try {
        await warmupProfileProxyBeforeStart(profile)
        const startedProfile = await startBrowserInstance(id)
        mergeProfileState(startedProfile)
        success++
      } catch (error: any) {
        const feedback = resolveActionFeedback(error, '实例启动失败')
        if (feedback.pendingAttach) {
          pending++
          pendingMessages.push(`${profile.profileName}：${feedback.message}`)
        } else {
          failed++
          failureMessages.push(`${profile.profileName}：${feedback.message}`)
        }
      } finally {
        updatePendingIds(setStartingIds, id, false)
      }
    }
    setBatchLoading(false)
    const summary = [`成功 ${success}`]
    if (pending > 0) summary.push(`待接管 ${pending}`)
    if (failed > 0) summary.push(`失败 ${failed}`)
    toast.success(`批量启动完成：${summary.join('，')}`)
    if (pendingMessages.length > 0) {
      const preview = pendingMessages.slice(0, 3)
      const more = pendingMessages.length > preview.length ? `\n另有 ${pendingMessages.length - preview.length} 个实例已打开窗口，仍在后台接管。` : ''
      toast.warning(`以下实例已打开窗口，仍在后台接管：\n${preview.join('\n')}${more}`)
    }
    if (failureMessages.length > 0) {
      const preview = failureMessages.slice(0, 3)
      const more = failureMessages.length > preview.length ? `\n另有 ${failureMessages.length - preview.length} 个实例启动失败，请逐个检查。` : ''
      toast.error(`以下实例启动失败：\n${preview.join('\n')}${more}`)
    }
    loadProfiles()
  }

  const handleBatchStop = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return
    setBatchLoading(true)
    let success = 0, failed = 0
    for (const id of ids) {
      const profile = profiles.find(p => p.profileId === id)
      if (!profile || !profile.running) continue
      updatePendingIds(setStoppingIds, id, true)
      try {
        const stoppedProfile = await stopBrowserInstance(id)
        mergeProfileState(stoppedProfile)
        success++
      } catch {
        failed++
      } finally {
        updatePendingIds(setStoppingIds, id, false)
      }
    }
    setBatchLoading(false)
    toast.success(`批量停止完成：成功 ${success}${failed > 0 ? `，失败 ${failed}` : ''}`)
    loadProfiles()
  }

  const handleBatchExport = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0 || profilePackageBusy) return
    const runningNames = profiles
      .filter(profile => ids.includes(profile.profileId) && profile.running)
      .map(profile => profile.profileName)
    if (runningNames.length > 0) {
      toast.error(`请先停止实例再导出：${runningNames.slice(0, 3).join('、')}${runningNames.length > 3 ? ' 等' : ''}`)
      return
    }
    setProfilePackageBusy(true)
    try {
      const result = await exportBrowserProfilePackage(ids)
      if (result.cancelled) return
      toast.success(`已导出 ${result.profileCount} 个实例`)
    } catch (error: any) {
      toast.error(error?.message || '导出实例失败')
    } finally {
      setProfilePackageBusy(false)
    }
  }

  const handleExportProfile = async (profile: BrowserProfile) => {
    if (profilePackageBusy) return
    if (profile.running) {
      toast.error(`请先停止实例再导出：${profile.profileName}`)
      return
    }
    setProfilePackageBusy(true)
    try {
      const result = await exportBrowserProfilePackage([profile.profileId])
      if (result.cancelled) return
      toast.success(`已导出：${profile.profileName}`)
    } catch (error: any) {
      toast.error(error?.message || '导出实例失败')
    } finally {
      setProfilePackageBusy(false)
    }
  }

  const handleOpenUserDataDir = async (profile: BrowserProfile) => {
    try {
      if (!profile.userDataDir.trim()) {
        toast.warning('该实例没有用户数据目录')
        return
      }
      const opened = await openUserDataDir(profile.userDataDir)
      if (!opened) {
        toast.warning('当前环境不支持打开数据目录')
      }
    } catch (error: any) {
      toast.error(error?.message || '打开数据目录失败')
    }
  }

  const handleImportProfiles = async () => {
    if (profilePackageBusy) return
    setProfilePackageBusy(true)
    try {
      const result = await importBrowserProfilePackage()
      if (result.cancelled) return
      const warnings = result.warnings || []
      if (warnings.length > 0) {
        toast.warning(`已导入 ${result.importedCount} 个实例，${warnings.length} 条提示：${warnings[0]}`)
      } else {
        toast.success(`已导入 ${result.importedCount} 个实例`)
      }
      setSelectedIds(new Set())
      await loadProfiles()
    } catch (error: any) {
      toast.error(error?.message || '导入实例失败')
    } finally {
      setProfilePackageBusy(false)
    }
  }

  const handleExportFullBackup = async () => {
    if (backupLoadingMode !== 'none') return
    if (runningCount > 0) {
      toast.warning(`建议先停止 ${runningCount} 个运行中实例后再备份`)
    }
    setBackupLoadingMode('export')
    try {
      const result = await exportFullBrowserBackup()
      if (result.cancelled) return
      toast.success(result.zipPath ? `备份已导出：${result.zipPath}` : (result.message || '备份已导出'))
    } catch (error: any) {
      toast.error(error?.message || '全量备份失败')
    } finally {
      setBackupLoadingMode('none')
    }
  }

  const handleImportFullBackup = async (resetFirst: boolean) => {
    if (backupLoadingMode !== 'none') return
    const mode: BackupLoadingMode = resetFirst ? 'import-reset' : 'import-merge'
    setBackupLoadingMode(mode)
    try {
      const result = await importFullBrowserBackup(resetFirst)
      if (result.cancelled) return
      toast.success(result.message || (resetFirst ? '备份已恢复' : '备份已合并'))
      setSelectedIds(new Set())
      setBackupModalOpen(false)
      await loadProfiles()
    } catch (error: any) {
      toast.error(error?.message || '导入备份失败')
    } finally {
      setBackupLoadingMode('none')
    }
  }

  const openDeleteConfirm = (profileId: string) => {
    const profile = profiles.find(item => item.profileId === profileId)
    setDeleteConfirm({
      open: true,
      mode: 'single',
      profileId,
      profileName: profile?.profileName,
      count: 1,
    })
  }

  const loadTrashProfiles = async () => {
    setTrashLoading(true)
    try {
      setTrashProfiles(await fetchBrowserProfileTrash())
    } catch (error: any) {
      toast.error(error?.message || '加载回收站失败')
    } finally {
      setTrashLoading(false)
    }
  }

  const openTrashModal = () => {
    setTrashModalOpen(true)
    void loadTrashProfiles()
  }

  const handleRestoreProfile = async (profileId: string) => {
    setRestoringId(profileId)
    try {
      await restoreBrowserProfile(profileId)
      toast.success('实例已恢复')
      await loadTrashProfiles()
      await loadProfiles()
    } catch (error: any) {
      toast.error(error?.message || '恢复失败')
    } finally {
      setRestoringId('')
    }
  }

  const handleConfirmPermanentDelete = async () => {
    const profile = permanentDeleteConfirm.profile
    if (!profile) return
    setPermanentlyDeletingId(profile.profileId)
    try {
      await permanentlyDeleteBrowserProfile(profile.profileId)
      toast.success('实例已彻底删除')
      setPermanentDeleteConfirm({ open: false, profile: null })
      await loadTrashProfiles()
    } catch (error: any) {
      toast.error(error?.message || '彻底删除失败')
    } finally {
      setPermanentlyDeletingId('')
    }
  }

  const openBatchDeleteConfirm = () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return
    setDeleteConfirm({ open: true, mode: 'batch', count: ids.length })
  }

  const closeDeleteConfirm = () => {
    if (batchLoading) return
    setDeleteConfirm({ open: false, mode: 'single', count: 0 })
  }

  const handleConfirmDelete = async () => {
    const ids = deleteConfirm.mode === 'batch'
      ? Array.from(selectedIds)
      : deleteConfirm.profileId ? [deleteConfirm.profileId] : []
    if (ids.length === 0) return
    setBatchLoading(true)
    try {
      for (const id of ids) {
        await deleteBrowserProfile(id)
      }
      setSelectedIds(prev => {
        const next = new Set(prev)
        ids.forEach(id => next.delete(id))
        return next
      })
      toast.success(ids.length > 1 ? `已删除 ${ids.length} 个实例` : '配置已删除')
      setDeleteConfirm({ open: false, mode: 'single', count: 0 })
      loadProfiles()
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    } finally {
      setBatchLoading(false)
    }
  }

  const handleCopy = async (profileId: string) => {
    if (!copyModal.profile) return
    setCopying(true)
    try {
      await copyBrowserProfile(profileId, copyName.trim(), copyOptions)
      toast.success('实例已复制')
      closeCopyModal()
      loadProfiles()
    } catch (error: any) {
      setOpError(typeof error === 'string' ? error : error?.message || '复制失败')
    } finally {
      setCopying(false)
    }
  }

  const copyConfirmDisabled =
    !copyName.trim() || !isBrowserProfileCopyOptionsValid(copyOptions)

  const saveProfileProxy = async (profile: BrowserProfile, proxy: BrowserProxy, force = false) => {
    try {
      const result = await switchBrowserProfileProxy(profile.profileId, proxy.proxyId, '', force)
      mergeProfileState(result.profile || { ...profile, proxyId: proxy.proxyId, proxyConfig: '' })
      if (!result.appliedLive) {
        toast.success('代理配置已更新，将在下次启动时生效')
      } else if (force) {
        toast.success('代理已热切换，旧连接已断开')
      } else if (result.gateway?.drainingConnections > 0) {
        toast.success(`代理已热切换，${result.gateway.drainingConnections} 条旧连接正在排空`)
      } else {
        toast.success('代理已热切换，新连接已使用新代理')
      }
    } catch (error: any) {
      toast.error(error?.message || '切换代理失败')
    }
  }

  const handleProxyDeletedFromPicker = (deletedProxyId: string, nextProxies: BrowserProxy[]) => {
    updateProxiesState(nextProxies)
    if (!proxyPickerProfile || proxyPickerProfile.proxyId !== deletedProxyId) return
    const fallbackProxy = nextProxies.find(proxy => proxy.proxyId === directProxyID || proxy.proxyConfig === 'direct://')
    if (fallbackProxy) {
      void saveProfileProxy(proxyPickerProfile, fallbackProxy)
    }
  }


  return (
    <div className="overflow-auto p-5 space-y-5 animate-fade-in h-full">
      <BrowserListHeader
        profileCount={profiles.length}
        filteredProfileCount={filteredProfiles.length}
        runningCount={runningCount}
        headerCollapsed={headerCollapsed}
        viewMode={viewMode}
        proxies={proxies}
        cores={cores}
        groups={groups}
        allTags={allTags}
        filters={filters}
        onFiltersChange={setFilters}
        onToggleHeaderCollapsed={() => setHeaderCollapsed((prev) => !prev)}
        onRefresh={() => { void loadProfiles() }}
        onOpenSettings={handleOpenSettings}
        onOpenTrash={openTrashModal}
        onImportProfiles={handleImportProfiles}
        onOpenBackup={() => setBackupModalOpen(true)}
        importingProfiles={profilePackageBusy}
        onViewModeChange={setViewMode}
      />

      {/* 批量操作工具栏 */}
      <BatchToolbar
        selectedCount={selectedIds.size}
        totalCount={filteredProfiles.length}
        onSelectAll={handleSelectAll}
        onDeselectAll={handleDeselectAll}
        onBatchStart={handleBatchStart}
        onBatchStop={handleBatchStop}
        onBatchExport={handleBatchExport}
        onOpenBackup={() => setBackupModalOpen(true)}
        onBatchDelete={openBatchDeleteConfirm}
        batchLoading={batchLoading}
        exporting={profilePackageBusy}
      />

      <BrowserBackupModal
        open={backupModalOpen}
        runningCount={runningCount}
        selectedCount={selectedIds.size}
        selectedExporting={profilePackageBusy}
        loadingMode={backupLoadingMode}
        onClose={() => setBackupModalOpen(false)}
        onExportSelected={() => { void handleBatchExport() }}
        onExportFull={() => { void handleExportFullBackup() }}
        onImportMerge={() => { void handleImportFullBackup(false) }}
        onImportReset={() => { void handleImportFullBackup(true) }}
      />

      <BrowserProfilesPanel
        loading={loading}
        viewMode={viewMode}
        profiles={filteredProfiles}
        proxies={proxies}
        selectedIds={selectedIds}
        resolveProfileCore={resolveProfileCore}
        getProfileCoreLabel={getProfileCoreLabel}
        getProfileStatus={getProfileStatus}
        isProfileStarting={isProfileStarting}
        isProfileStopping={isProfileStopping}
        isProfileBusy={isProfileBusy}
        onToggleSelect={toggleSelect}
        onSelectAll={handleSelectAll}
        onDeselectAll={handleDeselectAll}
        onRefreshProfiles={() => { void loadProfiles() }}
        onStart={(profileId) => { void handleStart(profileId) }}
        onStop={(profileId) => { void handleStop(profileId) }}
        onRestart={(profileId) => { void handleRestart(profileId) }}
        onOpenKeywords={openKwModal}
        onOpenExtensions={openExtensionModal}
        onOpenDataDir={(profile) => { void handleOpenUserDataDir(profile) }}
        onExport={(profile) => { void handleExportProfile(profile) }}
        onOpenCopy={openCopyModal}
        onOpenProxyPicker={setProxyPickerProfile}
        onDelete={openDeleteConfirm}
      />

      <ProxyPickerModal
        open={!!proxyPickerProfile}
        currentProxyId={proxyPickerProfile?.proxyId || directProxyID}
        title={proxyPickerProfile ? `切换代理：${proxyPickerProfile.profileName}` : '切换代理'}
        profileRunning={proxyPickerProfile?.running || false}
        onSelect={(proxy, options) => {
          if (proxyPickerProfile) {
            void saveProfileProxy(proxyPickerProfile, proxy, options?.force || false)
          }
        }}
        onOpenRouting={() => {
          if (proxyPickerProfile) setRoutingProfile(proxyPickerProfile)
        }}
        onProxyListUpdated={updateProxiesState}
        onProxyDeleted={handleProxyDeletedFromPicker}
        onClose={() => setProxyPickerProfile(null)}
      />

      <ProxyRoutingModal
        open={!!routingProfile}
        profileId={routingProfile?.profileId || ''}
        profileName={routingProfile?.profileName}
        running={routingProfile?.running || false}
        onClose={() => setRoutingProfile(null)}
      />

      <ProfileExtensionModal
        open={extensionModal.open}
        profile={extensionModal.profile}
        onClose={closeExtensionModal}
      />

      <BrowserListSettingsModal
        open={settingsModalOpen}
        settings={settings}
        fingerprintText={fingerprintText}
        launchText={launchText}
        startUrlsText={startUrlsText}
        savingSettings={savingSettings}
        cores={cores}
        onClose={() => setSettingsModalOpen(false)}
        onSave={handleSaveSettings}
        onSettingsChange={(patch) => setSettings((prev) => ({ ...prev, ...patch }))}
        onFingerprintTextChange={setFingerprintText}
        onLaunchTextChange={setLaunchText}
        onStartUrlsTextChange={setStartUrlsText}
        onAddCore={() => handleOpenCoreModal()}
        onEditCore={handleOpenCoreModal}
        onDeleteCore={handleDeleteCore}
        onSetDefaultCore={handleSetDefaultCore}
      />

      <BrowserCoreEditorModal
        open={coreModalOpen}
        coreForm={coreForm}
        coreValidation={coreValidation}
        savingCore={savingCore}
        onClose={() => setCoreModalOpen(false)}
        onSave={handleSaveCore}
        onValidate={handleValidateCorePath}
        onCoreFormChange={(patch) => {
          setCoreForm((prev) => ({ ...prev, ...patch }))
          if (Object.prototype.hasOwnProperty.call(patch, 'corePath')) {
            setCoreValidation(null)
          }
        }}
      />

      <BrowserListDialogs
        proxyErrorModal={proxyErrorModal}
        pendingStartId={pendingStartId}
        proxyErrorMsg={proxyErrorMsg}
        onCloseProxyError={() => {
          setProxyErrorModal(false)
          setPendingStartId(null)
        }}
        onStartDirect={() => {
          if (pendingStartId) {
            void handleStartDirect(pendingStartId)
          }
        }}
        startingDirect={pendingStartId ? startingIds.has(pendingStartId) : false}
        kwModal={kwModal}
        onCloseKeywords={closeKwModal}
        onKeywordsSaved={(keywords) => {
          updateProfilesState(prev => prev.map(p =>
            p.profileId === kwModal.profile!.profileId ? { ...p, keywords } : p
          ))
        }}
        copyModal={copyModal}
        copyName={copyName}
        copyOptions={copyOptions}
        onCopyNameChange={setCopyName}
        onCopyOptionsChange={setCopyOptions}
        onCloseCopy={closeCopyModal}
        onConfirmCopy={() => copyModal.profile && handleCopy(copyModal.profile.profileId)}
        copyConfirmDisabled={copyConfirmDisabled}
        copying={copying}
        deleteConfirm={deleteConfirm}
        deleting={batchLoading}
        onCloseDeleteConfirm={closeDeleteConfirm}
        onConfirmDelete={() => { void handleConfirmDelete() }}
        trashModalOpen={trashModalOpen}
        trashProfiles={trashProfiles}
        trashLoading={trashLoading}
        restoringId={restoringId}
        permanentlyDeletingId={permanentlyDeletingId}
        permanentDeleteConfirm={permanentDeleteConfirm}
        onCloseTrash={() => setTrashModalOpen(false)}
        onRestoreProfile={(profileId) => { void handleRestoreProfile(profileId) }}
        onOpenPermanentDelete={(profile) => setPermanentDeleteConfirm({ open: true, profile })}
        onClosePermanentDelete={() => setPermanentDeleteConfirm({ open: false, profile: null })}
        onConfirmPermanentDelete={() => { void handleConfirmPermanentDelete() }}
        opError={opError}
        onCloseOpError={() => setOpError('')}
      />
    </div>
  )
}
