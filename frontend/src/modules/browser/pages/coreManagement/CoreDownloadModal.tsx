import type { Dispatch, SetStateAction } from 'react'
import { Button, FormItem, Input, Modal, toast } from '../../../../shared/components'
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'
import type { BrowserProxy } from '../../types'
import type { CoreDownloadForm, CoreDownloadProgress } from '../coreManagement.types'
import { FINGERPRINT_CHROMIUM_RELEASES_URL } from './coreDownloadRecommendation'
import type { CoreDownloadRecommendation } from './coreDownloadRecommendation'

interface CoreDownloadModalProps {
  open: boolean
  form: CoreDownloadForm
  progress: CoreDownloadProgress | null
  recommendation: CoreDownloadRecommendation | null
  recommendationLoading: boolean
  recommendationError: string
  proxies: BrowserProxy[]
  setForm: Dispatch<SetStateAction<CoreDownloadForm>>
  setProgress: Dispatch<SetStateAction<CoreDownloadProgress | null>>
  onRefreshRecommendation: () => void
  onClose: () => void
  onStart: () => void
}

export function CoreDownloadModal({
  open,
  form,
  progress,
  recommendation,
  recommendationLoading,
  recommendationError,
  proxies,
  setForm,
  setProgress,
  onRefreshRecommendation,
  onClose,
  onStart,
}: CoreDownloadModalProps) {
  const downloading = progress !== null && progress.phase !== 'error'
  const isRedownload = form.mode === 'redownload'

  const handleClose = () => {
    if (progress && progress.phase !== 'done' && progress.phase !== 'error') {
      toast.warning('正在下载中，请稍候...')
      return
    }
    onClose()
    setProgress(null)
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={isRedownload ? '重新下载内核' : '下载内核'}
      width="480px"
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={downloading}>取消</Button>
          <Button onClick={onStart} loading={downloading}>{isRedownload ? '开始重新下载' : '开始下载'}</Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="内核名称" required>
          <Input
            value={form.name}
            onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
            placeholder={recommendation?.namePlaceholder || '例如: chrome-latest'}
            disabled={progress !== null || isRedownload}
          />
          {!isRedownload && (
            <p className="text-xs text-[var(--color-text-muted)] mt-1">该名称将同时作为数据存放的子文件夹名。</p>
          )}
        </FormItem>

        {isRedownload && (
          <div className="rounded-lg border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 p-3 text-xs leading-5 text-[var(--color-warning)]">
            重新下载会在校验新压缩包可用后替换当前内核目录；替换失败会自动恢复旧目录。正在使用该内核的实例请先停止。
          </div>
        )}

        <FormItem label="下载地址" required>
          <Input
            value={form.url}
            onChange={e => setForm(prev => ({ ...prev, url: e.target.value }))}
            placeholder={recommendationLoading ? '正在匹配当前环境...' : 'https://github.com/.../chromium.zip'}
            disabled={progress !== null}
          />
          <div className="text-xs text-[var(--color-text-muted)] mt-2 flex items-center justify-between gap-3 bg-[var(--color-bg-muted)] p-2 rounded">
            <span className="min-w-0 truncate">
              {recommendation
                ? `已按 ${recommendation.target.label} 推荐：${recommendation.assetName}`
                : recommendationLoading
                  ? '正在按当前环境获取推荐地址...'
                  : recommendationError || '未获取到当前环境推荐地址'}
            </span>
            {recommendation && (
              <button
                type="button"
                onClick={() => setForm(prev => ({ ...prev, url: recommendation.downloadUrl }))}
                className="shrink-0 text-[var(--color-accent)] hover:underline cursor-pointer font-medium"
                disabled={progress !== null}
              >
                使用推荐
              </button>
            )}
            <button
              type="button"
              onClick={() => BrowserOpenURL(recommendation?.releasesUrl || FINGERPRINT_CHROMIUM_RELEASES_URL)}
              className="shrink-0 text-[var(--color-accent)] hover:underline cursor-pointer font-medium"
            >
              打开 Releases
            </button>
            {!recommendationLoading && !recommendation && (
              <button
                type="button"
                onClick={onRefreshRecommendation}
                className="shrink-0 text-[var(--color-accent)] hover:underline cursor-pointer font-medium"
              >
                重试
              </button>
            )}
          </div>
        </FormItem>

        <FormItem label="下载代理设置">
          <select
            value={form.proxyMode}
            onChange={e => {
              const mode = e.target.value
              setForm(prev => ({
                ...prev,
                proxyMode: mode,
                proxyId: mode === 'custom' && proxies.length > 0 ? proxies[0].proxyId : '',
              }))
            }}
            className="w-full h-9 px-3 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] text-sm focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)] focus:border-[var(--color-accent)]"
            disabled={progress !== null}
          >
            <option value="system">跟随系统全局代理</option>
            <option value="direct">直连模式 (不使用代理)</option>
            {proxies.length > 0 && <option value="custom">指定应用代理配置...</option>}
          </select>
        </FormItem>

        {form.proxyMode === 'custom' && (
          <FormItem label="选择代理池节点" required>
            <select
              value={form.proxyId}
              onChange={e => setForm(prev => ({ ...prev, proxyId: e.target.value }))}
              className="w-full h-9 px-3 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] text-sm focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)] focus:border-[var(--color-accent)]"
              disabled={progress !== null}
            >
              {proxies.map(proxy => (
                <option key={proxy.proxyId} value={proxy.proxyId}>
                  {proxy.proxyName} ({proxy.proxyConfig})
                </option>
              ))}
            </select>
          </FormItem>
        )}

        {progress && (
          <div className="mt-4 p-4 border border-[var(--color-border-default)] rounded-lg bg-[var(--color-bg-secondary)]">
            <div className="flex justify-between text-sm mb-2">
              <span className="font-medium text-[var(--color-text-primary)]">{progress.message}</span>
              <span className="text-[var(--color-text-muted)]">{progress.progress}%</span>
            </div>
            <div className="w-full bg-[var(--color-bg-surface)] rounded-full h-2 overflow-hidden border border-[var(--color-border-muted)]">
              <div
                className="bg-[var(--color-accent)] h-2 rounded-full transition-all duration-300"
                style={{ width: `${Math.max(0, Math.min(100, progress.progress))}%` }}
              />
            </div>
          </div>
        )}
      </div>
    </Modal>
  )
}
