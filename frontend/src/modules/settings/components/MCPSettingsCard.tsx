import { useState } from 'react'
import { Check, Copy } from 'lucide-react'

import { Badge, Button, Card, Switch } from '../../../shared/components'

import type { MCPServerSettings } from '../api'

interface MCPSettingsCardProps {
  mcp: MCPServerSettings
  mcpSaving: boolean
  onEnabledChange: (enabled: boolean) => void
}

type ClientTransport = 'http' | 'stdio'

const TRANSPORT_OPTIONS: Array<{ value: ClientTransport, label: string, hint: string }> = [
  { value: 'http', label: 'HTTP', hint: '适用于支持远程 MCP 的客户端，例如 Claude Code、Cursor' },
  { value: 'stdio', label: 'stdio', hint: '适用于只支持本地子进程的客户端' },
]

/**
 * 生成可直接粘贴到客户端配置文件里的 JSON 片段。
 *
 * 直接给完整片段而不是让用户自己拼：MCP 客户端配置格式对新手不直观，
 * 而且鉴权开启时还要手动加 header，容易漏。
 */
function buildClientConfig(transport: ClientTransport, mcp: MCPServerSettings): string {
  if (transport === 'stdio') {
    return JSON.stringify({
      mcpServers: {
        'ant-browser': {
          command: mcp.executablePath || '<Ant Browser 可执行文件路径>',
          args: ['--mcp-stdio'],
        },
      },
    }, null, 2)
  }

  const server: Record<string, unknown> = {
    type: 'http',
    url: mcp.url || '<启用 MCP 服务后自动生成>',
  }
  if (mcp.authEnabled) {
    server.headers = { [mcp.authHeader]: '<你的 API Key>' }
  }
  return JSON.stringify({ mcpServers: { 'ant-browser': server } }, null, 2)
}

export function MCPSettingsCard({ mcp, mcpSaving, onEnabledChange }: MCPSettingsCardProps) {
  const [transport, setTransport] = useState<ClientTransport>('http')
  const [copied, setCopied] = useState(false)

  const config = buildClientConfig(transport, mcp)
  const activeHint = TRANSPORT_OPTIONS.find(item => item.value === transport)?.hint || ''

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(config)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      // 复制失败时保持静默：配置内容本身可见，用户可以手动选中复制。
    }
  }

  return (
    <Card title="MCP 服务">
      <div className="space-y-5">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <p className="text-sm font-medium text-[var(--color-text-primary)]">启用 MCP 服务</p>
              <Badge variant={mcp.ready ? 'success' : 'warning'} size="sm" dot>
                {mcp.ready ? '已就绪' : mcp.enabled ? '未就绪' : '已关闭'}
              </Badge>
            </div>
            <p className="mt-1 text-xs text-[var(--color-text-muted)]">
              让 AI 客户端通过标准协议管理实例、执行自动化脚本和查看代理池，
              共用本地 API 端口与鉴权设置。
            </p>
          </div>
          <Switch
            checked={mcp.enabled}
            onChange={onEnabledChange}
            disabled={mcpSaving}
          />
        </div>

        {mcp.enabled && (
          <>
            <div className="h-px bg-[var(--color-border-muted)]" />

            <div className="flex flex-wrap items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
              <span>端点</span>
              <code className="text-[var(--color-text-primary)]">{mcp.url || '服务未就绪'}</code>
              <span className="text-[var(--color-text-muted)]">·</span>
              <span>{mcp.toolCount} 个工具</span>
              {mcp.authEnabled && (
                <>
                  <span className="text-[var(--color-text-muted)]">·</span>
                  <span>需携带 <code className="text-[var(--color-text-primary)]">{mcp.authHeader}</code></span>
                </>
              )}
            </div>

            <div className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-1">
                  {TRANSPORT_OPTIONS.map(option => (
                    <Button
                      key={option.value}
                      size="sm"
                      variant={transport === option.value ? 'primary' : 'secondary'}
                      onClick={() => setTransport(option.value)}
                    >
                      {option.label}
                    </Button>
                  ))}
                </div>
                <Button size="sm" variant="secondary" onClick={() => { void handleCopy() }}>
                  {copied
                    ? <><Check size={14} className="mr-1" />已复制</>
                    : <><Copy size={14} className="mr-1" />复制配置</>}
                </Button>
              </div>

              <p className="text-xs text-[var(--color-text-muted)]">{activeHint}</p>

              <pre className="overflow-x-auto rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                {config}
              </pre>
            </div>
          </>
        )}
      </div>
    </Card>
  )
}
