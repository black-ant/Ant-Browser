import { Link, useLocation } from 'react-router-dom'
import {
  Activity,
  Bot,
  Bookmark,
  BookOpen,
  FileText,
  LayoutDashboard,
  ListChecks,
  Monitor,
  Settings,
  Store,
  Database,
  ChevronLeft,
  ChevronRight,
  Layers,
  PieChart,
  Cpu,
  Globe,
  Tag,
  type LucideIcon
} from 'lucide-react'
import clsx from 'clsx'
import { useLayoutStore } from '../../store/layoutStore'
import { projectConfig, navigationConfig } from '../../config'

// 导入应用logo
import logoImage from '../../resources/images/logo.png'

const iconMap: Record<string, LucideIcon> = {
  LayoutDashboard,
  Settings,
  Database,
  Layers,
  PieChart,
  Monitor,
  Store,
  Bot,
  ListChecks,
  Activity,
  FileText,
  Cpu,
  Globe,
  Bookmark,
  BookOpen,
  Tag,
}

function getIcon(iconName: string): LucideIcon {
  return iconMap[iconName] || LayoutDashboard
}

export function Sidebar() {
  const location = useLocation()
  const { sidebarCollapsed, toggleSidebar } = useLayoutStore()

  return (
    <aside className={clsx(
      'bg-[var(--color-bg-surface)] flex flex-col transition-all duration-300 border-r border-[var(--color-border-default)]',
      sidebarCollapsed ? 'w-16' : 'w-60'
    )}>
      {/* Logo */}
      <div className={clsx(
        'h-14 flex items-center border-b border-[var(--color-border-muted)]',
        sidebarCollapsed ? 'justify-center px-2' : 'px-5'
      )}>
        {!sidebarCollapsed ? (
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-full overflow-hidden flex-shrink-0 bg-[var(--color-accent)] flex items-center justify-center">
              <img 
                src={logoImage} 
                alt="应用Logo" 
                className="w-full h-full object-cover"
                onError={(e) => {
                  // 图片加载失败时显示首字母
                  e.currentTarget.style.display = 'none';
                  e.currentTarget.parentElement?.classList.add('fallback-logo');
                }}
              />
              <span className="text-xs font-bold text-[var(--color-text-inverse)] hidden fallback-content">
                {projectConfig.shortName.charAt(0)}
              </span>
            </div>
            <h2 className="text-base font-semibold text-[var(--color-text-primary)] tracking-tight truncate">
              {projectConfig.name}
            </h2>
          </div>
        ) : (
          <div className="w-8 h-8 rounded-full overflow-hidden bg-[var(--color-accent)] flex items-center justify-center">
            <img 
              src={logoImage} 
              alt="应用Logo" 
              className="w-full h-full object-cover"
              onError={(e) => {
                // 图片加载失败时显示首字母
                e.currentTarget.style.display = 'none';
                e.currentTarget.parentElement?.classList.add('fallback-logo');
              }}
            />
            <span className="text-xs font-bold text-[var(--color-text-inverse)] hidden fallback-content">
              {projectConfig.shortName.charAt(0)}
            </span>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 space-y-6 overflow-y-auto">
        {navigationConfig.map((section) => (
          <div key={section.title}>
            {!sidebarCollapsed && (
              <h3 className="px-3 mb-2 text-[10px] font-semibold text-[var(--color-text-muted)] uppercase tracking-widest">
                {section.title}
              </h3>
            )}
            <div className="space-y-1">
              {section.items.map((item) => {
                const Icon = getIcon(item.icon)
                const isActive = item.path === '/'
                  ? location.pathname === '/'
                  : location.pathname === item.path || location.pathname.startsWith(`${item.path}/`)
                
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={sidebarCollapsed ? item.name : undefined}
                    className={clsx(
                      'flex items-center rounded-lg transition-all duration-150',
                      isActive
                        ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)] shadow-sm'
                        : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                      sidebarCollapsed 
                        ? 'justify-center w-10 h-10 mx-auto' 
                        : 'px-3 py-2.5 gap-3'
                    )}
                  >
                    <Icon className="w-[18px] h-[18px] flex-shrink-0" />
                    {!sidebarCollapsed && (
                      <span className="text-sm font-medium truncate">{item.name}</span>
                    )}
                  </Link>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* Toggle Button */}
      <div className="p-3 border-t border-[var(--color-border-muted)]">
        <button
          onClick={toggleSidebar}
          className={clsx(
            'flex items-center rounded-lg text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-secondary)] transition-all duration-150',
            sidebarCollapsed 
              ? 'justify-center w-10 h-10 mx-auto' 
              : 'w-full px-3 py-2 gap-3'
          )}
          title={sidebarCollapsed ? '展开' : '收起'}
        >
          {sidebarCollapsed ? (
            <ChevronRight className="w-[18px] h-[18px]" />
          ) : (
            <>
              <ChevronLeft className="w-[18px] h-[18px]" />
              <span className="text-sm">收起侧边栏</span>
            </>
          )}
        </button>
      </div>
    </aside>
  )
}
