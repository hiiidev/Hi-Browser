package backend

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestParseRemoteDebuggingPort(t *testing.T) {
	t.Parallel()

	got := parseRemoteDebuggingPort(`chrome.exe --user-data-dir="D:\data\p1" --remote-debugging-port=49152`)
	if got != 49152 {
		t.Fatalf("expected port 49152, got %d", got)
	}
}

func TestBrowserInstanceStatusRecoversRunningProfileByUserDataDir(t *testing.T) {
	server := newRecordedCDPServer(t)
	defer server.Close()

	app := newRuntimeRecoveryTestApp(t)
	profile := &BrowserProfile{
		ProfileId:   "recover-status",
		ProfileName: "Recover Status",
		UserDataDir: "recover-status",
		Running:     false,
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}
	userDataDir := app.browserMgr.ResolveUserDataDir(profile)
	writeDevToolsActivePort(t, userDataDir, server.Port())

	snapshot, err := app.BrowserInstanceStatus(profile.ProfileId)
	if err != nil {
		t.Fatalf("BrowserInstanceStatus returned error: %v", err)
	}
	if snapshot == nil || !snapshot.Running || !snapshot.DebugReady {
		t.Fatalf("expected recovered running profile, got %+v", snapshot)
	}
	if snapshot.DebugPort != server.Port() {
		t.Fatalf("expected debug port %d, got %d", server.Port(), snapshot.DebugPort)
	}
}

func TestBrowserInstanceStartRecoversRunningProfileBeforeSessionCleanup(t *testing.T) {
	server := newRecordedCDPServer(t)
	defer server.Close()

	app := newRuntimeRecoveryTestApp(t)
	profile := &BrowserProfile{
		ProfileId:   "recover-start",
		ProfileName: "Recover Start",
		UserDataDir: "recover-start",
		Running:     false,
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}
	userDataDir := app.browserMgr.ResolveUserDataDir(profile)
	writeDevToolsActivePort(t, userDataDir, server.Port())

	snapshot, err := app.BrowserInstanceStart(profile.ProfileId)
	if err != nil {
		t.Fatalf("BrowserInstanceStart returned error: %v", err)
	}
	if snapshot == nil || !snapshot.Running || !snapshot.DebugReady {
		t.Fatalf("expected recovered running profile, got %+v", snapshot)
	}
	if snapshot.DebugPort != server.Port() {
		t.Fatalf("expected debug port %d, got %d", server.Port(), snapshot.DebugPort)
	}
}

func TestPrepareBrowserLaunchContextSkipsProcessScanWhenNoActivePort(t *testing.T) {
	app := newRuntimeRecoveryTestApp(t)
	profile := &BrowserProfile{
		ProfileId:   "cold-start",
		ProfileName: "Cold Start",
		UserDataDir: "cold-start",
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}

	originalFind := findBrowserUserDataProcesses
	defer func() { findBrowserUserDataProcesses = originalFind }()

	findCalls := 0
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		findCalls++
		return nil, nil
	}

	_, _, _, _, err := app.prepareBrowserLaunchContext(newBrowserStartInput(profile.ProfileId, nil, nil, false, false, false, "", ""), profile, nil)
	if err != nil {
		t.Fatalf("prepareBrowserLaunchContext returned error: %v", err)
	}
	if findCalls != 0 {
		t.Fatalf("cold start should not scan browser processes, got %d calls", findCalls)
	}
}
func TestPrepareBrowserLaunchContextTerminatesProcessAndRetriesSessionCleanup(t *testing.T) {
	app := newRuntimeRecoveryTestApp(t)
	profile := &BrowserProfile{
		ProfileId:   "recover-lock",
		ProfileName: "Recover Lock",
		UserDataDir: "recover-lock",
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}
	userDataDir := app.browserMgr.ResolveUserDataDir(profile)
	sessionsDir := filepath.Join(userDataDir, "Default", "Sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("create sessions dir failed: %v", err)
	}

	originalFind := findBrowserUserDataProcesses
	originalTerminate := terminateBrowserUserDataProcess
	originalClear := clearBrowserSessionRestoreData
	defer func() {
		findBrowserUserDataProcesses = originalFind
		terminateBrowserUserDataProcess = originalTerminate
		clearBrowserSessionRestoreData = originalClear
	}()

	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{{PID: 4321}}, nil
	}
	terminatedPID := 0
	terminateBrowserUserDataProcess = func(pid int, timeout time.Duration) error {
		terminatedPID = pid
		return nil
	}
	clearCalls := 0
	clearBrowserSessionRestoreData = func(string) error {
		clearCalls++
		if clearCalls == 1 {
			return errors.New("remove sessions dir: locked")
		}
		return nil
	}

	_, _, _, _, err := app.prepareBrowserLaunchContext(newBrowserStartInput(profile.ProfileId, nil, nil, false, false, false, "", ""), profile, nil)
	if err != nil {
		t.Fatalf("prepareBrowserLaunchContext returned error: %v", err)
	}
	if terminatedPID != 4321 {
		t.Fatalf("expected terminating pid 4321, got %d", terminatedPID)
	}
	if clearCalls != 2 {
		t.Fatalf("expected session cleanup to be retried once, got %d calls", clearCalls)
	}
}

func newRuntimeRecoveryTestApp(t *testing.T) *App {
	t.Helper()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = "data"
	cfg.Browser.RestoreLastSession = false
	exePath := createRuntimeRecoveryFakeBrowserExecutable(t, appRoot)
	cfg.Browser.Cores = []config.BrowserCore{
		{CoreId: "runtime-recovery-core", CoreName: "Runtime Recovery Core", CorePath: exePath, IsDefault: true},
	}

	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, appRoot)
	app.browserMgr.Profiles = make(map[string]*BrowserProfile)
	app.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
	return app
}

func createRuntimeRecoveryFakeBrowserExecutable(t *testing.T, appRoot string) string {
	t.Helper()

	path := filepath.Join(appRoot, "chrome", "chrome.exe")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fake browser dir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write fake browser failed: %v", err)
	}
	return filepath.Dir(path)
}

func writeDevToolsActivePort(t *testing.T, userDataDir string, debugPort int) {
	t.Helper()

	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatalf("create user data dir failed: %v", err)
	}
	content := fmt.Sprintf("%d\n/devtools/browser/test\n", debugPort)
	if err := os.WriteFile(filepath.Join(userDataDir, "DevToolsActivePort"), []byte(content), 0o644); err != nil {
		t.Fatalf("write DevToolsActivePort failed: %v", err)
	}
}
