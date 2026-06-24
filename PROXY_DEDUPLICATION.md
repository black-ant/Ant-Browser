# 代理智能识别去重功能

## 问题

在使用"智能识别"导入代理时，如果输入的文本中包含重复的代理配置（相同的 proxyConfig），会导致：
- 预览列表中出现重复项
- 导入后代理池中有多个相同配置的代理
- 浪费资源和造成混乱

## 解决方案

在 `buildImportPreview` 函数中添加去重逻辑，基于 `proxyConfig` 去重。

### 实现逻辑

```typescript
function buildImportPreview(candidates: ImportCandidate[], groupName: string): ProxyDisplayInfo[] {
  // 去重逻辑：基于 proxyConfig 去重（相同配置视为重复）
  const seen = new Map<string, ImportCandidate>()
  const uniqueCandidates: ImportCandidate[] = []

  for (const candidate of candidates) {
    const normalizedConfig = candidate.proxyConfig.trim()
    if (!seen.has(normalizedConfig)) {
      seen.set(normalizedConfig, candidate)
      uniqueCandidates.push(candidate)
    }
  }

  return uniqueCandidates.map((candidate, index) => {
    const info = parseProxyInfo(candidate.proxyConfig)
    return {
      proxyId: `preview-${index}`,
      proxyName: candidate.proxyName,
      proxyConfig: candidate.proxyConfig,
      groupName,
      sourceId: '',
      sourceUrl: '',
      sourceAutoRefresh: false,
      sourceRefreshIntervalM: 0,
      sourceLastRefreshAt: '',
      type: info.type || '-',
      server: info.server || '-',
      port: info.port || 0,
    }
  })
}
```

### 去重规则

1. **比较依据**：`proxyConfig`（代理配置字符串）
2. **规范化**：使用 `trim()` 去除首尾空格
3. **保留策略**：遇到重复时，保留第一个出现的配置
4. **去重范围**：仅在单次导入内去重，不跨已有代理池去重

### 示例

**输入（包含重复）**：
```
vmess://eyJ2IjoiMiIsInBzIjoi6aaZ5rivQSIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiJ1dWlkMSJ9
vmess://eyJ2IjoiMiIsInBzIjoi6aaZ5rivQiIsImFkZCI6IjUuNi43LjgiLCJwb3J0IjoiNDQzIiwiaWQiOiJ1dWlkMiJ9
vmess://eyJ2IjoiMiIsInBzIjoi6aaZ5rivQyIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiJ1dWlkMSJ9
```

**解析后**：
- 香港A：`vmess://1.2.3.4:443?uuid=uuid1`
- 香港B：`vmess://5.6.7.8:443?uuid=uuid2`
- 香港C：`vmess://1.2.3.4:443?uuid=uuid1`（与 A 配置相同）

**去重后**：
- ✅ 香港A：`vmess://1.2.3.4:443?uuid=uuid1`
- ✅ 香港B：`vmess://5.6.7.8:443?uuid=uuid2`
- ❌ 香港C：被去除（配置与 A 重复）

**预览列表显示**：仅显示 2 个代理（A 和 B）

## 行为说明

### ✅ 会去重的情况

1. **完全相同的配置**
   ```
   vmess://server1:443?uuid=xxx
   vmess://server1:443?uuid=xxx  # 重复，会被去除
   ```

2. **不同名称，相同配置**
   ```
   香港A: vmess://server:443
   香港B: vmess://server:443  # 配置相同，会被去除
   ```

3. **不同格式表示的相同配置**
   - 前提：经过 `parseProxyInfo` 后生成的 `proxyConfig` 相同
   - 例如：带不带空格、参数顺序不同但实质相同

### ❌ 不会去重的情况

1. **不同服务器**
   ```
   vmess://server1:443
   vmess://server2:443  # 服务器不同，保留
   ```

2. **不同端口**
   ```
   vmess://server:443
   vmess://server:8443  # 端口不同，保留
   ```

3. **不同 UUID/密码**
   ```
   vmess://server:443?uuid=uuid1
   vmess://server:443?uuid=uuid2  # UUID 不同，保留
   ```

4. **不同协议类型**
   ```
   vmess://server:443
   trojan://server:443  # 协议不同，保留
   ```

### ⚠️ 跨导入不去重

- 去重仅在**单次导入**内生效
- 不会与代理池中**已有的代理**对比去重
- 如果需要清理已有代理池中的重复项，需要手动删除

## 使用场景

### 场景 1：订阅源包含重复节点

某些订阅源可能会重复提供相同的节点（不同名称，相同配置）。

**之前**：
- 导入后代理池中有 10 个节点
- 实际只有 7 个不同的配置

**现在**：
- 自动去重，仅导入 7 个节点
- 避免资源浪费

### 场景 2：手动粘贴重复链接

用户不小心粘贴了重复的节点链接。

**之前**：
```
vmess://node1
vmess://node2
vmess://node1  # 重复
vmess://node3
```
导入 4 个节点（包含重复）

**现在**：
```
vmess://node1
vmess://node2
vmess://node1  # 自动去除
vmess://node3
```
导入 3 个节点（去重后）

### 场景 3：多个订阅源合并

合并多个订阅源时，可能存在相同的节点。

**之前**：
- 订阅 A：node1, node2, node3
- 订阅 B：node2, node4, node5
- 合并导入：6 个节点（node2 重复）

**现在**：
- 合并导入：5 个节点（node2 自动去重）

## 技术细节

### 为什么基于 proxyConfig 去重？

1. **准确性**：`proxyConfig` 是实际使用的配置字符串，完全相同则功能完全相同
2. **唯一性**：不同的配置必然有不同的 `proxyConfig`
3. **简单性**：直接字符串比较，无需复杂的解析和比对逻辑

### 为什么不基于 proxyName 去重？

- **名称可能不同但配置相同**：同一个节点可能有多个别名
- **名称可能相同但配置不同**：不同节点可能使用相同的名称

### 时间复杂度

- **去重**：O(n)，使用 Map 存储已见配置
- **空间复杂度**：O(n)，最坏情况下所有配置都不重复

### Map vs Set

使用 `Map<string, ImportCandidate>` 而不是 `Set<string>` 的原因：
- 保留第一个出现的完整 candidate 信息
- 方便未来扩展（例如显示去重统计、记录重复次数等）

## 用户提示

当导入过程中去重时，可以考虑在 UI 中显示提示信息：

```typescript
const duplicateCount = candidates.length - uniqueCandidates.length
if (duplicateCount > 0) {
  toast.info(`已自动去除 ${duplicateCount} 个重复代理`)
}
```

目前的实现是静默去重，用户在预览列表中只会看到去重后的结果。

## 未来增强

### 1. 显示去重统计

在预览模态框中显示：
```
解析到 15 个代理，去除 3 个重复，实际导入 12 个
```

### 2. 显示重复详情

提供"查看重复项"按钮，显示哪些代理被去重：
```
重复代理（已去除）：
- 香港C（与"香港A"配置相同）
- 美国B（与"美国A"配置相同）
```

### 3. 去重选项

允许用户选择去重策略：
- 自动去重（默认）
- 保留所有（不去重）
- 手动选择（勾选要保留的）

### 4. 跨池去重

导入时检查与已有代理池的重复：
```
检测到 2 个代理与现有代理池重复：
- 香港A（已存在）
- 美国B（已存在）

[ ] 跳过重复
[ ] 覆盖现有
[ ] 全部导入
```

## 总结

代理智能识别去重功能通过在预览阶段自动去除重复的代理配置，提升用户体验：

✅ **自动化**：无需手动识别和删除重复项
✅ **准确性**：基于实际配置去重，不会误删
✅ **高效性**：O(n) 时间复杂度，性能良好
✅ **透明性**：用户在预览中直接看到去重后的结果

这个功能与智能指纹生成、WebGL 修复一起，共同提升 Ant-Browser 的易用性和可靠性。
