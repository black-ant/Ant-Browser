export interface BackupExportProgress {
  phase: string
  progress: number
  message: string
  bytesTransferred?: number
  totalBytes?: number
  bytesPerSecond?: number
  componentId?: string
  componentName?: string
  entryIndex?: number
  entryTotal?: number
  timestamp?: string
}

export interface BackupExportLogItem {
  id: number
  phase: string
  time: string
  text: string
}

export interface AutomationRuntimeProgress {
  phase: string
  progress: number
  message: string
  component?: string
}
