import { useState, useMemo } from 'react'
import { Plus, Trash2, Edit2, Download, Upload, X, Save } from 'lucide-react'
import { Badge, Button, FormItem, Input, Modal, Table, Textarea, toast, Select } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'

interface Cookie {
  name: string
  value: string
  domain: string
  path: string
  expires?: number
  httpOnly?: boolean
  secure?: boolean
  sameSite?: 'Strict' | 'Lax' | 'None'
}

interface Props {
  open: boolean
  onClose: () => void
  initialCookies: string
  onSave: (cookies: string) => void
  onExtract?: () => void
  canExtract?: boolean
}

export function CookieEditorModal({ open, onClose, initialCookies, onSave, onExtract, canExtract }: Props) {
  const [viewMode, setViewMode] = useState<'table' | 'json'>('table')
  const [cookies, setCookies] = useState<Cookie[]>([])
  const [jsonText, setJsonText] = useState(initialCookies)
  const [editingCookie, setEditingCookie] = useState<Cookie | null>(null)
  const [editModalOpen, setEditModalOpen] = useState(false)

  // 解析 Cookie 文本
  const parseCookies = (text: string): Cookie[] => {
    if (!text.trim()) return []
    try {
      const parsed = JSON.parse(text)
      if (Array.isArray(parsed)) {
        return parsed.map(c => ({
          name: c.name || '',
          value: c.value || '',
          domain: c.domain || '',
          path: c.path || '/',
          expires: c.expires || c.expirationDate,
          httpOnly: c.httpOnly ?? false,
          secure: c.secure ?? false,
          sameSite: c.sameSite || 'Lax',
        }))
      }
    } catch (error) {
      console.error('解析Cookie失败:', error)
    }
    return []
  }

  // 序列化 Cookie 到 JSON
  const serializeCookies = (cookieList: Cookie[]): string => {
    return JSON.stringify(cookieList, null, 2)
  }

  // 初始化加载
  useMemo(() => {
    if (open) {
      setJsonText(initialCookies)
      setCookies(parseCookies(initialCookies))
    }
  }, [open, initialCookies])

  // 切换视图模式
  const handleSwitchMode = (mode: 'table' | 'json') => {
    if (mode === 'json') {
      // 切换到 JSON 模式，同步表格数据到 JSON
      setJsonText(serializeCookies(cookies))
    } else {
      // 切换到表格模式，尝试解析 JSON
      const parsed = parseCookies(jsonText)
      if (parsed.length > 0 || !jsonText.trim()) {
        setCookies(parsed)
      } else {
        toast.error('JSON 格式错误，无法切换到表格模式')
        return
      }
    }
    setViewMode(mode)
  }

  // 添加新 Cookie
  const handleAddCookie = () => {
    setEditingCookie({
      name: '',
      value: '',
      domain: '',
      path: '/',
      httpOnly: false,
      secure: false,
      sameSite: 'Lax',
    })
    setEditModalOpen(true)
  }

  // 编辑 Cookie
  const handleEditCookie = (cookie: Cookie) => {
    setEditingCookie({ ...cookie })
    setEditModalOpen(true)
  }

  // 删除 Cookie
  const handleDeleteCookie = (name: string, domain: string) => {
    setCookies(prev => prev.filter(c => !(c.name === name && c.domain === domain)))
    toast.success('Cookie 已删除')
  }

  // 保存编辑的 Cookie
  const handleSaveEdit = () => {
    if (!editingCookie) return
    if (!editingCookie.name.trim()) {
      toast.error('请输入 Cookie 名称')
      return
    }
    if (!editingCookie.domain.trim()) {
      toast.error('请输入域名')
      return
    }

    setCookies(prev => {
      // 检查是否已存在（通过 name + domain 判断）
      const index = prev.findIndex(c => c.name === editingCookie.name && c.domain === editingCookie.domain)
      if (index >= 0) {
        // 更新现有
        const newList = [...prev]
        newList[index] = editingCookie
        return newList
      } else {
        // 添加新的
        return [...prev, editingCookie]
      }
    })
    setEditModalOpen(false)
    toast.success('Cookie 已保存')
  }

  // 导出 Cookie
  const handleExport = () => {
    const text = viewMode === 'table' ? serializeCookies(cookies) : jsonText
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `cookies_${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    toast.success('Cookie 已导出')
  }

  // 导入 Cookie
  const handleImport = () => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.json,.txt'
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0]
      if (!file) return

      const reader = new FileReader()
      reader.onload = (event) => {
        const text = event.target?.result as string
        const parsed = parseCookies(text)
        if (parsed.length > 0) {
          setCookies(parsed)
          setJsonText(text)
          toast.success(`已导入 ${parsed.length} 个 Cookie`)
        } else {
          toast.error('导入失败，文件格式错误')
        }
      }
      reader.readAsText(file)
    }
    input.click()
  }

  // 最终保存
  const handleFinalSave = () => {
    const finalText = viewMode === 'table' ? serializeCookies(cookies) : jsonText
    onSave(finalText)
  }

  // 提取 Cookie
  const handleExtractClick = async () => {
    if (!onExtract) return
    await onExtract()
    // 重新加载
    setTimeout(() => {
      setJsonText(initialCookies)
      setCookies(parseCookies(initialCookies))
    }, 500)
  }

  const formatExpires = (expires?: number) => {
    if (!expires || expires <= 0) return 'Session'
    return new Date(expires * 1000).toLocaleString('zh-CN')
  }

  const columns: TableColumn<Cookie>[] = [
    {
      key: 'name',
      title: '名称',
      render: (value) => <span className="font-mono text-xs text-[var(--color-text-primary)]">{value}</span>,
    },
    {
      key: 'value',
      title: '值',
      render: (value) => (
        <span className="font-mono text-xs text-[var(--color-text-secondary)] max-w-[150px] truncate block" title={value as string}>
          {value}
        </span>
      ),
    },
    {
      key: 'domain',
      title: '域名',
      render: (value) => <span className="font-mono text-xs text-[var(--color-text-secondary)]">{value}</span>,
    },
    {
      key: 'path',
      title: '路径',
      render: (value) => <span className="font-mono text-xs text-[var(--color-text-muted)]">{value}</span>,
    },
    {
      key: 'expires',
      title: '过期时间',
      render: (value) => <span className="text-xs text-[var(--color-text-muted)]">{formatExpires(value as number)}</span>,
    },
    {
      key: 'secure',
      title: 'Secure',
      render: (value) => <Badge variant={value ? 'success' : 'default'}>{value ? '是' : '否'}</Badge>,
    },
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      render: (_, record) => (
        <div className="flex justify-end gap-1">
          <Button size="sm" variant="ghost" onClick={() => handleEditCookie(record)} title="编辑">
            <Edit2 className="w-3.5 h-3.5" />
          </Button>
          <Button size="sm" variant="ghost" onClick={() => handleDeleteCookie(record.name, record.domain)} title="删除">
            <Trash2 className="w-3.5 h-3.5 text-red-500" />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <>
      <Modal open={open} onClose={onClose} title="管理 Cookies" width="900px">
        <div className="space-y-4 py-4">
          {/* 工具栏 */}
          <div className="flex items-center justify-between">
            <div className="flex gap-2">
              <Button
                size="sm"
                variant={viewMode === 'table' ? 'primary' : 'secondary'}
                onClick={() => handleSwitchMode('table')}
              >
                表格模式
              </Button>
              <Button
                size="sm"
                variant={viewMode === 'json' ? 'primary' : 'secondary'}
                onClick={() => handleSwitchMode('json')}
              >
                JSON 模式
              </Button>
            </div>
            <div className="flex gap-2">
              {canExtract && onExtract && (
                <Button size="sm" variant="secondary" onClick={handleExtractClick}>
                  <Download className="w-3.5 h-3.5" />从浏览器提取
                </Button>
              )}
              <Button size="sm" variant="secondary" onClick={handleImport}>
                <Upload className="w-3.5 h-3.5" />导入
              </Button>
              <Button size="sm" variant="secondary" onClick={handleExport}>
                <Download className="w-3.5 h-3.5" />导出
              </Button>
              {viewMode === 'table' && (
                <Button size="sm" onClick={handleAddCookie}>
                  <Plus className="w-3.5 h-3.5" />添加
                </Button>
              )}
            </div>
          </div>

          {/* 内容区域 */}
          {viewMode === 'table' ? (
            <div className="border border-[var(--color-border-default)] rounded-lg overflow-hidden">
              <div className="overflow-auto" style={{ maxHeight: '450px' }}>
                {cookies.length === 0 ? (
                  <div className="py-12 text-center text-sm text-[var(--color-text-muted)]">
                    暂无 Cookie，点击"添加"按钮创建
                  </div>
                ) : (
                  <Table columns={columns} data={cookies} rowKey={(record) => `${record.name}_${record.domain}`} />
                )}
              </div>
            </div>
          ) : (
            <div>
              <p className="text-xs text-[var(--color-text-muted)] mb-2">JSON 格式的 Cookie 数组</p>
              <Textarea
                rows={18}
                value={jsonText}
                onChange={(e) => setJsonText(e.target.value)}
                placeholder={'[\n  {\n    "name": "session_id",\n    "value": "abc123",\n    "domain": ".example.com",\n    "path": "/"\n  }\n]'}
                className="font-mono text-sm"
              />
            </div>
          )}
        </div>

        <div className="flex justify-between items-center pt-4 border-t border-[var(--color-border-default)]">
          <div className="text-sm text-[var(--color-text-muted)]">
            共 {viewMode === 'table' ? cookies.length : parseCookies(jsonText).length} 个 Cookie
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={onClose}>
              <X className="w-4 h-4" />取消
            </Button>
            <Button onClick={handleFinalSave}>
              <Save className="w-4 h-4" />保存
            </Button>
          </div>
        </div>
      </Modal>

      {/* Cookie 编辑弹窗 */}
      <Modal
        open={editModalOpen}
        onClose={() => setEditModalOpen(false)}
        title={editingCookie?.name ? '编辑 Cookie' : '添加 Cookie'}
        width="600px"
      >
        <div className="space-y-4 py-4">
          <FormItem label="名称" required>
            <Input
              value={editingCookie?.name || ''}
              onChange={(e) => setEditingCookie(prev => prev ? { ...prev, name: e.target.value } : null)}
              placeholder="cookie_name"
            />
          </FormItem>

          <FormItem label="值" required>
            <Textarea
              rows={3}
              value={editingCookie?.value || ''}
              onChange={(e) => setEditingCookie(prev => prev ? { ...prev, value: e.target.value } : null)}
              placeholder="cookie_value"
            />
          </FormItem>

          <div className="grid grid-cols-2 gap-3">
            <FormItem label="域名" required>
              <Input
                value={editingCookie?.domain || ''}
                onChange={(e) => setEditingCookie(prev => prev ? { ...prev, domain: e.target.value } : null)}
                placeholder=".example.com"
              />
            </FormItem>
            <FormItem label="路径">
              <Input
                value={editingCookie?.path || '/'}
                onChange={(e) => setEditingCookie(prev => prev ? { ...prev, path: e.target.value } : null)}
                placeholder="/"
              />
            </FormItem>
          </div>

          <FormItem label="过期时间（Unix时间戳）">
            <Input
              type="number"
              value={editingCookie?.expires || ''}
              onChange={(e) => setEditingCookie(prev => prev ? { ...prev, expires: Number(e.target.value) } : null)}
              placeholder="留空表示会话Cookie"
            />
          </FormItem>

          <div className="grid grid-cols-3 gap-3">
            <FormItem label="SameSite">
              <Select
                value={editingCookie?.sameSite || 'Lax'}
                onChange={(e) => setEditingCookie(prev => prev ? { ...prev, sameSite: e.target.value as any } : null)}
                options={[
                  { value: 'Lax', label: 'Lax' },
                  { value: 'Strict', label: 'Strict' },
                  { value: 'None', label: 'None' },
                ]}
              />
            </FormItem>
            <div className="flex items-center gap-2 pt-6">
              <input
                type="checkbox"
                checked={editingCookie?.secure || false}
                onChange={(e) => setEditingCookie(prev => prev ? { ...prev, secure: e.target.checked } : null)}
                id="cookie-secure"
              />
              <label htmlFor="cookie-secure" className="text-sm text-[var(--color-text-secondary)]">Secure</label>
            </div>
            <div className="flex items-center gap-2 pt-6">
              <input
                type="checkbox"
                checked={editingCookie?.httpOnly || false}
                onChange={(e) => setEditingCookie(prev => prev ? { ...prev, httpOnly: e.target.checked } : null)}
                id="cookie-httponly"
              />
              <label htmlFor="cookie-httponly" className="text-sm text-[var(--color-text-secondary)]">HttpOnly</label>
            </div>
          </div>
        </div>

        <div className="flex justify-end gap-2 pt-4 border-t border-[var(--color-border-default)]">
          <Button variant="secondary" onClick={() => setEditModalOpen(false)}>
            <X className="w-4 h-4" />取消
          </Button>
          <Button onClick={handleSaveEdit}>
            <Save className="w-4 h-4" />保存
          </Button>
        </div>
      </Modal>
    </>
  )
}
