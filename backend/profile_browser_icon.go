package backend

import "strings"

func (a *App) prepareProfileBrowserIcon(profile *BrowserProfile, chromeBinaryPath string) (string, string) {
	if profile == nil || strings.TrimSpace(profile.IconBadge) == "" {
		return chromeBinaryPath, ""
	}
	path, err := preparePlatformProfileBrowserIcon(a.appStateRootAbs(), profile, chromeBinaryPath)
	if err != nil {
		return chromeBinaryPath, "实例系统图标角标生成失败，已使用原始浏览器图标：" + err.Error()
	}
	return path, ""
}
