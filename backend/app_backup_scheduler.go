package backend

import (
	"ant-chrome/backend/internal/backup/channels/openlist"
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
	app                *App
	mu                 sync.RWMutex
	settings           config.BackupConfig
	configurationError string
	state              backupScheduleState
	lastDate           string
	running            bool
	stopCh             chan struct{}
	done               chan struct{}
	stopOnce           sync.Once
	wg                 sync.WaitGroup
}

func newBackupScheduler(app *App) *backupScheduler {
	return &backupScheduler{
		app:      app,
		settings: defaultBackupConfig(),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
		state: backupScheduleState{
			Status: backupScheduleStatusNever,
		},
	}
}

func (a *App) startupInitBackupScheduler() {
	if a.backupScheduler != nil {
		return
	}
	scheduler := newBackupScheduler(a)
	if err := scheduler.loadLocalConfig(); err != nil {
		scheduler.configurationError = err.Error()
		scheduler.state.Status = backupScheduleStatusFailed
		scheduler.state.LastError = err.Error()
	}
	a.backupScheduler = scheduler
	a.backupScheduler.start()
}

func (s *backupScheduler) localConfigPath() string {
	if s == nil || s.app == nil {
		return ""
	}
	return s.app.resolveAppPath(backupLocalConfigFileName)
}

func (s *backupScheduler) loadLocalConfig() error {
	if s == nil || s.app == nil || s.app.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}

	if err := s.app.prepareBackupLocalConfig(); err != nil {
		return err
	}
	s.settings = normalizeBackupSettings(s.app.config.Backup)
	return nil
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
	if s.app == nil || s.app.config == nil || s.configurationError != "" {
		s.mu.RUnlock()
		return
	}
	schedule := s.settings.Schedule
	openList := s.settings.Channels.OpenList
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
		s.execute(openList)
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

func (s *backupScheduler) execute(openList config.OpenListChannelConfig) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	if strings.TrimSpace(openList.BaseURL) == "" {
		s.recordFailure("未配置 OpenList 地址")
		return
	}
	if strings.TrimSpace(openList.Token) == "" {
		s.recordFailure("未配置 OpenList Token")
		return
	}

	if runningNames := s.app.backupRunningProfileNames(); len(runningNames) > 0 {
		s.recordSkipped(fmt.Sprintf("实例正在运行：%s", strings.Join(runningNames, "、")))
		return
	}

	s.app.maintenanceMu.Lock()
	defer s.app.maintenanceMu.Unlock()
	result, err := s.app.backupOpenListUploadLocked(map[string]string{
		"baseURL":             openList.BaseURL,
		"remotePath":          openList.RemotePath,
		"token":               openList.Token,
		"uploadRateLimitMBps": strconv.Itoa(openList.UploadRateLimitMBps),
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
	return a.backupScheduler.saveSchedule(input)
}

func (s *backupScheduler) snapshot() map[string]interface{} {
	return backupScheduledSettingsSnapshot(s.app, s)
}

func backupScheduledSettingsResult(settings config.BackupConfig, state backupScheduleState, tokenConfigured bool) map[string]interface{} {
	return map[string]interface{}{
		"enabled":         settings.Schedule.Enabled,
		"dailyTime":       settings.Schedule.DailyTime,
		"tokenConfigured": tokenConfigured,
		"status":          state.Status,
		"lastRunAt":       state.LastRunAt,
		"lastSuccessAt":   state.LastSuccessAt,
		"lastError":       state.LastError,
		"lastRemoteName":  state.LastRemoteName,
	}
}

func backupScheduledSettingsSnapshot(app *App, scheduler *backupScheduler) map[string]interface{} {
	if scheduler == nil {
		settings := defaultBackupConfig()
		if app != nil {
			base := settings
			if app.config != nil {
				base = app.config.Backup
			}
			if stored, _, err := loadBackupLocalConfig(app.resolveAppPath(backupLocalConfigFileName), base); err == nil {
				settings = stored
			}
		}
		tokenConfigured := strings.TrimSpace(settings.Channels.OpenList.Token) != ""
		return backupScheduledSettingsResult(settings, backupScheduleState{Status: backupScheduleStatusNever}, tokenConfigured)
	}
	scheduler.mu.RLock()
	defer scheduler.mu.RUnlock()
	tokenConfigured := strings.TrimSpace(scheduler.settings.Channels.OpenList.Token) != ""
	return backupScheduledSettingsResult(scheduler.settings, scheduler.state, tokenConfigured)
}

func (s *backupScheduler) saveSchedule(input map[string]string) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.app == nil || s.app.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}

	next := normalizeBackupSettings(s.settings)
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
	if next.Schedule.Enabled {
		if next.Channels.OpenList.BaseURL == "" {
			return nil, fmt.Errorf("启用定时备份前请配置 OpenList 地址")
		}
		if next.Channels.OpenList.Token == "" {
			return nil, fmt.Errorf("启用定时备份前请配置 OpenList Token")
		}
	}
	if next.Schedule.Enabled {
		if _, err := openlist.NewClient(openlist.Config{
			BaseURL:             next.Channels.OpenList.BaseURL,
			RemotePath:          next.Channels.OpenList.RemotePath,
			Token:               next.Channels.OpenList.Token,
			UploadRateLimitMBps: next.Channels.OpenList.UploadRateLimitMBps,
		}); err != nil {
			return nil, fmt.Errorf("OpenList 配置无效：%w", err)
		}
	}

	return s.commitSettingsLocked(next)
}

func (s *backupScheduler) saveOpenList(input map[string]string) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.app == nil || s.app.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}

	next := normalizeBackupSettings(s.settings)
	if err := applyOpenListInput(&next, input); err != nil {
		return nil, err
	}
	if next.Channels.OpenList.BaseURL == "" {
		return nil, fmt.Errorf("OpenList 地址不能为空")
	}
	if next.Channels.OpenList.Token == "" {
		return nil, fmt.Errorf("OpenList Token 不能为空")
	}
	if _, err := openlist.NewClient(openlist.Config{
		BaseURL:             next.Channels.OpenList.BaseURL,
		RemotePath:          next.Channels.OpenList.RemotePath,
		Token:               next.Channels.OpenList.Token,
		UploadRateLimitMBps: next.Channels.OpenList.UploadRateLimitMBps,
	}); err != nil {
		return nil, fmt.Errorf("OpenList 配置无效：%w", err)
	}

	if _, err := s.commitSettingsLocked(next); err != nil {
		return nil, err
	}
	return backupOpenListSettingsResult(s.settings.Channels.OpenList), nil
}

func applyOpenListInput(next *config.BackupConfig, input map[string]string) error {
	if next == nil {
		return nil
	}
	if value := backupOpenListInputValue(input, "baseURL", "baseUrl"); value != "" {
		next.Channels.OpenList.BaseURL = value
	}
	if value, ok := backupOpenListInputValueWithPresence(input, "remotePath", "path"); ok {
		next.Channels.OpenList.RemotePath = value
	}
	if token := strings.TrimSpace(input["token"]); token != "" {
		next.Channels.OpenList.Token = token
	}
	if value := backupOpenListInputValue(input, "uploadRateLimitMBps", "uploadRateLimitMbps", "upload_rate_limit_mbps"); value != "" {
		rateLimit, err := strconv.Atoi(value)
		if err != nil || rateLimit < 0 {
			return fmt.Errorf("OpenList 上传限速必须是非负整数 MB/s")
		}
		next.Channels.OpenList.UploadRateLimitMBps = rateLimit
	}
	return nil
}

func (s *backupScheduler) commitSettingsLocked(next config.BackupConfig) (map[string]interface{}, error) {
	next = normalizeBackupSettings(next)
	if err := saveBackupLocalConfig(s.localConfigPath(), next); err != nil {
		return nil, err
	}
	previous := s.app.config.Backup
	s.app.config.Backup = next
	if err := s.app.config.Save(s.app.resolveAppPath("config.yaml")); err != nil {
		s.app.config.Backup = previous
		return nil, err
	}
	s.settings = next
	s.configurationError = ""
	return s.snapshotLocked(), nil
}

func (s *backupScheduler) snapshotLocked() map[string]interface{} {
	tokenConfigured := strings.TrimSpace(s.settings.Channels.OpenList.Token) != ""
	return backupScheduledSettingsResult(s.settings, s.state, tokenConfigured)
}
