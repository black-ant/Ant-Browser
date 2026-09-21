package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := Config{}
	config.Browser.RestoreLastSession = DefaultConfig().Browser.RestoreLastSession
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	normalizeConfig(&config)

	return &config, nil
}

// MigrateLegacyConfig 将 1.8.0 之前没有 config_version 的配置迁移到当前语义。
// 历史发布配置默认写入 restore_last_session=false，无法区分用户选择与旧默认值；
// 因此首次迁移统一恢复为 true。迁移完成后，用户显式关闭恢复会继续保留。
func MigrateLegacyConfig(config *Config) bool {
	if config == nil || config.ConfigVersion >= CurrentConfigVersion {
		return false
	}
	if config.ConfigVersion == 0 {
		config.Browser.RestoreLastSession = true
	}
	config.ConfigVersion = CurrentConfigVersion
	return true
}

// Save 保存配置到文件
func (c *Config) Save(configPath string) error {
	safeConfig := *c
	safeConfig.Backup = sanitizeBackupConfig(c.Backup)
	data, err := yaml.Marshal(&safeConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

func sanitizeBackupConfig(value BackupConfig) BackupConfig {
	value.Channels.OpenList.Token = ""
	value.Channels.S3 = S3ChannelConfig{}
	return value
}

// ProxyStore 代理数据文件结构
type ProxyStore struct {
	Proxies []BrowserProxy `yaml:"proxies"`
}

// LoadProxies 从独立文件加载代理列表
func LoadProxies(path string) ([]BrowserProxy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取代理文件失败: %w", err)
	}
	var store ProxyStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("解析代理文件失败: %w", err)
	}
	return store.Proxies, nil
}

// SaveProxies 将代理列表保存到独立文件
func SaveProxies(path string, proxies []BrowserProxy) error {
	store := ProxyStore{Proxies: proxies}
	data, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("序列化代理数据失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建代理目录失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入代理文件失败: %w", err)
	}
	return nil
}
