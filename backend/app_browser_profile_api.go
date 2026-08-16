package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"strings"
	"time"
)

type BrowserProfile = browser.Profile
type BrowserProfileInput = browser.ProfileInput
type BrowserTab = browser.Tab
type BrowserSettings = browser.Settings
type BrowserProxy = browser.Proxy
type BrowserCore = browser.Core
type BrowserCoreInput = browser.CoreInput
type BrowserCoreValidateResult = browser.CoreValidateResult
type BrowserCoreExtendedInfo = browser.CoreExtendedInfo
type BrowserProfileCopyOptions = browser.ProfileCopyOptions

// BrowserProfileList 获取所有实例列表
func (a *App) BrowserProfileList() []BrowserProfile { return a.browserMgr.List() }

// BrowserProfileListByTag 按标签筛选实例列表
func (a *App) BrowserProfileListByTag(tag string) []BrowserProfile {
	return a.browserMgr.ListByTag(tag)
}

// BrowserGetAllTags 获取所有已使用的标签
func (a *App) BrowserGetAllTags() []string {
	return a.browserMgr.GetAllTags()
}

// BrowserProfileSetKeywords 设置实例关键字
func (a *App) BrowserProfileSetKeywords(profileId string, keywords []string) (*BrowserProfile, error) {
	return a.browserMgr.SetKeywords(profileId, keywords)
}

func (a *App) BrowserProfileCreate(input BrowserProfileInput) (*BrowserProfile, error) {
	return a.browserMgr.Create(input)
}

// BrowserProfileCreateBatch 批量创建配置:名称为 prefix-编号(3位,从 startIndex 起,默认 1),
// 每个环境都会分配一套独立、唯一、自洽的指纹身份(与单个"新建配置"一致)。
// template 提供代理/内核/省内存等公共设置;其指纹参数会被忽略以保证各环境不同。
func (a *App) BrowserProfileCreateBatch(prefix string, count int, startIndex int, template BrowserProfileInput) ([]*BrowserProfile, error) {
	return a.browserMgr.CreateBatch(prefix, count, startIndex, template)
}

func (a *App) BrowserProfileUpdate(profileId string, input BrowserProfileInput) (*BrowserProfile, error) {
	return a.browserMgr.Update(profileId, input)
}

func (a *App) BrowserProfileDelete(profileId string) error { return a.browserMgr.Delete(profileId) }

// BrowserProfileTrashList 获取回收站实例列表
func (a *App) BrowserProfileTrashList() []BrowserProfile { return a.browserMgr.ListDeleted() }

// BrowserProfileRestore 从回收站恢复实例
func (a *App) BrowserProfileRestore(profileId string) (*BrowserProfile, error) {
	return a.browserMgr.Restore(profileId)
}

// BrowserProfilePermanentlyDelete 从回收站彻底删除实例
func (a *App) BrowserProfilePermanentlyDelete(profileId string) error {
	return a.browserMgr.PermanentlyDelete(profileId)
}

// BrowserProfilePermanentlyDeleteAll 清空回收站:一键彻底删除全部回收站实例,返回删除数量
func (a *App) BrowserProfilePermanentlyDeleteAll() (int, error) {
	return a.browserMgr.PermanentlyDeleteAll()
}

// BrowserProfileTrashCleanup 清理超过保留期的回收站实例
func (a *App) BrowserProfileTrashCleanup() error { return a.browserMgr.CleanupExpiredTrash() }

// BrowserProfileCopy 复制实例配置（除指纹参数外全部复制）
func (a *App) BrowserProfileCopy(profileId string, newName string) (*BrowserProfile, error) {
	return a.browserMgr.Copy(profileId, newName)
}

// BrowserProfileCopyWithMode 按模式复制实例配置。
func (a *App) BrowserProfileCopyWithMode(profileId string, newName string, mode string) (*BrowserProfile, error) {
	return a.browserMgr.CopyWithMode(profileId, newName, mode)
}

// BrowserProfileCopyWithOptions 按结构化选项复制实例配置。
func (a *App) BrowserProfileCopyWithOptions(profileId string, newName string, options BrowserProfileCopyOptions) (*BrowserProfile, error) {
	return a.browserMgr.CopyWithOptions(profileId, newName, options)
}

// migrateToSQLite 一次性迁移：若 SQLite 表为空则从旧文件导入数据，或初始化默认数据
// 迁移顺序：cores → proxies → profiles → bookmarks
func (a *App) migrateToSQLite() {
	log := logger.New("Migration")

	if cores, err := a.browserMgr.CoreDAO.List(); err == nil && len(cores) == 0 {
		if len(a.config.Browser.Cores) > 0 {
			for _, c := range a.config.Browser.Cores {
				if err := a.browserMgr.CoreDAO.Upsert(c); err != nil {
					log.Error("内核迁移失败", logger.F("core_id", c.CoreId), logger.F("error", err))
				}
			}
			log.Info("内核数据已迁移", logger.F("count", len(a.config.Browser.Cores)))
		} else {
			log.Info("内核表为空，将通过自动检测初始化")
		}
	}

	if proxies, err := a.browserMgr.ProxyDAO.List(); err == nil && len(proxies) == 0 {
		var srcProxies []browser.Proxy
		if loaded, err := config.LoadProxies(a.resolveAppPath("proxies.yaml")); err == nil && len(loaded) > 0 {
			srcProxies = loaded
		} else if len(a.config.Browser.Proxies) > 0 {
			srcProxies = a.config.Browser.Proxies
		} else {
			srcProxies = []browser.Proxy{
				{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
			}
			log.Info("代理表为空，初始化默认代理")
		}
		for _, p := range srcProxies {
			if err := a.browserMgr.ProxyDAO.Upsert(p); err != nil {
				log.Error("代理迁移失败", logger.F("proxy_id", p.ProxyId), logger.F("error", err))
			}
		}
		if len(srcProxies) > 0 {
			log.Info("代理数据已初始化", logger.F("count", len(srcProxies)))
		}
	}

	if profiles, err := a.browserMgr.ProfileDAO.List(); err == nil && len(profiles) == 0 {
		if len(a.config.Browser.Profiles) > 0 {
			for _, pc := range a.config.Browser.Profiles {
				coreId := strings.TrimSpace(pc.CoreId)
				if strings.EqualFold(coreId, "default") {
					coreId = ""
				}
				p := &browser.Profile{
					ProfileId:          pc.ProfileId,
					ProfileName:        pc.ProfileName,
					UserDataDir:        pc.UserDataDir,
					CoreId:             coreId,
					FingerprintArgs:    pc.FingerprintArgs,
					ProxyId:            pc.ProxyId,
					ProxyConfig:        pc.ProxyConfig,
					ProxyBindSourceID:  pc.ProxyBindSourceID,
					ProxyBindSourceURL: pc.ProxyBindSourceURL,
					ProxyBindName:      pc.ProxyBindName,
					ProxyBindUpdatedAt: pc.ProxyBindUpdatedAt,
					LaunchArgs:         pc.LaunchArgs,
					Tags:               pc.Tags,
					Keywords:           pc.Keywords,
					CreatedAt:          pc.CreatedAt,
					UpdatedAt:          pc.UpdatedAt,
				}
				if err := a.browserMgr.ProfileDAO.Upsert(p); err != nil {
					log.Error("实例迁移失败", logger.F("profile_id", pc.ProfileId), logger.F("error", err))
				}
			}
			log.Info("实例数据已迁移", logger.F("count", len(a.config.Browser.Profiles)))
		} else {
			log.Info("实例表为空，自动创建默认实例")
			defaultProfile := &browser.Profile{
				ProfileId:       generateUUID(),
				ProfileName:     "默认实例",
				UserDataDir:     "default",
				CoreId:          "",
				FingerprintArgs: a.config.Browser.DefaultFingerprintArgs,
				LaunchArgs:      a.config.Browser.DefaultLaunchArgs,
				Tags:            []string{"默认"},
				ProxyId:         "__direct__",
				ProxyConfig:     "direct://",
				CreatedAt:       time.Now().Format(time.RFC3339),
				UpdatedAt:       time.Now().Format(time.RFC3339),
			}
			if err := a.browserMgr.ProfileDAO.Upsert(defaultProfile); err != nil {
				log.Error("自动创建默认实例失败", logger.F("error", err))
			}
		}
	}

	if bookmarks, err := a.browserMgr.BookmarkDAO.List(); err == nil && len(bookmarks) == 0 {
		src := a.config.Browser.DefaultBookmarks
		if len(src) == 0 {
			src = []config.BrowserBookmark{
				{Name: "Google", URL: "https://www.google.com/"},
				{Name: "Gmail", URL: "https://mail.google.com/"},
				{Name: "Claude", URL: "https://claude.ai/"},
				{Name: "ChatGPT", URL: "https://chatgpt.com/"},
				{Name: "YouTube", URL: "https://www.youtube.com/"},
			}
		}
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(src); err != nil {
			log.Error("书签迁移失败", logger.F("error", err))
		} else {
			log.Info("书签数据已迁移", logger.F("count", len(src)))
		}
	}
}
