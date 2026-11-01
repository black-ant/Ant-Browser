import { Button, FormItem, Input, Modal, Select, Switch, Textarea } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import {
  CHAIN_QUICK_IMPORT_TEMPLATE,
  DIRECT_QUICK_IMPORT_TEMPLATE,
  DIRECT_PROXY_PROTOCOL_OPTIONS,
  type ChainImportForm,
  type DirectImportForm,
  type ProxyImportMode,
} from './helpers'

interface ProxyPoolImportModalProps {
  open: boolean
  groups: string[]
  importMode: ProxyImportMode
  importUrl: string
  importFetchProxyId: string
  importAutoRefresh: boolean
  importRefreshIntervalM: number
  importResolvedUrl: string
  importText: string
  importDnsServers: string
  importNamePrefix: string
  importGroupName: string
  chainImportText: string
  directImportText: string
  chainImportForm: ChainImportForm
  directImportForm: DirectImportForm
  fetchingImportUrl: boolean
  fetchProxyOptions: BrowserProxy[]
  canParseImport: boolean
  onClose: () => void
  onParse: () => void
  onFetchImportUrl: () => void
  onImportModeChange: (nextMode: ProxyImportMode) => void
  onImportUrlChange: (nextValue: string) => void
  onImportFetchProxyIdChange: (nextValue: string) => void
  onImportAutoRefreshChange: (nextValue: boolean) => void
  onImportRefreshIntervalMChange: (nextValue: number) => void
  onImportTextChange: (nextValue: string) => void
  onImportDnsServersChange: (nextValue: string) => void
  onImportNamePrefixChange: (nextValue: string) => void
  onImportGroupNameChange: (nextValue: string) => void
  onChainImportTextChange: (nextValue: string) => void
  onDirectImportTextChange: (nextValue: string) => void
  onApplyChainJSON: () => void
  onApplyDirectText: () => void
  onChainImportFormChange: (patch: Partial<ChainImportForm>) => void
  onChainImportHopChange: (hop: 'first' | 'second', field: keyof ChainImportForm['first'], value: string) => void
  onFillChainTemplate: () => void
  onCopyChainTemplate: () => void
  onFillDirectTemplate: () => void
  onCopyDirectTemplate: () => void
  onDirectImportFormChange: (patch: Partial<DirectImportForm>) => void
}

const importModeTabs: Array<{ value: ProxyImportMode; label: string; hint: string }> = [
  { value: 'clash', label: 'Clash 订阅 / YAML', hint: '订阅或 Clash 节点列表' },
  { value: 'direct', label: 'HTTP / SOCKS5', hint: '单层代理或批量文本' },
  { value: 'chain', label: '链式代理', hint: '本机出口 + 落地代理' },
]

export function ProxyPoolImportModal({
  open,
  groups,
  importMode,
  importUrl,
  importFetchProxyId,
  importAutoRefresh,
  importRefreshIntervalM,
  importResolvedUrl,
  importText,
  importDnsServers,
  importNamePrefix,
  importGroupName,
  chainImportText,
  directImportText,
  chainImportForm,
  directImportForm,
  fetchingImportUrl,
  fetchProxyOptions,
  canParseImport,
  onClose,
  onParse,
  onFetchImportUrl,
  onImportModeChange,
  onImportUrlChange,
  onImportFetchProxyIdChange,
  onImportAutoRefreshChange,
  onImportRefreshIntervalMChange,
  onImportTextChange,
  onImportDnsServersChange,
  onImportNamePrefixChange,
  onImportGroupNameChange,
  onChainImportTextChange,
  onDirectImportTextChange,
  onApplyChainJSON,
  onApplyDirectText,
  onChainImportFormChange,
  onChainImportHopChange,
  onFillChainTemplate,
  onCopyChainTemplate,
  onFillDirectTemplate,
  onCopyDirectTemplate,
  onDirectImportFormChange,
}: ProxyPoolImportModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="导入代理配置"
      width="720px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={fetchingImportUrl}>
            取消
          </Button>
          <Button onClick={onParse} disabled={fetchingImportUrl || !canParseImport}>
            解析
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid grid-cols-3 gap-2 rounded-2xl bg-[var(--color-bg-secondary)] p-1">
          {importModeTabs.map(tab => {
            const active = importMode === tab.value
            return (
              <button
                key={tab.value}
                type="button"
                onClick={() => onImportModeChange(tab.value)}
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

        {importMode === 'clash' && (
          <>
            <FormItem label="订阅 URL（可选）">
              <div className="grid grid-cols-[minmax(0,1fr)_150px_auto] gap-2">
                <Input
                  value={importUrl}
                  onChange={(event) => onImportUrlChange(event.target.value)}
                  placeholder="订阅 URL"
                  className="flex-1"
                />
                <Select
                  value={importFetchProxyId}
                  onChange={(event) => onImportFetchProxyIdChange(event.target.value)}
                  disabled={fetchingImportUrl}
                  options={[
                    { value: '', label: '直连拉取' },
                    ...fetchProxyOptions.map(proxy => {
                      const latency = proxy.lastLatencyMs
                      return {
                        value: proxy.proxyId,
                        label: `${proxy.proxyName || proxy.proxyId}${typeof latency === 'number' && latency > 0 ? ` · ${latency}ms` : ''}`,
                      }
                    }),
                  ]}
                />
                <Button
                  variant="secondary"
                  onClick={onFetchImportUrl}
                  loading={fetchingImportUrl}
                  disabled={!importUrl.trim()}
                >
                  从 URL 获取
                </Button>
              </div>
              {importResolvedUrl.trim() && (
                <p className="text-xs text-[var(--color-success)] mt-1 break-all">
                  已绑定订阅：{importResolvedUrl}
                </p>
              )}
            </FormItem>
            <Textarea
              value={importText}
              onChange={(event) => onImportTextChange(event.target.value)}
              rows={12}
              placeholder={`proxies:\n  - name: vless-v6\n    type: vless\n    server: example.com\n    port: 443\n    uuid: your-uuid\n    ...`}
            />
          </>
        )}
        {importMode === 'direct' && (
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <FormItem label="代理协议" required>
                <Select
                  options={[...DIRECT_PROXY_PROTOCOL_OPTIONS]}
                  value={directImportForm.protocol}
                  onChange={(event) =>
                    onDirectImportFormChange({ protocol: event.target.value as DirectImportForm['protocol'] })
                  }
                />
              </FormItem>
              <FormItem label="代理名称（可选）">
                <Input
                  value={directImportForm.proxyName}
                  onChange={(event) => onDirectImportFormChange({ proxyName: event.target.value })}
                  placeholder="节点名称"
                />
              </FormItem>
              <FormItem label="代理地址" required>
                <Input
                  value={directImportForm.server}
                  onChange={(event) => onDirectImportFormChange({ server: event.target.value })}
                  placeholder="例如：127.0.0.1 或 hk.example.com"
                />
              </FormItem>
              <FormItem label="代理端口" required>
                <Input
                  type="number"
                  min={1}
                  max={65535}
                  value={directImportForm.port}
                  onChange={(event) => onDirectImportFormChange({ port: event.target.value })}
                  placeholder="例如：1080"
                />
              </FormItem>
              <FormItem label="账号（可选）">
                <Input
                  value={directImportForm.username}
                  onChange={(event) => onDirectImportFormChange({ username: event.target.value })}
                  placeholder="留空则不使用认证"
                />
              </FormItem>
              <FormItem label="密码（可选）">
                <Input
                  type="password"
                  value={directImportForm.password}
                  onChange={(event) => onDirectImportFormChange({ password: event.target.value })}
                  placeholder="留空则不使用密码"
                />
              </FormItem>
            </div>
            <FormItem label="文本辅助（可选）" hint="支持单个 JSON、JSON 数组，或多行 http:// / https:// / socks5://，每行一个">
              <Textarea
                value={directImportText}
                onChange={(event) => onDirectImportTextChange(event.target.value)}
                rows={8}
                placeholder={DIRECT_QUICK_IMPORT_TEMPLATE}
              />
              <div className="mt-2 flex gap-2">
                <Button size="sm" variant="secondary" onClick={onFillDirectTemplate}>
                  填入模板
                </Button>
                <Button size="sm" variant="secondary" onClick={onCopyDirectTemplate}>
                  复制模板
                </Button>
                <Button size="sm" variant="secondary" onClick={onApplyDirectText} disabled={!directImportText.trim()}>
                  应用文本
                </Button>
              </div>
              <p className="mt-2 text-xs text-[var(--color-text-muted)]">
                留空则按上方表单导入；有内容则点击“解析”按文本直接导入，可批量。
              </p>
            </FormItem>
          </div>
        )}
        {importMode === 'chain' && (
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <FormItem label="代理名称（可选）">
                <Input
                  value={chainImportForm.proxyName}
                  onChange={(event) => onChainImportFormChange({ proxyName: event.target.value })}
                  placeholder="例如：双层英国链路"
                />
              </FormItem>
              <FormItem label="本地监听端口（可选）">
                <Input
                  type="number"
                  min={1}
                  max={65535}
                  value={chainImportForm.localPort}
                  onChange={(event) => onChainImportFormChange({ localPort: event.target.value })}
                  placeholder="留空自动分配"
                />
              </FormItem>
            </div>
            <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
              <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第一层代理</h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormItem label="协议">
                  <Select
                    value={chainImportForm.first.protocol}
                    onChange={(event) => onChainImportHopChange('first', 'protocol', event.target.value)}
                    options={[
                      { value: 'http', label: 'HTTP' },
                      { value: 'socks5', label: 'SOCKS5' },
                    ]}
                  />
                </FormItem>
                <FormItem label="代理地址" required>
                  <Input
                    value={chainImportForm.first.server}
                    onChange={(event) => onChainImportHopChange('first', 'server', event.target.value)}
                    placeholder="代理地址"
                  />
                </FormItem>
                <FormItem label="代理端口" required>
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={chainImportForm.first.port}
                    onChange={(event) => onChainImportHopChange('first', 'port', event.target.value)}
                    placeholder="端口"
                  />
                </FormItem>
                <FormItem label="账号（可选）">
                  <Input
                    value={chainImportForm.first.username}
                    onChange={(event) => onChainImportHopChange('first', 'username', event.target.value)}
                  />
                </FormItem>
                <FormItem label="密码（可选）">
                  <Input
                    type="password"
                    value={chainImportForm.first.password}
                    onChange={(event) => onChainImportHopChange('first', 'password', event.target.value)}
                  />
                </FormItem>
              </div>
            </div>
            <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
              <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第二层代理</h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormItem label="协议">
                  <Select
                    value={chainImportForm.second.protocol}
                    onChange={(event) => onChainImportHopChange('second', 'protocol', event.target.value)}
                    options={[
                      { value: 'http', label: 'HTTP' },
                      { value: 'socks5', label: 'SOCKS5' },
                    ]}
                  />
                </FormItem>
                <FormItem label="代理地址" required>
                  <Input
                    value={chainImportForm.second.server}
                    onChange={(event) => onChainImportHopChange('second', 'server', event.target.value)}
                    placeholder="代理地址"
                  />
                </FormItem>
                <FormItem label="代理端口" required>
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={chainImportForm.second.port}
                    onChange={(event) => onChainImportHopChange('second', 'port', event.target.value)}
                    placeholder="端口"
                  />
                </FormItem>
                <FormItem label="账号（可选）">
                  <Input
                    value={chainImportForm.second.username}
                    onChange={(event) => onChainImportHopChange('second', 'username', event.target.value)}
                  />
                </FormItem>
                <FormItem label="密码（可选）">
                  <Input
                    type="password"
                    value={chainImportForm.second.password}
                    onChange={(event) => onChainImportHopChange('second', 'password', event.target.value)}
                  />
                </FormItem>
              </div>
            </div>
            <FormItem label="JSON 辅助（可选）">
              <Textarea
                value={chainImportText}
                onChange={(event) => onChainImportTextChange(event.target.value)}
                rows={10}
                placeholder={CHAIN_QUICK_IMPORT_TEMPLATE}
              />
              <div className="mt-2 flex gap-2">
                <Button size="sm" variant="secondary" onClick={onFillChainTemplate}>
                  填入模板
                </Button>
                <Button size="sm" variant="secondary" onClick={onCopyChainTemplate}>
                  复制模板
                </Button>
                <Button size="sm" variant="secondary" onClick={onApplyChainJSON} disabled={!chainImportText.trim()}>
                  应用 JSON
                </Button>
              </div>
            </FormItem>
          </div>
        )}
        <FormItem label="分组名称（可选）">
          <Input
            value={importGroupName}
            onChange={(event) => onImportGroupNameChange(event.target.value)}
            placeholder="分组名称"
            list="proxy-groups-datalist"
          />
          {groups.length > 0 && (
            <datalist id="proxy-groups-datalist">
              {groups.map((group) => (
                <option key={group} value={group} />
              ))}
            </datalist>
          )}
          <p className="text-xs text-[var(--color-text-muted)] mt-1">
            填写后本次导入的代理将归入该分组，可按分组筛选
          </p>
        </FormItem>
        {importMode === 'clash' && (
          <FormItem label="名称前缀（可选）">
            <Input
              value={importNamePrefix}
              onChange={(event) => onImportNamePrefixChange(event.target.value)}
              placeholder="例如：HK、US、机场A"
            />
            <p className="text-xs text-[var(--color-text-muted)] mt-1">
              填写后代理名称将变为 <code className="px-1 bg-[var(--color-bg-secondary)] rounded">前缀-原名称</code>，留空则保持原名
            </p>
          </FormItem>
        )}
        {importMode === 'clash' && (
          <FormItem label="定时刷新">
            <div className="flex items-center gap-2">
              <Switch
                checked={importAutoRefresh}
                onChange={onImportAutoRefreshChange}
              />
              {importAutoRefresh && (
                <Select
                  value={String(importRefreshIntervalM)}
                  onChange={(event) => onImportRefreshIntervalMChange(Number(event.target.value) || 1440)}
                  className="w-40"
                  options={[
                    { value: '60', label: '每 1 小时' },
                    { value: '360', label: '每 6 小时' },
                    { value: '720', label: '每 12 小时' },
                    { value: '1440', label: '每 24 小时' },
                  ]}
                />
              )}
            </div>
            <p className="text-xs text-[var(--color-text-muted)] mt-1">
              {importAutoRefresh
                ? '开启后按所选间隔在后台拉取同步；应用没开也不会错过，下次启动时自动补拉。节点按名称/国家自动匹配更新'
                : '关闭：仅手动点“刷新订阅”时拉取'}
            </p>
          </FormItem>
        )}
        {importMode === 'clash' && (
          <FormItem label="批量 DNS 配置（可选）">
            <Textarea
              value={importDnsServers}
              onChange={(event) => onImportDnsServersChange(event.target.value)}
              rows={5}
              placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`}
            />
            <p className="text-xs text-[var(--color-text-muted)] mt-1">
              留空则不配置 DNS，填写后将应用到本次导入的所有代理
            </p>
          </FormItem>
        )}
      </div>
    </Modal>
  )
}
