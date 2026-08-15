export const FINGERPRINT_CHROMIUM_RELEASES_URL = 'https://github.com/adryfish/fingerprint-chromium/releases'

const FINGERPRINT_CHROMIUM_LATEST_RELEASE_API_URL = 'https://api.github.com/repos/adryfish/fingerprint-chromium/releases/latest'

type CoreDownloadPlatform = 'windows' | 'linux' | 'darwin' | ''
type CoreDownloadArch = 'amd64' | 'arm64' | ''

export interface CoreDownloadRuntimeEnvironment {
  platform?: string
  arch?: string
}

export interface CoreDownloadTarget {
  platform: CoreDownloadPlatform
  arch: CoreDownloadArch
  label: string
}

export interface CoreDownloadRecommendation {
  target: CoreDownloadTarget
  releaseTag: string
  assetName: string
  downloadUrl: string
  releasesUrl: string
  namePlaceholder: string
}

interface GitHubReleaseAsset {
  name?: string
  browser_download_url?: string
}

interface GitHubReleaseResponse {
  tag_name?: string
  html_url?: string
  assets?: GitHubReleaseAsset[]
}

// .dmg 是 macOS 内核镜像;后端已支持挂载解包(download_core_dmg_darwin.go),
// 因此纳入可识别归档后缀,darwin 才能选中 *_macos.dmg。
const archiveSuffixes = ['.zip', '.tar.xz', '.txz', '.tar.gz', '.tgz', '.tar.bz2', '.tbz2', '.tar', '.dmg']

function normalizePlatform(value: string | undefined): CoreDownloadPlatform {
  const text = (value || '').trim().toLowerCase()
  // 必须先判 darwin/mac,再判 win:Wails Environment() 在 macOS 返回 GOOS="darwin",
  // 而 "darwin" 恰好包含子串 "win",若先判 win 会把 macOS 误判成 Windows,
  // 导致下载到 Windows 内核包(实测的错误根因)。
  if (text.includes('darwin') || text.includes('mac')) return 'darwin'
  if (text.includes('win')) return 'windows'
  if (text.includes('linux')) return 'linux'
  return ''
}

function normalizeArch(value: string | undefined): CoreDownloadArch {
  const text = (value || '').trim().toLowerCase()
  if (text === 'amd64' || text === 'x64' || text === 'x86_64') return 'amd64'
  if (text === 'arm64' || text === 'aarch64') return 'arm64'
  if (hasAny(text, ['arm64', 'aarch64'])) return 'arm64'
  if (hasAny(text, ['amd64', 'x64', 'x86_64', 'win64', 'wow64'])) return 'amd64'
  return ''
}

function inferNavigatorEnvironment(): CoreDownloadRuntimeEnvironment {
  if (typeof navigator === 'undefined') return {}
  const nav = navigator as Navigator & { userAgentData?: { platform?: string; architecture?: string } }
  const platform = nav.userAgentData?.platform || navigator.platform || ''
  const userAgent = navigator.userAgent || ''
  const archSource = `${nav.userAgentData?.architecture || ''} ${navigator.platform || ''} ${userAgent}`
  return { platform, arch: archSource }
}

export function resolveCoreDownloadTarget(env: CoreDownloadRuntimeEnvironment | null | undefined): CoreDownloadTarget {
  const fallback = inferNavigatorEnvironment()
  const platform = normalizePlatform(env?.platform || fallback.platform)
  const arch = normalizeArch(env?.arch || fallback.arch)
  return { platform, arch, label: formatTargetLabel(platform, arch) }
}

function formatTargetLabel(platform: CoreDownloadPlatform, arch: CoreDownloadArch): string {
  const platformLabel = platform === 'windows' ? 'Windows' : platform === 'linux' ? 'Linux' : platform === 'darwin' ? 'macOS' : '未知系统'
  const archLabel = arch || '未知架构'
  return `${platformLabel} / ${archLabel}`
}

function hasSupportedArchiveSuffix(name: string): boolean {
  const lowerName = name.toLowerCase()
  return archiveSuffixes.some(suffix => lowerName.endsWith(suffix))
}

function hasAny(text: string, keywords: string[]): boolean {
  return keywords.some(keyword => text.includes(keyword))
}

function matchesArch(name: string, arch: CoreDownloadArch): boolean {
  if (arch === 'amd64') return hasAny(name, ['amd64', 'x64', 'x86_64'])
  if (arch === 'arm64') return hasAny(name, ['arm64', 'aarch64'])
  return false
}

function scoreAsset(asset: GitHubReleaseAsset, target: CoreDownloadTarget): number {
  const name = (asset.name || '').toLowerCase()
  if (!name || !asset.browser_download_url || !hasSupportedArchiveSuffix(name)) return -1
  if (!target.platform || !target.arch) return -1

  let score = 0
  if (target.platform === 'windows') {
    if (!name.includes('windows') || !matchesArch(name, target.arch)) return -1
    score += 100
    if (name.endsWith('.zip')) score += 20
  } else if (target.platform === 'linux') {
    if (!name.includes('linux') || !matchesArch(name, target.arch)) return -1
    score += 100
    if (name.endsWith('.tar.xz')) score += 20
  } else if (target.platform === 'darwin') {
    if (!hasAny(name, ['macos', 'darwin'])) return -1
    if (hasAny(name, ['amd64', 'x64', 'x86_64', 'arm64', 'aarch64']) && !matchesArch(name, target.arch)) return -1
    score += 100
    // macOS 发行版通常只提供 .dmg;后端已支持挂载解包,优先选它。
    if (name.endsWith('.dmg')) score += 20
  }

  return score
}

// 导出以便单元验证:给定资产列表与目标平台/架构,选出最合适的下载资产。
export function pickReleaseAsset(assets: GitHubReleaseAsset[], target: CoreDownloadTarget): GitHubReleaseAsset | null {
  return assets
    .map(asset => ({ asset, score: scoreAsset(asset, target) }))
    .filter(item => item.score >= 0)
    .sort((left, right) => right.score - left.score || (left.asset.name || '').localeCompare(right.asset.name || ''))[0]?.asset || null
}

export async function fetchCoreDownloadRecommendation(
  env: CoreDownloadRuntimeEnvironment | null | undefined,
  signal?: AbortSignal,
): Promise<CoreDownloadRecommendation | null> {
  const target = resolveCoreDownloadTarget(env)
  const response = await fetch(FINGERPRINT_CHROMIUM_LATEST_RELEASE_API_URL, {
    headers: { Accept: 'application/vnd.github+json' },
    signal,
  })
  if (!response.ok) {
    throw new Error(`GitHub Releases 请求失败: ${response.status}`)
  }
  const release = await response.json() as GitHubReleaseResponse
  const asset = pickReleaseAsset(release.assets || [], target)
  if (!asset?.name || !asset.browser_download_url) return null

  const releaseTag = (release.tag_name || '').trim()
  const majorVersion = releaseTag.split('.')[0] || ''
  return {
    target,
    releaseTag,
    assetName: asset.name,
    downloadUrl: asset.browser_download_url,
    releasesUrl: release.html_url || FINGERPRINT_CHROMIUM_RELEASES_URL,
    namePlaceholder: majorVersion ? `例如: chrome-${majorVersion}` : '例如: chrome-latest',
  }
}
