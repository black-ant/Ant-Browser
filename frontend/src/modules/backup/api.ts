import type { BackupChannelSelection } from './channels'

export interface BackupActionResult {
  cancelled?: boolean
  message?: string
  zipPath?: string
  imported?: number
  skipped?: number
  conflicts?: number
  includedEntries?: number
  skippedEntries?: number
  fileCount?: number
  partial?: boolean
  componentTotal?: number
  componentSuccess?: number
  componentFailed?: number
  localSaved?: boolean
  remoteUploaded?: boolean
  remoteName?: string
  remoteSize?: number
  remoteError?: string
  failedComponents?: Array<{
    componentId?: string
    componentName?: string
    error?: string
  }>
}

export interface BackupFileInfo {
  size: number
  modifiedAt: string
}

export type BackupDestinationSelection = BackupChannelSelection

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

export async function createBackupPackage(
  destinations: BackupDestinationSelection,
): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupCreatePackage) {
    throw new Error('当前环境不支持统一备份接口')
  }
  const payload: Record<string, string> = {}
  for (const [channelId, enabled] of Object.entries(destinations)) {
    if (typeof enabled === 'boolean') {
      payload[channelId] = String(enabled)
    }
  }
  return (await bindings.BackupCreatePackage(payload)) || {}
}

export async function exportSystemConfig(): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupExportPackage) {
    return { cancelled: false, message: '当前环境不支持后端导出接口' }
  }
  return (await bindings.BackupExportPackage()) || {}
}

export async function importSystemConfig(): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupImportPackage) {
    return { cancelled: false, message: '当前环境不支持后端导入接口' }
  }
  return (await bindings.BackupImportPackage()) || {}
}

export async function restoreLocalSystemConfig(zipPath: string): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupRestoreLocalPackage) {
    throw new Error('当前环境不支持本地备份恢复接口')
  }
  return (await bindings.BackupRestoreLocalPackage(zipPath.trim())) || {}
}

export async function openBackupPath(zipPath: string): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.OpenBackupPath) {
    throw new Error('当前环境不支持打开本地备份路径')
  }
  await bindings.OpenBackupPath(zipPath.trim())
}

export async function getBackupFileInfo(zipPath: string): Promise<BackupFileInfo> {
  const bindings: any = await getBindings()
  if (!bindings?.GetBackupFileInfo) {
    throw new Error('当前环境不支持读取本地备份文件信息')
  }
  const raw = (await bindings.GetBackupFileInfo(zipPath.trim())) || {}
  return {
    size: Number.isFinite(raw.size) ? Math.max(0, Number(raw.size)) : 0,
    modifiedAt: typeof raw.modifiedAt === 'string' ? raw.modifiedAt : '',
  }
}
