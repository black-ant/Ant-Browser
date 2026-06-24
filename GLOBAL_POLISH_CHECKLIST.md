# 全局 Polish 检查清单

## 目标
让界面从"能用"变成"舒服"

## 检查维度

### 1. 状态完整性 ✓

#### Loading 状态
**已完成的页面**：
- ✓ BrowserListPage - 列表加载、批量操作
- ✓ BrowserDevToolsPageNew_v2 - 连接状态、轮询加载
- ✓ DashboardPage_v2 - 初始加载
- ✓ AccountManagementPage_v2 - 列表加载
- ✓ ProxyPoolPage - 测速中、IP检测中

**检查点**：
- [ ] 所有异步操作都有 loading 指示
- [ ] Loading 期间按钮 disabled
- [ ] Loading 使用统一样式（spinner/skeleton/pulse）

#### Empty 状态
**已完成的页面**：
- ✓ BrowserListPage - "暂无浏览器实例"
- ✓ DashboardPage_v2 - "暂无活动记录"/"暂无错误"
- ✓ AccountManagementPage_v2 - "暂无平台账号"
- ✓ DevTools NetworkPanel - "暂无请求"
- ✓ DevTools ConsolePanel - "暂无日志"

**优化建议**：
```tsx
// 统一的空状态组件
<div className="py-12 text-center">
  <Icon className="w-12 h-12 mx-auto mb-3 text-[var(--color-text-muted)]" />
  <p className="text-sm text-[var(--color-text-muted)] mb-4">
    {emptyMessage}
  </p>
  {actionButton && <Button size="sm">{actionButton}</Button>}
</div>
```

#### Error 状态
**检查点**：
- [ ] 网络错误有友好提示
- [ ] 数据加载失败有重试按钮
- [ ] 表单验证错误清晰显示
- [ ] 错误信息不暴露技术细节

**优化建议**：
```tsx
// 统一的错误处理
catch (error: any) {
  const message = error?.message || error?.toString() || '操作失败'
  toast.error(message.includes('fetch') ? '网络请求失败' : message)
}
```

#### Disabled 状态
**已完成**：
- ✓ 内置代理不可删除
- ✓ 直连代理不可编辑
- ✓ Loading 时按钮 disabled
- ✓ 表单未填写时保存按钮 disabled（部分页面）

**检查点**：
- [ ] Disabled 按钮有 title 说明原因
- [ ] Disabled 样式明显（opacity-50/cursor-not-allowed）
- [ ] Disabled 时不触发 hover 效果

### 2. 响应式布局 📱

#### 文字溢出检查
**关键页面**：
- BrowserListPage - 实例名称、代理配置、指纹参数
- ProxyPoolPage - 代理配置、IP地址、错误信息
- AccountManagementPage - 邮箱、Cookie 详情
- DevTools - URL、Headers、响应内容

**常见问题**：
```tsx
// ❌ 问题：长文本撑破布局
<div className="flex">
  <div>{longText}</div>
</div>

// ✓ 解决：截断 + tooltip
<div className="flex">
  <div className="truncate max-w-0" title={longText}>{longText}</div>
</div>

// ✓ 解决：自动换行
<div className="break-words">{longText}</div>
```

**检查点**：
- [ ] 实例名称：truncate + title
- [ ] 代理配置：truncate + title
- [ ] URL：truncate + title
- [ ] 错误信息：break-words（不截断）
- [ ] Cookie 值：truncate + max-w-xs + title

#### 移动端适配
**当前状态**：
- ✓ 使用 responsive grid（grid-cols-2 lg:grid-cols-4）
- ✓ 使用 flex-wrap
- ✓ 最小宽度限制（min-w-0）

**需要检查**：
- [ ] 表格在小屏下的表现（<768px）
- [ ] 按钮组在小屏下是否换行
- [ ] Modal 在小屏下的宽度
- [ ] 侧边栏在小屏下的处理

**优化建议**：
```tsx
// 响应式表格
<div className="overflow-x-auto">
  <table className="min-w-[800px]">
    {/* 内容 */}
  </table>
</div>

// 响应式按钮组
<div className="flex flex-wrap gap-2">
  <Button>操作1</Button>
  <Button>操作2</Button>
</div>
```

### 3. 暗色主题对比度 🌙

#### 当前使用的颜色变量
```css
--color-text-primary
--color-text-secondary
--color-text-muted
--color-bg-card
--color-bg-elevated
--color-bg-muted
--color-border-default
```

**检查点**：
- [ ] 主要文字对比度 ≥ 4.5:1
- [ ] 次要文字对比度 ≥ 3:1
- [ ] 边框在暗色下可见
- [ ] Hover 状态在暗色下明显
- [ ] Badge 在暗色下清晰

**常见问题**：
```tsx
// ❌ 硬编码颜色（暗色主题下对比度低）
<div className="text-gray-500 bg-white">

// ✓ 使用 CSS 变量
<div className="text-[var(--color-text-secondary)] bg-[var(--color-bg-card)]">
```

**需要测试的场景**：
1. 切换到暗色主题
2. 检查所有页面的文字可读性
3. 检查 Badge/Button 的对比度
4. 检查 hover/active 状态

### 4. 危险操作确认 ⚠️

#### 已实现的确认
**删除操作**：
- ✓ BrowserListPage - 删除实例（单个/批量）
- ✓ AccountManagementPage_v2 - 删除账号
- ✓ ProxyPoolPage - 删除代理
- ✓ BrowserEditPage - 清空启动参数

**清空操作**：
- ✓ DevTools - 清空网络请求/控制台/Cookie
- ✓ AccountManagementPage - 清空所有账号

**检查点**：
- [x] 删除实例有确认
- [x] 批量删除有确认
- [x] 清空 Cookie 有确认（危险标记）
- [ ] 停止所有实例有确认
- [ ] 清空所有代理有确认
- [ ] 重置配置有确认

**确认 Modal 规范**：
```tsx
// 标准确认
await confirm({
  title: '确认删除',
  content: '删除后无法恢复，确定要删除吗？',
  confirmText: '删除',
  cancelText: '取消',
  danger: true, // 危险操作：红色按钮
})

// 带详情的确认
await confirm({
  title: '批量删除',
  content: `将删除 ${count} 个实例，此操作无法撤销。`,
  confirmText: '删除',
  danger: true,
})
```

### 5. 图标和 Tooltip 💡

#### 已有图标的操作
**页面级操作**：
- ✓ 添加（Plus）
- ✓ 刷新（RefreshCw）
- ✓ 导入（Upload）
- ✓ 导出（Download）
- ✓ 搜索（Search）

**行级操作**：
- ✓ 编辑（Edit2）
- ✓ 删除（Trash2）
- ✓ 启动（Play）
- ✓ 停止（Square）
- ✓ Cookie（Cookie）

#### 需要添加 tooltip 的场景
```tsx
// ❌ 没有提示（用户不知道这个按钮干什么）
<Button size="sm" variant="ghost">
  <Settings className="w-4 h-4" />
</Button>

// ✓ 有 tooltip
<Button size="sm" variant="ghost" title="打开设置">
  <Settings className="w-4 h-4" />
</Button>

// ✓ 或使用 Tooltip 组件（如果有）
<Tooltip content="打开设置">
  <Button size="sm" variant="ghost">
    <Settings className="w-4 h-4" />
  </Button>
</Tooltip>
```

**检查点**：
- [ ] 所有图标按钮都有 title
- [ ] DevTools 工具 tabs 有 title
- [ ] 复杂操作有说明文字
- [ ] Disabled 按钮说明原因

### 6. Toast 文案优化 📢

#### 当前文案模式
**成功**：
- "实例已启动"
- "代理已保存"
- "Cookie 已保存"

**错误**：
- error?.message || "操作失败"

**优化原则**：
1. **简短**：3-8字为佳
2. **明确**：说清楚什么成功/失败
3. **友好**：不暴露技术细节
4. **一致**：同类操作用相同模板

**优化建议**：
```tsx
// ✓ 好的文案
toast.success('已启动')
toast.success('已保存')
toast.error('启动失败')
toast.error('网络错误')

// ❌ 不好的文案
toast.success('实例已经成功启动了')
toast.error('Error: fetch failed at line 123')
toast.error('操作失败，请重试') // 太模糊
```

**分类模板**：
- 启动：`已启动` / `启动失败`
- 停止：`已停止` / `停止失败`
- 保存：`已保存` / `保存失败`
- 删除：`已删除` / `删除失败`
- 导入：`已导入 ${count} 项` / `导入失败`
- 导出：`已导出` / `导出失败`
- 测速：`测速完成` / `测速失败`
- 刷新：`已刷新` / `刷新失败`

## 验收标准

### 视觉验收
- [ ] **没有明显文字重叠**
  - 检查所有表格列
  - 检查所有 Badge 文字
  - 检查所有 Modal 标题

- [ ] **没有按钮内容挤出**
  - 检查图标按钮（固定宽度）
  - 检查文字按钮（padding 足够）
  - 检查 Loading 按钮（spinner 不挤出）

- [ ] **没有低对比度文字**
  - 切换暗色主题测试
  - 检查 disabled 状态
  - 检查 muted 文字

### 功能验收
- [ ] **空数据页面不显得坏掉**
  - 有友好的空状态提示
  - 有明确的操作引导
  - 图标和文案清晰

- [ ] **复杂页面仍然能快速扫描**
  - 关键信息突出（大字号/颜色）
  - 次要信息弱化（小字号/灰色）
  - 分组清晰（间距/边框）
  - 操作位置一致（右上角/行末）

## 实施计划

### Phase 1: 状态完整性（P0）
1. 补齐所有 loading 状态
2. 统一 empty 状态样式
3. 优化 error 处理
4. 检查 disabled 状态

### Phase 2: 响应式（P1）
1. 检查文字溢出
2. 添加 truncate + title
3. 测试移动端布局
4. 优化表格滚动

### Phase 3: 主题对比度（P1）
1. 切换暗色主题
2. 检查文字对比度
3. 检查边框可见性
4. 优化 hover 状态

### Phase 4: 交互优化（P2）
1. 危险操作加确认
2. 图标按钮加 tooltip
3. 优化 Toast 文案
4. 统一操作反馈

## 快速检查方法

### 1. 文字溢出检查
```bash
# 搜索可能有问题的模式
grep -r "className.*flex.*\"" frontend/src --include="*.tsx" | grep -v "truncate"
grep -r "title=" frontend/src --include="*.tsx" | wc -l
```

### 2. 硬编码颜色检查
```bash
# 搜索硬编码的颜色（应该使用 CSS 变量）
grep -r "text-gray-\|bg-white\|bg-gray-" frontend/src --include="*.tsx" | grep -v "var(--color"
```

### 3. 确认 Modal 检查
```bash
# 搜索删除操作（应该有确认）
grep -r "handleDelete\|onDelete" frontend/src --include="*.tsx" | grep -v "confirm"
```

### 4. Toast 文案检查
```bash
# 统计 Toast 使用
grep -r "toast\." frontend/src --include="*.tsx" | wc -l
grep -r "toast.error\|toast.success" frontend/src --include="*.tsx"
```

## 已优化的页面总结

### 状态完整 ✓
- BrowserListPage
- BrowserDevToolsPageNew_v2
- DashboardPage_v2
- AccountManagementPage_v2
- ProxyPoolPage

### 响应式处理 ✓
- 使用 truncate + title
- 使用 break-words
- 使用 min-w-0
- 使用 responsive grid

### 视觉一致 ✓
- 统一 Card 组件
- 统一 Badge variant
- 统一按钮样式
- 统一间距规范

## 下一步

1. 创建统一的空状态组件（EmptyState）
2. 创建统一的错误状态组件（ErrorState）
3. 创建统一的 Loading 组件（LoadingState）
4. 检查所有页面的暗色主题表现
5. 补齐缺失的确认 Modal
6. 统一 Toast 文案模板
