import { Button, Modal } from '../../../shared/components'
import type { BrowserProfilePackageImportPreview } from '../../browser/types'

interface ProfilePackageConflictModalProps {
  preview: BrowserProfilePackageImportPreview | null
  busy?: boolean
  onClose: () => void
  onConfirm: (mode: 'new' | 'overwrite') => void
}

export function ProfilePackageConflictModal({
  preview,
  busy = false,
  onClose,
  onConfirm,
}: ProfilePackageConflictModalProps) {
  return (
    <Modal
      open={preview !== null}
      onClose={busy ? () => undefined : onClose}
      title="实例冲突"
      width="620px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>取消</Button>
          <Button variant="secondary" onClick={() => onConfirm('new')} loading={busy}>
            新建实例
          </Button>
          <Button
            variant="danger"
            onClick={() => onConfirm('overwrite')}
            disabled={busy || !preview?.canOverwrite}
            title={!preview?.canOverwrite ? '存在无法安全判定的目标，无法自动覆盖' : undefined}
          >
            覆盖并移入回收站
          </Button>
        </>
      )}
    >
      {preview && (
        <div className="space-y-3 text-sm">
          <div className="text-[var(--color-text-secondary)]">
            发现 {preview.conflictCount} 个冲突，共 {preview.profileCount} 个实例。
          </div>
          <div className="text-xs text-[var(--color-warning)]">
            选择覆盖后，原实例及其用户数据会移入回收站，新恢复实例会作为新的活动实例创建。
          </div>
          <div className="max-h-64 overflow-y-auto rounded-lg border border-[var(--color-border-default)]">
            {preview.conflicts.map((conflict) => (
              <div
                key={`${conflict.sourceProfileId || conflict.sourceProfileName}-${conflict.targetProfileId || conflict.targetMatches}`}
                className="border-b border-[var(--color-border-muted)] px-3 py-2.5 last:border-0"
              >
                <div className="flex items-center justify-between gap-3">
                  <span
                    className="min-w-0 truncate text-[var(--color-text-primary)]"
                    title={conflict.sourceProfileName || conflict.sourceProfileId}
                  >
                    {conflict.sourceProfileName || conflict.sourceProfileId || '未命名实例'}
                  </span>
                  <span className="shrink-0 text-xs text-[var(--color-text-muted)]">
                    {conflict.matchType === 'profileId' ? 'ID 匹配' : '名称匹配'}
                  </span>
                </div>
                {conflict.sourceNameCollision ? (
                  <div className="mt-1 text-xs text-[var(--color-error)]">
                    实例包内有 {conflict.targetMatches} 个同名实例，无法自动覆盖
                  </div>
                ) : conflict.sourceTargetCollision ? (
                  <div className="mt-1 text-xs text-[var(--color-error)]">
                    多个导入实例匹配同一目标，无法自动覆盖
                  </div>
                ) : conflict.ambiguous ? (
                  <div className="mt-1 text-xs text-[var(--color-error)]">
                    存在 {conflict.targetMatches} 个同名实例，无法自动覆盖
                  </div>
                ) : (
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-[var(--color-text-secondary)]">
                    <span>目标：{conflict.targetProfileName || conflict.targetProfileId || '未命名实例'}</span>
                    {conflict.targetDeleted && <span className="text-[var(--color-warning)]">回收站</span>}
                    {conflict.targetRunning && <span className="text-[var(--color-error)]">运行中</span>}
                  </div>
                )}
              </div>
            ))}
          </div>
          {!preview.canOverwrite && (
            <div className="text-xs text-[var(--color-warning)]">
              当前存在无法安全判定的目标，覆盖不可用；可选择新建实例。
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}
