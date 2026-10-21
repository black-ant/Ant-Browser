package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupImportRollbackRestoresDatabaseAndFiles(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	app.config = config.DefaultConfig()
	if err := app.config.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proxies.yaml"), []byte("old-proxies"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	db, err := database.NewDB(filepath.Join(root, "data", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	app.db = db
	if _, err := db.GetConn().Exec(`INSERT INTO browser_groups (group_id, group_name) VALUES ('original', 'Original')`); err != nil {
		t.Fatal(err)
	}

	rollback, err := app.beginBackupImportRollback()
	if err != nil {
		t.Fatal(err)
	}
	defer rollback.cleanup()
	if _, err := db.GetConn().Exec(`INSERT INTO browser_groups (group_id, group_name) VALUES ('imported', 'Imported')`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("changed-config"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proxies.yaml"), []byte("changed-proxies"), 0644); err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(root, "data", "imported.txt")
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	rollback.recordCreatedPath(newFile)

	if err := rollback.rollback(app); err != nil {
		t.Fatal(err)
	}
	var groupName string
	if err := db.GetConn().QueryRow(`SELECT group_name FROM browser_groups WHERE group_id = 'original'`).Scan(&groupName); err != nil {
		t.Fatal(err)
	}
	if groupName != "Original" {
		t.Fatalf("original group = %q", groupName)
	}
	var importedCount int
	if err := db.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_groups WHERE group_id = 'imported'`).Scan(&importedCount); err != nil {
		t.Fatal(err)
	}
	if importedCount != 0 {
		t.Fatalf("imported group remains: %d", importedCount)
	}
	configData, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(configData) == "changed-config" {
		t.Fatal("config was not restored")
	}
	proxyData, err := os.ReadFile(filepath.Join(root, "proxies.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(proxyData) != "old-proxies" {
		t.Fatalf("proxies = %q", proxyData)
	}
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Fatalf("new file still exists, err=%v", err)
	}
}
