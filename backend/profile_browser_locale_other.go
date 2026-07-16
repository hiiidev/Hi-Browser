//go:build !darwin

package backend

func configurePlatformProfileLocale(profile *BrowserProfile, derivedBundle bool) error { return nil }
