import { WorkspaceShopProfile, WorkspaceShopProfiles } from '../../wailsjs/go/main/App'
import { devShopProfiles, useDevWorkspaceFallback } from '../workspace/devData'
import type { ShopProfile, ShopProfileStats } from './types'

export type AsmStatusKind = 'connected' | 'error' | 'unavailable'
const CLIENT_BACKEND_UNAVAILABLE = '未连接 Maka Browser 客户端后端，请在客户端本体中打开店铺授权页。'

function hasWorkspaceShopProfileBinding(): boolean {
  return Boolean((window as any)?.go?.main?.App?.WorkspaceShopProfiles)
}

function hasWorkspaceShopProfileDetailBinding(): boolean {
  return Boolean((window as any)?.go?.main?.App?.WorkspaceShopProfile)
}

export function asmStatusKind(status: string): AsmStatusKind {
  const normalizedStatus = status.trim()
  if (normalizedStatus === 'connected') return 'connected'
  if (normalizedStatus === 'error') return 'error'
  return 'unavailable'
}

export function asmStatusLabel(status: string): string {
  const kind = asmStatusKind(status)
  if (kind === 'connected') return 'ASM 已接入'
  if (kind === 'error') return 'ASM 异常'
  return 'ASM 待接入'
}

export function dataCompletenessLabel(status: string): string {
  if (status === 'complete') return '完整'
  if (status === 'partial') return '待完善'
  return '未知'
}

export function sourceLabel(source: string): string {
  if (source === 'asm') return 'ASM 店铺主数据'
  if (source === 'dev_mock') return '开发模拟资料'
  if (source === 'authorized_shop_projection') return '授权店铺投影'
  if (source) return source
  return '-'
}

function normalizeStringArray(input: unknown): string[] {
  return Array.isArray(input) ? input.map((item) => String(item || '').trim()).filter(Boolean) : []
}

function normalizeNumber(input: unknown): number {
  const value = Number(input)
  return Number.isFinite(value) ? value : 0
}

export function normalizeShopProfile(input: any): ShopProfile {
  return {
    shopId: String(input?.shopId || ''),
    shopName: String(input?.shopName || ''),
    asmShopId: String(input?.asmShopId || ''),
    shopCode: String(input?.shopCode || ''),
    shopAlias: String(input?.shopAlias || ''),
    fullShopName: String(input?.fullShopName || ''),
    platformCode: String(input?.platformCode || ''),
    platformName: String(input?.platformName || ''),
    platformSubtype: String(input?.platformSubtype || ''),
    shopStatusCode: normalizeNumber(input?.shopStatusCode),
    shopStatus: String(input?.shopStatus || ''),
    asmStatus: String(input?.asmStatus || 'unavailable'),
    authorizationStatus: String(input?.authorizationStatus || ''),
    authorizationStatusLabel: String(input?.authorizationStatusLabel || input?.authorizationStatus || ''),
    ownerName: String(input?.ownerName || ''),
    operatorName: String(input?.operatorName || ''),
    operatorUsername: String(input?.operatorUsername || ''),
    businessManagerName: String(input?.businessManagerName || ''),
    businessManagerUsername: String(input?.businessManagerUsername || ''),
    department: String(input?.department || ''),
    subCompanyName: String(input?.subCompanyName || ''),
    shopUrl: String(input?.shopUrl || ''),
    shopEmail: String(input?.shopEmail || ''),
    shopPhone: String(input?.shopPhone || ''),
    legalRepName: String(input?.legalRepName || ''),
    businessLicense: String(input?.businessLicense || ''),
    unifiedSocialCode: String(input?.unifiedSocialCode || ''),
    registeredAddress: String(input?.registeredAddress || ''),
    categoryIds: normalizeStringArray(input?.categoryIds),
    categoryNames: normalizeStringArray(input?.categoryNames),
    brandName: String(input?.brandName || ''),
    brandIds: normalizeStringArray(input?.brandIds),
    advancedMember: normalizeNumber(input?.advancedMember),
    advancedMemberName: String(input?.advancedMemberName || ''),
    trustPassExpireAt: String(input?.trustPassExpireAt || ''),
    jstShopCount: normalizeNumber(input?.jstShopCount),
    jstShopSummary: String(input?.jstShopSummary || ''),
    mabangShopCount: normalizeNumber(input?.mabangShopCount),
    mabangShopSummary: String(input?.mabangShopSummary || ''),
    erpShopCount: normalizeNumber(input?.erpShopCount),
    erpShopSummary: String(input?.erpShopSummary || ''),
    abnormalCount: normalizeNumber(input?.abnormalCount),
    abnormalSummary: String(input?.abnormalSummary || ''),
    tableSource: String(input?.tableSource || ''),
    isPush: normalizeNumber(input?.isPush),
    mainCategory: String(input?.mainCategory || ''),
    dataCompleteness: String(input?.dataCompleteness || 'unknown'),
    sourceCreatedAt: String(input?.sourceCreatedAt || ''),
    sourceUpdatedAt: String(input?.sourceUpdatedAt || ''),
    lastSyncedAt: String(input?.lastSyncedAt || ''),
    source: String(input?.source || ''),
  }
}

export async function fetchShopProfiles(): Promise<ShopProfile[]> {
  if (useDevWorkspaceFallback()) return devShopProfiles
  if (!hasWorkspaceShopProfileBinding()) {
    throw new Error(CLIENT_BACKEND_UNAVAILABLE)
  }
  const payload = await WorkspaceShopProfiles()
  return Array.isArray(payload) ? payload.map(normalizeShopProfile) : []
}

export async function fetchShopProfile(shopId: string): Promise<ShopProfile> {
  const normalizedShopId = shopId.trim()
  if (!normalizedShopId) {
    throw new Error('shop id is required')
  }

  if (useDevWorkspaceFallback()) {
    const profile = devShopProfiles.find((item) => item.shopId === normalizedShopId)
    if (!profile) throw new Error('shop profile not found')
    return profile
  }

  if (!hasWorkspaceShopProfileDetailBinding()) {
    throw new Error(CLIENT_BACKEND_UNAVAILABLE)
  }
  const payload = await WorkspaceShopProfile(normalizedShopId)
  return normalizeShopProfile(payload)
}

export function deriveShopProfileStats(profiles: ShopProfile[]): ShopProfileStats {
  return {
    total: profiles.length,
    asmConnected: profiles.filter((profile) => asmStatusKind(profile.asmStatus) === 'connected').length,
    unavailable: profiles.filter((profile) => asmStatusKind(profile.asmStatus) === 'unavailable').length,
    incomplete: profiles.filter((profile) => profile.dataCompleteness !== 'complete').length,
  }
}
