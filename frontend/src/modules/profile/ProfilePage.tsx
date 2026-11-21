import {
  Github,
  Mail,
  Globe,
  BookOpen,
  MessageSquare,
  Calendar,
  MapPin,
  Coffee,
  Terminal,
  ExternalLink,
  RefreshCw,
  Download,
  AlertTriangle,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Alert, Badge, Button, Card, ConfirmModal, Progress, toast } from '../../shared/components'
import { EventsOff, EventsOn, Quit } from '../../wailsjs/runtime/runtime'
import {
  applyAppUpdate,
  applyPendingAppUpdate,
  checkAppUpdate,
  clearPendingAppUpdate,
  createDefaultProfilePageData,
  downloadAppUpdate,
  getPendingAppUpdate,
  loadCurrentAppVersion,
  loadProfilePageData,
} from './api'
import type { AppUpdateInfo, IconKey, ProfilePageData, UpdatePendingInfo, UpdateProgress } from './types'

const ICON_MAP = {
  'book-open': BookOpen,
  globe: Globe,
  'message-square': MessageSquare,
  github: Github,
  mail: Mail,
  'external-link': ExternalLink,
}

const CHANNEL_ICON_CLASS: Partial<Record<IconKey, string>> = {
  'book-open': 'text-[#1e80ff]',
  globe: 'text-[#0f766e]',
  'message-square': 'text-[#16a34a]',
  github: 'text-[var(--color-text-primary)]',
  mail: 'text-[var(--color-accent)]',
}

export function ProfilePage() {
  const [pageData, setPageData] = useState<ProfilePageData>(() => createDefaultProfilePageData())
  const [appVersion, setAppVersion] = useState('')
  const [updateCheck, setUpdateCheck] = useState<AppUpdateInfo | null>(null)
  const [checkingUpdate, setCheckingUpdate] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [updateProgress, setUpdateProgress] = useState<UpdateProgress | null>(null)
  const [pendingUpdate, setPendingUpdate] = useState<UpdatePendingInfo | null>(null)
  const [confirmInstall, setConfirmInstall] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [testMode, setTestMode] = useState(false)

  useEffect(() => {
    let active = true

    const syncProfile = async () => {
      const data = await loadProfilePageData()
      if (!active) return
      setPageData(data)
    }

    void syncProfile()

    void loadCurrentAppVersion()
      .then((version) => {
        if (active) setAppVersion(version)
      })
      .catch(() => {
        if (active) setAppVersion('')
      })

    EventsOn('app:update:progress', (payload: UpdateProgress) => {
      if (!active || !payload) return
      setUpdateProgress(payload)
      if (payload.phase === 'done' || payload.phase === 'error') {
        setDownloading(false)
        if (payload.phase === 'error') {
          toast.error(payload.message || '更新下载失败')
        } else {
          // 新包已就绪：清掉旧 pending，避免确认时走“重新打开旧安装包”分支
          void clearPendingAppUpdate().then(() => setPendingUpdate(null))
        }
      }
    })
    EventsOn('app:update:pending', (payload: UpdatePendingInfo) => {
      if (active && payload) setPendingUpdate(payload)
    })
    EventsOn('app:update:available', (payload: AppUpdateInfo) => {
      if (active && payload) setUpdateCheck(payload)
    })
    void getPendingAppUpdate()
      .then((pending) => { if (active) setPendingUpdate(pending) })
      .catch(() => {})

    return () => {
      active = false
      EventsOff('app:update:progress')
      EventsOff('app:update:pending')
      EventsOff('app:update:available')
    }
  }, [])

  const openExternal = (url: string) => {
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  const handleCheckForUpdates = async () => {
    if (checkingUpdate) return
    setCheckingUpdate(true)
    try {
      setUpdateCheck(await checkAppUpdate())
    } catch (error: any) {
      setUpdateCheck({
        ok: false,
        currentVersion: appVersion,
        latestVersion: '',
        updateAvailable: false,
        releaseUrl: '',
        notesUrl: '',
        force: false,
        assets: {},
        error: error?.message || '检查更新失败',
      })
    } finally {
      setCheckingUpdate(false)
    }
  }

  const handleDownloadUpdate = async () => {
    if (downloading) return
    if (testMode) {
      runSimulatedDownload(true)
      return
    }
    setDownloading(true)
    setUpdateProgress({ phase: 'resolving', progress: 0, message: '准备下载更新包' })
    try {
      const started = await downloadAppUpdate('', '')
      if (!started) {
        toast.error('当前环境不支持应用内更新')
        setDownloading(false)
      }
    } catch (error: any) {
      toast.error(error?.message || '启动下载失败')
      setDownloading(false)
    }
  }

  const handleApplyUpdate = async () => {
    setConfirmInstall(false)
    setInstalling(true)
    try {
      const started = await applyAppUpdate(pendingUpdate?.version || '', '')
      if (!started) {
        toast.error('当前环境不支持自动安装')
        return
      }
      toast.success('即将退出并安装更新')
      window.setTimeout(() => Quit(), 800)
    } catch (error: any) {
      toast.error(error?.message || '准备安装失败')
    } finally {
      setInstalling(false)
    }
  }

  const handlePendingInstall = async () => {
    if (testMode) {
      setConfirmInstall(true)
      return
    }
    const started = await applyPendingAppUpdate()
    if (!started) {
      toast.error('当前环境不支持自动安装')
      return
    }
    setConfirmInstall(true)
  }

  const runSimulatedDownload = (autoConfirm = false) => {
    setUpdateProgress({ phase: 'resolving', progress: 0, message: '准备下载更新包' })
    window.setTimeout(() => {
      setUpdateProgress({ phase: 'downloading', progress: 60, message: '测试模式：模拟下载中' })
    }, 300)
    window.setTimeout(() => {
      setUpdateProgress({ phase: 'done', progress: 100, message: '测试模式：下载完成' })
      if (autoConfirm) setConfirmInstall(true)
    }, 1200)
  }

  const handleSimulateUpdate = () => {
    setTestMode(true)
    setPendingUpdate(null)
    setUpdateCheck({
      ok: true,
      currentVersion: appVersion || '1.9.5',
      latestVersion: '99.0.0-test',
      updateAvailable: true,
      releaseUrl: '',
      notesUrl: '',
      force: false,
      assets: {},
      error: '',
    })
    runSimulatedDownload(true)
    toast.success('已模拟发现新版本 v99.0.0-test')
  }

  const handleSimulatePending = () => {
    setTestMode(true)
    setUpdateCheck(null)
    setUpdateProgress(null)
    setPendingUpdate({
      version: '99.0.0-test',
      installerPath: '',
      pid: 0,
      createdAt: '',
    })
    setConfirmInstall(true)
    toast.success('已模拟待安装状态')
  }

  const handleClearTest = () => {
    setTestMode(false)
    setUpdateCheck(null)
    setUpdateProgress(null)
    setPendingUpdate(null)
    setConfirmInstall(false)
    toast.success('已清除测试状态')
  }

  const authorInfo = pageData.author
  const projectInfo = pageData.project
  const metaItems = [
    {
      label: authorInfo.location,
      icon: MapPin,
    },
    {
      label: `加入于 ${authorInfo.joinDate}`,
      icon: Calendar,
    },
  ].filter((item) => item.label.trim())

  return (
    <div className="mx-auto max-w-5xl space-y-4 animate-fade-in">
      <Card padding="lg" className="rounded-[26px]">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex min-w-0 flex-col gap-4 sm:flex-row sm:items-start">
            <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-[20px] bg-[#1f2d46] text-[34px] font-bold tracking-[0.08em] text-white shadow-sm">
              {authorInfo.initial}
            </div>

            <div className="min-w-0 space-y-4">
              <div className="space-y-1">
                <h1
                  className="text-[34px] font-bold leading-none tracking-tight text-[var(--color-text-primary)] sm:text-[38px]"
                >
                  {authorInfo.name}
                </h1>
                <p className="text-base text-[var(--color-text-muted)]">{authorInfo.title}</p>
              </div>

              <p className="max-w-3xl text-[15px] leading-8 text-[var(--color-text-secondary)]">
                {authorInfo.bio}
              </p>

              <div className="flex flex-wrap items-center gap-x-5 gap-y-3 text-sm text-[var(--color-text-muted)]">
                {metaItems.map(({ label, icon: Icon }) => (
                  <span key={label} className="inline-flex items-center gap-1.5">
                    <Icon className="h-4 w-4" />
                    {label}
                  </span>
                ))}
                {authorInfo.website ? (
                  <button
                    type="button"
                    className="inline-flex items-center gap-1.5 text-[var(--color-text-primary)] transition-colors hover:text-[var(--color-accent)]"
                    onClick={() => openExternal(authorInfo.website)}
                  >
                    <Globe className="h-4 w-4" />
                    {stripProtocol(authorInfo.website)}
                  </button>
                ) : null}
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-3 lg:justify-end">
            {authorInfo.github ? (
              <Button
                variant="ghost"
                className="h-11 rounded-2xl border border-transparent px-4 text-[var(--color-text-primary)] hover:border-[var(--color-border-default)] hover:bg-[var(--color-bg-muted)]"
                onClick={() => openExternal(authorInfo.github)}
              >
                <Github className="h-4 w-4" />
                GitHub
              </Button>
            ) : null}
          </div>
        </div>
      </Card>

      <div className="grid gap-4 md:grid-cols-3">
        {authorInfo.channels.map((channel) => {
          const Icon = getIcon(channel.icon)
          const iconClassName = CHANNEL_ICON_CLASS[channel.icon || 'globe'] || 'text-[var(--color-text-primary)]'
          const content = (
            <Card
              className="h-full rounded-[22px] border-[var(--color-border-default)] transition-all duration-200 hover:-translate-y-0.5 hover:border-[var(--color-border-strong)] hover:shadow-[var(--shadow-md)]"
              padding="lg"
            >
              <div className="flex items-center justify-between gap-4">
                <div className="flex items-center gap-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[var(--color-bg-muted)]">
                    <Icon className={`h-5 w-5 ${iconClassName}`} />
                  </div>
                  <div className="space-y-1 text-left">
                    <p className="text-[28px] font-bold leading-none text-[var(--color-text-primary)]">
                      {channel.name}
                    </p>
                    <p className="text-sm text-[var(--color-text-muted)]">{channel.description}</p>
                    <p className="text-xs text-[var(--color-text-secondary)]">{channel.detail}</p>
                  </div>
                </div>
                {channel.href ? <ExternalLink className="h-4 w-4 shrink-0 text-[var(--color-text-muted)]" /> : null}
              </div>
            </Card>
          )

          if (!channel.href) {
            return <div key={channel.name}>{content}</div>
          }

          return (
            <a
              key={channel.name}
              href={channel.href}
              target="_blank"
              rel="noopener noreferrer"
              className="block h-full"
            >
              {content}
            </a>
          )
        })}
      </div>

      <Card
        title="技术栈"
        actions={<Terminal className="h-4 w-4 text-[var(--color-text-muted)]" />}
        className="rounded-[24px]"
        padding="lg"
      >
        <div className="flex flex-wrap gap-x-8 gap-y-4 text-[15px] font-semibold text-[var(--color-text-primary)]">
          {authorInfo.skills.map((skill) => (
            <span key={skill}>{skill}</span>
          ))}
        </div>
      </Card>

      <Card title="更新链路测试" className="rounded-[24px]" padding="lg">
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="secondary" size="sm" onClick={handleSimulateUpdate}>模拟发现新版本</Button>
          <Button variant="secondary" size="sm" onClick={handleSimulatePending}>模拟待安装</Button>
          <Button variant="ghost" size="sm" onClick={handleClearTest}>清除测试状态</Button>
          <span className="text-xs text-[var(--color-text-muted)]">仅模拟界面状态，不触发真实下载或退出</span>
        </div>
      </Card>

      <Card
        title="关于本项目"
        actions={<Coffee className="h-4 w-4 text-[var(--color-text-muted)]" />}
        className="rounded-[24px]"
        padding="lg"
      >
        <div className="space-y-4 text-[15px] leading-8 text-[var(--color-text-secondary)]">
          {pendingUpdate ? (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-warning)]/30 bg-[var(--color-warning)]/10 px-3 py-2 text-sm">
              <div className="flex items-center gap-2 text-[var(--color-text-primary)]">
                <AlertTriangle className="h-4 w-4 text-[var(--color-warning)]" />
                上次下载的安装包尚未安装（v{pendingUpdate.version || '未知'}）
              </div>
              <div className="flex gap-2">
                <Button size="sm" onClick={() => { void handlePendingInstall() }}>重新打开安装</Button>
                <Button size="sm" variant="ghost" onClick={() => { void clearPendingAppUpdate().then(() => setPendingUpdate(null)) }}>忽略</Button>
              </div>
            </div>
          ) : null}
          <div className="flex flex-col gap-3 border-b border-[var(--color-border-muted)] pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="font-semibold text-[var(--color-text-primary)]">
                当前版本 {appVersion ? `v${appVersion}` : '未知'}
              </p>
              {updateCheck?.ok && updateCheck.updateAvailable ? (
                <Alert
                  type="warning"
                  title={`发现新版本 v${updateCheck.latestVersion}`}
                  message={
                    <div className="flex flex-wrap items-center gap-2">
                      <span>当前版本 v{appVersion || updateCheck.currentVersion || '未知'}，建议更新以获得最新功能与修复。</span>
                      {updateCheck.notesUrl ? (
                        <button
                          type="button"
                          className="underline underline-offset-2 hover:text-[var(--color-accent)]"
                          onClick={() => openExternal(updateCheck.notesUrl)}
                        >
                          查看更新说明
                        </button>
                      ) : null}
                    </div>
                  }
                />
              ) : updateCheck?.ok ? (
                <p className="text-sm text-[var(--color-text-muted)]">已是最新版本</p>
              ) : updateCheck ? (
                <Alert type="error" title="检查更新失败" message={updateCheck.error || '检查更新失败'} />
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2 sm:justify-end">
              <Button variant="secondary" size="sm" onClick={() => { void handleCheckForUpdates() }} loading={checkingUpdate}>
                <RefreshCw className="h-4 w-4" />
                检查更新
              </Button>
              {updateCheck?.ok && updateCheck.updateAvailable ? (
                updateProgress?.phase === 'done' ? (
                  <Button size="sm" onClick={() => setConfirmInstall(true)} loading={installing}>
                    退出并安装
                  </Button>
                ) : (
                  <Button size="sm" onClick={() => { void handleDownloadUpdate() }} loading={downloading}>
                    <Download className="h-4 w-4" />
                    下载更新
                  </Button>
                )
              ) : null}
              {updateCheck?.ok && updateCheck.updateAvailable && updateCheck.releaseUrl ? (
                <Button variant="ghost" size="sm" onClick={() => openExternal(updateCheck.releaseUrl)}>
                  打开下载页
                  <ExternalLink className="h-3.5 w-3.5" />
                </Button>
              ) : null}
            </div>
          </div>
          {updateProgress && (downloading || updateProgress.phase === 'done' || updateProgress.phase === 'error') ? (
            <div className="rounded-lg border border-[var(--color-border-muted)] px-3 py-2">
              <Progress
                percent={updateProgress.progress}
                status={updateProgress.phase === 'error' ? 'error' : updateProgress.phase === 'done' ? 'success' : 'normal'}
                size="sm"
              />
              <p className="mt-1 text-xs text-[var(--color-text-muted)]">{updateProgress.message}</p>
            </div>
          ) : null}
          <p>
            <Badge className="mr-1 rounded-xl px-3 py-1">{projectInfo.introBadge}</Badge>
            {projectInfo.introText}
          </p>
          <div className="flex flex-wrap gap-2">
            {projectInfo.techStack.map((item) => (
              <Badge key={item} className="rounded-xl px-3 py-1">
                {item}
              </Badge>
            ))}
          </div>
          <p>{projectInfo.description}</p>
          <div className="flex flex-wrap items-center gap-3 pt-2">
            {projectInfo.actions.map((action) => {
              const Icon = getIcon(action.icon)
              return (
                <Button
                  key={action.label}
                  variant="ghost"
                  className="h-10 rounded-xl border border-[var(--color-border-default)] px-3 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]"
                  onClick={() => openExternal(action.href)}
                >
                  <Icon className="h-4 w-4" />
                  {action.label}
                  <ExternalLink className="h-3 w-3" />
                </Button>
              )
            })}
          </div>
        </div>
      </Card>

      <ConfirmModal
        open={confirmInstall}
        title="退出并安装更新"
        content="应用将立即退出，随后自动打开已下载的安装包。系统可能弹出权限确认，请按提示完成安装。"
        confirmText="退出并安装"
        onConfirm={() => {
          if (testMode) {
            setConfirmInstall(false)
            toast.success('测试模式：跳过退出')
            return
          }
          if (pendingUpdate) {
            // pending 场景已由 handlePendingInstall 启动 waiter，这里只负责退出
            toast.success('即将退出并安装更新')
            window.setTimeout(() => Quit(), 800)
            return
          }
          void handleApplyUpdate()
        }}
        onClose={() => setConfirmInstall(false)}
      />
    </div>
  )
}

function getIcon(icon?: IconKey) {
  return ICON_MAP[icon || 'globe'] || Globe
}

function stripProtocol(value: string): string {
  return value.replace(/^https?:\/\//, '').replace(/\/$/, '')
}
