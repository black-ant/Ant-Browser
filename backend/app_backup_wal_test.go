package backend

import (
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestBackupCheckpointSQLiteWAL(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.GetConn().Exec(`CREATE TABLE backup_checkpoint_test (value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table returned error: %v", err)
	}
	if _, err := db.GetConn().Exec(`INSERT INTO backup_checkpoint_test (value) VALUES ('checkpointed')`); err != nil {
		t.Fatalf("insert returned error: %v", err)
	}

	app := NewApp(t.TempDir())
	app.db = db
	if err := app.backupCheckpointSQLiteWAL(); err != nil {
		t.Fatalf("backupCheckpointSQLiteWAL returned error: %v", err)
	}
}
