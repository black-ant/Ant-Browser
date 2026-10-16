import { Modal, Table } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { ProxyDisplayInfo } from './helpers'

interface ProxyVirtualGroupMembersModalProps {
  open: boolean
  groupName: string
  strategy?: string
  members: ProxyDisplayInfo[]
  onClose: () => void
}

const columns: TableColumn<ProxyDisplayInfo>[] = [
  {
    key: 'proxyName',
    title: '代理名称',
    width: '260px',
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
    key: 'groupName',
    title: '所属分组',
    width: '160px',
    render: (_, record) => record.groupName || '未分组',
  },
  {
    key: 'type',
    title: '类型',
    width: '100px',
    render: (_, record) => record.type ? record.type.toUpperCase() : '-',
  },
  {
    key: 'server',
    title: '服务器',
    width: '220px',
    render: (_, record) => (
      <span className='block truncate' title={record.server || '-'}>
        {record.server || '-'}
      </span>
    ),
  },
  {
    key: 'port',
    title: '端口',
    width: '90px',
    render: (_, record) => record.port || '-',
  },
]

function getStrategyLabel(strategy?: string) {
  return strategy === 'sticky' ? '代理保持' : '绝对轮询'
}

export function ProxyVirtualGroupMembersModal({
  open,
  groupName,
  strategy,
  members,
  onClose,
}: ProxyVirtualGroupMembersModalProps) {
  return (
    <Modal open={open} onClose={onClose} title='关联代理节点' width='920px'>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <span className='min-w-0 truncate font-medium text-[var(--color-text-primary)]' title={groupName}>
            {groupName || '虚拟分组'}
          </span>
          <span className='shrink-0 text-xs text-[var(--color-text-muted)]'>
            {getStrategyLabel(strategy)} · {members.length} 个节点
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
