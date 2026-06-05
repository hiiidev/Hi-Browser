package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestBrowserInstanceOpenURLUsesCDPCreateTarget(t *testing.T) {
	t.Parallel()

	server := newRecordedCDPServer(t)
	defer server.Close()

	app := newBrowserOpenURLTestApp(t)
	app.browserMgr.Profiles = map[string]*BrowserProfile{
		"profile-ready": {
			ProfileId:   "profile-ready",
			ProfileName: "Ready Browser",
			Running:     true,
			DebugReady:  true,
			DebugPort:   server.Port(),
			Pid:         12345,
		},
	}
	app.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)

	ok, err := app.BrowserInstanceOpenUrl("profile-ready", "https://open.example/")
	if err != nil {
		t.Fatalf("BrowserInstanceOpenUrl returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected BrowserInstanceOpenUrl to succeed")
	}

	want := []recordedCDPCommand{
		{Scope: "browser", Method: "Target.createTarget", URL: "https://open.example/"},
	}
	if got := server.Commands(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected CDP command sequence:\n got=%v\nwant=%v", got, want)
	}
}

func TestBrowserInstanceOpenURLFallsBackToWindowWhenDebugPending(t *testing.T) {
	app, exePath := newBrowserOpenURLTestAppWithCore(t)

	cmd := longLivedCommand(2 * time.Second)
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动长生命周期测试进程失败: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	profile := &BrowserProfile{
		ProfileId:   "profile-pending",
		ProfileName: "Pending Browser",
		UserDataDir: "profile-pending",
		Running:     true,
		DebugReady:  false,
		DebugPort:   0,
		Pid:         cmd.Process.Pid,
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}
	app.browserMgr.BrowserProcesses = map[string]*exec.Cmd{profile.ProfileId: cmd}

	expectedUserDataDir := app.browserMgr.ResolveUserDataDir(profile)
	var gotPath string
	var gotArgs []string

	originalStart := startBrowserWindowProcess
	startBrowserWindowProcess = func(chromeBinaryPath string, args []string) (*exec.Cmd, error) {
		gotPath = chromeBinaryPath
		gotArgs = append([]string{}, args...)
		return nil, nil
	}
	defer func() {
		startBrowserWindowProcess = originalStart
	}()

	ok, err := app.BrowserInstanceOpenUrl(profile.ProfileId, "https://pending.example/")
	if err != nil {
		t.Fatalf("BrowserInstanceOpenUrl returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected BrowserInstanceOpenUrl to succeed")
	}
	if gotPath != exePath {
		t.Fatalf("unexpected browser path: got=%q want=%q", gotPath, exePath)
	}

	wantArgs := []string{
		fmt.Sprintf("--user-data-dir=%s", expectedUserDataDir),
		"https://pending.example/",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected browser args:\n got=%v\nwant=%v", gotArgs, wantArgs)
	}
}

func TestBrowserInstanceOpenURLFallsBackToWindowWhenCDPOpenFails(t *testing.T) {
	app, exePath := newBrowserOpenURLTestAppWithCore(t)

	cmd := longLivedCommand(2 * time.Second)
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动长生命周期测试进程失败: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	profile := &BrowserProfile{
		ProfileId:   "profile-ready-fallback",
		ProfileName: "Fallback Browser",
		UserDataDir: "profile-ready-fallback",
		Running:     true,
		DebugReady:  true,
		DebugPort:   freeLoopbackPort(t),
		Pid:         cmd.Process.Pid,
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}
	app.browserMgr.BrowserProcesses = map[string]*exec.Cmd{profile.ProfileId: cmd}

	expectedUserDataDir := app.browserMgr.ResolveUserDataDir(profile)
	var gotPath string
	var gotArgs []string

	originalStart := startBrowserWindowProcess
	startBrowserWindowProcess = func(chromeBinaryPath string, args []string) (*exec.Cmd, error) {
		gotPath = chromeBinaryPath
		gotArgs = append([]string{}, args...)
		return nil, nil
	}
	defer func() {
		startBrowserWindowProcess = originalStart
	}()

	ok, err := app.BrowserInstanceOpenUrl(profile.ProfileId, "https://fallback.example/")
	if err != nil {
		t.Fatalf("BrowserInstanceOpenUrl returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected BrowserInstanceOpenUrl to succeed")
	}
	if gotPath != exePath {
		t.Fatalf("unexpected browser path: got=%q want=%q", gotPath, exePath)
	}

	wantArgs := []string{
		fmt.Sprintf("--user-data-dir=%s", expectedUserDataDir),
		"https://fallback.example/",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected browser args:\n got=%v\nwant=%v", gotArgs, wantArgs)
	}
}

func TestBrowserInstanceOpenURLMarksStaleProfileStopped(t *testing.T) {
	t.Parallel()

	app := newBrowserOpenURLTestApp(t)
	profile := &BrowserProfile{
		ProfileId:   "profile-stale",
		ProfileName: "Stale Browser",
		Running:     true,
		DebugReady:  false,
	}
	app.browserMgr.Profiles = map[string]*BrowserProfile{profile.ProfileId: profile}
	app.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)

	ok, err := app.BrowserInstanceOpenUrl(profile.ProfileId, "https://stale.example/")
	if err == nil {
		t.Fatal("expected BrowserInstanceOpenUrl to fail for stale runtime state")
	}
	if ok {
		t.Fatal("expected BrowserInstanceOpenUrl to return false for stale runtime state")
	}
	if !strings.Contains(err.Error(), "运行状态已失效") {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Running {
		t.Fatal("expected stale profile to be marked stopped")
	}
	if profile.DebugReady {
		t.Fatal("expected stale profile debug state to be cleared")
	}
	if profile.DebugPort != 0 || profile.Pid != 0 {
		t.Fatalf("expected runtime identifiers to be cleared, got debugPort=%d pid=%d", profile.DebugPort, profile.Pid)
	}
}

func newBrowserOpenURLTestApp(t *testing.T) *App {
	t.Helper()

	cfg := config.DefaultConfig()
	app := NewApp("")
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, t.TempDir())
	app.browserMgr.Profiles = make(map[string]*BrowserProfile)
	app.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
	return app
}

func newBrowserOpenURLTestAppWithCore(t *testing.T) (*App, string) {
	t.Helper()

	cfg := config.DefaultConfig()
	exePath := createFakeBrowserExecutable(t)
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:    "core-open-url-test",
			CoreName:  "Open URL Test Core",
			CorePath:  exePath,
			IsDefault: true,
		},
	}

	app := NewApp("")
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, t.TempDir())
	app.browserMgr.Profiles = make(map[string]*BrowserProfile)
	app.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
	return app, exePath
}

func createFakeBrowserExecutable(t *testing.T) string {
	t.Helper()

	candidates := browser.CoreExecutableCandidates()
	if len(candidates) == 0 {
		t.Fatal("no core executable candidates available")
	}

	baseDir := t.TempDir()
	exePath := filepath.Join(baseDir, filepath.FromSlash(candidates[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("创建测试内核目录失败: %v", err)
	}

	mode := os.FileMode(0o644)
	content := []byte("test-browser")
	if goruntime.GOOS != "windows" {
		mode = 0o755
		content = []byte("#!/bin/sh\nexit 0\n")
	}
	if err := os.WriteFile(exePath, content, mode); err != nil {
		t.Fatalf("写入测试内核可执行文件失败: %v", err)
	}
	return exePath
}
