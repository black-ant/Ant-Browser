import type { Dispatch, SetStateAction } from 'react'
import { Button, FormItem, Input, Modal, Select, Textarea } from '../../../shared/components'
import type { ChainEditForm, ChainHopForm } from './ProxyPickerModal.helpers'

interface ProxyEditModalProps {
  open: boolean
  chainEditMode: boolean
  editName: string
  editConfig: string
  editGroup: string
  editDnsServers: string
  chainEditForm: ChainEditForm
  saving: boolean
  setEditName: Dispatch<SetStateAction<string>>
  setEditConfig: Dispatch<SetStateAction<string>>
  setEditGroup: Dispatch<SetStateAction<string>>
  setEditDnsServers: Dispatch<SetStateAction<string>>
  setChainEditForm: Dispatch<SetStateAction<ChainEditForm>>
  updateChainHop: (hop: 'first' | 'second', field: keyof ChainHopForm, value: string) => void
  onClose: () => void
  onSave: () => void
}

export function ProxyEditModal({
  open,
  chainEditMode,
  editName,
  editConfig,
  editGroup,
  editDnsServers,
  chainEditForm,
  saving,
  setEditName,
  setEditConfig,
  setEditGroup,
  setEditDnsServers,
  setChainEditForm,
  updateChainHop,
  onClose,
  onSave,
}: ProxyEditModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="编辑代理"
      width="520px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>取消</Button>
          <Button onClick={onSave} loading={saving}>保存</Button>
        </>
      }
    >
      <div className="space-y-3">
        <FormItem label="代理名称" required>
          <Input
            value={chainEditMode ? chainEditForm.proxyName : editName}
            onChange={e => {
              if (chainEditMode) {
                setChainEditForm(prev => ({ ...prev, proxyName: e.target.value }))
              } else {
                setEditName(e.target.value)
              }
            }}
            placeholder="节点名称"
          />
        </FormItem>

        <FormItem label="分组名称（可选）">
          <Input value={editGroup} onChange={e => setEditGroup(e.target.value)} placeholder="分组名称" />
        </FormItem>

        {chainEditMode ? (
          <div className="space-y-3 rounded-md border border-[var(--color-border)] p-3">
            <FormItem label="本地监听端口（可选）">
              <Input
                type="number"
                min={1}
                max={65535}
                value={chainEditForm.localPort}
                onChange={e => setChainEditForm(prev => ({ ...prev, localPort: e.target.value }))}
                placeholder="留空自动分配"
              />
            </FormItem>
            <ChainHopSection title="第一层代理" hop="first" form={chainEditForm} updateChainHop={updateChainHop} />
            <ChainHopSection title="第二层代理" hop="second" form={chainEditForm} updateChainHop={updateChainHop} />
          </div>
        ) : (
          <FormItem label="代理配置" required>
            <Textarea
              value={editConfig}
              onChange={e => setEditConfig(e.target.value)}
              rows={6}
              placeholder="支持 http://、https://、socks5://、chain+socks5://"
            />
          </FormItem>
        )}

        <FormItem label="DNS 服务器（可选）">
          <Textarea
            value={editDnsServers}
            onChange={e => setEditDnsServers(e.target.value)}
            rows={4}
            placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`}
          />
        </FormItem>
      </div>
    </Modal>
  )
}

function ChainHopSection({
  title,
  hop,
  form,
  updateChainHop,
}: {
  title: string
  hop: 'first' | 'second'
  form: ChainEditForm
  updateChainHop: (hop: 'first' | 'second', field: keyof ChainHopForm, value: string) => void
}) {
  const hopForm = form[hop]

  return (
    <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
      <h4 className="text-sm font-medium text-[var(--color-text-primary)]">{title}</h4>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <FormItem label="协议">
          <Select
            value={hopForm.protocol}
            onChange={e => updateChainHop(hop, 'protocol', e.target.value)}
            options={[
              { value: 'http', label: 'HTTP' },
              { value: 'socks5', label: 'SOCKS5' },
            ]}
          />
        </FormItem>
        <FormItem label="代理地址" required>
          <Input value={hopForm.server} onChange={e => updateChainHop(hop, 'server', e.target.value)} />
        </FormItem>
        <FormItem label="代理端口" required>
          <Input type="number" min={1} max={65535} value={hopForm.port} onChange={e => updateChainHop(hop, 'port', e.target.value)} />
        </FormItem>
        <FormItem label="账号（可选）">
          <Input value={hopForm.username} onChange={e => updateChainHop(hop, 'username', e.target.value)} />
        </FormItem>
        <FormItem label="密码（可选）">
          <Input type="password" value={hopForm.password} onChange={e => updateChainHop(hop, 'password', e.target.value)} />
        </FormItem>
      </div>
    </div>
  )
}
