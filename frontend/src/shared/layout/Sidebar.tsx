import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Activity,
  Archive,
  Bell,
  Bookmark,
  BookOpen,
  FileText,
  LayoutDashboard,
  ListChecks,
  Monitor,
  Settings,
  Database,
  ChevronLeft,
  ChevronRight,
  Layers,
  PieChart,
  Cpu,
  Globe,
  Bot,
  Puzzle,
  Tag,
  User,
  type LucideIcon,
} from "lucide-react";
import clsx from "clsx";
import { useLayoutStore } from "../../store/layoutStore";
import { projectConfig, navigationConfig } from "../../config";
import { useNotificationStore } from "../../store/notificationStore";
import { GetAppConfig } from "../../wailsjs/go/main/App";

// 导入应用logo
import logoImage from "../../resources/images/logo.png";

const iconMap: Record<string, LucideIcon> = {
  LayoutDashboard,
  Settings,
  Database,
  Layers,
  PieChart,
  Monitor,
  ListChecks,
  Activity,
  Archive,
  Bell,
  FileText,
  Cpu,
  Globe,
  Bot,
  Puzzle,
  Bookmark,
  BookOpen,
  Tag,
  User,
};

function getIcon(iconName: string): LucideIcon {
  return iconMap[iconName] || LayoutDashboard;
}

export function Sidebar() {
  const location = useLocation();
  const { sidebarCollapsed, toggleSidebar } = useLayoutStore();
  const [appVersion, setAppVersion] = useState("");
  const { notifications } = useNotificationStore();
  const unreadNotificationCount = notifications.filter((notification) => !notification.read).length;

  useEffect(() => {
    const app = (window as any).go?.main?.App;
    if (!app?.GetAppConfig) return;

    GetAppConfig()
      .then((config) => {
        const version = typeof config?.version === "string" ? config.version.trim() : "";
        setAppVersion(version);
      })
      .catch(() => {
        setAppVersion("");
      });
  }, []);

  const isItemActive = (path: string) =>
    location.pathname === path ||
    (path !== "/" && location.pathname.startsWith(`${path}/`));

  return (
    <aside
      className={clsx(
        "bg-[var(--color-bg-surface)] flex flex-col transition-all duration-300 border-r border-[var(--color-border-default)]",
        sidebarCollapsed ? "w-16" : "w-60",
      )}
    >
      {/* Logo */}
      <div
        className={clsx(
          "h-14 flex items-center border-b border-[var(--color-border-muted)]",
          sidebarCollapsed ? "justify-center px-2" : "px-5",
        )}
      >
        {!sidebarCollapsed ? (
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-full overflow-hidden flex-shrink-0 bg-[var(--color-accent)] flex items-center justify-center">
              <img
                src={logoImage}
                alt="应用Logo"
                className="w-full h-full object-cover"
                onError={(e) => {
                  // 图片加载失败时显示首字母
                  e.currentTarget.style.display = "none";
                  e.currentTarget.parentElement?.classList.add("fallback-logo");
                }}
              />
              <span className="text-xs font-bold text-[var(--color-text-inverse)] hidden fallback-content">
                {projectConfig.shortName.charAt(0)}
              </span>
            </div>
            <div className="min-w-0 flex items-baseline gap-2">
              <h2 className="truncate text-base font-semibold tracking-tight text-[var(--color-text-primary)]">
                {projectConfig.name}
              </h2>
              {appVersion && (
                <span className="shrink-0 text-[10px] font-medium text-[var(--color-text-muted)]">
                  v{appVersion}
                </span>
              )}
            </div>
          </div>
        ) : (
          <div className="w-8 h-8 rounded-full overflow-hidden bg-[var(--color-accent)] flex items-center justify-center">
            <img
              src={logoImage}
              alt="应用Logo"
              className="w-full h-full object-cover"
              onError={(e) => {
                // 图片加载失败时显示首字母
                e.currentTarget.style.display = "none";
                e.currentTarget.parentElement?.classList.add("fallback-logo");
              }}
            />
            <span className="text-xs font-bold text-[var(--color-text-inverse)] hidden fallback-content">
              {projectConfig.shortName.charAt(0)}
            </span>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 space-y-4 overflow-y-auto">
        {navigationConfig.map((section) => (
          <div key={section.title}>
            {!sidebarCollapsed && (
              <h3 className="px-3 mb-2 text-[10px] font-semibold text-[var(--color-text-muted)] uppercase tracking-widest">
                {section.title}
              </h3>
            )}
            <div className="space-y-1">
              {section.items.map((item) => {
                const Icon = getIcon(item.icon);
                const isActive = isItemActive(item.path);

                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={sidebarCollapsed ? item.name : undefined}
                    className={clsx(
                      "relative flex items-center rounded-lg transition-all duration-150",
                      isActive
                        ? "bg-[var(--color-accent)] text-[var(--color-text-inverse)] shadow-sm"
                        : "text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]",
                      sidebarCollapsed
                        ? "justify-center w-10 h-10 mx-auto"
                        : "px-3 py-2.5 gap-3",
                    )}
                  >
                    <Icon className="w-[18px] h-[18px] flex-shrink-0" />
                    {!sidebarCollapsed && (
                      <span className="min-w-0 truncate text-sm font-medium">
                        {item.name}
                      </span>
                    )}
                    {item.path === "/notifications" && unreadNotificationCount > 0 && (
                      <span
                        aria-label={`${unreadNotificationCount} 条未读通知`}
                        className={clsx(
                          "flex items-center justify-center rounded-full bg-[var(--color-error)] text-[10px] font-semibold text-white",
                          sidebarCollapsed
                            ? "absolute right-0 top-0 h-4 min-w-4 px-1"
                            : "ml-auto h-5 min-w-5 px-1.5",
                        )}
                      >
                        {unreadNotificationCount > 9 ? "9+" : unreadNotificationCount}
                      </span>
                    )}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="border-t border-[var(--color-border-muted)] p-3">
        <Link
          to="/profile"
          title={sidebarCollapsed ? "关于我" : undefined}
          className={clsx(
            "flex items-center rounded-lg transition-all duration-150",
            isItemActive("/profile")
              ? "bg-[var(--color-accent)] text-[var(--color-text-inverse)] shadow-sm"
              : "text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]",
            sidebarCollapsed
              ? "relative mx-auto h-10 w-10 justify-center"
              : "gap-3 px-3 py-2.5",
          )}
        >
          <User className="h-[18px] w-[18px] shrink-0" />
          {!sidebarCollapsed && <span className="truncate text-sm font-medium">关于我</span>}
        </Link>
      </div>

      {/* Toggle Button */}
      <div className="p-3 border-t border-[var(--color-border-muted)]">
        <button
          onClick={toggleSidebar}
          className={clsx(
            "flex items-center rounded-lg text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-secondary)] transition-all duration-150",
            sidebarCollapsed
              ? "justify-center w-10 h-10 mx-auto"
              : "w-full px-3 py-2 gap-3",
          )}
          title={sidebarCollapsed ? "展开" : "收起"}
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
  );
}
