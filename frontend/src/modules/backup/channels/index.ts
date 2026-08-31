import { Cloud, Database, HardDrive } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export interface BackupChannelDefinition {
  id: string
  label: string
  description: string
  available: boolean
  configurable: boolean
  icon: LucideIcon
}

export const backupChannelDefinitions = [
  {
    id: 'local',
    label: '本地备份',
    description: '保存备份文件到本机',
    available: true,
    configurable: false,
    icon: HardDrive,
  },
  {
    id: 'openlist',
    label: 'OpenList 备份',
    description: '上传备份文件到 OpenList',
    available: true,
    configurable: true,
    icon: Cloud,
  },
  {
    id: 's3',
    label: 'S3 备份',
    description: '上传备份文件到 S3',
    available: true,
    configurable: true,
    icon: Database,
  },
] as const satisfies readonly BackupChannelDefinition[]

export type BackupChannelId = typeof backupChannelDefinitions[number]['id']

export type BackupChannelSelection = Partial<Record<BackupChannelId, boolean>>

export interface BackupChannelStatus {
  configured: boolean
  summary?: string
}
