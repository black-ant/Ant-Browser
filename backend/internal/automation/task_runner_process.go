package automation

import (
	"fmt"
	"os/exec"
	goruntime "runtime"
	"strings"

	"github.com/google/uuid"
)

func (m *Manager) StopAllTasks() {
	m.closeAllPageSessions()

	m.mu.Lock()
	tasks := make([]*activeTask, 0, len(m.activeTasks))
	for _, task := range m.activeTasks {
		tasks = append(tasks, task)
	}
	m.activeTasks = make(map[string]*activeTask)
	m.profileTask = make(map[string]string)
	m.mu.Unlock()

	for _, task := range tasks {
		if task == nil || task.cmd == nil || task.cmd.Process == nil {
			continue
		}
		_ = stopTaskProcess(task.cmd)
	}
}

func (m *Manager) registerTask(profileID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.profileTask[profileID]; ok && strings.TrimSpace(existing) != "" {
		return "", fmt.Errorf("实例 %s 已有自动化任务在运行中", profileID)
	}
	taskID := uuid.NewString()
	m.profileTask[profileID] = taskID
	m.activeTasks[taskID] = &activeTask{
		taskID:    taskID,
		profileID: profileID,
	}
	return taskID, nil
}

func (m *Manager) attachTaskCommand(taskID string, cmd *exec.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.activeTasks[taskID]; ok && task != nil {
		task.cmd = cmd
	}
}

func (m *Manager) unregisterTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.activeTasks[taskID]
	if !ok || task == nil {
		return
	}
	delete(m.activeTasks, taskID)
	if current, ok := m.profileTask[task.profileID]; ok && current == taskID {
		delete(m.profileTask, task.profileID)
	}
}

func (m *Manager) emitTaskEvent(event TaskEvent) {
	if m.emit == nil {
		return
	}
	m.emit(TaskEventName, event)
}

func stopTaskProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if goruntime.GOOS == "windows" {
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
		hideWindow(killCmd)
		if err := killCmd.Run(); err == nil {
			return nil
		}
	} else if cmd.Process.Pid > 0 {
		if err := killProcessGroup(cmd.Process.Pid); err == nil {
			return nil
		}
	}
	err := cmd.Process.Kill()
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "already finished") {
		return nil
	}
	return err
}
