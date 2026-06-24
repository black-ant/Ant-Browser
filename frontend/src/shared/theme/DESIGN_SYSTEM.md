# Ant Browser 设计系统

> 专业工作台风格指南 - 为效率而设计

## 设计原则

### 1. 专业工作台，非营销页
- 信息密度适中，留白适度
- 优先展示数据和操作
- 避免过度装饰和大面积色块
- 使用工具栏、表格、分栏、抽屉等工作台组件

### 2. 层次清晰
- 使用阴影、边框、背景色表达层级
- 不使用卡片套卡片
- 保持视觉呼吸感

### 3. 状态可见
- 使用 lucide-react 图标表达操作
- 使用 Badge 表达状态
- 加载、空状态、错误状态都有明确反馈

## 颜色系统

### 浅色主题
```css
背景：
- base: #f8fafc      /* 页面底色 */
- surface: #ffffff   /* 卡片/面板 */
- elevated: #ffffff  /* 悬浮元素 */
- muted: #f1f5f9     /* 悬停背景 */

边框：
- default: #e2e8f0   /* 默认边框 */
- muted: #f1f5f9     /* 微弱边框 */
- strong: #cbd5e1    /* 强调边框 */

文字：
- primary: #1e293b   /* 主要文字 */
- secondary: #475569 /* 次要文字 */
- muted: #94a3b8     /* 微弱文字 */

强调色：
- accent: #1e293b    /* 主强调色 */
```

### 深色主题
```css
背景：
- base: #0a0a0b      /* 页面底色 */
- surface: #141416   /* 卡片/面板 */
- elevated: #1c1c1f  /* 悬浮元素 */
- muted: #232327     /* 悬停背景 */

边框：
- default: #2a2a2e   /* 默认边框 */
- muted: #1f1f23     /* 微弱边框 */
- strong: #3f3f46    /* 强调边框 */

文字：
- primary: #e8e8ea   /* 主要文字 */
- secondary: #9d9da3 /* 次要文字 */
- muted: #6b6b70     /* 微弱文字 */

强调色：
- accent: #e8e8ea    /* 主强调色 */
```

### 状态色（通用）
```css
- success: #10b981 / #22c55e  /* 成功/运行中 */
- warning: #f59e0b            /* 警告 */
- error: #ef4444              /* 错误/失败 */
- info: #3b82f6 / #06b6d4     /* 信息 */
```

## 组件规范

### Button（按钮）

**变体：**
- `primary` - 主操作（渐变背景，hover 上移）
- `secondary` - 次要操作（边框，背景色）
- `outline` - 轮廓按钮（透明背景，彩色边框）
- `danger` - 危险操作（红色渐变）
- `ghost` - 幽灵按钮（无背景，hover 显示）

**尺寸：**
- `sm` - h-8, 紧凑工具栏
- `md` - h-10, 标准按钮
- `lg` - h-11, 强调操作

**使用原则：**
- 一个区域最多一个 primary 按钮
- 危险操作必须二次确认
- loading 状态使用内置 spinner
- 图标 + 文字更清晰

```tsx
<Button variant="primary" size="md">
  <Play className="w-4 h-4" />
  启动实例
</Button>
```

### Badge（徽标）

**变体：**
- `default` - 灰色，中性信息
- `success` - 绿色，成功/运行中
- `error` - 红色，错误/失败
- `warning` - 橙色，警告
- `info` - 蓝色，信息提示

**特性：**
- `dot` - 显示状态点
- `pulse` - 脉冲动画（运行中状态）
- `outline` - 轮廓样式（降低视觉权重）

**使用原则：**
- 状态徽标使用 dot + pulse
- 标签使用 outline 降低干扰
- 避免大量相同颜色堆叠

```tsx
<Badge variant="success" dot pulse>运行中</Badge>
<Badge variant="default" outline>标签</Badge>
```

### Table（表格）

**特性：**
- 圆角边框
- 渐变表头
- hover 行高亮
- 粘性表头
- 排序支持
- 优雅的空状态

**使用原则：**
- 数据密集场景的首选
- 列宽合理分配
- 操作列右对齐
- 状态使用 Badge 表达

```tsx
<Table
  columns={columns}
  data={data}
  rowKey="id"
  stickyHeader
  onSort={handleSort}
/>
```

### Modal（弹窗）

**特性：**
- 2xl 圆角
- 渐变标题栏
- 模糊遮罩
- 缩放动画
- 滚动内容区

**使用原则：**
- 宽度适中（500-800px）
- 标题简洁明确
- footer 按钮右对齐
- 复杂表单考虑使用抽屉

```tsx
<Modal
  open={open}
  onClose={onClose}
  title="编辑配置"
  width="600px"
>
  {/* 内容 */}
</Modal>
```

### Form（表单）

**组件：**
- `FormItem` - 表单项容器（label + 输入 + 错误提示）
- `Input` - 输入框
- `Select` - 下拉选择
- `Textarea` - 多行输入
- `Switch` - 开关

**特性：**
- 2px 边框
- focus ring 效果
- 错误状态带图标
- 合理的高度和间距

**使用原则：**
- label 简短清晰
- 必填项标记 *
- 实时或失焦验证
- 错误信息具体明确

```tsx
<FormItem label="实例名称" error={errors.name}>
  <Input
    value={name}
    onChange={setName}
    placeholder="请输入实例名称"
  />
</FormItem>
```

### Toast（提示）

**类型：**
- `success` - 成功提示
- `error` - 错误提示
- `warning` - 警告提示
- `info` - 信息提示

**特性：**
- xl 圆角
- 图标背景容器
- 增强阴影
- 自动消失

**使用原则：**
- 操作反馈必有提示
- 成功：简短确认
- 错误：具体原因
- 避免频繁弹出

```tsx
toast.success('实例启动成功')
toast.error('启动失败：端口被占用')
```

### Loading（加载）

**组件：**
- `Loading` - 旋转 spinner
- `Skeleton` - 骨架屏
- `DotsLoading` - 点状加载

**使用原则：**
- 数据加载用 Loading
- 列表加载用 Skeleton
- 按钮内用 DotsLoading
- fullscreen 用于整页加载

```tsx
<Loading size="lg" text="加载中..." />
<Skeleton width="100%" height="40px" />
```

### Empty（空状态）

**组件：**
- `Empty` - 基础空状态
- `EmptyList` - 列表空状态
- `EmptySearch` - 搜索无结果
- `EmptyError` - 加载失败

**使用原则：**
- 提供友好的提示
- 给出下一步操作
- 搜索无结果提供重置

```tsx
<EmptyList
  title="暂无实例"
  description="点击右上角按钮创建第一个实例"
  action={<Button>创建实例</Button>}
/>
```

## 间距系统

```css
xs: 0.25rem  /* 4px  - 紧密元素 */
sm: 0.5rem   /* 8px  - 相关元素 */
md: 1rem     /* 16px - 标准间距 */
lg: 1.5rem   /* 24px - 分组间距 */
xl: 2rem     /* 32px - 区块间距 */
```

## 圆角系统

```css
sm: 0.375rem  /* 6px  - 小元素 */
md: 0.5rem    /* 8px  - 标准圆角 */
lg: 0.75rem   /* 12px - 卡片 */
xl: 1rem      /* 16px - 大卡片/按钮 */
2xl: 1.5rem   /* 24px - 弹窗 */
```

## 阴影系统

```css
sm: 轻微阴影  /* hover 状态 */
md: 标准阴影  /* 卡片 */
lg: 强调阴影  /* 弹窗、悬浮 */
```

## 动画原则

### 时长
- 快速：150ms - hover、focus
- 标准：200ms - 过渡、展开
- 缓慢：300ms - 弹窗、页面切换

### 缓动
- `ease-out` - 进入动画
- `ease-in-out` - 过渡动画
- `ease-in` - 退出动画

### 常用动画
- `hover:-translate-y-0.5` - 按钮 hover 上移
- `animate-spin` - loading
- `animate-pulse` - 状态脉冲
- `animate-fade-in` - 淡入
- `animate-scale-in` - 缩放进入

## 响应式原则

### 断点（使用 Tailwind 默认）
```css
sm: 640px   /* 平板 */
md: 768px   /* 小屏幕桌面 */
lg: 1024px  /* 标准桌面 */
xl: 1280px  /* 大屏幕 */
```

### 自适应策略
- 工具栏响应式收起
- 表格横向滚动
- 弹窗宽度自适应
- 间距随屏幕缩放

## 无障碍

### 必须遵循
- 语义化 HTML
- 键盘可操作
- focus 状态明确
- ARIA 属性完整
- 颜色对比度 4.5:1

### 具体实践
- 按钮必须有文字或 aria-label
- 输入框关联 label
- 图标按钮提供工具提示
- 表单错误可读屏
- 模态框捕获焦点

## 常见反模式（避免）

❌ **卡片套卡片**
```tsx
// 错误
<Card>
  <Card>内容</Card>
</Card>
```

❌ **文字溢出**
```tsx
// 错误：没有处理长文本
<div>{longText}</div>

// 正确
<div className="truncate">{longText}</div>
<div className="line-clamp-2">{longText}</div>
```

❌ **按钮文字挤压**
```tsx
// 错误：padding 不足
<button className="px-1">文字</button>

// 正确：使用 Button 组件
<Button size="md">文字</Button>
```

❌ **大面积单色**
```tsx
// 错误：整页蓝色
<div className="bg-blue-500 min-h-screen">

// 正确：使用主题色
<div className="bg-[var(--color-bg-base)] min-h-screen">
```

❌ **状态不明确**
```tsx
// 错误：没有反馈
<button onClick={save}>保存</button>

// 正确
<Button loading={saving} onClick={save}>
  保存
</Button>
```

## 工作台页面布局

### 典型结构
```tsx
<div className="flex flex-col h-screen">
  {/* 顶部工具栏 */}
  <header className="border-b bg-[var(--color-bg-surface)]">
    <div className="flex items-center justify-between px-6 py-3">
      <div className="flex items-center gap-4">
        <h1>页面标题</h1>
        <Badge>状态</Badge>
      </div>
      <div className="flex items-center gap-2">
        <Button variant="secondary">次要操作</Button>
        <Button variant="primary">主要操作</Button>
      </div>
    </div>
  </header>

  {/* 筛选栏（可选）*/}
  <div className="border-b bg-[var(--color-bg-surface)] px-6 py-3">
    {/* 筛选器 */}
  </div>

  {/* 主内容区 */}
  <main className="flex-1 overflow-auto p-6">
    {/* 表格或内容 */}
  </main>
</div>
```

### 分栏布局
```tsx
<div className="flex h-screen">
  {/* 侧边栏 */}
  <aside className="w-64 border-r bg-[var(--color-bg-surface)]">
    {/* 导航或筛选 */}
  </aside>

  {/* 主区域 */}
  <main className="flex-1 overflow-auto">
    {/* 内容 */}
  </main>
</div>
```

## 主题切换

确保所有组件使用 CSS 变量：
- `var(--color-bg-base)`
- `var(--color-text-primary)`
- `var(--color-border-default)`
- 等等

避免硬编码颜色值，确保主题切换无缝。

## 最后

这是一个活文档，随项目演进持续更新。设计系统的目标是：
1. 提高开发效率
2. 保证体验一致
3. 降低维护成本

有疑问时，参考现有组件实现。
