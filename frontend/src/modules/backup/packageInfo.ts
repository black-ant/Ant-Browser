export interface BackupPackageInfo {
  packageType?: string
  profileCount?: number
  profileNames?: string[]
}

function normalizeProfileNames(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const names: string[] = []
  for (const item of value) {
    if (typeof item !== 'string') continue
    const name = item.trim()
    if (!name || seen.has(name)) continue
    seen.add(name)
    names.push(name)
  }
  return names
}

function normalizeProfileCount(value: unknown): number | undefined {
  const count = Number(value)
  if (!Number.isFinite(count) || count <= 0) return undefined
  return Math.floor(count)
}

export function inferBackupPackageInfoFromName(fileName: string): BackupPackageInfo {
  const baseName = fileName.trim().split(/[\\/]/).pop() || fileName.trim()
  const singleMatch = /^ant-chrome-profile-backup-single--(.+)--\d{8}-\d{6}(?:\.\d+)?\.zip$/iu.exec(baseName)
  if (singleMatch) {
    const name = singleMatch[1].trim()
    return {
      packageType: 'profile',
      profileCount: 1,
      profileNames: name && name !== '未命名实例' ? [name] : undefined,
    }
  }
  const multiMatch = /^ant-chrome-profile-backup-multi-(\d+)--/iu.exec(baseName)
  if (multiMatch) {
    return {
      packageType: 'profile',
      profileCount: normalizeProfileCount(multiMatch[1]),
    }
  }
  if (/^ant-chrome-profile-backup-/iu.test(baseName)) {
    return { packageType: 'profile' }
  }
  if (/^ant-chrome-backup-/iu.test(baseName)) {
    return { packageType: 'full' }
  }
  return {}
}

export function normalizeBackupPackageInfo(raw: unknown, fallbackName = ''): BackupPackageInfo {
  const value = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  const inferred = inferBackupPackageInfoFromName(fallbackName)
  const packageType = typeof value.packageType === 'string' && value.packageType.trim()
    ? value.packageType.trim()
    : inferred.packageType
  const rawNames = normalizeProfileNames(value.profileNames)
  const profileNames = rawNames.length > 0 ? rawNames : inferred.profileNames || []
  const profileCount = normalizeProfileCount(value.profileCount)
    || inferred.profileCount
    || (profileNames.length > 0 ? profileNames.length : undefined)

  return {
    packageType,
    profileCount,
    profileNames: profileNames.length > 0 ? profileNames : undefined,
  }
}
