export interface CoreDisplayInfo {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
  pathValid: boolean
  pathMessage: string
  chromeVersion: string
  instanceCount: number
}

export interface CoreSettingsForm {
  userDataRoot: string
  defaultFingerprintArgs: string
  defaultLaunchArgs: string
  defaultStartUrls: string
  lightStartEnabled: boolean
  restoreLastSession: boolean
  startReadyTimeoutMs: number
  startStableWindowMs: number
}

export interface CoreEditForm {
  coreName: string
  corePath: string
}

export interface CoreDownloadForm {
  coreId?: string
  name: string
  url: string
  proxyMode: string
  proxyId: string
  mode?: 'download' | 'redownload'
}

export interface CoreDownloadProgress {
  phase: string
  progress: number
  message: string
}
