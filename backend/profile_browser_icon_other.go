//go:build !darwin

package backend

func preparePlatformProfileBrowserIcon(_ string, _ *BrowserProfile, chromeBinaryPath string) (string, error) {
	return chromeBinaryPath, nil
}
