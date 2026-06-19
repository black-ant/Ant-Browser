package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// LegacyKey 是历史版本使用的硬编码密钥，仅用于把旧密文迁移到本机密钥。
var LegacyKey = []byte("ant-browser-secret-key-32bytes!!")

// 默认密钥：启动时通过 SetKey 替换为本机密钥（见 backend 启动流程）。
// 未替换时回退到 LegacyKey（兼容未初始化场景）。
var defaultKey = []byte("ant-browser-secret-key-32bytes!!")

// Encrypt 用当前默认密钥加密字符串
func Encrypt(plaintext string) (string, error) {
	return EncryptWith(plaintext, defaultKey)
}

// Decrypt 用当前默认密钥解密字符串
func Decrypt(ciphertext string) (string, error) {
	return DecryptWith(ciphertext, defaultKey)
}

// EncryptWith 用指定密钥加密字符串（AES-256-GCM）
func EncryptWith(plaintext string, key []byte) (string, error) {
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

// DecryptWith 用指定密钥解密字符串
func DecryptWith(ciphertext string, key []byte) (string, error) {
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
