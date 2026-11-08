package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// configWriteMu 串行化配置的“读改写 + 序列化”。
//
// yaml.Marshal 会遍历 Config 里的所有 map；如果另一个 goroutine 正在改 map
// （例如订阅源忽略名单），Go 会直接 fatal：concurrent map iteration and map write。
// 因此所有 Save 都经过这把锁，需要“改 map 后立即保存”的调用方必须用
// LockConfigWrite + SaveLocked 把两步包在同一临界区里。
var configWriteMu sync.Mutex

// LockConfigWrite 获取配置写锁，返回解锁函数。
func LockConfigWrite() func() {
	configWriteMu.Lock()
	return configWriteMu.Unlock
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	normalizeConfig(&config)

	return &config, nil
}

// Save 保存配置到文件
func (c *Config) Save(configPath string) error {
	unlock := LockConfigWrite()
	defer unlock()
	return c.SaveLocked(configPath)
}

// SaveLocked 在已持有 LockConfigWrite 锁的情况下保存配置。
// 用于把“修改配置 map”和“序列化写盘”放在同一个临界区内。
func (c *Config) SaveLocked(configPath string) error {
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
	value.Channels.OpenList.Username = ""
	value.Channels.OpenList.Password = ""
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
