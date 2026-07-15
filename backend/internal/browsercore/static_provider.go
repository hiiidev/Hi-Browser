package browsercore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const FingerprintChromiumStaticProvider = "fingerprint-chromium-static"

type StaticManifest struct {
	SchemaVersion    int       `json:"schemaVersion"`
	GeneratedAt      time.Time `json:"generatedAt"`
	Provider         string    `json:"provider"`
	SourceRepository string    `json:"sourceRepository"`
	Releases         []Release `json:"releases"`
}

type staticManifestCache struct {
	ETag      string         `json:"etag"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Manifest  StaticManifest `json:"manifest"`
}

type StaticManifestProvider struct {
	Client      *http.Client
	ManifestURL string
	CachePath   string
	BundledPath string
	mu          sync.Mutex
}

func NewFingerprintChromiumStaticProvider(manifestURL, cachePath, bundledPath string) *StaticManifestProvider {
	return &StaticManifestProvider{Client: &http.Client{Timeout: 20 * time.Second}, ManifestURL: strings.TrimSpace(manifestURL), CachePath: cachePath, BundledPath: bundledPath}
}

func (p *StaticManifestProvider) Name() string { return FingerprintChromiumStaticProvider }

func (p *StaticManifestProvider) ListReleases(ctx context.Context, options ListOptions) (ReleaseList, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cache := p.readCache()
	manifest, etag, err := p.fetchRemote(ctx, cache.ETag)
	if err == nil {
		if manifest.SchemaVersion == 0 {
			manifest = cache.Manifest
		}
		if len(manifest.Releases) > 0 {
			p.writeCache(staticManifestCache{ETag: firstNonEmpty(etag, cache.ETag), UpdatedAt: time.Now(), Manifest: manifest})
			return filterManifestReleases(manifest.Releases, options, false, "static-manifest"), nil
		}
	}
	if len(cache.Manifest.Releases) > 0 {
		return filterManifestReleases(cache.Manifest.Releases, options, true, "cache"), nil
	}
	bundled, bundledErr := readStaticManifestFile(p.BundledPath)
	if bundledErr == nil && len(bundled.Releases) > 0 {
		return filterManifestReleases(bundled.Releases, options, true, "bundled"), nil
	}
	if err != nil {
		return ReleaseList{}, err
	}
	if bundledErr != nil && !errors.Is(bundledErr, os.ErrNotExist) {
		return ReleaseList{}, bundledErr
	}
	return ReleaseList{Releases: []Release{}, Stale: true, Source: "bundled"}, nil
}

func (p *StaticManifestProvider) fetchRemote(ctx context.Context, etag string) (StaticManifest, string, error) {
	if p.ManifestURL == "" {
		return StaticManifest{}, "", fmt.Errorf("未配置浏览器内核 Manifest 地址")
	}
	u, err := url.Parse(p.ManifestURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") {
		return StaticManifest{}, "", fmt.Errorf("浏览器内核 Manifest 仅允许 HTTPS 地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.ManifestURL, nil)
	if err != nil {
		return StaticManifest{}, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Ant-Browser/browser-core-static-provider")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	secure := *client
	secure.CheckRedirect = secureManifestRedirect
	resp, err := secure.Do(req)
	if err != nil {
		return StaticManifest{}, "", fmt.Errorf("读取浏览器内核 Manifest 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return StaticManifest{}, etag, nil
	}
	if resp.StatusCode != http.StatusOK {
		return StaticManifest{}, "", fmt.Errorf("浏览器内核 Manifest 返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return StaticManifest{}, "", err
	}
	manifest, err := parseStaticManifest(data)
	if err != nil {
		return StaticManifest{}, "", err
	}
	return manifest, resp.Header.Get("ETag"), nil
}

func (p *StaticManifestProvider) GetLatestRelease(ctx context.Context, channel string) (Release, bool, error) {
	list, err := p.ListReleases(ctx, ListOptions{Channel: channel, Limit: 1})
	if err != nil || len(list.Releases) == 0 {
		return Release{}, list.Stale, err
	}
	return list.Releases[0], list.Stale, nil
}
func (p *StaticManifestProvider) SelectCompatibleAsset(release Release, goos, goarch string) (Asset, error) {
	return SelectCompatibleAsset(release, goos, goarch)
}
func (p *StaticManifestProvider) DownloadMetadata(release Release, asset Asset) DownloadInfo {
	return DownloadInfo{Provider: p.Name(), SourceRepository: "adryfish/fingerprint-chromium", ReleaseTag: release.TagName, ReleaseURL: release.HTMLURL, Asset: asset}
}
func (p *StaticManifestProvider) ParseVersion(release Release) string {
	return strings.TrimLeft(strings.TrimSpace(release.TagName), "vV")
}
func (p *StaticManifestProvider) Capabilities(version, host, target string) FingerprintCapabilities {
	return Capabilities(p.Name(), version, host, target)
}
func (p *StaticManifestProvider) ResolvePublisherChecksum(_ context.Context, _ Release, asset Asset) (string, bool, error) {
	sum := strings.TrimSpace(asset.PublisherSHA256)
	return sum, sum != "", nil
}

func parseStaticManifest(data []byte) (StaticManifest, error) {
	var manifest StaticManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("解析浏览器内核 Manifest 失败: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return manifest, fmt.Errorf("不支持的浏览器内核 Manifest 版本: %d", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.Provider) == "" {
		return manifest, fmt.Errorf("浏览器内核 Manifest 缺少 provider")
	}
	for _, release := range manifest.Releases {
		if strings.TrimSpace(release.TagName) == "" {
			return manifest, fmt.Errorf("浏览器内核 Manifest 包含空版本标签")
		}
		for _, asset := range release.Assets {
			u, err := url.Parse(asset.DownloadURL)
			if err != nil || !strings.EqualFold(u.Scheme, "https") {
				return manifest, fmt.Errorf("资产 %s 的下载地址不是 HTTPS", asset.Name)
			}
			if checksum := strings.TrimSpace(asset.PublisherSHA256); checksum != "" && !sha256Pattern.MatchString(checksum) {
				return manifest, fmt.Errorf("资产 %s 的 SHA-256 格式无效", asset.Name)
			}
		}
	}
	return manifest, nil
}
func readStaticManifestFile(path string) (StaticManifest, error) {
	if strings.TrimSpace(path) == "" {
		return StaticManifest{}, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StaticManifest{}, err
	}
	return parseStaticManifest(data)
}
func filterManifestReleases(in []Release, options ListOptions, stale bool, source string) ReleaseList {
	limit := options.Limit
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	out := make([]Release, 0, limit)
	for _, release := range in {
		if release.Draft || (strings.EqualFold(options.Channel, "stable") && release.Prerelease) {
			continue
		}
		if options.Version != "" && release.TagName != options.Version && strings.TrimLeft(release.TagName, "vV") != strings.TrimLeft(options.Version, "vV") {
			continue
		}
		out = append(out, release)
		if len(out) >= limit {
			break
		}
	}
	return ReleaseList{Releases: out, Stale: stale, Source: source}
}
func (p *StaticManifestProvider) readCache() staticManifestCache {
	var cache staticManifestCache
	if p.CachePath == "" {
		return cache
	}
	data, err := os.ReadFile(p.CachePath)
	if err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	return cache
}
func (p *StaticManifestProvider) writeCache(cache staticManifestCache) {
	if p.CachePath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.CachePath), 0755)
	if data, err := json.Marshal(cache); err == nil {
		_ = os.WriteFile(p.CachePath, data, 0600)
	}
}
func secureManifestRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("Manifest 重定向次数超过 5 次")
	}
	if !strings.EqualFold(req.URL.Scheme, "https") {
		return fmt.Errorf("Manifest 重定向到非 HTTPS 地址，已拒绝")
	}
	req.Header.Del("Authorization")
	return nil
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
