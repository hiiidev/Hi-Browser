package backend

func (a *App) applyProfileBrowserTaskbarIcon(profile *BrowserProfile, processID int) string {
	if profile == nil || profile.IconBadge == "" || processID <= 0 {
		return ""
	}
	if err := applyPlatformProfileBrowserTaskbarIcon(a.appStateRootAbs(), profile, processID); err != nil {
		return "实例任务栏角标应用失败，浏览器仍可正常使用：" + err.Error()
	}
	return ""
}
