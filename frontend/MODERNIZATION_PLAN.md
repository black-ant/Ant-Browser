# BrowserListPage 现代化改造计划

## 需要添加的导入
```typescript
import { StatusOverview } from './browser-list/StatusOverview'
import { EnhancedToolbar } from './browser-list/EnhancedToolbar'
import { EnhancedBatchToolbar } from './browser-list/EnhancedBatchToolbar'
import { createEnhancedColumns } from './browser-list/enhancedColumns'
```

## 需要添加的状态
```typescript
const [searchQuery, setSearchQuery] = useState('')
const [filterPanelOpen, setFilterPanelOpen] = useState(false)
const [refreshing, setRefreshing] = useState(false)
```

## 计算状态统计
```typescript
const statusStats = useMemo(() => {
  const starting = Array.from(startingIds).length
  const error = profiles.filter(p => p.lastError).length
  const proxyError = profiles.filter(p => p.runtimeWarning && p.runtimeWarning.includes('代理')).length
  
  return {
    total: profiles.length,
    running: runningCount,
    starting,
    error,
    proxyError,
  }
}, [profiles, runningCount, startingIds])
```

## 增强的搜索筛选
```typescript
const filteredProfiles = useMemo(() => {
  let result = profiles
  
  // 搜索
  if (searchQuery.trim()) {
    const query = searchQuery.toLowerCase()
    result = result.filter(p => 
      p.profileName.toLowerCase().includes(query) ||
      p.tags?.some(tag => tag.toLowerCase().includes(query)) ||
      p.keywords?.some(kw => kw.toLowerCase().includes(query))
    )
  }
  
  // 原有筛选逻辑...
  
  return result
}, [profiles, searchQuery, filters])
```

## 页面布局改造

### 1. 移除旧的页头，使用工作台布局
```tsx
<div className="flex flex-col h-screen bg-[var(--color-bg-base)]">
  {/* 顶部标题栏 */}
  <header className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
    <div className="flex items-center justify-between">
      <div>
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例管理</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-0.5">
          共 {profiles.length} 个实例
          {filteredProfiles.length !== profiles.length && (
            <span className="text-[var(--color-accent)]"> · 筛选后 {filteredProfiles.length} 个</span>
          )}
        </p>
      </div>
      <div className="flex items-center gap-2">
        <Button variant="secondary" size="sm" onClick={handleOpenSettings}>
          <Sliders className="w-4 h-4" />
          基础配置
        </Button>
        <Button 
          variant="secondary" 
          size="sm"
          onClick={() => { setCdKey(''); setExpandModalOpen(true); loadQuota() }}
        >
          <Gift className="w-4 h-4" />
          扩容
        </Button>
      </div>
    </div>
  </header>

  {/* 状态概览 */}
  <div className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
    <StatusOverview {...statusStats} />
  </div>

  {/* 工具栏 */}
  <EnhancedToolbar
    searchQuery={searchQuery}
    onSearchChange={setSearchQuery}
    onRefresh={handleRefresh}
    viewMode={viewMode}
    onViewModeChange={setViewMode}
    filterActive={filterPanelOpen}
    onToggleFilter={() => setFilterPanelOpen(prev => !prev)}
    refreshing={refreshing}
  />

  {/* 筛选面板（可折叠）*/}
  {filterPanelOpen && (
    <div className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
      <InstanceFilterBar
        filters={filters}
        onChange={setFilters}
        proxies={proxies}
        cores={cores}
        allTags={allTags}
        groups={groups}
      />
    </div>
  )}

  {/* 批量操作工具栏 */}
  {selectedIds.size > 0 && (
    <div className="flex-shrink-0 px-6 py-3 bg-[var(--color-bg-base)]">
      <EnhancedBatchToolbar
        selectedCount={selectedIds.size}
        totalCount={filteredProfiles.length}
        onSelectAll={handleSelectAll}
        onDeselectAll={handleDeselectAll}
        onBatchStart={handleBatchStart}
        onBatchStop={handleBatchStop}
        onBatchDelete={handleBatchDelete}
        batchLoading={batchLoading}
      />
    </div>
  )}

  {/* 主内容区 - 表格 */}
  <main className="flex-1 overflow-hidden">
    <div className="h-full overflow-auto px-6 py-4">
      {loading ? (
        <div className="flex items-center justify-center h-full">
          <Loading size="lg" text="加载中..." />
        </div>
      ) : filteredProfiles.length === 0 ? (
        <Empty
          title={profiles.length === 0 ? '还没有实例' : '未找到匹配的实例'}
          description={profiles.length === 0 ? '创建第一个浏览器实例开始' : '尝试调整搜索或筛选条件'}
          action={
            profiles.length === 0 ? (
              <Link to="/browser/create">
                <Button><Plus className="w-4 h-4" />新建实例</Button>
              </Link>
            ) : (
              <Button variant="secondary" onClick={() => { setSearchQuery(''); setFilters(EMPTY_FILTERS) }}>
                <X className="w-4 h-4" />清空筛选
              </Button>
            )
          }
        />
      ) : (
        <div className="bg-[var(--color-bg-surface)] rounded-xl border border-[var(--color-border-default)] overflow-hidden">
          <Table
            columns={enhancedColumns}
            data={filteredProfiles}
            rowKey="profileId"
            stickyHeader
            maxHeight="calc(100vh - 400px)"
          />
        </div>
      )}
    </div>
  </main>
</div>
```

## 需要创建的增强列配置
```typescript
const enhancedColumns = useMemo(() => createEnhancedColumns({
  proxies,
  getProfileCoreLabel,
  getProfileStatus,
  isProfileStarting,
  isProfileStopping,
  isProfileBusy,
  onStart: handleStart,
  onStop: handleStop,
  onRestart: handleRestart,
  onKeywords: openKwModal,
  onCopy: openCopyModal,
  onDelete: handleDelete,
  onRefresh: loadProfiles,
  onToggleSelect: toggleSelect,
  selectedIds,
  onSelectAll: handleSelectAll,
  onDeselectAll: handleDeselectAll,
  totalCount: filteredProfiles.length,
}), [proxies, selectedIds, filteredProfiles.length, startingIds, stoppingIds])
```

## handleRefresh 函数
```typescript
const handleRefresh = async () => {
  setRefreshing(true)
  try {
    await loadProfiles()
  } finally {
    setRefreshing(false)
  }
}
```
