import { useEffect, useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, GitBranch, Globe2, Plus, RefreshCw, ShieldBan, Trash2, Unplug, WifiOff } from 'lucide-react'
import { Button, Input, Modal, Select, Switch, toast } from '../../../shared/components'
import {
  fetchBrowserProxyGatewayStatus,
  fetchBrowserProxyRouting,
  saveBrowserProxyRouting,
} from '../api'
import type { ProxyGatewayStatus, ProxyRoutingConfig, ProxyRoutingRule } from '../types'

interface ProxyRoutingModalProps {
  open: boolean
  profileId: string
  profileName?: string
  running?: boolean
  onClose: () => void
  onSaved?: (config: ProxyRoutingConfig, status: ProxyGatewayStatus | null) => void
}

const MODE_OPTIONS = [
  { value: 'proxy', label: '全部代理', icon: Globe2, description: '所有新连接都通过当前代理' },
  { value: 'rule', label: '规则分流', icon: GitBranch, description: '按顺序匹配规则，未命中时走代理' },
  { value: 'direct', label: '全部直连', icon: WifiOff, description: '所有新连接都绕过代理' },
] as const

const MATCH_OPTIONS = [
  { value: 'domain', label: '精确域名' },
  { value: 'domain_suffix', label: '域名后缀' },
  { value: 'domain_keyword', label: '域名关键词' },
  { value: 'ip_cidr', label: 'IP / CIDR（字面 IP）' },
]

const ACTION_OPTIONS = [
  { value: 'proxy', label: '代理' },
  { value: 'direct', label: '直连' },
  { value: 'block', label: '阻断' },
]

const MATCH_PLACEHOLDERS: Record<string, string> = {
  domain: 'api.example.com',
  domain_suffix: '.example.com',
  domain_keyword: 'google',
  ip_cidr: '192.168.0.0/16',
}

function newRule(index: number): ProxyRoutingRule {
  const id = typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID()
    : `rule-${Date.now()}-${index}`
  return {
    id,
    name: '',
    enabled: true,
    matchType: 'domain_suffix',
    pattern: '',
    action: 'proxy',
  }
}

function statusText(status: ProxyGatewayStatus | null, running: boolean): string {
  if (!running) return '实例未运行，保存后将在下次启动时生效'
  if (!status) return '正在读取网关状态'
  return `活动连接 ${status.activeConnections} · 旧连接排空 ${status.drainingConnections}`
}

export function ProxyRoutingModal({ open, profileId, profileName, running = false, onClose, onSaved }: ProxyRoutingModalProps) {
  const [config, setConfig] = useState<ProxyRoutingConfig>({ mode: 'proxy', rules: [] })
  const [status, setStatus] = useState<ProxyGatewayStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [forceDisconnect, setForceDisconnect] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open || !profileId) return
    let cancelled = false
    setLoading(true)
    setError('')
    setForceDisconnect(false)
    const load = async () => {
      const routing = await fetchBrowserProxyRouting(profileId)
      if (!cancelled) setConfig({ mode: routing.mode || 'proxy', rules: routing.rules || [] })
      if (running) {
        try {
          const gateway = await fetchBrowserProxyGatewayStatus(profileId)
          if (!cancelled) setStatus(gateway)
        } catch {
          if (!cancelled) setStatus(null)
        }
      } else if (!cancelled) {
        setStatus(null)
      }
    }
    void load().catch((reason: any) => {
      if (!cancelled) setError(reason?.message || '分流配置读取失败')
    }).finally(() => {
      if (!cancelled) setLoading(false)
    })
    return () => { cancelled = true }
  }, [open, profileId, running])

  useEffect(() => {
    if (!open || !running || !profileId) return
    const timer = window.setInterval(() => {
      void fetchBrowserProxyGatewayStatus(profileId).then(setStatus).catch(() => undefined)
    }, 2500)
    return () => window.clearInterval(timer)
  }, [open, profileId, running])

  const enabledRuleCount = useMemo(() => config.rules.filter(rule => rule.enabled && rule.pattern.trim()).length, [config.rules])

  const updateRule = (index: number, patch: Partial<ProxyRoutingRule>) => {
    setConfig(previous => ({
      ...previous,
      rules: previous.rules.map((rule, ruleIndex) => ruleIndex === index ? { ...rule, ...patch } : rule),
    }))
  }

  const moveRule = (index: number, offset: -1 | 1) => {
    const target = index + offset
    if (target < 0 || target >= config.rules.length) return
    setConfig(previous => {
      const rules = [...previous.rules]
      const current = rules[index]
      rules[index] = rules[target]
      rules[target] = current
      return { ...previous, rules }
    })
  }

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      const nextConfig = { ...config, rules: config.rules.filter(rule => rule.pattern.trim()) }
      const nextStatus = await saveBrowserProxyRouting(profileId, nextConfig, forceDisconnect)
      setConfig(nextConfig)
      setStatus(nextStatus)
      onSaved?.(nextConfig, nextStatus)
      const suffix = nextStatus?.drainingConnections ? `，${nextStatus.drainingConnections} 条旧连接正在排空` : ''
      toast.success(`分流规则已${running ? '热' : ''}保存${suffix}`)
    } catch (reason: any) {
      const message = reason?.message || '分流规则保存失败，原配置保持不变'
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`代理分流${profileName ? `：${profileName}` : ''}`}
      width="1040px"
      footer={(
        <>
          <div className="mr-auto flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
            <Unplug className="h-3.5 w-3.5" />
            <span>{statusText(status, running)}</span>
          </div>
          <Button type="button" variant="secondary" onClick={onClose}>取消</Button>
          <Button type="button" onClick={() => { void save() }} loading={saving} disabled={loading || !profileId}>保存分流</Button>
        </>
      )}
    >
      <div className="space-y-4">
        {error && <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}

        <div className="grid grid-cols-1 gap-2 md:grid-cols-3">
          {MODE_OPTIONS.map(option => {
            const Icon = option.icon
            const selected = config.mode === option.value
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => setConfig(previous => ({ ...previous, mode: option.value }))}
                className={`flex min-h-[72px] items-start gap-2 rounded-lg border px-3 py-2 text-left transition-colors ${selected
                  ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)] text-[var(--color-text-primary)]'
                  : 'border-[var(--color-border)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)]'}`}
                aria-pressed={selected}
              >
                <Icon className="mt-0.5 h-4 w-4 shrink-0" />
                <span className="min-w-0">
                  <span className="block text-sm font-medium">{option.label}</span>
                  <span className="mt-1 block text-xs leading-4 text-[var(--color-text-muted)]">{option.description}</span>
                </span>
              </button>
            )
          })}
        </div>

        {config.mode === 'rule' && (
          <div className="overflow-hidden rounded-lg border border-[var(--color-border)]">
            <div className="flex items-center justify-between gap-3 border-b border-[var(--color-border)] bg-[var(--color-bg-muted)] px-3 py-2">
              <div className="min-w-0">
                <div className="text-sm font-medium text-[var(--color-text-primary)]">有序规则</div>
                <div className="text-xs leading-5 text-[var(--color-text-muted)]">
                  从上到下匹配，第一条命中后即停止；未命中时默认代理。域名后缀的前导点可省略（cn 与 .cn 相同）；IP/CIDR 只匹配浏览器提交的字面 IP，不会额外解析域名。已启用 {enabledRuleCount} 条
                </div>
              </div>
              <Button className="shrink-0" type="button" variant="secondary" size="sm" onClick={() => setConfig(previous => ({ ...previous, rules: [...previous.rules, newRule(previous.rules.length)] }))}>
                <Plus className="h-3.5 w-3.5" />
                添加规则
              </Button>
            </div>
            <div className="divide-y divide-[var(--color-border)]">
              {config.rules.length === 0 ? (
                <div className="px-3 py-6 text-center text-xs text-[var(--color-text-muted)]">暂无规则，未命中请求会走代理</div>
              ) : config.rules.map((rule, index) => (
                <div key={rule.id || index} className="grid grid-cols-1 gap-2 px-3 py-3 lg:grid-cols-[96px_minmax(140px,0.75fr)_minmax(390px,2fr)_minmax(105px,0.6fr)_96px] lg:items-center">
                  <div className="flex items-center gap-2 whitespace-nowrap text-xs text-[var(--color-text-muted)]" title={rule.enabled ? '点击停用这条规则' : '点击启用这条规则'}>
                    <span className="w-4 shrink-0 text-center">{index + 1}</span>
                    <Switch checked={rule.enabled} onChange={enabled => updateRule(index, { enabled })} />
                    <span className="min-w-[24px]">{rule.enabled ? '启用' : '停用'}</span>
                  </div>
                  <Input value={rule.name} onChange={event => updateRule(index, { name: event.target.value })} placeholder="规则名称（可选）" aria-label="规则名称" />
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(150px,0.8fr)_minmax(220px,1.2fr)]">
                    <Select value={rule.matchType} onChange={event => updateRule(index, { matchType: event.target.value })} options={MATCH_OPTIONS} aria-label="匹配类型" />
                    <Input value={rule.pattern} onChange={event => updateRule(index, { pattern: event.target.value })} placeholder={MATCH_PLACEHOLDERS[rule.matchType] || '请输入匹配内容'} aria-label="匹配内容" />
                  </div>
                  <Select value={rule.action} onChange={event => updateRule(index, { action: event.target.value })} options={ACTION_OPTIONS} aria-label="规则动作" />
                  <div className="flex items-center justify-end gap-1">
                    <button type="button" className="rounded p-1.5 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)] disabled:opacity-30" onClick={() => moveRule(index, -1)} disabled={index === 0} title="上移规则" aria-label="上移规则"><ArrowUp className="h-4 w-4" /></button>
                    <button type="button" className="rounded p-1.5 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)] disabled:opacity-30" onClick={() => moveRule(index, 1)} disabled={index === config.rules.length - 1} title="下移规则" aria-label="下移规则"><ArrowDown className="h-4 w-4" /></button>
                    <button type="button" className="rounded p-1.5 text-[var(--color-text-muted)] hover:bg-red-50 hover:text-red-600" onClick={() => setConfig(previous => ({ ...previous, rules: previous.rules.filter((_, ruleIndex) => ruleIndex !== index) }))} title="删除规则" aria-label="删除规则"><Trash2 className="h-4 w-4" /></button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {running && (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-border)] px-3 py-2">
            <div className="flex min-w-0 items-start gap-2">
              <ShieldBan className="mt-0.5 h-4 w-4 shrink-0 text-[var(--color-text-muted)]" />
              <div className="min-w-0">
                <div className="text-sm text-[var(--color-text-primary)]">强制断开旧连接</div>
                <div className="text-xs leading-4 text-[var(--color-text-muted)]">开启后保存时会立即关闭旧代理连接；关闭则让旧连接自然完成。</div>
              </div>
            </div>
            <Switch checked={forceDisconnect} onChange={setForceDisconnect} />
          </div>
        )}

        {loading && <div className="flex items-center gap-2 text-xs text-[var(--color-text-muted)]"><RefreshCw className="h-3.5 w-3.5 animate-spin" />正在读取当前分流配置</div>}
      </div>
    </Modal>
  )
}
