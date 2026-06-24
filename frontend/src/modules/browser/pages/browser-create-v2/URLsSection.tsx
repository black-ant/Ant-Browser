import { useState } from 'react'
import { ChevronDown, ChevronUp, Plus, X } from 'lucide-react'
import { Card, Input } from '../../../../shared/components'
import type { FingerprintProfileConfig } from './types'

interface URLsSectionProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
}

export function URLsSection({ config, updateConfig }: URLsSectionProps) {
  const [expanded, setExpanded] = useState(false)
  const [newUrl, setNewUrl] = useState('')

  const addUrl = () => {
    if (newUrl.trim()) {
      updateConfig(prev => ({
        ...prev,
        startupUrls: [...prev.startupUrls, newUrl.trim()]
      }))
      setNewUrl('')
    }
  }

  const removeUrl = (index: number) => {
    updateConfig(prev => ({
      ...prev,
      startupUrls: prev.startupUrls.filter((_, i) => i !== index)
    }))
  }

  return (
    <Card className="browser-create-accordion-card" padding="none">
      <button
        onClick={() => setExpanded(!expanded)}
        className="browser-create-accordion-toggle w-full p-6 flex items-center justify-between hover:bg-[var(--color-bg-muted)] transition-colors"
      >
        <h3 className="text-base font-semibold text-[var(--color-text-primary)]">URLs</h3>
        {expanded ? (
          <ChevronUp className="w-5 h-5 text-[var(--color-text-muted)]" />
        ) : (
          <ChevronDown className="w-5 h-5 text-[var(--color-text-muted)]" />
        )}
      </button>

      {expanded && (
        <div className="px-6 pb-6 border-t border-[var(--color-border-muted)]">
          <div className="pt-4 space-y-3">
            <p className="text-sm text-[var(--color-text-muted)]">
              启动浏览器时自动打开的网址列表
            </p>

            {/* URL 列表 */}
            {config.startupUrls.length > 0 && (
              <div className="space-y-2">
                {config.startupUrls.map((url, index) => (
                  <div key={index} className="flex items-center gap-2">
                    <Input
                      value={url}
                      onChange={(e) => {
                        updateConfig(prev => ({
                          ...prev,
                          startupUrls: prev.startupUrls.map((u, i) =>
                            i === index ? e.target.value : u
                          )
                        }))
                      }}
                      placeholder="https://example.com"
                      className="flex-1"
                    />
                    <button
                      type="button"
                      onClick={() => removeUrl(index)}
                      className="p-2 text-[var(--color-error)] hover:bg-[rgba(239,68,68,0.12)] rounded transition-colors"
                      title="删除"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            {/* 添加新 URL */}
            <div className="flex items-center gap-2">
              <Input
                value={newUrl}
                onChange={(e) => setNewUrl(e.target.value)}
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    addUrl()
                  }
                }}
                placeholder="输入网址，按 Enter 添加"
                className="flex-1"
              />
              <button
                type="button"
                onClick={addUrl}
                className="flex items-center gap-1 px-4 py-2 text-sm font-medium text-white bg-[#1890ff] hover:bg-[#1070d0] rounded-lg transition-colors"
              >
                <Plus className="w-4 h-4" />
                添加
              </button>
            </div>

            {config.startupUrls.length === 0 && (
              <div className="text-center py-8 text-[var(--color-text-muted)] text-sm">
                暂无启动网址
              </div>
            )}
          </div>
        </div>
      )}
    </Card>
  )
}
