package backend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/crypto"
	"ant-chrome/backend/internal/logger"
)

// BrowserAccount 浏览器账号
type BrowserAccount struct {
	AccountID         string   `json:"accountId"`
	AccountName       string   `json:"accountName"`
	Platform          string   `json:"platform"`
	Username          string   `json:"username"`
	Email             string   `json:"email"`
	Password          string   `json:"password"`           // 明文（仅用于传输，不存储）
	RelatedProfileIDs []string `json:"relatedProfileIds"` // 关联的实例ID
	Notes             string   `json:"notes"`
	Cookies           string   `json:"cookies"`   // 明文Cookie（仅用于传输）
	CreatedAt         string   `json:"createdAt"` // ISO8601格式
	UpdatedAt         string   `json:"updatedAt"`
	CookieCount       int      `json:"cookieCount"`        // 已存 Cookie 条数（非敏感，用于提醒）
	CookieEarliestExpiry int64 `json:"cookieEarliestExpiry"` // 最早过期 unix 秒；0=无/全 Session
}

// BrowserAccountInput 账号输入（用于创建和更新）
type BrowserAccountInput struct {
	AccountName       string   `json:"accountName"`
	Platform          string   `json:"platform"`
	Username          string   `json:"username"`
	Email             string   `json:"email"`
	Password          string   `json:"password"`
	RelatedProfileIDs []string `json:"relatedProfileIds"`
	Notes             string   `json:"notes"`
	Cookies           string   `json:"cookies"`
}

// BrowserAccountCreate 创建账号
func (a *App) BrowserAccountCreate(input BrowserAccountInput) (*BrowserAccount, error) {
	if input.AccountName == "" {
		return nil, fmt.Errorf("账号名称不能为空")
	}

	accountID := fmt.Sprintf("account-%s", generateUUID()[:8])
	now := time.Now().UTC().Format(time.RFC3339)

	// 加密敏感字段
	passwordEnc, err := crypto.Encrypt(input.Password)
	if err != nil {
		return nil, fmt.Errorf("加密密码失败: %w", err)
	}

	cookiesEnc, err := crypto.Encrypt(input.Cookies)
	if err != nil {
		return nil, fmt.Errorf("加密Cookies失败: %w", err)
	}

	// 序列化关联实例ID
	relatedIDsJSON, err := json.Marshal(input.RelatedProfileIDs)
	if err != nil {
		return nil, fmt.Errorf("序列化关联实例ID失败: %w", err)
	}

	// 插入数据库
	cookieCount, cookieEarliest := cookieStats(input.Cookies)
	_, err = a.db.GetConn().Exec(`
		INSERT INTO browser_accounts (
			account_id, account_name, platform, username, email,
			password_enc, related_profile_ids, notes, cookies_enc,
			created_at, updated_at, cookie_count, cookie_earliest_expiry
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, accountID, input.AccountName, input.Platform, input.Username, input.Email,
		passwordEnc, string(relatedIDsJSON), input.Notes, cookiesEnc, now, now, cookieCount, cookieEarliest)

	if err != nil {
		return nil, fmt.Errorf("插入账号失败: %w", err)
	}

	logger.New("Account").Info("[Account] 账号已创建", logger.F("account_id", accountID), logger.F("account_name", input.AccountName))

	return &BrowserAccount{
		AccountID:         accountID,
		AccountName:       input.AccountName,
		Platform:          input.Platform,
		Username:          input.Username,
		Email:             input.Email,
		Password:          "", // 不返回密码明文
		RelatedProfileIDs: input.RelatedProfileIDs,
		Notes:             input.Notes,
		Cookies:           "", // 不返回Cookie明文
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// BrowserAccountList 获取账号列表
func (a *App) BrowserAccountList() ([]BrowserAccount, error) {
	rows, err := a.db.GetConn().Query(`
		SELECT account_id, account_name, platform, username, email,
		       password_enc, related_profile_ids, notes, cookies_enc,
		       created_at, updated_at, cookie_count, cookie_earliest_expiry
		FROM browser_accounts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询账号列表失败: %w", err)
	}

	// 现有 profile id 集合，用于惰性清理无效关联
	validProfiles := map[string]bool{}
	a.browserMgr.Mutex.Lock()
	for id := range a.browserMgr.Profiles {
		validProfiles[id] = true
	}
	a.browserMgr.Mutex.Unlock()

	var accounts []BrowserAccount
	cleanups := map[string]string{} // accountID -> 清理后的 relatedIDs JSON
	for rows.Next() {
		var account BrowserAccount
		var passwordEnc, cookiesEnc, relatedIDsJSON string

		err := rows.Scan(
			&account.AccountID, &account.AccountName, &account.Platform,
			&account.Username, &account.Email, &passwordEnc, &relatedIDsJSON,
			&account.Notes, &cookiesEnc, &account.CreatedAt, &account.UpdatedAt,
			&account.CookieCount, &account.CookieEarliestExpiry,
		)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("扫描账号数据失败: %w", err)
		}

		// 列表不返回敏感明文
		account.Password = ""
		account.Cookies = ""

		// 反序列化关联实例ID
		var related []string
		if err := json.Unmarshal([]byte(relatedIDsJSON), &related); err != nil {
			related = []string{}
		}
		// 惰性清理：移除已不存在的 profile id
		filtered := make([]string, 0, len(related))
		for _, id := range related {
			if validProfiles[id] {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) != len(related) {
			if cleaned, e := json.Marshal(filtered); e == nil {
				cleanups[account.AccountID] = string(cleaned)
			}
		}
		account.RelatedProfileIDs = filtered

		accounts = append(accounts, account)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, fmt.Errorf("遍历账号列表失败: %w", rowsErr)
	}

	// 持久化清理结果（rows 关闭后再写，避免单连接占用）
	for accountID, cleaned := range cleanups {
		if _, e := a.db.GetConn().Exec(`UPDATE browser_accounts SET related_profile_ids = ? WHERE account_id = ?`, cleaned, accountID); e != nil {
			logger.New("Account").Error("清理无效关联失败", logger.F("account_id", accountID), logger.F("error", e))
		}
	}

	logger.New("Account").Info("[Account] 账号列表查询", logger.F("count", len(accounts)))
	return accounts, nil
}

// BrowserAccountGet 获取单个账号详情（包含敏感信息）
func (a *App) BrowserAccountGet(accountID string) (*BrowserAccount, error) {
	var account BrowserAccount
	var passwordEnc, cookiesEnc, relatedIDsJSON string

	err := a.db.GetConn().QueryRow(`
		SELECT account_id, account_name, platform, username, email,
		       password_enc, related_profile_ids, notes, cookies_enc,
		       created_at, updated_at
		FROM browser_accounts
		WHERE account_id = ?
	`, accountID).Scan(
		&account.AccountID, &account.AccountName, &account.Platform,
		&account.Username, &account.Email, &passwordEnc, &relatedIDsJSON,
		&account.Notes, &cookiesEnc, &account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("账号不存在: %s", accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("查询账号失败: %w", err)
	}

	// 解密敏感字段
	account.Password, err = crypto.Decrypt(passwordEnc)
	if err != nil {
		return nil, fmt.Errorf("解密密码失败: %w", err)
	}

	account.Cookies, err = crypto.Decrypt(cookiesEnc)
	if err != nil {
		return nil, fmt.Errorf("解密Cookies失败: %w", err)
	}

	// 反序列化关联实例ID
	if err := json.Unmarshal([]byte(relatedIDsJSON), &account.RelatedProfileIDs); err != nil {
		account.RelatedProfileIDs = []string{}
	}

	return &account, nil
}

// BrowserAccountUpdate 更新账号
func (a *App) BrowserAccountUpdate(accountID string, input BrowserAccountInput) (*BrowserAccount, error) {
	// 检查账号是否存在
	var exists bool
	err := a.db.GetConn().QueryRow("SELECT 1 FROM browser_accounts WHERE account_id = ?", accountID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("账号不存在: %s", accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("检查账号失败: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// 加密敏感字段
	passwordEnc, err := crypto.Encrypt(input.Password)
	if err != nil {
		return nil, fmt.Errorf("加密密码失败: %w", err)
	}

	cookiesEnc, err := crypto.Encrypt(input.Cookies)
	if err != nil {
		return nil, fmt.Errorf("加密Cookies失败: %w", err)
	}

	// 序列化关联实例ID
	relatedIDsJSON, err := json.Marshal(input.RelatedProfileIDs)
	if err != nil {
		return nil, fmt.Errorf("序列化关联实例ID失败: %w", err)
	}

	// 更新数据库
	cookieCount, cookieEarliest := cookieStats(input.Cookies)
	_, err = a.db.GetConn().Exec(`
		UPDATE browser_accounts
		SET account_name = ?, platform = ?, username = ?, email = ?,
		    password_enc = ?, related_profile_ids = ?, notes = ?,
		    cookies_enc = ?, updated_at = ?, cookie_count = ?, cookie_earliest_expiry = ?
		WHERE account_id = ?
	`, input.AccountName, input.Platform, input.Username, input.Email,
		passwordEnc, string(relatedIDsJSON), input.Notes, cookiesEnc, now, cookieCount, cookieEarliest, accountID)

	if err != nil {
		return nil, fmt.Errorf("更新账号失败: %w", err)
	}

	logger.New("Account").Info("[Account] 账号已更新", logger.F("account_id", accountID), logger.F("account_name", input.AccountName))

	return a.BrowserAccountGet(accountID)
}

// BrowserAccountDelete 删除账号
func (a *App) BrowserAccountDelete(accountID string) error {
	result, err := a.db.GetConn().Exec("DELETE FROM browser_accounts WHERE account_id = ?", accountID)
	if err != nil {
		return fmt.Errorf("删除账号失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("账号不存在: %s", accountID)
	}

	logger.New("Account").Info("[Account] 账号已删除", logger.F("account_id", accountID))
	return nil
}

// ============================================================================
// Cookie 同步 / 关联 / 导入导出
// ============================================================================

// cookieStats 从明文 Cookie（JSON 数组）解析条数与最早过期时间（unix 秒；0=无/全 Session）
func cookieStats(cookiesPlain string) (int, int64) {
	s := strings.TrimSpace(cookiesPlain)
	if s == "" {
		return 0, 0
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return 0, 0
	}
	var earliest int64
	for _, c := range arr {
		exp := cookieExpiryOf(c)
		if exp <= 0 {
			continue
		}
		if earliest == 0 || exp < earliest {
			earliest = exp
		}
	}
	return len(arr), earliest
}

func cookieExpiryOf(c map[string]any) int64 {
	for _, k := range []string{"expires", "expirationDate"} {
		if v, ok := c[k]; ok {
			switch n := v.(type) {
			case float64:
				return int64(n)
			case int64:
				return n
			case int:
				return int64(n)
			}
		}
	}
	return 0
}

// BrowserAccountSaveCookiesFromProfile 从运行中实例读取 Cookie 并加密保存到账号
func (a *App) BrowserAccountSaveCookiesFromProfile(accountID string, profileID string) (int, error) {
	cookies, err := a.BrowserGetCookies(profileID)
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(cookies)
	if err != nil {
		return 0, fmt.Errorf("序列化 Cookie 失败: %w", err)
	}
	enc, err := crypto.Encrypt(string(data))
	if err != nil {
		return 0, fmt.Errorf("加密 Cookie 失败: %w", err)
	}
	count, earliest := cookieStats(string(data))
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := a.db.GetConn().Exec(
		`UPDATE browser_accounts SET cookies_enc=?, updated_at=?, cookie_count=?, cookie_earliest_expiry=? WHERE account_id=?`,
		enc, now, count, earliest, accountID)
	if err != nil {
		return 0, fmt.Errorf("保存 Cookie 失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, fmt.Errorf("账号不存在: %s", accountID)
	}
	logger.New("Account").Info("[Account] 从实例读取并保存 Cookie",
		logger.F("account_id", accountID), logger.F("profile_id", profileID), logger.F("count", count))
	return count, nil
}

// BrowserAccountRestoreCookiesToProfile 把账号已存 Cookie 解密后回写到运行中实例
func (a *App) BrowserAccountRestoreCookiesToProfile(accountID string, profileID string, clearFirst bool) (int, error) {
	acc, err := a.BrowserAccountGet(accountID)
	if err != nil {
		return 0, err
	}
	plain := strings.TrimSpace(acc.Cookies)
	if plain == "" {
		return 0, fmt.Errorf("该账号未保存 Cookie")
	}
	var cookies []CookieInfo
	if err := json.Unmarshal([]byte(plain), &cookies); err != nil {
		return 0, fmt.Errorf("解析已存 Cookie 失败: %w", err)
	}
	if clearFirst {
		_ = a.BrowserClearCookies(profileID)
	}
	if err := a.BrowserSetCookies(profileID, cookies); err != nil {
		return 0, err
	}
	logger.New("Account").Info("[Account] 回写 Cookie 到实例",
		logger.F("account_id", accountID), logger.F("profile_id", profileID), logger.F("count", len(cookies)))
	return len(cookies), nil
}

// BrowserAccountSetProfiles 设置账号关联的实例集合（过滤无效/重复 id）
func (a *App) BrowserAccountSetProfiles(accountID string, profileIDs []string) error {
	valid := map[string]bool{}
	a.browserMgr.Mutex.Lock()
	for id := range a.browserMgr.Profiles {
		valid[id] = true
	}
	a.browserMgr.Mutex.Unlock()

	filtered := make([]string, 0, len(profileIDs))
	seen := map[string]bool{}
	for _, id := range profileIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || !valid[id] {
			continue
		}
		seen[id] = true
		filtered = append(filtered, id)
	}
	data, _ := json.Marshal(filtered)
	res, err := a.db.GetConn().Exec(
		`UPDATE browser_accounts SET related_profile_ids=?, updated_at=? WHERE account_id=?`,
		string(data), time.Now().UTC().Format(time.RFC3339), accountID)
	if err != nil {
		return fmt.Errorf("更新关联失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("账号不存在: %s", accountID)
	}
	return nil
}

// accountExport 账号导入/导出结构；敏感字段用 omitempty，脱敏导出时省略
type accountExport struct {
	AccountName       string   `json:"accountName"`
	Platform          string   `json:"platform"`
	Username          string   `json:"username"`
	Email             string   `json:"email"`
	Notes             string   `json:"notes"`
	RelatedProfileIDs []string `json:"relatedProfileIds"`
	Password          string   `json:"password,omitempty"`
	Cookies           string   `json:"cookies,omitempty"`
}

// BrowserAccountExport 导出账号 JSON；includeSecrets=false 时不含密码/Cookie（仅含敏感时才解密）
func (a *App) BrowserAccountExport(accountIDs []string, includeSecrets bool) (string, error) {
	accounts, err := a.BrowserAccountList()
	if err != nil {
		return "", err
	}
	idset := map[string]bool{}
	for _, id := range accountIDs {
		idset[id] = true
	}
	out := make([]accountExport, 0, len(accounts))
	for _, acc := range accounts {
		if len(accountIDs) > 0 && !idset[acc.AccountID] {
			continue
		}
		e := accountExport{
			AccountName:       acc.AccountName,
			Platform:          acc.Platform,
			Username:          acc.Username,
			Email:             acc.Email,
			Notes:             acc.Notes,
			RelatedProfileIDs: acc.RelatedProfileIDs,
		}
		if includeSecrets {
			if full, err := a.BrowserAccountGet(acc.AccountID); err == nil {
				e.Password = full.Password
				e.Cookies = full.Cookies
			}
		}
		out = append(out, e)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化导出失败: %w", err)
	}
	logger.New("Account").Info("[Account] 导出账号", logger.F("count", len(out)), logger.F("include_secrets", includeSecrets))
	if includeSecrets {
		a.recordActivity("export", "warn", fmt.Sprintf("导出账号（含敏感数据）%d 条", len(out)), "")
	} else {
		a.recordActivity("export", "info", fmt.Sprintf("导出账号（脱敏）%d 条", len(out)), "")
	}
	return string(data), nil
}

// BrowserAccountImport 从 JSON 批量导入账号（敏感字段按现有路径加密落库）
func (a *App) BrowserAccountImport(payload string) (int, error) {
	var items []accountExport
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		return 0, fmt.Errorf("解析导入内容失败: %w", err)
	}
	count := 0
	for _, it := range items {
		name := strings.TrimSpace(it.AccountName)
		if name == "" {
			continue
		}
		if _, err := a.BrowserAccountCreate(BrowserAccountInput{
			AccountName:       name,
			Platform:          it.Platform,
			Username:          it.Username,
			Email:             it.Email,
			Password:          it.Password,
			RelatedProfileIDs: it.RelatedProfileIDs,
			Notes:             it.Notes,
			Cookies:           it.Cookies,
		}); err != nil {
			logger.New("Account").Error("导入账号失败", logger.F("name", name), logger.F("error", err))
			continue
		}
		count++
	}
	logger.New("Account").Info("[Account] 批量导入账号", logger.F("count", count))
	a.recordActivity("import", "info", fmt.Sprintf("批量导入账号：成功 %d / %d 条", count, len(items)), "")
	return count, nil
}
