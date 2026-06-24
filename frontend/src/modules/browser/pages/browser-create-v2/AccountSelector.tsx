import { useState, useEffect } from 'react'
import { Search, X, ChevronDown, Users, Check } from 'lucide-react'
import { BrowserAccountList } from '../../../../wailsjs/go/main/App'
import type { BrowserAccount } from '../../api'

interface AccountSelectorProps {
  selectedIds: string[]
  onChange: (ids: string[]) => void
  disabled?: boolean
}

export function AccountSelector({ selectedIds, onChange, disabled = false }: AccountSelectorProps) {
  const [accounts, setAccounts] = useState<BrowserAccount[]>([])
  const [loading, setLoading] = useState(false)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  // 加载账号列表
  useEffect(() => {
    loadAccounts()
  }, [])

  const loadAccounts = async () => {
    setLoading(true)
    try {
      const result = await BrowserAccountList()
      setAccounts(result || [])
    } catch (error) {
      console.error('加载账号列表失败:', error)
    } finally {
      setLoading(false)
    }
  }

  // 过滤账号
  const filteredAccounts = accounts.filter(account => {
    if (!searchQuery) return true
    const query = searchQuery.toLowerCase()
    return (
      account.accountName?.toLowerCase().includes(query) ||
      account.username?.toLowerCase().includes(query) ||
      account.platform?.toLowerCase().includes(query)
    )
  })

  // 选中的账号
  const selectedAccounts = accounts.filter(account =>
    selectedIds.includes(account.accountId)
  )

  // 切换选中状态
  const toggleAccount = (accountId: string) => {
    if (selectedIds.includes(accountId)) {
      onChange(selectedIds.filter(id => id !== accountId))
    } else {
      onChange([...selectedIds, accountId])
    }
  }

  // 移除账号
  const removeAccount = (accountId: string, e?: React.MouseEvent) => {
    e?.stopPropagation()
    onChange(selectedIds.filter(id => id !== accountId))
  }

  // 清空所有
  const clearAll = (e: React.MouseEvent) => {
    e.stopPropagation()
    onChange([])
  }

  return (
    <div className="relative">
      {/* 主输入框 - 现代化设计 */}
      <div
        className={`
          group relative min-h-[48px] px-4 py-2.5
          bg-[var(--color-bg-input)] border-2 rounded-xl
          cursor-pointer transition-all duration-200
          ${dropdownOpen
            ? 'border-[var(--color-accent)] shadow-lg shadow-[var(--color-accent)]/10 ring-4 ring-[var(--color-accent)]/5'
            : 'border-[var(--color-border-default)] hover:border-[var(--color-border-strong)]'
          }
          ${disabled ? 'opacity-50 cursor-not-allowed bg-[var(--color-bg-muted)]' : ''}
        `}
        onClick={() => !disabled && setDropdownOpen(!dropdownOpen)}
      >
        <div className="flex items-center gap-2.5 flex-wrap">
          {/* 图标 */}
          <div className={`
            flex-shrink-0 w-8 h-8 rounded-lg flex items-center justify-center
            ${selectedAccounts.length > 0
              ? 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]'
              : 'bg-[var(--color-bg-muted)] text-[var(--color-text-muted)]'
            }
          `}>
            <Users className="w-4 h-4" />
          </div>

          {/* 已选中的账号标签 */}
          <div className="flex-1 min-w-0 flex items-center gap-2 flex-wrap">
            {selectedAccounts.length > 0 ? (
              selectedAccounts.map(account => (
                <div
                  key={account.accountId}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-[var(--color-accent)]/10 text-[var(--color-accent)] rounded-lg text-sm font-medium transition-colors hover:bg-[var(--color-accent)]/20"
                  onClick={(e) => e.stopPropagation()}
                >
                  <span className="max-w-[120px] truncate">
                    {account.accountName || account.username}
                  </span>
                  {!disabled && (
                    <button
                      onClick={(e) => removeAccount(account.accountId, e)}
                      className="hover:bg-[var(--color-accent)]/30 rounded p-0.5 transition-colors"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  )}
                </div>
              ))
            ) : (
              <span className="text-[var(--color-text-muted)] text-sm">选择账号...</span>
            )}
          </div>

          {/* 右侧操作按钮 */}
          <div className="flex-shrink-0 flex items-center gap-2">
            {selectedAccounts.length > 0 && !disabled && (
              <button
                onClick={clearAll}
                className="p-1.5 hover:bg-[var(--color-bg-muted)] rounded-lg transition-colors"
                title="清空"
              >
                <X className="w-4 h-4 text-[var(--color-text-muted)]" />
              </button>
            )}
            <div className={`transition-transform duration-200 ${dropdownOpen ? 'rotate-180' : ''}`}>
              <ChevronDown className="w-5 h-5 text-[var(--color-text-muted)]" />
            </div>
          </div>
        </div>
      </div>

      {/* 下拉列表 - 现代化设计 */}
      {dropdownOpen && (
        <>
          {/* 遮罩层 */}
          <div
            className="fixed inset-0 z-10"
            onClick={() => setDropdownOpen(false)}
          />

          {/* 下拉内容 */}
          <div className="absolute z-20 mt-2 w-full bg-[var(--color-bg-elevated)] border border-[var(--color-border-default)] rounded-xl shadow-2xl max-h-[380px] flex flex-col overflow-hidden animate-in fade-in slide-in-from-top-2 duration-200">
            {/* 搜索框 */}
            <div className="p-3 border-b border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="搜索账号名称、用户名或平台..."
                  className="w-full pl-10 pr-3 py-2.5 bg-[var(--color-bg-input)] border border-[var(--color-border-default)] rounded-lg text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:border-[var(--color-accent)] focus:ring-2 focus:ring-[var(--color-accent)]/20 transition-all"
                  onClick={(e) => e.stopPropagation()}
                  autoFocus
                />
              </div>
            </div>

            {/* 账号列表 */}
            <div className="overflow-y-auto flex-1">
              {loading ? (
                <div className="p-8 text-center">
                  <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[var(--color-bg-muted)] mb-3">
                    <div className="w-6 h-6 border-2 border-[var(--color-accent)] border-t-transparent rounded-full animate-spin"></div>
                  </div>
                  <p className="text-sm text-[var(--color-text-muted)]">加载中...</p>
                </div>
              ) : filteredAccounts.length === 0 ? (
                <div className="p-8 text-center">
                  <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[var(--color-bg-muted)] mb-3">
                    <Users className="w-6 h-6 text-[var(--color-text-muted)]" />
                  </div>
                  <p className="text-sm text-[var(--color-text-secondary)] font-medium">
                    {searchQuery ? '未找到匹配的账号' : '暂无账号'}
                  </p>
                  <p className="text-xs text-[var(--color-text-muted)] mt-1">
                    {searchQuery ? '尝试其他关键词' : '请先添加账号'}
                  </p>
                </div>
              ) : (
                <div className="p-2">
                  {filteredAccounts.map(account => {
                    const isSelected = selectedIds.includes(account.accountId)
                    return (
                      <div
                        key={account.accountId}
                        className={`
                          group relative px-3 py-2.5 cursor-pointer transition-all duration-150 rounded-lg
                          ${isSelected
                            ? 'bg-[var(--color-accent)]/10 hover:bg-[var(--color-accent)]/15'
                            : 'hover:bg-[var(--color-bg-muted)]'
                          }
                        `}
                        onClick={(e) => {
                          e.stopPropagation()
                          toggleAccount(account.accountId)
                        }}
                      >
                        <div className="flex items-center gap-3">
                          {/* 复选框样式 */}
                          <div className="flex-shrink-0">
                            <div
                              className={`
                                w-5 h-5 border-2 rounded-md flex items-center justify-center transition-all duration-150
                                ${isSelected
                                  ? 'bg-[var(--color-accent)] border-[var(--color-accent)] scale-100'
                                  : 'border-[var(--color-border-default)] group-hover:border-[var(--color-accent)] scale-95'
                                }
                              `}
                            >
                              {isSelected && (
                                <Check className="w-3.5 h-3.5 text-white animate-in zoom-in duration-150" strokeWidth={3} />
                              )}
                            </div>
                          </div>

                          {/* 账号信息 */}
                          <div className="flex-1 min-w-0">
                            <div className="text-sm font-medium text-[var(--color-text-primary)] truncate">
                              {account.accountName || account.username || '未命名账号'}
                            </div>
                            {(account.platform || account.username) && (
                              <div className="flex items-center gap-1.5 mt-0.5 text-xs text-[var(--color-text-muted)]">
                                {account.platform && (
                                  <span className="px-1.5 py-0.5 bg-[var(--color-bg-muted)] rounded">
                                    {account.platform}
                                  </span>
                                )}
                                {account.username && (
                                  <span className="truncate">
                                    {account.username}
                                  </span>
                                )}
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            {/* 底部统计 */}
            {filteredAccounts.length > 0 && (
              <div className="px-4 py-2.5 border-t border-[var(--color-border-default)] bg-[var(--color-bg-surface)] flex items-center justify-between">
                <span className="text-xs text-[var(--color-text-muted)]">
                  已选 <span className="font-semibold text-[var(--color-accent)]">{selectedAccounts.length}</span> / {filteredAccounts.length} 个账号
                </span>
                {selectedAccounts.length > 0 && (
                  <button
                    onClick={clearAll}
                    className="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] transition-colors"
                  >
                    清空选择
                  </button>
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
