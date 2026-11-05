import { Card } from '../../../../shared/components'

interface LaunchDocsFlowPageProps {
  baseUrl: string
}

interface FlowStep {
  step: string
  title: string
  path: string
  example?: string
}

export function LaunchDocsFlowPage({ baseUrl }: LaunchDocsFlowPageProps) {
  const steps: FlowStep[] = [
    {
      step: '01',
      title: '内核下载',
      path: '指纹浏览器 -> 内核管理 -> 下载内核 -> 设为默认',
      example: `chrome/
  chrome-<version>/
    chrome.exe`,
    },
    {
      step: '02',
      title: '代理绑定',
      path: '指纹浏览器 -> 代理池配置 -> 导入 Clash YAML / 录入 HTTP(S) / SOCKS5',
      example: `proxies:
  - name: hk-vless
    type: vless
    server: example.com
    port: 443`,
    },
    {
      step: '03',
      title: '实例创建',
      path: '指纹浏览器 -> 实例列表 -> 新建配置 -> 选择内核 / 代理 -> 保存',
      example: `{
  "profile": {
    "profileName": "buyer-001",
    "proxyId": "proxy-us",
    "keywords": ["buyer-001"]
  },
  "launchCode": "BUYER_001"
}`,
    },
    {
      step: '04',
      title: '实例触发',
      path: '指纹浏览器 -> 实例列表 -> 启动',
      example: `GET ${baseUrl}/api/health
POST ${baseUrl}/api/launch`,
    },
    {
      step: '05',
      title: '接口调用',
      path: '外部脚本 -> Launch API -> ant-chrome -> 浏览器实例',
      example: `curl -X POST ${baseUrl}/api/runtime/session \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": { "code": "BUYER_001" },
    "skipDefaultStartUrls": true
  }'`,
    },
  ]

  return (
    <div className="space-y-4">
      <Card className="bg-[var(--color-bg-elevated)] shadow-[var(--shadow-sm)]">
        <div className="space-y-1">
          <div className="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-text-muted)]">
            操作流程
          </div>
          <h1 className="text-2xl font-bold text-[var(--color-text-primary)]">
            从配置到调用
          </h1>
        </div>
      </Card>

      <Card className="bg-[var(--color-bg-elevated)] shadow-[var(--shadow-sm)]">
        <div className="relative">
          <div className="absolute bottom-4 left-[20px] top-4 w-px bg-[var(--color-border-default)]" />
          <div className="space-y-4">
            {steps.map((step) => (
              <section key={step.step} className="relative flex gap-3">
                <div className="relative z-10 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-[var(--color-accent)] text-sm font-semibold text-[var(--color-text-inverse)] shadow-[var(--shadow-sm)]">
                  {step.step}
                </div>
                <div className="min-w-0 flex-1 space-y-3 pb-2">
                  <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">
                    {step.title}
                  </h2>

                  <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3">
                    <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-text-muted)]">
                      操作路径
                    </div>
                    <div className="mt-2 text-sm text-[var(--color-text-primary)]">
                      {step.path}
                    </div>
                  </div>

                  {step.example ? (
                    <pre className="overflow-x-auto rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-3 text-xs leading-6 text-[var(--color-text-secondary)]">
{step.example}
                    </pre>
                  ) : null}
                </div>
              </section>
            ))}
          </div>
        </div>
      </Card>
    </div>
  )
}
