package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"
)

type BrowserProfile = browser.Profile
type BrowserProfileInput = browser.ProfileInput
type BrowserTab = browser.Tab
type BrowserSettings = browser.Settings
type BrowserProxy = browser.Proxy
type BrowserProxyVirtualGroup = browser.ProxyVirtualGroup
type BrowserCore = browser.Core
type BrowserCoreInput = browser.CoreInput
type BrowserCoreValidateResult = browser.CoreValidateResult
type BrowserCoreExtendedInfo = browser.CoreExtendedInfo
type BrowserProfileCopyOptions = browser.ProfileCopyOptions

// BrowserProfileList 获取所有实例列表
func (a *App) BrowserProfileList() []BrowserProfile {
	if a == nil || a.browserMgr == nil {
		return []BrowserProfile{}
	}
	return a.browserMgr.List()
}

// BrowserProfileListByTag 按标签筛选实例列表
func (a *App) BrowserProfileListByTag(tag string) []BrowserProfile {
	if a == nil || a.browserMgr == nil {
		return []BrowserProfile{}
	}
	return a.browserMgr.ListByTag(tag)
}

// BrowserGetAllTags 获取所有已使用的标签
func (a *App) BrowserGetAllTags() []string {
	if a == nil || a.browserMgr == nil {
		return []string{}
	}
	return a.browserMgr.GetAllTags()
}

// BrowserProfileSetKeywords 设置实例关键字
func (a *App) BrowserProfileSetKeywords(profileId string, keywords []string) (*BrowserProfile, error) {
	return a.browserMgr.SetKeywords(profileId, keywords)
}

func (a *App) BrowserProfileCreate(input BrowserProfileInput) (*BrowserProfile, error) {
	return a.browserMgr.Create(input)
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
func (a *App) migrateToSQLite() error {
	if a == nil || a.config == nil || a.browserMgr == nil {
		return fmt.Errorf(`SQLite 数据迁移上下文不可用`)
	}
	log := logger.New("Migration")

	cores, err := a.browserMgr.CoreDAO.List()
	if err != nil {
		return fmt.Errorf(`读取内核数据失败: %w`, err)
	}
	if len(cores) == 0 {
		if len(a.config.Browser.Cores) > 0 {
			for _, c := range a.config.Browser.Cores {
				if err := a.browserMgr.CoreDAO.Upsert(c); err != nil {
					return fmt.Errorf(`迁移内核 %q 失败: %w`, c.CoreId, err)
				}
			}
			log.Info("内核数据已迁移", logger.F("count", len(a.config.Browser.Cores)))
		} else {
			log.Info("内核表为空，将通过自动检测初始化")
		}
	}

	proxies, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return fmt.Errorf(`读取代理数据失败: %w`, err)
	}
	if len(proxies) == 0 {
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
		if err := a.browserMgr.ProxyDAO.ReplaceAll(srcProxies); err != nil {
			return fmt.Errorf(`初始化代理数据失败: %w`, err)
		}
		if len(srcProxies) > 0 {
			log.Info("代理数据已初始化", logger.F("count", len(srcProxies)))
		}
	}

	if a.browserMgr.ProxyVirtualGroupDAO != nil {
		groups, err := a.browserMgr.ProxyVirtualGroupDAO.List()
		if err != nil {
			return fmt.Errorf(`读取虚拟代理分组失败: %w`, err)
		} else {
			existingIDs := make(map[string]struct{}, len(groups))
			existingNames := make(map[string]struct{}, len(groups))
			for _, group := range groups {
				group = browser.NormalizeProxyVirtualGroup(group)
				if group.GroupId != "" {
					existingIDs[group.GroupId] = struct{}{}
				}
				if group.GroupName != "" {
					existingNames[strings.ToLower(group.GroupName)] = struct{}{}
				}
			}
			for _, configuredGroup := range a.config.Browser.ProxyVirtualGroups {
				group := browser.NormalizeProxyVirtualGroup(browser.ProxyVirtualGroup(configuredGroup))
				if _, exists := existingIDs[group.GroupId]; group.GroupId != "" && exists {
					continue
				}
				if _, exists := existingNames[strings.ToLower(group.GroupName)]; group.GroupName != "" && exists {
					continue
				}
				if err := a.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
					return fmt.Errorf(`迁移虚拟代理分组 %q 失败: %w`, group.GroupName, err)
				}
				if group.GroupId != "" {
					existingIDs[group.GroupId] = struct{}{}
				}
				if group.GroupName != "" {
					existingNames[strings.ToLower(group.GroupName)] = struct{}{}
				}
			}
		}
	}

	profiles, err := a.browserMgr.ProfileDAO.List()
	if err != nil {
		return fmt.Errorf(`读取实例数据失败: %w`, err)
	}
	if len(profiles) == 0 {
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
					ProxyMode:          pc.ProxyMode,
					ProxyId:            pc.ProxyId,
					ProxyConfig:        pc.ProxyConfig,
					ProxyGroupName:     pc.ProxyGroupName,
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
					return fmt.Errorf(`迁移实例 %q 失败: %w`, pc.ProfileId, err)
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
				return fmt.Errorf(`创建默认实例失败: %w`, err)
			}
		}
	}

	bookmarks, err := a.browserMgr.BookmarkDAO.List()
	if err != nil {
		return fmt.Errorf(`读取书签数据失败: %w`, err)
	}
	if len(bookmarks) == 0 {
		src := a.config.Browser.DefaultBookmarks
		if len(src) == 0 {
			src = append([]config.BrowserBookmark{}, defaultBookmarkList...)
		}
		src = normalizeBookmarkList(src)
		deletedURLs := mergeBookmarkDeletionURLs(a.config.Browser.DeletedBookmarkURLs)
		if deletionDAO, ok := a.browserMgr.BookmarkDAO.(browser.BookmarkDeletionDAO); ok {
			databaseDeletedURLs, err := deletionDAO.DeletedURLs()
			if err != nil {
				return fmt.Errorf(`读取书签删除标记失败: %w`, err)
			}
			deletedURLs = mergeBookmarkDeletionURLs(deletedURLs, databaseDeletedURLs)
		}
		src = filterBookmarksByDeletedURLs(src, deletedURLs)
		if len(deletedURLs) > 0 {
			deletionWriter, ok := a.browserMgr.BookmarkDAO.(browser.BookmarkDeletionWriter)
			if !ok {
				return fmt.Errorf(`书签数据库不支持写入删除标记`)
			}
			if err := deletionWriter.SyncDeletedURLs(deletedURLs); err != nil {
				return fmt.Errorf(`写入书签删除标记失败: %w`, err)
			}
		}
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(src); err != nil {
			return fmt.Errorf(`迁移书签失败: %w`, err)
		}
		log.Info("书签数据已迁移", logger.F("count", len(src)))
	}
	return nil
}
