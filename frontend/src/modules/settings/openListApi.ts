export interface OpenListConnection {
  baseURL: string
  remotePath: string
  username: string
  password: string
}

export interface OpenListBackupFile {
  name: string
  size: number
  modifiedAt: string
}

export interface ScheduledBackupSettings {
  enabled: boolean
  dailyTime: string
  baseURL: string
  remotePath: string
  username: string
  passwordConfigured: boolean
  status: 'never' | 'running' | 'success' | 'skipped' | 'failed'
  lastRunAt: string
  lastSuccessAt: string
  lastError: string
  lastRemoteName: string
}

export interface ScheduledBackupDraft {
  enabled: boolean
  dailyTime: string
  baseURL: string
  remotePath: string
  username: string
  password: string
}

export const defaultScheduledBackupSettings: ScheduledBackupSettings = {
  enabled: false,
  dailyTime: '02:00',
  baseURL: '',
  remotePath: 'ant-chrome/backups',
  username: '',
  passwordConfigured: false,
  status: 'never',
  lastRunAt: '',
  lastSuccessAt: '',
  lastError: '',
  lastRemoteName: '',
}

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

function toPayload(connection: OpenListConnection): Record<string, string> {
  return {
    baseURL: connection.baseURL.trim(),
    remotePath: connection.remotePath.trim(),
    username: connection.username.trim(),
    password: connection.password,
  }
}

export async function testOpenListConnection(connection: OpenListConnection): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListTest) {
    throw new Error('当前环境不支持 OpenList 连接测试')
  }
  await bindings.BackupOpenListTest(toPayload(connection))
}

export async function listOpenListBackups(connection: OpenListConnection): Promise<OpenListBackupFile[]> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListList) {
    throw new Error('当前环境不支持 OpenList 备份列表')
  }
  const raw = await bindings.BackupOpenListList(toPayload(connection))
  if (!Array.isArray(raw)) {
    return []
  }
  return raw
    .map(item => ({
      name: typeof item?.name === 'string' ? item.name : '',
      size: Number.isFinite(item?.size) ? Math.max(0, Number(item.size)) : 0,
      modifiedAt: typeof item?.modifiedAt === 'string' ? item.modifiedAt : '',
    }))
    .filter(item => item.name)
}

export async function uploadOpenListBackup(connection: OpenListConnection): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListUpload) {
    throw new Error('当前环境不支持 OpenList 备份上传')
  }
  return (await bindings.BackupOpenListUpload(toPayload(connection))) || {}
}

export async function restoreOpenListBackup(
  connection: OpenListConnection,
  fileName: string,
  resetFirst: boolean,
): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupOpenListRestore) {
    throw new Error('当前环境不支持 OpenList 备份恢复')
  }
  return (await bindings.BackupOpenListRestore(toPayload(connection), fileName, resetFirst)) || {}
}

export async function fetchScheduledBackupSettings(): Promise<ScheduledBackupSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupScheduledGetSettings) {
    return defaultScheduledBackupSettings
  }
  const raw = (await bindings.BackupScheduledGetSettings()) || {}
  const status = typeof raw.status === 'string' ? raw.status : defaultScheduledBackupSettings.status
  return {
    ...defaultScheduledBackupSettings,
    ...raw,
    enabled: raw.enabled === true,
    dailyTime: typeof raw.dailyTime === 'string' && raw.dailyTime ? raw.dailyTime : defaultScheduledBackupSettings.dailyTime,
    baseURL: typeof raw.baseURL === 'string' ? raw.baseURL : '',
    remotePath: typeof raw.remotePath === 'string' && raw.remotePath ? raw.remotePath : defaultScheduledBackupSettings.remotePath,
    username: typeof raw.username === 'string' ? raw.username : '',
    passwordConfigured: raw.passwordConfigured === true,
    status: ['never', 'running', 'success', 'skipped', 'failed'].includes(status) ? status as ScheduledBackupSettings['status'] : 'never',
    lastRunAt: typeof raw.lastRunAt === 'string' ? raw.lastRunAt : '',
    lastSuccessAt: typeof raw.lastSuccessAt === 'string' ? raw.lastSuccessAt : '',
    lastError: typeof raw.lastError === 'string' ? raw.lastError : '',
    lastRemoteName: typeof raw.lastRemoteName === 'string' ? raw.lastRemoteName : '',
  }
}

export async function saveScheduledBackupSettings(draft: ScheduledBackupDraft): Promise<ScheduledBackupSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupScheduledSaveSettings) {
    throw new Error('当前环境不支持定时备份设置')
  }
  const raw = (await bindings.BackupScheduledSaveSettings({
    enabled: String(draft.enabled),
    dailyTime: draft.dailyTime.trim(),
    baseURL: draft.baseURL.trim(),
    remotePath: draft.remotePath.trim(),
    username: draft.username.trim(),
    password: draft.password,
  })) || {}
  return {
    ...defaultScheduledBackupSettings,
    ...raw,
    enabled: raw.enabled === true,
    dailyTime: typeof raw.dailyTime === 'string' && raw.dailyTime ? raw.dailyTime : defaultScheduledBackupSettings.dailyTime,
    baseURL: typeof raw.baseURL === 'string' ? raw.baseURL : '',
    remotePath: typeof raw.remotePath === 'string' && raw.remotePath ? raw.remotePath : defaultScheduledBackupSettings.remotePath,
    username: typeof raw.username === 'string' ? raw.username : '',
    passwordConfigured: raw.passwordConfigured === true,
    status: ['never', 'running', 'success', 'skipped', 'failed'].includes(raw.status) ? raw.status : 'never',
    lastRunAt: typeof raw.lastRunAt === 'string' ? raw.lastRunAt : '',
    lastSuccessAt: typeof raw.lastSuccessAt === 'string' ? raw.lastSuccessAt : '',
    lastError: typeof raw.lastError === 'string' ? raw.lastError : '',
    lastRemoteName: typeof raw.lastRemoteName === 'string' ? raw.lastRemoteName : '',
  }
}
