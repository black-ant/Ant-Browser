package browser

import (
	"fmt"
	"os"
	"path/filepath"
)

// PrecheckResult 启动前检查结果
type PrecheckResult struct {
	OK      bool   `json:"ok"`
	Stage   string `json:"stage"`   // 失败的检查阶段
	Message string `json:"message"` // 失败原因
}

// minDiskSpaceBytes 启动所需最小磁盘空间（100MB）
const minDiskSpaceBytes = 100 * 1024 * 1024

// ValidateStartupPreconditions 执行启动前检查。
// 检查项：内核路径、用户数据目录权限、磁盘空间。
// 代理可用性检查放在启动流程中（桥接逻辑），这里不重复。
func (m *Manager) ValidateStartupPreconditions(profile *Profile) PrecheckResult {
	// 1. 内核路径检查
	chromePath, err := m.ResolveChromeBinary(profile)
	if err != nil {
		return PrecheckResult{
			OK:      false,
			Stage:   "内核路径",
			Message: fmt.Sprintf("内核路径解析失败：%v", err),
		}
	}
	if _, err := os.Stat(chromePath); err != nil {
		return PrecheckResult{
			OK:      false,
			Stage:   "内核路径",
			Message: fmt.Sprintf("内核文件不存在或无法访问：%s（%v）", chromePath, err),
		}
	}

	// 2. 用户数据目录权限检查
	userDataDir := m.ResolveUserDataDir(profile)
	parent := filepath.Dir(userDataDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return PrecheckResult{
			OK:      false,
			Stage:   "用户数据目录",
			Message: fmt.Sprintf("无法创建用户数据目录父级 %s：%v", parent, err),
		}
	}
	// 写入测试文件验证权限
	testFile := filepath.Join(parent, ".ant_write_test")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return PrecheckResult{
			OK:      false,
			Stage:   "用户数据目录",
			Message: fmt.Sprintf("用户数据目录无写权限 %s：%v", parent, err),
		}
	}
	_ = os.Remove(testFile)

	// 3. 磁盘空间检查
	available, err := getAvailableDiskSpace(parent)
	if err == nil && available > 0 && available < minDiskSpaceBytes {
		return PrecheckResult{
			OK:      false,
			Stage:   "磁盘空间",
			Message: fmt.Sprintf("磁盘空间不足：需要 %d MB，可用 %d MB", minDiskSpaceBytes/1024/1024, available/1024/1024),
		}
	}

	return PrecheckResult{OK: true}
}
