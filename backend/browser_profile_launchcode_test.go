package backend

import (
	"os"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

func newBrowserLaunchCodeTestApp(t *testing.T) *App {
	t.Helper()

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.launchCodeSvc = launchcode.NewLaunchCodeService(launchcode.NewMemoryLaunchCodeDAO())
	app.browserMgr.CodeProvider = app.launchCodeSvc
	return app
}

func TestBrowserProfileSetCodeUpdatesManagedProfile(t *testing.T) {
	app := newBrowserLaunchCodeTestApp(t)

	profile, err := app.browserMgr.Create(browser.ProfileInput{ProfileName: "mail-profile"})
	if err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	if profile == nil {
		t.Fatal("create profile returned nil")
	}

	code, err := app.BrowserProfileSetCode(profile.ProfileId, "mail01")
	if err != nil {
		t.Fatalf("BrowserProfileSetCode failed: %v", err)
	}
	if code != "MAIL01" {
		t.Fatalf("expected normalized code MAIL01, got %s", code)
	}

	if got := app.browserMgr.Profiles[profile.ProfileId].LaunchCode; got != "MAIL01" {
		t.Fatalf("expected managed profile launch code MAIL01, got %s", got)
	}
}

func TestEnsurePlaywrightTargetReadyResolvesUpdatedLaunchCode(t *testing.T) {
	app := newBrowserLaunchCodeTestApp(t)

	profile, err := app.browserMgr.Create(browser.ProfileInput{ProfileName: "mail-profile"})
	if err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	if profile == nil {
		t.Fatal("create profile returned nil")
	}

	if _, err := app.launchCodeSvc.SetCode(profile.ProfileId, "MAIL01"); err != nil {
		t.Fatalf("set launch code failed: %v", err)
	}

	managed := app.browserMgr.Profiles[profile.ProfileId]
	managed.Running = true
	managed.Pid = os.Getpid()

	selector, taskProfileID, err := app.ensurePlaywrightTargetReady(map[string]any{
		"profileId": profile.ProfileId,
	})
	if err != nil {
		t.Fatalf("ensurePlaywrightTargetReady failed: %v", err)
	}
	if taskProfileID != profile.ProfileId {
		t.Fatalf("expected task profile id %s, got %s", profile.ProfileId, taskProfileID)
	}
	if got, _ := selector["code"].(string); got != "MAIL01" {
		t.Fatalf("expected selector code MAIL01, got %v", selector["code"])
	}
	if managed.LaunchCode != "MAIL01" {
		t.Fatalf("expected managed profile launch code to sync to MAIL01, got %s", managed.LaunchCode)
	}
}
