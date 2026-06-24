import { Card, Input } from '../../../../shared/components'
import type { FingerprintProfileConfig, ProxyType } from './types'
import type { ValidationError } from './validation'
import { getFieldError } from './validation'
import type { BrowserProxy } from '../../types'

interface ProxyConfigSectionProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
  errors: ValidationError[]
  proxies: BrowserProxy[]  // 代理池列表
}

export function ProxyConfigSection({ config, updateConfig, errors, proxies }: ProxyConfigSectionProps) {
  const proxyTypes: { value: ProxyType; label: string }[] = [
    { value: 'none', label: '直连' },
    { value: 'http', label: 'HTTP' },
    { value: 'https', label: 'HTTPS' },
    { value: 'socks5', label: 'SOCKS5' },
  ]

  const hostError = getFieldError(errors, 'proxy.host')
  const portError = getFieldError(errors, 'proxy.port')

  return (
    <Card>
      <div className="p-6">
        <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-4">代理 IP</h3>

        <div className="space-y-4">
          {/* 代理模式切换 */}
          <div>
            <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
              代理模式
            </label>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => {
                  updateConfig(prev => ({
                    ...prev,
                    proxy: { ...prev.proxy, mode: 'pool', proxyId: '', type: 'none' }
                  }))
                }}
                className={`
                  px-4 py-2 text-sm font-medium rounded-lg border-2 transition-all
                  ${config.proxy.mode === 'pool'
                    ? 'border-[var(--color-primary)] bg-[var(--color-primary-light)] text-[var(--color-primary)]'
                    : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-hover)]'
                  }
                `}
              >
                代理池
              </button>
              <button
                type="button"
                onClick={() => {
                  updateConfig(prev => ({
                    ...prev,
                    proxy: { ...prev.proxy, mode: 'manual', proxyId: '' }
                  }))
                }}
                className={`
                  px-4 py-2 text-sm font-medium rounded-lg border-2 transition-all
                  ${config.proxy.mode === 'manual'
                    ? 'border-[var(--color-primary)] bg-[var(--color-primary-light)] text-[var(--color-primary)]'
                    : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-hover)]'
                  }
                `}
              >
                手动配置
              </button>
            </div>
          </div>

          {/* 代理池选择模式 */}
          {config.proxy.mode === 'pool' && (
            <div>
              <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                选择代理节点
              </label>
              <select
                value={config.proxy.proxyId}
                onChange={(e) => {
                  updateConfig(prev => ({
                    ...prev,
                    proxy: { ...prev.proxy, proxyId: e.target.value }
                  }))
                }}
                className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-default)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent"
              >
                <option value="">直连（不使用代理）</option>
                {proxies.map(proxy => (
                  <option key={proxy.proxyId} value={proxy.proxyId}>
                    {proxy.proxyName} - {proxy.proxyConfig}
                    {proxy.groupName && ` (${proxy.groupName})`}
                    {proxy.lastLatencyMs && ` - ${proxy.lastLatencyMs}ms`}
                  </option>
                ))}
              </select>
              {proxies.length === 0 && (
                <p className="mt-2 text-sm text-[var(--color-text-tertiary)]">
                  暂无可用代理节点，请先在代理池页面添加
                </p>
              )}
            </div>
          )}

          {/* 手动配置模式 */}
          {config.proxy.mode === 'manual' && (
            <>
              {/* 代理类型 */}
              <div>
                <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                  代理类型
                </label>
                <div className="flex gap-2">
                  {proxyTypes.map(type => (
                    <button
                      key={type.value}
                      type="button"
                      onClick={() => {
                        updateConfig(prev => ({
                          ...prev,
                          proxy: { ...prev.proxy, type: type.value }
                        }))
                      }}
                      className={`
                        px-4 py-2 text-sm font-medium rounded-lg border-2 transition-all
                        ${config.proxy.type === type.value
                          ? 'border-[var(--color-primary)] bg-[var(--color-primary-light)] text-[var(--color-primary)]'
                          : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-hover)]'
                        }
                      `}
                    >
                      {type.label}
                    </button>
                  ))}
                </div>
              </div>

              {/* 代理配置详情（仅在非 none 时显示） */}
              {config.proxy.type !== 'none' && (
                <>
                  {/* 代理地址和端口 */}
                  <div className="grid grid-cols-3 gap-3">
                    <div className="col-span-2">
                      <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                        代理 IP/域名
                      </label>
                      <Input
                        value={config.proxy.host}
                        onChange={(e) => {
                          updateConfig(prev => ({
                            ...prev,
                            proxy: { ...prev.proxy, host: e.target.value }
                          }))
                        }}
                        placeholder="例如: 127.0.0.1 或 proxy.example.com"
                        className={hostError ? 'border-[var(--color-error)]' : ''}
                      />
                      {hostError && (
                        <p className="mt-1 text-sm text-[var(--color-error)]">{hostError}</p>
                      )}
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                        端口
                      </label>
                      <Input
                        value={config.proxy.port}
                        onChange={(e) => {
                          updateConfig(prev => ({
                            ...prev,
                            proxy: { ...prev.proxy, port: e.target.value }
                          }))
                        }}
                        placeholder="7890"
                        className={portError ? 'border-[var(--color-error)]' : ''}
                      />
                      {portError && (
                        <p className="mt-1 text-sm text-[var(--color-error)]">{portError}</p>
                      )}
                    </div>
                  </div>

                  {/* 代理认证 */}
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                        用户名（可选）
                      </label>
                      <Input
                        value={config.proxy.username}
                        onChange={(e) => {
                          updateConfig(prev => ({
                            ...prev,
                            proxy: { ...prev.proxy, username: e.target.value }
                          }))
                        }}
                        placeholder="代理用户名"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                        密码（可选）
                      </label>
                      <Input
                        type="password"
                        value={config.proxy.password}
                        onChange={(e) => {
                          updateConfig(prev => ({
                            ...prev,
                            proxy: { ...prev.proxy, password: e.target.value }
                          }))
                        }}
                        placeholder="代理密码"
                      />
                    </div>
                  </div>
                </>
              )}
            </>
          )}
        </div>
      </div>
    </Card>
  )
}
