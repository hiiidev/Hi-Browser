package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/browsercore"
	gort "runtime"
	"strconv"
	"strings"
)

type BrowserFingerprintCapabilities = browsercore.FingerprintCapabilities
type BrowserFingerprintArgResult = browsercore.FingerprintArgResult

func (a *App) BrowserCoreFingerprintCapabilities(coreID, targetPlatform string) BrowserFingerprintCapabilities {
	return a.browserFingerprintCapabilities(coreID, targetPlatform)
}

func (a *App) BrowserCoreNormalizeFingerprintArgs(coreID, targetPlatform string, args []string) BrowserFingerprintArgResult {
	capabilities := a.browserFingerprintCapabilities(coreID, targetPlatform)
	return browsercore.NormalizeFingerprintArgsForCapabilities(args, capabilities)
}

func (a *App) browserFingerprintCapabilities(coreID, targetPlatform string) browsercore.FingerprintCapabilities {
	provider := "manual"
	version := ""
	if core, ok := a.resolveFingerprintCore(coreID); ok {
		if value := strings.TrimSpace(core.Provider); value != "" {
			provider = value
		}
		version = strings.TrimSpace(core.BrowserVersion)
		if version == "" && core.ChromiumMajor > 0 {
			version = strconv.Itoa(core.ChromiumMajor)
		}
		if version == "" {
			version = strings.TrimSpace(a.browserMgr.GetChromeVersion(core.CorePath))
		}
	}
	if targetPlatform = browsercore.NormalizePlatform(targetPlatform); targetPlatform == "" {
		targetPlatform = browsercore.NormalizePlatform(gort.GOOS)
	}
	return browsercore.Capabilities(provider, version, gort.GOOS, targetPlatform)
}

func (a *App) resolveFingerprintCore(coreID string) (browser.Core, bool) {
	if a == nil || a.browserMgr == nil {
		return browser.Core{}, false
	}
	if normalized := strings.TrimSpace(coreID); normalized != "" && !strings.EqualFold(normalized, "default") {
		return a.browserMgr.GetCore(normalized)
	}
	return a.browserMgr.GetDefaultCore()
}

func (a *App) normalizeBrowserProfileFingerprintInput(input BrowserProfileInput) (BrowserProfileInput, browsercore.FingerprintArgResult) {
	targetPlatform := fingerprintTargetPlatform(input.FingerprintArgs)
	result := a.BrowserCoreNormalizeFingerprintArgs(input.CoreId, targetPlatform, input.FingerprintArgs)
	input.FingerprintArgs = append([]string{}, result.Args...)
	return input, result
}

func fingerprintTargetPlatform(args []string) string {
	for index := len(args) - 1; index >= 0; index-- {
		arg := strings.TrimSpace(args[index])
		if strings.HasPrefix(arg, "--fingerprint-platform=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--fingerprint-platform="))
		}
	}
	return ""
}
