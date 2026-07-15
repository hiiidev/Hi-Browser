package browsercore

import (
	"strconv"
	"strings"
)

type FingerprintCapabilities struct {
	Provider              string   `json:"provider"`
	ChromiumMajor         int      `json:"chromiumMajor"`
	HostPlatform          string   `json:"hostPlatform"`
	TargetPlatform        string   `json:"targetPlatform"`
	SupportedParameters   []string `json:"supportedParameters"`
	DeprecatedParameters  []string `json:"deprecatedParameters"`
	UnsupportedParameters []string `json:"unsupportedParameters"`
	Warnings              []string `json:"warnings"`
	GPUSpoofingMode       string   `json:"gpuSpoofingMode"`
}

func Capabilities(provider, version, hostPlatform, targetPlatform string) FingerprintCapabilities {
	major := ChromiumMajor(version)
	targetPlatform = NormalizePlatform(targetPlatform)
	c := FingerprintCapabilities{
		Provider: provider, ChromiumMajor: major, HostPlatform: NormalizePlatform(hostPlatform), TargetPlatform: targetPlatform,
		SupportedParameters: []string{"--fingerprint", "--fingerprint-brand", "--fingerprint-platform", "--lang", "--accept-lang", "--timezone", "--window-size"},
		GPUSpoofingMode:     "legacy-vendor-renderer",
	}
	if major >= 144 {
		c.DeprecatedParameters = []string{"--fingerprint-gpu-vendor", "--fingerprint-gpu-renderer", "--disable-gpu-fingerprint"}
		c.UnsupportedParameters = append([]string(nil), c.DeprecatedParameters...)
		c.SupportedParameters = append(c.SupportedParameters, "--disable-spoofing=gpu")
		c.GPUSpoofingMode = "kernel-policy"
	} else {
		c.SupportedParameters = append(c.SupportedParameters, "--fingerprint-gpu-vendor", "--fingerprint-gpu-renderer", "--disable-gpu-fingerprint")
	}
	if c.HostPlatform != "" && targetPlatform != "" && c.HostPlatform != targetPlatform {
		c.Warnings = append(c.Warnings, "模拟平台与宿主平台不同，字体、GPU、系统 API 可能出现一致性风险")
	}
	return c
}

func ChromiumMajor(version string) int {
	version = strings.TrimLeft(strings.TrimSpace(version), "vV")
	part := strings.SplitN(version, ".", 2)[0]
	major, _ := strconv.Atoi(part)
	return major
}

func NormalizePlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "mac", "macos", "darwin", "osx":
		return "macos"
	case "win", "windows":
		return "windows"
	case "linux":
		return "linux"
	default:
		return strings.ToLower(strings.TrimSpace(platform))
	}
}
