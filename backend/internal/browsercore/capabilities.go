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
	SupportedBrands       []string `json:"supportedBrands"`
	SupportedParameters   []string `json:"supportedParameters"`
	DeprecatedParameters  []string `json:"deprecatedParameters"`
	UnsupportedParameters []string `json:"unsupportedParameters"`
	Warnings              []string `json:"warnings"`
	GPUSpoofingMode       string   `json:"gpuSpoofingMode"`
	ManualGPUConfig       bool     `json:"manualGpuConfig"`
	WebpageLanguage       bool     `json:"webpageLanguage"`
	ApplicationLocaleMode string   `json:"applicationLocaleMode"`
	IntlLocaleMode        string   `json:"intlLocaleMode"`
	TTSVoicesMode         string   `json:"ttsVoicesMode"`
	FontsMode             string   `json:"fontsMode"`
}

func Capabilities(provider, version, hostPlatform, targetPlatform string) FingerprintCapabilities {
	major := ChromiumMajor(version)
	hostPlatform = NormalizePlatform(hostPlatform)
	targetPlatform = NormalizePlatform(targetPlatform)
	if targetPlatform == "" {
		targetPlatform = hostPlatform
	}
	c := FingerprintCapabilities{
		Provider:              provider,
		ChromiumMajor:         major,
		HostPlatform:          hostPlatform,
		TargetPlatform:        targetPlatform,
		SupportedBrands:       []string{"Chrome", "Edge", "Opera", "Vivaldi", "Firefox", "Safari"},
		SupportedParameters:   []string{"--fingerprint", "--fingerprint-brand", "--fingerprint-platform", "--lang", "--accept-lang", "--timezone", "--window-size"},
		GPUSpoofingMode:       "legacy-vendor-renderer",
		ManualGPUConfig:       true,
		WebpageLanguage:       true,
		ApplicationLocaleMode: "chromium",
		IntlLocaleMode:        "chromium",
		TTSVoicesMode:         "host",
		FontsMode:             "host",
	}
	if major >= 144 {
		c.DeprecatedParameters = []string{"--fingerprint-gpu-vendor", "--fingerprint-gpu-renderer", "--fingerprint-webgl-vendor", "--fingerprint-webgl-renderer", "--disable-gpu-fingerprint"}
		c.UnsupportedParameters = append([]string(nil), c.DeprecatedParameters...)
		c.SupportedParameters = append(c.SupportedParameters, "--disable-spoofing=gpu")
		c.GPUSpoofingMode = "seed-driven-real-parameter-set"
		c.ManualGPUConfig = false
	} else {
		c.SupportedParameters = append(c.SupportedParameters, "--fingerprint-gpu-vendor", "--fingerprint-gpu-renderer", "--fingerprint-webgl-vendor", "--fingerprint-webgl-renderer", "--disable-gpu-fingerprint")
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
