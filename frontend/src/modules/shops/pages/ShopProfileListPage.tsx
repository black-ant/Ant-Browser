import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { Alert, Button, Card, DataTable, toast } from '../../../shared/components'
import { fetchShopProfiles } from '../api'
import { buildShopProfileColumns } from '../profilePresentation'
import type { ShopProfile } from '../types'
import { ShopProfileDetailDrawer } from './ShopProfileDetailPage'

export function ShopProfileListPage() {
  const navigate = useNavigate()
  const { shopId = '' } = useParams()
  const [profiles, setProfiles] = useState<ShopProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  const columns = useMemo(() => buildShopProfileColumns(), [])
  const asmSnapshotMissing = !loading && !errorMessage && profiles.length === 0
  const selectedShopId = decodeURIComponent(shopId || '').trim()

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      setErrorMessage('')
      setProfiles(await fetchShopProfiles())
    } catch (error) {
      console.error('load shop profiles failed', error)
      const message = error instanceof Error ? error.message : String(error || '加载店铺授权失败')
      setProfiles([])
      setErrorMessage(message)
      toast.error(message)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const openProfile = (profile: ShopProfile) => {
    navigate(`/shops/${encodeURIComponent(profile.shopId)}`)
  }

  const closeDrawer = () => {
    navigate('/shops')
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-hidden animate-fade-in">
      <div className="flex shrink-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold text-[var(--color-text-primary)]">店铺授权</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">
            查看 ASM 店铺主数据、本地授权状态和下一步可执行动作。
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void load(true)}
          loading={refreshing}
          className="w-full shrink-0 sm:w-auto"
        >
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      <Card padding="none" className="flex min-h-0 flex-1 flex-col" bodyClassName="flex min-h-0 flex-1 flex-col">
        {errorMessage ? (
          <div className="border-b border-[var(--color-border-muted)] p-4">
            <Alert
              type="warning"
              title="店铺授权未连接真实客户端链路"
              message={errorMessage}
            />
          </div>
        ) : asmSnapshotMissing ? (
          <div className="border-b border-[var(--color-border-muted)] p-4">
            <Alert
              type="warning"
              title="还没有可展示的店铺授权数据"
              message="当前客户端已连接真实 workspace，但服务端没有可用的 ASM 店铺快照。请先在管理端执行 ASM 店铺同步，完成后再刷新本页。"
            />
          </div>
        ) : null}
        <DataTable
          columns={columns}
          data={profiles}
          rowKey="shopId"
          loading={loading}
          emptyText={errorMessage ? '店铺授权未连接' : asmSnapshotMissing ? 'ASM 店铺授权尚未同步' : '暂无 ASM 店铺授权数据'}
          fillHeight
          selectable
          storageKey="client-shop-profile-table-columns"
          onRowClick={openProfile}
        />
      </Card>

      <ShopProfileDetailDrawer
        open={Boolean(selectedShopId)}
        shopId={selectedShopId}
        onClose={closeDrawer}
      />
    </div>
  )
}
