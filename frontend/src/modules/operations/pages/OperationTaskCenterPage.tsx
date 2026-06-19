import { useEffect, useMemo, useState } from 'react'
import { ListChecks, RefreshCw } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Badge, Button, Card, StatCard, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import { deriveOperationTaskCounts, fetchOperationTasks, operationTaskStatusLabel } from '../api'
import type { OperationTask, OperationTaskStatus } from '../types'

function statusBadge(status: OperationTaskStatus) {
  if (status === 'running') return <Badge variant="info">{operationTaskStatusLabel(status)}</Badge>
  if (status === 'blocked') return <Badge variant="warning">{operationTaskStatusLabel(status)}</Badge>
  if (status === 'failed') return <Badge variant="error">{operationTaskStatusLabel(status)}</Badge>
  if (status === 'completed') return <Badge variant="success">{operationTaskStatusLabel(status)}</Badge>
  return <Badge variant="default">{operationTaskStatusLabel(status)}</Badge>
}

function compactText(value: string) {
  return (
    <span className="block max-w-[220px] truncate text-xs text-[var(--color-text-muted)]" title={value || '-'}>
      {value || '-'}
    </span>
  )
}

export function OperationTaskCenterPage() {
  const [searchParams] = useSearchParams()
  const [tasks, setTasks] = useState<OperationTask[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const shopId = searchParams.get('shopId')?.trim() || ''
  const counts = useMemo(() => deriveOperationTaskCounts(tasks), [tasks])

  async function load(silent = false) {
    if (silent) setRefreshing(true)
    else setLoading(true)
    try {
      setTasks(await fetchOperationTasks({ shopId }))
    } catch (error) {
      console.error('load operation tasks failed', error)
      toast.error('加载运营任务失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [shopId])

  const columns: TableColumn<OperationTask>[] = [
    {
      key: 'title',
      title: '任务',
      width: 260,
      render: (value, row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <div className="max-w-[240px] truncate text-sm font-medium text-[var(--color-text-primary)]" title={String(value || row.taskId)}>
            {String(value || row.taskId || '-')}
          </div>
          <div className="max-w-[240px] truncate text-xs text-[var(--color-text-muted)]" title={row.taskType}>
            {row.taskType || '-'}
          </div>
        </div>
      ),
    },
    {
      key: 'shopName',
      title: '店铺',
      width: 220,
      render: (value, row) => (
        <span className="block max-w-[200px] truncate" title={String(value || row.shopId || '-')}>
          {String(value || row.shopId || '-')}
        </span>
      ),
    },
    { key: 'status', title: '状态', width: 120, render: (value) => statusBadge(String(value || 'waiting') as OperationTaskStatus) },
    { key: 'blockedReason', title: '阻塞原因', render: (value) => compactText(String(value || '')) },
    { key: 'updatedAt', title: '更新时间', width: 180, render: (value) => compactText(String(value || '')) },
  ]

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">运营任务</h1>
          <p className="mt-1 break-words text-sm text-[var(--color-text-muted)]">
            {shopId ? `当前仅看店铺 ${shopId} 的运营任务。` : '跨店铺任务视图，按状态查看等待、执行、阻塞和失败任务。'}
          </p>
        </div>
        <Button className="w-full shrink-0 sm:w-auto" variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      {shopId ? (
        <Alert
          type="info"
          title="已进入店铺级任务视图"
          message="这里展示该店铺关联的运营任务。若没有任务，说明当前服务端还没有为该店铺生成待执行事项。"
        />
      ) : null}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard title="总任务" value={loading ? '-' : counts.total} icon={<ListChecks className="h-5 w-5" />} />
        <StatCard title="等待中" value={loading ? '-' : counts.waiting} icon={<Badge variant="default">W</Badge>} />
        <StatCard title="运行中" value={loading ? '-' : counts.running} icon={<Badge variant="info">R</Badge>} />
        <StatCard title="阻塞" value={loading ? '-' : counts.blocked} icon={<Badge variant="warning">B</Badge>} />
        <StatCard title="失败" value={loading ? '-' : counts.failed} icon={<Badge variant="error">F</Badge>} />
      </div>

      <Card padding="none">
        <Table
          columns={columns}
          data={tasks}
          rowKey="taskId"
          loading={loading}
          emptyText={shopId ? '该店铺暂无运营任务' : '暂无运营任务'}
          maxHeight="calc(100vh - 340px)"
        />
      </Card>
    </div>
  )
}
