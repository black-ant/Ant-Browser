package backend

import (
	"ant-chrome/backend/internal/backupremote"
	"ant-chrome/backend/internal/config"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	backupScheduleStatusNever   = "never"
	backupScheduleStatusRunning = "running"
	backupScheduleStatusSuccess = "success"
	backupScheduleStatusSkipped = "skipped"
	backupScheduleStatusFailed  = "failed"
)

type backupScheduleState struct {
	Status         string
	LastRunAt      string
	LastSuccessAt  string
	LastError      string
	LastRemoteName string
}

type backupScheduler struct {
	app              *App
	mu               sync.RWMutex
	password         string
	passwordUsername string
	state            backupScheduleState
	lastDate         string
	running          bool
	stopCh           chan struct{}
	done             chan struct{}
	stopOnce         sync.Once
	wg               sync.WaitGroup
}

func newBackupScheduler(app *App) *backupScheduler {
	return &backupScheduler{
		app:    app,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
		state: backupScheduleState{
			Status: backupScheduleStatusNever,
		},
	}
}

func (a *App) startupInitBackupScheduler() {
	if a.backupScheduler != nil {
		return
	}
	a.backupScheduler = newBackupScheduler(a)
	a.backupScheduler.start()
}

func (a *App) stopBackupScheduler() {
	if a == nil || a.backupScheduler == nil {
		return
	}
	a.backupScheduler.stop()
}

func (s *backupScheduler) start() {
	go s.loop()
}

func (s *backupScheduler) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		<-s.done
		s.wg.Wait()
	})
}

func (s *backupScheduler) loop() {
	defer close(s.done)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	s.check(time.Now())
	for {
		select {
		case <-ticker.C:
			s.check(time.Now())
		case <-s.stopCh:
			return
		}
	}
}

func (s *backupScheduler) check(now time.Time) {
	s.mu.RLock()
	if s.app == nil || s.app.config == nil {
		s.mu.RUnlock()
		return
	}
	schedule := s.app.config.Backup.Schedule
	openList := s.app.config.Backup.OpenList
	password := s.password
	lastDate := s.lastDate
	running := s.running
	s.mu.RUnlock()

	if !schedule.Enabled || running {
		return
	}
	if !backupScheduleMatchesTime(schedule.DailyTime, now) {
		return
	}
	today := now.Format("2006-01-02")
	if lastDate == today {
		return
	}

	s.mu.Lock()
	if s.running || s.lastDate == today {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.lastDate = today
	s.state.Status = backupScheduleStatusRunning
	s.state.LastRunAt = now.UTC().Format(time.RFC3339)
	s.state.LastError = ""
	s.state.LastRemoteName = ""
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.execute(schedule, openList, password)
	}()
}

func backupScheduleMatchesTime(value string, now time.Time) bool {
	parsed, err := backupScheduleParseTime(value, now.Location())
	if err != nil {
		return false
	}
	return parsed.Hour() == now.Hour() && parsed.Minute() == now.Minute()
}

func backupScheduleParseTime(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if len(value) != 5 || value[2] != ':' {
		return time.Time{}, fmt.Errorf("time must use HH:MM format")
	}
	return time.ParseInLocation("15:04", value, location)
}

func (s *backupScheduler) execute(schedule config.BackupScheduleConfig, openList config.OpenListBackupConfig, password string) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	if strings.TrimSpace(openList.BaseURL) == "" {
		s.recordFailure("未配置 OpenList 地址")
		return
	}
	if strings.TrimSpace(openList.Username) != "" && password == "" {
		s.recordFailure("未配置 OpenList 密码；密码仅保存在当前运行期间")
		return
	}
	if strings.TrimSpace(openList.Username) == "" && password != "" {
		s.recordFailure("OpenList 用户名为空，但仍配置了密码")
		return
	}

	if runningNames := s.app.backupRunningProfileNames(); len(runningNames) > 0 {
		s.recordSkipped(fmt.Sprintf("实例正在运行：%s", strings.Join(runningNames, "、")))
		return
	}

	result, err := s.app.backupOpenListUploadLocked(map[string]string{
		"baseURL":    openList.BaseURL,
		"remotePath": openList.RemotePath,
		"username":   openList.Username,
		"password":   password,
	})
	if err != nil {
		s.recordFailure(err.Error())
		return
	}

	remoteName, _ := result["remoteName"].(string)
	s.mu.Lock()
	s.state.Status = backupScheduleStatusSuccess
	s.state.LastSuccessAt = time.Now().UTC().Format(time.RFC3339)
	s.state.LastError = ""
	s.state.LastRemoteName = remoteName
	s.mu.Unlock()
}

func (s *backupScheduler) recordSkipped(message string) {
	s.mu.Lock()
	s.state.Status = backupScheduleStatusSkipped
	s.state.LastError = message
	s.mu.Unlock()
}

func (s *backupScheduler) recordFailure(message string) {
	s.mu.Lock()
	s.state.Status = backupScheduleStatusFailed
	s.state.LastError = strings.TrimSpace(message)
	s.mu.Unlock()
}

func (a *App) BackupScheduledGetSettings() map[string]interface{} {
	if a == nil || a.backupScheduler == nil {
		return backupScheduledSettingsSnapshot(a, nil)
	}
	return a.backupScheduler.snapshot()
}

func (a *App) BackupScheduledSaveSettings(input map[string]string) (map[string]interface{}, error) {
	if a == nil {
		return nil, fmt.Errorf("应用未初始化")
	}
	if a.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}
	if a.backupScheduler == nil {
		a.startupInitBackupScheduler()
	}
	return a.backupScheduler.save(input)
}

func (s *backupScheduler) snapshot() map[string]interface{} {
	return backupScheduledSettingsSnapshot(s.app, s)
}

func backupScheduledSettingsSnapshot(app *App, scheduler *backupScheduler) map[string]interface{} {
	result := map[string]interface{}{
		"enabled":            false,
		"dailyTime":          "02:00",
		"baseURL":            "",
		"remotePath":         "ant-chrome/backups",
		"username":           "",
		"passwordConfigured": false,
		"status":             backupScheduleStatusNever,
		"lastRunAt":          "",
		"lastSuccessAt":      "",
		"lastError":          "",
		"lastRemoteName":     "",
	}
	if app != nil && app.config != nil {
		result["enabled"] = app.config.Backup.Schedule.Enabled
		result["dailyTime"] = app.config.Backup.Schedule.DailyTime
		result["baseURL"] = app.config.Backup.OpenList.BaseURL
		result["remotePath"] = app.config.Backup.OpenList.RemotePath
		result["username"] = app.config.Backup.OpenList.Username
	}
	if scheduler == nil {
		return result
	}
	scheduler.mu.RLock()
	defer scheduler.mu.RUnlock()
	username := strings.TrimSpace(result["username"].(string))
	result["passwordConfigured"] = username == "" || (scheduler.password != "" && scheduler.passwordUsername == username)
	result["status"] = scheduler.state.Status
	result["lastRunAt"] = scheduler.state.LastRunAt
	result["lastSuccessAt"] = scheduler.state.LastSuccessAt
	result["lastError"] = scheduler.state.LastError
	result["lastRemoteName"] = scheduler.state.LastRemoteName
	return result
}

func (s *backupScheduler) save(input map[string]string) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.app == nil || s.app.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}

	next := s.app.config.Backup
	if value := strings.TrimSpace(input["enabled"]); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("定时备份启用状态无效")
		}
		next.Schedule.Enabled = enabled
	}
	if value := strings.TrimSpace(input["dailyTime"]); value != "" {
		next.Schedule.DailyTime = value
	}
	if _, err := backupScheduleParseTime(next.Schedule.DailyTime, time.Local); err != nil {
		return nil, fmt.Errorf("定时备份时间必须是 HH:MM")
	}
	next.OpenList.BaseURL = strings.TrimSpace(input["baseURL"])
	next.OpenList.RemotePath = strings.TrimSpace(input["remotePath"])
	if next.OpenList.RemotePath == "" {
		next.OpenList.RemotePath = "ant-chrome/backups"
	}
	next.OpenList.Username = strings.TrimSpace(input["username"])
	password := input["password"]
	passwordAvailable := s.password != "" && s.passwordUsername == next.OpenList.Username
	if next.OpenList.Username == "" {
		password = ""
	}
	if next.Schedule.Enabled {
		if next.OpenList.BaseURL == "" {
			return nil, fmt.Errorf("启用定时备份前请配置 OpenList 地址")
		}
		if next.OpenList.Username != "" && password == "" && !passwordAvailable {
			return nil, fmt.Errorf("启用账号认证时必须输入 OpenList 密码")
		}
		if next.OpenList.Username == "" && password != "" {
			return nil, fmt.Errorf("OpenList 用户名为空时不能填写密码")
		}
	}
	if _, err := backupremote.NewClient(backupremote.Config{
		BaseURL:    next.OpenList.BaseURL,
		RemotePath: next.OpenList.RemotePath,
		Username:   next.OpenList.Username,
		Password:   password,
	}); err != nil && next.Schedule.Enabled {
		return nil, fmt.Errorf("OpenList 配置无效：%w", err)
	}

	previous := s.app.config.Backup
	s.app.config.Backup = next
	if err := s.app.config.Save(s.app.resolveAppPath("config.yaml")); err != nil {
		s.app.config.Backup = previous
		return nil, err
	}
	if !next.Schedule.Enabled || next.OpenList.Username == "" {
		s.password = ""
		s.passwordUsername = ""
	} else if password != "" {
		s.password = password
		s.passwordUsername = next.OpenList.Username
	} else if !passwordAvailable {
		s.password = ""
		s.passwordUsername = ""
	}
	return s.snapshotLocked(), nil
}

func (s *backupScheduler) snapshotLocked() map[string]interface{} {
	result := backupScheduledSettingsSnapshot(s.app, nil)
	username := strings.TrimSpace(result["username"].(string))
	result["passwordConfigured"] = username == "" || (s.password != "" && s.passwordUsername == username)
	result["status"] = s.state.Status
	result["lastRunAt"] = s.state.LastRunAt
	result["lastSuccessAt"] = s.state.LastSuccessAt
	result["lastError"] = s.state.LastError
	result["lastRemoteName"] = s.state.LastRemoteName
	return result
}
