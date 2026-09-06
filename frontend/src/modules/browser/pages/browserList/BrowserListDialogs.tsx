import { Link } from 'react-router-dom'
import { XCircle } from 'lucide-react'
import { Button, Modal } from '../../../../shared/components'
import { BrowserProfileCopyForm } from '../../components/BrowserProfileCopyForm'
import { KeywordsModal } from '../../components/KeywordsModal'
import type { BrowserProfile, BrowserProfileCopyOptions, BrowserProfilePackageImportPreview } from '../../types'

interface BrowserListDialogsProps {
  proxyErrorModal: boolean
  pendingStartId: string | null
  proxyErrorMsg: string
  onCloseProxyError: () => void
  onStartDirect: () => void
  startingDirect: boolean
  kwModal: { open: boolean; profile: BrowserProfile | null }
  onCloseKeywords: () => void
  onKeywordsSaved: (keywords: string[]) => void
  copyModal: { open: boolean; profile: BrowserProfile | null }
  copyName: string
  copyOptions: BrowserProfileCopyOptions
  onCopyNameChange: (value: string) => void
  onCopyOptionsChange: (value: BrowserProfileCopyOptions) => void
  onCloseCopy: () => void
  onConfirmCopy: () => void
  copyConfirmDisabled: boolean
  copying: boolean
  deleteConfirm: { open: boolean; mode: 'single' | 'batch'; profileName?: string; count: number }
  deleting: boolean
  onCloseDeleteConfirm: () => void
  onConfirmDelete: () => void
  trashModalOpen: boolean
  trashProfiles: BrowserProfile[]
  trashLoading: boolean
  restoringId: string
  permanentlyDeletingId: string
  permanentDeleteConfirm: { open: boolean; profile: BrowserProfile | null }
  onCloseTrash: () => void
  onRestoreProfile: (profileId: string) => void
  onOpenPermanentDelete: (profile: BrowserProfile) => void
  onClosePermanentDelete: () => void
  onConfirmPermanentDelete: () => void
  opError: string
  onCloseOpError: () => void
  profileImportPreview: BrowserProfilePackageImportPreview | null
  onCloseProfileImport: () => void
  onConfirmProfileImport: (mode: 'new' | 'overwrite') => void
}

export function BrowserListDialogs({
  proxyErrorModal,
  pendingStartId,
  proxyErrorMsg,
  onCloseProxyError,
  onStartDirect,
  startingDirect,
  kwModal,
  onCloseKeywords,
  onKeywordsSaved,
  copyModal,
  copyName,
  copyOptions,
  onCopyNameChange,
  onCopyOptionsChange,
  onCloseCopy,
  onConfirmCopy,
  copyConfirmDisabled,
  copying,
  deleteConfirm,
  deleting,
  onCloseDeleteConfirm,
  onConfirmDelete,
  trashModalOpen,
  trashProfiles,
  trashLoading,
  restoringId,
  permanentlyDeletingId,
  permanentDeleteConfirm,
  onCloseTrash,
  onRestoreProfile,
  onOpenPermanentDelete,
  onClosePermanentDelete,
  onConfirmPermanentDelete,
  opError,
  onCloseOpError,
  profileImportPreview,
  onCloseProfileImport,
  onConfirmProfileImport,
}: BrowserListDialogsProps) {
  const formatTime = (value?: string) => {
    if (!value) return '-'
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN')
  }

  const formatExpireTime = (value?: string) => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '-'
    date.setDate(date.getDate() + 3)
    return date.toLocaleString('zh-CN')
  }

  return (
    <>
      <Modal
        open={proxyErrorModal}
        onClose={onCloseProxyError}
        title="代理链路不可用"
        width="420px"
        footer={
          <>
            <Button variant="secondary" onClick={onCloseProxyError} disabled={startingDirect}>取消</Button>
            {pendingStartId && (
              <Button variant="secondary" onClick={onStartDirect} loading={startingDirect}>
                直连启动
              </Button>
            )}
            {pendingStartId && (
              <Link to={`/browser/edit/${pendingStartId}`}>
                <Button onClick={onCloseProxyError} disabled={startingDirect}>去修改代理</Button>
              </Link>
            )}
          </>
        }
      >
        <div className="space-y-3">
          <div className="flex items-start gap-3 p-3 rounded-lg bg-[var(--color-bg-secondary)]">
            <XCircle className="w-5 h-5 text-red-500 mt-0.5 shrink-0" />
            <p className="text-sm text-[var(--color-text-primary)]">{proxyErrorMsg}</p>
          </div>
          <p className="text-sm text-[var(--color-text-muted)]">请前往编辑页面重新选择可用链路；如果是订阅导入，先刷新订阅并确认该节点仍存在。</p>
        </div>
      </Modal>

      {kwModal.profile && (
        <KeywordsModal
          open={kwModal.open}
          profileId={kwModal.profile.profileId}
          profileName={kwModal.profile.profileName}
          initialKeywords={kwModal.profile.keywords || []}
          onClose={onCloseKeywords}
          onSaved={onKeywordsSaved}
        />
      )}

      <Modal
        open={copyModal.open}
        onClose={onCloseCopy}
        title="复制实例"
        width="720px"
        footer={
          <>
            <Button variant="secondary" onClick={onCloseCopy}>取消</Button>
            <Button onClick={onConfirmCopy} loading={copying} disabled={copyConfirmDisabled}>确认复制</Button>
          </>
        }
      >
        <BrowserProfileCopyForm
          sourceName={copyModal.profile?.profileName}
          copyName={copyName}
          copyOptions={copyOptions}
          onCopyNameChange={onCopyNameChange}
          onCopyOptionsChange={onCopyOptionsChange}
          autoFocusName
        />
      </Modal>

      <Modal
        open={deleteConfirm.open}
        onClose={onCloseDeleteConfirm}
        title="删除实例"
        width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={onCloseDeleteConfirm} disabled={deleting}>取消</Button>
            <Button variant="danger" onClick={onConfirmDelete} loading={deleting}>确定删除</Button>
          </>
        }
      >
        <div className="text-sm text-[var(--color-text-secondary)]">
          {deleteConfirm.mode === 'batch'
            ? `确定将选中的 ${deleteConfirm.count} 个实例移入回收站？3 天内可恢复。`
            : `确定将实例「${deleteConfirm.profileName || '未命名实例'}」移入回收站？3 天内可恢复。`}
        </div>
      </Modal>

      <Modal
        open={trashModalOpen}
        onClose={onCloseTrash}
        title="实例回收站"
        width="720px"
        footer={<Button variant="secondary" onClick={onCloseTrash}>关闭</Button>}
      >
        {trashLoading ? (
          <div className="py-10 text-center text-sm text-[var(--color-text-muted)]">加载中...</div>
        ) : trashProfiles.length === 0 ? (
          <div className="py-10 text-center text-sm text-[var(--color-text-muted)]">回收站为空</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border-default)] text-left text-xs text-[var(--color-text-muted)]">
                  <th className="py-2 pr-3 font-medium">实例</th>
                  <th className="py-2 pr-3 font-medium">删除时间</th>
                  <th className="py-2 pr-3 font-medium">自动清理</th>
                  <th className="py-2 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {trashProfiles.map((profile) => (
                  <tr key={profile.profileId} className="border-b border-[var(--color-border-muted)] last:border-0">
                    <td className="py-3 pr-3 text-[var(--color-text-primary)]">{profile.profileName || '未命名实例'}</td>
                    <td className="py-3 pr-3 text-[var(--color-text-secondary)]">{formatTime(profile.deletedAt)}</td>
                    <td className="py-3 pr-3 text-[var(--color-text-secondary)]">{formatExpireTime(profile.deletedAt)}</td>
                    <td className="py-3 text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => onRestoreProfile(profile.profileId)}
                          loading={restoringId === profile.profileId}
                          disabled={!!permanentlyDeletingId}
                        >
                          恢复
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => onOpenPermanentDelete(profile)}
                          loading={permanentlyDeletingId === profile.profileId}
                          disabled={!!restoringId}
                        >
                          彻底删除
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Modal>

      <Modal
        open={permanentDeleteConfirm.open}
        onClose={onClosePermanentDelete}
        title="彻底删除实例"
        width="420px"
        footer={
          <>
            <Button variant="secondary" onClick={onClosePermanentDelete} disabled={!!permanentlyDeletingId}>取消</Button>
            <Button variant="danger" onClick={onConfirmPermanentDelete} loading={!!permanentlyDeletingId}>彻底删除</Button>
          </>
        }
      >
        <div className="space-y-2 text-sm text-[var(--color-text-secondary)]">
          <p>确定彻底删除实例「{permanentDeleteConfirm.profile?.profileName || '未命名实例'}」？</p>
          <p className="text-red-500">这会删除配置、浏览器用户数据、快照、快捷码和插件绑定，删除后不可恢复。</p>
        </div>
      </Modal>

      <Modal
        open={profileImportPreview !== null}
        onClose={onCloseProfileImport}
        title="实例冲突"
        width="620px"
        footer={(
          <>
            <Button variant="secondary" onClick={onCloseProfileImport}>取消</Button>
            <Button variant="secondary" onClick={() => onConfirmProfileImport('new')}>新建实例</Button>
            <Button
              variant="danger"
              onClick={() => onConfirmProfileImport('overwrite')}
              disabled={!profileImportPreview?.canOverwrite}
              title={!profileImportPreview?.canOverwrite ? '存在无法安全判定的目标，无法自动覆盖' : undefined}
            >
              覆盖现有实例
            </Button>
          </>
        )}
      >
        {profileImportPreview && (
          <div className="space-y-3 text-sm">
            <div className="text-[var(--color-text-secondary)]">
              发现 {profileImportPreview.conflictCount} 个冲突，共 {profileImportPreview.profileCount} 个实例。
            </div>
            <div className="max-h-64 overflow-y-auto rounded-lg border border-[var(--color-border-default)]">
              {profileImportPreview.conflicts.map((conflict) => (
                <div key={`${conflict.sourceProfileId || conflict.sourceProfileName}-${conflict.targetProfileId || conflict.targetMatches}`} className="border-b border-[var(--color-border-muted)] px-3 py-2.5 last:border-0">
                  <div className="flex items-center justify-between gap-3">
                    <span className="min-w-0 truncate text-[var(--color-text-primary)]" title={conflict.sourceProfileName || conflict.sourceProfileId}>
                      {conflict.sourceProfileName || conflict.sourceProfileId || '未命名实例'}
                    </span>
                    <span className="shrink-0 text-xs text-[var(--color-text-muted)]">
                      {conflict.matchType === 'profileId' ? 'ID 匹配' : '名称匹配'}
                    </span>
                  </div>
                  {conflict.sourceNameCollision ? (
                    <div className="mt-1 text-xs text-[var(--color-error)]">实例包内有 {conflict.targetMatches} 个同名实例，无法自动覆盖</div>
                  ) : conflict.sourceTargetCollision ? (
                    <div className="mt-1 text-xs text-[var(--color-error)]">多个导入实例匹配同一目标，无法自动覆盖</div>
                  ) : conflict.ambiguous ? (
                    <div className="mt-1 text-xs text-[var(--color-error)]">存在 {conflict.targetMatches} 个同名实例，无法自动覆盖</div>
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
            {!profileImportPreview.canOverwrite && (
              <div className="text-xs text-[var(--color-warning)]">当前存在无法安全判定的目标，覆盖不可用；可选择新建实例。</div>
            )}
          </div>
        )}
      </Modal>

      <Modal
        open={!!opError}
        onClose={onCloseOpError}
        title="操作失败"
        width="420px"
        footer={<Button onClick={onCloseOpError}>知道了</Button>}
      >
        <div className="text-[var(--color-text-secondary)] whitespace-pre-line">{opError}</div>
      </Modal>
    </>
  )
}
