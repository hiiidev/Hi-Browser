package browsercore

import (
	"context"
	"time"
)

const FingerprintChromiumProvider = "fingerprint-chromium"

type ListOptions struct {
	Channel string
	Limit   int
	Version string
}

type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tagName"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"htmlUrl"`
	PublishedAt time.Time `json:"publishedAt"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Assets      []Asset   `json:"assets"`
}

type Asset struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Size                int64  `json:"size"`
	DownloadURL         string `json:"downloadUrl"`
	ContentType         string `json:"contentType"`
	Platform            string `json:"platform,omitempty"`
	Architecture        string `json:"architecture,omitempty"`
	PublisherSHA256     string `json:"publisherSha256,omitempty"`
	ChecksumAssetID     int64  `json:"checksumAssetId,omitempty"`
	ChecksumAssetName   string `json:"checksumAssetName,omitempty"`
	ChecksumDownloadURL string `json:"checksumDownloadUrl,omitempty"`
}

type ReleaseList struct {
	Releases []Release `json:"releases"`
	Stale    bool      `json:"stale"`
	Source   string    `json:"source"`
}

type DownloadInfo struct {
	Provider         string `json:"provider"`
	SourceRepository string `json:"sourceRepository"`
	ReleaseTag       string `json:"releaseTag"`
	ReleaseURL       string `json:"releaseUrl"`
	Asset            Asset  `json:"asset"`
}

type Provider interface {
	Name() string
	ListReleases(context.Context, ListOptions) (ReleaseList, error)
	GetLatestRelease(context.Context, string) (Release, bool, error)
	SelectCompatibleAsset(Release, string, string) (Asset, error)
	DownloadMetadata(Release, Asset) DownloadInfo
	ParseVersion(Release) string
	Capabilities(string, string, string) FingerprintCapabilities
	ResolvePublisherChecksum(context.Context, Release, Asset) (string, bool, error)
}
