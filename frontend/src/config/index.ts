// 配置模块入口
export { default as config } from './project.config'
export {
  projectConfig,
  featuresConfig,
  uiConfig,
} from './project.config'
export { navigationConfig } from './navigation.config'
export { profilePageConfig } from './profile.config'

export type { NavItem, NavSection } from './navigation.config'
export type {
  AuthorProfileConfig,
  ProfileChannelConfig,
  ProfileIconKey,
  ProjectProfileActionConfig,
  ProjectProfileConfig,
  RemoteAuthorSourceConfig,
  ProfilePageLocalConfig,
} from './profile.config'
