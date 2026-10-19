import type {
  AuthorProfileConfig,
  ProfileChannelConfig,
  ProfileIconKey,
  ProjectProfileActionConfig,
  ProjectProfileConfig,
} from '../../config/profile.config'

export type ProfileChannel = ProfileChannelConfig
export type AuthorProfile = AuthorProfileConfig
export type ProfileAction = ProjectProfileActionConfig
export type ProfileProject = ProjectProfileConfig
export type IconKey = ProfileIconKey

export interface ProfileLoadMeta {
  source: 'remote' | 'default'
  message?: string
}

export interface ProfilePageData {
  author: AuthorProfile
  project: ProfileProject
  meta: ProfileLoadMeta
}

export interface AppUpdateCheckResult {
  ok: boolean
  currentVersion: string
  latestVersion: string
  updateAvailable: boolean
  releaseUrl: string
  error: string
}
