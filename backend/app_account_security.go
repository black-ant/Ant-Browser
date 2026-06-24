package backend

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"

	"ant-chrome/backend/internal/crypto"
	"ant-chrome/backend/internal/logger"
)

// keystorePath 返回本机加密密钥文件路径（与数据库同目录）。
func (a *App) keystorePath() string {
	dbPath := "data/app.db"
	if a.config != nil {
		if p := strings.TrimSpace(a.config.Database.SQLite.Path); p != "" {
			dbPath = p
		}
	}
	return filepath.Join(filepath.Dir(a.resolveAppPath(dbPath)), "keystore.key")
}

// initAccountEncryptionKey 加载或生成本机密钥并设为加密默认密钥，随后迁移旧密文。
// 失败时终止启动：硬编码 LegacyKey 已知不安全，必须强制使用本机密钥保护账号敏感数据。
func (a *App) initAccountEncryptionKey() error {
	log := logger.New("Account")
	path := a.keystorePath()

	if key, err := os.ReadFile(path); err == nil && len(key) == 32 {
		if e := crypto.SetKey(key); e != nil {
			log.Error("设置加密密钥失败", logger.F("error", e))
			return e
		}
		log.Info("已加载本机加密密钥", logger.F("path", path))
	} else {
		key = make([]byte, 32)
		if _, e := rand.Read(key); e != nil {
			log.Error("生成本机密钥失败", logger.F("error", e))
			return e
		}
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			log.Error("创建密钥目录失败", logger.F("error", e))
			return e
		}
		if e := os.WriteFile(path, key, 0600); e != nil {
			log.Error("写入本机密钥失败", logger.F("error", e))
			return e
		}
		if e := crypto.SetKey(key); e != nil {
			log.Error("设置加密密钥失败", logger.F("error", e))
			return e
		}
		log.Info("已生成本机加密密钥", logger.F("path", path))
	}

	a.migrateAccountEncryption()
	return nil
}

// migrateAccountEncryption 把用 LegacyKey 加密的账号敏感字段重写为当前本机密钥。幂等、绝不写坏数据。
func (a *App) migrateAccountEncryption() {
	if a.db == nil {
		return
	}
	log := logger.New("Account")

	rows, err := a.db.GetConn().Query(`SELECT account_id, password_enc, cookies_enc FROM browser_accounts`)
	if err != nil {
		return
	}
	type accRow struct{ id, pw, ck string }
	var list []accRow
	for rows.Next() {
		var r accRow
		if err := rows.Scan(&r.id, &r.pw, &r.ck); err != nil {
			continue
		}
		list = append(list, r)
	}
	rows.Close()

	migrated := 0
	for _, r := range list {
		newPw, pwMig := reEncryptLegacy(r.pw)
		newCk, ckMig := reEncryptLegacy(r.ck)
		if !pwMig && !ckMig {
			continue
		}
		if _, err := a.db.GetConn().Exec(
			`UPDATE browser_accounts SET password_enc=?, cookies_enc=? WHERE account_id=?`,
			newPw, newCk, r.id); err != nil {
			log.Error("迁移账号加密失败", logger.F("account_id", r.id), logger.F("error", err))
			continue
		}
		migrated++
	}
	if migrated > 0 {
		log.Info("账号敏感字段已迁移到本机密钥", logger.F("count", migrated))
	}
}

// reEncryptLegacy 处理单个密文字段：
//   - 能用当前密钥解出 → 已是新密钥，原样返回 (enc, false)
//   - 否则用 LegacyKey 解出再用当前密钥重写 → (newEnc, true)
//   - 两者都失败（非法/未知密文）→ 保持原值 (enc, false)，绝不写坏
func reEncryptLegacy(enc string) (string, bool) {
	if strings.TrimSpace(enc) == "" {
		return enc, false
	}
	if _, err := crypto.Decrypt(enc); err == nil {
		return enc, false
	}
	plain, err := crypto.DecryptWith(enc, crypto.LegacyKey)
	if err != nil {
		return enc, false
	}
	reEnc, err := crypto.Encrypt(plain)
	if err != nil {
		return enc, false
	}
	return reEnc, true
}
