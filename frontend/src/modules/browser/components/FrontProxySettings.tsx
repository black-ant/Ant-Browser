import { useState } from 'react'
import { CheckCircle2, Search, XCircle } from 'lucide-react'
import { Button, Input, Select, Switch, toast } from '../../../shared/components'
import { scanLocalProxy } from '../api'
import type { LocalProxyScanResult } from '../types'

interface FrontProxySettingsProps {
  enabled: boolean
  auto: boolean
  addr: string
  onChange: (patch: Partial<{ frontProxyEnabled: boolean; frontProxyAuto: boolean; frontProxyAddr: string }>) => void
}

// FrontProxySettings 「全局前置代理」设置区块：开关 + 自动检测/固定地址切换 + 立即检测。
// 受控组件，自身只管理一次性的本地代理扫描结果，配置值由父级 settings 持有。
export function FrontProxySettings({ enabled, auto, addr, onChange }: FrontProxySettingsProps) {
  const [scanning, setScanning] = useState(false)
  const [scanResult, setScanResult] = useState<LocalProxyScanResult | null>(null)

  const handleScan = async () => {
    setScanning(true)
    try {
      const res = await scanLocalProxy()
      setScanResult(res)
      if (res.found) {
        // 固定模式且地址为空时，自动填入推荐地址，省去手填
        if (!auto && !(addr || '').trim()) onChange({ frontProxyAddr: res.best })
        toast.success(`检测到 ${res.candidates.length} 个本地代理`)
      } else {
        toast.error(res.error || '未检测到本地代理')
      }
    } catch (error: any) {
      toast.error(error?.message || '检测失败')
    } finally {
      setScanning(false)
    }
  }

  const activeAddr = auto ? scanResult?.best : addr

  return (
    <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2">
      <div className="flex flex-col gap-2 xl:flex-row xl:items-center">
        <div className="flex min-w-[230px] items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="text-sm font-medium leading-5 text-[var(--color-text-primary)]">全局前置代理</div>
            <div className="truncate text-[11px] leading-4 text-[var(--color-text-muted)]">
              带账密标准代理的启动和测速先走本地出口
            </div>
          </div>
          <Switch checked={!!enabled} onChange={v => onChange({ frontProxyEnabled: v })} />
        </div>

        {enabled && (
          <div className="grid min-w-0 flex-1 grid-cols-1 gap-2 md:grid-cols-[180px_minmax(220px,1fr)_80px] xl:items-center">
            <Select
              value={auto ? 'auto' : 'fixed'}
              onChange={e => onChange({ frontProxyAuto: e.target.value === 'auto' })}
              options={[
                { value: 'auto', label: '自动检测本地代理' },
                { value: 'fixed', label: '固定地址' },
              ]}
            />

            {auto ? (
              <div className="flex h-8 min-w-0 items-center rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] px-3 text-xs text-[var(--color-text-secondary)]">
                <span className="truncate">
                  {scanResult?.found ? `推荐：${scanResult.best}` : '每次桥接/测速时实时扫描 Clash、v2rayN、sing-box 常见端口'}
                </span>
              </div>
            ) : (
              <Input
                value={addr || ''}
                onChange={e => onChange({ frontProxyAddr: e.target.value })}
                placeholder="socks5://127.0.0.1:7891"
              />
            )}

            <Button
              size="sm"
              variant="secondary"
              onClick={handleScan}
              loading={scanning}
              title="检测本机常见前置代理端口"
              className="w-full"
            >
              <Search className="h-4 w-4" />检测
            </Button>
          </div>
        )}
      </div>

      {enabled && scanResult && (scanResult.found ? (
        <div className="mt-2 flex flex-wrap items-center gap-1.5 border-t border-[var(--color-border-muted)] pt-2">
          <span className="inline-flex items-center gap-1 text-[11px] text-[var(--color-success)]">
            <CheckCircle2 className="h-3.5 w-3.5" />
            {scanResult.candidates.length} 个候选
          </span>
          {scanResult.candidates.map(c => {
            const active = c.addr === activeAddr
            return (
              <button
                key={c.addr}
                type="button"
                onClick={() => { if (!auto) onChange({ frontProxyAddr: c.addr }) }}
                className={`rounded-md border px-2 py-0.5 text-[11px] transition-colors ${auto ? 'cursor-default' : 'cursor-pointer hover:border-[var(--color-accent)]'} ${active ? 'border-[var(--color-accent)] text-[var(--color-accent)]' : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)]'}`}
              >
                {c.addr}<span className="ml-1 opacity-60">{c.protocol}</span>
              </button>
            )
          })}
        </div>
      ) : (
        <div className="mt-2 flex items-center gap-1 border-t border-[var(--color-border-muted)] pt-2 text-[11px] text-[var(--color-error)]">
          <XCircle className="h-3.5 w-3.5" />{scanResult.error || '未检测到本地代理'}
        </div>
      ))}
    </div>
  )
}
