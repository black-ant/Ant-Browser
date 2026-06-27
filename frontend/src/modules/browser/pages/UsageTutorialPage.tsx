import { useEffect, useState } from 'react'
import { BookOpen, Download, Globe, Keyboard, Layers, Monitor, Rocket } from 'lucide-react'
import { Button, Card } from '../../../shared/components'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import type { ReactNode } from 'react'
import { fetchLaunchServerInfo, type LaunchServerInfo } from '../api'

const DEFAULT_LAUNCH_BASE_URL = 'http://127.0.0.1:19876'
const DEFAULT_API_AUTH: LaunchServerInfo['apiAuth'] = {
  requested: false,
  configured: false,
  enabled: false,
  header: 'X-Ant-Api-Key',
}

function StepCard({
  icon,
  title,
  children,
}: {
  icon: ReactNode
  title: string
  children: ReactNode
}) {
  return (
    <Card>
      <div className="flex items-start gap-3">
        <div className="w-9 h-9 rounded-lg bg-[var(--color-accent-muted)] text-[var(--color-accent)] flex items-center justify-center shrink-0">
          {icon}
        </div>
        <div className="flex-1 min-w-0">
          <h2 className="text-base font-semibold text-[var(--color-text-primary)]">{title}</h2>
          <div className="mt-2 text-sm text-[var(--color-text-secondary)] leading-relaxed space-y-2">{children}</div>
        </div>
      </div>
    </Card>
  )
}

function LinkButton({ url, children }: { url: string; children: ReactNode }) {
  return (
    <Button
      size="sm"
      variant="secondary"
      onClick={() => {
        void BrowserOpenURL(url)
      }}
    >
      {children}
    </Button>
  )
}

function buildLaunchCodeCurlSample(baseUrl: string, apiAuth: LaunchServerInfo['apiAuth']): string {
  const authComment = apiAuth.enabled
    ? `# 如果已启用认证，请追加请求头：${apiAuth.header}: <your-api-key>\n`
    : ''
  const authHeaderLine = apiAuth.enabled ? `  -H "${apiAuth.header}: <your-api-key>" \\\n` : ''
  const getSuffix = apiAuth.enabled ? ` \\\n  -H "${apiAuth.header}: <your-api-key>"` : ''

  return `${authComment}# 按 Code 启动
curl ${baseUrl}/api/launch/A3F9K2${getSuffix}

# 带参数启动
curl -X POST ${baseUrl}/api/launch \\
  -H "Content-Type: application/json" \\
${authHeaderLine}  -d '{"code":"A3F9K2","launchArgs":["--window-size=1280,800"]}'`
}

export function UsageTutorialPage() {
  const [launchBaseUrl, setLaunchBaseUrl] = useState(DEFAULT_LAUNCH_BASE_URL)
  const [launchServerReady, setLaunchServerReady] = useState(false)
  const [apiAuth, setApiAuth] = useState<LaunchServerInfo['apiAuth']>(DEFAULT_API_AUTH)

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

  const launchCodeCurlSample = buildLaunchCodeCurlSample(launchBaseUrl, apiAuth)

  return (
    <div className="space-y-5 animate-fade-in">
      <Card>
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-[var(--color-accent-muted)] text-[var(--color-accent)] text-xs font-medium mb-3">
              <BookOpen className="w-3.5 h-3.5" /> 使用教程
            </div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">从 0 到可用：内核、代理池、窗口启动</h1>
            <p className="text-sm text-[var(--color-text-secondary)] mt-2">
              按照下面步骤，你可以完成内核下载、代理池配置、窗口创建与启动。
            </p>
          </div>
        </div>
      </Card>

      <StepCard icon={<Download className="w-4 h-4" />} title="1) 下载并准备浏览器内核">
        <p>推荐使用 fingerprint-chromium。你可以优先走应用内下载：</p>
        <pre className="text-xs font-mono bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
{`左侧菜单 -> 指纹浏览器 -> 内核管理 -> 下载内核`}
        </pre>
        <p>如果应用内下载失败，再使用 GitHub 手动下载 ZIP 包，下载后解压到项目目录的 <code>chrome/</code> 下。</p>
        <p>建议目录结构示例：</p>
        <pre className="text-xs font-mono bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
{`chrome/
  chrome142/
    chrome.exe
    ...`}
        </pre>
        <div className="flex items-center gap-2 flex-wrap">
          <LinkButton url="https://github.com/adryfish/fingerprint-chromium">项目主页</LinkButton>
          <LinkButton url="https://github.com/adryfish/fingerprint-chromium/releases">Releases 下载</LinkButton>
        </div>
      </StepCard>

      <StepCard icon={<Layers className="w-4 h-4" />} title="2) 在“内核管理”中确认可用内核">
        <p>进入左侧 <code>指纹浏览器 &gt; 内核管理</code>。</p>
        <p>确认已识别到你解压后的内核路径，并设置一个默认内核。</p>
        <p>如果未识别到，请检查是否存在 <code>chrome.exe</code>，以及路径是否填写正确。</p>
      </StepCard>

      <StepCard icon={<Globe className="w-4 h-4" />} title="3) 创建代理池（HTTP/SOCKS5/Vmess/Vless/Trojan）">
        <p>进入左侧 <code>指纹浏览器 &gt; 代理池配置</code>。</p>
        <p>你可以逐条添加代理，也可以通过 YAML 批量导入。</p>
        <p>保存后，窗口编辑页里即可选择这些代理节点。</p>
      </StepCard>

      <StepCard icon={<Monitor className="w-4 h-4" />} title="4) 创建窗口并启动">
        <p>进入 <code>指纹浏览器 &gt; 窗口列表</code>，点击“新建配置”。</p>
        <p>设置窗口名称、选择内核、选择代理（可选）、调整启动参数。</p>
        <p>保存后返回列表，点击“启动”即可运行浏览器窗口。</p>
      </StepCard>

      <StepCard icon={<Keyboard className="w-4 h-4" />} title="5) 使用快捷键快速启动">
        <p>在应用内按 <code>Ctrl + K</code>（Mac 为 <code>Cmd + K</code>）即可呼出“快速启动浏览器”弹窗。</p>
        <p>你可以直接输入窗口 Code 回车启动，也可以在下方列表中选择窗口后启动。</p>
        <p>弹窗内支持键盘操作：</p>
        <pre className="text-xs font-mono bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
{`Ctrl/Cmd + K  呼出/收起快速启动弹窗
Enter          优先按输入的 Code 启动
↑ / ↓          在窗口列表中切换选中项
Esc            关闭弹窗`}
        </pre>
      </StepCard>

      <StepCard icon={<Rocket className="w-4 h-4" />} title="6) 自动化启动（可选）">
        <p>每个窗口可配置专属 Code。你可以通过快捷键弹窗输入 Code 启动，或通过接口调用启动。</p>
        <p>
          当前 Launch 地址：<code>{launchBaseUrl}</code>
          {!launchServerReady ? '（服务启动后会自动刷新）' : ''}
        </p>
        <p>
          {apiAuth.enabled
            ? <>当前 API 认证已启用，请为所有 <code>/api/*</code> 请求追加 <code>{apiAuth.header}: &lt;your-api-key&gt;</code>。</>
            : apiAuth.requested && !apiAuth.configured
              ? <>当前配置要求启用 API 认证，但 <code>api_key</code> 为空，认证尚未生效。</>
              : <>当前 API 认证未启用；如需开启，可在 <code>config.yaml</code> 的 <code>launch_server.auth</code> 下配置。</>}
        </p>
        <pre className="text-xs font-mono bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
{launchCodeCurlSample}
        </pre>
      </StepCard>
    </div>
  )
}
