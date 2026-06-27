package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
)

// ============================================================================
// 模板类型别名 (保持 Wails 绑定兼容)
// ============================================================================

type BrowserTemplate = browser.Template
type BrowserTemplateInput = browser.TemplateInput

// ============================================================================
// 窗口创建模板管理 API
// ============================================================================

// ListTemplates 获取所有创建模板
func (a *App) ListTemplates() []BrowserTemplate {
	log := logger.New("Template")
	if a.browserMgr.TemplateDAO == nil {
		log.Error("TemplateDAO 未初始化")
		return []BrowserTemplate{}
	}

	templates, err := a.browserMgr.TemplateDAO.List()
	if err != nil {
		log.Error("获取模板列表失败", logger.F("error", err))
		return []BrowserTemplate{}
	}

	result := make([]BrowserTemplate, 0, len(templates))
	for _, t := range templates {
		result = append(result, *t)
	}
	return result
}

// CreateTemplate 保存为新模板
func (a *App) CreateTemplate(input BrowserTemplateInput) (*BrowserTemplate, error) {
	log := logger.New("Template")
	if a.browserMgr.TemplateDAO == nil {
		return nil, fmt.Errorf("TemplateDAO 未初始化")
	}

	template, err := a.browserMgr.TemplateDAO.Create(input)
	if err != nil {
		log.Error("创建模板失败", logger.F("error", err))
		return nil, err
	}
	log.Info("模板已创建", logger.F("template_id", template.TemplateId), logger.F("template_name", template.TemplateName))
	return template, nil
}

// UpdateTemplate 更新模板
func (a *App) UpdateTemplate(templateId string, input BrowserTemplateInput) (*BrowserTemplate, error) {
	log := logger.New("Template")
	if a.browserMgr.TemplateDAO == nil {
		return nil, fmt.Errorf("TemplateDAO 未初始化")
	}

	template, err := a.browserMgr.TemplateDAO.Update(templateId, input)
	if err != nil {
		log.Error("更新模板失败", logger.F("template_id", templateId), logger.F("error", err))
		return nil, err
	}
	log.Info("模板已更新", logger.F("template_id", templateId))
	return template, nil
}

// DeleteTemplate 删除模板
func (a *App) DeleteTemplate(templateId string) error {
	log := logger.New("Template")
	if a.browserMgr.TemplateDAO == nil {
		return fmt.Errorf("TemplateDAO 未初始化")
	}

	if err := a.browserMgr.TemplateDAO.Delete(templateId); err != nil {
		log.Error("删除模板失败", logger.F("template_id", templateId), logger.F("error", err))
		return err
	}
	log.Info("模板已删除", logger.F("template_id", templateId))
	return nil
}
