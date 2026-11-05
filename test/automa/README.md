# Automa 导入测试案例

文件：`example-domain-import-test.automa.json`

## 案例流程

1. 手动启动工作流。
2. 打开 `https://example.com/`。
3. 等待 1 秒。
4. 显示一条 `Ant Chrome` 通知。

## 导入

1. 打开 Automa 工作流页面。
2. 选择导入工作流。
3. 选择本目录下的 `example-domain-import-test.automa.json`。
4. 导入后手动运行。

## 已确认

- JSON 使用本地 Automa `1.30.02` 扩展当前使用的工作流字段和节点格式。
- 文件已包含 `extVersion`、`drawflow`、`settings`、`globalData` 和 `includedWorkflows` 字段。
- 当前仓库只完成文件和 JSON 结构校验，尚未通过当前实例的 Automa UI 实际导入并运行。

## 其他案例

- `example-domain-read-heading.automa.json`：读取 `h1` 文本并写入 `pageTitle` 变量。
- `example-domain-click-link.automa.json`：打开页面后点击第一个链接。

## 复杂案例

- `example-domain-loop-export.automa.json`：循环遍历链接，收集文本，写入数据表并导出 JSON。
- `example-domain-javascript-export.automa.json`：执行网页 JavaScript，设置 `pageTitle` 变量，再导出变量 JSON。

复杂案例首次运行可能需要允许 Automa 的下载和通知权限。
