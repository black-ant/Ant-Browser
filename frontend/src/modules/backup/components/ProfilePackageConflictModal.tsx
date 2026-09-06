import { useEffect, useState } from 'react'
import { Button, Modal } from '../../../shared/components'
import type {
  BrowserProfilePackageImportAction,
  BrowserProfilePackageImportActionMode,
  BrowserProfilePackageImportPreview,
  BrowserProfilePackageImportPreviewProfile,
} from '../../browser/types'

interface ProfilePackageConflictModalProps {
  preview: BrowserProfilePackageImportPreview | null
  busy?: boolean
  onClose: () => void
  onConfirm: (actions: BrowserProfilePackageImportAction[]) => void
}

function defaultAction(row: BrowserProfilePackageImportPreviewProfile): BrowserProfilePackageImportAction {
  return {
    sourceProfileId: row.sourceProfileId,
    sourceIndex: row.sourceIndex,
    mode: 'new',
    profileName: row.suggestedProfileName,
  }
}

function rowIssue(row: BrowserProfilePackageImportPreviewProfile) {
  if (row.sourceNameCollision) return '备份内存在同名实例，不能覆盖'
  if (row.sourceTargetCollision) return '多个实例匹配同一目标，不能覆盖'
  if (row.ambiguous) return `已有 ${row.targetMatches} 个同名目标，不能覆盖`
  if (row.targetRunning) return '目标正在运行，不能覆盖'
  if (row.targetProfileName) return '已匹配现有实例'
  return '无冲突'
}

function modeLabel(mode: BrowserProfilePackageImportActionMode) {
  if (mode === 'overwrite') return '覆盖原有'
  if (mode === 'rename') return '重命名'
  return '新建'
}

export function ProfilePackageConflictModal({
  preview,
  busy = false,
  onClose,
  onConfirm,
}: ProfilePackageConflictModalProps) {
  const [actions, setActions] = useState<Record<number, BrowserProfilePackageImportAction>>({})
  const [validationError, setValidationError] = useState('')

  useEffect(() => {
    if (!preview) {
      setActions({})
      setValidationError('')
      return
    }
    const next: Record<number, BrowserProfilePackageImportAction> = {}
    preview.profiles.forEach((row) => {
      next[row.sourceIndex] = defaultAction(row)
    })
    setActions(next)
    setValidationError('')
  }, [preview])

  const updateAction = (row: BrowserProfilePackageImportPreviewProfile, mode: BrowserProfilePackageImportActionMode) => {
    setValidationError('')
    setActions((previous) => {
      const current = previous[row.sourceIndex] || defaultAction(row)
      return {
        ...previous,
        [row.sourceIndex]: {
          ...current,
          mode,
          profileName: mode === 'rename'
            ? (current.profileName?.trim() || row.suggestedProfileName)
            : row.suggestedProfileName,
        },
      }
    })
  }

  const updateRename = (row: BrowserProfilePackageImportPreviewProfile, profileName: string) => {
    setValidationError('')
    setActions((previous) => ({
      ...previous,
      [row.sourceIndex]: {
        ...(previous[row.sourceIndex] || defaultAction(row)),
        mode: 'rename',
        profileName,
      },
    }))
  }

  const handleConfirm = () => {
    if (!preview || busy) return
    const resolvedActions = preview.profiles.map((row) => actions[row.sourceIndex] || defaultAction(row))
    const invalidRename = resolvedActions.find((action) => action.mode === 'rename' && !action.profileName?.trim())
    if (invalidRename) {
      setValidationError('请填写所有重命名实例的新名称')
      return
    }
    onConfirm(resolvedActions.map((action) => ({
      ...action,
      profileName: action.mode === 'rename' ? action.profileName?.trim() : '',
    })))
  }

  return (
    <Modal
      open={preview !== null}
      onClose={busy ? () => undefined : onClose}
      title="确认还原实例"
      width="980px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>取消</Button>
          <Button onClick={handleConfirm} loading={busy}>确认还原</Button>
        </>
      )}
    >
      {preview && (
        <div className="space-y-3 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2 text-[var(--color-text-secondary)]">
            <span>共 {preview.profileCount} 个实例，逐行选择处理方式。</span>
            <span>冲突 {preview.conflictCount} 个</span>
          </div>
          <div className="text-xs text-[var(--color-warning)]">
            覆盖会把旧实例及用户数据移入回收站；新建和重命名都会保留旧实例。
          </div>
          <div className="overflow-x-auto rounded-lg border border-[var(--color-border-default)]">
            <table className="w-full min-w-[860px] border-collapse text-left">
              <thead className="bg-[var(--color-bg-muted)] text-xs text-[var(--color-text-secondary)]">
                <tr>
                  <th className="px-3 py-2 font-medium">还原实例</th>
                  <th className="px-3 py-2 font-medium">现有目标</th>
                  <th className="w-36 px-3 py-2 font-medium">操作</th>
                  <th className="min-w-56 px-3 py-2 font-medium">还原后名称</th>
                  <th className="px-3 py-2 font-medium">状态</th>
                </tr>
              </thead>
              <tbody>
                {preview.profiles.map((row) => {
                  const action = actions[row.sourceIndex] || defaultAction(row)
                  const overwriteDisabled = !row.canOverwrite
                  const finalName = action.mode === 'overwrite'
                    ? (row.sourceProfileName || row.targetProfileName || row.suggestedProfileName)
                    : row.suggestedProfileName
                  return (
                    <tr key={`${row.sourceIndex}-${row.sourceProfileId}`} className="border-t border-[var(--color-border-muted)] align-top">
                      <td className="max-w-56 px-3 py-3">
                        <div className="truncate text-[var(--color-text-primary)]" title={row.sourceProfileName || row.sourceProfileId}>
                          {row.sourceProfileName || row.sourceProfileId || '未命名实例'}
                        </div>
                        <div className="mt-1 text-xs text-[var(--color-text-muted)]">第 {row.sourceIndex + 1} 个</div>
                      </td>
                      <td className="max-w-56 px-3 py-3 text-[var(--color-text-secondary)]">
                        {row.targetProfileName ? (
                          <>
                            <div className="truncate" title={row.targetProfileName}>{row.targetProfileName}</div>
                            {row.matchType && <div className="mt-1 text-xs text-[var(--color-text-muted)]">{row.matchType === 'profileId' ? 'ID 匹配' : '名称匹配'}</div>}
                          </>
                        ) : (
                          <span className="text-[var(--color-text-muted)]">无</span>
                        )}
                      </td>
                      <td className="px-3 py-3">
                        <select
                          value={action.mode}
                          onChange={(event) => updateAction(row, event.target.value as BrowserProfilePackageImportActionMode)}
                          disabled={busy}
                          className="w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-2 py-1.5 text-sm text-[var(--color-text-primary)]"
                        >
                          <option value="new">新建</option>
                          <option value="overwrite" disabled={overwriteDisabled}>覆盖原有{overwriteDisabled ? '（不可用）' : ''}</option>
                          <option value="rename">重命名</option>
                        </select>
                      </td>
                      <td className="px-3 py-3">
                        {action.mode === 'rename' ? (
                          <input
                            value={action.profileName || ''}
                            onChange={(event) => updateRename(row, event.target.value)}
                            disabled={busy}
                            placeholder="输入新名称"
                            className="w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-2 py-1.5 text-sm text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]"
                          />
                        ) : (
                          <div className="truncate py-1.5 text-[var(--color-text-secondary)]" title={finalName}>
                            {finalName || '导入实例'}
                          </div>
                        )}
                      </td>
                      <td className="max-w-52 px-3 py-3 text-xs text-[var(--color-text-secondary)]">
                        <span className={row.canOverwrite ? 'text-[var(--color-warning)]' : ''}>{rowIssue(row)}</span>
                        {action.mode === 'overwrite' && row.canOverwrite && (
                          <div className="mt-1 text-[var(--color-error)]">将移入回收站</div>
                        )}
                        {action.mode !== 'overwrite' && (
                          <div className="mt-1 text-[var(--color-text-muted)]">{modeLabel(action.mode)}</div>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          {validationError && <div className="text-xs text-[var(--color-error)]">{validationError}</div>}
        </div>
      )}
    </Modal>
  )
}
