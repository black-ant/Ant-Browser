package backend

import (
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/cdp"
	"ant-chrome/backend/internal/logger"

	"github.com/gorilla/websocket"
)

const timezoneArgPrefix = "--timezone="

type profileTimezoneOverride struct {
	TimezoneID string
	Source     string
}

func (t profileTimezoneOverride) shouldApply() bool {
	return strings.TrimSpace(t.TimezoneID) != ""
}

func (t profileTimezoneOverride) cdpParams() map[string]any {
	return map[string]any{"timezoneId": strings.TrimSpace(t.TimezoneID)}
}

func timezoneOverrideFromLaunchArgs(args []string) profileTimezoneOverride {
	for i := len(args) - 1; i >= 0; i-- {
		arg := strings.TrimSpace(args[i])
		if !strings.HasPrefix(arg, timezoneArgPrefix) {
			continue
		}
		timezoneID := strings.TrimSpace(strings.TrimPrefix(arg, timezoneArgPrefix))
		if timezoneID == "" {
			continue
		}
		source := "profile"
		if i == len(args)-1 {
			source = "proxy"
		}
		return profileTimezoneOverride{TimezoneID: timezoneID, Source: source}
	}
	return profileTimezoneOverride{}
}

func (a *App) applyProfileTimezone(profileId string, timezone profileTimezoneOverride) error {
	if !timezone.shouldApply() {
		return nil
	}

	pc, err := a.profileCDP(profileId)
	if err != nil {
		return err
	}

	return applyTimezoneToPageTargets(pc, timezone.cdpParams())
}

func (a *App) applyAndWatchProfileTimezone(profileId string, debugPort int, timezone profileTimezoneOverride) error {
	err := a.applyProfileTimezone(profileId, timezone)
	a.startProfileTimezoneWatcher(profileId, debugPort, timezone)
	return err
}

func applyTimezoneToPageTargets(pc *profileCDP, params map[string]any) error {
	if pc.pipe != nil {
		return applyTimezoneToPipePageTargets(pc.pipe, params)
	}
	return applyTimezoneToPortPageTargets(pc.debugPort, params)
}

func applyTimezoneToPipePageTargets(conn *cdp.PipeConn, params map[string]any) error {
	res, err := conn.SendCommand("", "Target.getTargets", nil)
	if err != nil {
		return err
	}
	infos, ok := res["targetInfos"].([]interface{})
	if !ok {
		return fmt.Errorf("CDP target list has unexpected format")
	}

	applied := 0
	var firstErr error
	for _, item := range infos {
		info, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if targetType, _ := info["type"].(string); targetType != "page" {
			continue
		}
		targetID, _ := info["targetId"].(string)
		if strings.TrimSpace(targetID) == "" {
			continue
		}
		attachResult, err := conn.SendCommand("", "Target.attachToTarget", map[string]interface{}{
			"targetId": targetID,
			"flatten":  true,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sessionID, _ := attachResult["sessionId"].(string)
		if strings.TrimSpace(sessionID) == "" {
			if firstErr == nil {
				firstErr = fmt.Errorf("CDP attach did not return sessionId")
			}
			continue
		}
		if _, err := conn.SendCommand(sessionID, "Emulation.setTimezoneOverride", params); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			_, _ = conn.SendCommand("", "Target.detachFromTarget", map[string]interface{}{"sessionId": sessionID})
			continue
		}
		applied++
		_, _ = conn.SendCommand("", "Target.detachFromTarget", map[string]interface{}{"sessionId": sessionID})
	}

	if applied == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("no page target available for timezone override")
	}
	return nil
}

func applyTimezoneToPortPageTargets(debugPort int, params map[string]any) error {
	targets, err := cdpPortPageTargets(debugPort)
	if err != nil {
		return err
	}

	applied := 0
	var firstErr error
	for _, target := range targets {
		if target.Type != "page" || strings.TrimSpace(target.WebSocketDebuggerUrl) == "" {
			continue
		}
		if err := cdpWebSocketCall(target.WebSocketDebuggerUrl, "Emulation.setTimezoneOverride", params, 5*time.Second); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		applied++
	}

	if applied == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("no page target available for timezone override")
	}
	return nil
}

func (a *App) startProfileTimezoneWatcher(profileId string, debugPort int, timezone profileTimezoneOverride) {
	if !timezone.shouldApply() || debugPort <= 0 {
		return
	}
	go a.watchPortTimezoneTargets(profileId, debugPort, timezone)
}

func (a *App) watchPortTimezoneTargets(profileId string, debugPort int, timezone profileTimezoneOverride) {
	log := logger.New("Browser")
	wsURL, err := cdpBrowserWebSocketURL(debugPort)
	if err != nil {
		log.Warn("时区新标签监听启动失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("timezone", timezone.TimezoneID),
			logger.F("error", err.Error()))
		return
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Warn("时区新标签监听连接失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("timezone", timezone.TimezoneID),
			logger.F("error", err.Error()))
		return
	}
	defer conn.Close()

	stop := make(chan struct{})
	go a.closeGeolocationWatcherWhenInactive(profileId, debugPort, conn, stop)
	defer close(stop)

	nextID := 1
	pending := make(map[int]string)
	if err := writeTargetAttachCDPCommand(conn, nextID, "", "Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": false,
		"flatten":                true,
	}); err != nil {
		log.Warn("时区新标签监听启用失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("timezone", timezone.TimezoneID),
			logger.F("error", err.Error()))
		return
	}
	pending[nextID] = "Target.setAutoAttach"
	nextID++

	for {
		var msg targetAttachCDPMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if a.profileDebugPortActive(profileId, debugPort) {
				log.Warn("时区新标签监听中断",
					logger.F("profile_id", profileId),
					logger.F("debug_port", debugPort),
					logger.F("timezone", timezone.TimezoneID),
					logger.F("error", err.Error()))
			}
			return
		}

		if msg.ID > 0 {
			method := pending[msg.ID]
			delete(pending, msg.ID)
			if msg.Error != nil {
				log.Warn("时区新标签 CDP 命令失败",
					logger.F("profile_id", profileId),
					logger.F("debug_port", debugPort),
					logger.F("timezone", timezone.TimezoneID),
					logger.F("method", method),
					logger.F("error", msg.Error.Message))
				if method == "Target.setAutoAttach" {
					return
				}
			}
			continue
		}

		sessionID, ok := attachedPageSessionID(msg)
		if !ok {
			continue
		}
		method := "Emulation.setTimezoneOverride"
		if err := writeTargetAttachCDPCommand(conn, nextID, sessionID, method, timezone.cdpParams()); err != nil {
			log.Warn("新标签时区覆盖失败",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort),
				logger.F("timezone", timezone.TimezoneID),
				logger.F("session_id", sessionID),
				logger.F("error", err.Error()))
			return
		}
		pending[nextID] = method
		nextID++
	}
}
