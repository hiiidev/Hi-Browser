package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

type resolvedNodeRuntime struct {
	Source             string
	Path               string
	Version            string
	SystemNodeDetected bool
	SystemNodePath     string
	Resolution         string
	SystemNodeError    string
}

type nodeProbeResult struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type nodeCandidate struct {
	path       string
	resolution string
}

type SystemNodeProbeResult struct {
	OK      bool   `json:"ok"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

func (m *Manager) resolveNodeRuntime(runtimeDir string, auto config.AutomationConfig) resolvedNodeRuntime {
	mode := config.DefaultAutomationNodeSource
	if auto.NodeSource != "" {
		mode = strings.TrimSpace(auto.NodeSource)
	}

	if mode != config.AutomationNodeSourceBundled {
		if systemNode, err := m.resolveSystemNode(context.Background(), auto.SystemNodePath); err == nil {
			return systemNode
		} else if mode == config.AutomationNodeSourceSystem {
			return resolvedNodeRuntime{
				Source:          config.AutomationNodeSourceSystem,
				Version:         strings.TrimSpace(auto.NodeVersion),
				SystemNodePath:  strings.TrimSpace(auto.SystemNodePath),
				Resolution:      "已配置为 system，必须使用系统 Node",
				SystemNodeError: err.Error(),
			}
		} else {
			return resolvedNodeRuntime{
				Source:          config.AutomationNodeSourceBundled,
				Path:            m.nodeExecutablePath(runtimeDir),
				Version:         strings.TrimSpace(auto.NodeVersion),
				SystemNodePath:  strings.TrimSpace(auto.SystemNodePath),
				Resolution:      "系统 Node 不可用，已回退到内建 Node",
				SystemNodeError: err.Error(),
			}
		}
	}

	return resolvedNodeRuntime{
		Source:     config.AutomationNodeSourceBundled,
		Path:       m.nodeExecutablePath(runtimeDir),
		Version:    strings.TrimSpace(auto.NodeVersion),
		Resolution: "已配置为 bundled，始终使用内建 Node",
	}
}

func (m *Manager) resolveSystemNode(ctx context.Context, explicitPath string) (resolvedNodeRuntime, error) {
	candidatePaths := m.systemNodeCandidates(explicitPath)

	var lastErr error
	for _, candidate := range candidatePaths {
		probe, err := m.probeNodeExecutable(ctx, candidate.path)
		if err != nil {
			lastErr = err
			continue
		}
		return resolvedNodeRuntime{
			Source:             config.AutomationNodeSourceSystem,
			Path:               probe.Path,
			Version:            probe.Version,
			SystemNodeDetected: true,
			SystemNodePath:     probe.Path,
			Resolution:         candidate.resolution,
		}, nil
	}

	if lastErr != nil {
		return resolvedNodeRuntime{}, lastErr
	}
	return resolvedNodeRuntime{}, fmt.Errorf("未找到系统 Node")
}

func (m *Manager) systemNodeCandidates(explicitPath string) []nodeCandidate {
	candidates := make([]nodeCandidate, 0, 8)
	add := func(path, resolution string, requireExisting bool) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if requireExisting {
			if info, err := os.Stat(path); err != nil || info.IsDir() {
				return
			}
		}
		for _, existing := range candidates {
			if strings.EqualFold(filepath.Clean(existing.path), filepath.Clean(path)) {
				return
			}
		}
		candidates = append(candidates, nodeCandidate{path: path, resolution: resolution})
	}

	add(explicitPath, "已使用配置的系统 Node 路径", false)
	if lookupPath, err := exec.LookPath("node"); err == nil {
		add(lookupPath, "已使用 PATH 中的系统 Node", false)
	}

	home, _ := os.UserHomeDir()
	targetOS := strings.ToLower(strings.TrimSpace(m.options.TargetOS))
	if targetOS == "" {
		targetOS = goruntime.GOOS
	}
	for _, path := range commonSystemNodePaths(targetOS, home) {
		add(path, "已使用系统常见安装位置中的 Node", true)
	}
	return candidates
}

func commonSystemNodePaths(goos, home string) []string {
	home = strings.TrimSpace(home)
	paths := make([]string, 0, 8)
	appendHome := func(parts ...string) {
		if home != "" {
			paths = append(paths, filepath.Join(append([]string{home}, parts...)...))
		}
	}

	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		paths = append(paths, "/opt/homebrew/bin/node", "/usr/local/bin/node", "/opt/local/bin/node")
		appendHome(".volta", "bin", "node")
		appendHome(".asdf", "shims", "node")
		appendHome(".local", "share", "mise", "shims", "node")
		appendHome(".fnm", "aliases", "default", "bin", "node")
	case "linux":
		paths = append(paths, "/usr/local/bin/node", "/usr/bin/node", "/snap/bin/node")
		appendHome(".volta", "bin", "node")
		appendHome(".asdf", "shims", "node")
		appendHome(".local", "share", "mise", "shims", "node")
		appendHome(".fnm", "aliases", "default", "bin", "node")
	case "windows":
		if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "nodejs", "node.exe"))
		}
		if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Programs", "nodejs", "node.exe"))
		}
		appendHome("scoop", "apps", "nodejs", "current", "node.exe")
	}
	return paths
}

func (m *Manager) probeNodeExecutable(ctx context.Context, nodePath string) (nodeProbeResult, error) {
	nodePath = strings.TrimSpace(nodePath)
	if nodePath == "" {
		return nodeProbeResult{}, fmt.Errorf("Node 路径为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	script := `process.stdout.write(JSON.stringify({path: process.execPath, version: process.versions.node}));`
	cmd := exec.CommandContext(probeCtx, nodePath, "-e", script)
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nodeProbeResult{}, fmt.Errorf("检测 Node 可执行文件失败（%s）: %s", nodePath, message)
	}

	var probe nodeProbeResult
	if err := json.Unmarshal(output, &probe); err != nil {
		return nodeProbeResult{}, fmt.Errorf("解析 Node 探测结果失败: %w", err)
	}
	probe.Path = strings.TrimSpace(probe.Path)
	probe.Version = strings.TrimSpace(probe.Version)
	if probe.Path == "" {
		probe.Path = nodePath
	}
	if probe.Version == "" {
		return nodeProbeResult{}, fmt.Errorf("Node 版本为空")
	}
	if absPath, err := filepath.Abs(probe.Path); err == nil {
		probe.Path = absPath
	}
	return probe, nil
}

func (m *Manager) ProbeSystemNode(ctx context.Context, explicitPath string) (SystemNodeProbeResult, error) {
	resolved, err := m.resolveSystemNode(ctx, explicitPath)
	if err != nil {
		return SystemNodeProbeResult{}, err
	}
	return SystemNodeProbeResult{
		OK:      true,
		Path:    strings.TrimSpace(resolved.Path),
		Version: strings.TrimSpace(resolved.Version),
	}, nil
}

func (m *Manager) verifyNodeWithPlaywright(ctx context.Context, nodePath, runtimeDir string) (RuntimeCheckResult, error) {
	nodePath = strings.TrimSpace(nodePath)
	runtimeDir = strings.TrimSpace(runtimeDir)
	if nodePath == "" {
		return RuntimeCheckResult{}, fmt.Errorf("node path is empty")
	}
	if runtimeDir == "" {
		return RuntimeCheckResult{}, fmt.Errorf("runtime dir is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	script := `
const path = require('path');
const pkg = require(path.join(process.argv[1], 'node_modules', 'playwright-core', 'package.json'));
const playwright = require(path.join(process.argv[1], 'node_modules', 'playwright-core'));
process.stdout.write(JSON.stringify({
  nodeVersion: process.versions.node,
  playwrightVersion: pkg.version,
  hasChromium: !!playwright.chromium
}));
`

	cmd := exec.CommandContext(checkCtx, nodePath, "-e", script, runtimeDir)
	cmd.Dir = runtimeDir
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return RuntimeCheckResult{}, fmt.Errorf("playwright probe failed: %s", message)
	}

	var payload struct {
		NodeVersion       string `json:"nodeVersion"`
		PlaywrightVersion string `json:"playwrightVersion"`
		HasChromium       bool   `json:"hasChromium"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return RuntimeCheckResult{}, fmt.Errorf("parse playwright probe result failed: %w", err)
	}

	result := RuntimeCheckResult{
		OK:                strings.TrimSpace(payload.NodeVersion) != "" && strings.TrimSpace(payload.PlaywrightVersion) != "" && payload.HasChromium,
		NodeVersion:       strings.TrimSpace(payload.NodeVersion),
		PlaywrightVersion: strings.TrimSpace(payload.PlaywrightVersion),
	}
	if !result.OK {
		return RuntimeCheckResult{}, fmt.Errorf("playwright probe returned incomplete result")
	}
	return result, nil
}
