import { useEffect, useMemo, useRef, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Button, Card, toast } from '../../shared/components'
import { useAuthStore } from '../../store/authStore'
import { buildRunEvidenceIndex, fetchWorkspaceRuns, type RunRecord, type ShopRunEvidence } from '../runEvidence'
import {
  closeWorkspaceShop,
  fetchWorkspaceSharedLoginBindSession,
  fetchWorkspaceAuthorizedShops,
  focusWorkspaceShop,
  openWorkspaceShop,
  startWorkspaceSharedLoginBind,
  startWorkspaceSharedLoginValidate,
} from '../workspace/api'
import { SharedLoginSessionModal } from '../workspace/components/SharedLoginSessionModal'
import {
  isTerminalSharedLoginStatus,
  resolveSharedLoginActionError,
  sharedLoginActionSuccessMessage,
  type SharedLoginAction,
  type SharedLoginDialogState,
} from '../workspace/sharedLoginSession'
import type { WorkspaceAuthorizedShop } from '../workspace/types'
import { ShopWorkbenchDrawer } from './components/ShopWorkbenchDrawer'
import { WorkbenchQueues } from './components/WorkbenchQueues'
import { WorkbenchTable } from './components/WorkbenchTable'
import { evidenceForWorkbenchRow } from './rowState'
import { deriveWorkbenchState } from './statusMatrix'
import type { WorkbenchActionKey, WorkbenchQueueKey, WorkbenchRow } from './types'

type ActiveQueue = WorkbenchQueueKey | 'all'
type RunnableWorkbenchAction = Extract<WorkbenchActionKey, 'open' | 'close' | 'bind' | 'validate'> | 'focus'
type RunningWorkbenchAction = { shopId: string; action: RunnableWorkbenchAction }

const unsupportedActionMessage: Record<WorkbenchActionKey, string> = {
  open: '',
  close: '',
  bind: '',
  validate: '',
  retry: '重试动作会在后续批量执行任务接入，当前请先查看运行证据。',
  refresh: '刷新同步会在后续任务接入，当前请使用右上角刷新重新拉取工作台数据。',
  core_management: '需要先到指纹内核配置页修复内核，不在工作台里伪执行。',
  diagnostics: '诊断导出会在后续任务接入，当前可先查看抽屉里的运行证据。',
  none: '当前状态不可执行推荐动作。',
}

function emptyEvidence(): ShopRunEvidence {
  return {
    latestOpen: null,
    latestCredential: null,
    latestValidation: null,
    latestFailure: null,
    activeRun: null,
  }
}

function actionSuccessLabel(action: WorkbenchActionKey | 'focus', shop: WorkspaceAuthorizedShop) {
  const name = shop.shopName || shop.shopId
  if (action === 'open') return `已打开 ${name}`
  if (action === 'focus') return `已调起 ${name}`
  if (action === 'close') return `已关闭 ${name}`
  if (action === 'bind') return `${name} 更新凭据已发起`
  if (action === 'validate') return `${name} 本机验证已发起`
  return '动作已发起'
}

function actionFallbackError(action: WorkbenchActionKey | 'focus') {
  if (action === 'open') return '打开店铺后台失败'
  if (action === 'focus') return '调起店铺后台失败'
  if (action === 'close') return '关闭店铺后台失败'
  if (action === 'bind') return '发起更新凭据失败'
  if (action === 'validate') return '发起本机验证失败'
  return '动作执行失败'
}

function resolveRunnableAction(row: WorkbenchRow): RunnableWorkbenchAction | null {
  const action = row.recommendedAction
  if (action === 'open' || action === 'close' || action === 'bind' || action === 'validate') return action

  if (action === 'retry') {
    const failedTaskType = row.evidence.latestFailure?.taskType
    if (failedTaskType === 'open' || failedTaskType === 'bind' || failedTaskType === 'validate') {
      return failedTaskType
    }
    return 'open'
  }

  return null
}

export function WorkbenchPage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const [searchParams, setSearchParams] = useSearchParams()
  const [shops, setShops] = useState<WorkspaceAuthorizedShop[]>([])
  const [runs, setRuns] = useState<RunRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [activeQueue, setActiveQueue] = useState<ActiveQueue>('all')
  const [selectedRow, setSelectedRow] = useState<WorkbenchRow | null>(null)
  const [runningAction, setRunningAction] = useState<RunningWorkbenchAction | null>(null)
  const [sharedLoginDialog, setSharedLoginDialog] = useState<SharedLoginDialogState | null>(null)
  const runningActionRef = useRef<RunningWorkbenchAction | null>(null)

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      const [nextShops, nextRuns] = await Promise.all([
        fetchWorkspaceAuthorizedShops(),
        fetchWorkspaceRuns({ limit: 100 }),
      ])
      setShops(nextShops)
      setRuns(nextRuns.items)
    } catch (error) {
      console.error('load workbench failed', error)
      toast.error('加载店铺工作台失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const rows = useMemo<WorkbenchRow[]>(() => {
    const index = buildRunEvidenceIndex(runs)

    return shops
      .map((shop) => {
        const evidence = evidenceForWorkbenchRow(shop, index.byShop[shop.shopId] || emptyEvidence())
        const failureCode = evidence.latestFailure?.failureCode || ''
        const failureMessage = evidence.latestFailure?.failureMessage || ''
        const workbenchState = deriveWorkbenchState({
          reclaimPending: shop.reclaimPending,
          instanceRunning: shop.instanceRunning,
          activeRun: Boolean(evidence.activeRun),
          profileExists: shop.profileExists,
          coreReady: shop.coreReady,
          sharedLoginStatus: shop.sharedLoginStatus,
          failureCode,
          failureMessage,
        })

        return {
          shop,
          evidence,
          workbenchState,
          queue: workbenchState.queue,
          recommendedAction: workbenchState.recommendedAction,
          failureCode,
          failureMessage,
        }
      })
      .sort((a, b) => {
        const runningDelta = Number(Boolean(b.evidence.activeRun)) - Number(Boolean(a.evidence.activeRun))
        if (runningDelta !== 0) return runningDelta
        return (a.shop.shopName || a.shop.shopId).localeCompare(b.shop.shopName || b.shop.shopId, 'zh-CN')
      })
  }, [runs, shops])

  const requestedShopId = searchParams.get('shopId')?.trim() || ''

  useEffect(() => {
    if (!requestedShopId || selectedRow?.shop.shopId === requestedShopId) return
    const matched = rows.find((row) => row.shop.shopId === requestedShopId)
    if (matched) setSelectedRow(matched)
  }, [requestedShopId, rows, selectedRow])

  useEffect(() => {
    setSelectedRow((current) => {
      if (!current) return current
      return rows.find((row) => row.shop.shopId === current.shop.shopId) || current
    })
  }, [rows])

  const visibleRows = useMemo(() => {
    return activeQueue === 'all' ? rows : rows.filter((row) => row.queue === activeQueue)
  }, [activeQueue, rows])
  const requestedShopMissing = Boolean(
    requestedShopId && !loading && !rows.some((row) => row.shop.shopId === requestedShopId)
  )

  async function runRecommendedAction(row: WorkbenchRow) {
    const action = resolveRunnableAction(row)
    if (runningActionRef.current) {
      toast.info('已有推荐动作正在执行，请稍候')
      return
    }

    if (!action) {
      toast.info(unsupportedActionMessage[row.recommendedAction])
      return
    }

    if ((action === 'bind' || action === 'validate') && !accessToken.trim()) {
      toast.error('当前桌面登录态已失效，请重新登录')
      return
    }

    const nextRunningAction = { shopId: row.shop.shopId, action }
    runningActionRef.current = nextRunningAction
    setRunningAction(nextRunningAction)
    try {
      if (action === 'open') {
        const result = await openWorkspaceShop(row.shop.shopId)
        await load(true)
        if (!result.success) {
          toast.error(result.message || '打开店铺后台失败')
          return
        }
        toast.success(actionSuccessLabel(action, row.shop))
      } else if (action === 'close') {
        const closed = await closeWorkspaceShop(row.shop.profileId)
        await load(true)
        if (!closed) {
          toast.error('当前店铺后台未在运行')
          return
        }
        toast.success(actionSuccessLabel(action, row.shop))
      } else if (action === 'bind') {
        await startSharedLoginAction('bind', row.shop)
      } else {
        await startSharedLoginAction('validate', row.shop)
      }
    } catch (error: any) {
      console.error('run workbench recommended action failed', error)
      if (action === 'bind' || action === 'validate') {
        setSharedLoginDialog(null)
      }
      toast.error(action === 'bind' || action === 'validate' ? resolveSharedLoginActionError(action, error) : String(error?.message || actionFallbackError(action)))
    } finally {
      if (
        runningActionRef.current?.shopId === nextRunningAction.shopId
        && runningActionRef.current?.action === nextRunningAction.action
      ) {
        runningActionRef.current = null
        setRunningAction((current) => (
          current?.shopId === nextRunningAction.shopId && current.action === nextRunningAction.action ? null : current
        ))
      }
    }
  }

  async function focusRunningShop(row: WorkbenchRow) {
    if (runningActionRef.current) {
      toast.info('已有推荐动作正在执行，请稍候')
      return
    }

    const nextRunningAction = { shopId: row.shop.shopId, action: 'focus' as const }
    runningActionRef.current = nextRunningAction
    setRunningAction(nextRunningAction)
    try {
      const result = await focusWorkspaceShop(row.shop.shopId)
      await load(true)
      if (!result.success) {
        toast.error(result.message || '调起店铺后台失败')
        return
      }
      toast.success(actionSuccessLabel('focus', row.shop))
    } catch (error: any) {
      console.error('focus workspace shop failed', error)
      toast.error(String(error?.message || actionFallbackError('focus')))
    } finally {
      if (
        runningActionRef.current?.shopId === nextRunningAction.shopId
        && runningActionRef.current?.action === nextRunningAction.action
      ) {
        runningActionRef.current = null
        setRunningAction((current) => (
          current?.shopId === nextRunningAction.shopId && current.action === nextRunningAction.action ? null : current
        ))
      }
    }
  }

  function closeWorkbenchDrawer() {
    setSelectedRow(null)
    if (!requestedShopId) return

    const nextParams = new URLSearchParams(searchParams)
    nextParams.delete('shopId')
    setSearchParams(nextParams, { replace: true })
  }

  async function handleSharedLoginTerminal(
    action: SharedLoginAction,
    shopName: string,
    status: string,
    message: string,
  ) {
    await load(true)
    if (status === 'completed' || status === 'succeeded') {
      toast.success(sharedLoginActionSuccessMessage(action, shopName))
      return
    }
    if (status === 'expired') {
      toast.error('授权处理已过期，请重新发起')
      return
    }
    toast.error(message || '授权处理失败，请重试')
  }

  async function startSharedLoginAction(action: SharedLoginAction, shop: WorkspaceAuthorizedShop) {
    const token = accessToken.trim()
    const shopName = shop.shopName || shop.shopId
    setSharedLoginDialog({
      action,
      shopId: shop.shopId,
      shopName,
      session: null,
      starting: true,
      terminalHandled: false,
      errorMessage: '',
    })

    const result = action === 'bind'
      ? await startWorkspaceSharedLoginBind(token, shop.shopId)
      : await startWorkspaceSharedLoginValidate(token, shop.shopId)
    const nextShopName = result.detail.shopName || shopName
    const terminal = isTerminalSharedLoginStatus(result.bindSession.status)
    setSharedLoginDialog({
      action,
      shopId: shop.shopId,
      shopName: nextShopName,
      session: result.bindSession,
      starting: false,
      terminalHandled: terminal,
      errorMessage: '',
    })
    if (terminal) {
      await handleSharedLoginTerminal(action, nextShopName, result.bindSession.status, result.bindSession.message)
    }
  }

  useEffect(() => {
    if (!sharedLoginDialog || sharedLoginDialog.starting || !sharedLoginDialog.session) {
      return
    }

    const token = accessToken.trim()
    const bindSessionId = sharedLoginDialog.session.bindSessionId.trim()
    if (!token || !bindSessionId || isTerminalSharedLoginStatus(sharedLoginDialog.session.status)) {
      return
    }

    let cancelled = false
    const timer = window.setTimeout(async () => {
      try {
        const nextSession = await fetchWorkspaceSharedLoginBindSession(token, bindSessionId)
        if (cancelled) return

        const terminal = isTerminalSharedLoginStatus(nextSession.status)
        setSharedLoginDialog((current) => {
          if (!current || current.session?.bindSessionId !== bindSessionId) return current
          return {
            ...current,
            session: nextSession,
            terminalHandled: current.terminalHandled || terminal,
            errorMessage: '',
          }
        })

        if (terminal && !sharedLoginDialog.terminalHandled) {
          await handleSharedLoginTerminal(sharedLoginDialog.action, sharedLoginDialog.shopName, nextSession.status, nextSession.message)
        }
      } catch (error: any) {
        if (cancelled) return
        console.error('poll shared login session failed', error)
        setSharedLoginDialog((current) => {
          if (!current || current.session?.bindSessionId !== bindSessionId) return current
          return {
            ...current,
            errorMessage: String(error?.message || '轮询共享登录状态失败，请稍后重试'),
          }
        })
      }
    }, 1500)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [
    accessToken,
    sharedLoginDialog,
    sharedLoginDialog?.action,
    sharedLoginDialog?.shopName,
    sharedLoginDialog?.starting,
    sharedLoginDialog?.terminalHandled,
    sharedLoginDialog?.session?.bindSessionId,
    sharedLoginDialog?.session?.status,
    sharedLoginDialog?.session?.updatedAt,
  ])

  return (
    <div className="grid h-full grid-cols-1 gap-5 overflow-auto p-5 animate-fade-in lg:grid-cols-[240px_minmax(0,1fr)]">
      <div className="space-y-4">
        <WorkbenchQueues rows={rows} active={activeQueue} onSelect={setActiveQueue} />
      </div>

      <div className="min-w-0 space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="min-w-0">
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">店铺工作台</h1>
            <p className="mt-1 break-words text-sm text-[var(--color-text-muted)]">
              管理本机店铺窗口：打开后台、调到前台、关闭实例，并查看最近失败原因。
            </p>
          </div>
          <Button className="w-full shrink-0 sm:w-auto" variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
        </div>

        {runningAction ? (
          <div className="rounded-lg border border-[var(--color-accent)]/20 bg-[var(--color-accent)]/10 px-4 py-2 text-sm text-[var(--color-accent)]">
            正在处理店铺窗口：{runningAction.shopId}
          </div>
        ) : null}

        {requestedShopMissing ? (
          <Alert
            type="warning"
            title="该店铺还没有本地授权实例"
            message="ASM 店铺主数据已同步，但店铺工作台只处理已接入本地授权实例的店铺。请先完成授权接入或在管理端检查主数据授权关系。"
          />
        ) : null}

        <Card padding="none">
          <WorkbenchTable
            rows={visibleRows}
            loading={loading}
            runningAction={runningAction}
            onOpenDrawer={setSelectedRow}
            onAction={(row) => void runRecommendedAction(row)}
            onFocus={(row) => void focusRunningShop(row)}
          />
        </Card>
      </div>

      <ShopWorkbenchDrawer
        row={selectedRow}
        open={Boolean(selectedRow)}
        runningAction={runningAction}
        onClose={closeWorkbenchDrawer}
        onAction={(row) => void runRecommendedAction(row)}
        onFocus={(row) => void focusRunningShop(row)}
      />
      <SharedLoginSessionModal
        dialog={sharedLoginDialog}
        onClose={() => setSharedLoginDialog(null)}
      />
    </div>
  )
}
