package backend

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"ant-chrome/backend/internal/appupdate"
)

func (a *App) appUpdateLayout() appupdate.Layout {
	layout := a.runtimeLayout()
	installRoot := appUpdateInstallRootForOS(goruntime.GOOS, layout.InstallRoot)
	return appupdate.NewLayout(installRoot, appUpdateStateRootForOS(goruntime.GOOS, installRoot, layout.StateRoot))
}

func (a *App) appUpdateManager() appupdate.Manager {
	layout := a.appUpdateLayout()
	platform, err := appupdate.NewPlatformBackend(goruntime.GOOS, goruntime.GOARCH, appupdate.PlatformOptions{
		ProcessID: os.Getpid(),
	})
	if err != nil {
		platform = appupdate.UnsupportedBackend{Err: err}
	}
	return appupdate.Manager{
		LocalAppVersion: a.appVersion(),
		Layout:          layout,
		Platform:        platform,
		ManifestProvider: appupdate.DefaultManifestProvider(func() appupdate.ManifestSourceResolution {
			return appupdate.ResolveManifestSource(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
		}),
	}
}

func (a *App) CheckDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Check(a.appUpdateContext())
}

func (a *App) DownloadDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Download(a.appUpdateContext())
}

func (a *App) ApplyDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Apply(a.appUpdateContext())
}

func (a *App) GetDesktopAppUpdateState() (appupdate.State, error) {
	persistent, err := appupdate.ReadState(a.appUpdateLayout())
	if os.IsNotExist(err) {
		return appupdate.State{
			Kind:            appupdate.UpdateKindNone,
			Status:          appupdate.PersistentStatusIdle,
			LocalAppVersion: a.appVersion(),
		}, nil
	}
	if err != nil {
		return appupdate.State{}, err
	}
	return appUpdatePersistentStateToAPI(persistent, a.appVersion()), nil
}

func (a *App) ClearDesktopAppUpdateFailure() error {
	layout := a.appUpdateLayout()
	if err := os.Remove(layout.StatePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(layout.PlanPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *App) appUpdateContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func appUpdateStateRootForOS(goos, installRoot, fallbackStateRoot string) string {
	if strings.EqualFold(strings.TrimSpace(goos), "windows") {
		if root := windowsAppUpdateStateRoot(); strings.TrimSpace(root) != "" && !pathInsideRoot(root, installRoot) {
			return root
		}
	}
	return fallbackStateRoot
}

func appUpdateInstallRootForOS(goos, installRoot string) string {
	installRoot = filepath.Clean(strings.TrimSpace(installRoot))
	if !strings.EqualFold(strings.TrimSpace(goos), "darwin") {
		return installRoot
	}
	if strings.HasSuffix(strings.ToLower(filepath.Base(installRoot)), ".app") {
		return installRoot
	}
	for dir := installRoot; dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if strings.HasSuffix(strings.ToLower(filepath.Base(dir)), ".app") {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
	}
	return installRoot
}

func windowsAppUpdateStateRoot() string {
	if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); base != "" {
		return filepath.Join(base, "Ant Browser")
	}
	if base, err := os.UserConfigDir(); err == nil && strings.TrimSpace(base) != "" {
		return filepath.Join(base, "Ant Browser")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, "AppData", "Local", "Ant Browser")
	}
	return ""
}

func pathInsideRoot(path, root string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	root = filepath.Clean(strings.TrimSpace(root))
	if path == "" || root == "" {
		return false
	}
	if strings.EqualFold(path, root) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func appUpdatePersistentStateToAPI(persistent appupdate.PersistentState, fallbackLocalVersion string) appupdate.State {
	kind := appupdate.UpdateKindNone
	if persistent.Status == appupdate.PersistentStatusRolledBack ||
		persistent.Status == appupdate.PersistentStatusFailedManualRepair ||
		persistent.LastError.Code != "" {
		kind = appupdate.UpdateKindFailed
	}
	localVersion := persistent.LocalAppVersion
	if localVersion == "" {
		localVersion = fallbackLocalVersion
	}
	return appupdate.State{
		Kind:             kind,
		Status:           persistent.Status,
		LocalAppVersion:  localVersion,
		RemoteAppVersion: persistent.RemoteAppVersion,
		ManifestSource:   persistent.ManifestSource,
		ManifestURL:      persistent.ManifestURL,
		PayloadURL:       persistent.PayloadURL,
		Target:           persistent.Target,
		ErrorCode:        persistent.LastError.Code,
		ErrorMessage:     persistent.LastError.Message,
		Details:          persistent.LastError.Details,
	}
}
