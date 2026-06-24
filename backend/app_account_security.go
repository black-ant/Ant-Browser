package backend

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"

	"ant-chrome/backend/internal/crypto"
	"ant-chrome/backend/internal/fileutil"
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
		if e := fileutil.SecureFileWrite(path, key); e != nil {
			log.Error("写入本机密钥失败", logger.F("error", e))
			return e
		}
		if e := crypto.SetKey(key); e != nil {
			log.Error("设置加密密钥失败", logger.F("error", e))
			return e
		}
		log.Info("已生成本机加密密钥", logger.F("path", path))
		// 审计日志：记录密钥生成事件
		log.Info("安全审计：本机加密密钥已生成", logger.F("path", path))
	}

	a.migrateAccountEncryption()
	return nil
}

// migrateAccountEncryption 把用 LegacyKey 加密的账号敏感字段重写为当前本机密钥。
// 使用版本检测机制，强制将所有v1或无版本前缀的密文迁移到v2（本机密钥）。
// 幂等、绝不写坏数据。
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
		newPw, pwMig := reEncryptToV2(r.pw)
		newCk, ckMig := reEncryptToV2(r.ck)
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
		log.Info("账号敏感字段已迁移到v2加密（本机密钥）", logger.F("count", migrated))
		// 审计日志：记录加密迁移事件
		log.Info("安全审计：账号数据加密已迁移", logger.F("migrated_count", migrated), logger.F("encryption_version", "v2"))
	}
}

// reEncryptToV2 处理单个密文字段，强制迁移到v2版本：
//   - 已是v2版本 → 原样返回 (enc, false)
//   - v1版本或无版本 → 解密后用v2重新加密 → (newEnc, true)
//   - 解密失败 → 保持原值 (enc, false)，绝不写坏
func reEncryptToV2(enc string) (string, bool) {
	if strings.TrimSpace(enc) == "" {
		return enc, false
	}

	// 检查是否需要迁移
	if !crypto.NeedsMigration(enc) {
		return enc, false
	}

	// 尝试解密（自动处理v1/无版本）
	plain, err := crypto.Decrypt(enc)
	if err != nil {
		// 解密失败，保持原值
		return enc, false
	}

	// 用当前密钥重新加密为v2版本
	reEnc, err := crypto.Encrypt(plain)
	if err != nil {
		// 重新加密失败，保持原值
		return enc, false
	}

	return reEnc, true
}
