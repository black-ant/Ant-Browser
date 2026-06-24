# 智能指纹生成功能

## 概述

智能指纹生成器根据真实世界的浏览器、硬件、地区统计数据，自动生成合理且可信的指纹配置，避免手动配置的繁琐和不合理性。

## 功能特点

### 🎯 基于真实统计数据

所有配置都基于真实世界的市场份额和使用统计：

- **浏览器品牌分布**：Chrome 65%、Edge 20%、Firefox 10%、Safari 5%（全球平均）
- **操作系统分布**：Windows 75%、macOS 20%、Linux 5%
- **分辨率分布**：1920x1080 (45%)、1366x768 (18%)、2560x1440 (12%) 等
- **硬件配置**：从低配（4核4GB）到高配（16核32GB）的合理分布

### 🌍 地区自适应

根据代理 IP 的地理位置自动调整配置：

**中国（CN）**:
- 浏览器：Chrome 60%、Edge 30%、Firefox 8%、Safari 2%
- 语言：zh-CN
- 时区：Asia/Shanghai、Asia/Urumqi
- 字体：Arial, Microsoft YaHei, SimSun, SimHei...

**美国（US）**:
- 浏览器：Chrome 65%、Safari 20%、Edge 10%、Firefox 5%
- 语言：en-US
- 时区：America/New_York、America/Los_Angeles 等
- 字体：Arial, Helvetica, Times New Roman...

**日本（JP）**:
- 浏览器：Chrome 55%、Safari 25%、Edge 15%、Firefox 5%
- 语言：ja-JP
- 时区：Asia/Tokyo
- 字体：Arial, MS Gothic, Meiryo, Yu Gothic...

支持的国家/地区：CN、US、JP、GB、FR、DE、KR、SG、AU、CA 等

### 🎮 场景化配置

提供四种预设场景，自动调整硬件配置权重：

#### 1. 🎲 随机生成（默认）
- 按真实市场分布随机选择
- 各种配置均衡分布
- 适合大多数使用场景

#### 2. 💼 办公场景
- 偏向中低配硬件
  - 4核4GB (30%)
  - 4核8GB (30%)
  - 8核8GB (30%)
  - 高配 (10%)
- 常见办公分辨率：1920x1080、1366x768
- Do Not Track 关闭（企业环境常见）

#### 3. 🏠 家用场景
- 均衡的硬件配置
- 标准市场分布
- 适合个人用户

#### 4. 🎮 游戏场景
- 偏向高配硬件
  - 8核16GB (40%)
  - 16核16GB (20%)
  - 12核32GB (10%)
  - 其他 (30%)
- 高分辨率优先：2560x1440 权重翻倍
- 30 位色深（HDR）概率提升

## 使用方法

### 在 UI 中使用

1. **打开指纹配置面板**
   - 新建或编辑 profile
   - 进入"指纹参数"标签

2. **展开智能生成**
   - 找到紫色渐变的"智能生成指纹"卡片
   - 点击"展开"

3. **选择场景**
   - 🎲 随机生成
   - 💼 办公场景（中低配）
   - 🏠 家用场景（均衡）
   - 🎮 游戏场景（高配）

4. **点击"立即生成"**
   - 系统会自动生成完整的指纹配置
   - 包括：浏览器品牌、平台、分辨率、CPU核心、内存、字体、时区、语言等

5. **查看生成结果**
   - 所有字段会自动填充
   - 指纹种子自动生成（确保唯一性）
   - WebGL 策略默认为"自动混淆（推荐）"

### 编程方式使用

```typescript
import { generateSmartFingerprint, generateBatchFingerprints } from './fingerprintGenerator'

// 生成单个指纹
const fingerprint = generateSmartFingerprint({
  proxyCountry: 'US',           // 代理国家代码
  proxyTimezone: 'America/New_York',  // 可选：代理时区
  scenario: 'office',           // 场景：office, home, gaming, random
})

// 批量生成（确保多样性）
const fingerprints = generateBatchFingerprints(10, {
  proxyCountry: 'CN',
  scenario: 'random',
})

// 生成相似指纹（克隆场景）
const similar = generateSimilarFingerprint(existingConfig)
```

## 生成逻辑

### 1. 平台选择（按权重）
```
Windows: 75%
macOS: 20%
Linux: 5%
```

### 2. 浏览器品牌选择（按地区和权重）
根据 `proxyCountry` 选择对应地区的浏览器分布。

### 3. 语言和时区
- **优先级**：`proxyTimezone` > 国家时区映射 > 留空（后端自动推导）
- **语言**：根据国家代码映射
- **字体**：根据平台和地区组合选择

### 4. 硬件配置
根据场景调整权重后随机选择：

| 配置 | CPU | 内存 | 类型 | 默认权重 |
|------|-----|------|------|----------|
| 低配 1 | 4 核 | 4 GB | low | 15% |
| 中配 1 | 4 核 | 8 GB | medium | 20% |
| 中配 2 | 8 核 | 8 GB | medium | 30% |
| 高配 1 | 8 核 | 16 GB | high | 20% |
| 高配 2 | 16 核 | 16 GB | high | 10% |
| 顶配 | 12 核 | 32 GB | high | 5% |

**办公场景**：低配权重 ×2、中配权重 ×1.5、高配权重 ×0.5
**游戏场景**：高配权重 ×2、中配权重 ×1、低配权重 ×0.3

### 5. 分辨率
```
1920x1080: 45%
1366x768: 18%
2560x1440: 12%
1440x900: 10%
1536x864: 8%
1600x900: 7%
```

**游戏场景**：2560x1440 权重翻倍

### 6. 其他参数
- **色深**：24 位 (90%) / 30 位 (10%)
- **触摸点数**：0（桌面设备）
- **Do Not Track**：30% 概率启用
- **WebRTC 策略**：`disable_non_proxied_udp`（默认）

### 7. 指纹种子
每次生成都会创建唯一的 32 位随机整数作为种子，确保指纹唯一性。

## 优势

### ✅ 真实可信
- 基于真实市场统计数据
- 配置组合符合常见硬件规格
- 字体列表匹配平台和地区

### ✅ 智能适配
- 自动根据代理地理位置调整
- 场景化配置符合使用习惯
- 硬件配置合理匹配

### ✅ 避免穿帮
- 时区与地区一致
- 语言与地区一致
- 字体与平台/地区一致
- 硬件配置合理（不会出现 4 核 32GB 等异常组合）

### ✅ 批量友好
- 支持批量生成
- 自动确保种子唯一性
- 可生成相似但不同的配置

### ✅ 省时省力
- 一键生成完整配置
- 无需手动选择每个参数
- 自动处理复杂的关联关系

## 与其他功能的关系

### 与预设配置对比

| 特性 | 预设配置 | 智能生成 |
|------|----------|----------|
| 配置数量 | 8 个固定预设 | 无限组合 |
| 自定义程度 | 低 | 高 |
| 地区适配 | 手动选择 | 自动适配 |
| 场景化 | 部分 | 完整 |
| 统计学合理性 | 手工编写 | 算法保证 |

**建议**：
- 快速使用 → 选择预设
- 批量创建 → 使用智能生成
- 精确控制 → 手动配置

### 与 WebGL 策略配合

智能生成器默认使用 **"自动混淆"** 策略：
- 不设置 `webglVendor` 和 `webglRenderer`
- 让 fingerprint-chromium 内核基于种子自动生成
- 确保 Canvas/Audio/WebGL 指纹一致

用户仍可以手动切换策略：
- 生成后切换到"使用真实硬件"
- 或保持"自动混淆（推荐）"

### 与代理一致性配合

智能生成器**尊重后端的代理自动推导**：
- `timezone` 和 `lang` 可以留空
- 后端会在启动时根据代理 IP 自动设置
- 如果智能生成时提供了 `proxyCountry`，会预填充合理的时区和语言

## 实现细节

### 权重随机算法

```typescript
function weightedRandom<T>(items: { weight: number }[]): T {
  const totalWeight = items.reduce((sum, item) => sum + item.weight, 0)
  let random = Math.random() * totalWeight

  for (const item of items) {
    random -= item.weight
    if (random <= 0) return item
  }

  return items[items.length - 1]
}
```

### 批量生成去重

```typescript
function generateBatchFingerprints(count: number): FingerprintConfig[] {
  const fingerprints: FingerprintConfig[] = []
  const usedSeeds = new Set<string>()

  for (let i = 0; i < count; i++) {
    let config = generateSmartFingerprint()

    // 确保种子唯一
    while (config.seed && usedSeeds.has(config.seed)) {
      config = generateSmartFingerprint()
    }

    if (config.seed) usedSeeds.add(config.seed)
    fingerprints.push(config)
  }

  return fingerprints
}
```

## 未来扩展

### 计划中的功能

1. **集成代理信息**
   - 从已选代理自动获取国家代码和时区
   - 智能推荐最匹配的配置

2. **自定义权重**
   - 允许用户调整各项权重
   - 保存自定义场景

3. **历史统计分析**
   - 分析已有 profile 的配置分布
   - 避免生成重复或相似的配置

4. **更多地区支持**
   - 扩展更多国家/地区的数据
   - 支持更精确的城市级配置

5. **设备类型模拟**
   - 移动设备（带触摸）
   - 平板设备
   - 笔记本 vs 台式机

## 常见问题

### Q: 智能生成会覆盖已有配置吗？

A: 是的，点击"立即生成"会覆盖当前所有指纹配置。建议在新建 profile 时使用，或提前备份重要配置。

### Q: 生成的配置能通过 browserscan.net 检测吗？

A: 是的。智能生成器：
- 使用"自动混淆"策略，避免 WebGL 异常
- 确保时区/语言/字体与地区一致
- 硬件配置合理，不会出现异常组合
- 所有参数基于真实统计数据

### Q: 可以只生成部分配置吗？

A: 当前版本会生成完整配置。如果只想改变某些参数，建议：
1. 使用智能生成作为基础
2. 手动微调特定字段

### Q: 批量生成的指纹会重复吗？

A: 不会。`generateBatchFingerprints` 确保每个指纹的种子唯一，且会生成不同的硬件配置组合。

### Q: 支持哪些国家/地区？

A: 当前支持：CN（中国）、US（美国）、JP（日本）、GB（英国）、FR（法国）、DE（德国）、KR（韩国）、SG（新加坡）、AU（澳大利亚）、CA（加拿大）。

其他地区会使用默认全球分布。

### Q: 如何为特定代理生成指纹？

A: 目前需要手动传递国家代码。未来版本会自动从已选代理获取地理信息。

```typescript
// 编程方式
const fingerprint = generateSmartFingerprint({
  proxyCountry: 'US',
  proxyTimezone: 'America/Los_Angeles',
})
```

## 总结

智能指纹生成器通过算法和数据驱动，自动生成真实、合理、可信的浏览器指纹配置，大幅简化多账号管理的配置工作，同时确保生成的指纹能够通过各种反检测系统的验证。

配合 **WebGL 自动混淆**和**代理一致性推导**，形成完整的反检测解决方案。
