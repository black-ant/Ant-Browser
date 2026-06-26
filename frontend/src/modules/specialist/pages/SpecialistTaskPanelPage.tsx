import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle, Clock, ListChecks, RefreshCw, Store } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Badge, Button, Card, Select, StatCard, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import {
  fetchShopSpecialistTasks,
  fetchSpecialistTaskDetail,
  fetchTodaySpecialistTasks,
} from '../api'
import {
  requiredStepSummary,
  specialistTaskDeadlineTone,
  specialistTaskStatusLabel,
} from '../presentation'
import type { SpecialistTaskListResponse, SpecialistTaskRecord } from '../types'
import { SpecialistTaskDrawer } from '../components/SpecialistTaskDrawer'

function statusVariant(status: string) {
  if (status === 'completed') return 'success'
  if (status === 'appeal_in_review' || status === 'overdue') return 'warning'
  if (status === 'validation_failed_penalty' || status === 'rejected_rework') return 'error'
  if (status === 'submitted_pending_validation' || status === 'in_progress') return 'info'
  return 'default'
}

function deadlineClass(deadlineAt: string | null) {
  const tone = specialistTaskDeadlineTone(deadlineAt)
  if (tone === 'danger') return 'text-[var(--color-error)]'
  if (tone === 'warning') return 'text-[var(--color-warning)]'
  return 'text-[var(--color-text-secondary)]'
}

function formatTime(value: string | null | undefined) {
  if (!value) return '-'
  return value.replace('T', ' ').slice(0, 16)
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
      toast.error(message)
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
      const message = detailError instanceof Error ? detailError.message : '任务详情加载失败'
      toast.error(message)
    } finally {
      setDetailLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [shopId, statusFilter])

  const tasks = overview.items
  const counts = overview.summary
  const tableColumns = useMemo<TableColumn<SpecialistTaskRecord>[]>(() => [
    {
      key: 'title',
      title: '任务',
      width: 280,
      render: (_, row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <span className="max-w-[250px] truncate font-medium text-[var(--color-text-primary)]" title={row.title || row.id}>
            {row.title || row.id}
          </span>
          <span className="max-w-[250px] truncate text-xs text-[var(--color-text-muted)]" title={row.description}>
            {row.description || row.id}
          </span>
        </div>
      ),
    },
    {
      key: 'shopName',
      title: '店铺',
      width: 210,
      render: (_, row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <span className="max-w-[190px] truncate" title={row.shopName || row.shopId}>
            {row.shopName || row.shopId}
          </span>
          <span className="max-w-[190px] truncate text-xs text-[var(--color-text-muted)]" title={row.shopId}>
            {row.shopId}
          </span>
        </div>
      ),
    },
    {
      key: 'status',
      title: '状态',
      width: 136,
      render: (_, row) => (
        <Badge variant={statusVariant(String(row.status))}>{specialistTaskStatusLabel(String(row.status))}</Badge>
      ),
    },
    {
      key: 'sop',
      title: 'SOP',
      width: 96,
      render: (_, row) => (
        <span className="text-sm font-medium text-[var(--color-text-primary)]">{requiredStepSummary(row.sopSteps)}</span>
      ),
    },
    {
      key: 'deadlineAt',
      title: '截止',
      width: 150,
      render: (_, row) => (
        <span className={`text-xs font-medium ${deadlineClass(row.deadlineAt)}`}>{formatTime(row.deadlineAt)}</span>
      ),
    },
    {
      key: 'updatedAt',
      title: '更新',
      width: 150,
      render: (_, row) => (
        <span className="text-xs text-[var(--color-text-muted)]">{formatTime(row.updatedAt)}</span>
      ),
    },
  ], [])

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">专员任务台</h1>
          <p className="mt-1 break-words text-sm text-[var(--color-text-muted)]">
            {shopId ? `当前仅看店铺 ${shopId} 的专员任务。` : '今日待处理任务，按 SOP 勾选、证据回传和提交验收推进。'}
          </p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Select
            className="w-full sm:w-48"
            value={statusFilter}
            options={[
              { value: '', label: '全部状态' },
              { value: 'pending', label: '待开始' },
              { value: 'in_progress', label: '处理中' },
              { value: 'submitted_pending_validation', label: '已提交待校验' },
              { value: 'appeal_in_review', label: '申诉中' },
              { value: 'completed', label: '已完成' },
            ]}
            onChange={(event) => setStatusFilter(event.target.value)}
          />
          <Button className="w-full shrink-0 sm:w-auto" variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
        </div>
      </div>

      {shopId ? (
        <Alert
          type="info"
          title="店铺级任务视图"
          message="这里只展示当前店铺关联的专员任务，适合同屏打开 1688 后台后逐项处理。"
        />
      ) : null}
      {error ? <Alert type="error" title="任务读取失败" message={error} /> : null}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <StatCard title="任务数" value={loading ? '-' : counts.total} icon={<ListChecks className="h-5 w-5" />} />
        <StatCard title="待开始" value={loading ? '-' : counts.pending} icon={<Clock className="h-5 w-5" />} />
        <StatCard title="处理中" value={loading ? '-' : counts.inProgress} icon={<Store className="h-5 w-5" />} />
        <StatCard title="待校验" value={loading ? '-' : counts.submittedPendingValidation} icon={<CheckCircle className="h-5 w-5" />} />
        <StatCard title="申诉中" value={loading ? '-' : counts.appealInReview} icon={<AlertTriangle className="h-5 w-5" />} />
        <StatCard title="已完成" value={loading ? '-' : counts.completed} icon={<Badge variant="success">OK</Badge>} />
      </div>

      <Card padding="none">
        <Table
          columns={tableColumns}
          data={tasks}
          rowKey="id"
          loading={loading}
          emptyText={shopId ? '该店铺暂无专员任务' : '今日暂无专员任务'}
          maxHeight="calc(100vh - 360px)"
          onRowClick={(row) => void openTask(row)}
          className="min-w-0 [&_table]:table-fixed [&_td]:px-3 [&_th]:px-3"
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
        onTaskUpdated={(task) => setSelectedTask(task)}
        onReload={() => {
          void load(true)
        }}
      />
    </div>
  )
}
