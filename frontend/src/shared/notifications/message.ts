export type NotificationMessageContext = 'generic' | 'backup'

export interface NotificationMessageView {
  summary: string
  details?: string
  copyText?: string
}

const SUMMARY_LIMIT = 180

function compactWhitespace(value: string) {
  return value.replace(/\s+/g, ' ').trim()
}

function truncate(value: string, limit = SUMMARY_LIMIT) {
  if (value.length <= limit) return value
  return `${value.slice(0, limit - 1).trimEnd()}…`
}

function findStructuredPayloadStart(value: string) {
  const objectStart = value.indexOf('{')
  const arrayStart = value.indexOf('[')
  if (objectStart >= 0) {
    try {
      JSON.parse(value.slice(objectStart).trim())
      return objectStart
    } catch {
      return arrayStart >= 0 ? arrayStart : -1
    }
  }
  if (arrayStart >= 0) {
    try {
      JSON.parse(value.slice(arrayStart).trim())
      return arrayStart
    } catch {
      return -1
    }
  }
  return -1
}

function formatTechnicalDetails(value: string) {
  const payloadStart = findStructuredPayloadStart(value)
  if (payloadStart < 0) return value

  const prefix = value.slice(0, payloadStart).trim()
  const payload = value.slice(payloadStart).trim()
  try {
    const parsed = JSON.parse(payload)
    return prefix ? `${prefix}\n${JSON.stringify(parsed, null, 2)}` : JSON.stringify(parsed, null, 2)
  } catch {
    return value
  }
}

function getBackupSummary(value: string) {
  const partialFailure = value.match(/^备份已完成，但部分远程渠道失败[:：]\s*([^:：;，,\s]+)/)
  if (partialFailure?.[1]) return `备份完成，但 ${partialFailure[1]} 上传失败`

  const remoteFailure = value.match(/^远程备份失败[:：]\s*([^:：;，,\s]+)/)
  if (remoteFailure?.[1]) return `远程备份失败：${remoteFailure[1]}`

  const committedUploadWarning = value.match(/(OpenList|S3)[^\n]{0,180}?(?:虚拟盘文件已写入|已写入虚拟盘|virtual disk[^\n]{0,40}written)[^\n]{0,180}?(?:同步失败|目标同步|sync failed)/i)
  if (committedUploadWarning?.[1]) return `${committedUploadWarning[1]} 文件已写入，目标同步有警告`

  const channelFailure = value.match(/\b(OpenList|S3)\b[^\n]{0,100}?(?:失败|failed)/i)
  if (channelFailure?.[1]) return `${channelFailure[1]} 备份失败`

  const httpFailure = value.match(/\bHTTP\s+(\d{3})\b/i)
  if (httpFailure?.[1]) return `远程请求失败（HTTP ${httpFailure[1]}）`

  if (/^(?:备份|导出)失败[:：]/.test(value)) return value.split(/[:：]/, 1)[0]
  return undefined
}

function getTechnicalPrefix(value: string) {
  const payloadStart = findStructuredPayloadStart(value)
  const prefix = payloadStart >= 0 ? value.slice(0, payloadStart) : value
  const compact = compactWhitespace(prefix).replace(/[:：]\s*$/, '').trim()
  return compact && compact.length <= SUMMARY_LIMIT ? compact : undefined
}

export function getNotificationMessageView(
  message: string,
  context: NotificationMessageContext = 'generic',
): NotificationMessageView {
  const normalized = message.trim() || '未提供详细信息'
  const payloadStart = findStructuredPayloadStart(normalized)
  const needsDetails = payloadStart >= 0 || normalized.length > SUMMARY_LIMIT

  if (!needsDetails) return { summary: normalized }

  const inferredBackup = context === 'backup' || /备份|OpenList|S3|远程渠道/.test(normalized)
  const contextSummary = inferredBackup ? getBackupSummary(normalized) : undefined
  const summary = contextSummary || getTechnicalPrefix(normalized) || truncate(compactWhitespace(normalized))
  return {
    summary,
    details: formatTechnicalDetails(normalized),
    copyText: normalized,
  }
}
