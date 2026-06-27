package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
)

// ============================================================================
// 分组类型别名 (保持 Wails 绑定兼容)
// ============================================================================

type BrowserGroup = browser.Group
type BrowserGroupInput = browser.GroupInput
type BrowserGroupWithCount = browser.GroupWithCount

// ============================================================================
// 分组管理 API
// ============================================================================

// ListGroups 获取所有分组（带窗口计数）
func (a *App) ListGroups() []BrowserGroupWithCount {
	log := logger.New("Group")
	if a.browserMgr.GroupDAO == nil {
		log.Error("GroupDAO 未初始化")
		return []BrowserGroupWithCount{}
	}

	groups, err := a.browserMgr.GroupDAO.List()
	if err != nil {
		log.Error("获取分组列表失败", logger.F("error", err))
		return []BrowserGroupWithCount{}
	}

	// 统计每个分组的窗口数量
	profiles, _ := a.browserMgr.ProfileDAO.List()
	countMap := make(map[string]int)
	for _, p := range profiles {
		if p.GroupId != "" {
			countMap[p.GroupId]++
		}
	}

	result := make([]BrowserGroupWithCount, 0, len(groups))
	for _, g := range groups {
		result = append(result, BrowserGroupWithCount{
			Group:         *g,
			InstanceCount: countMap[g.GroupId],
		})
	}
	return result
}

// CreateGroup 创建分组
func (a *App) CreateGroup(input BrowserGroupInput) (*BrowserGroup, error) {
	log := logger.New("Group")
	if a.browserMgr.GroupDAO == nil {
		return nil, fmt.Errorf("GroupDAO 未初始化")
	}

	group, err := a.browserMgr.GroupDAO.Create(input)
	if err != nil {
		log.Error("创建分组失败", logger.F("error", err))
		return nil, err
	}
	log.Info("分组已创建", logger.F("group_id", group.GroupId), logger.F("group_name", group.GroupName))
	return group, nil
}

// UpdateGroup 更新分组
func (a *App) UpdateGroup(groupId string, input BrowserGroupInput) (*BrowserGroup, error) {
	log := logger.New("Group")
	if a.browserMgr.GroupDAO == nil {
		return nil, fmt.Errorf("GroupDAO 未初始化")
	}

	group, err := a.browserMgr.GroupDAO.Update(groupId, input)
	if err != nil {
		log.Error("更新分组失败", logger.F("group_id", groupId), logger.F("error", err))
		return nil, err
	}
	log.Info("分组已更新", logger.F("group_id", groupId))
	return group, nil
}

// DeleteGroup 删除分组
func (a *App) DeleteGroup(groupId string) error {
	log := logger.New("Group")
	if a.browserMgr.GroupDAO == nil {
		return fmt.Errorf("GroupDAO 未初始化")
	}

	if err := a.browserMgr.GroupDAO.Delete(groupId); err != nil {
		log.Error("删除分组失败", logger.F("group_id", groupId), logger.F("error", err))
		return err
	}
	log.Info("分组已删除", logger.F("group_id", groupId))
	return nil
}

// MoveInstancesToGroup 批量移动窗口到分组
func (a *App) MoveInstancesToGroup(profileIds []string, groupId string) error {
	log := logger.New("Group")
	dao, ok := a.browserMgr.ProfileDAO.(*browser.SQLiteProfileDAO)
	if !ok {
		return fmt.Errorf("ProfileDAO 不支持批量移动")
	}

	if err := dao.MoveToGroup(profileIds, groupId); err != nil {
		log.Error("批量移动窗口失败", logger.F("count", len(profileIds)), logger.F("error", err))
		return err
	}
	log.Info("窗口已移动到分组", logger.F("count", len(profileIds)), logger.F("group_id", groupId))
	return nil
}
