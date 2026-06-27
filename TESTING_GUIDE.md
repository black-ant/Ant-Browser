# 🧪 后端API测试指南

## 测试环境

✅ **Wails Dev服务器已启动**
- 应用地址：http://localhost:34115
- 后端API：已绑定
- 数据库：SQLite (data/ant-browser.db)

---

## 测试清单

### **1. 账号管理测试**

#### ✅ 创建账号
1. 打开应用
2. 导航到 "账号管理" 页面
3. 点击 "添加账号"
4. 填写信息：
   - 账号名称：测试账号1
   - 平台：Facebook
   - 用户名：test@example.com
   - 邮箱：test@example.com
   - 密码：TestPassword123
   - 备注：这是一个测试账号
5. 点击保存

**验证点：**
- [ ] 保存成功提示
- [ ] 账号出现在列表中
- [ ] 密码字段显示为 "******"（不显示明文）

#### ✅ 查看账号列表
1. 刷新页面
2. 确认账号仍然存在

**验证点：**
- [ ] 数据持久化成功
- [ ] 列表不显示密码明文

#### ✅ 编辑Cookie
1. 点击账号的 "Cookie" 按钮
2. 查看Cookie编辑器
3. 输入测试Cookie：
   ```
   session_id=abc123; user_token=xyz789
   ```
4. 保存

**验证点：**
- [ ] Cookie保存成功
- [ ] 再次打开可以看到保存的Cookie

#### ✅ 更新账号
1. 点击 "编辑" 按钮
2. 修改账号名称为：测试账号1（已修改）
3. 保存

**验证点：**
- [ ] 更新成功
- [ ] 修改后的名称显示正确

#### ✅ 删除账号
1. 点击 "删除" 按钮
2. 确认删除

**验证点：**
- [ ] 删除成功
- [ ] 账号从列表中消失

---

### **2. 数据库验证**

#### ✅ 验证数据加密
打开数据库查看加密状态：

```bash
# 打开数据库
sqlite3 data/ant-browser.db

# 查看账号表结构
.schema browser_accounts

# 查看账号数据（注意密码和Cookie已加密）
SELECT account_id, account_name, 
       substr(password_enc, 1, 20) as password_preview,
       substr(cookies_enc, 1, 20) as cookies_preview
FROM browser_accounts;

# 退出
.exit
```

**验证点：**
- [ ] password_enc 显示为base64编码字符串（不是明文）
- [ ] cookies_enc 显示为base64编码字符串（不是明文）
- [ ] 无法通过明文搜索找到密码

#### ✅ 验证索引
```sql
# 查看索引
.indexes browser_accounts

# 应该看到：
# idx_browser_accounts_platform
# idx_browser_accounts_created_at
```

---

### **3. localStorage清空验证**

#### ✅ 验证数据不依赖localStorage
1. 打开浏览器开发者工具（F12）
2. 切换到 Console 标签
3. 执行：
   ```javascript
   localStorage.clear()
   location.reload()
   ```
4. 页面刷新后，检查账号列表

**验证点：**
- [ ] 清空localStorage后数据仍然存在
- [ ] 数据从SQLite加载，不依赖localStorage

---

### **4. 性能测试**

#### ✅ 批量创建测试
1. 创建10个测试账号
2. 观察加载速度

**验证点：**
- [ ] 列表加载流畅
- [ ] 搜索功能正常
- [ ] 无明显卡顿

---

## 已知问题

### ⚠️ 端口冲突警告
```
ERR | listen tcp 127.0.0.1:34115: bind: Only one usage of each socket address...
```

**原因：** 之前的Wails窗口还在运行
**影响：** 无影响，应用仍正常工作
**解决：** 可以忽略，或者关闭之前的窗口

---

## 测试结果记录

### 账号管理
- [ ] 创建账号
- [ ] 查看列表
- [ ] 编辑Cookie
- [ ] 更新账号
- [ ] 删除账号

### 数据安全
- [ ] 密码已加密
- [ ] Cookie已加密
- [ ] 列表不泄露敏感信息

### 数据持久化
- [ ] 数据存储在SQLite
- [ ] 清空localStorage后数据不丢
- [ ] 重启应用后数据仍在

---

## 下一步

完成上述测试后：

1. **如果全部通过** ✅
   - 更新ExtensionManagementPage.tsx（类似账号页面）
   - 标记项目100%完成

2. **如果有问题** ⚠️
   - 记录错误信息
   - 检查浏览器控制台
   - 查看Wails日志

---

## 快速验证命令

```bash
# 查看数据库内容
sqlite3 data/ant-browser.db "SELECT * FROM browser_accounts;"

# 查看扩展表
sqlite3 data/ant-browser.db "SELECT * FROM browser_extensions;"

# 验证加密
sqlite3 data/ant-browser.db "SELECT account_name, password_enc FROM browser_accounts LIMIT 1;"
```

---

**🎉 应用已启动，可以开始测试了！**

访问：http://localhost:34115
