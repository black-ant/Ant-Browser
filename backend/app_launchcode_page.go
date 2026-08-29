package backend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/launchcode"
)

// 页面操作（Playwright / CDP）能力向 LaunchServer 的适配。
//
// 和 appProxyProvider 同样的理由：用独立小类型而不是把方法挂到 *App，
// 因为 Wails 会把 *App 上所有导出方法绑定给前端。适配器只做目标解析、
// 产物搬运和错误分类，真正的会话管理在 automation.Manager 里。

const (
	// pageStepDefaultTimeout 是单条页面指令的默认等待上限。
	pageStepDefaultTimeout = 30 * time.Second
	// pageStepMaxTimeout 兜住调用方传入的离谱值。
	pageStepMaxTimeout = 5 * time.Minute
	// pageSessionOpenTimeout 是首次建立会话时等待实例 CDP 就绪的上限。
	pageSessionOpenTimeout = 60 * time.Second
)

type appPageDriver struct {
	app *App
}

func newAppPageDriver(app *App) launchcode.PageDriver {
	return appPageDriver{app: app}
}

// RunPageSteps 解析目标实例，然后把指令交给常驻会话执行。
func (d appPageDriver) RunPageSteps(req launchcode.PageRequest) (*launchcode.PageResult, error) {
	if d.app == nil {
		return nil, launchcode.NewServiceError(http.StatusServiceUnavailable, "page automation api is unavailable")
	}

	ctx := context.Background()
	if _, err := d.app.ensureAutomationReady(ctx); err != nil {
		return nil, launchcode.NewServiceError(http.StatusServiceUnavailable, err.Error())
	}

	profileID, launchCode, err := d.app.resolvePageTarget(req.Selector)
	if err != nil {
		return nil, err
	}

	baseURL, authHeader, authValue, err := d.app.automationDemoEndpoint()
	if err != nil {
		return nil, launchcode.NewServiceError(http.StatusServiceUnavailable, err.Error())
	}

	artifactDir := filepath.Join(d.app.automationArtifactsRootDir(), "mcp-page", profileID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, launchcode.NewServiceError(http.StatusInternalServerError, "创建页面产物目录失败: "+err.Error())
	}

	commands := make([]automation.PageCommand, 0, len(req.Steps))
	for _, step := range req.Steps {
		commands = append(commands, automation.PageCommand{Action: step.Action, Args: step.Args})
	}

	outcome, err := d.app.automationMgr.RunPageCommands(ctx, automation.PageCommandRequest{
		ProfileID: profileID,
		// 用解析后的 profileId 而不是原始 selector：目标已经确定，
		// 让 Node 侧再解析一次只会引入歧义风险。
		Selector:         map[string]any{"profileId": profileID},
		Commands:         commands,
		LaunchBaseURL:    baseURL,
		LaunchAuthHeader: authHeader,
		LaunchAuthValue:  authValue,
		ArtifactDir:      artifactDir,
		Timeout:          pageStepTimeout(req.TimeoutMs),
		IdleTimeout:      d.app.pageSessionIdleTimeout(),
	})
	if err != nil {
		return nil, launchcode.NewServiceError(http.StatusInternalServerError, err.Error())
	}

	result := &launchcode.PageResult{
		ProfileID:  profileID,
		LaunchCode: launchCode,
		OK:         outcome.OK,
		Reused:     outcome.Reused,
		Error:      outcome.Error,
		Steps:      make([]launchcode.PageStepOutcome, 0, len(outcome.Results)),
	}
	for _, step := range outcome.Results {
		result.Steps = append(result.Steps, launchcode.PageStepOutcome{
			Action: step.Action,
			OK:     step.OK,
			Result: step.Result,
			Error:  step.Error,
		})
	}

	collectPageScreenshots(result)
	return result, nil
}

// ClosePageSession 释放常驻会话。与 RunPageSteps 不同，这里不启动实例——
// 要关掉的东西不存在时，静默成功比报错更合理。
func (d appPageDriver) ClosePageSession(selector launchcode.LaunchSelector) (string, error) {
	if d.app == nil || d.app.automationMgr == nil {
		return "", launchcode.NewServiceError(http.StatusServiceUnavailable, "page automation api is unavailable")
	}

	profileID, err := d.app.resolvePageTargetWithoutStart(selector)
	if err != nil {
		return "", err
	}

	d.app.automationMgr.ClosePageSession(profileID)
	return profileID, nil
}

// collectPageScreenshots 把落盘的截图读进内存并删除源文件。
//
// 截图对 MCP 客户端是一次性内容，留在磁盘上只会无限增长；同时把结果里的
// 绝对路径换成文件名，避免把宿主的目录结构塞进模型上下文。
func collectPageScreenshots(result *launchcode.PageResult) {
	for index := range result.Steps {
		step := &result.Steps[index]
		if step.Result == nil {
			continue
		}
		rawPath, _ := step.Result["path"].(string)
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" {
			continue
		}

		mimeType, _ := step.Result["mimeType"].(string)
		if strings.TrimSpace(mimeType) == "" {
			mimeType = "image/jpeg"
		}

		name := filepath.Base(rawPath)
		data, err := os.ReadFile(rawPath)
		if err != nil {
			step.Result["screenshotError"] = "读取截图失败: " + err.Error()
			delete(step.Result, "path")
			continue
		}
		_ = os.Remove(rawPath)

		if result.Screenshots == nil {
			result.Screenshots = make(map[string]launchcode.PageScreenshot)
		}
		result.Screenshots[name] = launchcode.PageScreenshot{Data: data, MIMEType: mimeType}

		delete(step.Result, "path")
		step.Result["screenshot"] = name
		step.Result["bytes"] = len(data)
	}
}

func pageStepTimeout(timeoutMs int) time.Duration {
	if timeoutMs <= 0 {
		return pageStepDefaultTimeout
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout > pageStepMaxTimeout {
		return pageStepMaxTimeout
	}
	return timeout
}

func (a *App) pageSessionIdleTimeout() time.Duration {
	if a == nil || a.config == nil || a.config.Automation.PageSessionIdleMs <= 0 {
		return 0
	}
	return time.Duration(a.config.Automation.PageSessionIdleMs) * time.Millisecond
}

// ensureAutomationReady 是自动化执行前的统一就绪检查。
// 脚本执行和页面会话共用，避免两处检查逐渐漂移。
func (a *App) ensureAutomationReady(ctx context.Context) (automation.RuntimeState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.automationMgr == nil {
		return automation.RuntimeState{}, fmt.Errorf("automation runtime manager is not initialized")
	}
	if a.config == nil || !a.config.Automation.Enabled {
		return automation.RuntimeState{}, fmt.Errorf("自动化支持尚未启用")
	}
	if err := ctx.Err(); err != nil {
		return automation.RuntimeState{}, fmt.Errorf("%s", automationRunContextErrorMessage(err))
	}
	if err := a.automationMgr.EnsureInstalled(ctx); err != nil {
		return automation.RuntimeState{}, err
	}
	if err := ctx.Err(); err != nil {
		return automation.RuntimeState{}, fmt.Errorf("%s", automationRunContextErrorMessage(err))
	}

	state := a.automationMgr.CurrentState()
	if !state.Ready {
		return automation.RuntimeState{}, fmt.Errorf("自动化运行时尚未就绪")
	}
	return state, nil
}

// resolvePageTarget 解析目标实例并确保它已启动、CDP 可接管。
// selector 为空时沿用当前挂在统一 CDP 入口上的实例。
func (a *App) resolvePageTarget(selector launchcode.LaunchSelector) (string, string, error) {
	if a.launchServer == nil {
		return "", "", launchcode.NewServiceError(http.StatusServiceUnavailable, "launch server is not initialized")
	}

	if selector.IsEmpty() {
		session, err := a.launchServer.ActiveRuntimeSession()
		if err != nil {
			return "", "", err
		}
		if session == nil || session.Profile == nil {
			return "", "", launchcode.NewServiceError(
				http.StatusBadRequest,
				"当前没有活动实例，请用 selector 指定目标实例",
			)
		}
		return session.Profile.ProfileId, session.LaunchCode, nil
	}

	session, err := a.launchServer.OpenRuntimeSession(
		selector,
		// 页面会话由调用方显式导航，不需要默认起始页干扰。
		launchcode.LaunchRequestParams{SkipDefaultStartURLs: true},
		pageSessionOpenTimeout,
	)
	if err != nil {
		return "", "", err
	}
	if session == nil || session.Profile == nil {
		return "", "", launchcode.NewServiceError(http.StatusServiceUnavailable, "runtime session is not available")
	}
	if !session.Ready {
		return "", "", launchcode.NewServiceError(
			http.StatusServiceUnavailable,
			"实例调试端口尚未就绪，请稍后重试",
		)
	}
	return session.Profile.ProfileId, session.LaunchCode, nil
}

// resolvePageTargetWithoutStart 只做定位，不触发启动，用于释放会话。
func (a *App) resolvePageTargetWithoutStart(selector launchcode.LaunchSelector) (string, error) {
	if a.launchServer == nil {
		return "", launchcode.NewServiceError(http.StatusServiceUnavailable, "launch server is not initialized")
	}

	if selector.IsEmpty() {
		session, err := a.launchServer.ActiveRuntimeSession()
		if err != nil {
			return "", err
		}
		if session == nil || session.Profile == nil {
			return "", launchcode.NewServiceError(
				http.StatusBadRequest,
				"当前没有活动实例，请用 selector 指定目标实例",
			)
		}
		return session.Profile.ProfileId, nil
	}

	profile, err := a.launchServer.FindProfile(selector)
	if err != nil {
		return "", err
	}
	if profile == nil {
		return "", launchcode.NewServiceError(http.StatusNotFound, "profile not found")
	}
	return profile.ProfileId, nil
}
