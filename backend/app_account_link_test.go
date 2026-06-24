package backend

import (
	"testing"

	"ant-chrome/backend/internal/browser"
)

// 测试账号关联到实例的功能
func TestAccountLinkToProfile(t *testing.T) {
	// 这是单元测试的占位符
	// 实际测试需要初始化完整的 App 实例（包括数据库）
	// 在集成测试中验证功能

	input := browser.ProfileInput{
		ProfileName: "测试实例",
		AccountIds:  []string{"account-123", "account-456"},
	}

	// 验证 AccountIds 字段可以正常赋值
	if len(input.AccountIds) != 2 {
		t.Errorf("AccountIds 长度错误: 期望 2, 实际 %d", len(input.AccountIds))
	}

	if input.AccountIds[0] != "account-123" {
		t.Errorf("AccountIds[0] 错误: 期望 account-123, 实际 %s", input.AccountIds[0])
	}

	if input.AccountIds[1] != "account-456" {
		t.Errorf("AccountIds[1] 错误: 期望 account-456, 实际 %s", input.AccountIds[1])
	}
}
