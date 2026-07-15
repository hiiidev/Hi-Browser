package browsercore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fingerprintRepo = "adryfish/fingerprint-chromium"

type githubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}
type githubRelease struct {
	ID          int64         `json:"id"`
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
	Prerelease  bool          `json:"prerelease"`
	Draft       bool          `json:"draft"`
	Assets      []githubAsset `json:"assets"`
}
type githubCache struct {
	ETag      string    `json:"etag"`
	UpdatedAt time.Time `json:"updatedAt"`
	Releases  []Release `json:"releases"`
}

type GitHubProvider struct {
	Client    *http.Client
	APIBase   string
	CachePath string
	TokenEnv  string
	mu        sync.Mutex
}

func NewFingerprintChromiumGitHubProvider(cachePath, tokenEnv string) *GitHubProvider {
	if tokenEnv == "" {
		tokenEnv = "GITHUB_TOKEN"
	}
	return &GitHubProvider{Client: &http.Client{Timeout: 20 * time.Second}, APIBase: "https://api.github.com", CachePath: cachePath, TokenEnv: tokenEnv}
}
func (p *GitHubProvider) Name() string { return FingerprintChromiumProvider }
func (p *GitHubProvider) ListReleases(ctx context.Context, options ListOptions) (ReleaseList, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cache := p.readCache()
	endpoint := strings.TrimRight(p.APIBase, "/") + "/repos/" + fingerprintRepo + "/releases?per_page=30"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ReleaseList{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Ant-Browser/browser-core-provider")
	if cache.ETag != "" {
		req.Header.Set("If-None-Match", cache.ETag)
	}
	if token := strings.TrimSpace(os.Getenv(p.TokenEnv)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, requestErr := client.Do(req)
	if requestErr != nil {
		return p.fromCache(cache, options, requestErr)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return p.filter(cache.Releases, options, false), nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		reset := resp.Header.Get("X-RateLimit-Reset")
		msg := "GitHub API 请求频率受限，请稍后重试或通过环境变量配置 GITHUB_TOKEN"
		if reset != "" {
			msg += "（重置时间戳 " + reset + "）"
		}
		return p.fromCache(cache, options, errors.New(msg))
	}
	if resp.StatusCode != http.StatusOK {
		return p.fromCache(cache, options, fmt.Errorf("GitHub API 返回 HTTP %d", resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return p.fromCache(cache, options, err)
	}
	var raw []githubRelease
	if err := json.Unmarshal(data, &raw); err != nil {
		return p.fromCache(cache, options, fmt.Errorf("解析 GitHub Release 数据失败: %w", err))
	}
	releases := make([]Release, 0, len(raw))
	for _, item := range raw {
		r := Release{ID: item.ID, TagName: item.TagName, Name: item.Name, Body: item.Body, HTMLURL: item.HTMLURL, PublishedAt: item.PublishedAt, Prerelease: item.Prerelease, Draft: item.Draft}
		for _, a := range item.Assets {
			r.Assets = append(r.Assets, Asset{ID: a.ID, Name: a.Name, Size: a.Size, DownloadURL: a.BrowserDownloadURL, ContentType: a.ContentType})
		}
		releases = append(releases, r)
	}
	cache = githubCache{ETag: resp.Header.Get("ETag"), UpdatedAt: time.Now(), Releases: releases}
	p.writeCache(cache)
	return p.filter(releases, options, false), nil
}
func (p *GitHubProvider) fromCache(cache githubCache, options ListOptions, cause error) (ReleaseList, error) {
	if len(cache.Releases) > 0 {
		result := p.filter(cache.Releases, options, true)
		result.Source = "cache"
		return result, nil
	}
	return ReleaseList{}, cause
}
func (p *GitHubProvider) filter(in []Release, options ListOptions, stale bool) ReleaseList {
	limit := options.Limit
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	out := make([]Release, 0, limit)
	for _, r := range in {
		if r.Draft || (strings.EqualFold(options.Channel, "stable") && r.Prerelease) {
			continue
		}
		if options.Version != "" && r.TagName != options.Version && strings.TrimLeft(r.TagName, "vV") != strings.TrimLeft(options.Version, "vV") {
			continue
		}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return ReleaseList{Releases: out, Stale: stale, Source: "github"}
}
func (p *GitHubProvider) GetLatestRelease(ctx context.Context, channel string) (Release, bool, error) {
	list, err := p.ListReleases(ctx, ListOptions{Channel: channel, Limit: 1})
	if err != nil || len(list.Releases) == 0 {
		return Release{}, list.Stale, err
	}
	return list.Releases[0], list.Stale, nil
}
func (p *GitHubProvider) SelectCompatibleAsset(r Release, os, arch string) (Asset, error) {
	return SelectCompatibleAsset(r, os, arch)
}
func (p *GitHubProvider) DownloadMetadata(r Release, a Asset) DownloadInfo {
	return DownloadInfo{Provider: p.Name(), SourceRepository: fingerprintRepo, ReleaseTag: r.TagName, ReleaseURL: r.HTMLURL, Asset: a}
}
func (p *GitHubProvider) ParseVersion(r Release) string {
	return strings.TrimLeft(strings.TrimSpace(r.TagName), "vV")
}
func (p *GitHubProvider) Capabilities(v, host, target string) FingerprintCapabilities {
	return Capabilities(p.Name(), v, host, target)
}
func (p *GitHubProvider) readCache() githubCache {
	var c githubCache
	if p.CachePath == "" {
		return c
	}
	b, err := os.ReadFile(p.CachePath)
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}
func (p *GitHubProvider) writeCache(c githubCache) {
	if p.CachePath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.CachePath), 0755)
	if b, err := json.Marshal(c); err == nil {
		_ = os.WriteFile(p.CachePath, b, 0600)
	}
}
