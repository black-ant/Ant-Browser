import { useEffect, useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import { Button, SideDrawer, toast } from '../../../shared/components'
import { fetchAccounts, type BrowserAccount } from '../api'
import { platformIcon } from '../config/platformPresets'

interface AccountPickerModalProps {
  open: boolean
  selectedIds: string[]
  onConfirm: (ids: string[]) => void
  onClose: () => void
}

// 平台账号列表抽屉：从右侧滑出，勾选已存在的账号关联到当前窗口。
export function AccountPickerModal({ open, selectedIds, onConfirm, onClose }: AccountPickerModalProps) {
  const [accounts, setAccounts] = useState<BrowserAccount[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [checked, setChecked] = useState<string[]>([])

  useEffect(() => {
    if (!open) return
    setSearch('')
    setChecked(selectedIds)
    setLoading(true)
    fetchAccounts()
      .then(setAccounts)
      .catch((error: any) => toast.error(error?.message || '加载账号列表失败'))
      .finally(() => setLoading(false))
  }, [open, selectedIds])

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return accounts
    return accounts.filter(acc =>
      acc.accountName.toLowerCase().includes(query) ||
      acc.username.toLowerCase().includes(query) ||
      acc.email.toLowerCase().includes(query) ||
      acc.platform.toLowerCase().includes(query),
    )
  }, [accounts, search])

  const allChecked = filtered.length > 0 && filtered.every(acc => checked.includes(acc.accountId))

  const toggleOne = (accountId: string) => {
    setChecked(prev =>
      prev.includes(accountId) ? prev.filter(id => id !== accountId) : [...prev, accountId],
    )
  }

  const toggleAll = () => {
    if (allChecked) {
      const filteredIds = new Set(filtered.map(acc => acc.accountId))
      setChecked(prev => prev.filter(id => !filteredIds.has(id)))
    } else {
      setChecked(prev => Array.from(new Set([...prev, ...filtered.map(acc => acc.accountId)])))
    }
  }

  const handleSave = () => {
    onConfirm(checked)
    onClose()
  }

  return (
    <SideDrawer
      open={open}
      onClose={onClose}
      title="平台账号列表"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={handleSave}>保存{checked.length > 0 ? ` (${checked.length})` : ''}</Button>
        </>
      }
    >
      <div className="flex h-full flex-col">
        {/* 搜索栏 */}
        <div className="relative mb-3 flex-shrink-0">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]" />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="搜索 Ctrl + F"
            className="h-9 w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-input)] pl-10 pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] outline-none focus:border-[var(--color-accent)]"
          />
        </div>

        {/* 表头 */}
        <div className="flex items-center gap-3 border-b border-[var(--color-border-default)] px-2 py-2 text-xs font-medium text-[var(--color-text-muted)]">
          <div className="w-8">
            <input type="checkbox" checked={allChecked} onChange={toggleAll} className="h-4 w-4 accent-[var(--color-accent)]" />
          </div>
          <div className="w-32">平台</div>
          <div className="flex-1">账号</div>
          <div className="w-24">密码</div>
          <div className="w-28">备注</div>
          <div className="w-24 text-right">已关联窗口</div>
        </div>

        {/* 列表 */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex h-40 items-center justify-center text-sm text-[var(--color-text-muted)]">加载中...</div>
          ) : filtered.length === 0 ? (
            <div className="flex h-60 flex-col items-center justify-center gap-3 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-blue-50 text-3xl dark:bg-blue-900/30">🔒</div>
              <p className="text-sm text-[var(--color-text-muted)]">暂无平台账号</p>
            </div>
          ) : (
            filtered.map(acc => {
              const isChecked = checked.includes(acc.accountId)
              return (
                <label
                  key={acc.accountId}
                  className={`flex cursor-pointer items-center gap-3 border-b border-[var(--color-border-muted)] px-2 py-2.5 text-sm transition-colors ${
                    isChecked ? 'bg-[var(--color-accent)]/5' : 'hover:bg-[var(--color-bg-muted)]'
                  }`}
                >
                  <div className="w-8">
                    <input
                      type="checkbox"
                      checked={isChecked}
                      onChange={() => toggleOne(acc.accountId)}
                      className="h-4 w-4 accent-[var(--color-accent)]"
                    />
                  </div>
                  <div className="flex w-32 items-center gap-1.5 truncate">
                    <span className="text-base">{platformIcon(acc.platform)}</span>
                    <span className="truncate text-[var(--color-text-secondary)]">{acc.platform || '—'}</span>
                  </div>
                  <div className="min-w-0 flex-1 truncate text-[var(--color-text-primary)]">
                    {acc.accountName || acc.username || acc.email || '—'}
                  </div>
                  <div className="w-24 truncate text-[var(--color-text-muted)]">••••••</div>
                  <div className="w-28 truncate text-[var(--color-text-muted)]" title={acc.notes}>{acc.notes || '—'}</div>
                  <div className="w-24 text-right text-[var(--color-text-muted)]">{acc.relatedProfileIds?.length || 0}</div>
                </label>
              )
            })
          )}
        </div>

        {/* 底部统计 */}
        <div className="flex-shrink-0 border-t border-[var(--color-border-default)] pt-2 text-xs text-[var(--color-text-muted)]">
          总计 {filtered.length} 个 · 已选 {checked.length} 个
        </div>
      </div>
    </SideDrawer>
  )
}
