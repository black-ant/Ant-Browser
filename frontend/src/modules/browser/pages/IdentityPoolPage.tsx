import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight, Plus, RotateCcw, ShieldCheck } from 'lucide-react'

import { Alert, Badge, Button, Card, Input, Modal, Select, Table, ConfirmModal, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import {
  IdentityPoolRecord,
  IdentityValidateAllReport,
  IdentityValidationResult,
  createIdentity,
  deleteIdentity,
  fetchIdentityPool,
  restoreDefaultIdentities,
  updateIdentity,
  validateAllIdentities,
  validateIdentity,
} from '../api'
import { IdentityEditModal } from './identityPool/IdentityEditModal'

const PAGE_SIZE = 50
const major = (v: string) => (v || '').split('.')[0]

export function IdentityPoolPage() {
  const [records, setRecords] = useState<IdentityPoolRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [platform, setPlatform] = useState('')
  const [version, setVersion] = useState('')
  const [page, setPage] = useState(0)

  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<IdentityPoolRecord | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [restoreOpen, setRestoreOpen] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const [validateResult, setValidateResult] = useState<{ rec: IdentityPoolRecord; res: IdentityValidationResult } | null>(null)
  const [allReport, setAllReport] = useState<IdentityValidateAllReport | null>(null)
  const [checkingAll, setCheckingAll] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setRecords(await fetchIdentityPool())
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setPage(0) }, [search, platform, version])

  const platformOptions = useMemo(
    () => [{ value: '', label: '全部平台' }, { value: 'windows', label: 'Windows' }, { value: 'macos', label: 'macOS' }, { value: 'linux', label: 'Linux' }],
    [],
  )

  const filtered = useMemo(() => {
    // User-Agent 模糊搜索:空格分隔多关键词,全部命中才算匹配(如 "mac 146")。
    // 时区/语言是创建环境时按出口 IP 自动对齐的,不参与池内搜索。
    const terms = search.trim().toLowerCase().split(/\s+/).filter(Boolean)
    const ver = version.trim()
    return records.filter(r => {
      if (platform && r.platform !== platform) return false
      if (ver && major(r.brandVersion) !== ver) return false
      if (terms.length) {
        const ua = (r.uaFull || '').toLowerCase()
        if (!terms.every(t => ua.includes(t))) return false
      }
      return true
    })
  }, [records, search, platform, version])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const pageClamped = Math.min(page, totalPages - 1)
  const paged = useMemo(
    () => filtered.slice(pageClamped * PAGE_SIZE, pageClamped * PAGE_SIZE + PAGE_SIZE),
    [filtered, pageClamped],
  )

  const handleAdd = () => { setEditing(null); setEditOpen(true) }
  const handleEdit = (rec: IdentityPoolRecord) => { setEditing(rec); setEditOpen(true) }

  const handleSubmit = async (rec: IdentityPoolRecord) => {
    setSaving(true)
    try {
      if (editing?.id) {
        await updateIdentity(editing.id, rec)
        toast.success('已更新身份模板')
      } else {
        await createIdentity(rec)
        toast.success('已新增身份模板')
      }
      setEditOpen(false)
      await load()
    } catch (e: any) {
      toast.error(e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDeleteConfirm = async () => {
    if (!deleteId) return
    try {
      await deleteIdentity(deleteId)
      toast.success('已删除')
      setDeleteId(null)
      await load()
    } catch (e: any) {
      toast.error(e?.message || '删除失败')
    }
  }

  const handleRestore = async () => {
    setRestoring(true)
    try {
      await restoreDefaultIdentities()
      toast.success('已恢复默认身份池')
      setRestoreOpen(false)
      await load()
    } catch (e: any) {
      toast.error(e?.message || '恢复失败')
    } finally {
      setRestoring(false)
    }
  }

  const handleValidateRow = async (rec: IdentityPoolRecord) => {
    const res = await validateIdentity(rec)
    if (res) setValidateResult({ rec, res })
  }

  const handleValidateAll = async () => {
    setCheckingAll(true)
    try {
      const report = await validateAllIdentities()
      if (report) setAllReport(report)
    } finally {
      setCheckingAll(false)
    }
  }

  const columns: TableColumn<IdentityPoolRecord>[] = [
    { key: 'platform', title: '平台', width: 90, render: v => <Badge variant="default">{v}</Badge> },
    { key: 'brandVersion', title: '版本', width: 90, render: v => <span className="font-mono text-xs">{v}</span> },
    {
      key: 'uaFull', title: 'User-Agent', width: 300,
      render: v => <span className="block max-w-[290px] truncate text-xs" title={String(v || '')}>{v}</span>,
    },
    { key: 'hardwareConcurrency', title: '硬件', width: 90, render: (_v, r) => <span className="text-xs">{r.hardwareConcurrency}核/{r.deviceMemory}G</span> },
    { key: 'screen', title: '屏幕', width: 110, render: (_v, r) => <span className="text-xs">{r.screen.width}×{r.screen.height}</span> },
    {
      key: 'timezone', title: '时区/语言', width: 110,
      render: () => (
        <span className="text-xs text-[var(--color-text-muted)]" title="创建环境时按出口 IP 自动对齐:直连=本地国家,绑代理=代理所在地区">
          跟随出口 IP
        </span>
      ),
    },
    { key: 'weight', title: '权重', width: 64, render: v => <span className="text-xs">{v}</span> },
    {
      key: 'actions', title: '操作', width: 200, align: 'right',
      render: (_v, r) => (
        <div className="flex justify-end gap-1.5 whitespace-nowrap">
          <Button size="sm" variant="ghost" onClick={() => handleEdit(r)}>编辑</Button>
          <Button size="sm" variant="ghost" onClick={() => void handleValidateRow(r)} title="自洽检查">检查</Button>
          <Button size="sm" variant="ghost" className="text-red-500 hover:text-red-600" onClick={() => r.id && setDeleteId(r.id)}>删除</Button>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-4 p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">身份池管理</h1>
          <p className="text-xs text-[var(--color-text-muted)]">共 {records.length} 条真机设备模板(UA/硬件/屏幕)· 新建/批量环境时从中加权采样</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => void handleValidateAll()} loading={checkingAll}><ShieldCheck className="mr-1 h-3.5 w-3.5" />检查全部</Button>
          <Button variant="secondary" size="sm" onClick={() => setRestoreOpen(true)}><RotateCcw className="mr-1 h-3.5 w-3.5" />全部恢复默认</Button>
          <Button size="sm" onClick={handleAdd}><Plus className="mr-1 h-3.5 w-3.5" />新增</Button>
        </div>
      </div>

      <Alert type="info" message="身份池只提供设备指纹(UA/硬件/屏幕);时区/语言/Locale 在创建环境时按出口 IP 自动对齐:直连=本地国家,绑代理=代理所在地区。编辑只影响之后新建的环境,已有环境指纹已固化。" />

      <Card padding="none">
        <div className="flex flex-wrap items-center gap-2 border-b border-[var(--color-border-muted)] p-3">
          <Input className="w-72" placeholder="模糊搜索 User-Agent,空格分隔多关键词" value={search} onChange={e => setSearch(e.target.value)} />
          <Select className="w-36" options={platformOptions} value={platform} onChange={e => setPlatform(e.target.value)} />
          <Input className="w-32" placeholder="主版本 如147" value={version} onChange={e => setVersion(e.target.value)} />
          <span className="ml-auto text-xs text-[var(--color-text-muted)]">筛选出 {filtered.length} 条</span>
        </div>
        <Table columns={columns} data={paged} rowKey="id" size="compact" loading={loading} emptyText="没有匹配的记录" />
        <div className="flex items-center justify-end gap-2 border-t border-[var(--color-border-muted)] p-2.5 text-xs text-[var(--color-text-secondary)]">
          <span>第 {pageClamped + 1} / {totalPages} 页</span>
          <Button size="sm" variant="ghost" disabled={pageClamped <= 0} onClick={() => setPage(p => Math.max(0, p - 1))}><ChevronLeft className="h-3.5 w-3.5" /></Button>
          <Button size="sm" variant="ghost" disabled={pageClamped >= totalPages - 1} onClick={() => setPage(p => Math.min(totalPages - 1, p + 1))}><ChevronRight className="h-3.5 w-3.5" /></Button>
        </div>
      </Card>

      <IdentityEditModal open={editOpen} record={editing} saving={saving} onClose={() => setEditOpen(false)} onSubmit={handleSubmit} />

      <ConfirmModal
        open={!!deleteId}
        onClose={() => setDeleteId(null)}
        onConfirm={() => void handleDeleteConfirm()}
        title="删除身份模板"
        content="删除后该模板不再参与新环境采样(不影响已有环境)。确认删除?"
        confirmText="删除"
        danger
      />

      <ConfirmModal
        open={restoreOpen}
        onClose={() => setRestoreOpen(false)}
        onConfirm={() => void handleRestore()}
        title="恢复默认身份池"
        content={restoring ? '恢复中…' : '将丢弃所有对身份池的修改,恢复为内置默认(3000+ 条)。确认?'}
        confirmText="恢复默认"
        danger
      />

      {/* 单条自洽检查结果 */}
      <Modal
        open={!!validateResult}
        onClose={() => setValidateResult(null)}
        title="自洽检查"
        footer={<Button onClick={() => setValidateResult(null)}>知道了</Button>}
      >
        {validateResult && (
          validateResult.res.ok ? (
            <div className="flex items-center gap-2 text-[var(--color-success)]"><ShieldCheck className="h-4 w-4" />该模板自洽,无问题。</div>
          ) : (
            <ul className="space-y-1 text-sm">
              {validateResult.res.issues.map((is, i) => (
                <li key={i} className={is.severity === 'error' ? 'text-[var(--color-error)]' : 'text-[var(--color-warning)]'}>
                  • [{is.field}] {is.message}
                </li>
              ))}
            </ul>
          )
        )}
      </Modal>

      {/* 全量检查报告 */}
      <Modal
        open={!!allReport}
        onClose={() => setAllReport(null)}
        width="640px"
        title="全量自洽检查"
        footer={<Button onClick={() => setAllReport(null)}>知道了</Button>}
      >
        {allReport && (
          <div className="space-y-3">
            <div className="flex gap-4 text-sm">
              <span>总计 <b>{allReport.total}</b></span>
              <span className="text-[var(--color-success)]">通过 <b>{allReport.okCount}</b></span>
              <span className="text-[var(--color-error)]">不自洽 <b>{allReport.badCount}</b></span>
            </div>
            {allReport.badCount > 0 && (
              <div className="max-h-[50vh] space-y-2 overflow-auto">
                {allReport.problems.slice(0, 200).map((p, i) => (
                  <div key={i} className="rounded border border-[var(--color-border-muted)] p-2 text-xs">
                    <div className="truncate font-mono text-[var(--color-text-muted)]" title={p.uaFull}>{p.platform} · {p.uaFull}</div>
                    <ul className="mt-1">
                      {p.issues.map((is, j) => (
                        <li key={j} className={is.severity === 'error' ? 'text-[var(--color-error)]' : 'text-[var(--color-warning)]'}>• [{is.field}] {is.message}</li>
                      ))}
                    </ul>
                  </div>
                ))}
                {allReport.problems.length > 200 && <div className="text-xs text-[var(--color-text-muted)]">仅显示前 200 条…</div>}
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}

export default IdentityPoolPage
