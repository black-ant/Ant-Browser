import { projectConfig } from './projectBase.config'

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
    // 占位个人信息:请替换为你的真实信息。留空的字段(email/website/github/location)与
    // 空的 channels 都不会显示,因此“没有的链接不写”即留空即可。
    name: '你的名字',
    initial: 'Z',
    title: '你的头衔 / 角色',
    bio: '在这里写一句话个人简介。(编辑 frontend/src/config/profile.config.ts 即可修改本页全部内容)',
    location: '',
    joinDate: '2026',
    email: '',
    website: '',
    github: '',
    skills: ['Go', 'React', 'TypeScript'],
    channels: [],
  },
  project: {
    name: projectConfig.name,
    introBadge: projectConfig.name,
    introText: '是一款专业指纹浏览器:为每个环境生成独立、唯一、自洽的指纹,并与代理地理自动对齐,适合多账号与跨境业务。',
    techStack: ['Wails', 'Go', 'React', 'TypeScript'],
    description: 'ZwBrowser 聚焦浏览器环境隔离、指纹自洽、代理池与地理对齐、批量环境创建与快捷启动,帮助多账号运营与本地测试统一管理浏览器环境。',
    actions: [],
  },
}

export default profilePageConfig
