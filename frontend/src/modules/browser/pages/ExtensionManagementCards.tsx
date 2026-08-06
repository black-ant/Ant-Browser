import { AlertCircle, CheckCircle2, Download, ExternalLink, FolderOpen, History, Power, Puzzle, RefreshCw, RotateCw, Search, Settings, Trash2, Users } from 'lucide-react'
import { Badge, Button, Card, Input } from '../../../shared/components'
import type { BrowserExtension, BrowserExtensionLookupResult, BrowserProxy } from '../types'
import { extensionStoreURL, formatExtensionSource, formatExtensionTime, getExtensionManifestMeta, getProxySpeedState } from './extensionManagementUtils'

export function ProxyStatePill({ useProxy, proxy }: { useProxy: boolean; proxy?: BrowserProxy }) {
  if (!useProxy) {
    return <span className="rounded-full bg-[var(--color-bg-muted)] px-2 py-0.5 text-xs text-[var(--color-text-muted)]">直连下载</span>
  }
  if (!proxy) {
    return <span className="rounded-full bg-red-50 px-2 py-0.5 text-xs text-red-600">代理未选择</span>
  }
  const state = getProxySpeedState(proxy)
  const status = state?.ok ? `${state.latencyMs}ms` : state ? '不可用' : '未测试'
  return (
    <span className="rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-700">
      使用代理：{proxy.proxyName || proxy.proxyId} · {status}
    </span>
  )
}


export interface ExtensionManagementHeaderProps {
  proxyButtonText: string
  loading: boolean
  importing: 'none' | 'file' | 'directory'
  downloadDirectoryLoading: boolean
  onOpenProxy: () => void
  onOpenHistory: () => void
  onImportFile: () => void
  onImportDirectory: () => void
  onOpenDownloadDirectory: () => void
  onRefresh: () => void
}

export function ExtensionManagementHeader({
  proxyButtonText,
  loading,
  importing,
  downloadDirectoryLoading,
  onOpenProxy,
  onOpenHistory,
  onImportFile,
  onImportDirectory,
  onOpenDownloadDirectory,
  onRefresh,
}: ExtensionManagementHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-3">
      <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">插件包管理</h1>
      <div className="flex gap-2">
        <Button size="sm" variant="secondary" onClick={onOpenProxy}>
          <Settings className="h-4 w-4" />
          {proxyButtonText}
        </Button>
        <Button size="sm" variant="secondary" onClick={onOpenHistory}>
          <History className="h-4 w-4" />
          历史
        </Button>
        <Button size="sm" variant="secondary" onClick={onImportFile} loading={importing === 'file'}>
          <Download className="h-4 w-4" />
          导入包
        </Button>
        <Button size="sm" variant="secondary" onClick={onImportDirectory} loading={importing === 'directory'}>
          <Download className="h-4 w-4" />
          导入目录
        </Button>
        <Button size="sm" variant="secondary" onClick={onOpenDownloadDirectory} loading={downloadDirectoryLoading}>
          <FolderOpen className="h-4 w-4" />
          下载目录安装
        </Button>
        <Button size="sm" variant="secondary" onClick={onRefresh} loading={loading}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>
    </div>
  )
}

export interface ExtensionInstallCardProps {
  query: string
  lookup: BrowserExtensionLookupResult | null
  querying: boolean
  installing: boolean
  useProxy: boolean
  selectedProxy?: BrowserProxy
  installedIds: Set<string>
  lastLookupProxyLabel: string
  onQueryChange: (value: string) => void
  onLookup: () => void
  onOpenWebStoreQuery: () => void
  onOpenManualInstall: () => void
  onOpenProxy: () => void
  onInstall: () => void
}

export function ExtensionInstallCard({
  query,
  lookup,
  querying,
  installing,
  useProxy,
  selectedProxy,
  installedIds,
  lastLookupProxyLabel,
  onQueryChange,
  onLookup,
  onOpenWebStoreQuery,
  onOpenManualInstall,
  onOpenProxy,
  onInstall,
}: ExtensionInstallCardProps) {
  const lookupSucceeded = lookup?.installable === true

  return (
    <Card>
      <div className="flex flex-col gap-3 md:flex-row">
        <Input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') onLookup()
          }}
          placeholder="Chrome Web Store 链接或 32 位插件 ID"
          className="flex-1"
        />
        <Button type="button" variant="secondary" onClick={onLookup} loading={querying}>
          <Search className="h-4 w-4" />
          查询
        </Button>
        <Button type="button" variant="secondary" onClick={onOpenWebStoreQuery}>
          <ExternalLink className="h-4 w-4" />
          网页查询
        </Button>
        <Button type="button" variant="secondary" onClick={onOpenManualInstall}>
          <Download className="h-4 w-4" />
          手动安装
        </Button>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <ProxyStatePill useProxy={useProxy} proxy={selectedProxy} />
        <Button type="button" size="sm" variant="ghost" onClick={onOpenProxy}>
          切换代理
        </Button>
      </div>

      {lookup ? (
        <div className="mt-3 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-4">
          <div className="flex items-start gap-3">
            <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${lookupSucceeded ? 'bg-[var(--color-success)]/15 text-[var(--color-success)]' : 'bg-[var(--color-error)]/15 text-[var(--color-error)]'}`}>
              {lookupSucceeded ? <CheckCircle2 className="h-5 w-5" /> : <AlertCircle className="h-5 w-5" />}
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-semibold text-[var(--color-text-primary)]">
                  {lookupSucceeded ? (lookup.name || '已找到插件') : '无法获取可安装信息'}
                </span>
                <Badge variant={lookupSucceeded ? 'success' : 'error'} size="sm" dot>
                  {lookupSucceeded ? '可安装' : '查询失败'}
                </Badge>
                {lookup.version ? <span className="text-xs text-[var(--color-text-muted)]">版本 {lookup.version}</span> : null}
              </div>
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--color-text-muted)]">
                <span className="min-w-0 break-all">
                  <span className="mr-1 text-[var(--color-text-secondary)]">插件 ID</span>
                  <span className="font-mono">{lookup.extensionId || '未识别'}</span>
                </span>
                {lastLookupProxyLabel ? <span>查询方式：{lastLookupProxyLabel}</span> : null}
              </div>
            </div>
          </div>

          {lookup.description ? (
            <div className="mt-4 border-t border-[var(--color-border-muted)] pt-3">
              <div className="text-xs font-medium text-[var(--color-text-secondary)]">插件简介</div>
              <div className="mt-1 line-clamp-2 text-sm leading-5 text-[var(--color-text-muted)]">{lookup.description}</div>
            </div>
          ) : null}

          {lookup.message ? (
            <div className={`mt-4 rounded-lg border px-3 py-2.5 ${lookupSucceeded ? 'border-[var(--color-border-default)] bg-[var(--color-bg-muted)]' : 'border-[var(--color-error)]/25 bg-[var(--color-error)]/10'}`}>
              <div className={`text-xs font-medium ${lookupSucceeded ? 'text-[var(--color-text-secondary)]' : 'text-[var(--color-error)]'}`}>
                {lookupSucceeded ? '查询提示' : '失败原因'}
              </div>
              <div className="mt-1 break-all text-xs leading-5 text-[var(--color-text-secondary)]">{lookup.message}</div>
            </div>
          ) : null}

          <div className="mt-4 flex flex-wrap justify-end gap-2 border-t border-[var(--color-border-muted)] pt-3">
            {lookup.storeUrl ? (
              <Button type="button" size="sm" variant="secondary" onClick={() => window.open(lookup.storeUrl, '_blank')}>
                <ExternalLink className="h-4 w-4" />
                打开商店页
              </Button>
            ) : null}
            {lookupSucceeded ? (
              <Button
                type="button"
                size="sm"
                onClick={onInstall}
                loading={installing}
                disabled={installedIds.has(lookup.extensionId)}
              >
                <Download className="h-4 w-4" />
                {installedIds.has(lookup.extensionId) ? '已安装' : '安装插件'}
              </Button>
            ) : (
              <Button type="button" size="sm" onClick={onLookup} loading={querying}>
                <RefreshCw className="h-4 w-4" />
                重新查询
              </Button>
            )}
            <Button type="button" size="sm" variant="secondary" onClick={onOpenManualInstall}>
              手动安装
            </Button>
          </div>
        </div>
      ) : null}
    </Card>
  )
}

export interface InstalledExtensionsListProps {
  items: BrowserExtension[]
  busyId: string
  busyAction: 'toggle' | 'delete' | ''
  updatingId: string
  onRestrictProfiles: (item: BrowserExtension) => void
  onUpdate: (item: BrowserExtension) => void
  onToggle: (item: BrowserExtension) => void
  onDelete: (item: BrowserExtension) => void
}

export function InstalledExtensionsList({ items, busyId, busyAction, updatingId, onRestrictProfiles, onUpdate, onToggle, onDelete }: InstalledExtensionsListProps) {
  return (
    <Card>
      <div className="mb-3 flex items-center justify-between">
        <div className="text-sm font-medium text-[var(--color-text-primary)]">已安装插件（{items.length}）</div>
      </div>
      <div className="space-y-2">
        {items.map((item) => (
          <InstalledExtensionCard
            key={item.extensionId}
            item={item}
            busy={busyId === item.extensionId}
            busyAction={busyId === item.extensionId ? busyAction : ''}
            updating={updatingId === item.extensionId}
            onRestrictProfiles={onRestrictProfiles}
            onUpdate={onUpdate}
            onToggle={onToggle}
            onDelete={onDelete}
          />
        ))}

        {items.length === 0 ? (
          <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-8 text-center text-sm text-[var(--color-text-muted)]">
            暂无插件，先通过上方输入插件 ID 或商店链接安装。
          </div>
        ) : null}
      </div>
    </Card>
  )
}

export interface InstalledExtensionCardProps {
  item: BrowserExtension
  busy: boolean
  busyAction: 'toggle' | 'delete' | ''
  updating: boolean
  onRestrictProfiles: (item: BrowserExtension) => void
  onUpdate: (item: BrowserExtension) => void
  onToggle: (item: BrowserExtension) => void
  onDelete: (item: BrowserExtension) => void
}

export function InstalledExtensionCard({ item, busy, busyAction, updating, onRestrictProfiles, onUpdate, onToggle, onDelete }: InstalledExtensionCardProps) {
  const meta = getExtensionManifestMeta(item)
  const storeUrl = extensionStoreURL(item)
  const actionButtonClass = 'min-w-[72px] will-change-transform hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.98]'
  return (
    <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-3 shadow-[var(--shadow-xs)]">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 gap-3">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)]">
            {item.iconDataUrl ? (
              <img src={item.iconDataUrl} alt="" className="h-9 w-9 object-contain" />
            ) : (
              <Puzzle className="h-6 w-6 text-[var(--color-text-muted)]" />
            )}
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-[var(--color-text-primary)]">{item.name || item.extensionId}</span>
              <Badge variant={item.enabled ? 'success' : 'error'} size="sm" dot>
                {item.enabled ? '已启用' : '已停用'}
              </Badge>
              <Badge variant={item.installMode === 'commandline' ? 'warning' : 'info'} size="sm">
                {item.installMode === 'commandline' ? '开发目录' : '生产持久'}
              </Badge>
              {item.version ? <span className="text-xs text-[var(--color-text-muted)]">v{item.version}</span> : null}
              {meta.manifestVersion ? <span className="text-xs text-[var(--color-text-muted)]">MV{meta.manifestVersion}</span> : null}
            </div>
            {item.description ? <div className="mt-1 line-clamp-2 text-sm text-[var(--color-text-secondary)]">{item.description}</div> : null}
            <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--color-text-muted)]">
              <span className="break-all font-mono">{item.extensionId}</span>
              <span>{formatExtensionSource(item.sourceUrl)}</span>
              <span>安装：{formatExtensionTime(item.installedAt)}</span>
              {item.updatedAt ? <span>更新：{formatExtensionTime(item.updatedAt)}</span> : null}
            </div>
            {meta.permissions.length > 0 || meta.hostPermissionCount > 0 ? (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {meta.permissions.map((permission) => (
                  <span key={permission} className="rounded-full bg-[var(--color-bg-muted)] px-2 py-0.5 text-xs text-[var(--color-text-muted)]">{permission}</span>
                ))}
                {meta.hostPermissionCount > 0 ? <span className="rounded-full bg-[var(--color-bg-muted)] px-2 py-0.5 text-xs text-[var(--color-text-muted)]">站点权限 {meta.hostPermissionCount}</span> : null}
              </div>
            ) : null}
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          {storeUrl ? (
            <Button type="button" size="sm" variant="secondary" onClick={() => window.open(storeUrl, '_blank')} className={actionButtonClass}>
              <ExternalLink className="h-4 w-4" />
              商店
            </Button>
          ) : null}
          <Button type="button" size="sm" variant="secondary" onClick={() => onRestrictProfiles(item)} className="min-w-[96px] will-change-transform hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.98]">
            <Users className="h-4 w-4" />
            限制实例
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={() => onUpdate(item)} disabled={updating} className={actionButtonClass}>
            <RotateCw className={`h-4 w-4 ${updating ? 'animate-spin' : ''}`} />
            更新
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={() => onToggle(item)} disabled={busy} className={actionButtonClass}>
            <Power className={`h-4 w-4 ${busy && busyAction === 'toggle' ? 'animate-pulse' : ''}`} />
            {item.enabled ? '停用' : '启用'}
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={() => onDelete(item)} disabled={busy} className={actionButtonClass}>
            <Trash2 className={`h-4 w-4 ${busy && busyAction === 'delete' ? 'animate-pulse' : ''}`} />
            删除
          </Button>
        </div>
      </div>
    </div>
  )
}
