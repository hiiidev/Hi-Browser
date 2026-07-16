//go:build darwin

package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/iconbadge"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const profileBrowserIconCacheFormatVersion = "2"

func preparePlatformProfileBrowserIcon(stateRoot string, profile *BrowserProfile, chromeBinaryPath string) (string, error) {
	sourceBundle, err := macOSAppBundleForExecutable(chromeBinaryPath)
	if err != nil {
		return "", err
	}
	sourceInfo, err := os.Stat(chromeBinaryPath)
	if err != nil {
		return "", fmt.Errorf("读取浏览器内核信息失败: %w", err)
	}
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join([]string{
		profileBrowserIconCacheFormatVersion,
		chromeBinaryPath,
		sourceInfo.ModTime().UTC().String(),
		fmt.Sprintf("%d", sourceInfo.Size()),
		profile.ProfileId,
		profile.IconBadge,
		profile.IconBadgeColor,
	}, "\x00"))))[:16]
	profileCacheRoot := filepath.Join(stateRoot, "data", "cache", "profile-browser-icons", profile.ProfileId)
	targetBundle := filepath.Join(profileCacheRoot, cacheKey, "Chromium.app")
	targetExecutable := filepath.Join(targetBundle, "Contents", "MacOS", filepath.Base(chromeBinaryPath))
	if info, statErr := os.Stat(targetExecutable); statErr == nil && !info.IsDir() {
		return targetExecutable, nil
	}

	buildRoot := filepath.Join(profileCacheRoot, ".building-"+uuid.NewString())
	defer os.RemoveAll(buildRoot)
	temporaryBundle := filepath.Join(buildRoot, "Chromium.app")
	if err := os.MkdirAll(buildRoot, 0o755); err != nil {
		return "", fmt.Errorf("创建角标缓存目录失败: %w", err)
	}
	if err := runProfileIconCommand("cp", "-cR", sourceBundle, temporaryBundle); err != nil {
		return "", fmt.Errorf("当前文件系统不支持浏览器包写时复制，已取消角标派生: %w", err)
	}

	sourceIcon, err := findMacOSBundleIcon(temporaryBundle)
	if err != nil {
		return "", err
	}
	basePNG := filepath.Join(buildRoot, "base.png")
	if err := runProfileIconCommand("sips", "-s", "format", "png", sourceIcon, "--out", basePNG); err != nil {
		return "", fmt.Errorf("转换浏览器图标失败: %w", err)
	}
	iconsetDir := filepath.Join(buildRoot, "profile.iconset")
	if err := iconbadge.WriteIconset(basePNG, iconsetDir, profile.IconBadge, profile.IconBadgeColor); err != nil {
		return "", fmt.Errorf("绘制实例角标失败: %w", err)
	}
	badgedICNS := filepath.Join(buildRoot, "profile.icns")
	if err := runProfileIconCommand("iconutil", "-c", "icns", iconsetDir, "-o", badgedICNS); err != nil {
		return "", fmt.Errorf("生成 macOS 图标失败: %w", err)
	}
	appIconPath := filepath.Join(temporaryBundle, "Contents", "Resources", "app.icns")
	if err := copyProfileIconFile(badgedICNS, appIconPath); err != nil {
		return "", err
	}

	plistPath := filepath.Join(temporaryBundle, "Contents", "Info.plist")
	bundleID := browser.ProfileBundleIdentifier(profile.ProfileId)
	if err := runProfileIconCommand("plutil", "-replace", "CFBundleIdentifier", "-string", bundleID, plistPath); err != nil {
		return "", fmt.Errorf("设置实例 Bundle ID 失败: %w", err)
	}
	if err := replaceOrInsertPlistString(plistPath, "CFBundleDisplayName", profile.ProfileName); err != nil {
		return "", fmt.Errorf("设置实例 App 名称失败: %w", err)
	}
	if err := replaceOrInsertPlistString(plistPath, "CFBundleIconFile", "app.icns"); err != nil {
		return "", fmt.Errorf("设置实例图标文件失败: %w", err)
	}
	// Chromium 148 declares AppIcon from Assets.car. Removing the asset-catalog
	// key makes macOS honor the per-profile app.icns written above.
	_ = runProfileIconCommand("plutil", "-remove", "CFBundleIconName", plistPath)
	_ = runProfileIconCommand("xattr", "-cr", temporaryBundle)
	if err := runProfileIconCommand("codesign", "--force", "--deep", "--sign", "-", "--timestamp=none", temporaryBundle); err != nil {
		return "", fmt.Errorf("签名实例浏览器包失败: %w", err)
	}
	if err := runProfileIconCommand("codesign", "--verify", "--deep", "--strict", temporaryBundle); err != nil {
		return "", fmt.Errorf("验证实例浏览器包签名失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetBundle), 0o755); err != nil {
		return "", fmt.Errorf("创建角标缓存目标目录失败: %w", err)
	}
	if err := os.Rename(temporaryBundle, targetBundle); err != nil {
		return "", fmt.Errorf("提交实例浏览器包失败: %w", err)
	}
	return targetExecutable, nil
}

func replaceOrInsertPlistString(plistPath, key, value string) error {
	if err := runProfileIconCommand("plutil", "-replace", key, "-string", value, plistPath); err == nil {
		return nil
	}
	return runProfileIconCommand("plutil", "-insert", key, "-string", value, plistPath)
}

func macOSAppBundleForExecutable(executablePath string) (string, error) {
	cleaned := filepath.Clean(executablePath)
	parts := strings.Split(cleaned, string(filepath.Separator))
	for index := len(parts) - 1; index >= 0; index-- {
		if strings.HasSuffix(parts[index], ".app") {
			bundle := strings.Join(parts[:index+1], string(filepath.Separator))
			if filepath.IsAbs(cleaned) {
				bundle = string(filepath.Separator) + strings.TrimPrefix(bundle, string(filepath.Separator))
			}
			return bundle, nil
		}
	}
	return "", fmt.Errorf("浏览器可执行文件不在 macOS App 包内")
}

func findMacOSBundleIcon(bundlePath string) (string, error) {
	icons, err := filepath.Glob(filepath.Join(bundlePath, "Contents", "Resources", "*.icns"))
	if err != nil || len(icons) == 0 {
		return "", fmt.Errorf("浏览器 App 包内未找到 ICNS 图标")
	}
	for _, iconPath := range icons {
		if strings.EqualFold(filepath.Base(iconPath), "app.icns") {
			return iconPath, nil
		}
	}
	return icons[0], nil
}

func copyProfileIconFile(sourcePath, destinationPath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取实例图标失败: %w", err)
	}
	if err := os.WriteFile(destinationPath, data, 0o644); err != nil {
		return fmt.Errorf("替换实例浏览器图标失败: %w", err)
	}
	return nil
}

func runProfileIconCommand(name string, args ...string) error {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, detail)
	}
	return nil
}
