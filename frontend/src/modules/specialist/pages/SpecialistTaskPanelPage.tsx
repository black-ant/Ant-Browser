import { useEffect, useMemo, useState } from 'react'
import { ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useSearchParams } from 'react-router-dom'
import {
  fetchShopSpecialistTasks,
  fetchSpecialistTaskDetail,
  fetchTodaySpecialistTasks,
} from '../api'
import {
  specialistTaskDeadlineTone,
  specialistTaskPriorityLabel,
  specialistTaskStatusLabel,
} from '../presentation'
import type { SpecialistTaskListResponse, SpecialistTaskRecord } from '../types'
import { SpecialistTaskDrawer } from '../components/SpecialistTaskDrawer'

const { Text, Title } = Typography

function statusColor(status: string) {
  if (status === 'completed') return 'success'
  if (status === 'appeal_in_review' || status === 'overdue') return 'warning'
  if (status === 'validation_failed_penalty' || status === 'rejected_rework') return 'error'
  if (status === 'submitted_pending_validation' || status === 'in_progress') return 'processing'
  return 'default'
}

function deadlineColor(deadlineAt: string | null) {
  const tone = specialistTaskDeadlineTone(deadlineAt)
  if (tone === 'danger') return '#cf1322'
  if (tone === 'warning') return '#d46b08'
  return undefined
}

function formatTime(value: string | null | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function emptyOverview(): SpecialistTaskListResponse {
  return {
    items: [],
    pagination: { page: 1, pageSize: 100, total: 0 },
    summary: {
      total: 0,
      pending: 0,
      inProgress: 0,
      submittedPendingValidation: 0,
      appealInReview: 0,
      overdue: 0,
      completed: 0,
    },
  }
}

export function SpecialistTaskPanelPage() {
  const { notification } = App.useApp()
  const [searchParams] = useSearchParams()
  const shopId = searchParams.get('shopId')?.trim() || ''
  const [statusFilter, setStatusFilter] = useState('')
  const [overview, setOverview] = useState<SpecialistTaskListResponse>(emptyOverview)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const [selectedTaskId, setSelectedTaskId] = useState('')
  const [selectedTask, setSelectedTask] = useState<SpecialistTaskRecord | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  async function load(silent = false) {
    if (silent) setRefreshing(true)
    else setLoading(true)
    setError('')
    try {
      const query = { pageSize: 100, status: statusFilter || undefined }
      const nextOverview = shopId
        ? await fetchShopSpecialistTasks(shopId, query)
        : await fetchTodaySpecialistTasks(query)
      setOverview(nextOverview)
    } catch (loadError) {
      const message = loadError instanceof Error ? loadError.message : '专员任务加载失败'
      setError(message)
      notification.error({ message: '任务读取失败', description: message })
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  async function openTask(task: SpecialistTaskRecord) {
    setSelectedTaskId(task.id)
    setSelectedTask(task)
    setDetailLoading(true)
    try {
      setSelectedTask(await fetchSpecialistTaskDetail(task.id))
    } catch (detailError) {
      notification.error({
        message: '任务详情加载失败',
        description: detailError instanceof Error ? detailError.message : '请稍后重试',
      })
    } finally {
      setDetailLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [shopId, statusFilter])

  const columns = useMemo<ColumnsType<SpecialistTaskRecord>>(() => [
    {
      title: '任务',
      dataIndex: 'title',
      key: 'title',
      width: 300,
      ellipsis: true,
      render: (_, row) => (
        <Space direction="vertical" size={0} className="min-w-0">
          <Text strong ellipsis={{ tooltip: row.title || row.id }} style={{ maxWidth: 270 }}>
            {row.title || row.id}
          </Text>
          <Text type="secondary" ellipsis={{ tooltip: row.description }} style={{ maxWidth: 270, fontSize: 12 }}>
            {row.description || row.id}
          </Text>
        </Space>
      ),
    },
    {
      title: '店铺',
      dataIndex: 'shopName',
      key: 'shopName',
      width: 220,
      ellipsis: true,
      render: (_, row) => (
        <Space direction="vertical" size={0} className="min-w-0">
          <Text ellipsis={{ tooltip: row.shopName || row.shopId }} style={{ maxWidth: 190 }}>
            {row.shopName || row.shopId}
          </Text>
          <Text type="secondary" ellipsis={{ tooltip: row.shopId }} style={{ maxWidth: 190, fontSize: 12 }}>
            {row.shopId}
          </Text>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 138,
      render: (status: string) => (
        <Tag color={statusColor(String(status))}>{specialistTaskStatusLabel(String(status))}</Tag>
      ),
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      key: 'priority',
      width: 90,
      render: (priority: string) => specialistTaskPriorityLabel(priority),
    },
    {
      title: '截止时间',
      dataIndex: 'deadlineAt',
      key: 'deadlineAt',
      width: 150,
      render: (deadlineAt: string | null) => (
        <Text style={{ color: deadlineColor(deadlineAt) }}>{formatTime(deadlineAt)}</Text>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 150,
      render: (value: string) => <Text type="secondary">{formatTime(value)}</Text>,
    },
  ], [])

  const counts = overview.summary
  const metrics = [
    ['任务数', counts.total],
    ['待开始', counts.pending],
    ['处理中', counts.inProgress],
    ['待校验', counts.submittedPendingValidation],
    ['申诉中', counts.appealInReview],
    ['已完成', counts.completed],
  ] as const

  return (
    <div className="space-y-4 p-5">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <Title level={4} style={{ margin: 0 }}>专员任务台</Title>
          <Text type="secondary">
            {shopId ? `当前仅看店铺 ${shopId} 的专员任务。` : '按 SOP 执行、回传证据并提交验收。'}
          </Text>
        </div>
        <Space wrap>
          <Select
            aria-label="任务状态"
            value={statusFilter}
            style={{ width: 190 }}
            options={[
              { value: '', label: '全部状态' },
              { value: 'pending', label: '待开始' },
              { value: 'in_progress', label: '处理中' },
              { value: 'submitted_pending_validation', label: '已提交待校验' },
              { value: 'appeal_in_review', label: '申诉中' },
              { value: 'completed', label: '已完成' },
            ]}
            onChange={setStatusFilter}
          />
          <Button
            icon={<ReloadOutlined />}
            loading={refreshing}
            onClick={() => void load(true)}
          >
            刷新任务
          </Button>
        </Space>
      </div>

      {shopId ? (
        <Alert
          type="info"
          showIcon
          message="店铺级任务视图"
          description="仅展示当前店铺的专员任务，适合同屏打开 1688 后台后逐项处理。"
        />
      ) : null}
      {error ? <Alert type="error" showIcon message="任务读取失败" description={error} /> : null}

      <Card size="small" styles={{ body: { padding: '12px 16px' } }}>
        <Row gutter={[16, 12]}>
          {metrics.map(([title, value]) => (
            <Col key={title} xs={12} sm={8} lg={4}>
              <Statistic title={title} value={loading ? '-' : value} valueStyle={{ fontSize: 20 }} />
            </Col>
          ))}
        </Row>
      </Card>

      <Card size="small" styles={{ body: { padding: 0 } }}>
        <Table
          rowKey="id"
          size="small"
          columns={columns}
          dataSource={overview.items}
          loading={loading}
          pagination={false}
          scroll={{ x: 1040, y: 'calc(100vh - 360px)' }}
          locale={{ emptyText: shopId ? '该店铺暂无专员任务' : '今日暂无专员任务' }}
          onRow={(row) => ({
            onClick: () => void openTask(row),
            onKeyDown: (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                void openTask(row)
              }
            },
            tabIndex: 0,
            style: { cursor: 'pointer' },
            'aria-label': `查看任务：${row.title || row.id}`,
          })}
        />
      </Card>

      <SpecialistTaskDrawer
        open={Boolean(selectedTaskId)}
        task={selectedTask}
        loading={detailLoading}
        onClose={() => {
          setSelectedTaskId('')
          setSelectedTask(null)
        }}
        onTaskUpdated={setSelectedTask}
        onReload={() => void load(true)}
      />
    </div>
  )
}
