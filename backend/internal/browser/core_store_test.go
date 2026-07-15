package browser

import (
	"ant-chrome/backend/internal/config"
	"strings"
	"testing"
)

func TestDeleteCoreBlockedWhenProfileReferencesIt(t *testing.T) {
	m := NewManager(&config.Config{Browser: config.BrowserConfig{Cores: []config.BrowserCore{{CoreId: "core-1", CoreName: "Core", CorePath: "chrome/core"}}}}, t.TempDir())
	m.Profiles["profile-1"] = &Profile{ProfileId: "profile-1", CoreId: "core-1"}
	err := m.DeleteCore("core-1")
	if err == nil || !strings.Contains(err.Error(), "Profile 引用") {
		t.Fatalf("unexpected %v", err)
	}
}
