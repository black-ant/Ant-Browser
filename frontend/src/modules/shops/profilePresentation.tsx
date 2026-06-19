import { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import { Badge } from '../../shared/components'
import type { DataTableColumn } from '../../shared/components'
import { authorizationStatusPresentation } from '../workbench/statusMatrix'
import { asmStatusKind, asmStatusLabel, dataCompletenessLabel, sourceLabel } from './api'
import type { ShopProfile } from './types'

export function platformLabel(platformCode: string) {
  if (!platformCode) return '-'
  if (platformCode === 'alibaba') return '1688 / Alibaba'
  return platformCode
}

export function formatProfileTime(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function asmBadge(status: string) {
  const kind = asmStatusKind(status)
  if (kind === 'connected') return <Badge variant="success">{asmStatusLabel(status)}</Badge>
  if (kind === 'error') return <Badge variant="error">{asmStatusLabel(status)}</Badge>
  return <Badge variant="warning">{asmStatusLabel(status)}</Badge>
}

export function dataCompletenessBadge(status: string) {
  if (status === 'complete') return <Badge variant="success">{dataCompletenessLabel(status)}</Badge>
  if (status === 'partial') return <Badge variant="warning">{dataCompletenessLabel(status)}</Badge>
  return <Badge variant="default">{dataCompletenessLabel(status)}</Badge>
}

export function authorizationBadge(profile: ShopProfile) {
  const label = profile.authorizationStatusLabel || profile.authorizationStatus || '未配置'
  const presentation = authorizationStatusPresentation(profile.authorizationStatus, label)
  if (presentation.queue === 'ready') {
    return <Badge variant="success">{label}</Badge>
  }
  if (presentation.status === 'validation_failed' || presentation.status === 'disabled') {
    return <Badge variant="error">{label}</Badge>
  }
  if (presentation.queue === 'manual' || presentation.queue === 'credential') {
    return <Badge variant="warning">{label}</Badge>
  }
  return <Badge variant="default">{label}</Badge>
}

export function shopProfileAction(profile: ShopProfile) {
  const presentation = authorizationStatusPresentation(profile.authorizationStatus, profile.authorizationStatusLabel)
  if (presentation.recommendedAction === 'open') {
    return {
      label: '打开后台',
      description: presentation.description,
    }
  }
  if (presentation.recommendedAction === 'bind' || presentation.recommendedAction === 'validate') {
    return {
      label: '更新凭据',
      description: presentation.description,
    }
  }
  return {
    label: presentation.primaryLabel,
    description: presentation.description,
  }
}

export function shopProfileTaskPath(profile: ShopProfile) {
  return `/operations?shopId=${encodeURIComponent(profile.shopId)}`
}

function compactText(value: string, width = 180) {
  return (
    <span className="block truncate text-[var(--color-text-secondary)]" style={{ maxWidth: width }} title={value || '-'}>
      {value || '-'}
    </span>
  )
}

function compactNumber(value: number) {
  return <span className="tabular-nums text-[var(--color-text-secondary)]">{Number.isFinite(value) ? value : 0}</span>
}

function yesNo(value: number) {
  if (value === 1) return '是'
  if (value === 0) return '否'
  return '-'
}

function joinValues(values: string[]) {
  return values.length > 0 ? values.join('、') : ''
}

const COLUMN_GROUPS = {
  shop: '店铺基础',
  platform: '平台与状态',
  ownership: '经营归属',
  brand: '品牌与类目',
  contact: '联系与主体',
  external: '外部系统',
  sync: '同步与异常',
  action: '固定列',
} as const

export function buildShopProfileColumns(): DataTableColumn<ShopProfile>[] {
  return [
    {
      key: 'shopName',
      title: '店铺',
      group: COLUMN_GROUPS.shop,
      width: 280,
      minWidth: 240,
      fixed: 'left',
      sortable: true,
      filterable: true,
      sortValue: (profile) => profile.shopName || profile.shopId,
      filterValue: (profile) => `${profile.shopName} ${profile.shopId} ${profile.fullShopName} ${profile.shopCode}`,
      render: (_, profile) => (
        <div className="flex min-w-0 flex-col gap-1">
          <Link
            className="max-w-[230px] truncate text-sm font-medium text-[var(--color-accent)] hover:underline"
            title={profile.shopName || profile.shopId}
            to={`/shops/${encodeURIComponent(profile.shopId)}`}
            onClick={(event) => event.stopPropagation()}
          >
            {profile.shopName || profile.shopId}
          </Link>
          <span className="max-w-[230px] truncate text-xs text-[var(--color-text-muted)]" title={profile.fullShopName || profile.shopId}>
            {profile.fullShopName || profile.shopId}
          </span>
        </div>
      ),
    },
    {
      key: 'shopCode',
      title: '店铺编码',
      group: COLUMN_GROUPS.shop,
      width: 170,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 150),
    },
    {
      key: 'shopAlias',
      title: '店铺别名',
      group: COLUMN_GROUPS.shop,
      width: 180,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 160),
    },
    {
      key: 'fullShopName',
      title: '完整店铺名',
      group: COLUMN_GROUPS.shop,
      width: 280,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 260),
    },
    {
      key: 'asmShopId',
      title: 'ASM ID',
      group: COLUMN_GROUPS.shop,
      width: 130,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 110),
    },
    {
      key: 'platformCode',
      title: '平台',
      group: COLUMN_GROUPS.platform,
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => `${profile.platformCode} ${profile.platformName} ${profile.platformSubtype}`,
      render: (_, profile) => <Badge variant="default">{profile.platformName || platformLabel(profile.platformCode)}</Badge>,
    },
    {
      key: 'platformSubtype',
      title: '平台子类型',
      group: COLUMN_GROUPS.platform,
      width: 140,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 120),
    },
    {
      key: 'shopStatus',
      title: '店铺状态',
      group: COLUMN_GROUPS.platform,
      width: 130,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 110),
    },
    {
      key: 'shopStatusCode',
      title: '状态码',
      group: COLUMN_GROUPS.platform,
      width: 110,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactNumber(Number(value)),
    },
    {
      key: 'asmStatus',
      title: 'ASM 状态',
      group: COLUMN_GROUPS.platform,
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => asmStatusLabel(profile.asmStatus),
      render: (value) => asmBadge(String(value || '')),
    },
    {
      key: 'authorizationStatus',
      title: '授权状态',
      group: COLUMN_GROUPS.platform,
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => profile.authorizationStatusLabel || profile.authorizationStatus,
      render: (_, profile) => authorizationBadge(profile),
    },
    {
      key: 'operatorName',
      title: '运营',
      group: COLUMN_GROUPS.ownership,
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => `${profile.operatorName} ${profile.operatorUsername}`,
      render: (_, profile) => compactText(profile.operatorName || profile.operatorUsername, 120),
    },
    {
      key: 'businessManagerName',
      title: '业务经理',
      group: COLUMN_GROUPS.ownership,
      width: 150,
      sortable: true,
      filterable: true,
      filterValue: (profile) => `${profile.businessManagerName} ${profile.businessManagerUsername}`,
      render: (_, profile) => compactText(profile.businessManagerName || profile.businessManagerUsername, 130),
    },
    {
      key: 'operatorUsername',
      title: '运营账号',
      group: COLUMN_GROUPS.ownership,
      width: 150,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'businessManagerUsername',
      title: '业务账号',
      group: COLUMN_GROUPS.ownership,
      width: 150,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'department',
      title: '部门',
      group: COLUMN_GROUPS.ownership,
      width: 150,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'subCompanyName',
      title: '分公司',
      group: COLUMN_GROUPS.ownership,
      width: 170,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 150),
    },
    {
      key: 'brandName',
      title: '品牌',
      group: COLUMN_GROUPS.brand,
      width: 150,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'brandIds',
      title: '品牌 ID',
      group: COLUMN_GROUPS.brand,
      width: 180,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => joinValues(profile.brandIds),
      render: (_, profile) => compactText(joinValues(profile.brandIds), 160),
    },
    {
      key: 'advancedMemberName',
      title: '高级会员',
      group: COLUMN_GROUPS.brand,
      width: 150,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'mainCategory',
      title: '主营类目',
      group: COLUMN_GROUPS.brand,
      width: 180,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 160),
    },
    {
      key: 'categoryNames',
      title: '全部类目',
      group: COLUMN_GROUPS.brand,
      width: 220,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => joinValues(profile.categoryNames),
      render: (_, profile) => compactText(joinValues(profile.categoryNames), 200),
    },
    {
      key: 'categoryIds',
      title: '类目 ID',
      group: COLUMN_GROUPS.brand,
      width: 180,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => joinValues(profile.categoryIds),
      render: (_, profile) => compactText(joinValues(profile.categoryIds), 160),
    },
    {
      key: 'shopUrl',
      title: '店铺地址',
      group: COLUMN_GROUPS.contact,
      width: 220,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 200),
    },
    {
      key: 'shopEmail',
      title: '邮箱',
      group: COLUMN_GROUPS.contact,
      width: 180,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 160),
    },
    {
      key: 'shopPhone',
      title: '电话',
      group: COLUMN_GROUPS.contact,
      width: 150,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'legalRepName',
      title: '法人',
      group: COLUMN_GROUPS.contact,
      width: 140,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 120),
    },
    {
      key: 'businessLicense',
      title: '营业执照',
      group: COLUMN_GROUPS.contact,
      width: 190,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 170),
    },
    {
      key: 'unifiedSocialCode',
      title: '统一社会信用代码',
      group: COLUMN_GROUPS.contact,
      width: 210,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 190),
    },
    {
      key: 'registeredAddress',
      title: '注册地址',
      group: COLUMN_GROUPS.contact,
      width: 240,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 220),
    },
    {
      key: 'trustPassExpireAt',
      title: '诚信通到期',
      group: COLUMN_GROUPS.contact,
      width: 170,
      sortable: true,
      defaultVisible: false,
      sortValue: (profile) => Date.parse(profile.trustPassExpireAt || '') || 0,
      render: (value) => compactText(formatProfileTime(String(value || '')), 150),
    },
    {
      key: 'jstShopCount',
      title: '聚水潭店铺',
      group: COLUMN_GROUPS.external,
      width: 130,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => `${profile.jstShopCount} ${profile.jstShopSummary}`,
      render: (_, profile) => compactText(profile.jstShopSummary || String(profile.jstShopCount || 0), 110),
    },
    {
      key: 'mabangShopCount',
      title: '马帮店铺',
      group: COLUMN_GROUPS.external,
      width: 130,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => `${profile.mabangShopCount} ${profile.mabangShopSummary}`,
      render: (_, profile) => compactText(profile.mabangShopSummary || String(profile.mabangShopCount || 0), 110),
    },
    {
      key: 'erpShopCount',
      title: 'ERP 店铺',
      group: COLUMN_GROUPS.external,
      width: 130,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => `${profile.erpShopCount} ${profile.erpShopSummary}`,
      render: (_, profile) => compactText(profile.erpShopSummary || String(profile.erpShopCount || 0), 110),
    },
    {
      key: 'abnormalSummary',
      title: '异常信息',
      group: COLUMN_GROUPS.sync,
      width: 180,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      filterValue: (profile) => `${profile.abnormalCount} ${profile.abnormalSummary}`,
      render: (_, profile) => compactText(profile.abnormalSummary || (profile.abnormalCount ? `${profile.abnormalCount} 条` : ''), 160),
    },
    {
      key: 'tableSource',
      title: '表来源',
      group: COLUMN_GROUPS.sync,
      width: 140,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(String(value || ''), 120),
    },
    {
      key: 'isPush',
      title: '是否推送',
      group: COLUMN_GROUPS.sync,
      width: 120,
      sortable: true,
      filterable: true,
      defaultVisible: false,
      render: (value) => compactText(yesNo(Number(value)), 100),
    },
    {
      key: 'sourceCreatedAt',
      title: 'ASM 创建时间',
      group: COLUMN_GROUPS.sync,
      width: 180,
      sortable: true,
      defaultVisible: false,
      sortValue: (profile) => Date.parse(profile.sourceCreatedAt || '') || 0,
      render: (value) => compactText(formatProfileTime(String(value || '')), 160),
    },
    {
      key: 'sourceUpdatedAt',
      title: 'ASM 更新时间',
      group: COLUMN_GROUPS.sync,
      width: 180,
      sortable: true,
      defaultVisible: false,
      sortValue: (profile) => Date.parse(profile.sourceUpdatedAt || '') || 0,
      render: (value) => compactText(formatProfileTime(String(value || '')), 160),
    },
    {
      key: 'lastSyncedAt',
      title: '最近同步',
      group: COLUMN_GROUPS.sync,
      width: 180,
      sortable: true,
      sortValue: (profile) => Date.parse(profile.lastSyncedAt || '') || 0,
      render: (value) => (
        <span className="block max-w-[160px] truncate text-xs text-[var(--color-text-muted)]" title={String(value || '-')}>
          {formatProfileTime(String(value || ''))}
        </span>
      ),
    },
    {
      key: 'actions',
      title: '操作',
      group: COLUMN_GROUPS.action,
      width: 210,
      minWidth: 190,
      fixed: 'right',
      align: 'right',
      hideable: false,
      resizable: false,
      render: (_, profile) => {
        const action = shopProfileAction(profile)
        return (
          <div className="client-data-table-actions" onClick={(event) => event.stopPropagation()}>
            <Link className="client-data-table-action-link" to={`/shops/${encodeURIComponent(profile.shopId)}`}>
              详情
            </Link>
            <Link className="client-data-table-action-link" to={`/workbench?shopId=${encodeURIComponent(profile.shopId)}`} title={action.description}>
              {action.label}
            </Link>
            <Link className="client-data-table-action-link" to={shopProfileTaskPath(profile)}>
              任务
            </Link>
          </div>
        )
      },
    },
  ]
}

export type DetailField = {
  label: string
  value: ReactNode
}

export type DetailGroup = {
  title: string
  subtitle: string
  tone?: 'primary' | 'muted'
  fields: DetailField[]
}

export function buildShopProfileDetailGroups(profile: ShopProfile): DetailGroup[] {
  return [
    {
      title: '概览',
      subtitle: '运营识别店铺时优先看的基础信息',
      fields: [
        { label: '店铺名称', value: profile.shopName },
        { label: '完整店铺名', value: profile.fullShopName },
        { label: '店铺编码', value: profile.shopCode },
        { label: 'Shop ID', value: profile.shopId },
        { label: 'ASM Shop ID', value: profile.asmShopId },
        { label: '店铺别名', value: profile.shopAlias },
        { label: '平台', value: profile.platformName || platformLabel(profile.platformCode) },
        { label: '平台子类型', value: profile.platformSubtype },
        { label: '主营类目', value: profile.mainCategory },
        { label: '全部类目', value: joinValues(profile.categoryNames) },
        { label: '类目 ID', value: joinValues(profile.categoryIds) },
      ],
    },
    {
      title: '经营归属',
      subtitle: '用于确认当前店铺归属和责任人',
      fields: [
        { label: '负责人', value: profile.ownerName },
        { label: '运营', value: profile.operatorName },
        { label: '运营账号', value: profile.operatorUsername },
        { label: '业务经理', value: profile.businessManagerName },
        { label: '业务经理账号', value: profile.businessManagerUsername },
        { label: '部门', value: profile.department },
        { label: '分公司', value: profile.subCompanyName },
      ],
    },
    {
      title: '授权与打开状态',
      subtitle: '区分 ASM 主资料状态和本地授权状态',
      fields: [
        { label: 'ASM 状态', value: asmBadge(profile.asmStatus) },
        { label: 'ASM 店铺状态', value: profile.shopStatus || profile.shopStatusCode },
        { label: '授权状态', value: authorizationBadge(profile) },
        { label: '资料完整度', value: dataCompletenessBadge(profile.dataCompleteness) },
        { label: '最近同步', value: formatProfileTime(profile.lastSyncedAt) },
        { label: '数据来源', value: sourceLabel(profile.source) },
      ],
    },
    {
      title: '品牌与联系',
      subtitle: '运营定位店铺和联系主体时使用',
      fields: [
        { label: '品牌', value: profile.brandName },
        { label: '品牌 ID', value: joinValues(profile.brandIds) },
        { label: '高级会员', value: profile.advancedMemberName },
        { label: '诚信通到期', value: formatProfileTime(profile.trustPassExpireAt) },
        {
          label: '店铺地址',
          value: profile.shopUrl ? (
            <a className="inline-flex items-center gap-1 text-[var(--color-accent)] hover:underline" href={profile.shopUrl} target="_blank" rel="noreferrer">
              {profile.shopUrl}
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          ) : '',
        },
        { label: '邮箱', value: profile.shopEmail },
        { label: '电话', value: profile.shopPhone },
      ],
    },
    {
      title: '主体资质',
      subtitle: '管理型资料，客户端弱化展示',
      tone: 'muted',
      fields: [
        { label: '法人', value: profile.legalRepName },
        { label: '营业执照', value: profile.businessLicense },
        { label: '统一社会信用代码', value: profile.unifiedSocialCode },
        { label: '注册地址', value: profile.registeredAddress },
      ],
    },
    {
      title: '外部系统',
      subtitle: 'ASM 返回的外部系统关联摘要',
      tone: 'muted',
      fields: [
        { label: '聚水潭店铺数', value: profile.jstShopCount },
        { label: '聚水潭店铺', value: profile.jstShopSummary },
        { label: '马帮店铺数', value: profile.mabangShopCount },
        { label: '马帮店铺', value: profile.mabangShopSummary },
        { label: 'ERP 店铺数', value: profile.erpShopCount },
        { label: 'ERP 店铺', value: profile.erpShopSummary },
      ],
    },
    {
      title: '同步与异常',
      subtitle: '资料治理和排障时使用',
      tone: 'muted',
      fields: [
        { label: '异常数量', value: profile.abnormalCount },
        { label: '异常摘要', value: profile.abnormalSummary },
        { label: '表来源', value: profile.tableSource },
        { label: '是否推送', value: yesNo(profile.isPush) },
        { label: 'ASM 创建时间', value: formatProfileTime(profile.sourceCreatedAt) },
        { label: 'ASM 更新时间', value: formatProfileTime(profile.sourceUpdatedAt) },
      ],
    },
  ]
}
