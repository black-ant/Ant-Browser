import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle, FileText, Send, ShieldAlert } from 'lucide-react'
import { Alert, Badge, Button, Card, Drawer, FormItem, Select, Textarea, toast } from '../../../shared/components'
import {
  appealSpecialistTask,
  blockSpecialistTask,
  submitSpecialistTask,
  submitSpecialistTaskEvidence,
  updateSpecialistTaskSopStep,
} from '../api'
import { evidenceTypeLabel, requiredStepSummary, specialistTaskStatusLabel } from '../presentation'
import type { SpecialistTaskRecord, SpecialistTaskSopStep } from '../types'

function statusVariant(status: string) {
  if (status === 'completed') return 'success'
  if (status === 'appeal_in_review' || status === 'overdue') return 'warning'
  if (status === 'validation_failed_penalty' || status === 'rejected_rework') return 'error'
  if (status === 'submitted_pending_validation' || status === 'in_progress') return 'info'
  return 'default'
}

function formatTime(value: string | null | undefined) {
  if (!value) return '-'
  return value.replace('T', ' ').slice(0, 16)
}

function compact(value: string | null | undefined) {
  return value?.trim() || '-'
}

function firstEvidenceType(step: SpecialistTaskSopStep | undefined) {
  return step?.evidenceTypes?.[0] || 'text_note'
}

interface SpecialistTaskDrawerProps {
  open: boolean
  task: SpecialistTaskRecord | null
  loading?: boolean
  onClose: () => void
  onTaskUpdated: (task: SpecialistTaskRecord) => void
  onReload: () => void
}

export function SpecialistTaskDrawer({
  open,
  task,
  loading = false,
  onClose,
  onTaskUpdated,
  onReload,
}: SpecialistTaskDrawerProps) {
  const [savingAction, setSavingAction] = useState('')
  const [operatorNote, setOperatorNote] = useState('')
  const [evidenceStepId, setEvidenceStepId] = useState('')
  const [evidenceType, setEvidenceType] = useState('text_note')
  const [evidenceText, setEvidenceText] = useState('')
  const [submitSummary, setSubmitSummary] = useState('')
  const [appealReason, setAppealReason] = useState('')
  const [blockReasonCode, setBlockReasonCode] = useState('backend_unavailable')
  const [blockReasonText, setBlockReasonText] = useState('')

  const steps = task?.sopSteps ?? []
  const requiredIncomplete = steps.some((step) => step.required && step.status !== 'done')
  const selectedStep = useMemo(
    () => steps.find((step) => step.stepId === evidenceStepId) ?? steps[0],
    [evidenceStepId, steps],
  )

  useEffect(() => {
    if (!task) return
    const firstStep = task.sopSteps[0]
    setOperatorNote('')
    setEvidenceStepId(firstStep?.stepId || '')
    setEvidenceType(firstEvidenceType(firstStep))
    setEvidenceText('')
    setSubmitSummary('')
    setAppealReason('')
    setBlockReasonCode('backend_unavailable')
    setBlockReasonText('')
  }, [task?.id])

  useEffect(() => {
    setEvidenceType(firstEvidenceType(selectedStep))
  }, [selectedStep?.stepId])

  async function runAction(actionName: string, action: () => Promise<SpecialistTaskRecord>, successMessage: string) {
    setSavingAction(actionName)
    try {
      const nextTask = await action()
      onTaskUpdated(nextTask)
      toast.success(successMessage)
      onReload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '专员任务操作失败')
    } finally {
      setSavingAction('')
    }
  }

  if (!task) {
    return (
      <Drawer open={open} title="任务详情" width="880px" onClose={onClose}>
        <div className="py-12 text-center text-sm text-[var(--color-text-muted)]">
          {loading ? '任务详情加载中...' : '请选择任务'}
        </div>
      </Drawer>
    )
  }

  const status = String(task.status || 'pending')
  const evidenceOptions = (selectedStep?.evidenceTypes?.length ? selectedStep.evidenceTypes : ['text_note']).map((type) => ({
    value: type,
    label: evidenceTypeLabel(type),
  }))

  return (
    <Drawer
      open={open}
      title={task.title || task.id}
      subtitle={`${task.shopName || task.shopId} / ${task.id}`}
      width="920px"
      onClose={onClose}
      footer={
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-xs text-[var(--color-text-muted)]">
            必填 SOP：{requiredStepSummary(steps)}
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={!appealReason.trim() || Boolean(savingAction)}
              loading={savingAction === 'appeal'}
              onClick={() =>
                void runAction(
                  'appeal',
                  async () => (await appealSpecialistTask(task.id, { reason: appealReason.trim() })).task,
                  '申诉已提交',
                )
              }
            >
              <ShieldAlert className="h-4 w-4" />
              申诉
            </Button>
            <Button
              variant="danger"
              size="sm"
              disabled={!blockReasonText.trim() || Boolean(savingAction)}
              loading={savingAction === 'block'}
              onClick={() =>
                void runAction(
                  'block',
                  async () => (
                    await blockSpecialistTask(task.id, {
                      reasonCode: blockReasonCode,
                      reasonText: blockReasonText.trim(),
                    })
                  ).task,
                  '阻塞原因已提交',
                )
              }
            >
              <AlertTriangle className="h-4 w-4" />
              无法处理
            </Button>
            <Button
              size="sm"
              disabled={requiredIncomplete || Boolean(savingAction)}
              loading={savingAction === 'submit'}
              title={requiredIncomplete ? '必填 SOP 完成后才能提交' : '提交处理结果'}
              onClick={() =>
                void runAction(
                  'submit',
                  async () => (await submitSpecialistTask(task.id, { summary: submitSummary.trim() })).task,
                  '任务已提交待验收',
                )
              }
            >
              <Send className="h-4 w-4" />
              提交结果
            </Button>
          </div>
        </div>
      }
    >
      <div className="space-y-4">
        <Card padding="sm">
          <div className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
            <div>
              <div className="text-xs text-[var(--color-text-muted)]">状态</div>
              <Badge className="mt-1" variant={statusVariant(status)}>
                {specialistTaskStatusLabel(status)}
              </Badge>
            </div>
            <div>
              <div className="text-xs text-[var(--color-text-muted)]">优先级</div>
              <div className="mt-1 font-medium text-[var(--color-text-primary)]">{compact(String(task.priority))}</div>
            </div>
            <div>
              <div className="text-xs text-[var(--color-text-muted)]">截止时间</div>
              <div className="mt-1 font-medium text-[var(--color-text-primary)]">{formatTime(task.deadlineAt)}</div>
            </div>
            <div>
              <div className="text-xs text-[var(--color-text-muted)]">主管</div>
              <div className="mt-1 font-medium text-[var(--color-text-primary)]">{compact(task.supervisorName)}</div>
            </div>
          </div>
          {task.description ? (
            <p className="mt-3 text-sm text-[var(--color-text-secondary)]">{task.description}</p>
          ) : null}
        </Card>

        {requiredIncomplete ? (
          <Alert type="warning" title="仍有必填 SOP 未完成" message="请先完成所有必填步骤，再提交处理结果。" />
        ) : null}

        <Card title="SOP 步骤" subtitle="勾选步骤只记录执行状态，真正状态校验由服务端完成。">
          <div className="space-y-3">
            {steps.map((step) => {
              const done = step.status === 'done'
              return (
                <div
                  key={step.stepId}
                  className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] p-3"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-[var(--color-text-primary)]">{step.title || step.stepId}</span>
                        {step.required ? <Badge variant="warning" size="sm">必填</Badge> : <Badge size="sm">可选</Badge>}
                        <Badge variant={done ? 'success' : 'default'} size="sm">
                          {done ? '已完成' : '未完成'}
                        </Badge>
                      </div>
                      {step.description ? (
                        <p className="mt-1 text-sm text-[var(--color-text-muted)]">{step.description}</p>
                      ) : null}
                      <div className="mt-2 text-xs text-[var(--color-text-muted)]">
                        证据：{step.evidenceTypes.map(evidenceTypeLabel).join(' / ') || '未限制'}
                      </div>
                      {step.operatorNote ? (
                        <div className="mt-2 text-xs text-[var(--color-text-secondary)]">备注：{step.operatorNote}</div>
                      ) : null}
                    </div>
                    <Button
                      size="sm"
                      variant={done ? 'secondary' : 'primary'}
                      loading={savingAction === `step:${step.stepId}`}
                      disabled={Boolean(savingAction)}
                      onClick={() =>
                        void runAction(
                          `step:${step.stepId}`,
                          async () => (
                            await updateSpecialistTaskSopStep(task.id, step.stepId, {
                              status: done ? 'not_started' : 'done',
                              operatorNote: operatorNote.trim(),
                              evidenceRefs: step.evidenceRefs,
                            })
                          ).task,
                          done ? 'SOP 步骤已改为未完成' : 'SOP 步骤已完成',
                        )
                      }
                    >
                      <CheckCircle className="h-4 w-4" />
                      {done ? '取消完成' : '标记完成'}
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
          <FormItem className="mt-4" label="步骤备注">
            <Textarea
              rows={3}
              value={operatorNote}
              placeholder="可在标记 SOP 步骤时同步写入备注"
              onChange={(event) => setOperatorNote(event.target.value)}
            />
          </FormItem>
        </Card>

        <Card title="证据提交" subtitle="证据会绑定到选定 SOP 步骤。截图或链接可先用文字描述留档。">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[220px_180px_1fr_auto] md:items-end">
            <FormItem label="SOP 步骤">
              <Select
                value={selectedStep?.stepId || ''}
                options={steps.map((step) => ({ value: step.stepId, label: step.title || step.stepId }))}
                onChange={(event) => setEvidenceStepId(event.target.value)}
              />
            </FormItem>
            <FormItem label="证据类型">
              <Select value={evidenceType} options={evidenceOptions} onChange={(event) => setEvidenceType(event.target.value)} />
            </FormItem>
            <FormItem label="证据说明">
              <Textarea
                rows={2}
                value={evidenceText}
                placeholder="填写截图说明、后台链接或操作结果"
                onChange={(event) => setEvidenceText(event.target.value)}
              />
            </FormItem>
            <Button
              className="md:mb-1"
              size="sm"
              disabled={!selectedStep || !evidenceText.trim() || Boolean(savingAction)}
              loading={savingAction === 'evidence'}
              onClick={() =>
                void runAction(
                  'evidence',
                  async () => {
                    const result = await submitSpecialistTaskEvidence(task.id, {
                      stepId: selectedStep?.stepId || '',
                      evidenceType,
                      payload: { text: evidenceText.trim() },
                    })
                    setEvidenceText('')
                    return result.task
                  },
                  '证据已提交',
                )
              }
            >
              <FileText className="h-4 w-4" />
              提交证据
            </Button>
          </div>
          {task.evidenceRecords.length ? (
            <div className="mt-3 space-y-2">
              {task.evidenceRecords.map((record) => (
                <div key={record.id} className="rounded-lg bg-[var(--color-bg-muted)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                  {evidenceTypeLabel(record.evidenceType)} / {record.stepId || '任务'} / {formatTime(record.createdAt)}
                </div>
              ))}
            </div>
          ) : null}
        </Card>

        <Card title="提交与异常处理" subtitle="申诉和无法处理会进入主管复核。">
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
            <FormItem label="处理结果摘要">
              <Textarea
                rows={4}
                value={submitSummary}
                placeholder="提交前可填写本次处理摘要"
                onChange={(event) => setSubmitSummary(event.target.value)}
              />
            </FormItem>
            <FormItem label="申诉原因">
              <Textarea
                rows={4}
                value={appealReason}
                placeholder="例如：平台统计延迟、任务不适用"
                onChange={(event) => setAppealReason(event.target.value)}
              />
            </FormItem>
            <div className="space-y-3">
              <FormItem label="无法处理原因">
                <Select
                  value={blockReasonCode}
                  options={[
                    { value: 'backend_unavailable', label: '1688 后台不可用' },
                    { value: 'permission_missing', label: '缺少后台权限' },
                    { value: 'data_not_found', label: '数据不存在' },
                    { value: 'other', label: '其他原因' },
                  ]}
                  onChange={(event) => setBlockReasonCode(event.target.value)}
                />
              </FormItem>
              <FormItem label="原因说明">
                <Textarea
                  rows={3}
                  value={blockReasonText}
                  placeholder="写清楚现场情况，方便主管复核"
                  onChange={(event) => setBlockReasonText(event.target.value)}
                />
              </FormItem>
            </div>
          </div>
        </Card>
      </div>
    </Drawer>
  )
}
