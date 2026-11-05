import type { DirectImportForm, ImportCandidate } from './helpers.types'

function normalizeDirectProxyConfig(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (/^socket:\/\//i.test(trimmed)) {
    return trimmed.replace(/^socket:\/\//i, 'socks5://')
  }
  if (/^socks:\/\//i.test(trimmed)) {
    return trimmed.replace(/^socks:\/\//i, 'socks5://')
  }
  return trimmed
}

function resolveDirectProxyName(
  rawName: string,
  scheme: string,
  server: string,
  port: number,
  index: number,
  prefix: string,
): string {
  const name = rawName.trim()
  const fallbackName = server
    ? `${scheme.toUpperCase()}-${server}${port > 0 ? `:${port}` : ''}`
    : `导入代理 ${index + 1}`
  const finalName = name || fallbackName
  return prefix ? `${prefix}-${finalName}` : finalName
}

function formatDirectProxyHost(raw: string): string {
  const host = raw.trim()
  if (!host) return ''
  if (host.startsWith('[') && host.endsWith(']')) {
    return host
  }
  return host.includes(':') ? `[${host}]` : host
}

function normalizeDirectProtocol(raw: unknown): DirectImportForm['protocol'] {
  const protocol = String(raw || '').trim().toLowerCase()
  if (protocol === 'http' || protocol === 'https' || protocol === 'socks5') {
    return protocol
  }
  if (protocol === 'socks' || protocol === 'socket') {
    return 'socks5'
  }
  throw new Error('protocol 仅支持 http / https / socks5')
}

export function parseDirectProxyURL(raw: string): DirectImportForm {
  const normalized = normalizeDirectProxyConfig(raw)
  if (!normalized) {
    throw new Error('请输入标准代理地址')
  }
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(normalized)) {
    throw new Error('单行文本需要包含协议头，需要包含协议头')
  }

  let parsedURL: URL
  try {
    parsedURL = new URL(normalized)
  } catch {
    throw new Error('单行代理文本格式无效')
  }

  const protocol = normalizeDirectProtocol(parsedURL.protocol.replace(/:$/, ''))
  const server = parsedURL.hostname.replace(/^\[(.*)\]$/, '$1').trim()
  if (!server) {
    throw new Error('代理地址缺少主机名')
  }

  const port = Number(parsedURL.port)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('代理地址缺少有效端口')
  }

  return {
    proxyName: '',
    protocol,
    server,
    port: String(port),
    username: parsedURL.username ? decodeURIComponent(parsedURL.username) : '',
    password: parsedURL.password ? decodeURIComponent(parsedURL.password) : '',
  }
}

export function toDirectImportForm(proxyName: string, proxyConfig: string): DirectImportForm | null {
  try {
    const form = parseDirectProxyURL(proxyConfig)
    return {
      ...form,
      proxyName,
    }
  } catch {
    return null
  }
}

export function buildDirectImportCandidate(form: DirectImportForm): ImportCandidate {
  const serverInput = form.server.trim()
  if (!serverInput) {
    throw new Error('请输入代理地址')
  }
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(serverInput)) {
    throw new Error('代理地址只需要填写主机名或 IP，不需要协议头')
  }

  const portInput = form.port.trim()
  if (!portInput) {
    throw new Error('请输入代理端口')
  }
  if (!/^\d+$/.test(portInput)) {
    throw new Error('代理端口必须为数字')
  }

  const port = Number(portInput)
  if (port < 1 || port > 65535) {
    throw new Error('代理端口必须在 1-65535 之间')
  }

  const username = form.username.trim()
  const password = form.password
  if (password && !username) {
    throw new Error('填写密码时请同时填写账号')
  }

  const auth = username
    ? `${encodeURIComponent(username)}${password ? `:${encodeURIComponent(password)}` : ''}@`
    : ''
  const rawConfig = `${form.protocol}://${auth}${formatDirectProxyHost(serverInput)}:${port}`

  let parsedURL: URL
  try {
    parsedURL = new URL(rawConfig)
  } catch {
    throw new Error('请输入有效的代理地址')
  }

  if (!parsedURL.hostname) {
    throw new Error('请输入有效的代理地址')
  }

  const normalizedConfig = normalizeDirectProxyConfig(parsedURL.toString()).replace(/\/$/, '')
  const normalizedServer = parsedURL.hostname.replace(/^\[(.*)\]$/, '$1')

  return {
    proxyName: resolveDirectProxyName(form.proxyName, form.protocol, normalizedServer, port, 0, ''),
    proxyConfig: normalizedConfig,
  }
}

interface ParsedDirectImportItem {
  form: DirectImportForm
  groupName: string
}

function parseDirectImportObject(payload: Record<string, unknown>, fallbackGroupName: string): ParsedDirectImportItem {
  const proxyName = String(payload.name ?? payload.proxyName ?? '').trim()
  const groupName = String(payload.group ?? payload.groupName ?? fallbackGroupName).trim()
  const proxyURL = String(payload.url ?? payload.proxyUrl ?? payload.proxy ?? payload.proxyConfig ?? '').trim()
  if (proxyURL) {
    const parsedForm = parseDirectProxyURL(proxyURL)
    return {
      form: {
        ...parsedForm,
        proxyName: proxyName || parsedForm.proxyName,
      },
      groupName,
    }
  }

  const protocol = normalizeDirectProtocol(payload.protocol ?? payload.scheme)
  const server = String(payload.server ?? payload.host ?? '').trim()
  if (!server) {
    throw new Error('JSON 缺少 server')
  }

  const portValue = Number(payload.port)
  if (!Number.isInteger(portValue) || portValue < 1 || portValue > 65535) {
    throw new Error('JSON 缺少有效 port')
  }

  const username = String(payload.username ?? payload.user ?? '').trim()
  const password = payload.password === undefined || payload.password === null ? '' : String(payload.password)
  if (password && !username) {
    throw new Error('填写 password 时请同时填写 username')
  }

  return {
    form: {
      proxyName,
      protocol,
      server,
      port: String(portValue),
      username,
      password,
    },
    groupName,
  }
}

function parseDirectImportItems(raw: string): { items: ParsedDirectImportItem[]; defaultGroupName: string } {
  const text = raw.trim()
  if (!text) {
    throw new Error('请输入 HTTP / SOCKS5 文本')
  }

  if (text.startsWith('{') || text.startsWith('[')) {
    let payload: unknown
    try {
      payload = JSON.parse(text)
    } catch {
      throw new Error('JSON 格式无效')
    }

    let defaultGroupName = ''
    let sources: unknown[] = []
    if (Array.isArray(payload)) {
      sources = payload
    } else if (payload && typeof payload === 'object') {
      const record = payload as Record<string, unknown>
      defaultGroupName = String(record.group ?? record.groupName ?? '').trim()
      if (Array.isArray(record.proxies)) {
        sources = record.proxies
      } else if (Array.isArray(record.items)) {
        sources = record.items
      } else if (Array.isArray(record.list)) {
        sources = record.list
      } else {
        sources = [record]
      }
    } else {
      throw new Error('JSON 根节点必须是对象或数组')
    }

    const items = sources.map((item, index) => {
      if (typeof item === 'string') {
        return {
          form: parseDirectProxyURL(item),
          groupName: defaultGroupName,
        }
      }
      if (!item || typeof item !== 'object' || Array.isArray(item)) {
        throw new Error(`第 ${index + 1} 项格式无效`)
      }
      return parseDirectImportObject(item as Record<string, unknown>, defaultGroupName)
    })

    if (items.length === 0) {
      throw new Error('JSON 未解析到可导入代理')
    }
    return { items, defaultGroupName }
  }

  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#') && !line.startsWith('//'))
  if (lines.length === 0) {
    throw new Error('请输入标准代理地址')
  }

  return {
    items: lines.map((line) => ({
      form: parseDirectProxyURL(line),
      groupName: '',
    })),
    defaultGroupName: '',
  }
}

export function parseDirectImportText(raw: string): { form: DirectImportForm; groupName: string } {
  const { items } = parseDirectImportItems(raw)
  if (items.length !== 1) {
    throw new Error('检测到多条代理，请直接点击解析进行批量导入')
  }
  return items[0]
}

export function buildDirectImportCandidatesFromText(raw: string): { candidates: ImportCandidate[]; defaultGroupName: string } {
  const { items, defaultGroupName } = parseDirectImportItems(raw)
  return {
    candidates: items.map((item) => ({
      ...buildDirectImportCandidate(item.form),
      groupName: item.groupName,
    })),
    defaultGroupName,
  }
}
