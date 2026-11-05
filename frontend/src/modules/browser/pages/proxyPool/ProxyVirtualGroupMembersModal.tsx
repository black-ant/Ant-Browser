import { useMemo } from 'react'
import { Modal, Table } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { ProxyVirtualGroupHealth } from '../../types'
import type { ProxyDisplayInfo } from './helpers'

interface ProxyVirtualGroupMembersModalProps {
  open: boolean
  groupName: string
  strategy?: string
  members: ProxyDisplayInfo[]
  health?: ProxyVirtualGroupHealth | null
  onClose: () => void
}

interface MemberStateMeta {
  label: string
  className: string
}

const memberStateMeta: Record<string, MemberStateMeta> = {
  healthy: { label: '健康', className: 'text-emerald-500' },
  suspect: { label: '可疑', className: 'text-amber-500' },
  degraded: { label: '已降级', className: 'text-red-500' },
  recovering: { label: '恢复中', className: 'text-sky-500' },
  unknown: { label: '未观测', className: 'text-[var(--color-text-muted)]' },
}

function getStrategyLabel(strategy?: string) {
  return strategy === 'sticky' ? '代理保持' : '绝对轮询'
}

function renderLatency(value: number) {
  if (typeof value !== 'number' || value < 0) {
    return <span className='text-xs text-[var(--color-text-muted)]'>-</span>
  }
  const color = value < 200 ? 'text-emerald-500' : value < 500 ? 'text-amber-500' : 'text-red-500'
  return <span className={`text-xs font-medium ${color}`}>{value} ms</span>
}

export function ProxyVirtualGroupMembersModal({
  open,
  groupName,
  strategy,
  members,
  health,
  onClose,
}: ProxyVirtualGroupMembersModalProps) {
  const healthByProxyId = useMemo(() => {
    const map = new Map<string, ProxyVirtualGroupHealth['members'][number]>()
    for (const item of health?.members || []) {
      map.set(item.proxyId.trim().toLowerCase(), item)
    }
    return map
  }, [health])

  const columns = useMemo<TableColumn<ProxyDisplayInfo>[]>(() => [
    {
      key: 'proxyName',
      title: '代理名称',
      render: (_, record) => (
        <div className='min-w-0'>
          <div className='truncate font-medium text-[var(--color-text-primary)]' title={record.proxyName || record.proxyId}>
            {record.proxyName || record.proxyId}
          </div>
          <div className='truncate text-[11px] text-[var(--color-text-muted)]' title={record.proxyId}>
            {record.proxyId}
          </div>
        </div>
      ),
    },
    {
      key: 'state',
      title: '状态',
      width: '96px',
      render: (_, record) => {
        const item = healthByProxyId.get(record.proxyId.trim().toLowerCase())
        const meta = item ? memberStateMeta[item.state] || memberStateMeta.unknown : memberStateMeta.unknown
        return (
          <span
            className={`text-xs font-medium ${meta.className}`}
            title={item?.lastError || (item?.lastProbeAt ? `最后探测：${item.lastProbeAt}` : '尚未观测')}
          >
            {meta.label}
          </span>
        )
      },
    },
    {
      key: 'latency',
      title: '延迟',
      width: '80px',
      render: (_, record) => {
        const item = healthByProxyId.get(record.proxyId.trim().toLowerCase())
        return renderLatency(item?.latencyMs ?? -1)
      },
    },
    {
      key: 'active',
      title: '当前使用',
      width: '104px',
      render: (_, record) => {
        const item = healthByProxyId.get(record.proxyId.trim().toLowerCase())
        if (item?.active) {
          return <span className='rounded bg-[var(--color-accent)]/12 px-1.5 py-0.5 text-[11px] text-[var(--color-accent)]'>当前</span>
        }
        if (item?.preferred) {
          return <span className='rounded bg-[var(--color-bg-muted)] px-1.5 py-0.5 text-[11px] text-[var(--color-text-secondary)]'>偏好</span>
        }
        return <span className='text-xs text-[var(--color-text-muted)]'>-</span>
      },
    },
    {
      key: 'groupName',
      title: '所属分组',
      width: '140px',
      render: (_, record) => (
        <span className='block truncate text-xs text-[var(--color-text-secondary)]' title={record.groupName || '未分组'}>
          {record.groupName || '未分组'}
        </span>
      ),
    },
    {
      key: 'type',
      title: '类型',
      width: '88px',
      render: (_, record) => (record.type ? record.type.toUpperCase() : '-'),
    },
    {
      key: 'server',
      title: '服务器',
      width: '180px',
      render: (_, record) => (
        <span className='block truncate' title={record.server || '-'}>
          {record.server || '-'}
        </span>
      ),
    },
    {
      key: 'port',
      title: '端口',
      width: '76px',
      render: (_, record) => record.port || '-',
    },
  ], [healthByProxyId])

  const usable = health ? health.healthy + health.suspect : 0
  const total = health?.total ?? members.length

  return (
    <Modal open={open} onClose={onClose} title='关联代理节点' width='920px'>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <span className='min-w-0 truncate font-medium text-[var(--color-text-primary)]' title={groupName}>
            {groupName || '虚拟分组'}
          </span>
          <span className='shrink-0 text-xs text-[var(--color-text-muted)]'>
            {getStrategyLabel(strategy)} · 可用 {usable}/{total}
            {health?.allDegraded ? ' · 全部降级' : ''}
          </span>
        </div>
        {members.length === 0 ? (
          <div className='rounded-lg border border-dashed border-[var(--color-border-default)] px-4 py-10 text-center text-sm text-[var(--color-text-muted)]'>
            未选择节点
          </div>
        ) : (
          <Table
            columns={columns}
            data={members}
            rowKey='proxyId'
            tableLayout='fixed'
            tableMinWidth='830px'
            maxHeight='min(60vh, 520px)'
            emptyText='未选择节点'
          />
        )}
      </div>
    </Modal>
  )
}
