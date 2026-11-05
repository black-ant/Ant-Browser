package browser

import (
	"os"
	"path/filepath"
	"strings"

	"ant-chrome/backend/internal/logger"
)

// ProfileExtensionLoadPaths 返回该实例需要随启动加载的插件目录（绝对路径）。
//
// 走 --load-extension 加载未打包目录：只对本次启动生效，不写注册表、
// 不写 Chrome 的 Secure Preferences，因此不会影响宿主机上的其他 Chrome，
// 也不会被 Chrome 启动时的完整性校验清掉。
func (m *Manager) ProfileExtensionLoadPaths(profile *Profile) []string {
	if m == nil || m.ExtensionDAO == nil || profile == nil {
		return nil
	}
	log := logger.New("Extension")

	settings, err := m.ExtensionDAO.GetProfileSettings(profile.ProfileId)
	if err != nil {
		log.Warn("读取实例插件配置失败，本次启动不加载插件",
			logger.F("profile_id", profile.ProfileId),
			logger.F("error", err))
		return nil
	}

	// 手动列表为空等价于没有手动配置，回落到默认安装集合。
	var extensions []Extension
	if settings.Configured && len(settings.ExtensionIDs) > 0 {
		extensions, err = m.ExtensionDAO.ListByIDs(settings.ExtensionIDs)
	} else {
		extensions, err = m.ExtensionDAO.ListDefaultInstall()
	}
	if err != nil {
		log.Warn("读取实例插件列表失败，本次启动不加载插件",
			logger.F("profile_id", profile.ProfileId),
			logger.F("error", err))
		return nil
	}

	paths := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		if !extension.Enabled {
			continue
		}
		installDir := strings.TrimSpace(extension.InstallDir)
		if installDir == "" {
			log.Warn("插件缺少安装目录，跳过加载",
				logger.F("profile_id", profile.ProfileId),
				logger.F("extension_id", extension.ExtensionID))
			continue
		}
		if !filepath.IsAbs(installDir) {
			installDir = m.ResolveRelativePath(installDir)
		}
		installDir = filepath.Clean(installDir)
		if _, statErr := os.Stat(filepath.Join(installDir, "manifest.json")); statErr != nil {
			log.Warn("插件目录不完整，跳过加载",
				logger.F("profile_id", profile.ProfileId),
				logger.F("extension_id", extension.ExtensionID),
				logger.F("dir", installDir),
				logger.F("error", statErr))
			continue
		}
		if strings.Contains(installDir, ",") {
			log.Warn("插件路径包含逗号，Chrome 无法解析，跳过加载",
				logger.F("profile_id", profile.ProfileId),
				logger.F("dir", installDir))
			continue
		}
		paths = append(paths, installDir)
	}

	log.Debug("实例插件加载清单",
		logger.F("profile_id", profile.ProfileId),
		logger.F("configured", settings.Configured),
		logger.F("count", len(paths)),
		logger.F("paths", strings.Join(paths, "|")))
	return paths
}
