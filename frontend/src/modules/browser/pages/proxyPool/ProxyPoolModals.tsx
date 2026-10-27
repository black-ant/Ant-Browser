import { Button, FormItem, Input, Modal, Select, Table, Textarea } from '../../../../shared/components'
import type { TableColumn } from '../../../../shared/components/Table'
import type { ProxyIPHealthResult } from '../../types'

import {
  type ChainImportForm,
  type DirectImportForm,
  type ProxyDisplayInfo,
  type ProxyImportMode,
} from './helpers'

export interface ProxyEditFormValue {
  proxyName: string
  proxyConfig: string
  preferredKernel: string
  dnsServers: string
  groupName: string
}

export { ProxyPoolImportModal } from './ProxyPoolImportModal'

interface ProxyPoolPreviewModalProps {
  open: boolean
  importMode: ProxyImportMode
  importDnsServers: string
  previewList: ProxyDisplayInfo[]
  removedPreviewProxyNames: string[]
  importing: boolean
  onClose: () => void
  onBack: () => void
  onConfirm: () => void
  onRemoveProxy: (proxyId: string) => void
}

export function ProxyPoolPreviewModal({
  open,
  importMode,
  importDnsServers,
  previewList,
  removedPreviewProxyNames,
  importing,
  onClose,
  onBack,
  onConfirm,
  onRemoveProxy,
}: ProxyPoolPreviewModalProps) {
  const previewColumns: TableColumn<ProxyDisplayInfo>[] = [
    { key: 'proxyName', title: '代理名称', width: '200px' },
    { key: 'type', title: '类型', width: '100px' },
    { key: 'server', title: '服务器', width: '200px' },
    { key: 'port', title: '端口', width: '100px', render: (value) => value || '-' },
    {
      key: 'actions',
      title: '操作',
      width: '96px',
      render: (_, record) => (
        <Button size="sm" variant="danger" onClick={() => onRemoveProxy(record.proxyId)}>
          删除
        </Button>
      ),
    },
  ]

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="确认导入以下代理"
      width="700px"
      footer={
        <>
          <Button variant="secondary" onClick={onBack}>
            返回修改
          </Button>
          <Button onClick={onConfirm} loading={importing} disabled={previewList.length === 0}>
            确认导入
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        {importMode === 'clash' && importDnsServers.trim() && (
          <p className="text-xs text-[var(--color-text-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 rounded">
            已配置批量 DNS，将应用到以下所有代理
          </p>
        )}
        <p className="text-xs text-[var(--color-text-muted)]">
          保留 {previewList.length} 条，删除 {removedPreviewProxyNames.length} 条。删除项不会进入后续比较环节。
        </p>
        <Table columns={previewColumns} data={previewList} rowKey="proxyId" maxHeight="380px" emptyText="无代理数据" />
      </div>
    </Modal>
  )
}

interface ProxyPoolEditModalProps {
  open: boolean
  saving: boolean
  groups: string[]
  editForm: ProxyEditFormValue
  chainEditMode: boolean
  chainEditForm: ChainImportForm
  directEditMode: boolean
  directEditForm: DirectImportForm
  onClose: () => void
  onSave: () => void
  onChange: (patch: Partial<ProxyEditFormValue>) => void
  onChainEditFormChange: (patch: Partial<ChainImportForm>) => void
  onDirectEditFormChange: (patch: Partial<DirectImportForm>) => void
  onChainEditHopChange: (hop: 'first' | 'second', field: keyof ChainImportForm['first'], value: string) => void
}

export function ProxyPoolEditModal({
  open,
  saving,
  groups,
  editForm,
  chainEditMode,
  chainEditForm,
  directEditMode,
  directEditForm,
  onClose,
  onSave,
  onChange,
  onChainEditFormChange,
  onDirectEditFormChange,
  onChainEditHopChange,
}: ProxyPoolEditModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="编辑代理"
      width="500px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            取消
          </Button>
          <Button onClick={onSave} loading={saving}>
            保存
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="代理名称" required>
          <Input
            value={chainEditMode ? chainEditForm.proxyName : directEditMode ? directEditForm.proxyName : editForm.proxyName}
            onChange={(event) => {
              if (chainEditMode) {
                onChainEditFormChange({ proxyName: event.target.value })
                return
              }
              if (directEditMode) {
                onDirectEditFormChange({ proxyName: event.target.value })
                return
              }
              onChange({ proxyName: event.target.value })
            }}
            placeholder="节点名称"
          />
        </FormItem>
        <FormItem label="分组名称（可选）">
          <Input
            value={editForm.groupName}
            onChange={(event) => onChange({ groupName: event.target.value })}
            placeholder="分组名称"
            list="edit-proxy-groups-datalist"
          />
          <datalist id="edit-proxy-groups-datalist">
            {groups.map((group) => (
              <option key={group} value={group} />
            ))}
          </datalist>
        </FormItem>
        <FormItem label="代理内核">
          <Select
            value={editForm.preferredKernel || 'auto'}
            onChange={(event) => onChange({ preferredKernel: event.target.value })}
            options={[
              { value: 'auto', label: '自动' },
              { value: 'xray', label: 'Xray' },
              { value: 'sing-box', label: 'sing-box' },
              { value: 'mihomo', label: 'Mihomo' },
              { value: 'mieru', label: 'mieru' },
            ]}
          />
        </FormItem>
        {chainEditMode ? (
          <div className="space-y-4">
            <FormItem label="本地监听端口（可选）">
              <Input
                type="number"
                min={1}
                max={65535}
                value={chainEditForm.localPort}
                onChange={(event) => onChainEditFormChange({ localPort: event.target.value })}
                placeholder="留空自动分配"
              />
            </FormItem>
            <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
              <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第一层代理</h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormItem label="协议">
                  <Select
                    value={chainEditForm.first.protocol}
                    onChange={(event) => onChainEditHopChange('first', 'protocol', event.target.value)}
                    options={[
                      { value: 'http', label: 'HTTP' },
                      { value: 'socks5', label: 'SOCKS5' },
                    ]}
                  />
                </FormItem>
                <FormItem label="代理地址" required>
                  <Input
                    value={chainEditForm.first.server}
                    onChange={(event) => onChainEditHopChange('first', 'server', event.target.value)}
                  />
                </FormItem>
                <FormItem label="代理端口" required>
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={chainEditForm.first.port}
                    onChange={(event) => onChainEditHopChange('first', 'port', event.target.value)}
                  />
                </FormItem>
                <FormItem label="账号（可选）">
                  <Input
                    value={chainEditForm.first.username}
                    onChange={(event) => onChainEditHopChange('first', 'username', event.target.value)}
                  />
                </FormItem>
                <FormItem label="密码（可选）">
                  <Input
                    type="password"
                    value={chainEditForm.first.password}
                    onChange={(event) => onChainEditHopChange('first', 'password', event.target.value)}
                  />
                </FormItem>
              </div>
            </div>
            <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
              <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第二层代理</h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormItem label="协议">
                  <Select
                    value={chainEditForm.second.protocol}
                    onChange={(event) => onChainEditHopChange('second', 'protocol', event.target.value)}
                    options={[
                      { value: 'http', label: 'HTTP' },
                      { value: 'socks5', label: 'SOCKS5' },
                    ]}
                  />
                </FormItem>
                <FormItem label="代理地址" required>
                  <Input
                    value={chainEditForm.second.server}
                    onChange={(event) => onChainEditHopChange('second', 'server', event.target.value)}
                  />
                </FormItem>
                <FormItem label="代理端口" required>
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={chainEditForm.second.port}
                    onChange={(event) => onChainEditHopChange('second', 'port', event.target.value)}
                  />
                </FormItem>
                <FormItem label="账号（可选）">
                  <Input
                    value={chainEditForm.second.username}
                    onChange={(event) => onChainEditHopChange('second', 'username', event.target.value)}
                  />
                </FormItem>
                <FormItem label="密码（可选）">
                  <Input
                    type="password"
                    value={chainEditForm.second.password}
                    onChange={(event) => onChainEditHopChange('second', 'password', event.target.value)}
                  />
                </FormItem>
              </div>
            </div>
          </div>
        ) : directEditMode ? (
          <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
            <h4 className="text-sm font-medium text-[var(--color-text-primary)]">代理参数</h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <FormItem label="协议">
                <Select
                  value={directEditForm.protocol}
                  onChange={(event) => onDirectEditFormChange({ protocol: event.target.value as DirectImportForm['protocol'] })}
                  options={[
                    { value: 'http', label: 'HTTP' },
                    { value: 'https', label: 'HTTPS' },
                    { value: 'socks5', label: 'SOCKS5' },
                  ]}
                />
              </FormItem>
              <FormItem label="代理地址" required>
                <Input
                  value={directEditForm.server}
                  onChange={(event) => onDirectEditFormChange({ server: event.target.value })}
                />
              </FormItem>
              <FormItem label="代理端口" required>
                <Input
                  type="number"
                  min={1}
                  max={65535}
                  value={directEditForm.port}
                  onChange={(event) => onDirectEditFormChange({ port: event.target.value })}
                />
              </FormItem>
              <FormItem label="账号（可选）">
                <Input
                  value={directEditForm.username}
                  onChange={(event) => onDirectEditFormChange({ username: event.target.value })}
                />
              </FormItem>
              <FormItem label="密码（可选）">
                <Input
                  type="password"
                  value={directEditForm.password}
                  onChange={(event) => onDirectEditFormChange({ password: event.target.value })}
                />
              </FormItem>
            </div>
          </div>
        ) : (
          <FormItem label="代理配置">
            <Textarea
              value={editForm.proxyConfig}
              onChange={(event) => onChange({ proxyConfig: event.target.value })}
              rows={10}
              placeholder="支持 Clash YAML、chain+socks5:// 或其它高级配置"
            />
          </FormItem>
        )}
        <FormItem label="DNS 服务器（可选）">
          <Textarea
            value={editForm.dnsServers}
            onChange={(event) => onChange({ dnsServers: event.target.value })}
            rows={6}
            placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`}
          />
        </FormItem>
      </div>
    </Modal>
  )
}

interface ProxyPoolIPHealthDetailModalProps {
  open: boolean
  detail: ProxyIPHealthResult | null
  onClose: () => void
}

export function ProxyPoolIPHealthDetailModal({
  open,
  detail,
  onClose,
}: ProxyPoolIPHealthDetailModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="IP健康原始返回"
      width="760px"
      footer={
        <Button variant="secondary" onClick={onClose}>
          关闭
        </Button>
      }
    >
      <div className="space-y-3">
        {detail && (
          <>
            <div className="text-xs text-[var(--color-text-muted)]">
              代理ID：{detail.proxyId} | 来源：{detail.source} | 时间：{detail.updatedAt}
            </div>
            {!detail.ok && <div className="text-sm text-red-500">{detail.error || '检测失败'}</div>}
            <pre className="max-h-[420px] overflow-auto text-xs leading-5 rounded-lg bg-[var(--color-bg-secondary)] border border-[var(--color-border)] p-3">
              {JSON.stringify(detail.rawData || {}, null, 2)}
            </pre>
          </>
        )}
      </div>
    </Modal>
  )
}
