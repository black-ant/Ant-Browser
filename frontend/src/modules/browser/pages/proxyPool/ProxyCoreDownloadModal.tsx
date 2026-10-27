import { useState } from 'react'
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
import { Button, FormItem, Input, Modal, Select, toast } from '../../../../shared/components'
import { browserProxyCoreDownloadInfo, browserProxyCoreOpenLocal } from '../../api'
import type { ProxyCoreDownloadInfoResult, ProxyCoreDownloadProgress, ProxyCoreStatusResult } from '../../types'

interface ProxyCoreDownloadModalProps {
  open: boolean
  core: string
  goos: string
  goarch: string
  downloadProxy: string
  progress: ProxyCoreDownloadProgress | null
  status: ProxyCoreStatusResult | null
  statusLoading: boolean
  onCoreChange: (core: string) => void
  onGOOSChange: (goos: string) => void
  onGOARCHChange: (goarch: string) => void
  onDownloadProxyChange: (proxy: string) => void
  onClose: () => void
  onStart: () => void
}

export function ProxyCoreDownloadModal({
  open,
  core,
  goos,
  goarch,
  downloadProxy,
  progress,
  status,
  statusLoading,
  onCoreChange,
  onGOOSChange,
  onGOARCHChange,
  onDownloadProxyChange,
  onClose,
  onStart,
}: ProxyCoreDownloadModalProps) {
  const downloading = !!progress && !['done', 'error'].includes(progress.phase)
  const [manualInfo, setManualInfo] = useState<ProxyCoreDownloadInfoResult | null>(null)
  const [manualLoading, setManualLoading] = useState(false)

  const loadManualInfo = async () => {
    const info = await browserProxyCoreDownloadInfo(core, goos, goarch, downloadProxy)
    setManualInfo(info)
    return info
  }

  const handleOpenRemote = async () => {
    try {
      setManualLoading(true)
      const info = await loadManualInfo()
      const url = info.downloadUrl || info.releaseUrl
      if (!url) {
        toast.error(info.message || '没有可打开的远程地址')
        return
      }
      BrowserOpenURL(url)
      if (!info.downloadUrl && info.message) toast.info(info.message)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '打开远程地址失败')
    } finally {
      setManualLoading(false)
    }
  }

  const handleOpenLocal = async () => {
    try {
      const ok = await browserProxyCoreOpenLocal(core, goos, goarch)
      if (!ok) toast.error('未连接后端')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '打开本地目录失败')
    }
  }

  const handleClose = () => {
    if (downloading) {
      toast.warning('正在下载中，请稍候')
      return
    }
    onClose()
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="下载代理内核"
      width="460px"
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={downloading}>关闭</Button>
          <Button onClick={onStart} loading={downloading}>开始下载</Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="代理内核">
          <Select
            value={core}
            onChange={e => onCoreChange(e.target.value)}
            disabled={downloading}
            options={[
              { value: 'xray', label: 'Xray（默认）' },
              { value: 'mihomo', label: 'Mihomo' },
              { value: 'sing-box', label: 'sing-box' },
              { value: 'mieru', label: 'mieru（按需）' },
            ]}
          />
        </FormItem>

        <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm">
          <div className="flex items-center justify-between gap-3">
            <span className="text-[var(--color-text-muted)]">内核状态</span>
            <span className={status?.installed ? 'text-green-600' : 'text-red-500'}>
              {statusLoading ? '检测中...' : status?.message || '未知'}
            </span>
          </div>
          {status?.binaryPath && (
            <div className="mt-1 truncate text-xs text-[var(--color-text-muted)]" title={status.binaryPath}>
              {status.binaryPath}
            </div>
          )}
        </div>

        <div className="grid grid-cols-2 gap-3">
          <FormItem label="目标系统">
            <Select
              value={goos}
              onChange={e => onGOOSChange(e.target.value)}
              disabled={downloading}
              options={[
                { value: 'windows', label: 'Windows' },
                { value: 'linux', label: 'Linux' },
                { value: 'darwin', label: 'macOS' },
              ]}
            />
          </FormItem>
          <FormItem label="目标架构">
            <Select
              value={goarch}
              onChange={e => onGOARCHChange(e.target.value)}
              disabled={downloading}
              options={[
                { value: 'amd64', label: 'amd64 / x64' },
                { value: 'arm64', label: 'arm64' },
                { value: '386', label: '386 / x86' },
              ]}
            />
          </FormItem>
        </div>

        <FormItem label="下载代理（可选）">
          <Input
            value={downloadProxy}
            onChange={e => onDownloadProxyChange(e.target.value)}
            disabled={downloading}
            placeholder="默认直连；如 socks5://127.0.0.1:7890"
          />
        </FormItem>

        {progress && (
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] p-3">
            <div className="mb-2 flex items-center justify-between gap-3 text-sm">
              <span className={progress.phase === 'error' ? 'text-red-500' : 'text-[var(--color-text-primary)]'}>
                {progress.message}
              </span>
              <span className="text-[var(--color-text-muted)]">{progress.progress}%</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)]">
              <div
                className={progress.phase === 'error' ? 'h-2 bg-red-500 transition-all' : 'h-2 bg-[var(--color-accent)] transition-all'}
                style={{ width: `${Math.max(0, Math.min(100, progress.progress))}%` }}
              />
            </div>
          </div>
        )}

        <div className="rounded-lg border border-[var(--color-border-default)] p-3 text-sm">
          <div className="flex gap-2">
            <Button variant="secondary" onClick={handleOpenLocal} disabled={downloading}>打开本地目录</Button>
            <Button variant="secondary" onClick={handleOpenRemote} loading={manualLoading} disabled={downloading}>打开远程地址</Button>
          </div>
          {(manualInfo || progress?.phase === 'error') && (
            <div className="mt-2 space-y-1 text-xs text-[var(--color-text-muted)]">
              <div>{manualInfo?.message || '自动下载失败时，请打开远程地址手动下载。'}</div>
              {manualInfo?.version && <div>版本：{manualInfo.version}</div>}
              <div>放置位置：{manualInfo?.installDir || '点击“打开本地目录”查看'}</div>
              {manualInfo?.assetName && <div>文件：{manualInfo.assetName}</div>}
            </div>
          )}
        </div>
      </div>
    </Modal>
  )
}
