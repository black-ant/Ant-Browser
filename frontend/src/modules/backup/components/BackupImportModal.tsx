import type { RefObject } from 'react'
import { AlertTriangle, CheckCircle2 } from 'lucide-react'

import { Button, Modal, Progress } from '../../../shared/components'
import { NotificationMessage } from '../../../shared/notifications/NotificationMessage'

import type { BackupExportLogItem, BackupExportProgress } from '../progress'

type BackupActionLoading = 'none' | 'export' | 'import-merge'

interface BackupProgressPanelProps {
  progress: BackupExportProgress
  loadingLabel: string
  logs?: BackupExportLogItem[]
  logsRef?: RefObject<HTMLDivElement>
}

interface BackupImportModalProps {
  open: boolean
  actionLoading: BackupActionLoading
  importProgress: BackupExportProgress | null
  onClose: () => void
  onImport: () => void
}

function BackupProgressPanel({ progress, loadingLabel, logs = [], logsRef }: BackupProgressPanelProps) {
  return (
    <div className="space-y-3 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3 shadow-[var(--shadow-xs)]">
      <div className="flex items-start justify-between gap-3">
        <NotificationMessage message={progress.message} context='backup' className="min-w-0 flex-1 text-[var(--color-text-secondary)]" />
        {progress.phase === 'error' && <span className="shrink-0 text-xs font-medium text-[var(--color-error)]">失败</span>}
        {progress.phase === 'done' && <span className="shrink-0 text-xs font-medium text-[var(--color-success)]">完成</span>}
        {progress.phase !== 'done' && progress.phase !== 'error' && (
          <span className="shrink-0 text-xs font-medium text-[var(--color-text-muted)]">{loadingLabel}</span>
        )}
      </div>
      {(progress.componentName || progress.componentId || logsRef) && (
        <div className="text-xs text-[var(--color-text-muted)]">
          当前组件：
          {' '}
          {progress.componentName || progress.componentId || '准备中'}
          {progress.entryIndex && progress.entryTotal
            ? `（${progress.entryIndex}/${progress.entryTotal}）`
            : ''}
        </div>
      )}
      <Progress
        percent={progress.progress}
        size="sm"
        status={progress.phase === 'error' ? 'error' : progress.phase === 'done' ? 'success' : 'normal'}
      />
      {progress.phase === 'error' && (
        <div role="alert" className="flex gap-2 rounded-md border border-[var(--color-error)]/50 bg-[var(--color-error)]/10 px-3 py-2 text-xs text-[var(--color-error)]">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="min-w-0">
            <p className="font-semibold">备份操作失败</p>
            <NotificationMessage message={progress.message} context='backup' className="mt-0.5 text-[var(--color-error)]" compact />
          </div>
        </div>
      )}
      {progress.phase === 'done' && (
        <div className="flex items-center gap-2 rounded-md border border-[var(--color-success)]/40 bg-[var(--color-success)]/10 px-3 py-2 text-xs text-[var(--color-success)]">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <NotificationMessage message={progress.message} context='backup' className="text-[var(--color-success)]" compact />
        </div>
      )}
      {logsRef && (
        <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-3 py-2">
          <div className="mb-1.5 flex items-center justify-between gap-2 text-[11px]">
            <span className="font-medium text-[var(--color-text-secondary)]">过程记录</span>
            <span className="text-[var(--color-text-muted)]">{logs.length} 条</span>
          </div>
          <div ref={logsRef} className="max-h-36 space-y-1 overflow-y-auto pr-1">
            {logs.length === 0 && (
              <p className="text-xs text-[var(--color-text-muted)]">等待导出日志...</p>
            )}
            {logs.map(item => (
              <div key={item.id} className="min-w-0 rounded-md bg-[var(--color-bg-surface)] px-2 py-1">
                <div className="flex min-w-0 items-start gap-2">
                  <span className="shrink-0 pt-0.5 font-mono text-[10px] leading-5 text-[var(--color-text-muted)]">{item.time}</span>
                  <NotificationMessage
                    message={item.text}
                    context='backup'
                    compact
                    className={item.phase === 'error' ? 'min-w-0 flex-1 text-[var(--color-error)]' : item.phase === 'done' ? 'min-w-0 flex-1 text-[var(--color-success)]' : 'min-w-0 flex-1 text-[var(--color-text-secondary)]'}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export function BackupImportModal({
  open,
  actionLoading,
  importProgress,
  onClose,
  onImport,
}: BackupImportModalProps) {
  const importRunning = actionLoading === 'import-merge'

  return (
    <Modal
      open={open}
      onClose={() => {
        if (actionLoading !== 'none') {
          return
        }
        onClose()
      }}
      title="导入备份"
      width="620px"
      closable={!importRunning}
      footer={(
        <>
          {!importRunning && (
            <Button variant="secondary" onClick={onClose}>
              取消
            </Button>
          )}
          <Button
            onClick={onImport}
            loading={actionLoading === 'import-merge'}
            disabled={actionLoading !== 'none' && actionLoading !== 'import-merge'}
          >
            合并导入
          </Button>
        </>
      )}
    >
      <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
        <p className="text-xs text-[var(--color-text-muted)]">支持全量备份和实例备份；当前数据不会被清空。</p>
        {importProgress && (
          <BackupProgressPanel progress={importProgress} loadingLabel="导入中" />
        )}
        {importRunning && (
          <p className="text-xs text-[var(--color-warning)]">
            当前正在导入备份，弹窗不可关闭。若需中断，请直接关闭应用。
          </p>
        )}
      </div>
    </Modal>
  )
}
