import { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  App,
  Button,
  Card,
  Checkbox,
  Col,
  Descriptions,
  Divider,
  Drawer,
  Empty,
  Form,
  Input,
  List,
  Row,
  Select,
  Skeleton,
  Space,
  Tag,
  Typography,
} from 'antd'
import {
  appealSpecialistTask,
  blockSpecialistTask,
  submitSpecialistTask,
  submitSpecialistTaskEvidence,
  updateSpecialistTaskSopStep,
} from '../api'
import {
  evidenceTypeLabel,
  requiredStepSummary,
  specialistTaskPriorityLabel,
  specialistTaskStatusLabel,
} from '../presentation'
import type { SpecialistTaskRecord, SpecialistTaskSopStep } from '../types'

const { Paragraph, Text } = Typography
const { TextArea } = Input

function statusColor(status: string) {
  if (status === 'completed') return 'success'
  if (status === 'appeal_in_review' || status === 'overdue') return 'warning'
  if (status === 'validation_failed_penalty' || status === 'rejected_rework') return 'error'
  if (status === 'submitted_pending_validation' || status === 'in_progress') return 'processing'
  return 'default'
}

function formatTime(value: string | null | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
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
  const { notification } = App.useApp()
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

  async function runAction(
    actionName: string,
    action: () => Promise<SpecialistTaskRecord>,
    successMessage: string,
  ) {
    setSavingAction(actionName)
    try {
      const nextTask = await action()
      onTaskUpdated(nextTask)
      notification.success({ message: successMessage })
      onReload()
    } catch (error) {
      notification.error({
        message: '专员任务操作失败',
        description: error instanceof Error ? error.message : '请稍后重试',
      })
    } finally {
      setSavingAction('')
    }
  }

  const evidenceOptions = (selectedStep?.evidenceTypes?.length
    ? selectedStep.evidenceTypes
    : ['text_note']).map((type) => ({ value: type, label: evidenceTypeLabel(type) }))

  return (
    <Drawer
      open={open}
      width={920}
      destroyOnClose
      title={task?.title || '任务详情'}
      extra={task ? <Tag color={statusColor(task.status)}>{specialistTaskStatusLabel(task.status)}</Tag> : null}
      onClose={onClose}
      footer={task ? (
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <Text type="secondary">必填 SOP：{requiredStepSummary(steps)}</Text>
          <Space wrap>
            <Button
              disabled={!appealReason.trim() || Boolean(savingAction)}
              loading={savingAction === 'appeal'}
              onClick={() => void runAction(
                'appeal',
                async () => (await appealSpecialistTask(task.id, { reason: appealReason.trim() })).task,
                '申诉已提交',
              )}
            >
              提交申诉
            </Button>
            <Button
              danger
              disabled={!blockReasonText.trim() || Boolean(savingAction)}
              loading={savingAction === 'block'}
              onClick={() => void runAction(
                'block',
                async () => (await blockSpecialistTask(task.id, {
                  reasonCode: blockReasonCode,
                  reasonText: blockReasonText.trim(),
                })).task,
                '阻塞原因已提交',
              )}
            >
              提交无法处理
            </Button>
            <Button
              type="primary"
              disabled={requiredIncomplete || Boolean(savingAction)}
              loading={savingAction === 'submit'}
              title={requiredIncomplete ? '完成全部必填 SOP 后才能提交' : '提交处理结果'}
              onClick={() => void runAction(
                'submit',
                async () => (await submitSpecialistTask(task.id, { summary: submitSummary.trim() })).task,
                '任务已提交待验收',
              )}
            >
              提交处理结果
            </Button>
          </Space>
        </div>
      ) : null}
    >
      {loading && !task ? <Skeleton active paragraph={{ rows: 8 }} /> : null}
      {!loading && !task ? <Empty description="请选择任务" /> : null}
      {task ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2, lg: 4 }}>
            <Descriptions.Item label="店铺">{task.shopName || task.shopId || '-'}</Descriptions.Item>
            <Descriptions.Item label="优先级">{specialistTaskPriorityLabel(task.priority)}</Descriptions.Item>
            <Descriptions.Item label="截止时间">{formatTime(task.deadlineAt)}</Descriptions.Item>
            <Descriptions.Item label="负责人">{task.assigneeName || task.assigneeUserId || '-'}</Descriptions.Item>
            <Descriptions.Item label="任务编号" span={2}>{task.id}</Descriptions.Item>
            <Descriptions.Item label="异常类型" span={2}>{task.anomalySignalType || '-'}</Descriptions.Item>
          </Descriptions>

          {task.description ? <Alert type="info" showIcon message="任务说明" description={task.description} /> : null}

          <Card size="small" title={`SOP 步骤（必填 ${requiredStepSummary(steps)}）`}>
            {steps.length ? (
              <List
                size="small"
                dataSource={steps}
                renderItem={(step) => {
                  const done = step.status === 'done'
                  return (
                    <List.Item
                      actions={[
                        <Checkbox
                          key="status"
                          checked={done}
                          disabled={Boolean(savingAction)}
                          onChange={() => void runAction(
                            `step:${step.stepId}`,
                            async () => (await updateSpecialistTaskSopStep(task.id, step.stepId, {
                              status: done ? 'not_started' : 'done',
                              operatorNote: operatorNote.trim(),
                              evidenceRefs: step.evidenceRefs,
                            })).task,
                            done ? 'SOP 步骤已改为未完成' : 'SOP 步骤已完成',
                          )}
                        >
                          {done ? '已完成' : '标记完成'}
                        </Checkbox>,
                      ]}
                    >
                      <List.Item.Meta
                        title={(
                          <Space wrap>
                            <Text strong>{step.title || step.stepId}</Text>
                            <Tag color={step.required ? 'warning' : 'default'}>{step.required ? '必填' : '可选'}</Tag>
                          </Space>
                        )}
                        description={(
                          <Space direction="vertical" size={2}>
                            {step.description ? <Text type="secondary">{step.description}</Text> : null}
                            <Text type="secondary" style={{ fontSize: 12 }}>
                              证据要求：{step.evidenceTypes.map(evidenceTypeLabel).join('、') || '未限制'}
                            </Text>
                            {step.operatorNote ? <Text style={{ fontSize: 12 }}>执行备注：{step.operatorNote}</Text> : null}
                          </Space>
                        )}
                      />
                    </List.Item>
                  )
                }}
              />
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该任务没有 SOP 步骤" />}
            <Divider style={{ margin: '12px 0' }} />
            <Form.Item label="本次步骤备注" style={{ marginBottom: 0 }}>
              <Input
                value={operatorNote}
                maxLength={500}
                placeholder="勾选步骤时一并提交，说明现场处理情况"
                onChange={(event) => setOperatorNote(event.target.value)}
              />
            </Form.Item>
          </Card>

          <Card size="small" title="提交证据">
            <Row gutter={12}>
              <Col xs={24} md={8}>
                <Form.Item label="SOP 步骤">
                  <Select
                    value={selectedStep?.stepId || undefined}
                    style={{ width: '100%' }}
                    placeholder="选择步骤"
                    options={steps.map((step) => ({ value: step.stepId, label: step.title || step.stepId }))}
                    onChange={setEvidenceStepId}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={6}>
                <Form.Item label="证据类型">
                  <Select
                    value={evidenceType}
                    style={{ width: '100%' }}
                    options={evidenceOptions}
                    onChange={setEvidenceType}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={10}>
                <Form.Item label="证据说明">
                  <TextArea
                    rows={2}
                    maxLength={2000}
                    showCount
                    value={evidenceText}
                    placeholder="填写截图说明、后台链接或操作结果"
                    onChange={(event) => setEvidenceText(event.target.value)}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Button
              type="primary"
              disabled={!selectedStep || !evidenceText.trim() || Boolean(savingAction)}
              loading={savingAction === 'evidence'}
              onClick={() => void runAction(
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
              )}
            >
              提交证据
            </Button>
            {task.evidenceRecords.length ? (
              <List
                className="mt-3"
                size="small"
                dataSource={task.evidenceRecords}
                renderItem={(record) => (
                  <List.Item>
                    <Text>
                      {evidenceTypeLabel(record.evidenceType)}，{record.stepId || '任务级'}，{formatTime(record.createdAt)}
                    </Text>
                  </List.Item>
                )}
              />
            ) : null}
          </Card>

          <Card size="small" title="提交与异常处理">
            <Row gutter={12}>
              <Col xs={24} lg={8}>
                <Form.Item label="处理结果摘要">
                  <TextArea
                    rows={4}
                    maxLength={2000}
                    showCount
                    value={submitSummary}
                    placeholder="填写本次处理结果，提交后进入验收"
                    onChange={(event) => setSubmitSummary(event.target.value)}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} lg={8}>
                <Form.Item label="申诉原因">
                  <TextArea
                    rows={4}
                    maxLength={1000}
                    showCount
                    value={appealReason}
                    placeholder="例如平台统计延迟、任务不适用"
                    onChange={(event) => setAppealReason(event.target.value)}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} lg={8}>
                <Form.Item label="无法处理原因">
                  <Select
                    value={blockReasonCode}
                    style={{ width: '100%' }}
                    options={[
                      { value: 'backend_unavailable', label: '1688 后台不可用' },
                      { value: 'permission_missing', label: '缺少后台权限' },
                      { value: 'data_not_found', label: '数据不存在' },
                      { value: 'other', label: '其他原因' },
                    ]}
                    onChange={setBlockReasonCode}
                  />
                </Form.Item>
                <Form.Item label="原因说明">
                  <TextArea
                    rows={2}
                    maxLength={1000}
                    showCount
                    value={blockReasonText}
                    placeholder="写清现场情况，方便主管复核"
                    onChange={(event) => setBlockReasonText(event.target.value)}
                  />
                </Form.Item>
              </Col>
            </Row>
            {requiredIncomplete ? (
              <Paragraph type="warning" style={{ marginBottom: 0 }}>
                完成全部必填 SOP 后才能提交处理结果，服务端会再次校验。
              </Paragraph>
            ) : null}
          </Card>
        </Space>
      ) : null}
    </Drawer>
  )
}
