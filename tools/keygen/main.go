package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	fmt.Println("=== ZwBrowser 兑换码生成器 ===")
	fmt.Println("生成 5 个有效兑换码 (每个可增加 10 额度):")
	fmt.Println(strings.Repeat("-", 30))

	for i := 0; i < 5; i++ {
		key := generateKey()
		fmt.Println(key)
	}
	fmt.Println(strings.Repeat("-", 30))
}

func generateKey() string {
	// A basic random 16-char string ABCDEFGH-IJKLMNOP...
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	part1 := string(b[0:4])
	part2 := string(b[4:8])
	part3 := string(b[8:12])
	part4 := string(b[12:16])

	payload := fmt.Sprintf("ANT-%s-%s-%s-%s", part1, part2, part3, part4)
	checksum := generateChecksum(payload)

	return fmt.Sprintf("%s-%s", payload, checksum)
}

// 这里的生成规则需要和 app_license.go 里的一致
func generateChecksum(payload string) string {
	salt := "ANT-LITE-KEY-SALT-VER-1"
	hash := sha256.Sum256([]byte(payload + salt))
	return strings.ToUpper(hex.EncodeToString(hash[:])[0:8]) // 取前8位作为校验
}
