package browser

import (
	"ant-chrome/backend/internal/config"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestCopyWithModeRegularKeepsFingerprintArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultFingerprintArgs = []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"}
	mgr := NewManager(cfg, t.TempDir())

	source := &Profile{
		ProfileId:       "src-regular",
		ProfileName:     "源实例",
		UserDataDir:     "src-regular",
		CoreId:          "core-1",
		FingerprintArgs: []string{"--fingerprint=12345", "--fingerprint-brand=Edge", "--fingerprint-platform=linux"},
		ProxyId:         "proxy-1",
		ProxyConfig:     "socks5://127.0.0.1:1080",
		LaunchArgs:      []string{"--disable-sync"},
		Tags:            []string{"tag-1"},
		Keywords:        []string{"kw-1"},
	}
	mgr.Profiles[source.ProfileId] = source

	copied, err := mgr.CopyWithMode(source.ProfileId, "源实例-副本", copyModeRegular)
	if err != nil {
		t.Fatalf("CopyWithMode regular failed: %v", err)
	}

	if !reflect.DeepEqual(copied.FingerprintArgs, source.FingerprintArgs) {
		t.Fatalf("expected regular copy to preserve fingerprint args, got=%v want=%v", copied.FingerprintArgs, source.FingerprintArgs)
	}
}

func TestCopyWithModeAutoFingerprintRemovesSeedButKeepsTemplate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultFingerprintArgs = []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"}
	mgr := NewManager(cfg, t.TempDir())

	source := &Profile{
		ProfileId:       "src-auto",
		ProfileName:     "源实例",
		UserDataDir:     "src-auto",
		FingerprintArgs: []string{"--fingerprint=67890", "--fingerprint-brand=Edge", "--fingerprint-platform=linux", "--lang=en-US"},
	}
	mgr.Profiles[source.ProfileId] = source

	copied, err := mgr.CopyWithMode(source.ProfileId, "源实例-自动指纹", copyModeAutoFingerprint)
	if err != nil {
		t.Fatalf("CopyWithMode auto_fingerprint failed: %v", err)
	}

	if hasFingerprintSeedArg(copied.FingerprintArgs) {
		t.Fatalf("expected auto fingerprint copy to remove explicit seed, got=%v", copied.FingerprintArgs)
	}
	want := []string{"--fingerprint-brand=Edge", "--fingerprint-platform=linux", "--lang=en-US"}
	if !reflect.DeepEqual(copied.FingerprintArgs, want) {
		t.Fatalf("expected auto fingerprint copy to keep source template, got=%v want=%v", copied.FingerprintArgs, want)
	}
}

func TestCopyWithOptionsAutoFingerprintReplacesSelectedGroups(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultFingerprintArgs = []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=windows",
		"--lang=zh-CN",
		"--timezone=Asia/Shanghai",
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--fingerprint-do-not-track=false",
		"--fingerprint-media-devices=1,1,0",
		"--fingerprint-touch-points=0",
	}
	mgr := NewManager(cfg, t.TempDir())

	source := &Profile{
		ProfileId:   "src-auto-groups",
		ProfileName: "源实例",
		UserDataDir: "src-auto-groups",
		FingerprintArgs: []string{
			"--fingerprint=67890",
			"--fingerprint-brand=Edge",
			"--fingerprint-platform=linux",
			"--lang=en-US",
			"--timezone=America/New_York",
			"--window-size=1440,900",
			"--fingerprint-hardware-concurrency=16",
			"--fingerprint-device-memory=16",
			"--fingerprint-canvas-noise=true",
			"--fingerprint-fonts=Arial,Helvetica",
			"--webrtc-ip-handling-policy=default_public_interface_only",
			"--fingerprint-do-not-track=true",
			"--fingerprint-media-devices=2,1,0",
			"--fingerprint-touch-points=5",
		},
	}
	mgr.Profiles[source.ProfileId] = source

	copied, err := mgr.CopyWithOptions(source.ProfileId, "源实例-自动化指纹", ProfileCopyOptions{
		Mode: copyModeAutoFingerprint,
		AutomationTargets: []string{
			copyAutomationTargetSeed,
			copyAutomationTargetIdentity,
			copyAutomationTargetLocale,
			copyAutomationTargetNetwork,
			copyAutomationTargetDevices,
		},
	})
	if err != nil {
		t.Fatalf("CopyWithOptions auto_fingerprint failed: %v", err)
	}

	want := []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=windows",
		"--lang=zh-CN",
		"--timezone=Asia/Shanghai",
		"--window-size=1440,900",
		"--fingerprint-hardware-concurrency=16",
		"--fingerprint-device-memory=16",
		"--fingerprint-canvas-noise=true",
		"--fingerprint-fonts=Arial,Helvetica",
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--fingerprint-do-not-track=false",
		"--fingerprint-media-devices=1,1,0",
		"--fingerprint-touch-points=0",
	}
	if !reflect.DeepEqual(copied.FingerprintArgs, want) {
		t.Fatalf("expected auto fingerprint copy to replace selected groups, got=%v want=%v", copied.FingerprintArgs, want)
	}
}

func TestCopyKeepsLegacyDefaultFingerprintBehavior(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultFingerprintArgs = []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"}
	mgr := NewManager(cfg, t.TempDir())

	source := &Profile{
		ProfileId:       "src-legacy",
		ProfileName:     "源实例",
		UserDataDir:     "src-legacy",
		FingerprintArgs: []string{"--fingerprint=99999", "--fingerprint-brand=Edge", "--fingerprint-platform=linux"},
	}
	mgr.Profiles[source.ProfileId] = source

	copied, err := mgr.Copy(source.ProfileId, "源实例-旧复制")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if !reflect.DeepEqual(copied.FingerprintArgs, cfg.Browser.DefaultFingerprintArgs) {
		t.Fatalf("expected legacy copy to use default fingerprint args, got=%v want=%v", copied.FingerprintArgs, cfg.Browser.DefaultFingerprintArgs)
	}
}

func TestCopyBlankNameUsesTimestampedCopyName(t *testing.T) {
	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, t.TempDir())

	source := &Profile{
		ProfileId:   "src-copy-name",
		ProfileName: "邮箱测试 (副本)",
		UserDataDir: "src-copy-name",
	}
	mgr.Profiles[source.ProfileId] = source

	copied, err := mgr.Copy(source.ProfileId, "")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	matched := regexp.MustCompile(`^邮箱测试（副本）\d{12}$`).MatchString(copied.ProfileName)
	if !matched {
		t.Fatalf("expected timestamped copy name without duplicated suffix, got=%q", copied.ProfileName)
	}
}

func TestCopyWithOptionsRejectsUnknownAutomationTarget(t *testing.T) {
	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, t.TempDir())

	source := &Profile{
		ProfileId:       "src-invalid-target",
		ProfileName:     "源实例",
		UserDataDir:     "src-invalid-target",
		FingerprintArgs: []string{"--fingerprint=12345", "--fingerprint-brand=Chrome"},
	}
	mgr.Profiles[source.ProfileId] = source

	_, err := mgr.CopyWithOptions(source.ProfileId, "源实例-失败", ProfileCopyOptions{
		Mode:              copyModeAutoFingerprint,
		AutomationTargets: []string{"unknown_target"},
	})
	if err == nil {
		t.Fatal("expected CopyWithOptions to reject unknown automation target")
	}
}

func hasFingerprintSeedArg(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(arg)), "--fingerprint=") {
			return true
		}
	}
	return false
}
