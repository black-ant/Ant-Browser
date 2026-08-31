export interface S3Connection {
  endpoint: string
  region: string
  bucket: string
  prefix: string
  forcePathStyle: boolean
}

export interface S3Draft extends S3Connection {
  accessKeyID: string
  secretAccessKey: string
  sessionToken: string
}

export interface S3Settings extends S3Connection {
  accessKeyIDConfigured: boolean
  secretAccessKeyConfigured: boolean
  credentialsConfigured: boolean
  sessionTokenConfigured: boolean
}

export type S3CredentialField = 'accessKeyID' | 'secretAccessKey' | 'sessionToken'

export interface S3BackupFile {
  name: string
  size: number
  modifiedAt: string
}

export const defaultS3Settings: S3Settings = {
  endpoint: '',
  region: 'us-east-1',
  bucket: '',
  prefix: '',
  forcePathStyle: false,
  accessKeyIDConfigured: false,
  secretAccessKeyConfigured: false,
  credentialsConfigured: false,
  sessionTokenConfigured: false,
}

const getBindings = async () => {
  try {
    return await import('../../../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

function toPayload(value: Partial<S3Draft> | undefined, clearSessionToken: boolean): Record<string, string> {
  const payload: Record<string, string> = {
    endpoint: value?.endpoint?.trim() || '',
    region: value?.region?.trim() || '',
    bucket: value?.bucket?.trim() || '',
    prefix: value?.prefix?.trim() || '',
    forcePathStyle: String(value?.forcePathStyle === true),
  }
  if (typeof value?.accessKeyID === 'string' && value.accessKeyID.trim()) {
    payload.accessKeyID = value.accessKeyID.trim()
  }
  if (typeof value?.secretAccessKey === 'string' && value.secretAccessKey.trim()) {
    payload.secretAccessKey = value.secretAccessKey.trim()
  }
  if (typeof value?.sessionToken === 'string' && (clearSessionToken || value.sessionToken.trim())) {
    payload.sessionToken = value.sessionToken.trim()
  }
  return payload
}

function normalizeS3Settings(raw: any): S3Settings {
  return {
    ...defaultS3Settings,
    endpoint: typeof raw?.endpoint === 'string' ? raw.endpoint : '',
    region: typeof raw?.region === 'string' && raw.region ? raw.region : defaultS3Settings.region,
    bucket: typeof raw?.bucket === 'string' ? raw.bucket : '',
    prefix: typeof raw?.prefix === 'string' ? raw.prefix : '',
    forcePathStyle: raw?.forcePathStyle === true,
    accessKeyIDConfigured: raw?.accessKeyIDConfigured === true,
    secretAccessKeyConfigured: raw?.secretAccessKeyConfigured === true,
    credentialsConfigured: raw?.credentialsConfigured === true,
    sessionTokenConfigured: raw?.sessionTokenConfigured === true,
  }
}

export async function fetchS3Settings(): Promise<S3Settings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3GetSettings) {
    return defaultS3Settings
  }
  return normalizeS3Settings(await bindings.BackupS3GetSettings())
}

export async function saveS3Settings(draft: S3Draft): Promise<S3Settings> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3SaveSettings) {
    throw new Error('当前环境不支持 S3 配置保存')
  }
  return normalizeS3Settings(await bindings.BackupS3SaveSettings(toPayload(draft, true)))
}

export async function testS3Connection(draft: S3Draft): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3Test) {
    throw new Error('当前环境不支持 S3 连接测试')
  }
  await bindings.BackupS3Test(toPayload(draft, false))
}

export async function revealS3Credential(field: S3CredentialField): Promise<string> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3RevealCredential) {
    throw new Error('当前环境不支持显示 S3 凭据')
  }
  return String((await bindings.BackupS3RevealCredential(field)) || '')
}

export async function listS3Backups(connection?: S3Connection): Promise<S3BackupFile[]> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3List) {
    throw new Error('当前环境不支持 S3 备份列表')
  }
  const raw = await bindings.BackupS3List(toPayload(connection, false))
  if (!Array.isArray(raw)) return []
  return raw
    .map(item => ({
      name: typeof item?.name === 'string' ? item.name : '',
      size: Number.isFinite(item?.size) ? Math.max(0, Number(item.size)) : 0,
      modifiedAt: typeof item?.modifiedAt === 'string' ? item.modifiedAt : '',
    }))
    .filter(item => item.name)
}

export async function uploadS3Backup(connection?: S3Connection): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3Upload) {
    throw new Error('当前环境不支持 S3 备份上传')
  }
  return (await bindings.BackupS3Upload(toPayload(connection, false))) || {}
}

export async function restoreS3Backup(fileName: string, connection?: S3Connection): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupS3Restore) {
    throw new Error('当前环境不支持 S3 备份恢复')
  }
  return (await bindings.BackupS3Restore(toPayload(connection, false), fileName)) || {}
}
