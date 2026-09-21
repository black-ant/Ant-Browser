import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Bell,
  BookOpenText,
  Bookmark,
  Boxes,
  Briefcase,
  ChevronsUpDown,
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
  Tags,
  UserCircle2,
  Workflow,
  type LucideIcon,
} from 'lucide-react'
import { useTheme } from '../shared/theme'
import { useNotificationStore } from '../store/notificationStore'
import { projectConfig } from '../config'
import './styles.css'

type NavItem = { label: string; path: string; icon: LucideIcon }
type NavSection = { label: string; items: NavItem[] }

const navSections: NavSection[] = [
  {
    label: '浏览器',
    items: [
      { label: '实例列表', path: '/browser/list', icon: Layers3 },
      { label: '自动化脚本', path: '/browser/automation', icon: Workflow },
      { label: '内核管理', path: '/browser/cores', icon: Boxes },
      { label: '插件包管理', path: '/browser/extensions', icon: Puzzle },
    ],
  },
  {
    label: '网络与数据',
    items: [
      { label: '代理池配置', path: '/browser/proxy-pool', icon: Route },
      { label: '默认书签', path: '/browser/bookmarks', icon: Bookmark },
      { label: '标签管理', path: '/browser/tags', icon: Tags },
    ],
  },
  {
    label: '系统',
    items: [
      { label: '系统设置', path: '/settings', icon: Settings2 },
      { label: '文档中心', path: '/system/docs', icon: BookOpenText },
      { label: '日志查看', path: '/browser/logs', icon: ScrollText },
    ],
  },
]

interface UiV2ShellProps {
  children: ReactNode
  onOpenQuickLaunch: () => void
}

export function UiV2Shell({ children, onOpenQuickLaunch }: UiV2ShellProps) {
  const location = useLocation()
  const { theme, setTheme } = useTheme()
  const [notificationsOpen, setNotificationsOpen] = useState(false)
  const notificationsRef = useRef<HTMLDivElement>(null)
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const unreadCount = notifications.filter((item) => !item.read).length

  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (notificationsRef.current && !notificationsRef.current.contains(event.target as Node)) {
        setNotificationsOpen(false)
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [])

  const toggleTheme = () => setTheme(theme === 'dark' ? 'light' : 'dark')

  return (
    <div className="ui-v2">
      <div className="ui-v2-shell">
        <aside className="ui-v2-sidebar">
          <div className="ui-v2-brand">
            <span className="ui-v2-brand-mark"><ScanFace size={18} strokeWidth={1.8} /></span>
            <span>{projectConfig.name}</span>
          </div>

          <button className="ui-v2-workspace" type="button">
            <Briefcase size={18} strokeWidth={1.8} />
            <span>本地工作区</span>
            <ChevronsUpDown className="chev" size={18} strokeWidth={1.8} />
          </button>

          <nav className="ui-v2-nav-scroll">
            {navSections.map((section) => (
              <section className="ui-v2-nav-group" key={section.label}>
                <div className="ui-v2-nav-label">{section.label}</div>
                {section.items.map((item) => {
                  const Icon = item.icon
                  const active = location.pathname === item.path || location.pathname.startsWith(`${item.path}/`)
                  return (
                    <Link className={`ui-v2-nav-btn${active ? ' active' : ''}`} key={item.path} to={item.path}>
                      <Icon size={18} strokeWidth={1.8} />
                      <span className="ui-v2-nav-text">{item.label}</span>
                    </Link>
                  )
                })}
              </section>
            ))}
          </nav>

          <div className="ui-v2-sidebar-foot">
            <div className="ui-v2-mini-health">
              <strong>实例列表 · UI V2</strong>
              <span>实例列表已正式接管；其他页面仍沿用原有界面。</span>
            </div>
            <Link className="ui-v2-mini-btn" to="/browser/list-legacy">
              <Layers3 size={16} strokeWidth={1.8} />
              打开旧版实例列表
            </Link>
            <button className="ui-v2-btn" type="button" onClick={toggleTheme}>
              {theme === 'dark' ? <Sun size={16} strokeWidth={1.8} /> : <Moon size={16} strokeWidth={1.8} />}
              {theme === 'dark' ? '切换白色主题' : '切换暗色主题'}
            </button>
          </div>
        </aside>

        <main className="ui-v2-main">
          <header className="ui-v2-topbar">
            <div className="ui-v2-page-crumb">
              <Layers3 size={18} strokeWidth={1.8} />
              <span>实例列表</span>
            </div>
            <div className="ui-v2-top-actions">
              <button className="ui-v2-circle-btn" type="button" title="快速启动" onClick={onOpenQuickLaunch}>
                <Search size={16} strokeWidth={1.8} />
              </button>
              <div className="ui-v2-notification-trigger" ref={notificationsRef}>
                <button className="ui-v2-circle-btn" type="button" title="通知" onClick={() => setNotificationsOpen((value) => !value)}>
                  <Bell size={16} strokeWidth={1.8} />
                </button>
                {unreadCount > 0 ? <span className="ui-v2-notification-count">{unreadCount > 99 ? '99+' : unreadCount}</span> : null}
                {notificationsOpen ? (
                  <div className="ui-v2-popover">
                    <div className="ui-v2-popover-head">
                      <strong>异常与通知</strong>
                      <span className="ui-v2-spacer" />
                      {unreadCount > 0 ? <button className="ui-v2-mini-btn" type="button" onClick={markAllAsRead}>全部已读</button> : null}
                      {notifications.length > 0 ? <button className="ui-v2-mini-icon" type="button" title="清空通知" onClick={clearNotifications}><ScrollText size={16} strokeWidth={1.8} /></button> : null}
                    </div>
                    <div className="ui-v2-popover-list">
                      {notifications.length === 0 ? (
                        <div className="ui-v2-empty"><strong>暂无通知</strong><span>新的异常与状态提醒会显示在这里。</span></div>
                      ) : notifications.slice(0, 20).map((item) => (
                        <div className="ui-v2-notification" key={item.id} onClick={() => markAsRead(item.id)}>
                          <Bell size={16} strokeWidth={1.8} />
                          <div>
                            <strong>{item.title}</strong>
                            <p>{item.message}</p>
                            <time>{item.time}</time>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
              <Link className="ui-v2-primary-btn" to="/browser/edit/new">
                <Plus size={16} strokeWidth={1.8} />
                新建配置
              </Link>
              <Link className="ui-v2-circle-btn" to="/profile" title="项目信息">
                <UserCircle2 size={16} strokeWidth={1.8} />
              </Link>
            </div>
          </header>
          {children}
        </main>
      </div>
    </div>
  )
}
