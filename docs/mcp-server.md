# MCP 服务

Ant Browser 内置 MCP（[Model Context Protocol](https://modelcontextprotocol.io/)）服务，
让 Claude Code、Claude Desktop、Cursor 等 AI 客户端可以直接管理浏览器实例、
执行自动化脚本和查看代理池，不需要为每个客户端单独写 HTTP 调用胶水。

## 快速开始

1. 打开 Ant Browser，进入 `设置 > MCP 服务`，确认开关已开启（默认开启）
2. 在卡片里选择传输方式，点击「复制配置」
3. 把配置粘贴到客户端的 MCP 配置文件里
4. 重启客户端

## 传输方式

MCP 端点挂在 LaunchServer 上，与 Launch API 共用端口（默认 `19876`）和鉴权设置，
不占用额外端口。

### HTTP（推荐）

适用于支持远程 MCP 的客户端。

```json
{
  "mcpServers": {
    "ant-browser": {
      "type": "http",
      "url": "http://127.0.0.1:19876/mcp"
    }
  }
}
```

Claude Code 可以直接用命令行添加：

```bash
claude mcp add --transport http ant-browser http://127.0.0.1:19876/mcp
```

### stdio

适用于只支持本地子进程的客户端。桥接进程把 stdio 转发到上面的 HTTP 端点，
工具仍在主程序里执行，因此**使用 stdio 时 Ant Browser 必须处于运行状态**。

```json
{
  "mcpServers": {
    "ant-browser": {
      "command": "C:\\Program Files\\Ant Browser\\ant-chrome.exe",
      "args": ["--mcp-stdio"]
    }
  }
}
```

`command` 需要换成本机实际的可执行文件路径，设置页里会自动填好。

## 鉴权

MCP 端点复用 Launch API 的鉴权配置（`config.yaml` 中的 `launch_server.auth`）。
开启后，HTTP 客户端需要带上对应请求头：

```json
{
  "mcpServers": {
    "ant-browser": {
      "type": "http",
      "url": "http://127.0.0.1:19876/mcp",
      "headers": { "X-Ant-Api-Key": "你的 API Key" }
    }
  }
}
```

stdio 方式不需要额外配置，桥接进程会自己读取本地配置并附加请求头。

服务只监听 `127.0.0.1`，并启用了 DNS rebinding 防护（非本机 Host 头会被拒绝），
恶意网页无法通过浏览器驱动本地服务。

## 配置项

```yaml
mcp:
    enabled: true      # 是否启用 MCP 服务
    path: /mcp         # 挂载路径
    stateless: false   # 无状态模式，不维护会话；此时 GET/DELETE 返回 405
```

端口和鉴权在 `launch_server` 段配置，不在这里重复。

## 工具清单

工具名统一使用 `ant_` 前缀，避免与客户端上其他 MCP 服务冲突。

### 实例管理

| 工具 | 说明 |
| --- | --- |
| `ant_instance_list` | 列出实例，可按标签、分组、关键字、运行状态过滤 |
| `ant_instance_get` | 按 selector 查询单个实例的完整配置 |
| `ant_instance_create` | 创建实例 |
| `ant_instance_update` | 更新实例配置（整体替换，非增量合并） |
| `ant_instance_delete` | 删除实例，运行中的实例需先停止 |

### 运行时

| 工具 | 说明 |
| --- | --- |
| `ant_instance_start` | 启动实例，不等待调试端口就绪 |
| `ant_instance_stop` | 停止实例，会关闭真实浏览器进程 |
| `ant_runtime_session` | 启动并等待 CDP 就绪，返回 `cdpUrl` |
| `ant_runtime_status` | 查询运行态，不触发启动 |
| `ant_runtime_active` | 查询当前挂在统一 CDP 入口上的实例 |

### 自动化脚本

| 工具 | 说明 |
| --- | --- |
| `ant_script_list` | 列出已导入的脚本 |
| `ant_script_get` | 查询脚本详情，含默认目标实例和默认参数 |
| `ant_script_run` | 执行脚本并等待结果 |
| `ant_script_runs` | 查询最近的执行记录 |

### 代理与内核

| 工具 | 说明 |
| --- | --- |
| `ant_proxy_list` | 列出代理节点 |
| `ant_proxy_test_speed` | 对指定代理测速 |
| `ant_proxy_check_health` | 检测代理出口 IP 归属地与风险评分 |
| `ant_core_list` | 列出已登记的浏览器内核 |

只读工具带 `readOnlyHint` 标注，删除和停止操作带 `destructiveHint`，
客户端可以据此做权限分级或二次确认。

## selector 规则

多数工具通过 `selector` 定位实例，字段可组合，全部命中才算匹配：

```json
{
  "selector": {
    "code": "BUYER_001",
    "profileId": "profile-123",
    "profileName": "Amazon US",
    "keywords": ["amazon-us"],
    "tags": ["电商"],
    "groupId": "group-sales",
    "matchMode": "unique"
  }
}
```

- 优先使用 `code`，其次 `profileId`，它们最精确
- `matchMode` 默认 `unique`，命中多个会报错并列出候选；确实想取第一个时用 `first`
- `ant_instance_list` 不走 selector，它的 `keyword` 是子串匹配，用于「筛一批」

## 让 AI 接管浏览器

典型流程：

1. 调 `ant_runtime_session`，传入目标实例的 selector
2. 若返回 `ready=true`，用返回的 `cdpUrl` 交给浏览器自动化工具接管
3. 若返回 `ready=false`，浏览器已启动但调试端口未就绪，稍后重试
4. 用完调 `ant_instance_stop`

注意统一 CDP 入口同一时刻只指向一个实例。切换目标前建议先用
`ant_runtime_active` 确认当前挂的是哪个实例。

## 常见问题

**客户端连不上？**

先确认 Ant Browser 正在运行，再检查设置页里显示的端点地址与客户端配置是否一致。
改过 Launch API 端口的话，客户端配置也要同步更新。

**stdio 方式报「无法连接」？**

桥接进程需要主程序在运行。如果主程序开着仍然失败，检查 `设置 > MCP 服务`
开关是否开启，以及 `config.yaml` 里的端口与实际监听端口是否一致
（首选端口被占用时主程序会启动失败）。

**工具报「selector 命中多个实例」？**

补充更精确的条件（`code` 或 `profileId`），或显式设置 `matchMode: "first"`。

**脚本执行失败？**

用 `ant_script_runs` 查看最近的执行记录，里面有具体错误信息。
自动化脚本依赖本地 automation runtime，首次使用需要在
`设置 > 自动化支持` 中完成安装。

## 与 Launch API 的关系

MCP 不是另一套 API，而是现有能力的另一种表达方式：工具调用的是与
`/api/*` 相同的业务逻辑。两者可以同时使用：

- 外部系统按固定协议集成 → 用 Launch API
- AI 客户端按自然语言驱动 → 用 MCP

Launch API 的说明见应用内的「接口文档」页面。
