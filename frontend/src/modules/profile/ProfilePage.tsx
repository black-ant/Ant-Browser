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
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Badge, Button, Card } from '../../shared/components'
import { checkForUpdates, createDefaultProfilePageData, loadCurrentAppVersion, loadProfilePageData } from './api'
import type { AppUpdateCheckResult, IconKey, ProfilePageData } from './types'

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
  const [updateCheck, setUpdateCheck] = useState<AppUpdateCheckResult | null>(null)
  const [checkingUpdate, setCheckingUpdate] = useState(false)

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

    return () => {
      active = false
    }
  }, [])

  const openExternal = (url: string) => {
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  const handleCheckForUpdates = async () => {
    if (checkingUpdate) return
    setCheckingUpdate(true)
    try {
      setUpdateCheck(await checkForUpdates())
    } catch (error: any) {
      setUpdateCheck({
        ok: false,
        currentVersion: appVersion,
        latestVersion: '',
        updateAvailable: false,
        releaseUrl: '',
        releaseName: '',
        publishedAt: '',
        error: error?.message || '检查更新失败',
      })
    } finally {
      setCheckingUpdate(false)
    }
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

      <Card
        title="关于本项目"
        actions={<Coffee className="h-4 w-4 text-[var(--color-text-muted)]" />}
        className="rounded-[24px]"
        padding="lg"
      >
        <div className="space-y-4 text-[15px] leading-8 text-[var(--color-text-secondary)]">
          <div className="flex flex-col gap-3 border-b border-[var(--color-border-muted)] pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="font-semibold text-[var(--color-text-primary)]">
                当前版本 {appVersion ? `v${appVersion}` : '未知'}
              </p>
              {updateCheck?.ok ? (
                <p className="text-sm text-[var(--color-text-muted)]">
                  {updateCheck.updateAvailable ? `发现新版本 v${updateCheck.latestVersion}` : '已是最新版本'}
                </p>
              ) : updateCheck ? (
                <p className="text-sm text-[var(--color-error)]">{updateCheck.error || '检查更新失败'}</p>
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2 sm:justify-end">
              <Button variant="secondary" size="sm" onClick={() => { void handleCheckForUpdates() }} loading={checkingUpdate}>
                <RefreshCw className="h-4 w-4" />
                检查更新
              </Button>
              {updateCheck?.ok && updateCheck.updateAvailable && updateCheck.releaseUrl ? (
                <Button variant="ghost" size="sm" onClick={() => openExternal(updateCheck.releaseUrl)}>
                  打开下载页
                  <ExternalLink className="h-3.5 w-3.5" />
                </Button>
              ) : null}
            </div>
          </div>
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
    </div>
  )
}

function getIcon(icon?: IconKey) {
  return ICON_MAP[icon || 'globe'] || Globe
}

function stripProtocol(value: string): string {
  return value.replace(/^https?:\/\//, '').replace(/\/$/, '')
}
