import { normalizeBackupPackageInfo, type BackupPackageInfo } from '../../packageInfo'

export interface OpenListConnection {
  baseURL: string
  remotePath: string
  token?: string
}

export interface OpenListDraft extends OpenListConnection {
  token: string
  uploadRateLimitMBps: string
}

export interface OpenListSettings extends OpenListConnection {
  tokenConfigured: boolean
  uploadRateLimitMBps: number
}

export interface OpenListBackupFile extends BackupPackageInfo {
  name: string
  size: number
  modifiedAt: string
}

export const defaultOpenListSettings: OpenListSettings = {
  baseURL: '',
  remotePath: 'ant-chrome/backups',
  tokenConfigured: false,
  uploadRateLimitMBps: 0,
}

const getBindings = async () => {
  try {
    return await import('../../../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

function toPayload(value?: Partial<OpenListDraft>): Record<string, string> {
  const payload: Record<string, string> = {
    baseURL: value?.baseURL?.trim() || '',
    remotePath: value?.remotePath?.trim() || '',
  }
  if (typeof value?.token === 'string') {
    payload.token = value.token.trim()
  }
  if (typeof value?.uploadRateLimitMBps === 'string') {
    payload.uploadRateLimitMBps = value.uploadRateLimitMBps.trim()
  }
  return payload
}

function normalizeOpenListSettings(raw: any): OpenListSettings {
  const rateLimit = Number(raw?.uploadRateLimitMBps)
  return {
    ...defaultOpenListSettings,
    baseURL: typeof raw?.baseURL === 'string' ? raw.baseURL : '',
    remotePath: typeof raw?.remotePath === 'string' && raw.remotePath ? raw.remotePath : defaultOpenListSettings.remotePath,
    tokenConfigured: raw?.tokenConfigured === true,
    uploadRateLimitMBps: Number.isInteger(rateLimit) && rateLimit >= 0 ? rateLimit : defaultOpenListSettings.uploadRateLimitMBps,
  }
}

export async function fetchOpenListSettings(): Promise<OpenListSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListGetSettings) {
    return defaultOpenListSettings
  }
  return normalizeOpenListSettings(await bindings.BackupOpenListGetSettings())
}

export async function revealOpenListToken(): Promise<string> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListRevealToken) {
    throw new Error('当前环境不支持显示 OpenList Token')
  }
  return String((await bindings.BackupOpenListRevealToken()) || '')
}

export async function saveOpenListSettings(draft: OpenListDraft): Promise<OpenListSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListSaveSettings) {
    throw new Error('当前环境不支持 OpenList 配置保存')
  }
  return normalizeOpenListSettings(await bindings.BackupOpenListSaveSettings(toPayload(draft)))
}

export async function testOpenListConnection(draft: OpenListDraft): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListTest) {
    throw new Error('当前环境不支持 OpenList 连接测试')
  }
  await bindings.BackupOpenListTest(toPayload(draft))
}

export async function listOpenListBackups(connection?: OpenListConnection): Promise<OpenListBackupFile[]> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListList) {
    throw new Error('当前环境不支持 OpenList 备份列表')
  }
  const raw = await bindings.BackupOpenListList(toPayload(connection))
  if (!Array.isArray(raw)) {
    return []
  }
  return raw
    .map(item => {
      const name = typeof item?.name === 'string' ? item.name : ''
      return {
        name,
        size: Number.isFinite(item?.size) ? Math.max(0, Number(item.size)) : 0,
        modifiedAt: typeof item?.modifiedAt === 'string' ? item.modifiedAt : '',
        ...normalizeBackupPackageInfo(item, name),
      }
    })
    .filter(item => item.name)
}

export async function uploadOpenListBackup(connection?: OpenListConnection): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListUpload) {
    throw new Error('当前环境不支持 OpenList 备份上传')
  }
  return (await bindings.BackupOpenListUpload(toPayload(connection))) || {}
}

export async function restoreOpenListBackup(
  fileName: string,
  connection?: OpenListConnection,
): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListRestore) {
    throw new Error('当前环境不支持 OpenList 备份恢复')
  }
  return (await bindings.BackupOpenListRestore(toPayload(connection), fileName)) || {}
}

export async function downloadOpenListBackup(
  fileName: string,
  connection?: OpenListConnection,
): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListDownload) {
    throw new Error('当前环境不支持 OpenList 备份下载')
  }
  return (await bindings.BackupOpenListDownload(toPayload(connection), fileName)) || {}
}
