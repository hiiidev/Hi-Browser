package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserStateRootForOSUsesHiBrowserDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	tests := []struct {
		name string
		goos string
		want string
	}{
		{
			name: "macOS",
			goos: "darwin",
			want: filepath.Join(home, "Library", "Application Support", "hi-browser"),
		},
		{
			name: "Linux",
			goos: "linux",
			want: filepath.Join(home, ".local", "share", "hi-browser"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := userStateRootForOS(test.goos, "fallback"); got != test.want {
				t.Errorf("userStateRootForOS(%q, fallback) = %q, want %q", test.goos, got, test.want)
			}
		})
	}
}

func TestEnsureWritableLayoutMigratesLegacyStateRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacyRoot := filepath.Join(home, "Library", "Application Support", legacyAppStateDirName)
	markerPath := filepath.Join(legacyRoot, "data", "app.db")
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", filepath.Dir(markerPath), err)
	}
	if err := os.WriteFile(markerPath, []byte("legacy-data"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", markerPath, err)
	}

	appRoot := filepath.Join(t.TempDir(), "Hi Browser.app", "Contents", "MacOS")
	if err := ensureWritableLayoutForOS(appRoot, "darwin"); err != nil {
		t.Fatalf("ensureWritableLayoutForOS(%q, darwin): %v", appRoot, err)
	}

	stateRoot := filepath.Join(home, "Library", "Application Support", appStateDirName)
	got, err := os.ReadFile(filepath.Join(stateRoot, "data", "app.db"))
	if err != nil {
		t.Fatalf("os.ReadFile(migrated app.db): %v", err)
	}
	if want := "legacy-data"; string(got) != want {
		t.Errorf("migrated app.db = %q, want %q", got, want)
	}
	if _, err := os.Stat(legacyRoot); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q) error = %v, want not exist", legacyRoot, err)
	}
}

func TestEnsureWritableLayoutDoesNotOverwriteExistingStateRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateParent := filepath.Join(home, "Library", "Application Support")
	legacyMarker := filepath.Join(stateParent, legacyAppStateDirName, "data", "legacy.db")
	currentMarker := filepath.Join(stateParent, appStateDirName, "data", "current.db")
	for path, content := range map[string]string{
		legacyMarker:  "legacy-data",
		currentMarker: "current-data",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", path, err)
		}
	}

	appRoot := filepath.Join(t.TempDir(), "Hi Browser.app", "Contents", "MacOS")
	if err := ensureWritableLayoutForOS(appRoot, "darwin"); err != nil {
		t.Fatalf("ensureWritableLayoutForOS(%q, darwin): %v", appRoot, err)
	}

	for path, want := range map[string]string{
		legacyMarker:  "legacy-data",
		currentMarker: "current-data",
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("os.ReadFile(%q): %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("os.ReadFile(%q) = %q, want %q", path, got, want)
		}
	}
}
