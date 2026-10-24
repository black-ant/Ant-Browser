import { Button, Card } from '../../../../shared/components'

interface ProxyPoolHeaderProps {
  checkingAllIPHealth: boolean
  currentConnectorStatus: string
  hasURLImportSources: boolean
  onCheckAllIPHealth: () => void
  onOpenImport: () => void
  onOpenVirtualGroups: () => void
  onOpenCoreDownload: () => void
  onOpenSettings: () => void
  onOpenUsageGuide: () => void
  onRefreshAllSources: () => void
  onTestAll: () => void
  refreshingAllSources: boolean
  testingAll: boolean
  testableCount: number
}

export function ProxyPoolHeader({
  checkingAllIPHealth,
  currentConnectorStatus,
  hasURLImportSources,
  onCheckAllIPHealth,
  onOpenImport,
  onOpenVirtualGroups,
  onOpenCoreDownload,
  onOpenSettings,
  onOpenUsageGuide,
  onRefreshAllSources,
  onTestAll,
  refreshingAllSources,
  testingAll,
  testableCount,
}: ProxyPoolHeaderProps) {
  return (
    <Card padding="none" className="shadow-[var(--shadow-sm)]">
      <div className="flex flex-row flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2.5">
        <h1 className="text-lg font-semibold text-[var(--color-text-primary)]">代理池配置</h1>
        <div className="flex items-center gap-2 rounded-lg bg-[var(--color-bg-muted)] px-2 py-0.5">
          <span className="inline-flex items-center gap-1 whitespace-nowrap px-2 text-xs text-[var(--color-text-muted)]">
            内核状态：{currentConnectorStatus || '未知'}
          </span>
          <Button size="sm" variant="secondary" className="!rounded-[4px]" onClick={onOpenCoreDownload}>下载内核</Button>
        </div>
        <div className="flex-1" />
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="secondary"
            className="!rounded-[4px]"
            onClick={onOpenSettings}
          >
            检测设置
          </Button>
          <Button
            size="sm"
            variant="secondary"
            className="!rounded-[4px]"
            onClick={onRefreshAllSources}
            loading={refreshingAllSources}
            disabled={!hasURLImportSources}
          >
            刷新订阅
          </Button>
          <Button
            size="sm"
            variant="secondary"
            className="!rounded-[4px]"
            onClick={onCheckAllIPHealth}
            loading={checkingAllIPHealth}
            disabled={testableCount === 0}
          >
            检测IP健康
          </Button>
          <Button
            size="sm"
            variant="secondary"
            className="!rounded-[4px]"
            onClick={onTestAll}
            loading={testingAll}
            disabled={testableCount === 0}
          >
            测试全部
          </Button>
          <Button size="sm" variant="secondary" className="!rounded-[4px]" onClick={onOpenUsageGuide}>
            使用说明
          </Button>
          <Button size='sm' variant='secondary' className='!rounded-[4px]' onClick={onOpenVirtualGroups}>虚拟节点</Button>
          <Button size="sm" variant="secondary" className="!rounded-[4px]" onClick={onOpenImport}>导入代理</Button>
        </div>
      </div>
    </Card>
  )
}
