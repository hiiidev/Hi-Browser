//go:build darwin

package backend

import (
	"ant-chrome/backend/internal/browser"
	"fmt"
	"strings"
)

func configurePlatformProfileLocale(profile *BrowserProfile, derivedBundle bool) error {
	if profile == nil || !derivedBundle {
		return nil
	}
	domain := browser.ProfileBundleIdentifier(profile.ProfileId)
	language := normalizeAppleLanguage(fingerprintLanguage(profile.FingerprintArgs))
	if language == "" {
		_ = runProfileIconCommand("defaults", "delete", domain, "AppleLanguages")
		_ = runProfileIconCommand("defaults", "delete", domain, "AppleLocale")
		return nil
	}

	primary := strings.SplitN(language, "-", 2)[0]
	languages := []string{language}
	if primary != "" && !strings.EqualFold(primary, language) {
		languages = append(languages, primary)
	}
	args := []string{"write", domain, "AppleLanguages", "-array"}
	args = append(args, languages...)
	if err := runProfileIconCommand("defaults", args...); err != nil {
		return fmt.Errorf("写入 AppleLanguages 失败: %w", err)
	}
	locale := strings.ReplaceAll(language, "-", "_")
	if err := runProfileIconCommand("defaults", "write", domain, "AppleLocale", "-string", locale); err != nil {
		return fmt.Errorf("写入 AppleLocale 失败: %w", err)
	}
	return nil
}

func normalizeAppleLanguage(value string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return ""
	}
	parts[0] = strings.ToLower(parts[0])
	if len(parts) >= 2 {
		if len(parts[1]) == 2 || len(parts[1]) == 3 {
			parts[1] = strings.ToUpper(parts[1])
		}
	}
	return strings.Join(parts, "-")
}
