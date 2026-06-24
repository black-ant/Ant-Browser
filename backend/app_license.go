package backend

import (
	appconfig "ant-chrome/backend/internal/config"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// LicenseStatus 授权状态
type LicenseStatus struct {
	MaxLimit  int      `json:"maxLimit"`
	UsedCount int      `json:"usedCount"`
	UsedKeys  []string `json:"usedKeys"`
}

// GetLicenseStatus 获取当前授权状态（给前端使用）
func (a *App) GetLicenseStatus() LicenseStatus {
	profilesCount := 0
	if a.browserMgr != nil {
		profilesCount = len(a.browserMgr.List())
	}
	usedKeys := a.config.App.UsedCDKeys
	if usedKeys == nil {
		usedKeys = []string{}
	}

	return LicenseStatus{
		MaxLimit:  a.config.App.MaxProfileLimit,
		UsedCount: profilesCount,
		UsedKeys:  usedKeys,
	}
}

// RedeemCDKey 验证并核销兑换码，成功返回新配置
func (a *App) RedeemCDKey(cdkey string) error {
	if a.config == nil {
		a.config = DefaultConfig()
	}
	cdkey = strings.ToUpper(cdkey)
	// 去除所有可能的不小心复制进去的空格、制表符、换行符
	cdkey = strings.ReplaceAll(cdkey, " ", "")
	cdkey = strings.ReplaceAll(cdkey, "\t", "")
	cdkey = strings.ReplaceAll(cdkey, "\n", "")
	cdkey = strings.ReplaceAll(cdkey, "\r", "")

	if cdkey == "" {
		return fmt.Errorf("兑换码不能为空")
	}

	// 1. 基本校验机制（非常简单：比如前缀必须是 ANT-，并且后面加上一个特定的哈希位能匹配）
	// 生成规则我们在 keygen 里实现。校验规则：
	// 假设 cdkey 长这样: ANT-XXXX-XXXX-XXXX-XXXX-CHECKSUM
	// 为了最简单的极简方案，我们这就不搞太复杂的非对称，纯用带盐的 SHA256 截断作为校验和。
	if !strings.HasPrefix(cdkey, "ANT-") {
		return fmt.Errorf("无效的兑换码格式")
	}

	parts := strings.Split(cdkey, "-")
	if len(parts) < 3 {
		return fmt.Errorf("无效的兑换码长度")
	}

	// 验证校验和
	checksumIndex := len(parts) - 1
	payload := strings.Join(parts[:checksumIndex], "-") // "ANT-XXXX-XXXX..."
	expectedChecksum := generateChecksum(payload)
	actualChecksum := parts[checksumIndex]

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("无效的兑换码 (Checksum Error)")
	}

	// 2. 防重放校验
	for _, usedKey := range a.config.App.UsedCDKeys {
		if usedKey == cdkey {
			return fmt.Errorf("该兑换码已被使用过")
		}
	}

	// 3. 兑现与本地保存
	a.config.App.MaxProfileLimit += appconfig.StandardCDKeyProfileBonus
	a.config.App.UsedCDKeys = append(a.config.App.UsedCDKeys, cdkey)

	configPath := a.resolveAppPath("config.yaml")
	if _, _, err := reconcileConfigWithLocalLicense(configPath, a.config); err != nil {
		return fmt.Errorf("保存本机额度状态失败: %v", err)
	}
	if err := a.config.Save(configPath); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// generateChecksum 生成简易校验和
func generateChecksum(payload string) string {
	salt := "ANT-LITE-KEY-SALT-VER-1"
	hash := sha256.Sum256([]byte(payload + salt))
	return strings.ToUpper(hex.EncodeToString(hash[:])[0:8]) // 取前8位作为校验
}

// RedeemGithubStar 给予用户一个 github star 的一次性奖励
func (a *App) RedeemGithubStar() error {
	if a.config == nil {
		a.config = DefaultConfig()
	}
	cdkey := appconfig.GithubStarRewardKey
	// 防重复领取
	for _, usedKey := range a.config.App.UsedCDKeys {
		if usedKey == cdkey {
			return fmt.Errorf("您已经领取过 GitHub Star 的赠送额度啦！")
		}
	}

	a.config.App.UsedCDKeys = append(a.config.App.UsedCDKeys, cdkey)
	a.config.App.MaxProfileLimit += appconfig.GithubStarProfileBonus
	if minLimit := appconfig.MinimumProfileLimitForUsedKeys(a.config.App.UsedCDKeys); a.config.App.MaxProfileLimit < minLimit {
		a.config.App.MaxProfileLimit = minLimit
	}

	configPath := a.resolveAppPath("config.yaml")
	if _, _, err := reconcileConfigWithLocalLicense(configPath, a.config); err != nil {
		return fmt.Errorf("保存本机额度状态失败: %v", err)
	}
	if err := a.config.Save(configPath); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// GenerateCDKeys 供内部隐藏管理员页面使用的发卡器接口
func (a *App) GenerateCDKeys(count int) ([]string, error) {
	if count <= 0 || count > 1000 {
		return nil, fmt.Errorf("生成数量无效 (1-1000)")
	}

	var keys []string

	for i := 0; i < count; i++ {
		// 使用 crypto/rand 生成随机16字符
		charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("生成随机数失败: %w", err)
		}

		// 将随机字节映射到字符集
		for j := range b {
			b[j] = charset[int(b[j])%len(charset)]
		}

		part1 := string(b[0:4])
		part2 := string(b[4:8])
		part3 := string(b[8:12])
		part4 := string(b[12:16])

		payload := fmt.Sprintf("ANT-%s-%s-%s-%s", part1, part2, part3, part4)
		checksum := generateChecksum(payload)

		keys = append(keys, fmt.Sprintf("%s-%s", payload, checksum))
	}

	return keys, nil
}
