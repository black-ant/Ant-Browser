package backend

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestBackupOpenListRevealTokenReturnsStoredValue(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	defer app.stopBackupScheduler()

	if _, err := app.BackupOpenListSaveSettings(map[string]string{
		`baseURL`:    `https://openlist.example.com/dav`,
		`remotePath`: `backups`,
		`token`:      `openlist-token`,
	}); err != nil {
		t.Fatalf(`save OpenList settings: %v`, err)
	}

	got, err := app.BackupOpenListRevealToken()
	if err != nil {
		t.Fatalf(`reveal OpenList token: %v`, err)
	}
	if got != `openlist-token` {
		t.Fatalf(`revealed OpenList token = %q, want %q`, got, `openlist-token`)
	}
}
