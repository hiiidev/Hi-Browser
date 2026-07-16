package browser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	gort "runtime"
	"strings"
	"syscall"
	"time"

	"ant-chrome/backend/internal/browsercore"
	"ant-chrome/backend/internal/logger"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DownloadAndExtractCore 执行异步下载解压并在过程中发送事件
func (m *Manager) DownloadAndExtractCore(ctx context.Context, coreName string, targetUrl string, proxyConfig string) {
	coreName = strings.TrimSpace(coreName)
	for _, core := range m.ListCores() {
		if strings.EqualFold(core.CoreName, coreName) || filepath.Base(core.CorePath) == coreName {
			m.startDownloadTask(ctx, CoreInput{
				CoreId:    core.CoreId,
				CoreName:  core.CoreName,
				CorePath:  core.CorePath,
				IsDefault: core.IsDefault,
			}, targetUrl, proxyConfig, false, "", nil, "", nil)
			return
		}
	}
	m.startDownloadTask(ctx, CoreInput{CoreName: coreName}, targetUrl, proxyConfig, false, "", nil, "", nil)
}

func (m *Manager) StartDownloadTask(ctx context.Context, coreInput CoreInput, targetURL, proxyConfig string, replaceExisting bool) (string, error) {
	return m.startDownloadTask(ctx, coreInput, targetURL, proxyConfig, replaceExisting, "", nil, "", nil)
}

func (m *Manager) StartDownloadTaskWithHTTPClient(ctx context.Context, coreInput CoreInput, targetURL, proxyConfig string, replaceExisting bool, client *http.Client) (string, error) {
	return m.startDownloadTask(ctx, coreInput, targetURL, proxyConfig, replaceExisting, "", client, "", nil)
}

func (m *Manager) StartDownloadTaskWithHTTPFallback(ctx context.Context, coreInput CoreInput, targetURL, proxyConfig string, replaceExisting bool, client *http.Client, fallbackURL string, fallbackClient *http.Client) (string, error) {
	return m.startDownloadTask(ctx, coreInput, targetURL, proxyConfig, replaceExisting, "", client, fallbackURL, fallbackClient)
}

func (m *Manager) startDownloadTask(parent context.Context, coreInput CoreInput, targetURL, proxyConfig string, replaceExisting bool, reuseTaskID string, client *http.Client, fallbackURL string, fallbackClient *http.Client) (string, error) {
	if err := validateCoreDownloadURL(targetURL); err != nil {
		return "", err
	}
	if strings.TrimSpace(fallbackURL) != "" {
		if err := validateCoreDownloadURL(fallbackURL); err != nil {
			return "", fmt.Errorf("备用下载地址无效: %w", err)
		}
	}
	assetKey := strings.TrimSpace(targetURL)
	m.DownloadMutex.Lock()
	if existingID := m.DownloadAssets[assetKey]; existingID != "" {
		if task := m.DownloadTasks[existingID]; task != nil && task.State.Phase != "failed" && task.State.Phase != "completed" && task.State.Phase != "cancelled" {
			m.DownloadMutex.Unlock()
			return existingID, nil
		}
	}
	taskID := reuseTaskID
	if taskID == "" {
		taskID = uuid.NewString()
	}
	ctx, cancel := context.WithCancel(parent)
	now := time.Now().Format(time.RFC3339)
	task := &downloadTask{State: DownloadTaskState{TaskID: taskID, Phase: "queued", Progress: 0, Message: "等待下载", CanRetry: false, CreatedAt: now, UpdatedAt: now, AssetName: filepath.Base(strings.SplitN(targetURL, "?", 2)[0])}, Cancel: cancel, URL: targetURL, ProxyConfig: proxyConfig, CoreInput: coreInput, ReplaceExisting: replaceExisting, Client: client, FallbackURL: fallbackURL, FallbackClient: fallbackClient}
	m.DownloadTasks[taskID] = task
	m.DownloadAssets[assetKey] = taskID
	m.DownloadMutex.Unlock()
	go m.downloadAndExtractCore(ctx, taskID, coreInput, targetURL, proxyConfig, replaceExisting, client, fallbackURL, fallbackClient)
	return taskID, nil
}

func (m *Manager) GetDownloadTask(taskID string) (DownloadTaskState, bool) {
	m.DownloadMutex.Lock()
	defer m.DownloadMutex.Unlock()
	t := m.DownloadTasks[taskID]
	if t == nil {
		return DownloadTaskState{}, false
	}
	return t.State, true
}
func (m *Manager) CancelDownloadTask(taskID string) error {
	m.DownloadMutex.Lock()
	t := m.DownloadTasks[taskID]
	m.DownloadMutex.Unlock()
	if t == nil {
		return fmt.Errorf("下载任务不存在")
	}
	t.Cancel()
	return nil
}
func (m *Manager) RetryDownloadTask(ctx context.Context, taskID string) (string, error) {
	m.DownloadMutex.Lock()
	t := m.DownloadTasks[taskID]
	if t == nil {
		m.DownloadMutex.Unlock()
		return "", fmt.Errorf("下载任务不存在")
	}
	input, urlValue, proxyValue, replace, client, fallbackURL, fallbackClient := t.CoreInput, t.URL, t.ProxyConfig, t.ReplaceExisting, t.Client, t.FallbackURL, t.FallbackClient
	delete(m.DownloadAssets, urlValue)
	m.DownloadMutex.Unlock()
	return m.startDownloadTask(ctx, input, urlValue, proxyValue, replace, taskID, client, fallbackURL, fallbackClient)
}

// RedownloadCore 重新下载指定内核，验证成功后替换原目录并保留原配置。
func (m *Manager) RedownloadCore(ctx context.Context, coreId string, targetUrl string, proxyConfig string) {
	core, ok := m.GetCore(coreId)
	if !ok {
		runtime.EventsEmit(ctx, "download:progress", DownloadProgress{Phase: "error", Progress: 0, Message: "内核不存在"})
		return
	}
	_, _ = m.startDownloadTask(ctx, CoreInput{
		CoreId:    core.CoreId,
		CoreName:  core.CoreName,
		CorePath:  core.CorePath,
		IsDefault: core.IsDefault,
	}, targetUrl, proxyConfig, true, "", nil, "", nil)
}

func (m *Manager) downloadAndExtractCore(ctx context.Context, taskID string, coreInput CoreInput, targetUrl string, proxyConfig string, replaceExisting bool, clientOverride *http.Client, fallbackURL string, fallbackClient *http.Client) {
	log := logger.New("Browser")
	t := time.Now()

	sendEvent := func(phase string, progress int, msg string) {
		eventPhase := phase
		if phase == "done" {
			eventPhase = "completed"
		}
		if phase == "error" {
			eventPhase = "failed"
		}
		m.DownloadMutex.Lock()
		if task := m.DownloadTasks[taskID]; task != nil {
			task.State.Phase = eventPhase
			task.State.Progress = float64(progress)
			task.State.Message = msg
			task.State.UpdatedAt = time.Now().Format(time.RFC3339)
			if eventPhase != "downloading" {
				task.State.SpeedBytesPerSecond = 0
				task.State.EstimatedSeconds = 0
			}
			if eventPhase == "failed" {
				task.State.CanRetry = true
				task.State.ErrorCode = "install_failed"
				task.State.ErrorDetail = msg
			}
		}
		state := DownloadTaskState{TaskID: taskID, Phase: eventPhase, Progress: float64(progress), Message: msg}
		if task := m.DownloadTasks[taskID]; task != nil {
			state = task.State
		}
		m.DownloadMutex.Unlock()
		runtime.EventsEmit(ctx, "download:progress", DownloadProgress{
			TaskID:   taskID,
			Phase:    phase,
			Progress: progress,
			Message:  msg,
		})
		runtime.EventsEmit(ctx, "browser-core:download-progress", state)
		switch eventPhase {
		case "completed":
			runtime.EventsEmit(ctx, "browser-core:download-completed", state)
		case "failed":
			runtime.EventsEmit(ctx, "browser-core:download-failed", state)
		}
	}

	sendEvent("downloading", 0, "开始解析下载地址")

	coreName := strings.TrimSpace(coreInput.CoreName)
	if coreName == "" {
		sendEvent("error", 0, "内核名称不能为空")
		return
	}

	for _, c := range m.ListCores() {
		if replaceExisting && strings.EqualFold(c.CoreId, strings.TrimSpace(coreInput.CoreId)) {
			continue
		}
		if strings.EqualFold(c.CoreName, coreName) || filepath.Base(c.CorePath) == coreName {
			sendEvent("error", 0, "名称已存在，请换一个名称")
			return
		}
	}

	targetCorePath := strings.TrimSpace(coreInput.CorePath)
	if targetCorePath == "" {
		targetCorePath = filepath.Join("chrome", coreName)
	}
	targetDir := m.ResolveRelativePath(targetCorePath)
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		sendEvent("error", 0, "创建内核目录失败")
		return
	}

	if _, err := os.Stat(targetDir); err == nil && !replaceExisting {
		sendEvent("error", 0, "同名内核目录已存在，请改名下载；如需覆盖，请在内核列表使用重新下载")
		return
	} else if !os.IsNotExist(err) {
		sendEvent("error", 0, "检查内核目录失败: "+err.Error())
		return
	}

	// 2. 准备 HttpClient（优先从 Windows 注册表读取真实系统代理，而非仅靠环境变量）
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxyConfig == "__system__" {
		// http.ProxyFromEnvironment 只读环境变量，而 Clash 的全局代理写在 Windows 注册表里
		// 必须直接读取注册表才能拿到正确的代理地址
		if sysProxy, rErr := readSystemProxy(); rErr == nil && sysProxy != "" {
			if proxyURL, pErr := url.Parse(sysProxy); pErr == nil {
				transport.Proxy = http.ProxyURL(proxyURL)
				sendEvent("downloading", 0, "已使用系统代理")
			} else {
				// 解析失败则回退到环境变量
				transport.Proxy = http.ProxyFromEnvironment
			}
		} else {
			// 没有系统代理配置或读取失败，尝试环境变量兜底
			transport.Proxy = http.ProxyFromEnvironment
			sendEvent("downloading", 0, "系统注册表无代理配置，使用环境变量兜底")
		}
	} else if proxyConfig != "" && proxyConfig != "direct://" && proxyConfig != "__direct__" {
		if proxyURL, pErr := url.Parse(proxyConfig); pErr == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		} else {
			sendEvent("error", 0, "代理地址解析失败: "+pErr.Error())
			return
		}
	}

	client := &http.Client{
		Timeout:       0,
		Transport:     transport,
		CheckRedirect: secureCoreDownloadRedirect,
	}
	if clientOverride != nil {
		custom := *clientOverride
		custom.CheckRedirect = secureCoreDownloadRedirect
		custom.Timeout = 0
		client = &custom
	}

	cacheDir := m.ResolveRelativePath(filepath.Join("download-cache", "browser-core"))
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		sendEvent("error", 0, "创建下载缓存目录失败: "+err.Error())
		return
	}
	tempFilePath := filepath.Join(cacheDir, taskID+coreArchiveSuffixFromURL(targetUrl)+".part")
	tempFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		sendEvent("error", 0, "创建临时文件失败: "+err.Error())
		return
	}
	removePart := false
	defer func() {
		tempFile.Close()
		if removePart {
			os.Remove(tempFilePath)
		}
	}()

	sendEvent("downloading", 0, "开始分析下载链接(检测多线程支持)...")

	updateStats := func(downloaded, total, speed int64) {
		m.DownloadMutex.Lock()
		defer m.DownloadMutex.Unlock()
		if task := m.DownloadTasks[taskID]; task != nil {
			task.State.DownloadedBytes = downloaded
			task.State.TotalBytes = total
			task.State.SpeedBytesPerSecond = speed
			if speed > 0 && total > downloaded {
				task.State.EstimatedSeconds = (total - downloaded) / speed
			}
		}
	}
	err = doConcurrentDownloadWithFallback(ctx, client, targetUrl, fallbackClient, fallbackURL, tempFile, sendEvent, updateStats)
	if err != nil {
		if ctx.Err() != nil {
			sendEvent("error", 0, "下载已取消")
			m.setTaskCancelled(taskID)
			return
		}
		if errors.Is(err, syscall.ENOSPC) {
			sendEvent("error", 0, "下载失败：磁盘空间不足")
			return
		}
		sendEvent("error", 0, "下载失败: "+err.Error())
		return
	}

	if err := tempFile.Sync(); err != nil {
		sendEvent("error", 0, "下载文件写盘失败: "+err.Error())
		return
	}
	if err := tempFile.Close(); err != nil {
		sendEvent("error", 0, "关闭下载文件失败: "+err.Error())
		return
	}
	shaValue, archiveSize, err := sha256File(tempFilePath)
	if err != nil {
		sendEvent("error", 0, "计算 SHA-256 失败: "+err.Error())
		return
	}
	if coreInput.Metadata != nil && strings.TrimSpace(coreInput.Metadata.ArchiveSha256) != "" && !verifySHA256(coreInput.Metadata.ArchiveSha256, shaValue) {
		sendEvent("error", 0, "发布者 SHA-256 校验失败，安装已停止")
		return
	}
	checksumMessage := "下载完成，SHA-256 已计算；发布者未提供独立校验值"
	if coreInput.Metadata != nil && coreInput.Metadata.VerificationStatus == "publisher-sha256" {
		checksumMessage = "下载完成，发布者 SHA-256 校验通过"
	}
	sendEvent("checksum", 100, checksumMessage)
	sendEvent("extracting", 0, "下载完成，正在准备解压文件...")
	log.Info("内核下载完成", logger.F("source", safeDownloadURLLabel(targetUrl)), logger.F("temp", tempFilePath), logger.F("cost", time.Since(t).String()))

	stagingRoot := filepath.Join(parentDir, ".staging")
	if err := os.MkdirAll(stagingRoot, 0755); err != nil {
		sendEvent("error", 0, "创建 staging 目录失败: "+err.Error())
		return
	}
	tempExtractDir := filepath.Join(stagingRoot, taskID)
	_ = os.RemoveAll(tempExtractDir)
	err = os.MkdirAll(tempExtractDir, 0755)
	if err != nil {
		sendEvent("error", 0, "创建临时解压目录失败: "+err.Error())
		return
	}
	cleanupTempExtract := true
	defer func() {
		if cleanupTempExtract {
			os.RemoveAll(tempExtractDir)
		}
	}()

	// 3. 执行解压，并剥离顶层文件夹
	if err := extractCoreArchiveAndStripRootContext(ctx, tempFilePath, tempExtractDir, func(p int, msg string) {
		sendEvent("extracting", p, msg)
	}); err != nil {
		sendEvent("error", 0, "解压失败: "+err.Error())
		return
	}

	if !m.ValidateCorePath(tempExtractDir).Valid {
		sendEvent("error", 0, fmt.Sprintf("解压后未找到当前平台可用的浏览器可执行文件（当前平台 %s，候选：%s），请检查压缩包内容！", CoreExecutablePlatform(), strings.Join(CoreExecutableCandidates(), ", ")))
		return
	}
	executable, _, _ := FindCoreExecutable(tempExtractDir)
	if err := validateExecutableArchitecture(executable); err != nil {
		sendEvent("error", 0, "内核架构验证失败: "+err.Error())
		return
	}

	if err := replaceCoreDirectory(targetDir, tempExtractDir, replaceExisting); err != nil {
		sendEvent("error", 0, "替换内核目录失败: "+err.Error())
		return
	}
	cleanupTempExtract = false

	coreToSave := CoreInput{
		CoreId:    strings.TrimSpace(coreInput.CoreId),
		CoreName:  coreName,
		CorePath:  targetCorePath,
		IsDefault: coreInput.IsDefault,
	}
	if coreToSave.CoreId == "" {
		coreToSave.CoreId = uuid.NewString()
		coreToSave.IsDefault = len(m.ListCores()) == 0
	}
	installedExecutable, _, _ := FindCoreExecutable(targetDir)
	coreVersion := m.GetChromeVersion(targetCorePath)
	metadata := Core{Provider: "manual-url", BrowserVersion: coreVersion, ChromiumMajor: browsercore.ChromiumMajor(coreVersion), Platform: gort.GOOS, Architecture: gort.GOARCH, ArchiveSha256: shaValue, ExecutablePath: installedExecutable, InstalledAt: time.Now().Format(time.RFC3339), LastVerifiedAt: time.Now().Format(time.RFC3339), VerificationStatus: "local-sha256", InstallationStatus: "installed", ManagedByApp: false, ArchiveSize: archiveSize}
	if coreInput.Metadata != nil {
		metadata = *coreInput.Metadata
		if coreVersion != "" {
			metadata.BrowserVersion = coreVersion
		}
		if metadata.ChromiumMajor == 0 {
			metadata.ChromiumMajor = browsercore.ChromiumMajor(coreVersion)
		}
		metadata.ArchiveSha256 = shaValue
		metadata.ExecutablePath = installedExecutable
		metadata.ArchiveSize = archiveSize
		metadata.InstalledAt = time.Now().Format(time.RFC3339)
		metadata.LastVerifiedAt = metadata.InstalledAt
		metadata.InstallationStatus = "installed"
		if metadata.VerificationStatus == "" {
			metadata.VerificationStatus = "local-sha256"
		}
	}
	coreToSave.Metadata = &metadata
	if err := m.SaveCore(coreToSave); err != nil {
		if !replaceExisting {
			_ = os.RemoveAll(targetDir)
		}
		sendEvent("error", 0, "保存配置入库失败: "+err.Error())
		return
	}
	removePart = true

	if replaceExisting && strings.TrimSpace(coreInput.CoreId) != "" {
		sendEvent("done", 100, "内核重新下载成功！")
		log.Info("内核重新下载成功", logger.F("core_id", coreToSave.CoreId), logger.F("core_name", coreName))
	} else {
		sendEvent("done", 100, "内核下载与配置成功！")
		log.Info("内核下载配置入库成功", logger.F("core_name", coreName))
	}
}

func (m *Manager) setTaskCancelled(taskID string) {
	m.DownloadMutex.Lock()
	defer m.DownloadMutex.Unlock()
	if t := m.DownloadTasks[taskID]; t != nil {
		t.State.Phase = "cancelled"
		t.State.Message = "下载已取消"
		t.State.CanRetry = true
		t.State.UpdatedAt = time.Now().Format(time.RFC3339)
	}
}

func validateCoreDownloadURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("下载地址无效: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("内核下载仅允许 HTTPS 地址")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("下载地址缺少主机名")
	}
	return nil
}
func safeDownloadURLLabel(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "invalid-url"
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}
func secureCoreDownloadRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("下载重定向次数超过 5 次")
	}
	if !strings.EqualFold(req.URL.Scheme, "https") {
		return fmt.Errorf("下载重定向到非 HTTPS 地址，已拒绝")
	}
	req.Header.Del("Authorization")
	return nil
}
func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func verifySHA256(expected, actual string) bool {
	return strings.EqualFold(strings.TrimSpace(expected), strings.TrimSpace(actual))
}
func coreArchiveSuffixFromURL(raw string) string {
	lower := strings.ToLower(strings.SplitN(strings.SplitN(raw, "?", 2)[0], "#", 2)[0])
	for _, suffix := range coreArchiveSuffixes() {
		if strings.HasSuffix(lower, suffix) {
			return suffix
		}
	}
	return ".archive"
}

func replaceCoreDirectory(targetDir string, tempExtractDir string, replaceExisting bool) error {
	if !replaceExisting {
		return os.Rename(tempExtractDir, targetDir)
	}

	backupDir := targetDir + ".backup_" + time.Now().Format("20060102150405")
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.Rename(targetDir, backupDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(tempExtractDir, targetDir); err != nil {
		if backupDir != "" {
			_ = os.Rename(backupDir, targetDir)
		}
		return err
	}

	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}
