package browsercore

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type scoredAsset struct {
	asset Asset
	score int
}

func SelectCompatibleAsset(release Release, goos, goarch string) (Asset, error) {
	wantedOS := assetOSTokens(goos)
	wantedArch := assetArchTokens(goarch)
	var candidates []scoredAsset
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if excludedAsset(name) || !supportedArchive(name) {
			continue
		}
		osScore := tokenScore(name, wantedOS)
		archScore := tokenScore(name, wantedArch)
		if osScore == 0 || archScore == 0 {
			continue
		}
		score := osScore*100 + archScore*10 + archivePreference(name)
		asset.Platform = goos
		asset.Architecture = goarch
		candidates = append(candidates, scoredAsset{asset: asset, score: score})
	}
	if len(candidates) == 0 {
		return Asset{}, fmt.Errorf("版本 %s 没有适用于 %s/%s 的二进制资产", release.TagName, goos, goarch)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > 1 && candidates[0].score == candidates[1].score {
		return Asset{}, fmt.Errorf("版本 %s 存在多个同等匹配的 %s/%s 资产：%s、%s", release.TagName, goos, goarch, candidates[0].asset.Name, candidates[1].asset.Name)
	}
	return candidates[0].asset, nil
}

func excludedAsset(name string) bool {
	return strings.Contains(name, "source code") || strings.Contains(name, "source-code") || strings.Contains(name, "sources") || strings.Contains(name, "checksum") || strings.Contains(name, "checksums") || strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sha256sum") || strings.HasSuffix(name, ".txt")
}

func supportedArchive(name string) bool {
	for _, suffix := range []string{".zip", ".tar.gz", ".tgz", ".tar.xz", ".txz"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func archivePreference(name string) int {
	switch {
	case strings.HasSuffix(name, ".zip"):
		return 6
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return 5
	case strings.HasSuffix(name, ".tar.xz"), strings.HasSuffix(name, ".txz"):
		return 4
	default:
		return 0
	}
}

func assetOSTokens(goos string) []string {
	switch goos {
	case "darwin":
		return []string{"macos", "darwin", "osx", "mac"}
	case "windows":
		return []string{"windows", "win"}
	default:
		return []string{goos}
	}
}
func assetArchTokens(arch string) []string {
	switch arch {
	case "amd64":
		return []string{"amd64", "x86_64", "x64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	default:
		return []string{arch}
	}
}
func tokenScore(name string, tokens []string) int {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	for i, token := range tokens {
		for _, separator := range []string{"-", "_", ".", " "} {
			base = strings.ReplaceAll(base, separator, " ")
		}
		for _, part := range strings.Fields(base) {
			if part == token {
				return len(tokens) - i + 2
			}
		}
		if strings.Contains(name, token) {
			return len(tokens) - i
		}
	}
	return 0
}
