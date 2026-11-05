import { Button, FormItem, Input, Modal, Select, Table, Textarea } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserProxy } from '../types'
import { DIRECT_QUICK_IMPORT_TEMPLATE } from '../pages/proxyPool/helpers'
import {
  DIRECT_PROXY_PROTOCOL_OPTIONS,
  type ChainHopForm,
  type ChainImportForm,
  type DirectImportForm,
  type ProxyDisplayInfo,
  type ProxyImportMode,
} from './ProxyImportModal.types'

interface ProxyImportModalViewProps {
  open: boolean
  onClose: () => void
  fetchingImportUrl: boolean
  canParseImport: boolean
  importMode: ProxyImportMode
  importUrl: string
  importFetchProxyId: string
  importResolvedUrl: string
  importText: string
  importDnsServers: string
  importNamePrefix: string
  importGroupName: string
  directImportText: string
  directImportForm: DirectImportForm
  chainImportForm: ChainImportForm
  groups: string[]
  fetchProxyOptions: BrowserProxy[]
  previewModalOpen: boolean
  previewList: ProxyDisplayInfo[]
  importing: boolean
  previewColumns: TableColumn<ProxyDisplayInfo>[]
  onParseImport: () => void
  onImportModeChange: (mode: ProxyImportMode) => void
  onImportUrlChange: (value: string) => void
  onImportFetchProxyIdChange: (value: string) => void
  onImportResolvedUrlChange: (value: string) => void
  onFetchImportURL: () => Promise<void>
  onImportTextChange: (value: string) => void
  onImportDnsServersChange: (value: string) => void
  onImportNamePrefixChange: (value: string) => void
  onImportGroupNameChange: (value: string) => void
  onDirectImportTextChange: (value: string) => void
  onDirectImportFormChange: import("react").Dispatch<import("react").SetStateAction<DirectImportForm>>
  onChainImportFormChange: import("react").Dispatch<import("react").SetStateAction<ChainImportForm>>
  onUpdateChainHop: (hop: 'first' | 'second', field: keyof ChainHopForm, value: string) => void
  onFillDirectTemplate: () => void
  onCopyDirectTemplate: () => Promise<void>
  onApplyDirectText: () => void
  onPreviewModalOpenChange: (open: boolean) => void
  onConfirmImport: () => Promise<void>
}

export function ProxyImportModalView({
  open,
  onClose,
  fetchingImportUrl,
  canParseImport,
  importMode,
  importUrl,
  importFetchProxyId,
  importResolvedUrl,
  importText,
  importDnsServers,
  importNamePrefix,
  importGroupName,
  directImportText,
  directImportForm,
  chainImportForm,
  groups,
  fetchProxyOptions,
  previewModalOpen,
  previewList,
  importing,
  previewColumns,
  onParseImport,
  onImportModeChange,
  onImportUrlChange,
  onImportFetchProxyIdChange,
  onImportResolvedUrlChange,
  onFetchImportURL,
  onImportTextChange,
  onImportDnsServersChange,
  onImportNamePrefixChange,
  onImportGroupNameChange,
  onDirectImportTextChange,
  onDirectImportFormChange,
  onChainImportFormChange,
  onUpdateChainHop,
  onFillDirectTemplate,
  onCopyDirectTemplate,
  onApplyDirectText,
  onPreviewModalOpenChange,
  onConfirmImport,
}: ProxyImportModalViewProps) {
  return (
    <>
      <Modal
        open={open}
        onClose={onClose}
        title="导入代理配置"
        width="600px"
        footer={
          <>
            <Button variant="secondary" onClick={onClose} disabled={fetchingImportUrl}>取消</Button>
            <Button onClick={onParseImport} disabled={fetchingImportUrl || !canParseImport}>解析</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="grid grid-cols-3 gap-2">
            <Button
              variant={importMode === 'clash' ? undefined : 'secondary'}
              onClick={() => onImportModeChange('clash')}
            >
              Clash 订阅 / YAML
            </Button>
            <Button
              variant={importMode === 'direct' ? undefined : 'secondary'}
              onClick={() => onImportModeChange('direct')}
            >
              HTTP / SOCKS5
            </Button>
            <Button
              variant={importMode === 'chain' ? undefined : 'secondary'}
              onClick={() => onImportModeChange('chain')}
            >
              链式代理
            </Button>
          </div>
          {importMode === 'clash' && (
            <>
              <FormItem label="订阅 URL（可选）">
                <div className="grid grid-cols-[minmax(0,1fr)_150px_auto] gap-2">
                  <Input
                    value={importUrl}
                    onChange={e => {
                      const next = e.target.value
                      onImportUrlChange(next)
                      if (importResolvedUrl.trim() && next.trim() !== importResolvedUrl.trim()) {
                        onImportResolvedUrlChange('')
                      }
                    }}
                    placeholder="订阅 URL"
                    className="flex-1"
                  />
                  <Select
                    value={importFetchProxyId}
                    onChange={e => onImportFetchProxyIdChange(e.target.value)}
                    disabled={fetchingImportUrl}
                    options={[
                      { value: '', label: '直连拉取' },
                      ...fetchProxyOptions.map(proxy => ({
                        value: proxy.proxyId,
                        label: proxy.proxyName || proxy.proxyId,
                      })),
                    ]}
                  />
                  <Button
                    variant="secondary"
                    onClick={onFetchImportURL}
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
                onChange={e => onImportTextChange(e.target.value)}
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
                    onChange={e => onDirectImportFormChange(prev => ({ ...prev, protocol: e.target.value as DirectImportForm['protocol'] }))}
                  />
                </FormItem>
                <FormItem label="代理名称（可选）">
                  <Input
                    value={directImportForm.proxyName}
                    onChange={e => onDirectImportFormChange(prev => ({ ...prev, proxyName: e.target.value }))}
                    placeholder="节点名称"
                  />
                </FormItem>
                <FormItem label="代理地址" required>
                  <Input
                    value={directImportForm.server}
                    onChange={e => onDirectImportFormChange(prev => ({ ...prev, server: e.target.value }))}
                    placeholder="例如：127.0.0.1 或 hk.example.com"
                  />
                </FormItem>
                <FormItem label="代理端口" required>
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={directImportForm.port}
                    onChange={e => onDirectImportFormChange(prev => ({ ...prev, port: e.target.value }))}
                    placeholder="例如：1080"
                  />
                </FormItem>
                <FormItem label="账号（可选）">
                  <Input
                    value={directImportForm.username}
                    onChange={e => onDirectImportFormChange(prev => ({ ...prev, username: e.target.value }))}
                    placeholder="留空则不使用认证"
                  />
                </FormItem>
                <FormItem label="密码（可选）">
                  <Input
                    type="password"
                    value={directImportForm.password}
                    onChange={e => onDirectImportFormChange(prev => ({ ...prev, password: e.target.value }))}
                    placeholder="留空则不使用密码"
                  />
                </FormItem>
              </div>
              <FormItem label="文本辅助（可选）" hint="支持单个 JSON、JSON 数组，或多行 http:// / https:// / socks5://，每行一个">
                <Textarea
                  value={directImportText}
                  onChange={e => onDirectImportTextChange(e.target.value)}
                  rows={8}
                  placeholder={DIRECT_QUICK_IMPORT_TEMPLATE}
                />
                <div className="mt-2 flex gap-2">
                  <Button size="sm" variant="secondary" onClick={onFillDirectTemplate}>
                    填入模板
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => void onCopyDirectTemplate()}>
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
                    onChange={e => onChainImportFormChange(prev => ({ ...prev, proxyName: e.target.value }))}
                    placeholder="链路名称"
                  />
                </FormItem>
                <FormItem label="本地监听端口（可选）">
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={chainImportForm.localPort}
                    onChange={e => onChainImportFormChange(prev => ({ ...prev, localPort: e.target.value }))}
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
                      onChange={e => onUpdateChainHop('first', 'protocol', e.target.value)}
                      options={[
                        { value: 'http', label: 'HTTP' },
                        { value: 'socks5', label: 'SOCKS5' },
                      ]}
                    />
                  </FormItem>
                  <FormItem label="代理地址" required>
                    <Input
                      value={chainImportForm.first.server}
                      onChange={e => onUpdateChainHop('first', 'server', e.target.value)}
                      placeholder="例如：s1.example.com"
                    />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input
                      type="number"
                      min={1}
                      max={65535}
                      value={chainImportForm.first.port}
                      onChange={e => onUpdateChainHop('first', 'port', e.target.value)}
                      placeholder="例如：1080"
                    />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input
                      value={chainImportForm.first.username}
                      onChange={e => onUpdateChainHop('first', 'username', e.target.value)}
                      placeholder="留空则不使用认证"
                    />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input
                      type="password"
                      value={chainImportForm.first.password}
                      onChange={e => onUpdateChainHop('first', 'password', e.target.value)}
                      placeholder="留空则不使用密码"
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
                      onChange={e => onUpdateChainHop('second', 'protocol', e.target.value)}
                      options={[
                        { value: 'http', label: 'HTTP' },
                        { value: 'socks5', label: 'SOCKS5' },
                      ]}
                    />
                  </FormItem>
                  <FormItem label="代理地址" required>
                    <Input
                      value={chainImportForm.second.server}
                      onChange={e => onUpdateChainHop('second', 'server', e.target.value)}
                      placeholder="例如：s2.example.com"
                    />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input
                      type="number"
                      min={1}
                      max={65535}
                      value={chainImportForm.second.port}
                      onChange={e => onUpdateChainHop('second', 'port', e.target.value)}
                      placeholder="例如：1081"
                    />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input
                      value={chainImportForm.second.username}
                      onChange={e => onUpdateChainHop('second', 'username', e.target.value)}
                      placeholder="留空则不使用认证"
                    />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input
                      type="password"
                      value={chainImportForm.second.password}
                      onChange={e => onUpdateChainHop('second', 'password', e.target.value)}
                      placeholder="留空则不使用密码"
                    />
                  </FormItem>
                </div>
              </div>
            </div>
          )}

          <FormItem label="分组名称（可选）">
            <Input
              value={importGroupName}
              onChange={e => onImportGroupNameChange(e.target.value)}
              placeholder="分组名称"
              list="proxy-groups-datalist"
            />
            {groups.length > 0 && (
              <datalist id="proxy-groups-datalist">
                {groups.map(g => <option key={g} value={g} />)}
              </datalist>
            )}
            <p className="text-xs text-[var(--color-text-muted)] mt-1">填写后本次导入的代理将归入该分组，可按分组筛选</p>
          </FormItem>
          {importMode === 'clash' && (
            <FormItem label="名称前缀（可选）">
              <Input
                value={importNamePrefix}
                onChange={e => onImportNamePrefixChange(e.target.value)}
                placeholder="例如：HK、US、机场A"
              />
              <p className="text-xs text-[var(--color-text-muted)] mt-1">
                填写后代理名称将变为 <code className="px-1 bg-[var(--color-bg-secondary)] rounded">前缀-原名称</code>，留空则保持原名
              </p>
            </FormItem>
          )}
          {importMode === 'clash' && (
            <FormItem label="批量 DNS 配置（可选）">
              <Textarea value={importDnsServers} onChange={e => onImportDnsServersChange(e.target.value)} rows={5}
                placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`} />
              <p className="text-xs text-[var(--color-text-muted)] mt-1">留空则不配置 DNS，填写后将应用到本次导入的所有代理</p>
            </FormItem>
          )}
        </div>
      </Modal>

      <Modal
        open={previewModalOpen}
        onClose={() => onPreviewModalOpenChange(false)}
        title="确认导入以下代理"
        width="700px"
        footer={
          <>
            <Button variant="secondary" onClick={() => onPreviewModalOpenChange(false)}>返回修改</Button>
            <Button onClick={onConfirmImport} loading={importing} disabled={previewList.length === 0}>确认导入</Button>
          </>
        }
      >
        <div className="space-y-3">
          {importMode === 'clash' && importDnsServers.trim() && (
            <p className="text-xs text-[var(--color-text-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 rounded">已配置批量 DNS，将应用到以下所有代理</p>
          )}
          <Table columns={previewColumns} data={previewList} rowKey="proxyId" maxHeight="380px" emptyText="无代理数据" />
        </div>
      </Modal>
    </>
  )
}
