import type { OpenListConnection } from './channels/openlist/api'
import type { S3Connection } from './channels/s3/api'
import type { BackupTypeSelection } from './components/BackupTypeModal'

export interface BackupResumeState {
  pendingBackupTypes: BackupTypeSelection
  pendingProfileIds: string[]
  openListConnection?: OpenListConnection | null
  s3Connection?: S3Connection | null
}

export interface BackupRouteState {
  backupResume?: BackupResumeState
}

export function createBackupRouteState(
  pendingBackupTypes: BackupTypeSelection,
  pendingProfileIds: string[],
  openListConnection?: OpenListConnection | null,
  s3Connection?: S3Connection | null,
): BackupRouteState {
  return {
    backupResume: {
      pendingBackupTypes,
      pendingProfileIds: [...pendingProfileIds],
      openListConnection,
      s3Connection,
    },
  }
}
