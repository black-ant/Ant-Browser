package backend

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"

	"golang.org/x/crypto/scrypt"
)

const (
	portableSessionVersion       = 1
	portableSessionDir           = "portable-session"
	portableSessionKDF           = "scrypt"
	portableSessionPasswordBytes = 8
	portableSessionStartTimeout  = 20 * time.Second
	portableSessionEntryLimit    = 16 * 1024 * 1024
)

type portableProfileSession struct {
	Version   int          `json:"version"`
	ProfileID string       `json:"profileId"`
	Cookies   []CookieInfo `json:"cookies"`
}

type portableSessionEnvelope struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	KDF        string `json:"kdf"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func portableSessionArchivePath(profileID string) string {
	return portableSessionDir + "/" + profileID + ".json"
}

func validatePortableSessionPassword(password string) error {
	if len([]rune(password)) < portableSessionPasswordBytes {
		return fmt.Errorf("登录态迁移密码至少需要 %d 个字符", portableSessionPasswordBytes)
	}
	return nil
}

func derivePortableSessionKey(password string, salt []byte) ([]byte, error) {
	if err := validatePortableSessionPassword(password); err != nil {
		return nil, err
	}
	return scrypt.Key([]byte(password), salt, 1<<15, 8, 1, 32)
}

func encryptPortableProfileSession(profileID string, cookies []CookieInfo, password string) ([]byte, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, fmt.Errorf("登录态迁移缺少实例 ID")
	}
	if err := validatePortableSessionPassword(password); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(portableProfileSession{
		Version:   portableSessionVersion,
		ProfileID: profileID,
		Cookies:   cookies,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化登录态失败: %w", err)
	}

	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("生成登录态迁移盐失败: %w", err)
	}
	key, err := derivePortableSessionKey(password, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建登录态加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建登录态认证加密器失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成登录态迁移随机数失败: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, payload, []byte(profileID))

	return json.Marshal(portableSessionEnvelope{
		Format:     profilePackageFormat,
		Version:    portableSessionVersion,
		KDF:        portableSessionKDF,
		Salt:       base64.RawStdEncoding.EncodeToString(salt),
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	})
}

func decryptPortableProfileSession(profileID string, encrypted []byte, password string) ([]CookieInfo, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, fmt.Errorf("登录态迁移缺少实例 ID")
	}
	if err := validatePortableSessionPassword(password); err != nil {
		return nil, err
	}

	var envelope portableSessionEnvelope
	if err := json.Unmarshal(encrypted, &envelope); err != nil {
		return nil, fmt.Errorf("登录态迁移文件格式无效: %w", err)
	}
	if envelope.Format != profilePackageFormat || envelope.Version != portableSessionVersion || envelope.KDF != portableSessionKDF {
		return nil, fmt.Errorf("不支持的登录态迁移文件格式")
	}
	salt, err := base64.RawStdEncoding.DecodeString(envelope.Salt)
	if err != nil || len(salt) == 0 {
		return nil, fmt.Errorf("登录态迁移文件的盐无效")
	}
	nonce, err := base64.RawStdEncoding.DecodeString(envelope.Nonce)
	if err != nil || len(nonce) == 0 {
		return nil, fmt.Errorf("登录态迁移文件的随机数无效")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil || len(ciphertext) == 0 {
		return nil, fmt.Errorf("登录态迁移文件的密文无效")
	}

	key, err := derivePortableSessionKey(password, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建登录态解密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建登录态认证解密器失败: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("登录态迁移文件的随机数长度无效")
	}
	payload, err := gcm.Open(nil, nonce, ciphertext, []byte(profileID))
	if err != nil {
		return nil, fmt.Errorf("登录态迁移密码错误或文件已损坏")
	}

	var session portableProfileSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("解析登录态数据失败: %w", err)
	}
	if session.Version != portableSessionVersion || !strings.EqualFold(strings.TrimSpace(session.ProfileID), profileID) {
		return nil, fmt.Errorf("登录态数据与实例不匹配")
	}
	return session.Cookies, nil
}

func (a *App) collectPortableProfileSessions(profiles []browser.Profile) (map[string][]CookieInfo, error) {
	sessions := make(map[string][]CookieInfo, len(profiles))
	for _, profile := range profiles {
		profileID := strings.TrimSpace(profile.ProfileId)
		if profileID == "" {
			return nil, fmt.Errorf("实例缺少 profileId，无法迁移登录态")
		}

		startedProfile, startErr := a.browserInstanceStartInternal(
			profileID,
			nil,
			[]string{"about:blank"},
			true,
			false,
			true,
			"",
			"",
		)
		started := startedProfile != nil && (startedProfile.Running || startedProfile.DebugPort > 0)
		if startErr != nil {
			if started {
				_, _ = a.BrowserInstanceStop(profileID)
			}
			return nil, fmt.Errorf("实例「%s」启动失败，无法读取登录态: %w", profile.ProfileName, startErr)
		}
		if !started {
			return nil, fmt.Errorf("实例「%s」未启动，无法读取登录态", profile.ProfileName)
		}

		var cookies []CookieInfo
		getErr := func() error {
			if startedProfile != nil && !startedProfile.DebugReady && startedProfile.DebugPort > 0 {
				readyProfile, ready := a.waitForBrowserDebugReady(profileID, startedProfile.DebugPort, portableSessionStartTimeout)
				if !ready || readyProfile == nil || !readyProfile.DebugReady {
					return fmt.Errorf("实例调试接口未就绪")
				}
			}
			var err error
			cookies, err = a.BrowserGetCookies(profileID)
			return err
		}()
		stopProfile, stopErr := a.BrowserInstanceStop(profileID)
		if getErr != nil {
			return nil, fmt.Errorf("实例「%s」读取登录态失败: %w", profile.ProfileName, getErr)
		}
		if stopErr != nil || (stopProfile != nil && stopProfile.Running) {
			if stopErr == nil {
				stopErr = fmt.Errorf("浏览器停止后仍处于运行状态")
			}
			return nil, fmt.Errorf("实例「%s」停止失败，未生成迁移包: %w", profile.ProfileName, stopErr)
		}
		sessions[profileID] = cookies
	}
	return sessions, nil
}

func readPortableProfileSessionFile(fileReader io.Reader, fileSize uint64, profileID string, password string) ([]CookieInfo, error) {
	if fileSize > portableSessionEntryLimit {
		return nil, fmt.Errorf("实例登录态迁移文件过大")
	}
	data, err := io.ReadAll(io.LimitReader(fileReader, portableSessionEntryLimit+1))
	if err != nil {
		return nil, fmt.Errorf("读取登录态迁移文件失败: %w", err)
	}
	if int64(len(data)) > portableSessionEntryLimit {
		return nil, fmt.Errorf("实例登录态迁移文件过大")
	}
	return decryptPortableProfileSession(profileID, data, password)
}

func (a *App) restorePortableProfileSessions(prepared []preparedProfilePackageImport, sessions map[string][]CookieInfo, warnings *[]string) {
	for _, item := range prepared {
		cookies, exists := sessions[item.OldProfileID]
		if !exists {
			continue
		}
		profileID := strings.TrimSpace(item.Profile.ProfileId)
		startedProfile, startErr := a.browserInstanceStartInternal(
			profileID,
			nil,
			[]string{"about:blank"},
			true,
			false,
			true,
			"",
			"",
		)
		started := startedProfile != nil && (startedProfile.Running || startedProfile.DebugPort > 0)
		if startErr != nil || !started {
			if started {
				_, _ = a.BrowserInstanceStop(profileID)
			}
			appendPortableSessionWarning(warnings, item.Profile.ProfileName, fmt.Sprintf("目标实例未能启动，登录态未恢复：%v", startErr))
			continue
		}

		if startedProfile != nil && !startedProfile.DebugReady && startedProfile.DebugPort > 0 {
			readyProfile, ready := a.waitForBrowserDebugReady(profileID, startedProfile.DebugPort, portableSessionStartTimeout)
			if !ready || readyProfile == nil || !readyProfile.DebugReady {
				appendPortableSessionWarning(warnings, item.Profile.ProfileName, "目标实例调试接口未就绪，登录态未恢复")
				_, _ = a.BrowserInstanceStop(profileID)
				continue
			}
		}
		setErr := a.BrowserSetCookies(profileID, cookies)
		_, stopErr := a.BrowserInstanceStop(profileID)
		if setErr != nil {
			appendPortableSessionWarning(warnings, item.Profile.ProfileName, fmt.Sprintf("Cookie 写入失败，登录态未恢复：%v", setErr))
			continue
		}
		if stopErr != nil {
			appendPortableSessionWarning(warnings, item.Profile.ProfileName, fmt.Sprintf("登录态已写入，但实例停止失败：%v", stopErr))
		}
	}
}

func appendPortableSessionWarning(warnings *[]string, profileName string, message string) {
	if warnings == nil {
		return
	}
	name := strings.TrimSpace(profileName)
	if name == "" {
		name = "未命名实例"
	}
	*warnings = append(*warnings, fmt.Sprintf("实例「%s」%s", name, message))
}
