import type { BrowserFingerprintCapabilityReport, BrowserFingerprintCapabilityRow, BrowserFingerprintCheckResult } from '../../modules/browser/types'

function joinValues(values?: string[]) {
  return values?.length ? values.join(', ') : '—'
}

function displayValue(value: unknown): string {
  if (value === undefined || value === null || value === '') return '—'
  if (Array.isArray(value)) return joinValues(value)
  return String(value)
}

type MatchMode = 'exact' | 'contains' | 'platform' | 'browser-version' | 'platform-version' | 'display'
type MatchStatus = 'match' | 'version_compatible' | 'mismatch' | 'not_configured' | 'observe'

function normalizePlatform(value: unknown) {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (!normalized) return ''
  if (['windows', 'win', 'win32', 'win64', 'wince'].includes(normalized)) return 'windows'
  if (normalized.startsWith('linux') || normalized === 'x11') return 'linux'
  if (normalized === 'mac' || normalized === 'macos' || normalized.includes('mac')) return 'macos'
  return normalized
}

function versionParts(value: unknown): number[] {
  const match = String(value ?? '').replace(/_/g, '.').match(/\d+(?:\.\d+)*/)
  return match ? match[0].split('.').map((item) => Number.parseInt(item, 10)).filter(Number.isFinite) : []
}

function versionList(value: unknown, patterns: RegExp[]): number[][] {
  const text = String(value ?? '').replace(/_/g, '.')
  return patterns.flatMap((pattern) => Array.from(text.matchAll(pattern), (match) => versionParts(match[1])).filter((parts) => parts.length > 0))
}

function sameVersionPrefix(left: number[], right: number[]) {
  if (!left.length || !right.length) return false
  const size = Math.min(left.length, right.length)
  return left.slice(0, size).every((part, index) => part === right[index])
}

function matchStatus(expected: unknown, actual: unknown, mode: MatchMode = 'exact'): MatchStatus {
  if (mode === 'display') return 'observe'
  if (expected === undefined || expected === null || expected === '') return 'not_configured'
  if (mode === 'platform') return normalizePlatform(expected) && normalizePlatform(expected) === normalizePlatform(actual) ? 'match' : 'mismatch'
  if (mode === 'browser-version') {
    if (String(actual ?? '').includes(String(expected))) return 'match'
    const expectedParts = versionParts(expected)
    const versions = versionList(actual, [/(?:Chrome|Chromium|Edg|OPR|Vivaldi)\/([0-9]+(?:[._][0-9]+)*)/g, /"version"\s*:\s*"([0-9]+(?:[._][0-9]+)*)"/g])
    return expectedParts.length && versions.some((parts) => parts[0] === expectedParts[0]) ? 'version_compatible' : 'mismatch'
  }
  if (mode === 'platform-version') {
    if (String(actual ?? '').includes(String(expected))) return 'match'
    const expectedParts = versionParts(expected)
    const versions = versionList(actual, [/Windows NT\s+([0-9]+(?:[._][0-9]+)*)/g, /Mac OS X\s+([0-9]+(?:[._][0-9]+)*)/g, /Android\s+([0-9]+(?:[._][0-9]+)*)/g, /(?:CPU (?:iPhone )?OS|iPhone OS)\s+([0-9]+(?:[._][0-9]+)*)/g, /"platformVersion"\s*:\s*"([0-9]+(?:[._][0-9]+)*)"/g])
    return expectedParts.length && versions.some((parts) => sameVersionPrefix(expectedParts, parts)) ? 'version_compatible' : 'mismatch'
  }
  if (Array.isArray(actual)) {
    const expectedItems = String(expected).split(',').map((item) => item.trim()).filter(Boolean)
    const actualItems = actual.map((item) => String(item).trim()).filter(Boolean)
    return expectedItems.length > 0 && expectedItems.every((item, index) => actualItems[index] === item) ? 'match' : 'mismatch'
  }
  if (mode === 'contains') return String(actual).includes(String(expected)) ? 'match' : 'mismatch'
  return String(expected) === String(actual) ? 'match' : 'mismatch'
}

function statusLabel(status: MatchStatus) {
  return status === 'match' ? '一致' : status === 'version_compatible' ? '口径匹配' : status === 'mismatch' ? '不一致' : status === 'not_configured' ? '未设期望' : '观察值'
}

function CheckRow({ label, expected, actual, mode }: { label: string; expected?: unknown; actual: unknown; mode?: MatchMode }) {
  const status = matchStatus(expected, actual, mode)
  return <div className="fish-check-row"><span>{label}</span><code>{displayValue(expected)}</code><code>{displayValue(actual)}</code><strong className={`fish-status-badge ${status}`}>{statusLabel(status)}</strong></div>
}

export function FishFingerprintCheckReport({ result }: { result: BrowserFingerprintCheckResult }) {
  return (
    <div className="fish-check-table">
      <div className="fish-check-head"><span>项目</span><span>配置值</span><span>实际值</span><span>状态</span></div>
      <CheckRow label="语言" expected={result.expected.language} actual={result.runtime.language} />
      <CheckRow label="语言列表" expected={result.expected.acceptLanguage} actual={result.runtime.languages} />
      <CheckRow label="时区" expected={result.expected.timezone} actual={result.runtime.timezone} />
      <CheckRow label="CPU 核心" expected={result.expected.hardwareConcurrency} actual={result.runtime.hardwareConcurrency} />
      <CheckRow label="设备内存" actual={result.runtime.deviceMemory} mode="display" />
      <CheckRow label="颜色深度" actual={result.runtime.colorDepth} mode="display" />
      <CheckRow label="触控点" actual={result.runtime.maxTouchPoints} mode="display" />
      <CheckRow label="Do Not Track" actual={result.runtime.doNotTrack} mode="display" />
      <CheckRow label="窗口大小" expected={result.expected.windowSize} actual={`${result.runtime.outerWidth},${result.runtime.outerHeight}`} />
      <CheckRow label="平台" expected={result.expected.platform} actual={result.runtime.platform} mode="platform" />
      <CheckRow label="品牌版本" expected={result.expected.brandVersion} actual={result.runtime.userAgent} mode="browser-version" />
      <CheckRow label="系统版本" expected={result.expected.platformVersion} actual={result.runtime.userAgent} mode="platform-version" />
      <CheckRow label="UA" expected={result.expected.brand} actual={result.runtime.userAgent} mode="contains" />
      <CheckRow label="UA Data" actual={result.runtime.userAgentData} mode="display" />
      <CheckRow label="Webdriver" expected={false} actual={result.runtime.webdriver} />
      <CheckRow label="屏幕" actual={`${result.runtime.screenWidth}x${result.runtime.screenHeight} depth ${result.runtime.colorDepth} @ ${result.runtime.devicePixelRatio}`} mode="display" />
      <CheckRow label="WebGL Vendor" actual={result.runtime.webglVendor} mode="display" />
      <CheckRow label="WebGL Renderer" actual={result.runtime.webglRenderer} mode="display" />
      <CheckRow label="Canvas Hash" actual={result.runtime.canvasHash} mode="display" />
      <CheckRow label="Audio Hash" actual={result.runtime.audioHash} mode="display" />
      <CheckRow label="ClientRects Hash" actual={result.runtime.clientRectsHash} mode="display" />
      <CheckRow label="媒体设备数量" actual={result.runtime.mediaDeviceCount} mode="display" />
      <CheckRow label="Canvas 噪声" actual={result.expected.canvasNoise ? '已配置' : '未配置'} mode="display" />
      <CheckRow label="Audio 噪声" actual="不作为期望" mode="display" />
      <CheckRow label="ClientRects 噪声" actual={result.expected.clientRectsNoise ? '已配置' : '未配置'} mode="display" />
      <CheckRow label="插件" actual={result.runtime.plugins} mode="display" />
      <CheckRow label="种子" actual={result.expected.seed || '未显式设置'} mode="display" />
      <CheckRow label="排除伪装" actual={result.expected.disableSpoofing || '无'} mode="display" />
      <CheckRow label="WebRTC" actual={result.expected.webrtcPolicy || '未显式设置'} mode="display" />
    </div>
  )
}

const MATRIX_LABELS: Record<string, string> = {
  kept: '保留', injected: '补齐', inferred: '补齐', converted: '转换', removed: '清理', disabled: '关闭', overridden: '已覆盖', not_effective: '实测无效', pending: '待启动', kept_legacy: '旧版保留', kept_unconfirmed: '保守保留', kept_unknown: '原样保留',
}

function MatrixRow({ row }: { row: BrowserFingerprintCapabilityRow }) {
  return <div className="fish-matrix-row"><strong>{row.capability}</strong><span className={`fish-matrix-state ${row.status}`}>{MATRIX_LABELS[row.status] || row.status}</span><div><span>{row.action || '—'}</span>{row.inputArg || row.runtimeArg ? <code>{row.inputArg ? `配置：${row.inputArg}` : ''}{row.inputArg && row.runtimeArg && row.inputArg !== row.runtimeArg ? ' → ' : ''}{row.runtimeArg && row.inputArg !== row.runtimeArg ? `运行：${row.runtimeArg}` : ''}</code> : null}{row.note ? <small>{row.note}</small> : null}</div></div>
}

export function FishFingerprintMatrixReport({ report }: { report: BrowserFingerprintCapabilityReport | null }) {
  if (!report) return <div className="fish-empty compact"><strong>正在读取版本适配矩阵</strong><span>等待当前内核返回指纹能力信息。</span></div>
  return (
    <div>
      <div className="fish-report-summary"><strong>{report.chromeVersion ? `Chrome ${report.chromeVersion}` : '内核版本未知'}</strong><span>配置 {report.rawArgs?.length || 0} 项 · 运行 {report.launchArgs?.length || 0} 项</span></div>
      {report.warnings?.length ? <div className="fish-warning-list">{report.warnings.map((warning, index) => <span key={`${warning}-${index}`}>{warning}</span>)}</div> : null}
      <div className="fish-matrix-table"><div className="fish-matrix-head"><span>能力</span><span>状态</span><span>处理</span></div>{report.rows?.length ? report.rows.map((row, index) => <MatrixRow key={`${row.capability}-${row.inputArg}-${index}`} row={row} />) : <div className="fish-empty compact"><strong>没有需要转换的指纹参数</strong><span>当前配置可直接使用。</span></div>}</div>
    </div>
  )
}
