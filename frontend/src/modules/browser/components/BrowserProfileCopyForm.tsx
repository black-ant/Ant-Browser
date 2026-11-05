import clsx from 'clsx'
import { FormItem, Input } from '../../../shared/components'
import {
  BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS,
  isBrowserProfileCopyOptionsValid,
} from '../copyOptions'
import type { BrowserProfileAutomationTarget, BrowserProfileCopyOptions } from '../types'

interface BrowserProfileCopyFormProps {
  sourceName?: string
  copyName: string
  copyOptions: BrowserProfileCopyOptions
  onCopyNameChange: (value: string) => void
  onCopyOptionsChange: (value: BrowserProfileCopyOptions) => void
  autoFocusName?: boolean
}

export function BrowserProfileCopyForm({
  sourceName,
  copyName,
  copyOptions,
  onCopyNameChange,
  onCopyOptionsChange,
  autoFocusName = false,
}: BrowserProfileCopyFormProps) {
  const setMode = (mode: BrowserProfileCopyOptions['mode']) => {
    onCopyOptionsChange({
      ...copyOptions,
      mode,
    })
  }

  const toggleAutomationTarget = (target: BrowserProfileAutomationTarget) => {
    const nextTargets = copyOptions.automationTargets.includes(target)
      ? copyOptions.automationTargets.filter((item) => item !== target)
      : [...copyOptions.automationTargets, target]

    onCopyOptionsChange({
      ...copyOptions,
      automationTargets: nextTargets,
    })
  }

  const automationInvalid =
    copyOptions.mode === 'auto_fingerprint' && !isBrowserProfileCopyOptionsValid(copyOptions)
  const selectedAutomationCount = copyOptions.automationTargets.length

  return (
    <div className="space-y-4">
      <div className="space-y-4">
        {sourceName ? (
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm text-[var(--color-text-secondary)]">
            源实例：{sourceName}
          </div>
        ) : null}

        <FormItem label="新实例名称" required>
          <Input
            value={copyName}
            onChange={(event) => onCopyNameChange(event.target.value)}
            placeholder="请输入新实例名称"
            autoFocus={autoFocusName}
          />
        </FormItem>
      </div>

      <div className="space-y-3">
        <div className="text-sm font-medium text-[var(--color-text-secondary)]">复制方式</div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label
            className={clsx(
              'flex min-w-0 cursor-pointer items-center gap-3 rounded-lg border px-3 py-3 transition-colors',
              copyOptions.mode === 'regular'
                ? 'border-[var(--color-border-strong)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] shadow-[inset_0_0_0_1px_var(--color-border-strong)]'
                : 'border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)]',
            )}
          >
            <input
              type="radio"
              name="browser-profile-copy-mode"
              className="h-4 w-4 shrink-0 border-[var(--color-border-strong)] text-black focus:ring-black"
              checked={copyOptions.mode === 'regular'}
              onChange={() => setMode('regular')}
            />
            <span className="min-w-0">
              <span className="block text-sm font-medium text-[var(--color-text-primary)]">直接复制</span>
              <span className="block text-xs text-[var(--color-text-muted)]">保持当前复制方式</span>
            </span>
          </label>

          <label
            className={clsx(
              'flex min-w-0 cursor-pointer items-center gap-3 rounded-lg border px-3 py-3 transition-colors',
              copyOptions.mode === 'auto_fingerprint'
                ? 'border-[var(--color-border-strong)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] shadow-[inset_0_0_0_1px_var(--color-border-strong)]'
                : 'border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)]',
            )}
          >
            <input
              type="radio"
              name="browser-profile-copy-mode"
              className="h-4 w-4 shrink-0 border-[var(--color-border-strong)] text-black focus:ring-black"
              checked={copyOptions.mode === 'auto_fingerprint'}
              onChange={() => setMode('auto_fingerprint')}
            />
            <span className="min-w-0">
              <span className="block text-sm font-medium text-[var(--color-text-primary)]">自动化指纹</span>
              <span className="block text-xs text-[var(--color-text-muted)]">按勾选项自动处理</span>
            </span>
          </label>
        </div>

        {copyOptions.mode === 'auto_fingerprint' ? (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
            <FormItem
              label={
                <span className="flex items-center justify-between gap-3">
                  <span>自动化哪些指纹<span className="ml-0.5 text-[var(--color-error)]">*</span></span>
                  <span className="text-xs font-normal text-[var(--color-text-muted)]">已选 {selectedAutomationCount} 项</span>
                </span>
              }
              error={automationInvalid ? '请至少勾选一项' : undefined}
            >
              <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 xl:grid-cols-3">
                {BROWSER_PROFILE_AUTOMATION_TARGET_OPTIONS.map((option) => {
                  const checked = copyOptions.automationTargets.includes(option.value)
                  return (
                    <label
                      key={option.value}
                      className={clsx(
                        'flex min-w-0 cursor-pointer items-start gap-3 rounded-lg border px-3 py-3 transition-colors',
                        checked
                          ? 'border-[var(--color-border-strong)] bg-[var(--color-bg-secondary)] shadow-[inset_0_0_0_1px_var(--color-border-strong)]'
                          : 'border-[var(--color-border-default)] bg-[var(--color-bg-surface)] hover:border-[var(--color-border-strong)]',
                      )}
                    >
                      <input
                        type="checkbox"
                        className="mt-0.5 h-4 w-4 shrink-0 rounded border-[var(--color-border-strong)] text-black focus:ring-black"
                        checked={checked}
                        onChange={() => toggleAutomationTarget(option.value)}
                      />
                      <span className="min-w-0">
                        <span className="block text-sm font-medium leading-5 text-[var(--color-text-primary)]">
                          {option.label}
                        </span>
                        <span className="mt-0.5 block text-xs leading-4 text-[var(--color-text-muted)]">
                          {option.detail}
                        </span>
                      </span>
                    </label>
                  )
                })}
              </div>
            </FormItem>
          </div>
        ) : null}
      </div>
    </div>
  )
}
