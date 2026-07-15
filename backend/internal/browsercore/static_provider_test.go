package browsercore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

const staticManifestFixture = `{
  "schemaVersion": 1,
  "generatedAt": "2026-07-15T00:00:00Z",
  "provider": "fingerprint-chromium-static",
  "sourceRepository": "adryfish/fingerprint-chromium",
  "releases": [{
    "id": 1,
    "tagName": "v144.0.1",
    "name": "stable",
    "htmlUrl": "https://github.com/adryfish/fingerprint-chromium/releases/tag/v144.0.1",
    "publishedAt": "2026-07-15T00:00:00Z",
    "prerelease": false,
    "draft": false,
    "assets": [{
      "id": 2,
      "name": "fingerprint-chromium-macos-arm64.zip",
      "size": 123,
      "downloadUrl": "https://example.test/core.zip",
      "contentType": "application/zip",
      "publisherSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }]
  }]
}`

func TestStaticManifestProviderETagAndCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("If-None-Match") == `"manifest-v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"manifest-v1"`)
		_, _ = w.Write([]byte(staticManifestFixture))
	}))
	defer server.Close()
	p := NewFingerprintChromiumStaticProvider(server.URL, filepath.Join(t.TempDir(), "cache.json"), "")
	p.Client = server.Client()
	first, err := p.ListReleases(context.Background(), ListOptions{Channel: "stable", Limit: 10})
	if err != nil || len(first.Releases) != 1 || first.Stale {
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

func TestStaticManifestProviderBundledFallback(t *testing.T) {
	bundled := filepath.Join(t.TempDir(), "browser-core-manifest.json")
	if err := os.WriteFile(bundled, []byte(staticManifestFixture), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	p := NewFingerprintChromiumStaticProvider(server.URL, "", bundled)
	p.Client = server.Client()
	list, err := p.ListReleases(context.Background(), ListOptions{Channel: "stable", Limit: 10})
	if err != nil || len(list.Releases) != 1 || !list.Stale || list.Source != "bundled" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
}

func TestStaticManifestChecksum(t *testing.T) {
	manifest, err := parseStaticManifest([]byte(staticManifestFixture))
	if err != nil {
		t.Fatal(err)
	}
	p := NewFingerprintChromiumStaticProvider("", "", "")
	sum, found, err := p.ResolvePublisherChecksum(context.Background(), manifest.Releases[0], manifest.Releases[0].Assets[0])
	if err != nil || !found || len(sum) != 64 {
		t.Fatalf("sum=%q found=%v err=%v", sum, found, err)
	}
}
