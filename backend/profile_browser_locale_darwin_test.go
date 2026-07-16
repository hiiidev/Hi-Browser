//go:build darwin

package backend

import (
	"ant-chrome/backend/internal/browser"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestConfigurePlatformProfileLocaleUsesIsolatedPreferences(t *testing.T) {
	profiles := []*BrowserProfile{
		{ProfileId: "locale-" + uuid.NewString(), FingerprintArgs: []string{"--lang=en-US", "--timezone=America/New_York"}},
		{ProfileId: "locale-" + uuid.NewString(), FingerprintArgs: []string{"--lang=ja-JP", "--timezone=Asia/Tokyo"}},
	}
	for _, profile := range profiles {
		domain := browser.ProfileBundleIdentifier(profile.ProfileId)
		t.Cleanup(func() { _, _ = exec.Command("defaults", "delete", domain).CombinedOutput() })
		if err := configurePlatformProfileLocale(profile, true); err != nil {
			t.Fatal(err)
		}
	}

	assertPreferenceContains(t, browser.ProfileBundleIdentifier(profiles[0].ProfileId), "AppleLanguages", "en-US", "en")
	assertPreferenceContains(t, browser.ProfileBundleIdentifier(profiles[0].ProfileId), "AppleLocale", "en_US")
	assertPreferenceContains(t, browser.ProfileBundleIdentifier(profiles[1].ProfileId), "AppleLanguages", "ja-JP", "ja")
	assertPreferenceContains(t, browser.ProfileBundleIdentifier(profiles[1].ProfileId), "AppleLocale", "ja_JP")

	profiles[0].FingerprintArgs = []string{"--timezone=UTC"}
	if err := configurePlatformProfileLocale(profiles[0], true); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("defaults", "read", browser.ProfileBundleIdentifier(profiles[0].ProfileId), "AppleLanguages").CombinedOutput(); err == nil {
		t.Fatalf("AppleLanguages still exists after language was cleared: %s", output)
	}
}

func TestConfigurePlatformProfileLocaleSkipsOriginalBundle(t *testing.T) {
	profile := &BrowserProfile{ProfileId: "locale-" + uuid.NewString(), FingerprintArgs: []string{"--lang=en-US"}}
	domain := browser.ProfileBundleIdentifier(profile.ProfileId)
	t.Cleanup(func() { _, _ = exec.Command("defaults", "delete", domain).CombinedOutput() })
	if err := configurePlatformProfileLocale(profile, false); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("defaults", "read", domain).CombinedOutput(); err == nil {
		t.Fatalf("preferences unexpectedly created for original bundle: %s", output)
	}
}

func assertPreferenceContains(t *testing.T, domain, key string, values ...string) {
	t.Helper()
	output, err := exec.Command("defaults", "read", domain, key).CombinedOutput()
	if err != nil {
		t.Fatalf("read %s %s: %v: %s", domain, key, err, output)
	}
	for _, value := range values {
		if !strings.Contains(string(output), value) {
			t.Errorf("%s %s = %s, want %q", domain, key, output, value)
		}
	}
}
