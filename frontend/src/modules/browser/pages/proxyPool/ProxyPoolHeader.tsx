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
      <div className="flex flex-col gap-3 px-4 py-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">代理池配置</h1>
          <div className="flex items-center gap-2 rounded-lg bg-[var(--color-bg-muted)] px-2 py-1">
            <span className="inline-flex items-center gap-1 whitespace-nowrap px-2 text-xs text-[var(--color-text-muted)]">
              内核状态：{currentConnectorStatus || '未知'}
            </span>
            <Button size="sm" variant="secondary" onClick={onOpenCoreDownload}>下载内核</Button>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2 xl:justify-end">
          <Button
            size="sm"
            variant="secondary"
            onClick={onOpenSettings}
          >
            检测设置
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={onRefreshAllSources}
            loading={refreshingAllSources}
            disabled={!hasURLImportSources}
          >
            刷新订阅
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={onCheckAllIPHealth}
            loading={checkingAllIPHealth}
            disabled={testableCount === 0}
          >
            检测IP健康
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={onTestAll}
            loading={testingAll}
            disabled={testableCount === 0}
          >
            测试全部
          </Button>
          <Button size="sm" variant="secondary" onClick={onOpenUsageGuide}>
            使用说明
          </Button>
          <Button size='sm' variant='secondary' onClick={onOpenVirtualGroups}>虚拟节点</Button>
          <Button size="sm" variant="secondary" onClick={onOpenImport}>导入代理</Button>
        </div>
      </div>
    </Card>
  )
}
