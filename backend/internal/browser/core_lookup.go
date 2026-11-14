package browser

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"strings"
)

func normalizeProfileCoreID(coreId string) string {
	coreId = strings.TrimSpace(coreId)
	if strings.EqualFold(coreId, "default") {
		return ""
	}
	return coreId
}

// GetCore 根据 coreId 获取内核配置
func (m *Manager) GetCore(coreId string) (Core, bool) {
	coreId = normalizeProfileCoreID(coreId)
	if coreId == "" {
		return Core{}, false
	}
	for _, core := range m.ListCores() {
		if strings.EqualFold(core.CoreId, coreId) {
			return core, true
		}
	}
	return Core{}, false
}

// GetDefaultCore 获取默认内核
func (m *Manager) GetDefaultCore() (Core, bool) {
	cores := m.ListCores()
	for _, core := range cores {
		if core.IsDefault {
			return core, true
		}
	}
	if len(cores) > 0 {
		return cores[0], true
	}
	return Core{}, false
}

// ResolveCoreExecutable 解析内核可执行文件路径
func (m *Manager) ResolveCoreExecutable(core Core) (string, error) {
	corePath := strings.TrimSpace(core.CorePath)
	if corePath == "" {
		return "", fmt.Errorf("浏览器内核路径为空，请在“内核管理”中补充内核目录")
	}

	baseDir := m.ResolveRelativePath(corePath)
	exePath, _, ok := FindCoreExecutable(baseDir)
	if !ok {
		return "", fmt.Errorf("浏览器内核目录无效：未找到可执行文件（候选：%s）。请检查内核目录是否完整或重新下载内核", strings.Join(CoreExecutableCandidates(), ", "))
	}
	if err := fsutil.EnsureExecutable(exePath); err != nil {
		return "", fmt.Errorf("浏览器内核文件不可执行：%s。原因：%w。请检查文件权限或重新解压内核", exePath, err)
	}

	return exePath, nil
}

// ValidateCorePath 验证内核路径是否有效
func (m *Manager) ValidateCorePath(corePath string) CoreValidateResult {
	corePath = strings.TrimSpace(corePath)
	if corePath == "" {
		return CoreValidateResult{Valid: false, Message: "路径不能为空"}
	}

	baseDir := m.ResolveRelativePath(corePath)

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return CoreValidateResult{Valid: false, Message: fmt.Sprintf("目录不存在: %s", baseDir)}
	}
	exePath, _, ok := FindCoreExecutable(baseDir)
	if !ok {
		return CoreValidateResult{Valid: false, Message: fmt.Sprintf("未找到浏览器可执行文件（候选：%s）", strings.Join(CoreExecutableCandidates(), ", "))}
	}
	if err := fsutil.ValidateExecutable(exePath); err != nil {
		return CoreValidateResult{Valid: false, Message: fmt.Sprintf("浏览器可执行文件不可用：%v", err)}
	}

	return CoreValidateResult{Valid: true, Message: fmt.Sprintf("路径有效: %s", exePath)}
}

// ResolveChromeBinary 解析 Chrome 二进制路径（简化版）
func (m *Manager) ResolveChromeBinary(profile *Profile) (string, error) {
	log := logger.New("Browser")
	coreId := normalizeProfileCoreID(profile.CoreId)

	var core Core
	var found bool

	if coreId != "" {
		core, found = m.GetCore(coreId)
		if !found {
			return "", fmt.Errorf("实例绑定的浏览器内核不存在：%s。请在“内核管理”中重新绑定有效内核", coreId)
		}
	} else {
		core, found = m.GetDefaultCore()
	}
	if !found {
		return "", fmt.Errorf("未配置可用浏览器内核。请先在“内核管理”中添加内核并设置默认内核")
	}

	exePath, err := m.ResolveCoreExecutable(core)
	if err != nil {
		log.Error("内核路径解析失败", logger.F("core_id", core.CoreId), logger.F("error", err.Error()))
		return "", err
	}

	log.Debug("使用内核", logger.F("core_id", core.CoreId), logger.F("path", exePath))
	return exePath, nil
}
