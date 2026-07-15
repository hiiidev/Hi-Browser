package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRangeDownloadAndResume(t *testing.T) {
	payload := bytes.Repeat([]byte("range-data-"), 10000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := 0
		if value := r.Header.Get("Range"); value != "" {
			_, _ = fmt.Sscanf(value, "bytes=%d-", &start)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		}
		_, _ = w.Write(payload[start:])
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "core.part")
	if err := os.WriteFile(path, payload[:1234], 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := doConcurrentDownload(context.Background(), server.Client(), server.URL, f, func(string, int, string) {}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, payload) {
		t.Fatalf("download mismatch %d", len(got))
	}
}
func TestDownloadRangeFallback(t *testing.T) {
	payload := []byte("complete payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "core.part")
	_ = os.WriteFile(path, []byte("partial"), 0600)
	f, _ := os.OpenFile(path, os.O_RDWR, 0600)
	defer f.Close()
	if err := doConcurrentDownload(context.Background(), server.Client(), server.URL, f, func(string, int, string) {}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q", got)
	}
}
func TestDownloadCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 100; i++ {
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	path := filepath.Join(t.TempDir(), "part")
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	defer f.Close()
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	if err := doConcurrentDownload(ctx, server.Client(), server.URL, f, func(string, int, string) {}); err == nil {
		t.Fatal("expected cancellation")
	}
}
func TestSHA256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a")
	_ = os.WriteFile(path, []byte("abc"), 0600)
	got, size, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("abc")))
	if got != want || size != 3 {
		t.Fatalf("got %s %d", got, size)
	}
}
func TestVerifySHA256(t *testing.T) {
	if !verifySHA256("ABC", "abc") {
		t.Fatal("expected match")
	}
	if verifySHA256("abc", "def") {
		t.Fatal("expected mismatch")
	}
}
func TestZipSlipRejected(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	entry, _ := zw.Create("../escape")
	_, _ = entry.Write([]byte("bad"))
	_ = zw.Close()
	_ = f.Close()
	err := extractCoreArchiveAndStripRoot(archive, filepath.Join(t.TempDir(), "dest"), func(int, string) {})
	if err == nil || !strings.Contains(err.Error(), "非法文件路径") {
		t.Fatalf("unexpected %v", err)
	}
}
func TestZipBombFileCountRejected(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "many.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	for i := 0; i < maxCoreArchiveFiles+1; i++ {
		_, _ = zw.Create(fmt.Sprintf("f/%d", i))
	}
	_ = zw.Close()
	_ = f.Close()
	if err := extractCoreArchiveAndStripRoot(archive, filepath.Join(t.TempDir(), "dest"), func(int, string) {}); err == nil {
		t.Fatal("expected limit error")
	}
}
func TestReplaceCoreDirectoryAtomic(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "core")
	staging := filepath.Join(root, "staging")
	_ = os.MkdirAll(target, 0755)
	_ = os.WriteFile(filepath.Join(target, "old"), []byte("old"), 0600)
	_ = os.MkdirAll(staging, 0755)
	_ = os.WriteFile(filepath.Join(staging, "new"), []byte("new"), 0600)
	if err := replaceCoreDirectory(target, staging, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "new")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "old")); !os.IsNotExist(err) {
		t.Fatalf("old still exists")
	}
}

func TestPlatformExecutableDiscovery(t *testing.T) {
	tests := []struct{ goos, relative string }{{"windows", "chrome.exe"}, {"linux", "chrome"}, {"darwin", "Chromium.app/Contents/MacOS/Chromium"}}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(tt.relative))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("binary"), 0755); err != nil {
				t.Fatal(err)
			}
			got, _, ok := findCoreExecutableForPlatform(root, tt.goos)
			if !ok || got != path {
				t.Fatalf("got %q ok=%v", got, ok)
			}
		})
	}
}
