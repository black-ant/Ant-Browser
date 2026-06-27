package browser

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
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

// ListCores 获取所有内核配置
func (m *Manager) ListCores() []Core {
	if m.CoreDAO != nil {
		cores, err := m.CoreDAO.List()
		if err == nil {
			// 同步到内存 config，供其他逻辑使用
			m.Config.Browser.Cores = cores
			return cores
		}
	}
	return m.Config.Browser.Cores
}

// SaveCore 保存内核配置（新增或更新）
func (m *Manager) SaveCore(input CoreInput) error {
	log := logger.New("Browser")
	coreId := strings.TrimSpace(input.CoreId)
	coreName := strings.TrimSpace(input.CoreName)
	corePath := strings.TrimSpace(input.CorePath)

	if coreName == "" {
		return fmt.Errorf("内核名称不能为空")
	}
	if corePath == "" {
		return fmt.Errorf("内核路径不能为空")
	}

	if m.CoreDAO != nil {
		if coreId == "" {
			coreId = uuid.NewString()
		}
		if input.IsDefault {
			if err := m.CoreDAO.SetDefault(""); err != nil {
				// SetDefault 空串只清除，忽略错误
				_ = err
			}
		}
		core := Core{CoreId: coreId, CoreName: coreName, CorePath: corePath, IsDefault: input.IsDefault}
		if err := m.CoreDAO.Upsert(core); err != nil {
			return err
		}
		// 同步内存
		m.syncCoresFromDAO()
		log.Info("内核配置保存", logger.F("core_id", coreId), logger.F("core_name", coreName))
		return nil
	}

	// 降级：写 config.yaml
	existingIndex := -1
	for i, core := range m.Config.Browser.Cores {
		if coreId != "" && strings.EqualFold(core.CoreId, coreId) {
			existingIndex = i
			break
		}
	}
	if existingIndex >= 0 {
		m.Config.Browser.Cores[existingIndex].CoreName = coreName
		m.Config.Browser.Cores[existingIndex].CorePath = corePath
		if input.IsDefault {
			m.clearDefaultCore()
			m.Config.Browser.Cores[existingIndex].IsDefault = true
		}
	} else {
		if coreId == "" {
			coreId = uuid.NewString()
		}
		newCore := Core{CoreId: coreId, CoreName: coreName, CorePath: corePath,
			IsDefault: input.IsDefault || len(m.Config.Browser.Cores) == 0}
		if newCore.IsDefault {
			m.clearDefaultCore()
		}
		m.Config.Browser.Cores = append(m.Config.Browser.Cores, newCore)
	}
	log.Info("内核配置保存（文件）", logger.F("core_id", coreId))
	return m.Config.Save(m.ResolveRelativePath("config.yaml"))
}

// DeleteCore 删除内核配置
func (m *Manager) DeleteCore(coreId string) error {
	log := logger.New("Browser")
	coreId = strings.TrimSpace(coreId)
	if coreId == "" {
		return fmt.Errorf("内核ID不能为空")
	}

	if m.CoreDAO != nil {
		if err := m.CoreDAO.Delete(coreId); err != nil {
			return err
		}
		m.syncCoresFromDAO()
		log.Info("内核配置删除", logger.F("core_id", coreId))
		return nil
	}

	// 降级
	index := -1
	for i, core := range m.Config.Browser.Cores {
		if strings.EqualFold(core.CoreId, coreId) {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("内核不存在: %s", coreId)
	}
	wasDefault := m.Config.Browser.Cores[index].IsDefault
	m.Config.Browser.Cores = append(m.Config.Browser.Cores[:index], m.Config.Browser.Cores[index+1:]...)
	if wasDefault && len(m.Config.Browser.Cores) > 0 {
		m.Config.Browser.Cores[0].IsDefault = true
	}
	log.Info("内核配置删除（文件）", logger.F("core_id", coreId))
	return m.Config.Save(m.ResolveRelativePath("config.yaml"))
}

// SetDefaultCore 设置默认内核
func (m *Manager) SetDefaultCore(coreId string) error {
	log := logger.New("Browser")
	coreId = strings.TrimSpace(coreId)
	if coreId == "" {
		return fmt.Errorf("内核ID不能为空")
	}

	if m.CoreDAO != nil {
		if err := m.CoreDAO.SetDefault(coreId); err != nil {
			return err
		}
		m.syncCoresFromDAO()
		log.Info("设置默认内核", logger.F("core_id", coreId))
		return nil
	}

	// 降级
	found := false
	for i := range m.Config.Browser.Cores {
		if strings.EqualFold(m.Config.Browser.Cores[i].CoreId, coreId) {
			m.Config.Browser.Cores[i].IsDefault = true
			found = true
		} else {
			m.Config.Browser.Cores[i].IsDefault = false
		}
	}
	if !found {
		return fmt.Errorf("内核不存在: %s", coreId)
	}
	log.Info("设置默认内核（文件）", logger.F("core_id", coreId))
	return m.Config.Save(m.ResolveRelativePath("config.yaml"))
}

// syncCoresFromDAO 从 DAO 同步内核列表到内存 config
func (m *Manager) syncCoresFromDAO() {
	if m.CoreDAO == nil {
		return
	}
	if cores, err := m.CoreDAO.List(); err == nil {
		m.Config.Browser.Cores = cores
	}
}

// clearDefaultCore 清除所有默认标记
func (m *Manager) clearDefaultCore() {
	for i := range m.Config.Browser.Cores {
		m.Config.Browser.Cores[i].IsDefault = false
	}
}

// ResolveChromeBinary 解析 Chrome 二进制路径（简化版）
func (m *Manager) ResolveChromeBinary(profile *Profile) (string, error) {
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

// GetChromeVersion 解析内核的 Chrome 版本号。
//
// 反检测要点：该版本号会被前端用来拼装 navigator.userAgent。它必须等于内核真实二进制版本，
// 否则与 navigator.userAgentData（Client Hints，照实报告二进制版本）不一致，被检测站判为
// 「更改了 UserAgent / 浏览器版本不同」。因此优先读取真实可执行文件的版本资源；
// manifest 文件名/字段可能领先或滞后于实际二进制，仅作回退。
func (m *Manager) GetChromeVersion(corePath string) string {
	corePath = strings.TrimSpace(corePath)
	if corePath == "" {
		return ""
	}

	baseDir := m.ResolveRelativePath(corePath)

	// 1) 优先读真实内核可执行文件的版本资源（Windows；其它平台返回空，自动回退）。
	if exePath, _, ok := FindCoreExecutable(baseDir); ok {
		if version := readExecutableProductVersion(exePath); version != "" {
			return version
		}
	}

	// 2) 回退 manifest.json 的 version 字段。
	manifestPath := filepath.Join(baseDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// 3) 再回退 *.manifest 文件名，如 "142.0.7444.175.manifest"。
		matches, _ := filepath.Glob(filepath.Join(baseDir, "*.manifest"))
		if len(matches) > 0 {
			baseName := filepath.Base(matches[0])
			version := strings.TrimSuffix(baseName, ".manifest")
			if version != "" {
				return version
			}
		}
		return ""
	}

	// 解析 JSON
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}

	return manifest.Version
}

// CountInstancesByCore 统计使用指定内核的窗口数量
func (m *Manager) CountInstancesByCore(coreId string) int {
	coreId = strings.TrimSpace(coreId)
	count := 0
	countByCoreID := func(profileCoreId string) {
		// 如果窗口的 CoreId 为空，则使用默认内核
		if profileCoreId == "" {
			defaultCore, found := m.GetDefaultCore()
			if found && strings.EqualFold(defaultCore.CoreId, coreId) {
				count++
			}
		} else if strings.EqualFold(profileCoreId, coreId) {
			count++
		}
	}

	if len(m.Profiles) > 0 {
		for _, profile := range m.Profiles {
			countByCoreID(normalizeProfileCoreID(profile.CoreId))
		}
		return count
	}

	for _, profile := range m.Config.Browser.Profiles {
		countByCoreID(normalizeProfileCoreID(profile.CoreId))
	}
	return count
}

// GetCoresExtendedInfo 获取所有内核的扩展信息
func (m *Manager) GetCoresExtendedInfo() []CoreExtendedInfo {
	cores := m.ListCores()
	result := make([]CoreExtendedInfo, 0, len(cores))
	for _, core := range cores {
		info := CoreExtendedInfo{
			CoreId:        core.CoreId,
			ChromeVersion: m.GetChromeVersion(core.CorePath),
			InstanceCount: m.CountInstancesByCore(core.CoreId),
		}
		result = append(result, info)
	}
	return result
}
