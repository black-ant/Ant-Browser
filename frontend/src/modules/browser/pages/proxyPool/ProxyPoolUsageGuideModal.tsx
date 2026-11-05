import { useState } from 'react'

import { Button, Modal } from '../../../../shared/components'

type UsageGuideTab = 'clash' | 'direct' | 'ipfly'
type ChannelDetailTab = 'ipfly'

interface ProxyPoolUsageGuideModalProps {
  open: boolean
  onClose: () => void
}

const usageTabs: Array<{ value: UsageGuideTab; label: string; hint: string }> = [
  { value: 'clash', label: 'Clash 订阅 / YAML', hint: '订阅拉取或粘贴 YAML' },
  { value: 'direct', label: 'HTTP / SOCKS5', hint: '单层代理导入' },
  { value: 'ipfly', label: '特定渠道', hint: '渠道落地说明' },
]

const channelDetailTabs: Array<{ value: ChannelDetailTab; label: string; hint: string }> = [
  { value: 'ipfly', label: 'IPFLY 落地', hint: 'VPN 出口 + IPFLY 链式代理' },
]

const clashYAML = `proxies:
  - name: hk-vless
    type: vless
    server: example.com
    port: 443
    uuid: your-uuid
    network: ws
    tls: true
    servername: example.com
    ws-opts:
      path: /proxy`

const directJSON = `{
  "name": "hk-socks5",
  "group": "海外出口",
  "protocol": "socks5",
  "server": "ep.example.com",
  "port": "6616",
  "username": "账号",
  "password": "密码"
}`

const directLines = `http://账号:密码@gw.example.com:8080
https://账号:密码@gw.example.com:8443
socks5://账号:密码@ep.example.com:6616`

const ipflyJSON = `{
  "name": "IPFLY-落地",
  "group": "",
  "localPort": "",
  "first": {
    "protocol": "socks5",
    "server": "127.0.0.1",
    "port": "7890",
    "username": "",
    "password": ""
  },
  "second": {
    "protocol": "socks5",
    "server": "ep.ipflygates.com",
    "port": "6616",
    "username": "IPFLY账号",
    "password": "IPFLY密码"
  }
}`

export function ProxyPoolUsageGuideModal({ open, onClose }: ProxyPoolUsageGuideModalProps) {
  const [activeTab, setActiveTab] = useState<UsageGuideTab>('ipfly')

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="代理池使用说明"
      width="920px"
      footer={<Button onClick={onClose}>知道了</Button>}
    >
      <div className="space-y-4">
        <div className="grid grid-cols-3 gap-2 rounded-2xl bg-[var(--color-bg-secondary)] p-1">
          {usageTabs.map(tab => {
            const active = activeTab === tab.value
            return (
              <button
                key={tab.value}
                type="button"
                onClick={() => setActiveTab(tab.value)}
                className={active
                  ? 'rounded-xl bg-[var(--color-text-primary)] px-4 py-3 text-left text-white shadow-sm'
                  : 'rounded-xl px-4 py-3 text-left text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)] hover:text-[var(--color-text-primary)]'
                }
              >
                <span className="block text-sm font-semibold">{tab.label}</span>
                <span className={active ? 'mt-1 block text-xs text-white/75' : 'mt-1 block text-xs text-[var(--color-text-muted)]'}>{tab.hint}</span>
              </button>
            )
          })}
        </div>

        {activeTab === 'clash' && <ClashGuide />}
        {activeTab === 'direct' && <DirectGuide />}
        {activeTab === 'ipfly' && <IPFLYGuide />}
      </div>
    </Modal>
  )
}

function ClashGuide() {
  return (
    <div className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <GuideCard title="1. 获取内容" text="填订阅 URL 后点击从 URL 获取；无法直连时选择已有代理拉取。" />
        <GuideCard title="2. 检查 YAML" text="文本里必须有 proxies 列表，每个节点至少包含 name、type、server、port。" />
        <GuideCard title="3. 解析写入" text="解析后先看节点数量和名称，再确认导入到代理池。" />
      </div>
      <div className="grid gap-3 lg:grid-cols-[0.95fr_1.05fr]">
        <InfoPanel
          title="支持内容"
          items={[
            '完整 Clash 订阅地址，可直连拉取，也可借助代理拉取。',
            '完整 Clash YAML，或只包含 proxies 的 YAML 片段。',
            '常见节点类型按当前连接栈解析；xray 栈和 mihomo 栈不自动混用。',
          ]}
        />
        <CodeBlock value={clashYAML} />
      </div>
      <InfoPanel
        title="导入前检查"
        items={[
          '订阅拉取失败时，先换“拉取代理”，不要把订阅 URL 当 YAML 粘贴。',
          '节点重名会影响后续识别，导入前给订阅来源或节点名称做区分。',
          '解析为空时，直接检查 YAML 缩进、proxies 拼写、port 是否是数字。',
        ]}
      />
    </div>
  )
}

function DirectGuide() {
  return (
    <div className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <GuideCard title="1. 单个导入" text="用表单填写协议、地址、端口、账号密码；留空文本辅助。" />
        <GuideCard title="2. 批量导入" text="在文本辅助填 JSON、JSON 数组或多行代理 URL，再点击应用文本或解析。" />
        <GuideCard title="3. 写入代理池" text="解析后确认名称、分组和数量，再保存到代理池。" />
      </div>
      <div className="grid gap-3 lg:grid-cols-[0.95fr_1.05fr]">
        <InfoPanel
          title="字段规则"
          items={[
            '表单地址只填域名或 IP，不带 http://、https://、socks5://。',
            '文本 URL 必须带协议头，格式是 协议://账号:密码@地址:端口。',
            '支持 http、https、socks5；密码有值时账号也必须填写。',
          ]}
        />
        <CodeBlock value={directJSON} />
      </div>
      <div className="grid gap-3 lg:grid-cols-[0.95fr_1.05fr]">
        <InfoPanel
          title="批量文本"
          items={[
            '每行一个代理 URL，空行会忽略。',
            '以 # 或 // 开头的行会忽略，可用来临时注释。',
            '需要分组时用 JSON 的 group 字段；纯 URL 行不带分组。',
          ]}
        />
        <CodeBlock value={directLines} />
      </div>
    </div>
  )
}

function IPFLYGuide() {
  const [activeDetailTab, setActiveDetailTab] = useState<ChannelDetailTab>('ipfly')

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2 rounded-xl bg-[var(--color-bg-secondary)] p-2">
        {channelDetailTabs.map(tab => {
          const active = activeDetailTab === tab.value
          return (
            <button
              key={tab.value}
              type="button"
              onClick={() => setActiveDetailTab(tab.value)}
              className={active
                ? 'rounded-full bg-[var(--color-text-primary)] px-3 py-1.5 text-left text-white shadow-sm'
                : 'rounded-full border border-[var(--color-border-muted)] bg-[var(--color-bg-primary)] px-3 py-1.5 text-left text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
              }
            >
              <span className="text-xs font-semibold">{tab.label}</span>
            </button>
          )
        })}
      </div>

      {activeDetailTab === 'ipfly' && (
        <>
      <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-amber-900">
        <div className="text-sm font-semibold">IPFLY 在大陆网络下使用链式代理落地</div>
        <div className="mt-2 text-xs leading-5">
          第一层填本机 VPN / Mihomo 出口，第二层填 IPFLY 网关。流量路径：浏览器 → xray → 本机 VPN → IPFLY → 目标网站。
        </div>
      </div>
      <div className="grid gap-3 lg:grid-cols-4">
        <GuideCard title="1. 开本机 VPN" text="确认 127.0.0.1:7890 可出海外。" />
        <GuideCard title="2. 导入链式代理" text="第一层填本机出口，第二层填 IPFLY。" />
        <GuideCard title="3. 调检测超时" text="测速目标 timeoutMs 建议 10000。" />
        <GuideCard title="4. 绑定浏览器" text="IP 健康通过后再分配给实例。" />
      </div>
      <div className="grid gap-3 lg:grid-cols-[0.9fr_1.1fr]">
        <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-primary)] p-4 text-xs leading-6 text-[var(--color-text-secondary)]">
          <div className="font-semibold text-[var(--color-text-primary)]">字段填写</div>
          <div className="mt-2 font-mono">第一层：socks5://127.0.0.1:7890</div>
          <div className="font-mono">第二层：socks5://账号:密码@ep.ipflygates.com:6616</div>
          <div className="mt-2">账号密码用 IPFLY 后台给出的值替换。</div>
        </div>
        <CodeBlock value={ipflyJSON} />
      </div>
        </>
      )}
    </div>
  )
}

function GuideCard({ title, text }: { title: string; text: string }) {
  return (
    <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-primary)] p-4">
      <div className="text-sm font-semibold text-[var(--color-text-primary)]">{title}</div>
      <div className="mt-2 text-xs leading-5 text-[var(--color-text-secondary)]">{text}</div>
    </div>
  )
}

function InfoPanel({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-primary)] p-4">
      <div className="text-sm font-semibold text-[var(--color-text-primary)]">{title}</div>
      <ul className="mt-2 space-y-1.5 text-xs leading-5 text-[var(--color-text-secondary)]">
        {items.map(item => (
          <li key={item} className="flex gap-2">
            <span className="mt-2 h-1 w-1 shrink-0 rounded-full bg-[var(--color-text-muted)]" />
            <span>{item}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function CodeBlock({ value }: { value: string }) {
  return (
    <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-[var(--color-bg-muted)] p-4 text-xs leading-5 text-[var(--color-text-secondary)]">
      {value}
    </pre>
  )
}
