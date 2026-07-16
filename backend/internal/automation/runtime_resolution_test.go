package automation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveSystemNodeFindsUserToolchainOutsidePATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a POSIX shell script")
	}
	home := t.TempDir()
	nodePath := filepath.Join(home, ".volta", "bin", "node")
	if err := os.MkdirAll(filepath.Dir(nodePath), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '{\"path\":\"%s\",\"version\":\"22.15.1\"}'\n", nodePath)
	if err := os.WriteFile(nodePath, []byte(payload), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	manager := NewManager(t.TempDir(), nil, nil, Options{TargetOS: "darwin", TargetArch: "arm64"})
	resolved, err := manager.resolveSystemNode(context.Background(), "")
	if err != nil {
		t.Fatalf("resolveSystemNode() returned error: %v", err)
	}
	if !resolved.SystemNodeDetected || resolved.Path == "" {
		t.Fatalf("resolveSystemNode() = %+v, want a detected common-location Node", resolved)
	}
	if strings.Contains(resolved.Resolution, "PATH") {
		t.Fatalf("resolveSystemNode() unexpectedly used PATH: %+v", resolved)
	}
}

func TestCommonSystemNodePathsIncludeHomebrew(t *testing.T) {
	paths := commonSystemNodePaths("darwin", "/Users/example")
	want := "/opt/homebrew/bin/node"
	for _, path := range paths {
		if path == want {
			return
		}
	}
	t.Fatalf("commonSystemNodePaths() = %v, want %q", paths, want)
}
