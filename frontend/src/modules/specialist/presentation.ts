export type DeadlineTone = 'neutral' | 'warning' | 'danger'

const statusLabels: Record<string, string> = {
  pending: '待开始',
  in_progress: '处理中',
  submitted_pending_validation: '已提交待校验',
  appeal_in_review: '申诉中',
  rejected_rework: '驳回重做',
  completed: '已完成',
  validation_failed_penalty: '校验失败',
  overdue: '已逾期',
  cancelled: '已取消',
}

const evidenceLabels: Record<string, string> = {
  screenshot: '截图',
  backend_url: '后台链接',
  product_url: '商品链接',
  text_note: '文字说明',
  operation_summary: '处理摘要',
}

const priorityLabels: Record<string, string> = {
  critical: '紧急',
  high: '高',
  medium: '中',
  low: '低',
}

export function specialistTaskStatusLabel(status: string): string {
  return statusLabels[status] ?? (status || '-')
}

export function specialistTaskPriorityLabel(priority: string): string {
  return priorityLabels[priority] ?? (priority || '-')
}

export function specialistTaskDeadlineTone(deadlineAt: string | null): DeadlineTone {
  if (!deadlineAt) return 'neutral'
  const timestamp = new Date(deadlineAt).getTime()
  if (!Number.isFinite(timestamp)) return 'neutral'
  const remainingMs = timestamp - Date.now()
  if (remainingMs < 0) return 'danger'
  if (remainingMs <= 60 * 60 * 1000) return 'warning'
  return 'neutral'
}

export function requiredStepSummary(steps: Array<{ required: boolean; status: string }>): string {
  const requiredSteps = steps.filter((step) => step.required)
  if (!requiredSteps.length) return '0/0'
  const doneCount = requiredSteps.filter((step) => step.status === 'done').length
  return `${doneCount}/${requiredSteps.length}`
}

export function evidenceTypeLabel(type: string): string {
  return evidenceLabels[type] ?? (type || '-')
}
