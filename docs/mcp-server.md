# MCP 服务

Ant Browser 内置 MCP（[Model Context Protocol](https://modelcontextprotocol.io/)）服务，
让 Claude Code、Claude Desktop、Cursor 等 AI 客户端可以直接管理浏览器实例、
驱动页面操作、执行自动化脚本和查看代理池，不需要为每个客户端单独写 HTTP 调用胶水。

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

### 页面操作（Playwright / CDP）

直接驱动浏览器，不需要事先写好脚本。所有工具的 `selector` 都可省略，
省略时作用于当前挂在统一 CDP 入口上的实例。

| 工具 | 说明 |
| --- | --- |
| `ant_page_goto` | 导航到 URL，自动启动实例并接管 CDP |
| `ant_page_snapshot` | 页面快照：URL、标题和全部可交互元素及其选择器 |
| `ant_page_screenshot` | 截图，以图像内容返回给客户端 |
| `ant_page_click` | 点击元素 |
| `ant_page_fill` | 填写输入框，支持一次传多个字段 |
| `ant_page_press` | 发送键盘按键 |
| `ant_page_select` | 选择下拉框选项 |
| `ant_page_wait` | 等待元素状态、URL 变化或加载完成 |
| `ant_page_extract` | 抽取元素的文本、HTML 或属性 |
| `ant_page_evaluate` | 在页面上下文里执行 JavaScript |
| `ant_page_tabs` | 列出、新建、切换、关闭标签页 |
| `ant_page_release` | 释放常驻页面会话 |

推荐用 `ant_page_snapshot` 而不是 `ant_page_screenshot` 来决定下一步：
它只返回能点能填的元素，比截图省上下文，而且给出的 `selector` 可以直接回传给
`ant_page_click` / `ant_page_fill`，不需要自己猜 CSS。

`ant_page_evaluate` 会在页面里执行任意 JavaScript。它没有突破既有权限边界
（MCP 只监听 localhost 且受 API Key 保护，`ant_script_run` 早已能跑任意 Node 代码），
但仍建议只在快照和抽取覆盖不到时使用。

### 自动化脚本

脚本适合固定流程的批量任务；探索性的、需要边看边决定的操作用上面的页面工具。

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

## 让 AI 操作浏览器

典型流程：

1. `ant_page_goto` 打开目标页面（会自动启动实例并接管 CDP）
2. `ant_page_snapshot` 看清页面上有什么可交互元素
3. 用快照给出的 `selector` 调 `ant_page_click` / `ant_page_fill` 等工具
4. 需要读数据时用 `ant_page_extract`，需要看版式时用 `ant_page_screenshot`
5. 操作完调 `ant_page_release` 释放会话（不释放的话空闲一段时间后也会自动回收）

注意统一 CDP 入口同一时刻只指向一个实例。切换目标前建议先用
`ant_runtime_active` 确认当前挂的是哪个实例。

### 页面会话模型

页面工具背后是一个按实例常驻的 Node 进程：

- **首次调用**慢一些（需要启动实例、握手 CDP），之后的调用复用同一个连接
- **空闲回收**：默认 5 分钟无调用即自动关闭，可用 `config.yaml` 里的
  `automation.page_session_idle_ms` 调整
- **与脚本互斥**：在同一实例上执行 `ant_script_run` 会先关掉常驻会话让位，
  脚本跑完后下次页面调用会重新建立
- **实例停止即失效**：浏览器关闭或实例被停止时会话自动作废，
  再次调用页面工具会重新建立

释放会话只是断开 CDP 连接，浏览器和页面本身不受影响。

页面工具依赖本地 automation runtime（Node + playwright-core），
与自动化脚本共用同一套运行时，首次使用需要在 `设置 > 自动化支持` 中完成安装。

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

**页面工具报「自动化运行时尚未就绪」？**

页面操作和自动化脚本共用 Node + playwright-core 运行时，
需要先在 `设置 > 自动化支持` 里完成安装。

**页面工具报「当前没有活动实例」？**

省略 `selector` 时页面工具作用于当前挂在统一 CDP 入口上的实例。
还没有实例挂上去时，显式传一次 `selector` 即可。

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
