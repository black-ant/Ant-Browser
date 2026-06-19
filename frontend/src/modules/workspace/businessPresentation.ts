import type { WorkspaceAuthorizedShop, WorkspaceSummary } from './types'

export type ExecutionBlockerKind =
  | 'workspace-host'
  | 'maka-runtime'
  | 'session'
  | 'credential'
  | 'profile'
  | 'core'

export interface ExecutionBlocker {
  kind: ExecutionBlockerKind
  title: string
  message: string
  actionLabel: string
  actionPath: string
}

export function makaRuntimeReachable(summary: WorkspaceSummary) {
  return summary.antRuntimeReachable
}

export function workspaceHealthLabel(summary: WorkspaceSummary) {
  if (!summary.serverReachable) return 'Workspace Host 不可达'
  if (!makaRuntimeReachable(summary)) return 'Maka Runtime 未连通'
  if (!summary.sessionReady) return '桌面会话未就绪'
  return summary.deviceStatus || summary.agentStatus || summary.status || 'ready'
}

export function workspaceHealthTone(summary: WorkspaceSummary) {
  if (!summary.serverReachable || !makaRuntimeReachable(summary)) {
    return 'text-amber-600 bg-amber-50 border-amber-200'
  }
  if (!summary.sessionReady) {
    return 'text-slate-600 bg-slate-100 border-slate-200'
  }
  return 'text-emerald-600 bg-emerald-50 border-emerald-200'
}

export function readyRate(total: number, ready: number) {
  if (!Number.isFinite(total) || total <= 0) return '0%'

  const finiteReady = Number.isFinite(ready) ? ready : 0
  const clampedReady = Math.min(Math.max(finiteReady, 0), total)
  return `${Math.round((clampedReady / total) * 100)}%`
}

export function deriveExecutionBlockers(summary: WorkspaceSummary, shops: WorkspaceAuthorizedShop[]): ExecutionBlocker[] {
  const blockers: ExecutionBlocker[] = []

  if (!summary.serverReachable) {
    blockers.push({
      kind: 'workspace-host',
      title: 'Workspace Host 不可达',
      message: '客户端暂时无法连接业务服务，先恢复服务连接再进入店铺动作。',
      actionLabel: '查看系统设置',
      actionPath: '/settings',
    })
    return blockers
  }

  if (summary.serverReachable && !makaRuntimeReachable(summary)) {
    blockers.push({
      kind: 'maka-runtime',
      title: 'Maka Runtime 未连通',
      message: '本机运行时未就绪，打开后台、调到前台和更新凭据都可能失败。',
      actionLabel: '查看日志',
      actionPath: '/browser/logs',
    })
  }

  if (summary.serverReachable && makaRuntimeReachable(summary) && !summary.sessionReady) {
    blockers.push({
      kind: 'session',
      title: '桌面会话未就绪',
      message: '当前桌面登录态或设备会话还未 ready，请刷新或重新登录。',
      actionLabel: '刷新执行台',
      actionPath: '/',
    })
  }

  const credentialCount = shops.filter((shop) => shop.sharedLoginStatus !== 'ready').length
  if (credentialCount > 0) {
    blockers.push({
      kind: 'credential',
      title: `${credentialCount} 个店铺需要凭据处理`,
      message: '这些店铺还不能直接打开后台，优先进入店铺授权更新或验证凭据。',
      actionLabel: '进入店铺授权',
      actionPath: '/shops',
    })
  }

  const missingProfileCount = shops.filter((shop) => !shop.profileExists).length
  if (missingProfileCount > 0) {
    blockers.push({
      kind: 'profile',
      title: `${missingProfileCount} 个店铺缺少本地 Profile`,
      message: '店铺尚未完成本地实例映射，不能直接打开后台。',
      actionLabel: '进入店铺工作台',
      actionPath: '/workbench',
    })
  }

  const missingCoreCount = shops.filter((shop) => shop.profileExists && !shop.coreReady).length
  if (missingCoreCount > 0) {
    blockers.push({
      kind: 'core',
      title: `${missingCoreCount} 个店铺缺少可用内核`,
      message: '本地指纹内核不可用会阻塞 managed 店铺打开。',
      actionLabel: '检查内核',
      actionPath: '/browser/cores',
    })
  }

  return blockers
}
