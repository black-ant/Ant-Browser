package browser

import (
	"ant-chrome/backend/internal/config"
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
	candidates := CoreExecutableCandidatesForBackend(core.CoreBackend)
	exePath, _, ok := findCoreExecutableWithCandidates(baseDir, candidates)
	if !ok {
		return "", fmt.Errorf("浏览器内核目录无效：未找到 %s 后端的可执行文件（候选：%s）。请检查内核目录是否完整、内核后端是否选对，或重新下载内核",
			CoreBackendLabel(core.CoreBackend), strings.Join(candidates, ", "))
	}
	if err := fsutil.EnsureExecutable(exePath); err != nil {
		return "", fmt.Errorf("浏览器内核文件不可执行：%s。原因：%w。请检查文件权限或重新解压内核", exePath, err)
	}

	return exePath, nil
}

// ValidateCorePath 验证内核路径是否有效（后端未知，接受所有后端候选名）
func (m *Manager) ValidateCorePath(corePath string) CoreValidateResult {
	return m.ValidateCorePathForBackend(corePath, "")
}

// ValidateCorePathForBackend 按指定后端验证内核路径。
// backend 为空串时接受所有后端的候选名。
func (m *Manager) ValidateCorePathForBackend(corePath string, backend string) CoreValidateResult {
	corePath = strings.TrimSpace(corePath)
	if corePath == "" {
		return CoreValidateResult{Valid: false, Message: "路径不能为空"}
	}

	baseDir := m.ResolveRelativePath(corePath)

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return CoreValidateResult{Valid: false, Message: fmt.Sprintf("目录不存在: %s", baseDir)}
	}

	candidates := CoreExecutableCandidatesAnyBackend()
	backendHint := ""
	if strings.TrimSpace(backend) != "" {
		candidates = CoreExecutableCandidatesForBackend(backend)
		backendHint = CoreBackendLabel(backend) + " 后端"
	}

	exePath, _, ok := findCoreExecutableWithCandidates(baseDir, candidates)
	if !ok {
		if backendHint != "" {
			return CoreValidateResult{Valid: false, Message: fmt.Sprintf("未找到 %s 的浏览器可执行文件（候选：%s）", backendHint, strings.Join(candidates, ", "))}
		}
		return CoreValidateResult{Valid: false, Message: fmt.Sprintf("未找到浏览器可执行文件（候选：%s）", strings.Join(candidates, ", "))}
	}
	if err := fsutil.ValidateExecutable(exePath); err != nil {
		return CoreValidateResult{Valid: false, Message: fmt.Sprintf("浏览器可执行文件不可用：%v", err)}
	}

	return CoreValidateResult{Valid: true, Message: fmt.Sprintf("路径有效: %s", exePath)}
}

// ResolveProfileCore 返回实例实际使用的内核配置（显式 CoreId 优先，否则回落默认内核）。
func (m *Manager) ResolveProfileCore(profile *Profile) (Core, bool) {
	if profile == nil {
		return m.GetDefaultCore()
	}
	if coreId := normalizeProfileCoreID(profile.CoreId); coreId != "" {
		if core, ok := m.GetCore(coreId); ok {
			return core, true
		}
	}
	return m.GetDefaultCore()
}

// ResolveChromeBinary 解析 Chrome 二进制路径（简化版）
func (m *Manager) ResolveChromeBinary(profile *Profile) (string, error) {
	exePath, _, err := m.ResolveChromeBinaryWithCore(profile)
	return exePath, err
}

// ResolveChromeBinaryWithCore 解析二进制路径并返回命中的内核配置。
// 启动流程需要内核信息来注入按后端约定的环境变量。
func (m *Manager) ResolveChromeBinaryWithCore(profile *Profile) (string, Core, error) {
	log := logger.New("Browser")
	coreId := normalizeProfileCoreID(profile.CoreId)

	var core Core
	var found bool

	if coreId != "" {
		core, found = m.GetCore(coreId)
	}
	if !found {
		core, found = m.GetDefaultCore()
	}
	if !found {
		return "", Core{}, fmt.Errorf("未配置可用浏览器内核。请先在“内核管理”中添加内核并设置默认内核")
	}

	exePath, err := m.ResolveCoreExecutable(core)
	if err != nil {
		log.Error("内核路径解析失败", logger.F("core_id", core.CoreId), logger.F("error", err.Error()))
		return "", Core{}, err
	}

	log.Debug("使用内核",
		logger.F("core_id", core.CoreId),
		logger.F("core_backend", config.NormalizeCoreBackend(core.CoreBackend)),
		logger.F("path", exePath),
	)
	return exePath, core, nil
}
