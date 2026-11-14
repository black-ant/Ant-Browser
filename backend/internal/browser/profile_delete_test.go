package browser

import (
	"ant-chrome/backend/internal/config"
	"errors"
	"os/exec"
	"testing"
)

func TestDeleteRejectsRunningProfile(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), t.TempDir())
	profile := &Profile{ProfileId: "profile-running", Running: true}
	manager.Profiles[profile.ProfileId] = profile

	err := manager.Delete(profile.ProfileId)
	if !errors.Is(err, ErrProfileRunning) {
		t.Fatalf("Delete() error = %v, want ErrProfileRunning", err)
	}
	if _, exists := manager.Profiles[profile.ProfileId]; !exists {
		t.Fatal("Delete() removed a running profile")
	}
	if profile.DeletedAt != "" {
		t.Fatalf("Delete() set DeletedAt = %q for a running profile", profile.DeletedAt)
	}
}

func TestDeleteRejectsProfileWithActiveProcess(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), t.TempDir())
	profile := &Profile{ProfileId: "profile-starting"}
	manager.Profiles[profile.ProfileId] = profile
	manager.BrowserProcesses[profile.ProfileId] = &exec.Cmd{}

	err := manager.Delete(profile.ProfileId)
	if !errors.Is(err, ErrProfileRunning) {
		t.Fatalf("Delete() error = %v, want ErrProfileRunning", err)
	}
	if _, exists := manager.Profiles[profile.ProfileId]; !exists {
		t.Fatal("Delete() removed a profile with an active process")
	}
}

func TestDeleteRejectsDetectedRuntime(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), t.TempDir())
	profile := &Profile{ProfileId: "profile-detected-runtime"}
	manager.Profiles[profile.ProfileId] = profile
	manager.ProfileRuntimeDetector = func(candidate *Profile) bool {
		return candidate != nil && candidate.ProfileId == profile.ProfileId
	}

	err := manager.Delete(profile.ProfileId)
	if !errors.Is(err, ErrProfileRunning) {
		t.Fatalf("Delete() error = %v, want ErrProfileRunning", err)
	}
	if _, exists := manager.Profiles[profile.ProfileId]; !exists {
		t.Fatal("Delete() removed a profile with a detected runtime")
	}
}
