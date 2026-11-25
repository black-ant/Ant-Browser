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

export interface AppUpdateAsset {
  name: string
  url: string
  sha256: string
}

export interface AppUpdateInfo extends AppUpdateCheckResult {
  notesUrl: string
  force: boolean
  assets: Record<string, AppUpdateAsset>
  // 当前通道下高于当前版本的可选版本（按版本降序，最多 2 个）。
  versions?: string[]
}

export interface UpdateProgress {
  phase: string
  progress: number
  message: string
}

export interface UpdatePendingInfo {
  version: string
  installerPath: string
  pid: number
  createdAt: string
  // 安装包已丢失（被删除或移动）时为 true，此时只能提示用户重新下载。
  missing?: boolean
}

export interface AppDownloadedUpdate {
  version: string
  assetName: string
  installerPath: string
}
