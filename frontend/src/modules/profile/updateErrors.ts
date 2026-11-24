// Wails 绑定的 Promise 失败时 reject 的是后端错误字符串，不是 Error 实例；
// 直接读 error.message 会得到 undefined，把真实原因吞成通用文案。
export function resolveUpdateError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message
  }
  if (typeof error === 'string' && error.trim()) {
    return error
  }
  const message = (error as { message?: unknown } | null | undefined)?.message
  if (typeof message === 'string' && message.trim()) {
    return message
  }
  return fallback
}
