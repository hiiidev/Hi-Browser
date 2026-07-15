package browsercore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func testRelease(names ...string) Release {
	r := Release{TagName: "v144.0.1"}
	for i, name := range names {
		r.Assets = append(r.Assets, Asset{ID: int64(i + 1), Name: name, DownloadURL: "https://example.test/" + name})
	}
	return r
}

func TestSelectCompatibleAssetPlatforms(t *testing.T) {
	tests := []struct{ os, arch, want string }{
		{"windows", "amd64", "fingerprint-chromium-windows-x64.zip"}, {"darwin", "amd64", "fingerprint-chromium-macos-x64.tar.xz"}, {"darwin", "arm64", "fingerprint-chromium-macos-arm64.tar.xz"}, {"linux", "amd64", "fingerprint-chromium-linux-x86_64.tar.gz"}, {"linux", "arm64", "fingerprint-chromium-linux-aarch64.tar.gz"}}
	r := testRelease("fingerprint-chromium-windows-x64.zip", "fingerprint-chromium-macos-x64.tar.xz", "fingerprint-chromium-macos-arm64.tar.xz", "fingerprint-chromium-linux-x86_64.tar.gz", "fingerprint-chromium-linux-aarch64.tar.gz", "checksums.txt", "Source code.zip")
	for _, tt := range tests {
		t.Run(tt.os+"_"+tt.arch, func(t *testing.T) {
			got, err := SelectCompatibleAsset(r, tt.os, tt.arch)
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != tt.want {
				t.Fatalf("got %s want %s", got.Name, tt.want)
			}
		})
	}
}
func TestSelectCompatibleAssetMissing(t *testing.T) {
	_, err := SelectCompatibleAsset(testRelease("linux-x64.zip"), "darwin", "arm64")
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestSelectCompatibleAssetTie(t *testing.T) {
	_, err := SelectCompatibleAsset(testRelease("macos-arm64-a.zip", "macos-arm64-b.zip"), "darwin", "arm64")
	if err == nil || !strings.Contains(err.Error(), "多个同等匹配") {
		t.Fatalf("unexpected %v", err)
	}
}

func TestSelectCompatibleAssetGenericMacDMG(t *testing.T) {
	asset, err := SelectCompatibleAsset(testRelease("ungoogled-chromium_148.0.7778.215-1.1_macos.dmg"), "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(asset.Name, ".dmg") {
		t.Fatalf("unexpected %s", asset.Name)
	}
}

func TestGitHubProviderJSONAndETagCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept") == "" {
			t.Error("missing headers")
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(`[{"id":1,"tag_name":"v144.0.1","name":"stable","body":"notes","html_url":"https://github.test/release","published_at":"2026-07-01T00:00:00Z","prerelease":false,"draft":false,"assets":[{"id":9,"name":"fingerprint-chromium-macos-arm64.zip","size":123,"browser_download_url":"https://download.test/core.zip","content_type":"application/zip"}]}]`))
	}))
	defer server.Close()
	p := NewFingerprintChromiumGitHubProvider(filepath.Join(t.TempDir(), "cache.json"), "TEST_TOKEN")
	p.APIBase = server.URL
	p.Client = server.Client()
	first, err := p.ListReleases(context.Background(), ListOptions{Channel: "stable", Limit: 10})
	if err != nil || len(first.Releases) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := p.ListReleases(context.Background(), ListOptions{Channel: "stable", Limit: 10})
	if err != nil || len(second.Releases) != 1 || second.Stale {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}
func TestGitHubProviderRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "123")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	p := NewFingerprintChromiumGitHubProvider("", "")
	p.APIBase = server.URL
	p.Client = server.Client()
	_, err := p.ListReleases(context.Background(), ListOptions{})
	if err == nil || !strings.Contains(err.Error(), "频率受限") {
		t.Fatalf("unexpected %v", err)
	}
}

func TestNormalizeFingerprintArgs(t *testing.T) {
	result := NormalizeFingerprintArgs([]string{"--fingerprint-platform=mac", "--lang=en-US", "--fingerprint-gpu-vendor=AMD", "--fingerprint-gpu-renderer=X", "--lang=zh-CN"}, 144)
	joined := strings.Join(result.Args, " ")
	for _, want := range []string{"--fingerprint-platform=macos", "--lang=zh-CN", "--accept-lang=zh-CN,zh", "--disable-spoofing=gpu"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
	if strings.Contains(joined, "fingerprint-gpu") {
		t.Fatalf("legacy gpu remains: %s", joined)
	}
}
func TestParseSHA256Checksum(t *testing.T) {
	sum := strings.Repeat("a", 64)
	got, err := ParseSHA256Checksum(sum+"  core.zip\n", "core.zip")
	if err != nil || got != sum {
		t.Fatalf("got %s err=%v", got, err)
	}
	if _, err := ParseSHA256Checksum(strings.Repeat("b", 64)+"  other.zip\n"+strings.Repeat("c", 64)+"  third.zip\n", "core.zip"); err == nil {
		t.Fatal("expected mismatch")
	}
}
