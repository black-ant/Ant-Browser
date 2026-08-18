import type { RefObject } from 'react'
import { AlertTriangle, CheckCircle2, Download, RotateCcw, Upload } from 'lucide-react'

import { Button, Card, Modal, Progress } from '../../../shared/components'

import type { BackupExportLogItem, BackupExportProgress } from '../progress'

type BackupActionLoading = 'none' | 'init' | 'export' | 'import-reset' | 'import-merge'

interface BackupProgressPanelProps {
  progress: BackupExportProgress
  loadingLabel: string
  logs?: BackupExportLogItem[]
  logsRef?: RefObject<HTMLDivElement>
}

interface BackupSettingsCardProps {
  actionLoading: BackupActionLoading
  exportProgress: BackupExportProgress | null
  exportLogs: BackupExportLogItem[]
  exportLogsRef: RefObject<HTMLDivElement>
  onInitialize: () => void
  onExport: () => void
  onOpenImport: () => void
}

interface BackupImportModalProps {
  open: boolean
  actionLoading: BackupActionLoading
  importProgress: BackupExportProgress | null
  onClose: () => void
  onImport: (resetFirst: boolean) => void
}

function BackupProgressPanel({ progress, loadingLabel, logs = [], logsRef }: BackupProgressPanelProps) {
  return (
    <div className="rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 space-y-2">
      <div className="flex items-center justify-between text-xs">
        <span className="text-[var(--color-text-secondary)]">{progress.message}</span>
        {progress.phase === 'error' && <span className="text-[var(--color-error)]">失败</span>}
        {progress.phase === 'done' && <span className="text-[var(--color-success)]">完成</span>}
        {progress.phase !== 'done' && progress.phase !== 'error' && (
          <span className="text-[var(--color-text-muted)]">{loadingLabel}</span>
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
            <p className="mt-0.5 break-words leading-5">{progress.message}</p>
          </div>
        </div>
      )}
      {progress.phase === 'done' && (
        <div className="flex items-center gap-2 rounded-md border border-[var(--color-success)]/40 bg-[var(--color-success)]/10 px-3 py-2 text-xs text-[var(--color-success)]">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span className="break-words">{progress.message}</span>
        </div>
      )}
      {logsRef && (
        <div className="rounded border border-[var(--color-border-muted)] bg-[var(--color-bg-primary)] px-2 py-2">
          <div className="flex items-center justify-between text-xs mb-1">
            <span className="text-[var(--color-text-secondary)]">导出日志</span>
            <span className="text-[var(--color-text-muted)]">{logs.length} 条</span>
          </div>
          <div ref={logsRef} className="max-h-36 overflow-y-auto pr-1 space-y-1">
            {logs.length === 0 && (
              <p className="text-xs text-[var(--color-text-muted)]">等待导出日志...</p>
            )}
            {logs.map(item => (
              <div key={item.id} className="text-xs leading-5 font-mono">
                <span className="text-[var(--color-text-muted)] mr-2">{item.time}</span>
                <span className={item.phase === 'error' ? 'text-[var(--color-error)]' : item.phase === 'done' ? 'text-[var(--color-success)]' : 'text-[var(--color-text-secondary)]'}>
                  {item.text}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export function BackupSettingsCard({
  actionLoading,
  exportProgress,
  exportLogs,
  exportLogsRef,
  onInitialize,
  onExport,
  onOpenImport,
}: BackupSettingsCardProps) {
  return (
    <Card title="全局备份与恢复" subtitle="管理应用配置、数据库和浏览器数据">
      <div className="space-y-3">
        <div className="flex gap-2 rounded-md border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-xs text-[var(--color-warning)]">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="leading-5">
            <p className="font-medium">导出前请先停止所有实例。</p>
            <p>运行中的实例会被后端拒绝导出；导入备份会先停止运行中的实例。</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="danger"
            size="sm"
            onClick={onInitialize}
            loading={actionLoading === 'init'}
            disabled={actionLoading !== 'none' && actionLoading !== 'init'}
          >
            <RotateCcw className="w-4 h-4" />
            初始化系统
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={onExport}
            loading={actionLoading === 'export'}
            disabled={actionLoading !== 'none' && actionLoading !== 'export'}
          >
            <Download className="w-4 h-4" />
            导出全量备份
          </Button>
          <Button size="sm" onClick={onOpenImport} disabled={actionLoading !== 'none'}>
            <Upload className="w-4 h-4" />
            导入备份
          </Button>
        </div>
        {exportProgress && (
          <BackupProgressPanel
            progress={exportProgress}
            loadingLabel="处理中"
            logs={exportLogs}
            logsRef={exportLogsRef}
          />
        )}
      </div>
    </Card>
  )
}

export function BackupImportModal({
  open,
  actionLoading,
  importProgress,
  onClose,
  onImport,
}: BackupImportModalProps) {
  const importRunning = actionLoading === 'import-reset' || actionLoading === 'import-merge'

  return (
    <Modal
      open={open}
      onClose={() => {
        if (actionLoading !== 'none') {
          return
        }
        onClose()
      }}
      title="导入全局备份"
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
            variant="danger"
            onClick={() => onImport(true)}
            loading={actionLoading === 'import-reset'}
            disabled={actionLoading !== 'none' && actionLoading !== 'import-reset'}
          >
            清空后恢复
          </Button>
          <Button
            onClick={() => onImport(false)}
            loading={actionLoading === 'import-merge'}
            disabled={actionLoading !== 'none' && actionLoading !== 'import-merge'}
          >
            合并导入
          </Button>
        </>
      )}
    >
      <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
        <div className="rounded-md border border-[var(--color-error)]/40 bg-[var(--color-error)]/10 px-3 py-2 text-sm text-[var(--color-text-secondary)]">
          <p className="font-medium text-[var(--color-error)]">清空后恢复会删除当前业务数据。</p>
          <p className="mt-1 text-xs leading-5 text-[var(--color-text-muted)]">仅在确认备份包可用且不需要保留当前实例、代理、书签时使用。无法保证跨机器的 Cookie 和密码可解密。</p>
        </div>
        <p className="text-xs text-[var(--color-text-muted)]">合并导入会保留当前数据，并按 ID、路径和 URL 判重。</p>
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
