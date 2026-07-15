package browsercore

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var sha256Pattern = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)

func (p *GitHubProvider) ResolvePublisherChecksum(ctx context.Context, release Release, asset Asset) (string, bool, error) {
	checksumAsset, ok := findChecksumAsset(release.Assets, asset.Name)
	if !ok {
		return "", false, nil
	}
	u, err := url.Parse(checksumAsset.DownloadURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") {
		return "", false, fmt.Errorf("校验文件下载地址不是有效 HTTPS")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumAsset.DownloadURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "Ant-Browser/browser-core-provider")
	client := p.Client
	if client == nil {
		client = &http.Client{}
	}
	secureClient := *client
	secureClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("校验文件重定向次数超过 5 次")
		}
		if !strings.EqualFold(req.URL.Scheme, "https") {
			return fmt.Errorf("校验文件重定向到非 HTTPS 地址，已拒绝")
		}
		req.Header.Del("Authorization")
		return nil
	}
	resp, err := secureClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("下载校验文件返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", false, err
	}
	sum, err := ParseSHA256Checksum(string(data), asset.Name)
	if err != nil {
		return "", false, err
	}
	return sum, true, nil
}

func findChecksumAsset(assets []Asset, target string) (Asset, bool) {
	lower := strings.ToLower(target)
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if name == lower+".sha256" || name == lower+".sha256sum" {
			return asset, true
		}
	}
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "checksum") || strings.Contains(name, "sha256sum") {
			return asset, true
		}
	}
	return Asset{}, false
}

func ParseSHA256Checksum(content, targetName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	base := filepath.Base(targetName)
	var single string
	validLines := 0
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) == 0 {
			continue
		}
		candidate := strings.TrimSpace(fields[0])
		if !sha256Pattern.MatchString(candidate) {
			continue
		}
		validLines++
		if single == "" {
			single = strings.ToLower(candidate)
		}
		if len(fields) > 1 && strings.TrimLeft(filepath.Base(fields[len(fields)-1]), "*") == base {
			return strings.ToLower(candidate), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if validLines == 1 && single != "" {
		return single, nil
	}
	return "", fmt.Errorf("校验文件中未找到资产 %s 的 SHA-256", base)
}
