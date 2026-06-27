const backgroundAttachMarkers = [
  '系统会继续在后台连接',
  '系统将继续后台连接',
  '程序将继续后台重试',
]

function extractActionMessage(error: unknown): string {
  const message =
    typeof error === 'string'
      ? error
      : error && typeof error === 'object' && 'message' in error
        ? String((error as { message?: unknown }).message || '')
        : ''

  const normalized = message.trim()
  if (normalized) {
    return normalized
  }

  return ''
}

export function isBackgroundAttachMessage(message: string): boolean {
  return backgroundAttachMarkers.some(marker => message.includes(marker))
}

export function resolveActionFeedback(error: unknown, fallback: string): { message: string; tone: 'error' | 'warning'; pendingAttach: boolean } {
  const normalized = extractActionMessage(error)
  const message = normalized || `${fallback}，但系统没有返回明确原因。请在窗口详情中查看最近错误，或检查应用日志。`
  const pendingAttach = isBackgroundAttachMessage(message)
  return {
    message,
    tone: pendingAttach ? 'warning' : 'error',
    pendingAttach,
  }
}

export function resolveActionErrorMessage(error: unknown, fallback: string): string {
  return resolveActionFeedback(error, fallback).message
}
