package backend

import (
	"fmt"
)

func (a *App) backupCheckpointSQLiteWAL() error {
	if a == nil || a.db == nil || a.db.GetConn() == nil {
		return nil
	}

	var busy, logFrames, checkpointedFrames int
	if err := a.db.GetConn().QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		return fmt.Errorf(`SQLite WAL checkpoint failed: %w`, err)
	}
	if busy != 0 {
		return fmt.Errorf(`SQLite WAL checkpoint is busy: log_frames=%d checkpointed_frames=%d`, logFrames, checkpointedFrames)
	}
	return nil
}
