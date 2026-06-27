import { Link } from 'react-router-dom'
import { Play, Square, RotateCcw, Key, Settings, Copy, Trash2, AlertCircle, WifiOff, Users, Tag } from 'lucide-react'
import { Badge, Button } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { BrowserProfile, BrowserProxy } from '../../types'
import { LaunchCodeCell } from './LaunchCodeCell'
import { formatTime } from './helpers'

interface ColumnConfig {
  proxies: BrowserProxy[]
  getProfileCoreLabel: (record: BrowserProfile) => string
  getProfileStatus: (record: BrowserProfile) => { label: string; variant: any }
  isProfileStarting: (id: string) => boolean
  isProfileStopping: (id: string) => boolean
  isProfileBusy: (id: string) => boolean
  onStart: (id: string) => void
  onStop: (id: string) => void
  onRestart: (id: string) => void
  onKeywords: (record: BrowserProfile) => void
  onCopy: (record: BrowserProfile) => void
  onDelete: (id: string) => void
  onRefresh: () => void
  onToggleSelect: (id: string) => void
  selectedIds: Set<string>
  onSelectAll: () => void
  onDeselectAll: () => void
  totalCount: number
}

export function createEnhancedColumns(config: ColumnConfig): TableColumn<BrowserProfile>[] {
  const actionButtonClass = '!h-8 !w-8 !p-0 rounded-lg text-[var(--color-text-secondary)] hover:text-[var(--color-accent)] hover:bg-[var(--color-accent)]/10'
  const actionIconClass = 'h-4 w-4'

  return [
    // 复选框列
    {
      key: 'selection',
      title: (
        <input
          type="checkbox"
          className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)]"
          checked={config.selectedIds.size > 0 && config.selectedIds.size === config.totalCount}
          ref={(input) => {
            if (input) input.indeterminate = config.selectedIds.size > 0 && config.selectedIds.size < config.totalCount
          }}
          onChange={(e) => {
            if (e.target.checked) config.onSelectAll()
            else config.onDeselectAll()
          }}
        />
      ),
      width: 48,
      render: (_, record) => (
        <input
          type="checkbox"
          className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)]"
          checked={config.selectedIds.has(record.profileId)}
          onChange={() => config.onToggleSelect(record.profileId)}
          onClick={(e) => e.stopPropagation()}
        />
      ),
    },
    // 窗口名称 + 标签
    {
      key: 'profileName',
      title: '窗口名称',
      width: 120,
      render: (value, record) => (
        <div className="flex min-w-0 flex-col gap-1">
          <Link
            className="block max-w-[110px] truncate text-sm font-medium text-[var(--color-accent)] hover:underline"
            to={`/browser/detail/${record.profileId}`}
            title={value}
          >
            {value}
          </Link>
          {/* 标签 */}
          {record.tags && record.tags.length > 0 && (
            <div className="flex gap-1 flex-wrap">
              {record.tags.slice(0, 3).map(tag => (
                <Badge key={tag} variant="default" size="sm" outline>
                  <Tag className="w-2.5 h-2.5" />
                  {tag}
                </Badge>
              ))}
              {record.tags.length > 3 && (
                <Badge variant="default" size="sm" outline>
                  +{record.tags.length - 3}
                </Badge>
              )}
            </div>
          )}
          {/* 账号信息 */}
          {record.accountIds && record.accountIds.length > 0 && (
            <Badge variant="info" size="sm" outline>
              <Users className="w-2.5 h-2.5" />
              {record.accountIds.length} 个账号
            </Badge>
          )}
        </div>
      ),
    },
    // 状态
    {
      key: 'status',
      title: '状态',
      width: 96,
      render: (_, record) => {
        const status = config.getProfileStatus(record)
        const isStarting = config.isProfileStarting(record.profileId)
        const isStopping = config.isProfileStopping(record.profileId)

        if (isStarting) {
          return <Badge variant="info" dot pulse>启动中</Badge>
        }
        if (isStopping) {
          return <Badge variant="default" dot pulse>停止中</Badge>
        }

        return <Badge variant={status.variant} dot pulse={record.running}>{status.label}</Badge>
      },
    },
    // 代理状态
    {
      key: 'proxyStatus',
      title: '代理',
      width: 150,
      render: (_, record) => {
        const proxy = config.proxies.find(p => p.proxyId === record.proxyId)
        const hasProxyError = record.runtimeWarning && record.runtimeWarning.includes('代理')

        return (
          <div className="flex min-w-0 flex-col gap-1">
            {proxy ? (
              <span className="block max-w-[130px] truncate text-xs text-[var(--color-text-secondary)]" title={proxy.proxyName}>
                {proxy.proxyName}
              </span>
            ) : (
              <span className="block max-w-[130px] truncate text-xs text-[var(--color-text-muted)]" title={record.proxyId || '无代理'}>
                {record.proxyId || '无代理'}
              </span>
            )}
            {hasProxyError && (
              <Badge variant="warning" size="sm">
                <WifiOff className="w-2.5 h-2.5" />
                代理异常
              </Badge>
            )}
          </div>
        )
      },
    },
    // 内核
    {
      key: 'coreId',
      title: '内核',
      width: 220,
      render: (_, record) => (
        <span className="block max-w-[210px] truncate text-xs text-[var(--color-text-secondary)]" title={config.getProfileCoreLabel(record)}>
          {config.getProfileCoreLabel(record)}
        </span>
      ),
    },
    // 运行时提示
    {
      key: 'runtime',
      title: '运行时提示',
      width: 130,
      render: (_, record) => {
        if (record.lastError) {
          return (
            <div className="flex min-w-0 items-center gap-1.5">
              <AlertCircle className="h-3.5 w-3.5 flex-shrink-0 text-[var(--color-error)]" />
              <span className="block max-w-[104px] truncate text-xs text-[var(--color-error)]" title={record.lastError}>
                {record.lastError}
              </span>
            </div>
          )
        }
        if (record.runtimeWarning) {
          return (
            <div className="flex min-w-0 items-center gap-1.5">
              <AlertCircle className="h-3.5 w-3.5 flex-shrink-0 text-[var(--color-warning)]" />
              <span className="block max-w-[104px] truncate text-xs text-[var(--color-warning)]" title={record.runtimeWarning}>
                {record.runtimeWarning}
              </span>
            </div>
          )
        }
        if (record.running && record.debugReady) {
          return (
            <span className="text-xs text-[var(--color-success)]">
              ✓ 就绪
            </span>
          )
        }
        return <span className="text-xs text-[var(--color-text-muted)]">-</span>
      },
    },
    // 快捷打开码
    {
      key: 'launchCode',
      title: '快捷码',
      width: 160,
      render: (value, record) => (
        <LaunchCodeCell
          profileId={record.profileId}
          code={value || ''}
          onRefresh={config.onRefresh}
        />
      ),
    },
    // 最近更新
    {
      key: 'updatedAt',
      title: '最近更新',
      width: 150,
      render: (value) => (
        <span className="block max-w-[132px] truncate text-xs text-[var(--color-text-secondary)]" title={formatTime(value)}>
          {formatTime(value)}
        </span>
      ),
    },
    // 操作列 - 紧凑图标按钮
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      width: 240,
      render: (_, record) => {
        const isStarting = config.isProfileStarting(record.profileId)
        const isStopping = config.isProfileStopping(record.profileId)
        const isBusy = config.isProfileBusy(record.profileId)

        return (
          <div className="flex items-center justify-end gap-1.5" onClick={(e) => e.stopPropagation()}>
            {/* 启动/停止 */}
            {record.running ? (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => config.onStop(record.profileId)}
                title="停止"
                loading={isStopping}
                className={actionButtonClass}
              >
                {!isStopping && <Square className={actionIconClass} />}
              </Button>
            ) : (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => config.onStart(record.profileId)}
                title="启动"
                loading={isStarting}
                className={`${actionButtonClass} text-[var(--color-success)] hover:text-[var(--color-success)] hover:bg-[var(--color-success)]/10`}
              >
                {!isStarting && <Play className={`${actionIconClass} fill-current`} />}
              </Button>
            )}
            {/* 重启 */}
            <Button
              size="sm"
              variant="ghost"
              onClick={() => config.onRestart(record.profileId)}
              title="重启"
              disabled={isBusy}
              className={actionButtonClass}
            >
              <RotateCcw className={actionIconClass} />
            </Button>
            {/* 关键字 */}
            <Button
              size="sm"
              variant="ghost"
              onClick={() => config.onKeywords(record)}
              title="关键字"
              disabled={isBusy}
              className={actionButtonClass}
            >
              <Key className={actionIconClass} />
            </Button>
            {/* 配置 */}
            <Link to={`/browser/edit/${record.profileId}`}>
              <Button
                size="sm"
                variant="ghost"
                title="配置"
                disabled={isBusy}
                className={actionButtonClass}
              >
                <Settings className={actionIconClass} />
              </Button>
            </Link>
            {/* 克隆 */}
            <Button
              size="sm"
              variant="ghost"
              onClick={() => config.onCopy(record)}
              title="克隆"
              disabled={isBusy}
              className={actionButtonClass}
            >
              <Copy className={actionIconClass} />
            </Button>
            {/* 删除 */}
            <Button
              size="sm"
              variant="ghost"
              onClick={() => config.onDelete(record.profileId)}
              title="删除"
              disabled={isBusy}
              className={`${actionButtonClass} text-[var(--color-error)] hover:text-[var(--color-error)] hover:bg-[var(--color-error)]/10`}
            >
              <Trash2 className={actionIconClass} />
            </Button>
          </div>
        )
      },
    },
  ]
}
