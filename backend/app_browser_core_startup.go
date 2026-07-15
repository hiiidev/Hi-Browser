package backend

import (
	"ant-chrome/backend/internal/logger"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startBrowserCoreMaintenance() {
	if a == nil || a.browserMgr == nil {
		return
	}
	for _, dir := range []string{a.resolveAppPath("chrome/.staging")} {
		_ = os.RemoveAll(dir)
		_ = os.MkdirAll(dir, 0755)
	}
	cacheDir := a.resolveAppPath("download-cache/browser-core")
	_ = filepath.WalkDir(cacheDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr == nil && time.Since(info.ModTime()) > 7*24*time.Hour {
			_ = os.Remove(path)
		}
		return nil
	})
	if a.config.BrowserCore.AutoCheckUpdates != nil && !*a.config.BrowserCore.AutoCheckUpdates {
		return
	}
	if last, err := time.Parse(time.RFC3339, strings.TrimSpace(a.config.BrowserCore.LastUpdateCheckAt)); err == nil && time.Since(last) < 24*time.Hour {
		return
	}
	go func() {
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		select {
		case <-a.coreAPIContext().Done():
			return
		case <-timer.C:
		}
		releases, err := a.BrowserCoreAvailableReleases()
		if err != nil {
			logger.New("BrowserCore").Warn("后台检查内核更新失败", logger.F("error", err.Error()))
			return
		}
		a.config.BrowserCore.LastUpdateCheckAt = time.Now().Format(time.RFC3339)
		_ = a.config.Save(a.resolveAppPath("config.yaml"))
		if len(releases) == 0 || strings.EqualFold(releases[0].ReleaseTag, a.config.BrowserCore.SkippedVersion) {
			return
		}
		latest := releases[0]
		for _, core := range a.browserMgr.ListCores() {
			if core.ManagedByApp && compareVersions(core.ReleaseTag, latest.ReleaseTag) >= 0 {
				return
			}
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "browser-core:update-available", latest)
		}
	}()
}

func compareVersions(left, right string) int {
	a := versionParts(left)
	b := versionParts(right)
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}
func versionParts(value string) []int {
	value = strings.TrimLeft(strings.TrimSpace(value), "vV")
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '-' || r == '_' })
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}
