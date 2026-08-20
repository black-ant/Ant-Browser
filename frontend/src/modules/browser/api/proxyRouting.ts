import type {
  BrowserProxySwitchResult,
  ProxyGatewayStatus,
  ProxyRoutingConfig,
  ProxyRoutingRule,
} from '../types'
import { getBindings, getGoApp } from './runtime'

function resolveBinding(name: string): ((...args: any[]) => any) | null {
  const bindings = (globalThis as any).__antBrowserBindings
  const app = getGoApp()
  const candidate = bindings?.[name] || app?.[name]
  return typeof candidate === 'function' ? candidate : null
}

async function loadBinding(name: string): Promise<((...args: any[]) => any) | null> {
  try {
    const bindings: any = await getBindings()
    const candidate = bindings?.[name] || resolveBinding(name)
    return typeof candidate === 'function' ? candidate : null
  } catch {
    return resolveBinding(name)
  }
}

function normalizeRules(rules: ProxyRoutingRule[]): ProxyRoutingRule[] {
  return (rules || []).map((rule, index) => ({
    id: String(rule.id || `rule-${Date.now()}-${index}`),
    name: String(rule.name || ''),
    enabled: rule.enabled !== false,
    matchType: rule.matchType || 'domain_suffix',
    pattern: String(rule.pattern || ''),
    action: rule.action || 'proxy',
  })).filter(rule => rule.pattern.trim())
}

export async function switchBrowserProfileProxy(
  profileId: string,
  proxyId: string,
  proxyConfig = '',
  force = false,
): Promise<BrowserProxySwitchResult> {
  const call = await loadBinding('BrowserProfileSwitchProxy')
  if (call) {
    return await call(profileId, { proxyId, proxyConfig, force })
  }
  throw new Error('当前环境不支持运行中切换代理，请重新构建应用')
}

export async function fetchBrowserProxyRouting(profileId: string): Promise<ProxyRoutingConfig> {
  const call = await loadBinding('BrowserProxyRoutingGet')
  if (call) {
    const result = await call(profileId)
    return {
      mode: result?.mode || 'proxy',
      rules: normalizeRules(result?.rules || []),
    }
  }
  return { mode: 'proxy', rules: [] }
}

export async function saveBrowserProxyRouting(
  profileId: string,
  config: ProxyRoutingConfig,
  force = false,
): Promise<ProxyGatewayStatus> {
  const normalized = { mode: config.mode || 'proxy', rules: normalizeRules(config.rules || []) }
  const call = await loadBinding('BrowserProxyRoutingSave')
  if (call) {
    return await call(profileId, normalized, force)
  }
  throw new Error('当前环境不支持保存代理分流，请重新构建应用')
}

export async function fetchBrowserProxyGatewayStatus(profileId: string): Promise<ProxyGatewayStatus> {
  const call = await loadBinding('BrowserProxyGatewayStatus')
  if (call) {
    return await call(profileId)
  }
  throw new Error('当前环境不支持读取代理网关状态，请重新构建应用')
}
