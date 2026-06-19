# 发布前验证流程（Release Checklist）

目标：每次发布前有**固定**验证流程，不靠记忆、不靠手工乱点。

## 1. 自动门槛（必须全绿）

```bash
# 后端编译/vet/测试 + 前端类型检查/构建（任一失败即非零退出）
bash scripts/verify.sh

# 运行时二进制清单校验（避免打包缺 xray / sing-box / chrome 内核）
bash scripts/release-check.sh
```

CI 已在 push / PR 自动跑 `backend go test` 与 `frontend typecheck + build`（见 `.github/workflows/ci.yml`）。本地提交前请先跑 `scripts/verify.sh`。

## 2. 最小手工 E2E 清单（`wails dev` 实跑）

逐项勾选，全部通过才发布：

- [ ] **创建实例**：新建一个实例，列表出现、可编辑。
- [ ] **启动实例**：点击启动 → 状态 starting→running，浏览器窗口打开。
- [ ] **停止实例**：点击停止 → 状态回到 stopped，进程关闭。
- [ ] **批量启动**：多选启动 → 按队列（默认并发 3）推进，UI 不假死。
- [ ] **代理保存**：新增/编辑一个代理并保存 → 重启应用后仍在（含测速结果）。
- [ ] **订阅刷新**：刷新一个 URL 订阅源 → 节点更新，忽略/重命名保留。
- [ ] **账号保存**：新建账号（含密码）→ 保存后重开详情，密码字段可解密回显。
- [ ] **Cookie**：从运行中实例读取 Cookie → 保存 → 回写，无报错。
- [ ] **扩展绑定**：给实例绑定一个本地扩展 → 启动浏览器，扩展实际加载（`--load-extension` 生效）。
- [ ] **Dashboard**：CPU/内存曲线随时间增长；启动/停止/测速产生真实活动；最近错误面板正常。

## 3. 发布步骤

1. `scripts/verify.sh` 全绿。
2. `scripts/release-check.sh` 必需项齐备。
3. 完成第 2 节手工 E2E 清单。
4. 触发打包工作流（`Publish Linux / macOS Packages`，`workflow_dispatch`）。

## 4. 回归高风险点（本轮重构后重点复测）

- 实例启动三段式锁重构：批量启动稳定性、启动中点停止不复活。
- 代理刷新/保存事务化：部分失败不丢数据、健康列不被清空。
- DevTools `useCdpSession`：连接 / 断线重连（最多 3 次）/ 手动停止。
- 加密密钥若改为本机派生：换机后旧密文需重新录入或迁移。
