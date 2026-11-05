package backend

import (
	"context"
	"testing"

	"ant-chrome/backend/internal/logger"
)

func TestFrontendOperationLogHonorsDebugLevel(t *testing.T) {
	logger.Init(context.Background(), `info`)
	defer logger.Close()

	writer := logger.GetMemoryWriter()
	writer.Clear()
	app := NewApp(t.TempDir())
	app.FrontendOperationLog(`debug`, `BackupOpenListList`, true, 4, ``)
	if entries := writer.GetEntries(); len(entries) != 0 {
		t.Fatalf(`debug operation was recorded at info level: %+v`, entries)
	}

	app.FrontendOperationLog(`info`, `BackupCreatePackage`, true, 8, ``)
	entries := writer.GetEntries()
	if len(entries) != 1 || entries[0].Level != `INFO` || entries[0].Message != `前端操作完成` {
		t.Fatalf(`info operation entries = %+v, want one INFO entry`, entries)
	}

	writer.Clear()
	app.FrontendOperationLog(` Error `, `BackupRestore`, true, 12, `forced failure level`)
	entries = writer.GetEntries()
	if len(entries) != 1 || entries[0].Level != `ERROR` || entries[0].Message != `前端操作失败` {
		t.Fatalf(`error operation entries = %+v, want one ERROR entry`, entries)
	}
}
