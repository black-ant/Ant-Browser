/**
 * 项目配置文件
 * 
 * 基于此脚手架创建新项目时，修改此文件即可完成定制
 */

// 项目基础信息
export const projectConfig = {
  name: 'Ant Browser',
  shortName: 'Ant',
  description: '面向多账号隔离、代理绑定和本地环境管理的桌面浏览器工具',
  primaryColor: 'primary',
}

// 导航菜单配置
export interface NavItem {
  name: string
  path: string
  icon: string
}

export interface NavSection {
  title: string
  items: NavItem[]
}

export const navigationConfig: NavSection[] = [
  {
    title: '主菜单',
    items: [
      { name: '控制台', path: '/', icon: 'LayoutDashboard' },
    ]
  },
  {
    title: '指纹浏览器',
    items: [
      { name: '窗口列表', path: '/browser/list', icon: 'Monitor' },
      { name: '账号管理', path: '/browser/accounts', icon: 'Key' },
      { name: '扩展管理', path: '/browser/extensions', icon: 'Layers' },
      { name: '开发工具', path: '/browser/devtools', icon: 'Code' },
      { name: '自动化接口（实验）', path: '/browser/automation', icon: 'Bot' },
      { name: '内核管理', path: '/browser/cores', icon: 'Cpu' },
      { name: '代理池配置', path: '/browser/proxy-pool', icon: 'Globe' },
      { name: '默认书签', path: '/browser/bookmarks', icon: 'Bookmark' },
      { name: '标签管理', path: '/browser/tags', icon: 'Tag' },
    ]
  },
  {
    title: '系统维护',
    items: [
      { name: '系统设置', path: '/settings', icon: 'Settings' },
      { name: '使用教程', path: '/system/tutorial', icon: 'BookOpen' },
      { name: '日志查看', path: '/browser/logs', icon: 'FileText' },
      { name: '接口文档', path: '/browser/launch-api', icon: 'BookOpen' },
    ]
  },
]

// 功能开关
export const featuresConfig = {
  dashboard: true,
  data: true,
  settings: true,
}

// UI 配置
export const uiConfig = {
  pagination: {
    defaultPageSize: 20,
    pageSizeOptions: [10, 20, 50, 100],
  },
  dateFormat: 'YYYY-MM-DD HH:mm:ss',
  locale: 'zh-CN',
}

export default {
  project: projectConfig,
  navigation: navigationConfig,
  features: featuresConfig,
  ui: uiConfig,
}
