package proxy

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestXrayManagerResolveBinaryFromEnvironment(t *testing.T) {
	path := writeRuntimeBinary(t, "xray")
	t.Setenv("XRAY_BINARY_PATH", path)

	cfg := config.DefaultConfig()
	cfg.Browser.XrayBinaryPath = ""
	manager := &XrayManager{Config: cfg, AppRoot: t.TempDir()}

	got, err := manager.resolveBinary()
	if err != nil {
		t.Fatalf("resolveBinary() returned error: %v", err)
	}
	if got != path {
		t.Errorf("resolveBinary() = %q, want %q", got, path)
	}
	checkRuntimeBinaryExecutable(t, path)
}

func TestSingBoxManagerResolveBinaryFromEnvironment(t *testing.T) {
	path := writeRuntimeBinary(t, "sing-box")
	t.Setenv("SINGBOX_BINARY_PATH", path)

	cfg := config.DefaultConfig()
	cfg.Browser.SingBoxBinaryPath = ""
	manager := &SingBoxManager{Config: cfg, AppRoot: t.TempDir()}

	got, err := manager.resolveBinary()
	if err != nil {
		t.Fatalf("resolveBinary() returned error: %v", err)
	}
	if got != path {
		t.Errorf("resolveBinary() = %q, want %q", got, path)
	}
	checkRuntimeBinaryExecutable(t, path)
}

func TestProxyManagersResolvePackagedRuntimeLayout(t *testing.T) {
	appRoot := t.TempDir()
	platformDir := runtime.GOOS + "-" + runtime.GOARCH
	xrayPath := writeRuntimeBinaryAt(t, filepath.Join(appRoot, "bin", platformDir, runtimeBinaryName("xray")))
	singBoxPath := writeRuntimeBinaryAt(t, filepath.Join(appRoot, "bin", platformDir, runtimeBinaryName("sing-box")))

	cfg := config.DefaultConfig()
	cfg.Browser.XrayBinaryPath = ""
	cfg.Browser.SingBoxBinaryPath = ""
	t.Setenv("XRAY_BINARY_PATH", "")
	t.Setenv("SINGBOX_BINARY_PATH", "")

	xrayManager := &XrayManager{Config: cfg, AppRoot: appRoot}
	if got, err := xrayManager.resolveBinary(); err != nil || got != xrayPath {
		t.Fatalf("xray resolveBinary() = %q, %v, want %q", got, err, xrayPath)
	}
	singBoxManager := &SingBoxManager{Config: cfg, AppRoot: appRoot}
	if got, err := singBoxManager.resolveBinary(); err != nil || got != singBoxPath {
		t.Fatalf("sing-box resolveBinary() = %q, %v, want %q", got, err, singBoxPath)
	}
}

func writeRuntimeBinary(t *testing.T, name string) string {
	t.Helper()
	return writeRuntimeBinaryAt(t, filepath.Join(t.TempDir(), name))
}

func writeRuntimeBinaryAt(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("runtime"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) returned error: %v", path, err)
	}
	return path
}

func runtimeBinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func checkRuntimeBinaryExecutable(t *testing.T, path string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) returned error: %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("resolved runtime mode = %v, want an executable mode", info.Mode())
	}
}
