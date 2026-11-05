export const DOC_TUTORIAL = `# 使用教程

## 应用内路径

- 内核：\`指纹浏览器 -> 内核管理 -> 下载内核 / 识别本地内核\`
- 代理：\`指纹浏览器 -> 代理池配置 -> 导入代理\`
- 插件：\`指纹浏览器 -> 插件包管理 -> 安装 / 导入插件\`
- 实例：\`指纹浏览器 -> 实例列表 -> 新建配置 -> 启动\`
- 迁移：\`实例列表 -> 导入实例 / 勾选实例 -> 导出\`

## 最小上手顺序

1. 打开 \`指纹浏览器 > 内核管理\`
2. 准备一个可用内核
3. 打开 \`指纹浏览器 > 代理池配置\`，导入代理或确认直连
4. 需要固定插件时，打开 \`指纹浏览器 > 插件包管理\` 安装插件
5. 打开 \`指纹浏览器 > 实例列表\`，点 \`新建配置\`
6. 选择内核、代理、插件后保存并启动

## 实例迁移

- 批量导出：\`实例列表 -> 勾选实例 -> 导出\`
- 单个导出：\`实例列表 -> 更多 -> 导出\`
- 导入实例：\`实例列表 -> 导入实例 -> 选择 ZIP\`

- 导出前先停止实例，避免浏览器数据文件被占用
- 导出包包含实例配置和完整浏览器用户数据目录
- 导入时会生成新实例，不覆盖当前已有实例
- 代理只按名称适配本地同名代理，未匹配时自动清空代理

## 最小 HTTP 对接

只想启动实例，用 \`POST /api/launch\`；需要等待实例和 CDP 都可用，用 \`POST /api/runtime/session\`。

\`\`\`bash
curl http://127.0.0.1:19876/api/health

curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code": "BUYER_001",
    "skipDefaultStartUrls": true
  }'
\`\`\`

## 返回里主要看这几个字段

\`\`\`json
{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "launchCode": "BUYER_001",
  "debugReady": true,
  "cdpUrl": "http://127.0.0.1:19876"
}
\`\`\`

## Playwright 接管

\`\`\`javascript
import { chromium } from "playwright";

const res = await fetch("http://127.0.0.1:19876/api/launch", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    code: "BUYER_001",
    skipDefaultStartUrls: true
  })
});

const data = await res.json();
const browser = await chromium.connectOverCDP(data.cdpUrl);
\`\`\`

需要稳定接管时，不要自己轮询，直接用 \`POST /api/runtime/session\`。
`

export const DOC_OPERATION_FLOW = `# 操作流程

## 首次配置

\`\`\`text
1. 内核管理：准备 Chrome 内核
2. 代理池配置：导入或录入代理
3. 插件包管理：安装插件，设置默认安装或指定实例
4. 实例列表：创建实例并选择内核、代理、分组
5. 启动实例：确认 debugReady 后再接管
6. 迁移实例：停止实例后导出 ZIP，新环境中导入为新实例
\`\`\`

## 插件接入流程

\`\`\`text
插件包管理 -> 输入商店链接 / 插件 ID -> 安装
插件包管理 -> 默认安装 / 设置实例 -> 按分组选择实例 -> 保存
实例列表 -> 单个实例 -> 插件 -> 手动设置本实例（可选）
\`\`\`

## 分组限制怎么用

- 在实例列表或分组管理里先维护实例分组
- 在插件包管理里点插件卡片的 \`设置实例\`
- 弹窗会按分组展示实例，并显示每组已选数量
- 点 \`选择本组\` 可以快速让整组实例写入该插件
- 点 \`取消本组\` 可以快速让整组实例排除此插件

## 启动前检查

\`\`\`text
1. 实例有可用内核
2. 代理可连通，或确认不需要代理
3. 必要插件已安装，并且该实例在插件限制范围内
4. 启动后 debugReady=true
\`\`\`
`

export const DOC_SKILL_USAGE = `# SKILL 使用说明

## 先准备好 3 个前提

1. Ant Browser 和 OpenClaw 在同一台机器上
2. Ant Browser 的 LaunchServer 可访问，默认是 \`http://127.0.0.1:19876\`
3. OpenClaw 已安装 \`ant-chrome-openclaw\` skill，并且已有指向 Ant Browser 的远程 CDP 浏览器配置

## 安装 Skill

如果你不知道项目根目录在哪，先点上面的 \`打开根目录\`。

### Windows

直接安装：

\`\`\`powershell
pwsh -File skills/ant-chrome-openclaw/scripts/install_ant_chrome_openclaw.ps1 -SetDefaultProfile
\`\`\`

如果没探测到 OpenClaw 路径，再用：

\`\`\`powershell
pwsh -File skills/ant-chrome-openclaw/scripts/install_ant_chrome_openclaw.ps1 -TargetSkillsDir "C:\\path\\to\\openclaw\\skills" -ConfigFile "C:\\path\\to\\openclaw\\openclaw.json" -SetDefaultProfile
\`\`\`

### Linux

直接安装：

\`\`\`bash
bash skills/ant-chrome-openclaw/scripts/install_ant_chrome_openclaw.sh \
  --set-default-profile
\`\`\`

如果没探测到 OpenClaw 路径，再用：

\`\`\`bash
bash skills/ant-chrome-openclaw/scripts/install_ant_chrome_openclaw.sh \\
  --target-skills-dir /path/to/openclaw/skills \\
  --config-file /path/to/openclaw/openclaw.json \\
  --set-default-profile
\`\`\`

如果 Ant Browser 开了 API Key，安装时补上对应参数即可。

## 在对话里怎么触发

每次提问开头都明确写：

\`\`\`text
使用 ant-chrome-openclaw skill。
\`\`\`

然后直接写你的目标，不要只说“帮我打开浏览器”。

## 推荐提问模板

### 启动并接管

\`\`\`text
使用 ant-chrome-openclaw skill。
先检查 LaunchServer。
如果实例 BUYER_001 没有运行，就启动它。
确认 debugReady=true 后接管浏览器，并打开 https://example.com
\`\`\`

### 只接管当前活动实例

\`\`\`text
使用 ant-chrome-openclaw skill。
先检查当前 active 实例。
如果当前活动实例已经是 BUYER_001，就直接接管，不要切换到别的实例。
\`\`\`

### 按条件匹配实例

\`\`\`text
使用 ant-chrome-openclaw skill。
按 keyword=buyer-001 查实例状态。
如果唯一命中，就接管。
如果多命中，不要自动切换，先告诉我。
\`\`\`

### 停止实例

\`\`\`text
使用 ant-chrome-openclaw skill。
停止 launchCode=BUYER_001 对应的实例。
\`\`\`

## 稳定使用规则

- 先在 Ant Browser 前端里把实例、代理、标签和内核配置好
- 优先提供精确标识，推荐顺序是 \`launchCode\`、\`profileId\`、\`profileName\`
- 只有在 \`debugReady=true\` 且 \`cdpUrl\` 非空时才接管
- 如果 selector 命中多个实例，不要自动选，先返回结果给用户确认
- \`browser stop\` 只是断开 OpenClaw 接管，不等于关闭 Ant Browser 实例
- 当前统一 CDP 入口一次只指向一个活动实例，切换前先看 \`GET /api/runtime/active\`

## 最小排查顺序

\`\`\`text
1. GET /api/health
2. GET /api/runtime/active
3. 按 code 或 selector 调 launch / runtime/session
4. 如果失败，再看 GET /api/launch/logs?limit=20
\`\`\`
`

export const DOC_CORE_INTRO = `# 内核介绍

## 推荐目录

\`\`\`text
chrome/
  chrome-<version>/
    chrome.exe
    ...
\`\`\`

## 两种准备方式

\`\`\`text
方式 A：应用内下载
指纹浏览器 -> 内核管理 -> 下载内核

方式 B：手动下载
下载 ZIP -> 解压到 chrome/<version>/ -> 回到内核管理确认识别
\`\`\`

## 下载渠道

- [fingerprint-chromium](https://github.com/adryfish/fingerprint-chromium)
- [Releases](https://github.com/adryfish/fingerprint-chromium/releases)

## 在应用里怎么用

\`\`\`text
内核管理 -> 设默认内核
实例编辑页 -> 选择内核
\`\`\`

## 全局设置

\`\`\`text
内核管理 -> 全局设置 -> 编辑
\`\`\`

- 用户数据根目录决定实例数据保存位置
- 默认启动参数会追加到实例启动参数里
- 默认指纹参数会追加到指纹内核启动参数里
- 默认启动页面为空时，不会额外打开页面
- 轻启动模式用于减少启动阶段的额外等待

## 自检

\`\`\`text
1. 目录下能看到 chrome.exe
2. 内核管理页能识别到该目录
3. 实例绑定的是正确版本
\`\`\`
`

export const DOC_PROXY_INTRO = `# 代理介绍

## 直接录入示例

\`\`\`text
<proxy-url>
<proxy-url>
<proxy-url>
\`\`\`

## Clash YAML 导入示例

\`\`\`yaml
proxies:
  - name: hk-vless
    type: vless
    server: example.com
    port: 443
    uuid: your-uuid
    tls: true
    servername: example.com
\`\`\`

## 批量 DNS 示例

\`\`\`yaml
dns:
  enable: true
  nameserver:
    - 119.29.29.29
    - 223.5.5.5
\`\`\`

## 应用内路径

\`\`\`text
代理池配置 -> 导入 Clash YAML / 录入 HTTP(S) / SOCKS5
实例编辑页 -> 选择代理池节点
\`\`\`

## 在实例接口里绑定代理

\`\`\`json
{
  "profile": {
    "profileName": "buyer-001",
    "proxyId": "proxy-us",
    "keywords": ["buyer-001"]
  },
  "launchCode": "BUYER_001"
}
\`\`\`

不需要代理时，\`proxyId\` 留空即可。
\`proxyId\` 与 \`proxyConfig\` 同时传时，优先使用 \`proxyId\` 对应的代理池节点。
如果 \`proxyId\` 无效但传了 \`proxyConfig\`，会自动改为使用该 \`proxyConfig\`。
如果 \`proxyId\` 无效且 \`proxyConfig\` 也为空，请求会直接报错。
`

export const DOC_EXTENSION_INTRO = `# 插件包管理

## 入口

\`\`\`text
指纹浏览器 -> 插件包管理
\`\`\`

## 安装方式

- 输入 Chrome Web Store 链接或 32 位插件 ID 后安装
- 本地已有 \`.crx\` / \`.zip\` 时，用手动安装导入文件
- 已解压的插件目录可直接导入目录
- 安装完成后，勾选 \`默认安装\` 才会写入未手动设置的实例

## 设置实例

\`\`\`text
插件包管理 -> 插件卡片 -> 设置实例
\`\`\`

- 勾选的实例会写入该插件
- 未勾选的实例会排除该插件
- 弹窗按实例分组展示，未设置分组的实例进入 \`未分组\`
- 每个分组支持 \`选择本组\` 和 \`取消本组\`
- 分组标题会显示该组 \`已选 / 总数\`

## 单实例插件配置

\`\`\`text
实例列表 -> 实例行 -> 插件
\`\`\`

- 默认情况下，实例继承已勾选 \`默认安装\` 的插件
- 打开手动设置后，该实例只写入弹窗内勾选的插件
- 手动设置适合少量特殊实例

## 推荐用法

\`\`\`text
同一批业务实例 -> 放入同一分组
同一批业务需要的插件 -> 在设置实例里选择对应分组
个别实例例外 -> 再用单实例插件配置微调
\`\`\`
`

export const DOC_API_OVERVIEW = `# 接口总览

## 基础地址

\`\`\`text
http://127.0.0.1:19876
\`\`\`

## 认证示例

\`\`\`bash
curl -H "X-Ant-Api-Key: <your-api-key>" http://127.0.0.1:19876/api/health
\`\`\`

没开认证时，去掉这个请求头即可。

## 全部接口

| 分类 | 方法 | 路径 |
|------|------|------|
| 健康检查 | \`GET\` | \`/api/health\` |
| 实例管理 | \`GET\` | \`/api/profiles\` |
| 实例管理 | \`POST\` | \`/api/profiles\` |
| 实例管理 | \`GET\` | \`/api/profiles/{profileId}\` |
| 实例管理 | \`PUT\` | \`/api/profiles/{profileId}\` |
| 实例管理 | \`DELETE\` | \`/api/profiles/{profileId}\` |
| 实例管理 | \`GET\` | \`/api/profiles/{profileId}/status\` |
| 实例管理 | \`POST\` | \`/api/profiles/{profileId}/stop\` |
| 启动 | \`GET\` | \`/api/launch/{code}\` |
| 启动 | \`POST\` | \`/api/launch\` |
| 运行态 | \`GET\` | \`/api/runtime/active\` |
| 运行态 | \`POST\` | \`/api/runtime/session\` |
| 运行态 | \`POST\` | \`/api/runtime/status\` |
| 运行态 | \`POST\` | \`/api/runtime/stop\` |
| 自动化脚本 | \`GET\` | \`/api/automation/scripts\` |
| 自动化脚本 | \`GET\` | \`/api/automation/scripts/{scriptId}\` |
| 自动化脚本 | \`POST\` | \`/api/automation/scripts/run\` |
| 自动化脚本 | \`GET\` | \`/api/automation/scripts/runs\` |
| 调用日志 | \`GET\` | \`/api/launch/logs\` |
| CDP 统一入口 | \`GET\` | \`/json/version\` |
| CDP 统一入口 | \`GET\` | \`/json/list\` |
| CDP 统一入口 | \`WS\` | \`/devtools/...\` |

## selector 最小写法

\`\`\`json
{
  "selector": {
    "code": "BUYER_001",
    "matchMode": "unique"
  }
}
\`\`\`

也可以把 \`code / profileId / profileName / keyword / tags / groupId / matchMode\` 放在请求体顶层；新接入建议统一放进 \`selector\`。

## 怎么选接口

| 场景 | 用哪个 |
|------|--------|
| 已知唯一 Code | \`GET /api/launch/{code}\` |
| 需要参数 / selector | \`POST /api/launch\` |
| 需要 ready 后再接管 | \`POST /api/runtime/session\` |
| 只有 selector，想查状态 | \`POST /api/runtime/status\` |
| 只有 selector，想停止 | \`POST /api/runtime/stop\` |
`
