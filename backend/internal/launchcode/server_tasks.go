package launchcode

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LaunchTask struct {
	ID        string `json:"id"`
	StartedAt string `json:"startedAt"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	ClientIP  string `json:"clientIp"`
	Status    string `json:"status"`
	ElapsedMs int64  `json:"elapsedMs"`
}

type launchTaskState struct {
	LaunchTask
	startedAt time.Time
}

func (s *LaunchServer) beginLaunchTask(r *http.Request) string {
	startedAt := time.Now()
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	s.nextTaskID++
	id := fmt.Sprintf("task-%d", s.nextTaskID)
	s.activeTasks[id] = &launchTaskState{
		LaunchTask: LaunchTask{
			ID:        id,
			StartedAt: startedAt.Format(time.RFC3339),
			Method:    r.Method,
			Path:      r.URL.Path,
			ClientIP:  remoteIP(r.RemoteAddr),
			Status:    "running",
		},
		startedAt: startedAt,
	}
	return id
}

func (s *LaunchServer) finishLaunchTask(id string) {
	s.taskMu.Lock()
	delete(s.activeTasks, id)
	s.taskMu.Unlock()
}

func (s *LaunchServer) trackLaunchTask(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		taskID := s.beginLaunchTask(r)
		defer s.finishLaunchTask(taskID)
		next.ServeHTTP(w, r)
	})
}

func (s *LaunchServer) listLaunchTasks(limit int) []LaunchTask {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	now := time.Now()
	s.taskMu.Lock()
	items := make([]LaunchTask, 0, len(s.activeTasks))
	for _, task := range s.activeTasks {
		item := task.LaunchTask
		item.ElapsedMs = now.Sub(task.startedAt).Milliseconds()
		if item.ElapsedMs < 0 {
			item.ElapsedMs = 0
		}
		items = append(items, item)
	}
	s.taskMu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].StartedAt > items[j].StartedAt
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *LaunchServer) handleLaunchTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "method not allowed",
		})
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 100 {
				n = 100
			}
			limit = n
		}
	}

	items := s.listLaunchTasks(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"items": items,
		"count": len(items),
		"limit": limit,
	})
}
