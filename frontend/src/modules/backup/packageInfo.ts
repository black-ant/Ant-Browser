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

export function normalizeBackupPackageInfo(raw: unknown): BackupPackageInfo {
  const value = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  const packageType = typeof value.packageType === 'string' && value.packageType.trim()
    ? value.packageType.trim()
    : undefined
  const rawNames = normalizeProfileNames(value.profileNames)
  const profileNames = rawNames
  const profileCount = normalizeProfileCount(value.profileCount)
    || (profileNames.length > 0 ? profileNames.length : undefined)

  return {
    packageType,
    profileCount,
    profileNames: profileNames.length > 0 ? profileNames : undefined,
  }
}
