export interface ScheduledBackupSettings {
  enabled: boolean
  dailyTime: string
  tokenConfigured: boolean
  status: 'never' | 'running' | 'success' | 'skipped' | 'failed'
  lastRunAt: string
  lastSuccessAt: string
  lastError: string
  lastRemoteName: string
}

export interface ScheduledBackupDraft {
  enabled: boolean
  dailyTime: string
}

export const defaultScheduledBackupSettings: ScheduledBackupSettings = {
  enabled: false,
  dailyTime: '02:00',
  tokenConfigured: false,
  status: 'never',
  lastRunAt: '',
  lastSuccessAt: '',
  lastError: '',
  lastRemoteName: '',
}

const getBindings = async () => {
  try {
    return await import('../../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

function normalizeScheduledBackupSettings(raw: any): ScheduledBackupSettings {
  const status = typeof raw?.status === 'string' ? raw.status : defaultScheduledBackupSettings.status
  return {
    ...defaultScheduledBackupSettings,
    enabled: raw?.enabled === true,
    dailyTime: typeof raw?.dailyTime === 'string' && raw.dailyTime ? raw.dailyTime : defaultScheduledBackupSettings.dailyTime,
    tokenConfigured: raw?.tokenConfigured === true,
    status: ['never', 'running', 'success', 'skipped', 'failed'].includes(status) ? status as ScheduledBackupSettings['status'] : 'never',
    lastRunAt: typeof raw?.lastRunAt === 'string' ? raw.lastRunAt : '',
    lastSuccessAt: typeof raw?.lastSuccessAt === 'string' ? raw.lastSuccessAt : '',
    lastError: typeof raw?.lastError === 'string' ? raw.lastError : '',
    lastRemoteName: typeof raw?.lastRemoteName === 'string' ? raw.lastRemoteName : '',
  }
}

export async function fetchScheduledBackupSettings(): Promise<ScheduledBackupSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupScheduledGetSettings) {
    return defaultScheduledBackupSettings
  }
  return normalizeScheduledBackupSettings(await bindings.BackupScheduledGetSettings())
}

export async function saveScheduledBackupSettings(draft: ScheduledBackupDraft): Promise<ScheduledBackupSettings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupScheduledSaveSettings) {
    throw new Error('当前环境不支持定时备份设置')
  }
  return normalizeScheduledBackupSettings(await bindings.BackupScheduledSaveSettings({
    enabled: String(draft.enabled),
    dailyTime: draft.dailyTime.trim(),
  }))
}
