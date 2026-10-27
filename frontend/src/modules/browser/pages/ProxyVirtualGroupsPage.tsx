import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, FolderOpen, Pencil, Plus, Search, Trash2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, ConfirmModal, Input, Modal, Select, Switch, Table, toast } from '../../../shared/components'
import type { SortOrder, TableColumn } from '../../../shared/components/Table'
import type { BrowserProxy, BrowserProxyVirtualGroup } from '../types'
import {
  deleteBrowserProxyVirtualGroup,
  fetchBrowserProxies,
  fetchBrowserProxyVirtualGroups,
  saveBrowserProxyVirtualGroup,
} from '../api'
import { useProxyChecks } from './proxyPool/useProxyChecks'
import { ProxyVirtualGroupMembersModal } from './proxyPool/ProxyVirtualGroupMembersModal'
import { toDisplayList, type ProxyDisplayInfo } from './proxyPool/helpers'

interface VirtualGroupForm {
  groupId: string
  groupName: string
  strategy: 'round_robin' | 'sticky'
  selectedProxyId: string
  memberProxyIds: string[]
  memberGroupNames: string[]
  sortOrder: number
  createdAt: string
  updatedAt: string
}

const createEmptyForm = (): VirtualGroupForm => ({
  groupId: '',
  groupName: '',
  strategy: 'round_robin',
  selectedProxyId: '',
  memberProxyIds: [],
  memberGroupNames: [],
  sortOrder: 0,
  createdAt: '',
  updatedAt: '',
})

function isDirectProxy(proxy: BrowserProxy) {
  return proxy.proxyId.trim().toLowerCase() === '__direct__'
    || proxy.proxyConfig.trim().toLowerCase() === 'direct://'
}

const ungroupedProxyGroupValue = '__ungrouped__'
const otherRegionValue = '__other_region__'

const REGION_KEYWORD_CODES: Array<[string, string]> = [
  ['美国', 'US'], ['日本', 'JP'], ['香港', 'HK'], ['台湾', 'TW'], ['新加坡', 'SG'],
  ['韩国', 'KR'], ['英国', 'UK'], ['德国', 'DE'], ['法国', 'FR'], ['加拿大', 'CA'],
  ['澳大利亚', 'AU'], ['土耳其', 'TR'], ['印度', 'IN'], ['俄罗斯', 'RU'], ['荷兰', 'NL'],
  ['泰国', 'TH'], ['越南', 'VN'], ['马来西亚', 'MY'], ['菲律宾', 'PH'], ['巴西', 'BR'],
  ['阿根廷', 'AR'], ['意大利', 'IT'], ['西班牙', 'ES'], ['阿联酋', 'AE'], ['澳门', 'MO'],
]

const REGION_CODE_ALIASES: Record<string, string> = {
  USA: 'US', SGP: 'SG', KOR: 'KR', GB: 'UK',
}

function deriveProxyRegion(name: string) {
  const text = (name || '').trim()
  for (const [keyword, code] of REGION_KEYWORD_CODES) {
    if (text.includes(keyword)) return code
  }
  const tokenMatch = text.match(/(?:^|[\s\-_#*·])([A-Za-z]{2,3})(?=[\s\-_\d#*·]|$)/)
  if (tokenMatch) {
    const code = tokenMatch[1].toUpperCase()
    if (REGION_CODE_ALIASES[code]) return REGION_CODE_ALIASES[code]
    if (/^(US|JP|HK|TW|SG|KR|UK|DE|FR|CA|AU|TR|IN|RU|NL|TH|VN|MY|PH|BR|AR|IT|ES|AE|MO)$/.test(code)) return code
  }
  return ''
}

const virtualGroupStrategyOptions = [
  { value: 'round_robin', label: '绝对轮询' },
  { value: 'sticky', label: '代理保持' },
]

function getVirtualGroupStrategyLabel(strategy?: string) {
  return strategy === 'sticky' ? '代理保持' : '绝对轮询'
}

export function ProxyVirtualGroupsPage() {
  const navigate = useNavigate()
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [virtualGroups, setVirtualGroups] = useState<BrowserProxyVirtualGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [editorOpen, setEditorOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState<VirtualGroupForm>(createEmptyForm())
  const [deleteTarget, setDeleteTarget] = useState<BrowserProxyVirtualGroup | null>(null)
  const [membersModal, setMembersModal] = useState<{
    groupName: string
    strategy?: string
    members: ProxyDisplayInfo[]
  } | null>(null)
  const [proxyFilter, setProxyFilter] = useState('')
  const [proxyGroupFilter, setProxyGroupFilter] = useState('')
  const [proxyRegionFilter, setProxyRegionFilter] = useState('')
  const [showSelectedOnly, setShowSelectedOnly] = useState(false)
  const [showAvailableOnly, setShowAvailableOnly] = useState(false)
  const [proxySortColumn, setProxySortColumn] = useState<string>('latency')
  const [proxySortOrder, setProxySortOrder] = useState<SortOrder>('asc')
  const [editorStep, setEditorStep] = useState<'details' | 'nodes'>('details')

  const load = async () => {
    setLoading(true)
    try {
      const [poolProxies, configuredGroups] = await Promise.all([
        fetchBrowserProxies(),
        fetchBrowserProxyVirtualGroups(),
      ])
      setProxies(poolProxies)
      setVirtualGroups(configuredGroups)
    } catch (error: any) {
      toast.error(error?.message || '加载虚拟分组失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const proxyById = useMemo(() => {
    const result = new Map<string, BrowserProxy>()
    for (const proxy of proxies) {
      const proxyId = proxy.proxyId.trim()
      if (proxyId) result.set(proxyId.toLowerCase(), proxy)
    }
    return result
  }, [proxies])

  const displayProxyById = useMemo(() => (
    new Map(toDisplayList(proxies).map(proxy => [proxy.proxyId.trim().toLowerCase(), proxy] as const))
  ), [proxies])

  const availableProxies = useMemo(
    () => proxies.filter(proxy => proxy.proxyId.trim() && !isDirectProxy(proxy)),
    [proxies],
  )


  const proxyGroupOptions = useMemo(() => {
    const names = new Map<string, string>()
    let hasUngrouped = false
    availableProxies.forEach(proxy => {
      const groupName = (proxy.groupName || '').trim()
      if (!groupName) {
        hasUngrouped = true
        return
      }
      const key = groupName.toLowerCase()
      if (!names.has(key)) names.set(key, groupName)
    })

    const options = [
      { value: '', label: '全部实体分组' },
      ...Array.from(names.values())
        .sort((left, right) => left.localeCompare(right, 'zh-CN'))
        .map(label => ({ value: label.toLowerCase(), label })),
    ]
    if (hasUngrouped) options.push({ value: ungroupedProxyGroupValue, label: '未分组' })
    return options
  }, [availableProxies])

  const proxyRegionOptions = useMemo(() => {
    const counts = new Map<string, number>()
    let other = 0
    availableProxies.forEach(proxy => {
      const code = deriveProxyRegion(proxy.proxyName)
      if (code) counts.set(code, (counts.get(code) || 0) + 1)
      else other += 1
    })
    const options = [
      { value: '', label: '全部地区' },
      ...Array.from(counts.entries())
        .sort((left, right) => left[0].localeCompare(right[0]) || right[1] - left[1])
        .map(([code, count]) => ({ value: code.toLowerCase(), label: `${code}（${count}）` })),
    ]
    if (other > 0) options.push({ value: otherRegionValue, label: `其他（${other}）` })
    return options
  }, [availableProxies])

  const {
    latencyMap,
    latencyEngineMap,
    latencyErrorMap,
    testingAll,
    handleTestOne,
    handleTestAll,
  } = useProxyChecks({ proxies: availableProxies })

  const resolveMemberProxyIds = (group: BrowserProxyVirtualGroup) => {
    const memberProxyIds = [
      ...(group.memberProxyIds || []),
      ...proxies
        .filter(proxy => (group.memberGroupNames || []).some(name => (
          name.trim().toLowerCase() === (proxy.groupName || '').trim().toLowerCase()
        )))
        .map(proxy => proxy.proxyId),
    ]
    const canonicalProxyIds = new Map(
      availableProxies.map(proxy => [proxy.proxyId.trim().toLowerCase(), proxy.proxyId.trim()]),
    )
    return memberProxyIds
      .map(proxyId => proxyId.trim())
      .filter(proxyId => proxyId && canonicalProxyIds.has(proxyId.toLowerCase()))
      .map(proxyId => canonicalProxyIds.get(proxyId.toLowerCase()) || proxyId)
  }

  const openMembersModal = (group: BrowserProxyVirtualGroup) => {
    const members = resolveMemberProxyIds(group)
      .map(proxyId => displayProxyById.get(proxyId.trim().toLowerCase()))
      .filter((proxy): proxy is ProxyDisplayInfo => !!proxy)
    setMembersModal({
      groupName: group.groupName,
      strategy: group.strategy,
      members,
    })
  }

  const filteredAvailableProxies = useMemo(() => {
    const keyword = proxyFilter.trim().toLowerCase()
    const filtered = availableProxies.filter(proxy => {
      const groupName = (proxy.groupName || '').trim()
      const matchesGroup = !proxyGroupFilter
        || (proxyGroupFilter === ungroupedProxyGroupValue ? !groupName : groupName.toLowerCase() === proxyGroupFilter)
      const region = deriveProxyRegion(proxy.proxyName)
      const matchesRegion = !proxyRegionFilter
        || (proxyRegionFilter === otherRegionValue ? !region : region.toLowerCase() === proxyRegionFilter)
      const matchesKeyword = !keyword || [proxy.proxyName, proxy.proxyId, groupName]
        .some(value => value.toLowerCase().includes(keyword))
      const matchesSelection = !showSelectedOnly || form.memberProxyIds.some(proxyId => (
        proxyId.toLowerCase() === proxy.proxyId.toLowerCase()
      ))
      const latency = latencyMap[proxy.proxyId]
      const matchesAvailable = !showAvailableOnly || (typeof latency === 'number' && latency > 0)
      return matchesGroup && matchesRegion && matchesKeyword && matchesSelection && matchesAvailable
    })
    if (proxySortColumn !== 'latency' || !proxySortOrder) return filtered
    // 延迟排序语义：可用（正数）优先按数值升序，其后依次为测速中/超时/不支持/失败，未测速最后
    const tuple = (proxy: BrowserProxy): [number, number] => {
      const latency = latencyMap[proxy.proxyId]
      if (latency === undefined) return [5, Number.MAX_SAFE_INTEGER]
      if (latency === -1) return [1, Number.MAX_SAFE_INTEGER]
      if (latency === -2) return [2, Number.MAX_SAFE_INTEGER]
      if (latency === -3) return [3, Number.MAX_SAFE_INTEGER]
      if (latency === -4) return [4, Number.MAX_SAFE_INTEGER]
      return [0, latency]
    }
    return [...filtered].sort((a, b) => {
      const [ca, va] = tuple(a)
      const [cb, vb] = tuple(b)
      const cmp = ca !== cb ? ca - cb : va - vb
      return proxySortOrder === 'asc' ? cmp : -cmp
    })
  }, [availableProxies, form.memberProxyIds, proxyFilter, proxyGroupFilter, proxyRegionFilter, showSelectedOnly, showAvailableOnly, proxySortColumn, proxySortOrder, latencyMap])
  const filteredAvailableDisplayProxies = useMemo(
    () => filteredAvailableProxies
      .map(proxy => displayProxyById.get(proxy.proxyId.trim().toLowerCase()))
      .filter((proxy): proxy is ProxyDisplayInfo => !!proxy),
    [filteredAvailableProxies, displayProxyById],
  )

  const closeEditor = () => {
    setEditorOpen(false)
    setSaving(false)
    setProxyGroupFilter('')
    setProxyRegionFilter('')
    setShowSelectedOnly(false)
    setEditorStep('details')
  }

  const openCreate = () => {
    setForm(createEmptyForm())
    setProxyFilter('')
    setProxyGroupFilter('')
    setProxyRegionFilter('')
    setShowSelectedOnly(false)
    setEditorStep('details')
    setEditorOpen(true)
  }

  const openEdit = (group: BrowserProxyVirtualGroup) => {
    setForm({
      groupId: group.groupId,
      groupName: group.groupName,
      strategy: group.strategy === 'sticky' ? 'sticky' : 'round_robin',
      selectedProxyId: group.selectedProxyId || '',
      memberProxyIds: resolveMemberProxyIds(group),
      memberGroupNames: group.memberGroupNames || [],
      sortOrder: group.sortOrder || 0,
      createdAt: group.createdAt || '',
      updatedAt: group.updatedAt || '',
    })
    setProxyFilter('')
    setProxyGroupFilter('')
    setProxyRegionFilter('')
    setShowSelectedOnly(false)
    setEditorStep('details')
    setEditorOpen(true)
  }

  const toggleMemberProxy = (proxyId: string) => {
    const isSelected = form.memberProxyIds.some(id => id.toLowerCase() === proxyId.toLowerCase())
    if (isSelected && form.memberProxyIds.length === 1) setShowSelectedOnly(false)
    setForm(previous => ({
      ...previous,
      memberProxyIds: previous.memberProxyIds.some(id => id.toLowerCase() === proxyId.toLowerCase())
        ? previous.memberProxyIds.filter(id => id.toLowerCase() !== proxyId.toLowerCase())
        : [...previous.memberProxyIds, proxyId],
    }))
  }

  const handleSave = async () => {
    const groupName = form.groupName.trim()
    const canonicalProxyIds = new Map(
      availableProxies.map(proxy => [proxy.proxyId.trim().toLowerCase(), proxy.proxyId.trim()]),
    )
    const memberProxyIds = form.memberProxyIds
      .map(proxyId => proxyId.trim())
      .filter(proxyId => proxyId && canonicalProxyIds.has(proxyId.toLowerCase()))
      .map(proxyId => canonicalProxyIds.get(proxyId.toLowerCase()) || proxyId)

    if (!groupName) {
      toast.error('请输入虚拟分组名称')
      return
    }
    if (memberProxyIds.length === 0) {
      toast.error('至少选择一个代理节点')
      return
    }

    setSaving(true)
    try {
      await saveBrowserProxyVirtualGroup({
        groupId: form.groupId,
        groupName,
        strategy: form.strategy,
        selectedProxyId: form.selectedProxyId,
        memberProxyIds,
        memberGroupNames: form.memberGroupNames,
        sortOrder: form.sortOrder,
        createdAt: form.createdAt,
        updatedAt: form.updatedAt,
      })
      toast.success(form.groupId ? '虚拟分组已更新' : '虚拟分组已创建')
      closeEditor()
      await load()
    } catch (error: any) {
      toast.error(error?.message || '保存虚拟分组失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteBrowserProxyVirtualGroup(deleteTarget.groupId)
      toast.success('虚拟分组已删除')
      setDeleteTarget(null)
      await load()
    } catch (error: any) {
      toast.error(error?.message || '删除虚拟分组失败')
    }
  }

  const handleContinueToNodes = () => {
    const groupName = form.groupName.trim()
    if (!groupName) {
      toast.error('请输入虚拟分组名称')
      return
    }
    setForm(previous => ({ ...previous, groupName }))
    setEditorStep('nodes')
  }

  const renderLatency = (proxy: BrowserProxy) => {
    const value = latencyMap[proxy.proxyId]
    if (value === undefined) return <span className='text-xs text-[var(--color-text-muted)]'>-</span>
    if (value === -1) return <span className='animate-pulse text-xs text-[var(--color-text-muted)]'>测试中...</span>
    const error = latencyErrorMap[proxy.proxyId] || ''
    if (value === -2) return <span className='text-xs text-red-500' title={error || '测速超时'}>超时</span>
    if (value === -3) return <span className='text-xs text-gray-400' title={error || '协议不支持'}>不支持</span>
    if (value === -4) return <span className='text-xs text-red-500' title={error || '测速失败'}>失败</span>
    const color = value < 200 ? 'text-green-500' : value < 500 ? 'text-yellow-500' : 'text-red-500'
    return <span className={`text-xs font-medium ${color}`}>{value} ms</span>
  }

  const renderLatencyEngine = (proxy: BrowserProxy) => {
    const value = latencyMap[proxy.proxyId]
    if (value === undefined || value < 0) return <span className='text-xs text-[var(--color-text-muted)]'>-</span>
    return latencyEngineMap[proxy.proxyId]
      ? <span className='whitespace-nowrap text-xs text-[var(--color-text-secondary)]'>{latencyEngineMap[proxy.proxyId]}</span>
      : <span className='text-xs text-[var(--color-text-muted)]'>-</span>
  }

  const proxyColumns: TableColumn<BrowserProxy>[] = [
    {
      key: 'select',
      title: '',
      width: '56px',
      align: 'center',
      render: (_, proxy) => (
        <input
          type='checkbox'
          aria-label={`选择${proxy.proxyName || proxy.proxyId}`}
          checked={form.memberProxyIds.some(id => id.toLowerCase() === proxy.proxyId.toLowerCase())}
          onChange={() => toggleMemberProxy(proxy.proxyId)}
          className='h-4 w-4 cursor-pointer accent-[var(--color-accent)]'
        />
      ),
    },
    {
      key: 'proxyName',
      title: '代理节点',
      render: (_, proxy) => (
        <div className='min-w-0'>
          <div className='truncate font-medium text-[var(--color-text-primary)]' title={proxy.proxyName || proxy.proxyId}>
            {proxy.proxyName || proxy.proxyId}
          </div>
          <div className='truncate text-[11px] text-[var(--color-text-muted)]' title={proxy.proxyId}>
            {proxy.proxyId}
          </div>
        </div>
      ),
    },
    {
      key: 'groupName',
      title: '所属分组',
      width: '180px',
      render: (_, proxy) => (
        <div className='flex min-w-0 items-center gap-1.5 text-xs text-[var(--color-text-secondary)]'>
          <FolderOpen className='h-3.5 w-3.5 shrink-0 text-[var(--color-text-muted)]' />
          <span className='truncate' title={proxy.groupName || '未分组'}>{proxy.groupName || '未分组'}</span>
        </div>
      ),
    },
    {
      key: 'latency',
      title: '延迟',
      width: '90px',
      sortable: true,
      render: (_, proxy) => renderLatency(proxy),
    },
    {
      key: 'latencyEngine',
      title: '测速类型',
      width: '100px',
      render: (_, proxy) => renderLatencyEngine(proxy),
    },
    {
      key: 'actions',
      title: '操作',
      width: '100px',
      render: (_, proxy) => (
        <Button
          size='sm'
          variant='secondary'
          className='min-w-[56px] justify-center'
          loading={latencyMap[proxy.proxyId] === -1}
          onClick={event => {
            event.stopPropagation()
            const displayProxy = displayProxyById.get(proxy.proxyId.trim().toLowerCase())
            if (displayProxy) void handleTestOne(displayProxy)
          }}
        >
          测速
        </Button>
      ),
    },
  ]

  const columns: TableColumn<BrowserProxyVirtualGroup>[] = [
    {
      key: 'groupName',
      title: '虚拟分组',
      width: '220px',
      render: (_, group) => <span className='font-medium text-[var(--color-text-primary)]'>{group.groupName}</span>,
    },
    {
      key: 'memberProxyIds',
      title: '关联代理节点',
      render: (_, group) => {
        const memberProxyIds = resolveMemberProxyIds(group)
        return (
          <button
            type='button'
            className='w-full rounded-md text-left hover:bg-[var(--color-bg-muted)]'
            onClick={() => openMembersModal(group)}
            title='查看关联代理节点'
          >
            {memberProxyIds.length === 0 ? (
              <span className='text-xs text-[var(--color-text-muted)]'>未选择节点</span>
            ) : (
              <div className='flex flex-wrap gap-1.5'>
                {memberProxyIds.map(proxyId => {
                  const proxy = proxyById.get(proxyId.toLowerCase())
                  return (
                    <span key={proxyId} className='rounded bg-[var(--color-accent)]/10 px-2 py-0.5 text-xs text-[var(--color-accent)]'>
                      {proxy?.proxyName || proxyId}
                    </span>
                  )
                })}
              </div>
            )}
          </button>
        )
      },
    },
    {
      key: 'strategy',
      title: '选择策略',
      width: '120px',
      render: (_, group) => <span className='text-sm text-[var(--color-text-secondary)]'>{getVirtualGroupStrategyLabel(group.strategy)}</span>,
    },
    {
      key: 'actions',
      title: '操作',
      width: '140px',
      align: 'center',
      render: (_, group) => (
        <div className='flex items-center justify-center gap-1'>
          <Button size='sm' variant='ghost' onClick={() => openEdit(group)} title='编辑'>
            <Pencil className='h-4 w-4' />
          </Button>
          <Button size='sm' variant='ghost' onClick={() => setDeleteTarget(group)} title='删除'>
            <Trash2 className='h-4 w-4 text-red-500' />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div className='space-y-4 animate-fade-in'>
      <Card padding='none' className='shadow-[var(--shadow-sm)]'>
        <div className='flex items-center justify-between gap-3 px-4 py-3'>
          <div className='flex min-w-0 items-center gap-2'>
            <Button
              size='sm'
              variant='ghost'
              onClick={() => navigate('/browser/proxy-pool')}
              title='返回代理池'
              aria-label='返回代理池'
            >
              <ArrowLeft className='h-4 w-4' />
            </Button>
            <h1 className='text-xl font-semibold text-[var(--color-text-primary)]'>虚拟连接分组</h1>
          </div>
          <Button size='sm' onClick={openCreate}>
            <Plus className='h-4 w-4' />新建虚拟分组
          </Button>
        </div>
      </Card>

      <Card padding='none'>
        <Table
          columns={columns}
          data={virtualGroups}
          rowKey='groupId'
          tableLayout='fixed'
          loading={loading}
          emptyText='暂无虚拟分组'
        />
      </Card>

      <ProxyVirtualGroupMembersModal
        open={membersModal !== null}
        groupName={membersModal?.groupName || ''}
        strategy={membersModal?.strategy}
        members={membersModal?.members || []}
        onClose={() => setMembersModal(null)}
      />

      <Modal
        open={editorOpen}
        onClose={closeEditor}
        title={form.groupId ? '编辑虚拟分组' : '新建虚拟分组'}
        width='920px'
        footer={(
          <>
            <Button
              variant='secondary'
              onClick={editorStep === 'details' ? closeEditor : () => setEditorStep('details')}
              disabled={saving}
            >
              {editorStep === 'details' ? '取消' : '返回'}
            </Button>
            {editorStep === 'details' ? (
              <Button onClick={handleContinueToNodes}>下一步</Button>
            ) : (
              <Button onClick={handleSave} loading={saving}>保存</Button>
            )}
          </>
        )}
      >
        {editorStep === 'details' ? (
          <div className='space-y-4'>
            <label className='block'>
              <span className='text-sm font-medium text-[var(--color-text-primary)]'>虚拟分组名称</span>
              <Input
                value={form.groupName}
                onChange={event => setForm(previous => ({ ...previous, groupName: event.target.value }))}
                placeholder='例如：主线路轮询'
                autoFocus
              />
            </label>
            <label className='block'>
              <span className='text-sm font-medium text-[var(--color-text-primary)]'>选择策略</span>
              <Select
                aria-label='选择策略'
                value={form.strategy}
                onChange={event => setForm(previous => ({ ...previous, strategy: event.target.value as VirtualGroupForm['strategy'], selectedProxyId: event.target.value === 'sticky' ? previous.selectedProxyId : '' }))}
                options={virtualGroupStrategyOptions}
              />
            </label>
          </div>
        ) : (
          <div className='space-y-4'>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div className='flex items-center gap-2'>
                <span className='text-sm font-medium text-[var(--color-text-primary)]'>选择代理节点</span>
                <span className='text-xs text-[var(--color-text-muted)]'>
                  {form.memberProxyIds.length}/{availableProxies.length}
                </span>
              </div>
              <label className='inline-flex cursor-pointer select-none items-center gap-1.5 text-xs text-[var(--color-text-muted)]'>
                <Switch
                  checked={showSelectedOnly}
                  onChange={setShowSelectedOnly}
                  aria-label='仅显示已选节点'
                />
                <span>仅显示已选</span>
              </label>
              <label className='inline-flex cursor-pointer select-none items-center gap-1.5 text-xs text-[var(--color-text-muted)]'>
                <Switch
                  checked={showAvailableOnly}
                  onChange={setShowAvailableOnly}
                  aria-label='仅显示可用节点'
                />
                <span>仅显示可选</span>
              </label>
            </div>
            {proxyRegionOptions.length > 1 && (
              <div className='flex flex-wrap items-center gap-1.5' role='group' aria-label='按地区快速筛选'>
                {proxyRegionOptions.map(option => {
                  const active = proxyRegionFilter === option.value
                  return (
                    <button
                      key={option.value || 'all'}
                      type='button'
                      aria-pressed={active}
                      onClick={() => setProxyRegionFilter(option.value)}
                      className={`rounded-full border px-2.5 py-1 text-xs transition-colors ${active
                        ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)] font-medium text-[var(--color-accent)]'
                        : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)]'}`}
                    >
                      {option.label}
                    </button>
                  )
                })}
              </div>
            )}
            {availableProxies.length === 0 ? (
              <div className='rounded-lg border border-dashed border-[var(--color-border)] px-3 py-8 text-center text-sm text-[var(--color-text-muted)]'>
                请先在代理池中创建代理节点
              </div>
            ) : (
              <div className='space-y-3'>
                <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
                  <div className='relative min-w-0 flex-1'>
                    <Search className='pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]' />
                    <Input
                      value={proxyFilter}
                      onChange={event => setProxyFilter(event.target.value)}
                      placeholder='筛选代理名、ID或所属分组'
                      aria-label='筛选代理节点'
                      className='w-full pl-9'
                    />
                  </div>
                  <Select
                    aria-label='按实体分组筛选'
                    value={proxyGroupFilter}
                    onChange={event => setProxyGroupFilter(event.target.value)}
                    options={proxyGroupOptions}
                    className='w-full shrink-0 sm:w-44'
                  />
                  <div className='flex items-center justify-between gap-2 sm:shrink-0'>
                    <span className='text-xs text-[var(--color-text-muted)]'>
                      显示 {filteredAvailableProxies.length}/{availableProxies.length}
                    </span>
                    <Button
                      size='sm'
                      variant='secondary'
                      loading={testingAll}
                      disabled={filteredAvailableProxies.length === 0}
                      onClick={() => void handleTestAll(filteredAvailableDisplayProxies)}
                    >
                      全部测速
                    </Button>
                  </div>
                </div>
                <div className='overflow-hidden rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] md:hidden'>
                  {filteredAvailableProxies.length === 0 ? (
                    <div className='px-3 py-8 text-center text-sm text-[var(--color-text-muted)]'>没有匹配的代理节点</div>
                  ) : (
                    filteredAvailableProxies.map(proxy => {
                      const selected = form.memberProxyIds.some(id => id.toLowerCase() === proxy.proxyId.toLowerCase())
                      return (
                        <div
                          key={proxy.proxyId}
                          className={`flex items-center gap-3 border-b border-[var(--color-border-muted)] px-3 py-3 last:border-0 ${selected ? 'bg-[var(--color-accent)]/5' : ''}`}
                        >
                          <input
                            type='checkbox'
                            aria-label={`选择${proxy.proxyName || proxy.proxyId}`}
                            checked={selected}
                            onChange={() => toggleMemberProxy(proxy.proxyId)}
                            className='h-4 w-4 shrink-0 cursor-pointer accent-[var(--color-accent)]'
                          />
                          <div className='min-w-0 flex-1'>
                            <div className='flex min-w-0 items-center gap-2'>
                              <span className='min-w-0 truncate text-sm font-medium text-[var(--color-text-primary)]' title={proxy.proxyName || proxy.proxyId}>
                                {proxy.proxyName || proxy.proxyId}
                              </span>
                              {proxy.groupName && (
                                <span className='max-w-[42%] shrink-0 truncate rounded bg-[var(--color-accent)]/10 px-1.5 py-0.5 text-[11px] text-[var(--color-accent)]' title={proxy.groupName}>
                                  {proxy.groupName}
                                </span>
                              )}
                            </div>
                            <div className='mt-0.5 truncate text-[11px] text-[var(--color-text-muted)]' title={proxy.proxyId}>
                              {proxy.proxyId}
                            </div>
                            <div className='mt-1 flex items-center gap-2 whitespace-nowrap'>
                              {renderLatency(proxy)}
                              <span className='h-3 w-px bg-[var(--color-border)]' aria-hidden='true' />
                              {renderLatencyEngine(proxy)}
                            </div>
                          </div>
                          <Button
                            size='sm'
                            variant='secondary'
                            className='min-w-[56px] shrink-0 justify-center'
                            loading={latencyMap[proxy.proxyId] === -1}
                            onClick={event => {
                              event.stopPropagation()
                              const displayProxy = displayProxyById.get(proxy.proxyId.trim().toLowerCase())
                              if (displayProxy) void handleTestOne(displayProxy)
                            }}
                          >
                            测速
                          </Button>
                        </div>
                      )
                    })
                  )}
                </div>
                <div className='hidden md:block'>
                  <Table
                    columns={proxyColumns}
                    data={filteredAvailableProxies}
                    rowKey='proxyId'
                    tableLayout='fixed'
                    tableMinWidth='820px'
                    maxHeight='none'
                    emptyText='没有匹配的代理节点'
                    sortColumn={proxySortColumn}
                    sortOrder={proxySortOrder}
                    onSort={result => {
                      setProxySortColumn(result.column)
                      setProxySortOrder(result.order)
                    }}
                  />
                </div>
              </div>
            )}
          </div>
        )}
      </Modal>

      <ConfirmModal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title='删除虚拟分组'
        content={`确定删除「${deleteTarget?.groupName || ''}」吗？已关联的实例不会被删除。`}
        confirmText='删除'
        danger
      />
    </div>
  )
}
