package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupPrepareRemoteMetadataUsesRemoteZIPName(t *testing.T) {
	directory := t.TempDir()
	metadataPath := filepath.Join(directory, "local.json")
	data, err := json.Marshal(backupMetadata{
		Format:     "ant-chrome-backup-metadata",
		Version:    backupMetadataVersion,
		BackupFile: "local.zip",
	})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, data, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	uploadPath, cleanup, err := backupPrepareRemoteMetadata(metadataPath, "local.zip", "remote.zip")
	if err != nil {
		t.Fatalf("prepare remote metadata: %v", err)
	}
	defer cleanup()
	content, err := os.ReadFile(uploadPath)
	if err != nil {
		t.Fatalf("read prepared metadata: %v", err)
	}
	var uploaded backupMetadata
	if err := json.Unmarshal(content, &uploaded); err != nil {
		t.Fatalf("parse prepared metadata: %v", err)
	}
	if uploaded.BackupFile != "remote.zip" {
		t.Fatalf("prepared backup file = %q, want remote.zip", uploaded.BackupFile)
	}
	content, err = os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read original metadata: %v", err)
	}
	var original backupMetadata
	if err := json.Unmarshal(content, &original); err != nil {
		t.Fatalf("parse original metadata: %v", err)
	}
	if original.BackupFile != "local.zip" {
		t.Fatalf("original backup file = %q, want local.zip", original.BackupFile)
	}
}
