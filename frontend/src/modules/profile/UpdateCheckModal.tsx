import { CheckCircle2, Download, ExternalLink, FolderOpen } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Alert, Button, ConfirmModal, FormItem, Input, Modal, Progress, Select, toast } from '../../shared/components'
import { EventsOn, Quit } from '../../wailsjs/runtime/runtime'
import { useLayoutStore } from '../../store/layoutStore'
import {
  applyAppUpdate,
  checkAppUpdateVersion,
  downloadAppUpdateWithChannel,
  getDownloadedAppUpdate,
  openUpdatePackageLocation,
} from './api'
import { ProxyNodeSelect } from './ProxyNodeSelect'
import type { AppDownloadedUpdate, AppUpdateInfo, UpdateProgress } from './types'
import { resolveUpdateError } from './updateErrors'
import {
  loadUpdateChannel,
  loadUpdateProxySelection,
  normalizeUpdateVersion,
  saveUpdateChannel,
  saveUpdateProxySelection,
  type UpdateChannel,
} from './updatePrefs'
import { useUpdateDownloadProxies } from './useUpdateDownloadProxies'

interface UpdateCheckModalProps {
  open: boolean
  onClose: () => void
  currentVersion: string
}

export function UpdateCheckModal({ open, onClose, currentVersion }: UpdateCheckModalProps) {
  const setAppUpdateAvailable = useLayoutStore((state) => state.setAppUpdateAvailable)
  const [info, setInfo] = useState<AppUpdateInfo | null>(null)
  const [checking, setChecking] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [progress, setProgress] = useState<UpdateProgress | null>(null)
  const [confirmInstall, setConfirmInstall] = useState(false)
  // 恢复上次检查/下载用的代理，避免每次打开都退回直连（直连读取不到更新清单）。
  const [savedSelection] = useState(() => loadUpdateProxySelection())
  const [channel, setChannel] = useState<UpdateChannel>(() => loadUpdateChannel())
  // 空字符串表示跟随通道内的最新版本，非空表示用户指定了具体版本。
  const [checkVersion, setCheckVersion] = useState('')
  const [proxyMode, setProxyMode] = useState<'direct' | 'pool' | 'custom'>(savedSelection?.mode ?? 'direct')
  const [proxyId, setProxyId] = useState(savedSelection?.proxyId ?? '')
  const [customProxy, setCustomProxy] = useState(savedSelection?.customProxy ?? '')
  const [downloaded, setDownloaded] = useState<AppDownloadedUpdate | null>(null)
  const { options: proxyOptions, ready: proxyOptionsReady } = useUpdateDownloadProxies(open)
  // 下载完成后自动进入安装确认；用户中途关掉弹窗则不打扰。
  const autoInstallRef = useRef(false)
  const openRef = useRef(open)

  useEffect(() => {
    openRef.current = open
  }, [open])

  useEffect(() => {
    saveUpdateProxySelection({ mode: proxyMode, proxyId, customProxy })
  }, [proxyMode, proxyId, customProxy])

  useEffect(() => {
    saveUpdateChannel(channel)
  }, [channel])

  const selectedPoolProxy = proxyMode === 'pool'
    ? proxyOptions.find(item => item.proxy.proxyId === proxyId)?.proxy
    : undefined

  // 检查和下载共用同一份代理选择；返回 null 表示「选了代理池但没选到可用节点」。
  const resolveProxySelection = (): { proxyConfig: string; proxyID: string } | null => {
    if (proxyMode === 'custom') {
      return { proxyConfig: customProxy.trim(), proxyID: '' }
    }
    if (proxyMode === 'pool') {
      if (!selectedPoolProxy) return null
      return { proxyConfig: selectedPoolProxy.proxyConfig?.trim() || '', proxyID: selectedPoolProxy.proxyId }
    }
    return { proxyConfig: '', proxyID: '' }
  }

  const runCheck = async () => {
    if (checking) return
    const selection = resolveProxySelection()
    if (!selection) {
      toast.error(proxyId ? '所选节点已不可用，请重新选择' : '请先选择代理节点')
      return
    }
    setChecking(true)
    try {
      const result = await checkAppUpdateVersion(selection.proxyConfig, selection.proxyID, channel, checkVersion)
      setInfo(result)
      setAppUpdateAvailable(result.ok && result.updateAvailable)
      if (!result.ok && checkVersion !== '') {
        // 指定版本不存在时退回“最新版本”，避免下拉卡在一个查不到的版本上。
        setCheckVersion('')
      }
      if (result.ok && result.updateAvailable) {
        toast.success(`发现新版本 v${result.latestVersion}`)
      }
    } catch (error: any) {
      setInfo({
        ok: false,
        currentVersion,
        latestVersion: '',
        updateAvailable: false,
        releaseUrl: '',
        notesUrl: '',
        force: false,
        assets: {},
        error: resolveUpdateError(error, '检查更新失败'),
      })
      // 检查失败时保留上次的红点状态：失败不等于没有新版本。
      toast.error(resolveUpdateError(error, '检查更新失败'))
    } finally {
      setChecking(false)
    }
  }

  useEffect(() => {
    if (!open) return
    void getDownloadedAppUpdate()
      .then(setDownloaded)
      .catch(() => setDownloaded(null))
  }, [open])

  // 打开或切换代理后自动检查：代理池模式要等节点列表就绪且能匹配到上次的节点。
  // 自定义代理地址不在这里触发（否则每敲一个字符都会请求一次），改为输入框失焦时检查。
  useEffect(() => {
    if (!open) return
    if (proxyMode === 'pool' && (!proxyOptionsReady || !selectedPoolProxy)) return
    const timer = window.setTimeout(() => {
      if (downloading || installing) return
      void runCheck()
    }, 300)
    return () => window.clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, channel, checkVersion, proxyMode, proxyId, proxyOptionsReady, selectedPoolProxy?.proxyId])

  useEffect(() => {
    const off = EventsOn('app:update:progress', (payload: UpdateProgress) => {
      if (!payload) return
      setProgress(payload)
      if (payload.phase === 'done' || payload.phase === 'error') {
        setDownloading(false)
        if (payload.phase === 'error') {
          autoInstallRef.current = false
          toast.error(payload.message || '更新下载失败')
        } else {
          void getDownloadedAppUpdate()
            .then(setDownloaded)
            .catch(() => setDownloaded(null))
          if (autoInstallRef.current) {
            autoInstallRef.current = false
            if (openRef.current) setConfirmInstall(true)
          }
        }
      }
    })
    return () => off()
  }, [])

  const startDownload = async (autoInstall: boolean) => {
    if (downloading || installing) return
    if (!updateAvailable) {
      toast.error('当前没有可下载的更新')
      return
    }
    const selection = resolveProxySelection()
    if (!selection) {
      toast.error(proxyId ? '所选节点已不可用，请重新选择' : '请先选择代理节点')
      return
    }
    if (proxyMode === 'custom' && !customProxy.trim()) {
      toast.error('请填写代理地址')
      return
    }
    const { proxyConfig, proxyID } = selection

    autoInstallRef.current = autoInstall
    setDownloading(true)
    setProgress({
      phase: 'resolving',
      progress: 0,
      message: proxyConfig || proxyID ? '准备下载（使用指定代理）' : '准备下载（直连）',
    })
    try {
      // 显式传版本：用户可能在下拉里选了非最新版本，后端按版本精确解析资产。
      const started = await downloadAppUpdateWithChannel(info.latestVersion, '', proxyConfig, proxyID, channel)
      if (!started) {
        autoInstallRef.current = false
        toast.error('当前环境不支持应用内更新')
        setDownloading(false)
      }
    } catch (error: any) {
      autoInstallRef.current = false
      toast.error(resolveUpdateError(error, '启动下载失败'))
      setDownloading(false)
    }
  }

  const handlePrimaryAction = async () => {
    if (downloading || installing) return
    // 本地已有可安装的包（版本匹配或当前检查失败），直接进入安装确认。
    if (canInstallDownloaded) {
      setConfirmInstall(true)
      return
    }
    void startDownload(true)
  }

  const handleDownloadOnly = () => {
    void startDownload(false)
  }

  const handleOpenLocation = async () => {
    try {
      const opened = await openUpdatePackageLocation()
      if (!opened) {
        toast.error('当前环境不支持打开文件位置')
      }
    } catch (error: any) {
      toast.error(resolveUpdateError(error, '打开文件位置失败'))
    }
  }

  const handleInstall = async () => {
    if (installing) return
    setInstalling(true)
    try {
      const started = await applyAppUpdate(downloaded?.version || info?.latestVersion || '', '')
      if (!started) {
        toast.error('当前环境不支持自动安装')
        return
      }
      toast.success('即将退出并静默安装')
      window.setTimeout(() => Quit(), 800)
    } catch (error: any) {
      toast.error(resolveUpdateError(error, '准备安装失败'))
      // 安装包可能已被清理，重新读取一次状态，避免按钮继续停在可安装状态。
      void getDownloadedAppUpdate()
        .then(setDownloaded)
        .catch(() => setDownloaded(null))
    } finally {
      setInstalling(false)
    }
  }

  const updateAvailable = info?.ok === true && info.updateAvailable
  // 本地已下载的包就是当前最新版本时，直接给安装入口，避免重新下载。
  const downloadedMatchesLatest =
    !!downloaded &&
    !!info?.latestVersion &&
    normalizeUpdateVersion(downloaded.version) === normalizeUpdateVersion(info.latestVersion)
  // 检查失败时也允许安装已下载的包（版本由后端再次校验）；检查成功但已有更新版本时，要求版本匹配。
  const canInstallDownloaded = !!downloaded && (!info?.ok || (updateAvailable && downloadedMatchesLatest))
  const canDownloadAndInstall = updateAvailable || canInstallDownloaded
  const versionLabel = currentVersion || info?.currentVersion || '未知'
  const latestVersionLabel = info?.latestVersion
    ? `v${info.latestVersion}`
    : checking
      ? '检查中…'
      : '—'
  const statusLabel = checking
    ? '检查中'
    : updateAvailable
      ? '有可用更新'
      : info?.ok
        ? '已是最新'
        : info
          ? '检查失败'
          : '未检查'
  const statusTone = checking
    ? 'bg-[var(--color-accent-muted)] text-[var(--color-accent)]'
    : updateAvailable
      ? 'bg-[var(--color-warning)]/15 text-[var(--color-warning)]'
      : info?.ok
        ? 'bg-[var(--color-success)]/15 text-[var(--color-success)]'
        : info
          ? 'bg-[var(--color-error)]/15 text-[var(--color-error)]'
          : 'bg-[var(--color-bg-surface)] text-[var(--color-text-muted)]'

  return (
    <>
      <Modal
        open={open}
        onClose={onClose}
        title="检测更新"
        width="560px"
        footer={
          <>
            <Button variant="secondary" onClick={onClose}>关闭</Button>
            <Button
              onClick={() => { void handlePrimaryAction() }}
              loading={downloading || installing}
              disabled={!canDownloadAndInstall || checking}
            >
              <Download className="h-4 w-4" />
              {canInstallDownloaded ? '退出并安装' : '下载并且安装'}
            </Button>
            <Button
              variant="ghost"
              disabled={!info?.releaseUrl}
              onClick={() => {
                if (info?.releaseUrl) window.open(info.releaseUrl, '_blank', 'noopener,noreferrer')
              }}
            >
              打开项目页
              <ExternalLink className="h-3.5 w-3.5" />
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-5">
                <div>
                  <p className="text-xs text-[var(--color-text-muted)]">当前版本</p>
                  <p className="mt-1 text-lg font-semibold text-[var(--color-text-primary)]">
                    v{versionLabel}
                  </p>
                </div>
                <div className="h-9 w-px bg-[var(--color-border-muted)]" />
                <div>
                  <p className="text-xs text-[var(--color-text-muted)]">最新版本</p>
                  <p className={`mt-1 text-lg font-semibold ${updateAvailable ? 'text-[var(--color-warning)]' : 'text-[var(--color-text-primary)]'}`}>
                    {latestVersionLabel}
                  </p>
                </div>
              </div>
              <span className={`rounded-full px-2.5 py-1 text-xs font-medium ${statusTone}`}>
                {statusLabel}
              </span>
            </div>
          </div>

          <div className="space-y-3 rounded-lg border border-[var(--color-border-muted)] p-3">
            <FormItem label="更新通道" hint="正式版只更新稳定发布；预览版会包含标记为预发布的版本">
              <Select
                value={channel}
                onChange={(event) => {
                  setChannel(event.target.value as UpdateChannel)
                  setCheckVersion('')
                }}
                options={[
                  { value: 'stable', label: '正式版' },
                  { value: 'preview', label: '预览版' },
                ]}
              />
            </FormItem>
            {info?.ok && (info.versions?.length ?? 0) > 0 ? (
              <FormItem label="版本" hint="只列出高于当前版本的最新两个版本">
                <Select
                  value={checkVersion || info.latestVersion}
                  onChange={(event) => setCheckVersion(event.target.value)}
                  options={(info.versions || []).map((version) => ({ value: version, label: `v${version}` }))}
                />
              </FormItem>
            ) : null}
            <FormItem label="检查 / 下载代理">
              <Select
                value={proxyMode}
                onChange={(event) => setProxyMode(event.target.value as 'direct' | 'pool' | 'custom')}
                options={[
                  { value: 'direct', label: '直连' },
                  { value: 'pool', label: '使用代理池节点' },
                  { value: 'custom', label: '自定义代理地址' },
                ]}
              />
            </FormItem>
            {proxyMode === 'pool' ? (
              <>
                <FormItem label="代理节点">
                  <ProxyNodeSelect
                    value={proxyId}
                    onChange={setProxyId}
                    options={proxyOptions.map(({ proxy, latencyMs }) => ({
                      value: proxy.proxyId,
                      label: proxy.proxyName || proxy.proxyId,
                      latencyMs,
                    }))}
                  />
                </FormItem>
                {proxyOptions.length === 0 ? (
                  <p className="text-xs text-[var(--color-warning)]">
                    暂无可用节点，请先到代理池测速，或改用直连/自定义代理。
                  </p>
                ) : null}
              </>
            ) : null}
            {proxyMode === 'custom' ? (
              <FormItem label="代理地址" hint="例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080">
                <Input
                  value={customProxy}
                  onChange={(event) => setCustomProxy(event.target.value)}
                  onBlur={() => {
                    if (customProxy.trim()) void runCheck()
                  }}
                  placeholder="socks5://127.0.0.1:7890"
                />
              </FormItem>
            ) : null}
          </div>

          {info && !info.ok ? (
            <Alert
              type="error"
              title="查不到最新版本信息"
              message={
                <div className="space-y-1">
                  <p>
                    {info.error.includes('预览版本')
                      ? '预览通道当前没有可用的预览版本，可切换回正式版。'
                      : '当前网络或代理节点读不到 GitHub 上的更新清单，请在上方切换代理或节点。'}
                  </p>
                  {info.error ? (
                    <p className="break-all text-xs text-[var(--color-text-muted)]">{info.error}</p>
                  ) : null}
                </div>
              }
            />
          ) : null}

          {canInstallDownloaded && downloaded ? (
            <div className="flex items-center gap-3 rounded-lg border border-[var(--color-success)]/30 bg-[var(--color-success)]/10 px-3 py-2">
              <div className="flex min-w-0 flex-1 items-center gap-2">
                <CheckCircle2 className="h-4 w-4 shrink-0 text-[var(--color-success)]" />
                <span className="shrink-0 text-sm text-[var(--color-text-primary)]">已下载 v{downloaded.version}</span>
                <span className="truncate text-xs text-[var(--color-text-muted)]" title={downloaded.installerPath}>
                  {downloaded.installerPath}
                </span>
              </div>
              <Button
                size="sm"
                variant="ghost"
                className="shrink-0"
                onClick={() => { void handleOpenLocation() }}
              >
                <FolderOpen className="h-3.5 w-3.5" />
                打开地址
              </Button>
            </div>
          ) : downloading || progress?.phase === 'error' ? (
            <div className="rounded-lg border border-[var(--color-border-muted)] px-3 py-2">
              <Progress
                percent={progress?.progress ?? 0}
                status={progress?.phase === 'error' ? 'error' : 'normal'}
                size="sm"
              />
              <p className="mt-1 text-xs text-[var(--color-text-muted)]">{progress?.message || '准备下载…'}</p>
            </div>
          ) : updateAvailable ? (
            <div className="flex items-center justify-between gap-2 rounded-lg border border-[var(--color-border-muted)] px-3 py-2">
              <span className="text-xs text-[var(--color-text-muted)]">尚未下载</span>
              <Button size="sm" variant="secondary" onClick={handleDownloadOnly} loading={downloading}>
                下载
              </Button>
            </div>
          ) : null}
        </div>
      </Modal>

      <ConfirmModal
        open={confirmInstall}
        title="退出并安装更新"
        content="应用将立即退出，安装包会在后台静默安装，完成后自动启动新版本。若静默安装失败，会自动弹出安装向导。"
        confirmText="退出并安装"
        onClose={() => setConfirmInstall(false)}
        onConfirm={() => { void handleInstall() }}
      />
    </>
  )
}
