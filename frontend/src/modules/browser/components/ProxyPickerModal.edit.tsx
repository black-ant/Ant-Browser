import type { Dispatch, SetStateAction } from 'react'
import { Button, FormItem, Input, Modal, Select, Textarea } from '../../../shared/components'
import type { BrowserProxy } from '../types'
import { detectProxyNodeProtocol } from '../pages/proxyPool/helpers'
import type { ChainEditForm, ChainHopForm } from './ProxyPickerModal.helpers'

interface ProxyEditModalProps {
  open: boolean
  chainEditMode: boolean
  editName: string
  editConfig: string
  editGroup: string
  editDnsServers: string
  chainEditForm: ChainEditForm
  proxies: BrowserProxy[]
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
  proxies,
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
            <ChainHopSection title="第一层代理" hop="first" form={chainEditForm} proxies={proxies} setForm={setChainEditForm} updateChainHop={updateChainHop} />
            <ChainHopSection title="第二层落地代理" hop="second" form={chainEditForm} proxies={proxies} setForm={setChainEditForm} updateChainHop={updateChainHop} />
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
  proxies,
  setForm,
  updateChainHop,
}: {
  title: string
  hop: 'first' | 'second'
  form: ChainEditForm
  proxies: BrowserProxy[]
  setForm: Dispatch<SetStateAction<ChainEditForm>>
  updateChainHop: (hop: 'first' | 'second', field: keyof ChainHopForm, value: string) => void
}) {
  const hopForm = form[hop]
  const firstProtocol = hop === 'first'
    ? (form.firstMode === 'node' ? form.firstNodeProtocol : hopForm.protocol)
    : hopForm.protocol
  const nodeOptions = hop === 'first'
    ? proxies.filter(proxy => detectProxyNodeProtocol(proxy.proxyConfig) === form.firstNodeProtocol)
    : []
  const selectedNodeId = nodeOptions.find(proxy => proxy.proxyConfig.trim() === form.firstProxyConfig.trim())?.proxyId || ''
  const updateProtocol = (value: string) => {
    if (hop !== 'first') {
      updateChainHop(hop, 'protocol', value)
      return
    }
    if (value === 'http' || value === 'socks5') {
      setForm(prev => ({ ...prev, firstMode: 'standard' }))
      updateChainHop('first', 'protocol', value)
      return
    }
    setForm(prev => ({ ...prev, firstMode: 'node', firstNodeProtocol: value as ChainEditForm['firstNodeProtocol'], ...(prev.firstNodeSource === 'pool' ? { firstProxyConfig: '' } : {}) }))
  }
  const nodeMode = hop === 'first' && form.firstMode === 'node'
  const nodeProtocolLabel = form.firstNodeProtocol === 'hysteria2' ? 'HY2' : form.firstNodeProtocol.toUpperCase()

  return (
    <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
      <h4 className="text-sm font-medium text-[var(--color-text-primary)]">{title}</h4>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <FormItem label="协议">
          <Select
            value={firstProtocol}
            onChange={e => updateProtocol(e.target.value)}
            options={hop === 'first' ? [
              { value: 'http', label: 'HTTP' },
              { value: 'socks5', label: 'SOCKS5' },
              { value: 'vless', label: 'VLESS' },
              { value: 'vmess', label: 'VMess' },
              { value: 'trojan', label: 'Trojan' },
              { value: 'ss', label: 'SS' },
              { value: 'hysteria2', label: 'HY2' },
            ] : [
              { value: 'http', label: 'HTTP' },
              { value: 'socks5', label: 'SOCKS5' },
            ]}
          />
        </FormItem>
        {nodeMode && (
          <FormItem label="节点来源">
            <Select
              value={form.firstNodeSource}
              onChange={e => setForm(prev => ({ ...prev, firstNodeSource: e.target.value as ChainEditForm['firstNodeSource'], ...(e.target.value === 'pool' ? { firstProxyConfig: '' } : {}) }))}
              options={[{ value: 'pool', label: '从代理池选择' }, { value: 'manual', label: '手动输入' }]}
            />
          </FormItem>
        )}
      </div>
      {nodeMode ? form.firstNodeSource === 'pool' ? (
        <FormItem label="已导入节点" required>
          <Select
            value={selectedNodeId}
            onChange={e => {
              const proxy = nodeOptions.find(item => item.proxyId === e.target.value)
              setForm(prev => ({ ...prev, firstProxyConfig: proxy?.proxyConfig || '' }))
            }}
            options={[
              { value: '', label: nodeOptions.length ? '请选择节点' : `暂无 ${nodeProtocolLabel} 节点` },
              ...nodeOptions.map(proxy => ({ value: proxy.proxyId, label: `${proxy.proxyName}${proxy.groupName ? ` · ${proxy.groupName}` : ''}` })),
            ]}
          />
          <p className="text-xs text-[var(--color-text-muted)] mt-1">选择后复制节点配置，不与原节点保持引用。</p>
        </FormItem>
      ) : (
        <FormItem label={`${nodeProtocolLabel} 节点配置`} required>
          <Textarea
            value={form.firstProxyConfig}
            onChange={e => setForm(prev => ({ ...prev, firstProxyConfig: e.target.value }))}
            rows={6}
            placeholder={`${form.firstNodeProtocol}://...（也支持兼容 Clash 单节点 YAML）`}
          />
        </FormItem>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
      )}
    </div>
  )
}
