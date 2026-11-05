const browserProfileCopySuffixPattern = /\s*(?:\(\s*副本\s*\)|（副本）)\s*(?:\d{12})?$/u

function formatBrowserProfileCopyTimestamp(date: Date): string {
  const pad = (value: number) => value.toString().padStart(2, '0')

  return [
    pad(date.getFullYear() % 100),
    pad(date.getMonth() + 1),
    pad(date.getDate()),
    pad(date.getHours()),
    pad(date.getMinutes()),
    pad(date.getSeconds()),
  ].join('')
}

function normalizeBrowserProfileCopyBaseName(sourceName: string): string {
  const trimmed = sourceName.trim()
  if (!trimmed) {
    return ''
  }

  let baseName = trimmed
  while (baseName) {
    const nextName = baseName.replace(browserProfileCopySuffixPattern, '').trim()
    if (nextName === baseName) {
      break
    }
    if (!nextName) {
      return trimmed
    }
    baseName = nextName
  }

  return baseName
}

export function buildBrowserProfileCopyName(sourceName: string, now: Date = new Date()): string {
  const baseName = normalizeBrowserProfileCopyBaseName(sourceName) || '未命名实例'
  return `${baseName}（副本）${formatBrowserProfileCopyTimestamp(now)}`
}
