//go:build !windows

package backend

func applyPlatformProfileBrowserTaskbarIcon(_ string, _ *BrowserProfile, _ int) error {
	return nil
}
