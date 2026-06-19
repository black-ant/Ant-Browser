import { ReactNode, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ExternalLink, ListChecks } from 'lucide-react'
import clsx from 'clsx'
import { Alert, Badge, Button, Drawer, Loading, toast } from '../../../shared/components'
import { fetchShopProfile, sourceLabel } from '../api'
import {
  asmBadge,
  authorizationBadge,
  buildShopProfileDetailGroups,
  formatProfileTime,
  platformLabel,
  shopProfileAction,
} from '../profilePresentation'
import type { ShopProfile } from '../types'

function isEmptyValue(value: ReactNode) {
  return value === null || value === undefined || value === ''
}

function DetailRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid gap-1 border-b border-[var(--color-border-muted)] py-3 last:border-0 sm:grid-cols-[132px_minmax(0,1fr)] sm:gap-4">
      <span className="text-sm text-[var(--color-text-muted)]">{label}</span>
      <span className="min-w-0 break-all text-sm text-[var(--color-text-primary)] sm:text-right">
        {isEmptyValue(value) ? '-' : value}
      </span>
    </div>
  )
}

function SummaryItem({ label, value, title }: { label: string; value: ReactNode; title?: string }) {
  return (
    <div className="min-w-0 border-b border-[var(--color-border-muted)] py-2 last:border-0">
      <div className="text-xs text-[var(--color-text-muted)]">{label}</div>
      <div className="mt-1 min-w-0 truncate text-sm font-medium text-[var(--color-text-primary)]" title={title || (typeof value === 'string' ? value : undefined)}>
        {isEmptyValue(value) ? '-' : value}
      </div>
    </div>
  )
}

function SummaryBlock({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-4 py-3">
      <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">{title}</h3>
      <div className="mt-2">{children}</div>
    </section>
  )
}

function completeCategory(profile: ShopProfile) {
  return profile.categoryNames.length > 0 ? profile.categoryNames.join('、') : profile.mainCategory
}

interface ShopProfileDetailDrawerProps {
  shopId: string
  open: boolean
  onClose: () => void
}

export function ShopProfileDetailDrawer({ shopId, open, onClose }: ShopProfileDetailDrawerProps) {
  const [profile, setProfile] = useState<ShopProfile | null>(null)
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let cancelled = false
    const normalizedShopId = shopId.trim()

    if (!open || !normalizedShopId) {
      setProfile(null)
      setErrorMessage('')
      setLoading(false)
      return () => {
        cancelled = true
      }
    }

    async function load() {
      setLoading(true)
      try {
        setErrorMessage('')
        const next = await fetchShopProfile(normalizedShopId)
        if (!cancelled) setProfile(next)
      } catch (error) {
        console.error('load shop profile failed', error)
        const message = error instanceof Error ? error.message : '加载店铺详情失败'
        toast.error(message)
        if (!cancelled) {
          setProfile(null)
          setErrorMessage(message)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [open, shopId])

  const title = profile?.shopName || profile?.shopId || shopId
  const action = profile ? shopProfileAction(profile) : null
  const workbenchUrl = profile ? `/workbench?shopId=${encodeURIComponent(profile.shopId)}` : '/workbench'
  const operationsUrl = profile ? `/operations?shopId=${encodeURIComponent(profile.shopId)}` : '/operations'

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={title}
      subtitle="ASM 店铺主数据 / 客户端授权详情"
    >
      {loading ? (
        <div className="py-12">
          <Loading text="加载店铺授权详情..." />
        </div>
      ) : errorMessage ? (
        <Alert type="warning" title="店铺授权未连接真实客户端链路" message={errorMessage} />
      ) : profile ? (
        <div className="space-y-5">
          <section className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-4">
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="min-w-0 break-words text-lg font-semibold text-[var(--color-text-primary)]">
                    {profile.shopName || profile.shopId}
                  </h2>
                  <Badge variant="default">{profile.platformName || platformLabel(profile.platformCode)}</Badge>
                </div>
                <p className="mt-2 break-words text-sm text-[var(--color-text-secondary)]">
                  {profile.fullShopName || profile.shopAlias || profile.shopId}
                </p>
                <div className="mt-3 grid gap-2 text-sm text-[var(--color-text-muted)] sm:grid-cols-2">
                  <span className="min-w-0 truncate" title={profile.shopCode || '-'}>
                    店铺编码：{profile.shopCode || '-'}
                  </span>
                  <span className="min-w-0 truncate" title={profile.shopId}>
                    Shop ID：{profile.shopId}
                  </span>
                  <span className="min-w-0 truncate" title={completeCategory(profile) || '-'}>
                    类目：{completeCategory(profile) || '-'}
                  </span>
                  <span className="min-w-0 truncate" title={profile.asmShopId || '-'}>
                    ASM ID：{profile.asmShopId || '-'}
                  </span>
                </div>
              </div>

              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] p-3">
                <div className="flex flex-wrap gap-2">
                  {asmBadge(profile.asmStatus)}
                  {authorizationBadge(profile)}
                  <Badge variant="default">{sourceLabel(profile.source)}</Badge>
                </div>
                <p className="mt-3 text-sm text-[var(--color-text-secondary)]">
                  {action?.description || '进入店铺工作台处理本地授权和打开后台。'}
                </p>
                <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-1">
                  <Link to={workbenchUrl} title={action?.description}>
                    <Button size="sm" className="w-full">
                      <ExternalLink className="h-4 w-4" />
                      进入工作台
                    </Button>
                  </Link>
                  <Link to={operationsUrl} title="店铺级运营任务入口，完整任务系统后续接入">
                    <Button variant="secondary" size="sm" className="w-full">
                      <ListChecks className="h-4 w-4" />
                      运营任务
                    </Button>
                  </Link>
                </div>
              </div>
            </div>
          </section>

          <div className="grid gap-3 lg:grid-cols-3">
            <SummaryBlock title="经营归属">
              <SummaryItem label="运营" value={profile.operatorName || profile.operatorUsername} />
              <SummaryItem label="业务经理" value={profile.businessManagerName || profile.businessManagerUsername} />
              <SummaryItem label="部门 / 分公司" value={[profile.department, profile.subCompanyName].filter(Boolean).join(' / ')} />
            </SummaryBlock>
            <SummaryBlock title="工作台摘要">
              <SummaryItem label="推荐动作" value={action?.label || '进入工作台'} />
              <SummaryItem label="授权状态" value={profile.authorizationStatusLabel || profile.authorizationStatus || '未配置'} />
              <SummaryItem label="处理入口" value="工作台查看最近打开、验证和失败证据" title="工作台查看最近打开、验证和失败证据" />
            </SummaryBlock>
            <SummaryBlock title="最近同步">
              <SummaryItem label="同步时间" value={formatProfileTime(profile.lastSyncedAt)} />
              <SummaryItem label="ASM 更新时间" value={formatProfileTime(profile.sourceUpdatedAt)} />
              <SummaryItem label="资料完整度" value={profile.dataCompleteness === 'complete' ? '完整' : profile.dataCompleteness === 'partial' ? '部分完整' : '未知'} />
            </SummaryBlock>
          </div>

          {buildShopProfileDetailGroups(profile).map((group) => (
            <section
              key={group.title}
              className={clsx(
                'rounded-lg border border-[var(--color-border-default)] px-4 py-3',
                group.tone === 'muted' ? 'bg-[var(--color-bg-subtle)]' : 'bg-[var(--color-bg-surface)]',
              )}
            >
              <div className="mb-1">
                <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">{group.title}</h3>
                <p className="mt-1 text-xs text-[var(--color-text-muted)]">{group.subtitle}</p>
              </div>
              {group.fields.map((field) => (
                <DetailRow key={field.label} label={field.label} value={field.value} />
              ))}
            </section>
          ))}
        </div>
      ) : (
        <p className="text-sm text-[var(--color-text-muted)]">店铺授权详情不存在</p>
      )}
    </Drawer>
  )
}

export function ShopProfileDetailPage() {
  const { shopId = '' } = useParams()
  const navigate = useNavigate()
  return (
    <ShopProfileDetailDrawer
      shopId={shopId}
      open={Boolean(shopId)}
      onClose={() => navigate('/shops')}
    />
  )
}
