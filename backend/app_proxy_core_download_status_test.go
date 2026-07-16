package backend

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestProxyCoreStatusUsesRuntimeEnvironment(t *testing.T) {
	tests := []struct {
		name          string
		core          string
		environment   string
		connectorType string
		wantMessage   string
	}{
		{
			name:          "active xray",
			core:          "xray",
			environment:   "XRAY_BINARY_PATH",
			connectorType: "xray",
			wantMessage:   "已启用",
		},
		{
			name:          "configured sing-box",
			core:          "sing-box",
			environment:   "SINGBOX_BINARY_PATH",
			connectorType: "xray",
			wantMessage:   "已配置",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binaryPath := filepath.Join(t.TempDir(), test.core)
			if err := os.WriteFile(binaryPath, []byte("runtime"), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) returned error: %v", binaryPath, err)
			}
			t.Setenv(test.environment, binaryPath)

			cfg := config.DefaultConfig()
			cfg.Browser.DefaultConnectorType = test.connectorType
			app := NewApp(t.TempDir())
			app.config = cfg
			spec, err := normalizeProxyCoreSpec(test.core)
			if err != nil {
				t.Fatalf("normalizeProxyCoreSpec(%q) returned error: %v", test.core, err)
			}

			got := app.proxyCoreStatus(spec, proxyCoreTarget{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
			if !got.Installed || !got.Configured {
				t.Errorf("proxyCoreStatus() installed/configured = %v/%v, want true/true", got.Installed, got.Configured)
			}
			if got.BinaryPath != binaryPath {
				t.Errorf("proxyCoreStatus().BinaryPath = %q, want %q", got.BinaryPath, binaryPath)
			}
			if got.Source != "environment" {
				t.Errorf("proxyCoreStatus().Source = %q, want environment", got.Source)
			}
			if got.Message != test.wantMessage {
				t.Errorf("proxyCoreStatus().Message = %q, want %q", got.Message, test.wantMessage)
			}
		})
	}
}
