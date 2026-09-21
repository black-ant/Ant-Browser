import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Bell,
  BookOpenText,
  Bookmark,
  Briefcase,
  ChevronsUpDown,
  Fingerprint,
  Folders,
  Layers3,
  Moon,
  Plus,
  Puzzle,
  Route,
  ScanFace,
  ScrollText,
  Search,
  Settings2,
  Sun,
  Workflow,
  type LucideIcon,
} from 'lucide-react'
import { projectConfig } from '../../config'
import { useTheme } from '../../shared/theme'
import { useNotificationStore } from '../../store/notificationStore'
import '../styles.css'
import './fish.css'
import './fish-remaining.css'

type FishNavItem = { label: string; path: string; icon: LucideIcon; count?: string }
type FishNavGroup = { label?: string; items: FishNavItem[] }

const navGroups: FishNavGroup[] = [
  {
    items: [
      { label: '浏览器环境', path: '/browser/list', icon: Layers3 },
      { label: '分组与标签', path: '/browser/tags', icon: Folders },
    ],
  },
  {
    label: '网络与指纹',
    items: [
      { label: '代理', path: '/browser/proxy-pool', icon: Route },
      { label: '内核与指纹', path: '/browser/cores', icon: Fingerprint },
      { label: '扩展', path: '/browser/extensions', icon: Puzzle },
      { label: '默认书签', path: '/browser/bookmarks', icon: Bookmark },
    ],
  },
  {
    label: '自动化',
    items: [
      { label: '自动化任务', path: '/browser/automation', icon: Workflow },
    ],
  },
  {
    label: '系统',
    items: [
      { label: '日志', path: '/browser/logs', icon: ScrollText },
      { label: '文档', path: '/system/docs', icon: BookOpenText },
      { label: '设置', path: '/settings', icon: Settings2 },
    ],
  },
]

interface FishShellProps {
  children: ReactNode
  onOpenQuickLaunch: () => void
}
export function FishShell({ children, onOpenQuickLaunch }: FishShellProps) {
  const location = useLocation()
  const { theme, setTheme } = useTheme()
  const [notificationsOpen, setNotificationsOpen] = useState(false)
  const notificationRef = useRef<HTMLDivElement>(null)
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const unreadCount = notifications.filter((item) => !item.read).length
  const hasRuntime = Boolean((window as Window & { runtime?: unknown }).runtime)
  const path = location.pathname
  const isEditRoute = path.startsWith('/browser/edit') || path.startsWith('/browser/edit-fish')
  const isDetailRoute = path.startsWith('/browser/detail/')
  const isCopyRoute = path.startsWith('/browser/copy/')
  const isProxyRoute = path === '/browser/proxy-pool'
  const isCoreRoute = path === '/browser/cores'
  const isTagsRoute = path === '/browser/tags'
  const isExtensionsRoute = path === '/browser/extensions'
  const isBookmarksRoute = path === '/browser/bookmarks'
  const isAutomationRoute = path === '/browser/automation' || path.startsWith('/browser/automation/')
  const isLogsRoute = path === '/browser/logs'
  const isDocsRoute = path === '/system/docs'
  const isSettingsRoute = path === '/settings'
  const isProfileRoute = path === '/profile'
  const isChartsRoute = path === '/charts'
  const pageTitle = isEditRoute ? (path.endsWith('/new') ? '新建环境' : '编辑环境') : isDetailRoute ? '实例详情' : isCopyRoute ? '复制环境' : isProxyRoute ? '代理池' : isCoreRoute ? '内核与指纹' : isTagsRoute ? '分组与标签' : isExtensionsRoute ? '扩展' : isBookmarksRoute ? '默认书签' : isAutomationRoute ? (path === '/browser/automation' ? '自动化任务' : '自动化脚本') : isLogsRoute ? '运行日志' : isDocsRoute ? '文档' : isSettingsRoute ? '设置' : isProfileRoute ? '项目信息' : isChartsRoute ? '数据可视化' : '浏览器环境'
  const PageIcon = isCoreRoute ? Fingerprint : isProxyRoute ? Route : isTagsRoute ? Folders : isExtensionsRoute ? Puzzle : isBookmarksRoute ? Bookmark : isAutomationRoute ? Workflow : isLogsRoute ? ScrollText : isDocsRoute ? BookOpenText : isSettingsRoute ? Settings2 : isProfileRoute ? Briefcase : Layers3
  const showCreateEnvironment = path.startsWith('/browser/') && !isEditRoute

  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (notificationRef.current && !notificationRef.current.contains(event.target as Node)) setNotificationsOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [])

  return (
    <div className="ui-v2 fish-ui">
      <div className="fish-app">
        <aside className="fish-sidebar">
          <div className="fish-brand">
            <span className="fish-brand-mark"><ScanFace size={18} strokeWidth={1.8} /></span>
            <span>{projectConfig.name}</span>
          </div>

          <button className="fish-workspace" type="button">
            <Briefcase size={18} strokeWidth={1.8} />
            <span>个人工作区</span>
            <ChevronsUpDown className="fish-chev" size={18} strokeWidth={1.8} />
          </button>

          <nav className="fish-nav-scroll">
            {navGroups.map((group, groupIndex) => (
              <section className="fish-nav-group" key={group.label || `main-${groupIndex}`}>
                {group.label ? <div className="fish-nav-label">{group.label}</div> : null}
                {group.items.map((item) => {
                  const Icon = item.icon
                  const active = location.pathname === item.path || (item.path === '/browser/list' && (isEditRoute || isDetailRoute || isCopyRoute)) || (item.path !== '/' && location.pathname.startsWith(`${item.path}/`))
                  return (
                    <Link className={`fish-nav-btn${active ? ' active' : ''}`} key={item.path} to={item.path}>
                      <Icon size={18} strokeWidth={1.8} />
                      <span className="fish-nav-text">{item.label}</span>
                      {item.count ? <span className="fish-nav-count">{item.count}</span> : null}
                    </Link>
                  )
                })}
              </section>
            ))}
          </nav>

          <div className="fish-sidebar-foot">
            <div className="fish-mini-health">
              <div className="fish-health-row"><span>浏览器核心</span><span className={hasRuntime ? 'fish-online' : 'fish-idle'} /></div>
              <div className="fish-health-sub">{hasRuntime ? '桌面运行时已连接 · 状态实时同步' : '前端预览模式 · 使用本地模拟数据'}</div>
            </div>
            <button className="fish-btn" type="button" onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>
              {theme === 'dark' ? <Sun size={16} strokeWidth={1.8} /> : <Moon size={16} strokeWidth={1.8} />}
              切换主题
            </button>
          </div>
        </aside>

        <main className="fish-main">
          <header className="fish-topbar">
            <div className="fish-page-crumb"><PageIcon size={18} strokeWidth={1.8} /><span>{pageTitle}</span></div>
            <div className="fish-top-actions">
              <button className="fish-circle-btn" type="button" title="命令" onClick={onOpenQuickLaunch}><Search size={16} strokeWidth={1.8} /></button>
              <div className="fish-notification-wrap" ref={notificationRef}>
                <button className="fish-circle-btn" type="button" title="通知" onClick={() => setNotificationsOpen((value) => !value)}><Bell size={16} strokeWidth={1.8} /></button>
                {unreadCount > 0 ? <span className="fish-notification-count">{unreadCount > 9 ? '9+' : unreadCount}</span> : null}
                {notificationsOpen ? (
                  <div className="fish-popover">
                    <div className="fish-popover-head">
                      <strong>异常与通知</strong><span className="fish-spacer" />
                      {unreadCount > 0 ? <button className="fish-mini-btn" type="button" onClick={markAllAsRead}>全部已读</button> : null}
                      {notifications.length > 0 ? <button className="fish-mini-icon" type="button" title="清空通知" onClick={clearNotifications}><ScrollText size={16} strokeWidth={1.8} /></button> : null}
                    </div>
                    <div className="fish-popover-list">
                      {notifications.length === 0 ? <div className="fish-empty compact"><strong>暂无通知</strong><span>新的异常会显示在这里。</span></div> : notifications.slice(0, 12).map((item) => (
                        <button className="fish-notification" type="button" key={item.id} onClick={() => markAsRead(item.id)}>
                          <Bell size={16} strokeWidth={1.8} /><span><strong>{item.title}</strong><small>{item.message}</small></span>
                        </button>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
              {showCreateEnvironment ? <Link className="fish-primary-btn" to="/browser/edit/new"><Plus size={16} strokeWidth={1.8} /><span>新建环境</span></Link> : null}
              <Link className="fish-avatar-top" to="/profile" title="项目信息" aria-label="项目信息" />
            </div>
          </header>
          {children}
        </main>
      </div>
    </div>
  )
}
