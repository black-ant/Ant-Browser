package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
	}{
		{"空字符串", ""},
		{"简单密码", "password123"},
		{"复杂密码", "P@ssw0rd!@#$%^&*()"},
		{"中文密码", "我的密码123"},
		{"长文本", "这是一个很长的测试文本，包含各种字符：!@#$%^&*()1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{"Cookie", "session=abc123; user_id=456; token=xyz789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密
			encrypted, err := Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("加密失败: %v", err)
			}

			// 空字符串应该返回空
			if tt.plaintext == "" {
				if encrypted != "" {
					t.Errorf("空字符串加密后应该返回空，但得到: %s", encrypted)
				}
				return
			}

			// 加密后不应该等于原文
			if encrypted == tt.plaintext {
				t.Errorf("加密后的数据不应该等于原文")
			}

			// 解密
			decrypted, err := Decrypt(encrypted)
			if err != nil {
				t.Fatalf("解密失败: %v", err)
			}

			// 解密后应该等于原文
			if decrypted != tt.plaintext {
				t.Errorf("解密结果不匹配:\n原文: %s\n解密: %s", tt.plaintext, decrypted)
			}
		})
	}
}

func TestEncryptDifferentEachTime(t *testing.T) {
	plaintext := "test_password"

	encrypted1, _ := Encrypt(plaintext)
	encrypted2, _ := Encrypt(plaintext)

	if encrypted1 == encrypted2 {
		t.Error("相同明文多次加密应该产生不同密文（因为nonce随机）")
	}

	// 但都应该能正确解密
	decrypted1, _ := Decrypt(encrypted1)
	decrypted2, _ := Decrypt(encrypted2)

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("两次加密的结果都应该能正确解密")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext string
	}{
		{"空字符串", ""},
		{"无效base64", "not-valid-base64!@#"},
		{"过短数据", "YWJj"}, // "abc" base64
		{"错误密文", "dGhpcyBpcyBub3QgYSB2YWxpZCBjaXBoZXJ0ZXh0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Decrypt(tt.ciphertext)

			if tt.ciphertext == "" {
				// 空字符串应该成功返回空
				if err != nil || result != "" {
					t.Error("空字符串应该成功解密为空")
				}
			} else {
				// 其他无效数据应该返回错误
				if err == nil {
					t.Error("无效密文应该返回错误")
				}
			}
		})
	}
}

func TestSetKey(t *testing.T) {
	// 保存原始密钥
	originalKey := make([]byte, len(defaultKey))
	copy(originalKey, defaultKey)
	defer func() {
		defaultKey = originalKey // 恢复原始密钥
	}()

	// 测试无效长度
	err := SetKey([]byte("short"))
	if err == nil {
		t.Error("短密钥应该返回错误")
	}

	// 测试有效密钥
	newKey := []byte("new-secret-key-must-be-32bytes!!") // 32字节
	err = SetKey(newKey)
	if err != nil {
		t.Errorf("设置有效密钥失败: %v", err)
	}

	// 使用新密钥加解密
	plaintext := "test with new key"
	encrypted, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("使用新密钥加密失败: %v", err)
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("使用新密钥解密失败: %v", err)
	}

	if decrypted != plaintext {
		t.Error("使用新密钥加解密失败")
	}
}

func TestEncryptWithDecryptWithCrossKey(t *testing.T) {
	keyA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	keyB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	plain := "session=abc123; pw=我的密码!@#"

	encA, err := EncryptWith(plain, keyA)
	if err != nil {
		t.Fatalf("EncryptWith 失败: %v", err)
	}
	// 正确密钥可解
	got, err := DecryptWith(encA, keyA)
	if err != nil || got != plain {
		t.Fatalf("DecryptWith 同密钥应成功，got=%q err=%v", got, err)
	}
	// 错误密钥必须失败（迁移逻辑据此判断密文归属）
	if _, err := DecryptWith(encA, keyB); err == nil {
		t.Fatalf("DecryptWith 异密钥应失败，却成功了")
	}
}

func TestLegacyMigrationRoundTrip(t *testing.T) {
	// 模拟：旧数据用 LegacyKey 加密，切换到新密钥后能解出并重写
	plain := "token=xyz789"
	legacyEnc, err := EncryptWith(plain, LegacyKey)
	if err != nil {
		t.Fatalf("用 LegacyKey 加密失败: %v", err)
	}
	newKey := []byte("0123456789abcdef0123456789abcdef")
	if err := SetKey(newKey); err != nil {
		t.Fatalf("SetKey 失败: %v", err)
	}
	defer SetKey(LegacyKey) // 还原，避免影响其他测试

	// 新密钥解不出旧密文
	if _, err := Decrypt(legacyEnc); err == nil {
		t.Fatalf("新密钥不应能解出旧密文")
	}
	// 用 LegacyKey 解出后用新密钥重写
	recovered, err := DecryptWith(legacyEnc, LegacyKey)
	if err != nil || recovered != plain {
		t.Fatalf("用 LegacyKey 解密失败: got=%q err=%v", recovered, err)
	}
	reEnc, err := Encrypt(recovered)
	if err != nil {
		t.Fatalf("用新密钥重新加密失败: %v", err)
	}
	final, err := Decrypt(reEnc)
	if err != nil || final != plain {
		t.Fatalf("迁移后用新密钥应能解出，got=%q err=%v", final, err)
	}
}
