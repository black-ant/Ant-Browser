# 账号关联功能集成测试场景

## 场景 1: 创建实例时关联账号
```
输入:
POST /api/profiles
{
  "profile": {
    "profileName": "测试实例",
    "accountIds": ["account-001", "account-002"]
  }
}

预期:
1. 实例创建成功
2. account-001.relatedProfileIds 包含新实例ID
3. account-002.relatedProfileIds 包含新实例ID
```

## 场景 2: 更新实例添加账号关联
```
输入:
PUT /api/profiles/{id}
{
  "profile": {
    "accountIds": ["account-003"]
  }
}

预期:
1. 实例更新成功
2. account-003.relatedProfileIds 包含实例ID
3. 之前的关联（account-001, account-002）保持不变
```

## 场景 3: 账号不存在时的处理
```
输入:
POST /api/profiles
{
  "profile": {
    "accountIds": ["non-existent-account"]
  }
}

预期:
1. 实例创建成功
2. 日志记录警告："关联账号不存在，跳过"
3. 不影响实例创建流程
```

## 场景 4: 空账号列表
```
输入:
POST /api/profiles
{
  "profile": {
    "accountIds": []
  }
}

预期:
1. 实例创建成功
2. 不执行账号关联逻辑
```

## 场景 5: 重复关联
```
输入:
第一次 PUT: accountIds: ["account-001"]
第二次 PUT: accountIds: ["account-001"]

预期:
1. 第二次不会重复添加
2. account-001.relatedProfileIds 只包含一个实例ID
```
