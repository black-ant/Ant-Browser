package backend

import (
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

func newLaunchCodeTestApp(t *testing.T) *App {
	t.Helper()

	cfg := config.DefaultConfig()
	app := NewApp(t.TempDir())
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, app.appRoot)
	app.launchCodeSvc = launchcode.NewLaunchCodeService(launchcode.NewMemoryLaunchCodeDAO())
	app.browserMgr.CodeProvider = app.launchCodeSvc
	return app
}

func TestBrowserProfileCreateAppliesRequestedLaunchCode(t *testing.T) {
	app := newLaunchCodeTestApp(t)

	profile, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName: "buyer-create",
		LaunchCode:  "buyer_ui",
	})
	if err != nil {
		t.Fatalf("BrowserProfileCreate returned error: %v", err)
	}
	if profile.LaunchCode != "BUYER_UI" {
		t.Fatalf("expected launch code to be normalized, got %q", profile.LaunchCode)
	}

	resolvedProfileID, err := app.launchCodeSvc.Resolve("buyer_ui")
	if err != nil {
		t.Fatalf("expected launch code to resolve: %v", err)
	}
	if resolvedProfileID != profile.ProfileId {
		t.Fatalf("launch code resolved wrong profile: got=%s want=%s", resolvedProfileID, profile.ProfileId)
	}
}

func TestBrowserProfileCreateRollsBackOnDuplicateLaunchCode(t *testing.T) {
	app := newLaunchCodeTestApp(t)

	existing, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName: "buyer-existing",
		LaunchCode:  "BUYER_DUP",
	})
	if err != nil {
		t.Fatalf("creating existing profile failed: %v", err)
	}
	before := len(app.browserMgr.List())

	_, err = app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName: "buyer-duplicate",
		LaunchCode:  "buyer_dup",
	})
	if err == nil {
		t.Fatal("expected duplicate launch code to fail")
	}

	after := len(app.browserMgr.List())
	if after != before {
		t.Fatalf("duplicate launch code should roll back created profile: before=%d after=%d", before, after)
	}

	resolvedProfileID, err := app.launchCodeSvc.Resolve("BUYER_DUP")
	if err != nil {
		t.Fatalf("expected original launch code to remain resolvable: %v", err)
	}
	if resolvedProfileID != existing.ProfileId {
		t.Fatalf("duplicate rollback changed launch code owner: got=%s want=%s", resolvedProfileID, existing.ProfileId)
	}
}
