import { Download, ExternalLink, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Alert, Button, ConfirmModal, FormItem, Input, Modal, Progress, Select, toast } from '../../shared/components'
import { EventsOn, Quit } from '../../wailsjs/runtime/runtime'
import { fetchBrowserProxies } from '../browser/api'
import type { BrowserProxy } from '../browser/types'
import { useLayoutStore } from '../../store/layoutStore'
import {
  applyAppUpdate,
  checkAppUpdate,
  downloadAppUpdateWithProxySelection,
  getDownloadedAppUpdate,
} from './api'
import type { AppDownloadedUpdate, AppUpdateInfo, UpdateProgress } from './types'

function normalizeVersion(value: string): string {
  return value.trim().replace(/^[vV]/, '')
}

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
  const [proxyMode, setProxyMode] = useState<'direct' | 'pool' | 'custom'>('direct')
  const [proxyId, setProxyId] = useState('')
  const [customProxy, setCustomProxy] = useState('')
  const [proxyOptions, setProxyOptions] = useState<BrowserProxy[]>([])
  const [downloaded, setDownloaded] = useState<AppDownloadedUpdate | null>(null)

  const runCheck = async () => {
    if (checking) return
    setChecking(true)
    try {
      const result = await checkAppUpdate()
      setInfo(result)
      setAppUpdateAvailable(result.ok && result.updateAvailable)
      if (result.ok && result.updateAvailable) {
        toast.success(`发现新版本 v${result.latestVersion}`)
      } else if (result.ok) {
        toast.success('已是最新版本')
      } else {
        toast.error(result.error || '检查更新失败')
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
        error: error?.message || '检查更新失败',
      })
      setAppUpdateAvailable(false)
      toast.error(error?.message || '检查更新失败')
    } finally {
      setChecking(false)
    }
  }

  useEffect(() => {
    if (!open) return
    if (!downloading) void runCheck()
    void fetchBrowserProxies()
      .then((list) => setProxyOptions(list.filter((item) => item.proxyId && item.proxyId !== '__direct__')))
      .catch(() => setProxyOptions([]))
    void getDownloadedAppUpdate()
      .then(setDownloaded)
      .catch(() => setDownloaded(null))
  }, [open])

  useEffect(() => {
    const off = EventsOn('app:update:progress', (payload: UpdateProgress) => {
      if (!payload) return
      setProgress(payload)
      if (payload.phase === 'done' || payload.phase === 'error') {
        setDownloading(false)
        if (payload.phase === 'error') {
          toast.error(payload.message || '更新下载失败')
        } else {
          void getDownloadedAppUpdate()
            .then(setDownloaded)
            .catch(() => setDownloaded(null))
        }
      }
    })
    return () => off()
  }, [])

  const handleDownload = async () => {
    if (downloading) return
    if (proxyMode === 'pool' && !proxyId) {
      toast.error('请先选择代理节点')
      return
    }
    if (proxyMode === 'custom' && !customProxy.trim()) {
      toast.error('请填写代理地址')
      return
    }

    let proxyConfig = ''
    let proxyID = ''
    if (proxyMode === 'pool') {
      const selected = proxyOptions.find((item) => item.proxyId === proxyId)
      proxyConfig = selected?.proxyConfig?.trim() || ''
      proxyID = selected?.proxyId || proxyId
    } else if (proxyMode === 'custom') {
      proxyConfig = customProxy.trim()
    }

    setDownloading(true)
    setProgress({
      phase: 'resolving',
      progress: 0,
      message: proxyConfig || proxyID ? '准备下载（使用指定代理）' : '准备下载（直连）',
    })
    try {
      const started = await downloadAppUpdateWithProxySelection('', '', proxyConfig, proxyID)
      if (!started) {
        toast.error('当前环境不支持应用内更新')
        setDownloading(false)
      }
    } catch (error: any) {
      toast.error(error?.message || '启动下载失败')
      setDownloading(false)
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
      toast.success('即将退出并安装更新')
      window.setTimeout(() => Quit(), 800)
    } catch (error: any) {
      toast.error(error?.message || '准备安装失败')
      // 安装包可能已被清理，重新读取一次状态，避免按钮继续停在“退出并安装”。
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
    normalizeVersion(downloaded.version) === normalizeVersion(info.latestVersion)
  const canInstallDownloaded = updateAvailable && downloadedMatchesLatest
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
            <Button onClick={() => { void runCheck() }} loading={checking}>
              <RefreshCw className="h-4 w-4" />
              重新检查
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
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
            <p className="mt-3 border-t border-[var(--color-border-muted)] pt-3 text-xs text-[var(--color-text-muted)]">
              {checking
                ? '正在获取最新版本信息…'
                : updateAvailable
                  ? '检测到可用更新，请选择下载代理后下载当前平台安装包。'
                  : info?.ok
                    ? '当前版本已包含最新发布内容。'
                    : info
                      ? '检查更新失败，请查看下方原因。'
                      : '点击“重新检查”获取最新版本信息。'}
            </p>
          </div>

          {updateAvailable ? (
            <Alert
              type="warning"
              title={`发现新版本 v${info.latestVersion}`}
              message={
                <div className="flex flex-wrap items-center gap-2">
                  <span>建议更新以获得最新功能与修复。</span>
                  {info.notesUrl ? (
                    <button
                      type="button"
                      className="underline underline-offset-2 hover:text-[var(--color-accent)]"
                      onClick={() => window.open(info.notesUrl, '_blank', 'noopener,noreferrer')}
                    >
                      查看更新说明
                    </button>
                  ) : null}
                </div>
              }
            />
          ) : info?.ok ? (
            <Alert type="success" title="已是最新版本" message={`当前版本 v${versionLabel} 已是最新版本。`} />
          ) : info ? (
            <Alert type="error" title="检查更新失败" message={info.error || '检查更新失败'} />
          ) : null}

          {updateAvailable ? (
            <>
              <div className="space-y-3 rounded-lg border border-[var(--color-border-muted)] p-3">
                <FormItem label="下载代理">
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
                  <FormItem label="代理节点">
                    <Select
                      value={proxyId}
                      onChange={(event) => setProxyId(event.target.value)}
                      options={proxyOptions.map((item) => ({
                        value: item.proxyId,
                        label: item.proxyName || item.proxyId,
                      }))}
                    />
                  </FormItem>
                ) : null}
                {proxyMode === 'custom' ? (
                  <FormItem label="代理地址" hint="例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080">
                    <Input
                      value={customProxy}
                      onChange={(event) => setCustomProxy(event.target.value)}
                      placeholder="socks5://127.0.0.1:7890"
                    />
                  </FormItem>
                ) : null}
              </div>

              {progress ? (
                <div className="rounded-lg border border-[var(--color-border-muted)] px-3 py-2">
                  <Progress
                    percent={progress.progress}
                    status={progress.phase === 'error' ? 'error' : progress.phase === 'done' ? 'success' : 'normal'}
                    size="sm"
                  />
                  <p className="mt-1 text-xs text-[var(--color-text-muted)]">{progress.message}</p>
                </div>
              ) : null}

              {canInstallDownloaded && progress?.phase !== 'done' ? (
                <p className="text-xs text-[var(--color-text-muted)]">
                  本地已下载 v{downloaded?.version}，无需重新下载。
                </p>
              ) : null}

              <div className="flex flex-wrap justify-end gap-2">
                {info.releaseUrl ? (
                  <Button
                    variant="ghost"
                    onClick={() => window.open(info.releaseUrl, '_blank', 'noopener,noreferrer')}
                  >
                    打开下载页
                    <ExternalLink className="h-3.5 w-3.5" />
                  </Button>
                ) : null}
                {progress?.phase === 'done' || canInstallDownloaded ? (
                  <Button onClick={() => setConfirmInstall(true)} loading={installing}>
                    退出并安装
                  </Button>
                ) : (
                  <Button onClick={() => { void handleDownload() }} loading={downloading}>
                    <Download className="h-4 w-4" />
                    {progress?.phase === 'error' ? '重新下载' : '下载更新'}
                  </Button>
                )}
              </div>
            </>
          ) : null}
        </div>
      </Modal>

      <ConfirmModal
        open={confirmInstall}
        title="退出并安装更新"
        content="应用将立即退出，随后自动打开已下载的安装包。系统可能弹出权限确认，请按提示完成安装。"
        confirmText="退出并安装"
        onClose={() => setConfirmInstall(false)}
        onConfirm={() => { void handleInstall() }}
      />
    </>
  )
}
