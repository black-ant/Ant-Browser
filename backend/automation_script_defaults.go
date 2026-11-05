package backend

import (
	"ant-chrome/backend/internal/automation"
)

func (a *App) ensureAutomationScriptDefaults(store *automation.ScriptStore) error {
	_ = store
	return nil
}
