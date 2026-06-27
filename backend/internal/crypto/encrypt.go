package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const (
	// 加密版本标识
	encryptionVersionV1 = "v1:" // 使用 LegacyKey 的旧版本（不安全）
	encryptionVersionV2 = "v2:" // 使用本机密钥的新版本（安全）
)

// LegacyKey 是历史版本使用的硬编码密钥，仅用于把旧密文迁移到本机密钥。
// 警告：此密钥已公开，所有使用此密钥加密的数据均不安全。
// 新数据必须使用本机生成的随机密钥（通过 SetKey 设置）。
var LegacyKey = []byte("ant-browser-secret-key-32bytes!!")

// 默认密钥：启动时通过 SetKey 替换为本机密钥（见 backend 启动流程）。
// 未替换时回退到 LegacyKey（兼容未初始化场景）。
var defaultKey = []byte("ant-browser-secret-key-32bytes!!")

// Encrypt 用当前默认密钥加密字符串，并添加v2版本前缀
func Encrypt(plaintext string) (string, error) {
	// 空明文直接返回空，不加版本前缀（否则 "v2:" 会污染存储，
	// 且 NeedsMigration 会把它误判为已迁移）。
	if plaintext == "" {
		return "", nil
	}
	ciphertext, err := encryptWith(plaintext, defaultKey)
	if err != nil {
		return "", err
	}
	// 添加版本前缀，标识使用的是本机密钥
	return encryptionVersionV2 + ciphertext, nil
}

// Decrypt 用当前默认密钥解密字符串，自动检测版本
func Decrypt(ciphertext string) (string, error) {
	// 检测加密版本
	if strings.HasPrefix(ciphertext, encryptionVersionV2) {
		// v2: 使用当前密钥（本机密钥）
		ciphertext = strings.TrimPrefix(ciphertext, encryptionVersionV2)
		return decryptWith(ciphertext, defaultKey)
	} else if strings.HasPrefix(ciphertext, encryptionVersionV1) {
		// v1: 使用 LegacyKey（已废弃，仅用于迁移）
		ciphertext = strings.TrimPrefix(ciphertext, encryptionVersionV1)
		return decryptWith(ciphertext, LegacyKey)
	}

	// 无版本前缀：假定为 LegacyKey 加密（旧数据兼容）
	// 尝试当前密钥
	if plaintext, err := decryptWith(ciphertext, defaultKey); err == nil {
		return plaintext, nil
	}
	// 回退到 LegacyKey
	return decryptWith(ciphertext, LegacyKey)
}

// EncryptWith 用指定密钥加密字符串（AES-256-GCM），无版本前缀
func EncryptWith(plaintext string, key []byte) (string, error) {
	ciphertext, err := encryptWith(plaintext, key)
	if err != nil {
		return "", err
	}
	// 使用 LegacyKey 时添加 v1 前缀标识
	if string(key) == string(LegacyKey) {
		return encryptionVersionV1 + ciphertext, nil
	}
	return ciphertext, nil
}

// DecryptWith 用指定密钥解密字符串
func DecryptWith(ciphertext string, key []byte) (string, error) {
	// 移除版本前缀（如果存在）
	ciphertext = strings.TrimPrefix(ciphertext, encryptionVersionV1)
	ciphertext = strings.TrimPrefix(ciphertext, encryptionVersionV2)
	return decryptWith(ciphertext, key)
}

// encryptWith 内部加密函数（无版本前缀）
func encryptWith(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建cipher失败: %w", err)
	}

	// 使用GCM模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}

	// 创建随机nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}

	// 加密
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptWith 内部解密函数（无版本前缀处理）
func decryptWith(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Base64解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64解码失败: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建cipher失败: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文数据过短")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// SetKey 设置自定义加密密钥（必须是32字节）
func SetKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("密钥长度必须是32字节，当前: %d", len(key))
	}
	defaultKey = key
	return nil
}

// NeedsMigration 检查密文是否需要迁移到v2版本
func NeedsMigration(ciphertext string) bool {
	// v1前缀或无前缀的旧数据都需要迁移
	return strings.HasPrefix(ciphertext, encryptionVersionV1) ||
		(!strings.HasPrefix(ciphertext, encryptionVersionV2) && ciphertext != "")
}
