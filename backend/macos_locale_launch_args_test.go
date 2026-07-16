package backend

import (
	"ant-chrome/backend/internal/browsercore"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSanitizeManagedLaunchArgsRemovesLegacyMacOSLocale(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantRemoved []string
	}{
		{
			name: "removes separate and inline values",
			args: []string{
				"--lang=en-US",
				"-AppleLanguages", "(en-US, en)",
				"-AppleLocale=ja_JP",
				"--disable-sync",
			},
			wantArgs:    []string{"--lang=en-US", "--disable-sync"},
			wantRemoved: []string{"-AppleLanguages", "-AppleLocale"},
		},
		{
			name:        "matches legacy keys case insensitively",
			args:        []string{"--foo", "bar", "-applelocale", "en_US", "--baz"},
			wantArgs:    []string{"--foo", "bar", "--baz"},
			wantRemoved: []string{"-AppleLocale"},
		},
		{
			name:        "does not consume another switch as a value",
			args:        []string{"-AppleLocale", "--disable-sync"},
			wantArgs:    []string{"--disable-sync"},
			wantRemoved: []string{"-AppleLocale"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotArgs, gotRemoved := sanitizeManagedLaunchArgs(test.args)
			if !reflect.DeepEqual(gotArgs, test.wantArgs) {
				t.Errorf("sanitizeManagedLaunchArgs(%v) args = %v, want %v", test.args, gotArgs, test.wantArgs)
			}
			if !reflect.DeepEqual(gotRemoved, test.wantRemoved) {
				t.Errorf("sanitizeManagedLaunchArgs(%v) removed = %v, want %v", test.args, gotRemoved, test.wantRemoved)
			}
		})
	}
}

func TestBuildBrowserLaunchArgsAlwaysIsolatesMacOSKeychain(t *testing.T) {
	capabilities := browsercore.Capabilities("fingerprint-chromium", "148.0.7778.215", "darwin", "macos")
	got := buildBrowserLaunchArgs(&BrowserProfile{}, capabilities, "/tmp/profile", 9222, "direct://", nil,
		[]string{"--use-mock-keychain", "--password-store=basic"}, []string{"--use-mock-keychain", "--password-store", "basic"}, nil)
	wantCount := 0
	if runtime.GOOS == "darwin" {
		wantCount = 1
	}
	count := 0
	for _, arg := range got {
		if arg == "--use-mock-keychain" {
			count++
		}
		if strings.HasPrefix(arg, "--password-store") || arg == "basic" {
			t.Fatalf("manual password-store override survived: %v", got)
		}
	}
	if count != wantCount {
		t.Fatalf("mock keychain arg count = %d, want %d: %v", count, wantCount, got)
	}
}

func TestBuildBrowserLaunchArgsUsesChromiumLocaleOnly(t *testing.T) {
	profile := &BrowserProfile{
		ProfileId: "profile-locale",
		FingerprintArgs: []string{
			"--fingerprint-platform=macos",
			"--lang=en-US",
			"--accept-lang=en-US,en",
			"-AppleLanguages",
			"(en-US, en)",
			"-AppleLocale=en_US",
		},
	}
	capabilities := browsercore.Capabilities("fingerprint-chromium", "148.0.7778.215", "darwin", "macos")

	got := buildBrowserLaunchArgs(profile, capabilities, "/tmp/profile-locale", 9222, "direct://", nil, nil, nil, []string{"about:blank"})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"--lang=en-US", "--accept-lang=en-US,en", "about:blank"} {
		if !strings.Contains(joined, want) {
			t.Errorf("buildBrowserLaunchArgs() = %q, want to contain %q", joined, want)
		}
	}
	for _, unwanted := range []string{"-AppleLanguages", "-AppleLocale", "(en-US, en)", "en_US"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("buildBrowserLaunchArgs() = %q, do not want %q", joined, unwanted)
		}
	}
}
