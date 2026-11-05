import { useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { Keyboard, Play, Search, Tag } from 'lucide-react'
import { Badge, Button, Modal, toast } from '../../../shared/components'
import { startBrowserInstanceByCode } from '../api'
import type { BrowserProfile } from '../types'
import { resolveActionFeedback } from '../utils/actionErrors'

interface QuickLaunchModalProps {
  open: boolean
  onClose: () => void
}

import { buildSearchText, GROUP_ALL, GROUP_UNGROUPED, normalizeCode, normalizeText, pickPrimaryTag, UNTAGGED_LABEL, type GroupFilterOption, type ProfileTagSection } from './QuickLaunchModal.helpers'
import { useQuickLaunchData } from './useQuickLaunchData'
export function QuickLaunchModal({ open, onClose }: QuickLaunchModalProps) {
  const [query, setQuery] = useState('')
  const [groupFilter, setGroupFilter] = useState(GROUP_ALL)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [startingCode, setStartingCode] = useState('')
  const [activeTag, setActiveTag] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const sectionScrollRef = useRef<HTMLDivElement>(null)
  const sectionRefs = useRef<Record<string, HTMLElement | null>>({})
  const sectionsScrollableRef = useRef(false)
  const autoScrollingRef = useRef(false)
  const autoScrollTimerRef = useRef<number | null>(null)


  const { profiles, groups, loading } = useQuickLaunchData(open, inputRef)

  useEffect(() => {
    if (!open) return
    setQuery('')
    setGroupFilter(GROUP_ALL)
    setSelectedIndex(0)
    return () => {
      setStartingCode('')
      setActiveTag('')
    }
  }, [open])
  const groupNameMap = useMemo(() => {
    const map = new Map<string, string>()
    groups.forEach((group) => {
      map.set(group.groupId, group.groupName)
    })
    return map
  }, [groups])

  const groupOptions = useMemo<GroupFilterOption[]>(() => {
    const countMap = new Map<string, number>()
    profiles.forEach((profile) => {
      const key = (profile.groupId || '').trim() || GROUP_UNGROUPED
      countMap.set(key, (countMap.get(key) || 0) + 1)
    })

    const options: GroupFilterOption[] = groups.map((group) => ({
      id: group.groupId,
      name: group.groupName,
      count: countMap.get(group.groupId) || 0,
    }))

    for (const [key, count] of countMap.entries()) {
      if (key === GROUP_UNGROUPED) continue
      if (!options.some((item) => item.id === key)) {
        options.push({ id: key, name: groupNameMap.get(key) || `分组 ${key}`, count })
      }
    }

    options.push({
      id: GROUP_UNGROUPED,
      name: '未分组',
      count: countMap.get(GROUP_UNGROUPED) || 0,
    })

    return options
  }, [profiles, groups, groupNameMap])

  useEffect(() => {
    if (groupFilter === GROUP_ALL) return
    if (!groupOptions.some((item) => item.id === groupFilter)) {
      setGroupFilter(GROUP_ALL)
    }
  }, [groupFilter, groupOptions])

  const groupFilteredProfiles = useMemo(() => {
    if (groupFilter === GROUP_ALL) return profiles
    if (groupFilter === GROUP_UNGROUPED) {
      return profiles.filter((profile) => !(profile.groupId || '').trim())
    }
    return profiles.filter((profile) => (profile.groupId || '').trim() === groupFilter)
  }, [profiles, groupFilter])

  const filteredProfiles = useMemo(() => {
    const q = normalizeText(query)
    if (!q) return groupFilteredProfiles
    return groupFilteredProfiles.filter((item) => buildSearchText(item).includes(q))
  }, [groupFilteredProfiles, query])

  useEffect(() => {
    if (filteredProfiles.length === 0) {
      setSelectedIndex(0)
      return
    }
    if (selectedIndex >= filteredProfiles.length) {
      setSelectedIndex(0)
    }
  }, [filteredProfiles, selectedIndex])

  const profileIndexMap = useMemo(() => {
    const map = new Map<string, number>()
    filteredProfiles.forEach((profile, index) => {
      map.set(profile.profileId, index)
    })
    return map
  }, [filteredProfiles])

  const tagSections = useMemo<ProfileTagSection[]>(() => {
    if (!filteredProfiles.length) return []

    const bucket = new Map<string, BrowserProfile[]>()
    for (const profile of filteredProfiles) {
      const tag = pickPrimaryTag(profile)
      if (!bucket.has(tag)) {
        bucket.set(tag, [])
      }
      bucket.get(tag)!.push(profile)
    }

    const tags = Array.from(bucket.keys()).sort((a, b) => {
      if (a === UNTAGGED_LABEL) return 1
      if (b === UNTAGGED_LABEL) return -1
      return a.localeCompare(b, 'zh-CN')
    })

    return tags.map((tag) => ({ tag, items: bucket.get(tag)! }))
  }, [filteredProfiles])

  useEffect(() => {
    if (tagSections.length === 0) {
      setActiveTag('')
      return
    }
    if (!activeTag || !tagSections.some(s => s.tag === activeTag)) {
      setActiveTag(tagSections[0].tag)
    }
  }, [tagSections, activeTag])

  useEffect(() => {
    const target = filteredProfiles[selectedIndex]
    if (!target) return
    if (!sectionsScrollableRef.current) return
    setActiveTag(pickPrimaryTag(target))
  }, [selectedIndex, filteredProfiles])

  useEffect(() => {
    const container = sectionScrollRef.current
    if (!container || tagSections.length === 0) return

    const isScrollable = container.scrollHeight > container.clientHeight + 4
    sectionsScrollableRef.current = isScrollable
    if (!isScrollable) {
      autoScrollingRef.current = false
      return
    }

    const syncActiveTagByScroll = () => {
      if (autoScrollingRef.current) return

      const containerRect = container.getBoundingClientRect()
      const anchorTop = containerRect.top + 16
      let nextActiveTag = tagSections[0].tag

      for (const section of tagSections) {
        const el = sectionRefs.current[section.tag]
        if (!el) continue
        if (el.getBoundingClientRect().top <= anchorTop) {
          nextActiveTag = section.tag
        } else {
          break
        }
      }

      if (container.scrollTop + container.clientHeight >= container.scrollHeight - 4) {
        nextActiveTag = tagSections[tagSections.length - 1].tag
      }

      setActiveTag((prev) => (prev === nextActiveTag ? prev : nextActiveTag))
    }

    syncActiveTagByScroll()
    container.addEventListener('scroll', syncActiveTagByScroll, { passive: true })
    return () => {
      container.removeEventListener('scroll', syncActiveTagByScroll)
    }
  }, [tagSections])

  useEffect(() => {
    return () => {
      if (autoScrollTimerRef.current != null) {
        window.clearTimeout(autoScrollTimerRef.current)
      }
    }
  }, [])

  const startByCode = async (code: string): Promise<boolean> => {
    const normalized = normalizeCode(code)
    if (!normalized || startingCode) return false

    setStartingCode(normalized)
    try {
      const profile = await startBrowserInstanceByCode(normalized)
      if (profile?.runtimeWarning) {
        toast.warning(`实例「${profile.profileName || normalized}」已启动：${profile.runtimeWarning}`)
      }
      onClose()
      return true
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '按 Code 启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
      return false
    } finally {
      setStartingCode('')
    }
  }

  const startProfile = async (profile: BrowserProfile) => {
    if (!profile.launchCode) {
      toast.error('该实例尚未分配 Code，请先在实例列表设置')
      return
    }
    await startByCode(profile.launchCode)
  }

  const onPanelKeyDown = async (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
      return
    }

    if (event.key === 'Enter') {
      event.preventDefault()
      if (query.trim()) {
        const ok = await startByCode(query)
        if (ok) return
      }
      const target = filteredProfiles[selectedIndex]
      if (target) {
        void startProfile(target)
      }
      return
    }

    if (!filteredProfiles.length) return

    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setSelectedIndex((prev) => (prev + 1) % filteredProfiles.length)
      return
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      setSelectedIndex((prev) => (prev - 1 + filteredProfiles.length) % filteredProfiles.length)
    }
  }

  const jumpToTag = (tag: string) => {
    setActiveTag(tag)
    const container = sectionScrollRef.current
    const target = sectionRefs.current[tag]
    if (!container || !target) return
    if (!sectionsScrollableRef.current) return

    const containerRect = container.getBoundingClientRect()
    const targetRect = target.getBoundingClientRect()
    const targetTop = Math.max(container.scrollTop + (targetRect.top - containerRect.top) - 8, 0)
    autoScrollingRef.current = true
    if (autoScrollTimerRef.current != null) {
      window.clearTimeout(autoScrollTimerRef.current)
    }

    container.scrollTo({
      top: targetTop,
      behavior: 'smooth',
    })
    autoScrollTimerRef.current = window.setTimeout(() => {
      autoScrollingRef.current = false
    }, 420)
  }

  return (
    <Modal open={open} onClose={onClose} title="快速启动浏览器" width="1120px">
      <div className="space-y-4" onKeyDown={onPanelKeyDown}>
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(300px,420px)_1fr] lg:items-center">
          <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] px-3 bg-[var(--color-bg-surface)]">
            <Search className="w-4 h-4 text-[var(--color-text-muted)]" />
            <input
              ref={inputRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="输入 Code 或实例名 / 标签 / 关键字"
              className="h-9 w-full border-0 bg-transparent text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none px-0"
            />
            <Button
              size="sm"
              className="shrink-0 whitespace-nowrap px-2.5"
              loading={!!startingCode}
              onClick={() => void startByCode(query)}
              disabled={!query.trim()}
            >
              按Code启动
            </Button>
          </div>

          <div className="min-w-0 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2">
            <div className="flex items-center gap-2 overflow-x-auto">
              <button
                type="button"
                onClick={() => {
                  setGroupFilter(GROUP_ALL)
                  setSelectedIndex(0)
                }}
                className={[
                  'shrink-0 px-3 py-1.5 rounded-md text-xs transition-colors',
                  groupFilter === GROUP_ALL
                    ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
                    : 'text-[var(--color-text-secondary)] bg-[var(--color-bg-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                ].join(' ')}
              >
                全部分组（{profiles.length}）
              </button>
              {groupOptions.map((option) => (
                <button
                  key={option.id}
                  type="button"
                  onClick={() => {
                    setGroupFilter(option.id)
                    setSelectedIndex(0)
                  }}
                  className={[
                    'shrink-0 px-3 py-1.5 rounded-md text-xs transition-colors',
                    groupFilter === option.id
                      ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
                      : 'text-[var(--color-text-secondary)] bg-[var(--color-bg-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                  ].join(' ')}
                >
                  {option.name}（{option.count}）
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="rounded-lg border border-[var(--color-border-default)] overflow-hidden">
          <div className="h-[520px] bg-[var(--color-bg-elevated)] flex">
            <aside className="w-52 shrink-0 border-r border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-2 overflow-y-auto">
              <div className="px-2 py-2 text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wide">
                标签导航
              </div>
              <div className="space-y-1">
                {tagSections.map(section => (
                  <button
                    key={section.tag}
                    type="button"
                    onClick={() => jumpToTag(section.tag)}
                    className={[
                      'w-full flex items-center justify-between rounded-md px-2.5 py-2 text-left transition-colors',
                      activeTag === section.tag
                        ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
                        : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                    ].join(' ')}
                  >
                    <span className="inline-flex items-center gap-1.5 truncate text-sm">
                      <Tag className="w-3.5 h-3.5 shrink-0" />
                      <span className="truncate">{section.tag}</span>
                    </span>
                    <span className="text-xs opacity-80">{section.items.length}</span>
                  </button>
                ))}
              </div>
            </aside>

            <div ref={sectionScrollRef} className="flex-1 overflow-y-auto p-4">
              {loading ? (
                <div className="px-4 py-8 text-sm text-[var(--color-text-muted)] text-center">加载中...</div>
              ) : filteredProfiles.length === 0 ? (
                <div className="px-4 py-8 text-sm text-[var(--color-text-muted)] text-center">没有匹配的实例</div>
              ) : (
                <div className="space-y-4">
                  {tagSections.map((section) => (
                    <section
                      key={section.tag}
                      className="space-y-2"
                      ref={(el) => {
                        sectionRefs.current[section.tag] = el
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <h3 className="text-sm font-semibold text-[var(--color-text-primary)] inline-flex items-center gap-1.5">
                          <Tag className="w-3.5 h-3.5" /> {section.tag}
                        </h3>
                        <span className="text-xs text-[var(--color-text-muted)]">{section.items.length} 个实例</span>
                      </div>

                      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                        {section.items.map((profile) => {
                          const index = profileIndexMap.get(profile.profileId) ?? -1
                          const selected = index === selectedIndex
                          const profileCode = normalizeCode(profile.launchCode)

                          return (
                            <button
                              key={profile.profileId}
                              type="button"
                              onMouseEnter={() => setSelectedIndex(index >= 0 ? index : 0)}
                              onDoubleClick={() => void startProfile(profile)}
                              className={[
                                'w-full text-left rounded-lg border p-3 transition-colors',
                                selected
                                  ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                                  : 'border-[var(--color-border-default)] hover:bg-[var(--color-bg-secondary)]',
                              ].join(' ')}
                            >
                              <div className="flex items-start justify-between gap-2">
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2 flex-wrap">
                                    <span className="text-sm font-medium text-[var(--color-text-primary)] truncate max-w-[180px]">{profile.profileName}</span>
                                    <Badge variant={profile.running ? 'success' : 'warning'} size="sm" dot>
                                      {profile.running ? '运行中' : '已停止'}
                                    </Badge>
                                  </div>

                                  <div className="mt-1.5">
                                    {profile.launchCode ? (
                                      <code className="text-[11px] px-1.5 py-0.5 rounded bg-[var(--color-bg-secondary)] text-[var(--color-accent)] border border-[var(--color-border-muted)]">
                                        {profile.launchCode}
                                      </code>
                                    ) : (
                                      <Badge size="sm" variant="warning">无 Code</Badge>
                                    )}
                                  </div>

                                  {(profile.tags?.length || 0) > 0 && (
                                    <div className="mt-2 flex items-center gap-1.5 flex-wrap">
                                      {profile.tags.slice(0, 3).map((tag) => (
                                        <Badge key={tag} size="sm">{tag}</Badge>
                                      ))}
                                    </div>
                                  )}
                                </div>

                                <Button
                                  size="sm"
                                  loading={!!startingCode && profileCode === startingCode}
                                  onClick={(e) => {
                                    e.stopPropagation()
                                    void startProfile(profile)
                                  }}
                                  disabled={!profile.launchCode}
                                >
                                  <Play className="w-3.5 h-3.5 fill-current" /> 启动
                                </Button>
                              </div>
                            </button>
                          )
                        })}
                      </div>
                    </section>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        <div className="flex items-center justify-between text-xs text-[var(--color-text-muted)]">
          <div className="inline-flex items-center gap-1.5">
            <Keyboard className="w-3.5 h-3.5" />
            <span>回车优先按输入 Code 启动；↑/↓ 按顺序切换卡片；Esc 关闭</span>
          </div>
          <span>全局快捷键: Ctrl/Cmd + K</span>
        </div>
      </div>
    </Modal>
  )
}
