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
    title: '指纹浏览器',
    items: [
      { name: '实例列表', path: '/browser/list', icon: 'Monitor' },
      { name: '自动化脚本', path: '/browser/automation', icon: 'Bot' },
      { name: '内核管理', path: '/browser/cores', icon: 'Cpu' },
      { name: '插件包管理', path: '/browser/extensions', icon: 'Puzzle' },
      { name: '代理池配置', path: '/browser/proxy-pool', icon: 'Globe' },
      { name: '默认书签', path: '/browser/bookmarks', icon: 'Bookmark' },
      { name: '标签管理', path: '/browser/tags', icon: 'Tag' },
    ]
  },
  {
    title: '系统维护',
    items: [
      { name: '系统设置', path: '/settings', icon: 'Settings' },
      { name: '备份与恢复', path: '/system/backup', icon: 'Archive' },
      { name: '文档中心', path: '/system/docs', icon: 'BookOpen' },
      { name: '日志查看', path: '/browser/logs', icon: 'FileText' },
    ]
  },
]
