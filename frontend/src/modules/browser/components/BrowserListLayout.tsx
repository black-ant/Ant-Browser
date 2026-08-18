import { Link } from 'react-router-dom'
import { Archive, CheckCircle, ChevronRight, ChevronUp, Edit2, LayoutGrid, List, Play, Plus, RefreshCw, Sliders, Star, Trash2, Upload, XCircle } from 'lucide-react'

import { Button, Card, FormItem, Input, Modal, Switch, Table, Textarea } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'

import type { BrowserCore, BrowserCoreInput, BrowserGroupWithCount, BrowserProxy, BrowserSettings } from '../types'
import { InstanceFilterBar } from './InstanceFilterBar'
import type { InstanceFilters } from './InstanceFilterBar'

export type BrowserViewMode = 'card' | 'table'

interface BrowserListHeaderProps {
  profileCount: number
  filteredProfileCount: number
  runningCount: number
  headerCollapsed: boolean
  viewMode: BrowserViewMode
  proxies: BrowserProxy[]
  cores: BrowserCore[]
  groups: BrowserGroupWithCount[]
  allTags: string[]
  filters: InstanceFilters
  onFiltersChange: (next: InstanceFilters) => void
  onToggleHeaderCollapsed: () => void
  onRefresh: () => void
  onOpenSettings: () => void
  onOpenTrash: () => void
  onImportProfiles: () => void
  importingProfiles?: boolean
  onViewModeChange: (next: BrowserViewMode) => void
}

export function BrowserListHeader({
  profileCount,
  filteredProfileCount,
  runningCount,
  headerCollapsed,
  viewMode,
  proxies,
  cores,
  groups,
  allTags,
  filters,
  onFiltersChange,
  onToggleHeaderCollapsed,
  onRefresh,
  onOpenSettings,
  onOpenTrash,
  onImportProfiles,
  importingProfiles = false,
  onViewModeChange,
}: BrowserListHeaderProps) {
  const statItems = [
    { label: '总数', value: profileCount },
    { label: '运行', value: runningCount },
    { label: '停止', value: Math.max(0, profileCount - runningCount) },
  ]

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3 min-w-0">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例列表</h1>
          <div className="flex flex-wrap items-center gap-2">
            {statItems.map((item) => (
              <div
                key={item.label}
                className="flex h-8 items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-3 text-sm"
              >
                <span className="text-[var(--color-text-muted)]">{item.label}</span>
                <span className="font-semibold text-[var(--color-text-primary)]">{item.value}</span>
              </div>
            ))}
            {filteredProfileCount !== profileCount && (
              <div className="flex h-8 items-center gap-2 rounded-lg border border-[var(--color-accent)]/30 bg-[var(--color-accent)]/5 px-3 text-sm">
                <span className="text-[var(--color-text-muted)]">筛选</span>
                <span className="font-semibold text-[var(--color-accent)]">{filteredProfileCount}</span>
              </div>
            )}
          </div>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="secondary" size="sm" onClick={onToggleHeaderCollapsed}>
            {headerCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronUp className="w-4 h-4" />}
            {headerCollapsed ? '展开面板' : '收起面板'}
          </Button>
          <Button variant="secondary" size="sm" onClick={onRefresh}>
            <RefreshCw className="w-4 h-4" />刷新
          </Button>
          <Button variant="secondary" size="sm" onClick={onOpenSettings}>
            <Sliders className="w-4 h-4" />基础配置
          </Button>
          <Button variant="secondary" size="sm" onClick={onOpenTrash}>
            <Trash2 className="w-4 h-4" />回收站
          </Button>
          <Button variant="secondary" size="sm" onClick={onImportProfiles} loading={importingProfiles}>
            <Upload className="w-4 h-4" />导入实例
          </Button>
          <Link to="/settings">
            <Button variant="secondary" size="sm" title="打开全局备份与恢复">
              <Archive className="w-4 h-4" />全局备份
            </Button>
          </Link>
          <div className="flex items-center bg-[var(--color-bg-secondary)] rounded-md border border-[var(--color-border-default)] p-0.5 ml-2">
            <button
              className={`p-1.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors ${viewMode === 'card' ? 'bg-[var(--color-bg-surface)] shadow-sm text-[var(--color-accent)]' : ''}`}
              onClick={() => onViewModeChange('card')}
              title="卡片视图"
            >
              <LayoutGrid className="w-4 h-4" />
            </button>
            <button
              className={`p-1.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors ${viewMode === 'table' ? 'bg-[var(--color-bg-surface)] shadow-sm text-[var(--color-accent)]' : ''}`}
              onClick={() => onViewModeChange('table')}
              title="表格视图"
            >
              <List className="w-4 h-4" />
            </button>
          </div>
          <span className="w-px h-4 bg-[var(--color-border-muted)] mx-1 self-center"></span>
          <Link to="/browser/edit/new">
            <Button size="sm">
              <Play className="w-4 h-4" />新建配置
            </Button>
          </Link>
        </div>
      </div>
      {!headerCollapsed && (
        <InstanceFilterBar
          filters={filters}
          onChange={onFiltersChange}
          proxies={proxies}
          cores={cores}
          allTags={allTags}
          groups={groups}
        />
      )}
    </>
  )
}

interface BrowserListSettingsModalProps {
  open: boolean
  settings: BrowserSettings
  fingerprintText: string
  launchText: string
  startUrlsText: string
  savingSettings: boolean
  cores: BrowserCore[]
  onClose: () => void
  onSave: () => void
  onSettingsChange: (patch: Partial<BrowserSettings>) => void
  onFingerprintTextChange: (next: string) => void
  onLaunchTextChange: (next: string) => void
  onStartUrlsTextChange: (next: string) => void
  onAddCore: () => void
  onEditCore: (core: BrowserCore) => void
  onDeleteCore: (coreId: string) => void
  onSetDefaultCore: (coreId: string) => void
}

export function BrowserListSettingsModal({
  open,
  settings,
  fingerprintText,
  launchText,
  startUrlsText,
  savingSettings,
  cores,
  onClose,
  onSave,
  onSettingsChange,
  onFingerprintTextChange,
  onLaunchTextChange,
  onStartUrlsTextChange,
  onAddCore,
  onEditCore,
  onDeleteCore,
  onSetDefaultCore,
}: BrowserListSettingsModalProps) {
  const coreColumns: TableColumn<BrowserCore>[] = [
    { key: 'coreName', title: '名称' },
    { key: 'corePath', title: '路径' },
    {
      key: 'isDefault',
      title: '默认',
      render: (value) => (value ? <Star className="w-4 h-4 text-yellow-500 fill-yellow-500" /> : null),
    },
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      render: (_, record) => (
        <div className="flex justify-end gap-1">
          {!record.isDefault && (
            <Button size="sm" variant="ghost" onClick={() => onSetDefaultCore(record.coreId)} title="设为默认">
              <Star className="w-4 h-4" />
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={() => onEditCore(record)} title="编辑">
            <Edit2 className="w-4 h-4" />
          </Button>
          <Button size="sm" variant="ghost" onClick={() => onDeleteCore(record.coreId)} title="删除">
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="基础配置"
      width="700px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={onSave} loading={savingSettings}>保存</Button>
        </>
      }
    >
      <div className="space-y-6">
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-[var(--color-text-primary)]">内核管理</span>
            <div className="flex gap-2">
              <Button size="sm" onClick={onAddCore}>
                <Plus className="w-4 h-4" />新增内核
              </Button>
            </div>
          </div>
          <Card padding="none">
            <Table columns={coreColumns} data={cores} rowKey="coreId" />
          </Card>
        </div>

        <FormItem label="用户数据根目录">
          <Input
            value={settings.userDataRoot}
            onChange={(event) => onSettingsChange({ userDataRoot: event.target.value })}
            placeholder="data"
          />
        </FormItem>
        <FormItem label="默认指纹参数" hint="每行一个参数">
          <Textarea
            value={fingerprintText}
            onChange={(event) => onFingerprintTextChange(event.target.value)}
            rows={3}
            placeholder="--fingerprint-brand=Chrome"
          />
        </FormItem>
        <FormItem label="默认启动参数" hint="每行一个参数">
          <Textarea
            value={launchText}
            onChange={(event) => onLaunchTextChange(event.target.value)}
            rows={3}
            placeholder="--disable-sync"
          />
        </FormItem>
        <FormItem label="默认启动页面" hint="每行一个 URL，留空则启动时不再自动打开页面">
          <Textarea
            value={startUrlsText}
            onChange={(event) => onStartUrlsTextChange(event.target.value)}
            rows={4}
            placeholder="启动 URL"
          />
        </FormItem>
        <FormItem label="轻启动模式" hint="先起空白页，实例就绪后再打开默认页面">
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] px-3 py-2">
            <span className="text-sm text-[var(--color-text-primary)]">延后打开启动页</span>
            <Switch
              checked={settings.lightStartEnabled}
              onChange={(checked) => onSettingsChange({ lightStartEnabled: checked })}
            />
          </div>
        </FormItem>
        <FormItem label="默认恢复历史标签" hint="实例选择跟随内核时使用；不影响启动页和启动书签。实例未覆盖时，下次启动恢复之前的标签页和窗口。">
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] px-3 py-2">
            <div>
              <p className="text-sm text-[var(--color-text-primary)]">内核默认</p>
            </div>
            <Switch
              checked={settings.restoreLastSession}
              onChange={(checked) => onSettingsChange({ restoreLastSession: checked })}
            />
          </div>
        </FormItem>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-end">
          <FormItem label="启动就绪超时（毫秒）">
            <Input
              type="number"
              min={1000}
              step={500}
              value={settings.startReadyTimeoutMs}
              onChange={(event) =>
                onSettingsChange({
                  startReadyTimeoutMs: Math.max(1000, Number(event.target.value) || 3000),
                })
              }
              placeholder="3000"
            />
          </FormItem>
          <FormItem label="启动稳定窗口（毫秒）">
            <Input
              type="number"
              min={0}
              step={100}
              value={settings.startStableWindowMs}
              onChange={(event) =>
                onSettingsChange({
                  startStableWindowMs: Math.max(0, Number(event.target.value) || 1200),
                })
              }
              placeholder="1200"
            />
          </FormItem>
        </div>
      </div>
    </Modal>
  )
}

interface BrowserCoreEditorModalProps {
  open: boolean
  coreForm: BrowserCoreInput
  coreValidation: { valid: boolean; message: string } | null
  savingCore: boolean
  onClose: () => void
  onSave: () => void
  onValidate: () => void
  onCoreFormChange: (patch: Partial<BrowserCoreInput>) => void
}

export function BrowserCoreEditorModal({
  open,
  coreForm,
  coreValidation,
  savingCore,
  onClose,
  onSave,
  onValidate,
  onCoreFormChange,
}: BrowserCoreEditorModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={coreForm.coreId ? '编辑内核' : '新增内核'}
      width="500px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={onSave} loading={savingCore}>保存</Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="内核名称" required>
          <Input
            value={coreForm.coreName}
            onChange={(event) => onCoreFormChange({ coreName: event.target.value })}
            placeholder="Chrome 142"
          />
        </FormItem>
        <FormItem label="内核路径" required>
          <div className="flex gap-2">
            <Input
              value={coreForm.corePath}
              onChange={(event) => onCoreFormChange({ corePath: event.target.value })}
              placeholder="chrome 或 D:/browsers/chrome-120"
              className="flex-1"
            />
            <Button variant="secondary" onClick={onValidate}>验证</Button>
          </div>
          {coreValidation && (
            <div className={`flex items-center gap-1 mt-1 text-sm ${coreValidation.valid ? 'text-green-600' : 'text-red-600'}`}>
              {coreValidation.valid ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
              {coreValidation.message}
            </div>
          )}
        </FormItem>
      </div>
    </Modal>
  )
}
