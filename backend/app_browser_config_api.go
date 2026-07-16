package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/browsercore"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	proxyinternal "ant-chrome/backend/internal/proxy"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	gort "runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type BrowserCoreReleaseInfo struct {
	Provider    string            `json:"provider"`
	Version     string            `json:"version"`
	ReleaseTag  string            `json:"releaseTag"`
	PublishedAt string            `json:"publishedAt"`
	ReleaseURL  string            `json:"releaseUrl"`
	Notes       string            `json:"notes"`
	Asset       browsercore.Asset `json:"asset"`
	Stale       bool              `json:"stale"`
}

type BrowserCorePreparationStatus struct {
	HasValidCore bool                    `json:"hasValidCore"`
	Platform     string                  `json:"platform"`
	Architecture string                  `json:"architecture"`
	Recommended  *BrowserCoreReleaseInfo `json:"recommended,omitempty"`
	Message      string                  `json:"message"`
}

func (a *App) GetBrowserSettings() BrowserSettings {
	return BrowserSettings{
		UserDataRoot:           a.config.Browser.UserDataRoot,
		DefaultFingerprintArgs: append([]string{}, a.config.Browser.DefaultFingerprintArgs...),
		DefaultLaunchArgs:      append([]string{}, a.config.Browser.DefaultLaunchArgs...),
		DefaultStartURLs:       append([]string{}, a.config.Browser.DefaultStartURLs...),
		LightStartEnabled:      browserLightStartEnabled(a.config),
		RestoreLastSession:     a.config.Browser.RestoreLastSession,
		StartReadyTimeoutMs:    browserStartReadyTimeoutMillis(a.config),
		StartStableWindowMs:    browserStartStableWindowMillis(a.config),
		DefaultConnectorType:   config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType),
	}
}

func (a *App) SaveBrowserSettings(settings BrowserSettings) error {
	log := logger.New("Browser")
	a.config.Browser.UserDataRoot = strings.TrimSpace(settings.UserDataRoot)
	a.config.Browser.DefaultFingerprintArgs = append([]string{}, settings.DefaultFingerprintArgs...)
	a.config.Browser.DefaultLaunchArgs = append([]string{}, settings.DefaultLaunchArgs...)
	if settings.DefaultStartURLs != nil {
		a.config.Browser.DefaultStartURLs = normalizeNonEmptyStrings(settings.DefaultStartURLs)
	} else if a.config.Browser.DefaultStartURLs == nil {
		a.config.Browser.DefaultStartURLs = config.DefaultBrowserStartURLs()
	}
	lightStartEnabled := settings.LightStartEnabled
	a.config.Browser.LightStartEnabled = &lightStartEnabled
	a.config.Browser.RestoreLastSession = settings.RestoreLastSession
	a.config.Browser.DefaultConnectorType = config.NormalizeBrowserConnectorType(settings.DefaultConnectorType)
	if settings.StartReadyTimeoutMs > 0 {
		a.config.Browser.StartReadyTimeoutMs = settings.StartReadyTimeoutMs
	} else if a.config.Browser.StartReadyTimeoutMs <= 0 {
		a.config.Browser.StartReadyTimeoutMs = browserStartReadyTimeoutMillis(nil)
	}
	if settings.StartStableWindowMs > 0 {
		a.config.Browser.StartStableWindowMs = settings.StartStableWindowMs
	} else if a.config.Browser.StartStableWindowMs <= 0 {
		a.config.Browser.StartStableWindowMs = browserStartStableWindowMillis(nil)
	}
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("浏览器配置保存失败", logger.F("error", err))
		return err
	}
	return nil
}

func (a *App) BrowserCoreList() []BrowserCore {
	return a.browserMgr.ListCores()
}

func (a *App) GetBrowserCoreSettings() config.BrowserCoreConfig { return a.config.BrowserCore }
func (a *App) SaveBrowserCoreSettings(settings config.BrowserCoreConfig) error {
	settings.Provider = browsercore.FingerprintChromiumStaticProvider
	if strings.TrimSpace(settings.Channel) == "" {
		settings.Channel = "stable"
	}
	settings.ManifestURL = strings.TrimSpace(settings.ManifestURL)
	if settings.ManifestURL == "" {
		settings.ManifestURL = config.DefaultConfig().BrowserCore.ManifestURL
	}
	if settings.KeepVersions < 1 {
		settings.KeepVersions = 1
	}
	if settings.KeepVersions > 10 {
		settings.KeepVersions = 10
	}
	a.config.BrowserCore = settings
	return a.config.Save(a.resolveAppPath("config.yaml"))
}

func (a *App) BrowserCoreSave(input BrowserCoreInput) error {
	return a.browserMgr.SaveCore(input)
}

func (a *App) BrowserCoreDelete(coreId string) error {
	return a.browserMgr.DeleteCore(coreId)
}

func (a *App) BrowserCoreSetDefault(coreId string) error {
	return a.browserMgr.SetDefaultCore(coreId)
}

func (a *App) BrowserCoreValidate(corePath string) BrowserCoreValidateResult {
	return a.browserMgr.ValidateCorePath(corePath)
}

func (a *App) BrowserCoreVerify(coreID string) BrowserCoreValidateResult {
	return a.browserMgr.VerifyCore(coreID)
}

func (a *App) BrowserCoreExtendedInfo() []BrowserCoreExtendedInfo {
	return a.browserMgr.GetCoresExtendedInfo()
}

// BrowserCoreScan 重新扫描 chrome 目录，自动注册新内核
func (a *App) BrowserCoreScan() []BrowserCore {
	a.autoDetectCores()
	return a.browserMgr.ListCores()
}

// BrowserCoreImportLocal 选择一个已解压内核目录或归档文件并注册。
func (a *App) BrowserCoreImportLocal() (*BrowserCore, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("app context is nil")
	}
	if a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is nil")
	}

	selectedPath, err := wailsruntime.OpenFileDialog(a.ctx, browserCoreImportDialogOptions(gort.GOOS))
	if err != nil {
		return nil, err
	}
	selectedPath = strings.TrimSpace(selectedPath)
	if selectedPath == "" {
		return nil, nil
	}

	absPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return nil, err
	}
	return a.importLocalBrowserCoreArchive(absPath)
}

func browserCoreImportDialogOptions(goos string) wailsruntime.OpenDialogOptions {
	options := wailsruntime.OpenDialogOptions{Title: "选择 Chrome 内核归档文件"}
	// Wails 2.12 on macOS converts every pattern to UTType. The catch-all *.* pattern
	// becomes an invalid type and may terminate the process inside NSOpenPanel.
	// Leaving Filters empty makes NSOpenPanel accept files safely; archive validation
	// still happens in importLocalBrowserCoreArchive after selection.
	if !strings.EqualFold(strings.TrimSpace(goos), "darwin") {
		options.Filters = []wailsruntime.FileFilter{
			{DisplayName: "Chrome 内核归档 (" + browser.SupportedCoreArchiveDescription() + ")", Pattern: browser.SupportedCoreArchivePattern()},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		}
	}
	return options
}

func (a *App) importLocalBrowserCoreArchive(archivePath string) (*BrowserCore, error) {
	archiveName := strings.TrimSpace(filepath.Base(archivePath))
	coreName := strings.TrimSpace(coreNameFromArchiveName(archiveName))
	if coreName == "" {
		coreName = "本地内核"
	}

	targetCorePath := filepath.Join("chrome", coreName)
	targetDir := a.browserMgr.ResolveRelativePath(targetCorePath)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("同名内核目录已存在：%s", targetCorePath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, err
	}
	tempExtractDir, err := os.MkdirTemp(parentDir, coreName+"_import_*")
	if err != nil {
		return nil, err
	}
	cleanupTempExtract := true
	defer func() {
		if cleanupTempExtract {
			_ = os.RemoveAll(tempExtractDir)
		}
	}()

	a.emitBrowserCoreImportProgress("extracting", 0, "开始解压本地内核包...")
	if err := browser.ExtractCoreArchiveAndStripRootForImport(archivePath, tempExtractDir, func(progress int, message string) {
		a.emitBrowserCoreImportProgress("extracting", progress, message)
	}); err != nil {
		a.emitBrowserCoreImportProgress("error", 0, "解压失败: "+err.Error())
		return nil, fmt.Errorf("解压失败: %w", err)
	}
	a.emitBrowserCoreImportProgress("validating", 90, "正在校验内核可执行文件...")
	if _, _, ok := browser.FindCoreExecutable(tempExtractDir); !ok {
		err := fmt.Errorf("所选归档不是当前平台可用的内核包：当前平台 %s，未找到浏览器可执行文件（候选：%s）", browser.CoreExecutablePlatform(), strings.Join(browser.CoreExecutableCandidates(), ", "))
		a.emitBrowserCoreImportProgress("error", 0, err.Error())
		return nil, err
	}
	a.emitBrowserCoreImportProgress("saving", 95, "正在保存内核配置...")
	if err := os.Rename(tempExtractDir, targetDir); err != nil {
		a.emitBrowserCoreImportProgress("error", 0, "保存内核目录失败: "+err.Error())
		return nil, err
	}
	cleanupTempExtract = false

	input := browser.CoreInput{
		CoreName:  coreName,
		CorePath:  targetCorePath,
		IsDefault: len(a.browserMgr.ListCores()) == 0,
	}
	if err := a.browserMgr.SaveCore(input); err != nil {
		a.emitBrowserCoreImportProgress("error", 0, "保存配置失败: "+err.Error())
		return nil, err
	}
	for _, saved := range a.browserMgr.ListCores() {
		if normalizeCorePathForCompare(saved.CorePath) == normalizeCorePathForCompare(targetCorePath) {
			a.emitBrowserCoreImportProgress("done", 100, "导入完成")
			return &saved, nil
		}
	}
	err = fmt.Errorf("本地内核已保存但未能读取结果")
	a.emitBrowserCoreImportProgress("error", 0, err.Error())
	return nil, err
}

func (a *App) emitBrowserCoreImportProgress(phase string, progress int, message string) {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "core-import:progress", map[string]interface{}{
		"phase":    phase,
		"progress": progress,
		"message":  message,
	})
}

func coreNameFromArchiveName(name string) string {
	name = strings.TrimSpace(name)
	for _, suffix := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tbz2", ".zip", ".tar"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return strings.TrimSpace(name[:len(name)-len(suffix)])
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// BrowserCoreImportLocalDirectory 选择一个已解压内核目录并直接注册，不下载、不复制文件。
func (a *App) BrowserCoreImportLocalDirectory() (*BrowserCore, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("app context is nil")
	}
	if a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is nil")
	}

	selectedDir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择已解压的 Chrome 内核目录",
	})
	if err != nil {
		return nil, err
	}
	selectedDir = strings.TrimSpace(selectedDir)
	if selectedDir == "" {
		return nil, nil
	}

	absDir, err := filepath.Abs(selectedDir)
	if err != nil {
		return nil, err
	}
	if _, _, ok := browser.FindCoreExecutable(absDir); !ok {
		return nil, fmt.Errorf("所选目录不是当前平台可用的内核目录：当前平台 %s，未找到浏览器可执行文件（候选：%s）", browser.CoreExecutablePlatform(), strings.Join(browser.CoreExecutableCandidates(), ", "))
	}

	corePath := a.relativeCorePathIfPossible(absDir)
	coreName := strings.TrimSpace(filepath.Base(absDir))
	if coreName == "" || coreName == "." || coreName == string(filepath.Separator) {
		coreName = "本地内核"
	}

	for _, existing := range a.browserMgr.ListCores() {
		if normalizeCorePathForCompare(existing.CorePath) == normalizeCorePathForCompare(corePath) {
			return &existing, nil
		}
	}

	input := browser.CoreInput{
		CoreName:  coreName,
		CorePath:  corePath,
		IsDefault: len(a.browserMgr.ListCores()) == 0,
	}
	if err := a.browserMgr.SaveCore(input); err != nil {
		return nil, err
	}

	for _, saved := range a.browserMgr.ListCores() {
		if normalizeCorePathForCompare(saved.CorePath) == normalizeCorePathForCompare(corePath) {
			return &saved, nil
		}
	}
	return nil, fmt.Errorf("本地内核已保存但未能读取结果")
}

func (a *App) relativeCorePathIfPossible(absDir string) string {
	for _, root := range []string{a.appRootAbs(), a.appStateRootAbs()} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootAbs, absDir)
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(filepath.ToSlash(rel), "../") && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return absDir
}

// BrowserCoreDownload 在线下载并自动解压配置内核
func (a *App) BrowserCoreDownload(coreName, url, proxyConfig string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	targetURL, client, err := a.prepareBrowserCoreDownload(url, proxyConfig)
	if err != nil {
		return err
	}
	fallbackURL, fallbackClient, err := a.prepareBrowserCoreDownloadFallback(url, proxyConfig)
	if err != nil {
		return err
	}
	_, err = a.browserMgr.StartDownloadTaskWithHTTPFallback(a.ctx, browser.CoreInput{CoreName: coreName}, targetURL, proxyConfig, false, client, fallbackURL, fallbackClient)
	return err
}

func (a *App) BrowserCoreAvailableReleases() ([]BrowserCoreReleaseInfo, error) {
	provider := a.browserCoreProvider()
	list, err := provider.ListReleases(a.coreAPIContext(), browsercore.ListOptions{Channel: a.config.BrowserCore.Channel, Limit: 10})
	if err != nil {
		return nil, err
	}
	result := make([]BrowserCoreReleaseInfo, 0, len(list.Releases))
	for _, release := range list.Releases {
		asset, selectErr := provider.SelectCompatibleAsset(release, gort.GOOS, gort.GOARCH)
		if selectErr != nil {
			continue
		}
		result = append(result, BrowserCoreReleaseInfo{Provider: provider.Name(), Version: provider.ParseVersion(release), ReleaseTag: release.TagName, PublishedAt: release.PublishedAt.Format(time.RFC3339), ReleaseURL: release.HTMLURL, Notes: releaseNotesSummary(release.Body), Asset: asset, Stale: list.Stale})
	}
	return result, nil
}

func (a *App) BrowserCoreInstallRelease(releaseTag, proxyConfig string) (string, error) {
	provider := a.browserCoreProvider()
	list, err := provider.ListReleases(a.coreAPIContext(), browsercore.ListOptions{Channel: a.config.BrowserCore.Channel, Limit: 10, Version: releaseTag})
	if err != nil {
		return "", err
	}
	if len(list.Releases) == 0 {
		return "", fmt.Errorf("未找到版本 %s", releaseTag)
	}
	release := list.Releases[0]
	asset, err := provider.SelectCompatibleAsset(release, gort.GOOS, gort.GOARCH)
	if err != nil {
		return "", err
	}
	version := provider.ParseVersion(release)
	caps := provider.Capabilities(version, gort.GOOS, gort.GOOS)
	capsJSON, _ := json.Marshal(caps)
	coreName := fmt.Sprintf("fingerprint-chromium-%s-%s-%s", version, gort.GOOS, gort.GOARCH)
	verificationStatus := "publisher-checksum-unavailable"
	if checksum, found, checksumErr := provider.ResolvePublisherChecksum(a.coreAPIContext(), release, asset); checksumErr != nil {
		return "", checksumErr
	} else if found {
		asset.PublisherSHA256 = checksum
		verificationStatus = "publisher-sha256"
	}
	metadata := BrowserCore{Provider: provider.Name(), SourceRepository: "adryfish/fingerprint-chromium", ReleaseTag: release.TagName, BrowserVersion: version, ChromiumMajor: browsercore.ChromiumMajor(version), AssetId: asset.ID, AssetName: asset.Name, Platform: gort.GOOS, Architecture: gort.GOARCH, ManagedByApp: true, ReleaseUrl: release.HTMLURL, CapabilitiesJson: string(capsJSON), VerificationStatus: verificationStatus, ArchiveSha256: asset.PublisherSHA256}
	targetURL, client, clientErr := a.prepareBrowserCoreDownload(asset.DownloadURL, proxyConfig)
	if clientErr != nil {
		return "", clientErr
	}
	fallbackURL, fallbackClient, fallbackErr := a.prepareBrowserCoreDownloadFallback(asset.DownloadURL, proxyConfig)
	if fallbackErr != nil {
		return "", fallbackErr
	}
	return a.browserMgr.StartDownloadTaskWithHTTPFallback(a.coreAPIContext(), browser.CoreInput{CoreName: coreName, CorePath: filepath.ToSlash(filepath.Join("chrome", coreName)), IsDefault: len(a.browserMgr.ListCores()) == 0, Metadata: &metadata}, targetURL, proxyConfig, false, client, fallbackURL, fallbackClient)
}

func (a *App) BrowserCoreDownloadTask(taskID string) (browser.DownloadTaskState, error) {
	state, ok := a.browserMgr.GetDownloadTask(taskID)
	if !ok {
		return state, fmt.Errorf("下载任务不存在")
	}
	return state, nil
}
func (a *App) BrowserCoreCancelDownload(taskID string) error {
	return a.browserMgr.CancelDownloadTask(taskID)
}
func (a *App) BrowserCoreRetryDownload(taskID string) (string, error) {
	return a.browserMgr.RetryDownloadTask(a.coreAPIContext(), taskID)
}

func (a *App) BrowserCorePreparation() BrowserCorePreparationStatus {
	status := BrowserCorePreparationStatus{Platform: gort.GOOS, Architecture: gort.GOARCH}
	for _, core := range a.browserMgr.ListCores() {
		if a.browserMgr.ValidateCorePath(core.CorePath).Valid {
			status.HasValidCore = true
			status.Message = "已检测到可用浏览器内核"
			return status
		}
	}
	status.Message = "未检测到可用内核，需要用户确认后安装"
	if releases, err := a.BrowserCoreAvailableReleases(); err == nil && len(releases) > 0 {
		status.Recommended = &releases[0]
	}
	return status
}

func (a *App) browserCoreProvider() browsercore.Provider {
	manifestURL := ""
	if a.config != nil {
		manifestURL = strings.TrimSpace(a.config.BrowserCore.ManifestURL)
	}
	return browsercore.NewFingerprintChromiumStaticProvider(manifestURL, a.resolveAppPath("data/cache/browser-core/static-manifest.json"), filepath.Join(a.appRootAbs(), "browser-core-manifest.json"))
}
func (a *App) coreAPIContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
func releaseNotesSummary(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	r := []rune(value)
	if len(r) > 280 {
		return string(r[:280]) + "..."
	}
	return value
}

// BrowserCoreRedownload 重新下载并替换指定内核目录
func (a *App) BrowserCoreRedownload(coreId, url, proxyConfig string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	core, ok := a.browserMgr.GetCore(coreId)
	if !ok {
		return fmt.Errorf("内核不存在")
	}
	targetURL, client, err := a.prepareBrowserCoreDownload(url, proxyConfig)
	if err != nil {
		return err
	}
	fallbackURL, fallbackClient, err := a.prepareBrowserCoreDownloadFallback(url, proxyConfig)
	if err != nil {
		return err
	}
	_, err = a.browserMgr.StartDownloadTaskWithHTTPFallback(a.ctx, browser.CoreInput{CoreId: core.CoreId, CoreName: core.CoreName, CorePath: core.CorePath, IsDefault: core.IsDefault}, targetURL, proxyConfig, true, client, fallbackURL, fallbackClient)
	return err
}

const browserCoreGitHubProxyPrefix = "https://gh-proxy.com/"

func (a *App) prepareBrowserCoreDownload(rawURL, proxyConfig string) (string, *http.Client, error) {
	targetURL := strings.TrimSpace(rawURL)
	clientProxyConfig := proxyConfig
	if strings.TrimSpace(proxyConfig) == "__gh_proxy__" {
		proxiedURL, err := browserCoreGitHubProxyURL(targetURL)
		if err != nil {
			return "", nil, err
		}
		targetURL = proxiedURL
		clientProxyConfig = "__direct__"
	}
	client, err := a.browserCoreDownloadClient(clientProxyConfig)
	if err != nil {
		return "", nil, err
	}
	return targetURL, client, nil
}

func browserCoreGitHubProxyURL(rawURL string) (string, error) {
	value := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("GitHub 加速地址解析失败: %w", err)
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", fmt.Errorf("GitHub 加速仅支持 https://github.com 下载地址")
	}
	return browserCoreGitHubProxyPrefix + value, nil
}

func (a *App) prepareBrowserCoreDownloadFallback(rawURL, proxyConfig string) (string, *http.Client, error) {
	switch strings.TrimSpace(proxyConfig) {
	case "", "__system__", "__direct__", "direct://":
	default:
		return "", nil, nil
	}
	proxiedURL, err := browserCoreGitHubProxyURL(rawURL)
	if err != nil {
		// Automatic fallback only applies to GitHub Release download URLs.
		return "", nil, nil
	}
	directClient, err := a.browserCoreDownloadClient("__direct__")
	if err != nil {
		return "", nil, err
	}
	return proxiedURL, directClient, nil
}

func (a *App) browserCoreDownloadClient(proxyConfig string) (*http.Client, error) {
	value := strings.TrimSpace(proxyConfig)
	if value == "__direct__" || value == "direct://" {
		return &http.Client{Transport: &http.Transport{}}, nil
	}
	if value == "" || value == "__system__" {
		return nil, nil
	}
	connector := config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
	return proxyinternal.BuildProxyHTTPClient(value, "", a.getLatestProxies(), a.xrayMgr, a.singboxMgr, a.clashMgr, connector, 0)
}
