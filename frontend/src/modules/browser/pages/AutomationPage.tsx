import { useEffect, useState } from 'react'
import { Bot, Copy, Rocket } from 'lucide-react'
import { Button, Card, toast } from '../../../shared/components'
import { fetchLaunchServerInfo, type LaunchServerInfo } from '../api'

const DEFAULT_LAUNCH_BASE_URL = 'http://127.0.0.1:19876'
const DEFAULT_API_AUTH: LaunchServerInfo['apiAuth'] = {
  requested: false,
  configured: false,
  enabled: false,
  header: 'X-Ant-Api-Key',
}

function buildAuthHeaderLine(apiAuth: LaunchServerInfo['apiAuth']): string {
  if (!apiAuth.enabled) return ''
  return `  -H "${apiAuth.header}: <your-api-key>" \\\n`
}

function buildSampleCreateRequest(baseUrl: string, apiAuth: LaunchServerInfo['apiAuth']): string {
  return `curl -X POST ${baseUrl}/api/profiles \\
  -H "Content-Type: application/json" \\
${buildAuthHeaderLine(apiAuth)}  -d '{
    "profile": {
      "profileName": "buyer-001",
      "userDataDir": "buyers/buyer-001",
      "proxyId": "proxy-us",
      "launchArgs": ["--lang=en-US"],
      "tags": ["电商", "北美"],
      "keywords": ["buyer-001", "amazon"],
      "groupId": "group-sales-us"
    },
    "launchCode": "BUYER_001"
  }'`
}

function buildSampleCreateAndLaunchRequest(baseUrl: string, apiAuth: LaunchServerInfo['apiAuth']): string {
  return `curl -X POST ${baseUrl}/api/profiles \\
  -H "Content-Type: application/json" \\
${buildAuthHeaderLine(apiAuth)}  -d '{
    "profile": {
      "profileName": "buyer-002",
      "userDataDir": "buyers/buyer-002",
      "proxyConfig": "http://user:pass@127.0.0.1:8080",
      "launchArgs": ["--disable-sync"],
      "keywords": ["buyer-002"]
    },
    "autoLaunch": true,
    "start": {
      "launchArgs": ["--window-size=1280,800"],
      "startUrls": ["https://example.com/order"],
      "skipDefaultStartUrls": true
    }
  }'`
}

function buildSampleRequest(baseUrl: string, apiAuth: LaunchServerInfo['apiAuth']): string {
  return `curl -X POST ${baseUrl}/api/launch \\
  -H "Content-Type: application/json" \\
${buildAuthHeaderLine(apiAuth)}  -d '{
    "code": "A3F9K2",
    "launchArgs": ["--window-size=1280,800", "--lang=en-US"],
    "startUrls": ["https://example.com"],
    "skipDefaultStartUrls": true
  }'`
}

const sampleCreateResponse = `{
  "ok": true,
  "created": true,
  "launched": false,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001"
}`

const sampleCreateAndLaunchResponse = `{
  "ok": true,
  "created": true,
  "launched": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440001",
  "profileName": "buyer-002",
  "launchCode": "A3F9K2",
  "pid": 12345,
  "debugPort": 9222,
  "cdpUrl": "http://127.0.0.1:19876"
}`

const sampleResponse = `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "账号 A",
  "pid": 12345,
  "debugPort": 9222,
  "cdpUrl": "http://127.0.0.1:19876"
}`

function buildSampleLogsRequest(baseUrl: string, apiAuth: LaunchServerInfo['apiAuth']): string {
  if (!apiAuth.enabled) {
    return `curl ${baseUrl}/api/launch/logs?limit=20`
  }
  return `curl ${baseUrl}/api/launch/logs?limit=20 \\
  -H "${apiAuth.header}: <your-api-key>"`
}

function CopyCodeButton({ text }: { text: string }) {
  return (
    <Button
      size="sm"
      variant="secondary"
      onClick={() => {
        navigator.clipboard.writeText(text).then(() => toast.success('已复制'))
      }}
    >
      <Copy className="w-3.5 h-3.5" /> 复制
    </Button>
  )
}

function CodeBlock({ text }: { text: string }) {
  return (
    <pre className="text-xs leading-relaxed font-mono text-[var(--color-text-primary)] bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
      {text}
    </pre>
  )
}

type AutomationTabKey = 'guide' | 'profiles' | 'launch' | 'logs'

const AUTOMATION_TABS: { key: AutomationTabKey; label: string; description: string }[] = [
  { key: 'guide', label: '接入说明', description: '先理解整体调用方式和推荐流程。' },
  { key: 'profiles', label: '配置管理', description: '集中查看窗口创建、配置落库和返回结构。' },
  { key: 'launch', label: '启动调用', description: '集中查看参数化唤起和启动响应。' },
  { key: 'logs', label: '日志排障', description: '集中查看日志查询和后续排障入口。' },
]

export function AutomationPage() {
  const [launchBaseUrl, setLaunchBaseUrl] = useState(DEFAULT_LAUNCH_BASE_URL)
  const [launchServerReady, setLaunchServerReady] = useState(false)
  const [apiAuth, setApiAuth] = useState<LaunchServerInfo['apiAuth']>(DEFAULT_API_AUTH)
  const [activeTab, setActiveTab] = useState<AutomationTabKey>('guide')

  useEffect(() => {
    let disposed = false

    void fetchLaunchServerInfo()
      .then((info) => {
        if (disposed) return
        if (info.baseUrl) {
          setLaunchBaseUrl(info.baseUrl)
        }
        setLaunchServerReady(info.ready)
        setApiAuth(info.apiAuth)
      })
      .catch(() => {})

    return () => {
      disposed = true
    }
  }, [])

  const sampleCreateRequest = buildSampleCreateRequest(launchBaseUrl, apiAuth)
  const sampleCreateAndLaunchRequest = buildSampleCreateAndLaunchRequest(launchBaseUrl, apiAuth)
  const sampleRequest = buildSampleRequest(launchBaseUrl, apiAuth)
  const sampleLogsRequest = buildSampleLogsRequest(launchBaseUrl, apiAuth)
  const activeTabMeta = AUTOMATION_TABS.find(tab => tab.key === activeTab) || AUTOMATION_TABS[0]

  return (
    <div className="space-y-5 animate-fade-in">
      <Card>
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-[var(--color-accent-muted)] text-[var(--color-accent)] text-xs font-medium mb-3">
              <Bot className="w-3.5 h-3.5" /> 自动化接口（实验）
            </div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">外部脚本配置与唤起接口</h1>
            <p className="text-sm text-[var(--color-text-secondary)] mt-2">
              已支持通过本地 <code>HTTP + JSON</code> 协议管理窗口配置并唤起窗口，并通过同一个固定端口暴露 CDP 入口。只要能发 HTTP 请求，和调用语言无关；Playwright、Selenium、自研调度器都只是接入方。
            </p>
            <p className="text-xs text-[var(--color-text-muted)] mt-2">
              当前 Launch 地址：<code>{launchBaseUrl}</code>
              {!launchServerReady ? '（服务启动后会自动刷新）' : ''}
            </p>
            <p className="text-xs text-[var(--color-text-muted)] mt-1">
              {apiAuth.enabled
                ? <>当前 API 认证已启用，请为所有 <code>/api/*</code> 请求追加 <code>{apiAuth.header}: &lt;your-api-key&gt;</code>。</>
                : apiAuth.requested && !apiAuth.configured
                  ? <>当前配置要求启用 API 认证，但 <code>api_key</code> 为空，认证尚未生效。</>
                  : <>当前 API 认证未启用；如需开启，可在 <code>config.yaml</code> 的 <code>launch_server.auth</code> 下配置。</>}
            </p>
          </div>
        </div>
      </Card>

      <div className="space-y-3">
        <div className="overflow-x-auto">
          <div className="flex min-w-max border-b border-[var(--color-border)]">
            {AUTOMATION_TABS.map(tab => (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={[
                  'px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap',
                  activeTab === tab.key
                    ? 'border-b-2 border-[var(--color-primary)] text-[var(--color-primary)]'
                    : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]',
                ].join(' ')}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </div>

        <Card className="bg-[var(--color-bg-surface)]/70">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-[var(--color-accent-muted)] p-2 text-[var(--color-accent)]">
              <Bot className="w-4 h-4" />
            </div>
            <div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">{activeTabMeta.label}</p>
              <p className="text-sm text-[var(--color-text-secondary)] mt-1">{activeTabMeta.description}</p>
            </div>
          </div>
        </Card>
      </div>

      {activeTab === 'guide' && (
        <div className="space-y-5">
          <Card title="推荐接入顺序" subtitle="稳定性优先时，建议把创建、启动、接管拆开处理">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-sm">
              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-4">
                <p className="text-xs uppercase tracking-[0.14em] text-[var(--color-text-muted)]">Step 1</p>
                <p className="mt-2 font-medium text-[var(--color-text-primary)]">先创建配置</p>
                <p className="mt-1 text-[var(--color-text-secondary)]">先拿到 <code>profileId</code> 和 <code>launchCode</code>，把落库和启动拆开。</p>
              </div>
              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-4">
                <p className="text-xs uppercase tracking-[0.14em] text-[var(--color-text-muted)]">Step 2</p>
                <p className="mt-2 font-medium text-[var(--color-text-primary)]">再调用启动</p>
                <p className="mt-1 text-[var(--color-text-secondary)]">启动失败时更容易单独重试，也更容易记录调度结果。</p>
              </div>
              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-4">
                <p className="text-xs uppercase tracking-[0.14em] text-[var(--color-text-muted)]">Step 3</p>
                <p className="mt-2 font-medium text-[var(--color-text-primary)]">最后接 CDP</p>
                <p className="mt-1 text-[var(--color-text-secondary)]">统一使用响应里的 <code>cdpUrl</code>，不要自己拼内部调试端口。</p>
              </div>
            </div>
          </Card>

          <Card title="触发创建的方式" subtitle="推荐按用途选择 /api/profiles 的三种调用模式">
            <div className="text-sm text-[var(--color-text-secondary)] space-y-2">
              <p><code>仅创建配置</code>: 传 <code>profile</code>，不传 <code>autoLaunch</code>，接口只落库不启动浏览器。</p>
              <p><code>创建并立即启动</code>: 传 <code>profile</code> + <code>autoLaunch=true</code>，可再用 <code>start</code> 追加本次启动参数。</p>
              <p><code>先创建后单独唤起</code>: 先调用 <code>POST /api/profiles</code> 取得 <code>profileId</code> / <code>launchCode</code>，再调用 <code>POST /api/launch</code> 或 <code>GET /api/launch/{'{code}'}</code>。</p>
              <p>稳定性优先时，推荐默认走“先创建后单独唤起”，这样创建和启动失败可以分开处理、分开重试。</p>
            </div>
          </Card>
        </div>
      )}

      {activeTab === 'profiles' && (
        <div className="space-y-5">
          <Card
            title="仅创建窗口配置"
            subtitle="POST /api/profiles"
            actions={<CopyCodeButton text={sampleCreateRequest} />}
          >
            <CodeBlock text={sampleCreateRequest} />
            <div className="mt-3 text-sm text-[var(--color-text-secondary)] space-y-1">
              <p><code>profile</code>: 持久化的窗口配置，支持窗口名、代理、标签、关键字、分组、默认启动参数等字段。</p>
              <p><code>launchCode</code>: 可选的自定义启动码；如果不传，系统会自动生成。</p>
              <p><code>autoLaunch</code> + <code>start</code>: 可选，表示创建后立即启动，并附带一次性启动参数。</p>
              <p>同一资源还支持 <code>GET /api/profiles</code>、<code>GET/PUT/DELETE /api/profiles/{'{profileId}'}</code>，用于后续查询、更新、删除。</p>
            </div>
          </Card>

          <Card
            title="创建响应"
            subtitle="创建成功后返回 profileId + launchCode"
            actions={<CopyCodeButton text={sampleCreateResponse} />}
          >
            <CodeBlock text={sampleCreateResponse} />
          </Card>

          <Card
            title="创建并立即启动"
            subtitle="POST /api/profiles + autoLaunch=true"
            actions={<CopyCodeButton text={sampleCreateAndLaunchRequest} />}
          >
            <CodeBlock text={sampleCreateAndLaunchRequest} />
            <div className="mt-3 text-sm text-[var(--color-text-secondary)] space-y-1">
              <p><code>autoLaunch=true</code>: 当前请求在创建完成后会直接启动窗口。</p>
              <p><code>start</code>: 只作用于本次启动，不会写回窗口持久化配置。</p>
              <p>如果创建已经成功但自动启动失败，响应里仍会标出 <code>created=true</code>，便于脚本分支处理。</p>
            </div>
          </Card>

          <Card
            title="创建并启动响应"
            subtitle="返回 created + launched + cdpUrl"
            actions={<CopyCodeButton text={sampleCreateAndLaunchResponse} />}
          >
            <CodeBlock text={sampleCreateAndLaunchResponse} />
          </Card>
        </div>
      )}

      {activeTab === 'launch' && (
        <div className="space-y-5">
          <Card title="启动接口使用建议" subtitle="把选择目标、附加参数和页面打开策略放在一次请求里">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm text-[var(--color-text-secondary)]">
              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-4">
                <p className="font-medium text-[var(--color-text-primary)]">目标匹配</p>
                <p className="mt-1"><code>code</code> 用于精确唤起；<code>key</code> 适合关键字检索和批量调度。</p>
              </div>
              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-4">
                <p className="font-medium text-[var(--color-text-primary)]">接管方式</p>
                <p className="mt-1">外部统一使用固定 <code>cdpUrl</code> 连接，不直接依赖内部实际 <code>debugPort</code>。</p>
              </div>
            </div>
          </Card>

          <Card
            title="参数化唤起接口"
            subtitle="POST /api/launch"
            actions={<CopyCodeButton text={sampleRequest} />}
          >
            <CodeBlock text={sampleRequest} />
            <div className="mt-3 text-sm text-[var(--color-text-secondary)] space-y-1">
              <p><code>code</code> / <code>key</code>: 二选一即可；<code>code</code> 按 LaunchCode 精确匹配，<code>key</code> 按窗口关键字优先精确、未命中时再模糊匹配。</p>
              <p><code>matchMode</code>: 多命中时的行为控制，支持 <code>unique</code> / <code>first</code> / <code>all</code>；传 <code>key</code> 时默认 <code>first</code>。</p>
              <p><code>launchArgs</code>: 仅本次启动附加的 Chrome 启动参数。</p>
              <p><code>startUrls</code>: 启动后打开的页面列表。</p>
              <p><code>skipDefaultStartUrls</code>: 设为 <code>true</code> 时不追加系统默认起始页。</p>
            </div>
          </Card>

          <Card
            title="启动响应"
            subtitle="成功返回 pid + cdpUrl；外部统一使用固定端口接 CDP"
            actions={<CopyCodeButton text={sampleResponse} />}
          >
            <CodeBlock text={sampleResponse} />
          </Card>
        </div>
      )}

      {activeTab === 'logs' && (
        <div className="space-y-5">
          <Card
            title="调用记录"
            subtitle="GET /api/launch/logs?limit=20"
            actions={<CopyCodeButton text={sampleLogsRequest} />}
          >
            <CodeBlock text={sampleLogsRequest} />
            <p className="mt-3 text-sm text-[var(--color-text-secondary)]">
              可查询最近接口调用记录（默认 50 条，最大 200 条），用于排查自动化脚本调用问题。
            </p>
          </Card>

          <Card title="排障提示" subtitle="先看最近调用，再看窗口是否已经完成后台接管">
            <div className="text-sm text-[var(--color-text-secondary)] space-y-2">
              <p>如果返回里已经有 <code>pid</code>，但 <code>debugReady=false</code>，说明窗口已拉起，只是 CDP 还在后台附着。</p>
              <p>如果接口直接返回错误，优先查看最近日志和窗口最近错误，再决定是否重试。</p>
              <p>排查自动化脚本时，建议把请求参数、响应体和调用日志一起保存，方便复现。</p>
            </div>
          </Card>

          <Card>
            <div className="flex items-start gap-2 text-sm text-[var(--color-text-secondary)]">
              <Rocket className="w-4 h-4 mt-0.5 text-[var(--color-accent)]" />
              <p>
                当前这部分接口已经可用，后续会继续补充自动化任务编排、模板脚本、连接状态监控等增强能力。
              </p>
            </div>
          </Card>
        </div>
      )}
    </div>
  )
}
