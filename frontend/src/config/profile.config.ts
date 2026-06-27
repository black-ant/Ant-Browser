import { projectConfig } from './project.config'
import { PROJECT_GITHUB_URL } from './links'

export type ProfileIconKey =
  | 'book-open'
  | 'globe'
  | 'message-square'
  | 'github'
  | 'mail'
  | 'external-link'

export interface ProfileChannelConfig {
  name: string
  description: string
  detail: string
  href?: string
  icon?: ProfileIconKey
}

export interface AuthorProfileConfig {
  name: string
  initial: string
  title: string
  bio: string
  location: string
  joinDate: string
  email: string
  website: string
  github: string
  skills: string[]
  channels: ProfileChannelConfig[]
}

export interface ProjectProfileActionConfig {
  label: string
  href: string
  icon: ProfileIconKey
}

export interface ProjectProfileConfig {
  name: string
  introBadge: string
  introText: string
  techStack: string[]
  description: string
  actions: ProjectProfileActionConfig[]
}

export interface RemoteAuthorSourceConfig {
  authorURL: string
  timeoutMs: number
}

export interface ProfilePageLocalConfig {
  remoteAuthor: RemoteAuthorSourceConfig
  defaultAuthor: AuthorProfileConfig
  project: ProjectProfileConfig
}

export const profilePageConfig: ProfilePageLocalConfig = {
  remoteAuthor: {
    // 留空时直接使用本地默认资料；需要远程作者页时再替换为真实地址。
    // https://static.antblack.de/profile/author.json
    // https://raw.githubusercontent.com/<user>/<repo>/main/author.json
    authorURL: '',
    timeoutMs: 1000,
  },
  defaultAuthor: {
    name: '志字辈小蚂蚁',
    initial: '志',
    title: '全栈开发工程师',
    bio: '热爱开源，专注于 Web 和桌面应用开发。致力于打造优雅、高效的开发工具和框架。',
    location: '中国',
    joinDate: '2020',
    email: 'contact@antblack.dev',
    website: 'http://blog.antblack.de',
    github: 'https://github.com/black-ant',
    skills: ['Go', 'React', 'TypeScript', 'Wails', 'Node.js', 'Docker'],
    channels: [
      {
        name: '掘金',
        description: '技术文章与开发记录',
        detail: 'juejin.cn',
        href: 'https://juejin.cn/user/3790771822007822',
        icon: 'book-open',
      },
      {
        name: '个人博客',
        description: '独立站文章与项目归档',
        detail: 'blog.antblack.de',
        href: 'http://blog.antblack.de',
        icon: 'globe',
      },
      {
        name: '公众号',
        description: '微信搜索后即可关注',
        detail: '整理中',
        icon: 'message-square',
      },
    ],
  },
  project: {
    name: projectConfig.name,
    introBadge: projectConfig.name,
    introText: '是一个面向多账号隔离、代理绑定和本地环境管理的桌面浏览器工具。',
    techStack: ['Wails', 'React', 'TypeScript'],
    description: '项目当前聚焦浏览器窗口隔离、代理池配置、浏览器内核管理、标签检索和快捷启动等核心能力，适合跨境电商、社媒运营、本地测试以及需要统一管理浏览器环境的团队场景。',
    actions: [
      {
        label: '查看源码',
        href: PROJECT_GITHUB_URL,
        icon: 'github',
      },
      {
        label: '下载发布版',
        href: `${PROJECT_GITHUB_URL}/releases`,
        icon: 'globe',
      },
    ],
  },
}

export default profilePageConfig
