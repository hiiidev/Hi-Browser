package browsercore

import (
	"strconv"
	"strings"
)

const (
	fingerprintArgSourceUser       = "user"
	fingerprintArgSourceMigration  = "compatibility-migration"
	fingerprintArgSourceCapability = "core-capability-adjustment"
)

type FingerprintArgEntry struct {
	Arg    string `json:"arg"`
	Source string `json:"source"`
}

type FingerprintArgResult struct {
	Args     []string              `json:"args"`
	Warnings []string              `json:"warnings"`
	Adjusted []string              `json:"adjusted"`
	Entries  []FingerprintArgEntry `json:"entries"`
}

func NormalizeFingerprintArgs(args []string, chromiumMajor int) FingerprintArgResult {
	version := ""
	if chromiumMajor > 0 {
		version = strconv.Itoa(chromiumMajor)
	}
	return NormalizeFingerprintArgsForCapabilities(args, Capabilities("", version, "", ""))
}

func NormalizeFingerprintArgsForCapabilities(args []string, capabilities FingerprintCapabilities) FingerprintArgResult {
	values := make(map[string]string)
	sources := make(map[string]string)
	order := make([]string, 0, len(args)+1)
	result := FingerprintArgResult{
		Args:     make([]string, 0, len(args)),
		Warnings: make([]string, 0),
		Adjusted: make([]string, 0),
		Entries:  make([]FingerprintArgEntry, 0, len(args)+1),
	}
	for _, raw := range args {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		key := raw
		if index := strings.Index(raw, "="); index >= 0 {
			key = raw[:index]
		}
		if _, exists := values[key]; exists {
			result.Warnings = append(result.Warnings, "参数 "+key+" 重复，已保留最后一个值")
		} else {
			order = append(order, key)
		}
		values[key] = raw
		sources[key] = fingerprintArgSourceUser
	}
	if platform, ok := values["--fingerprint-platform"]; ok && strings.EqualFold(strings.TrimPrefix(platform, "--fingerprint-platform="), "mac") {
		values["--fingerprint-platform"] = "--fingerprint-platform=macos"
		sources["--fingerprint-platform"] = fingerprintArgSourceMigration
		result.Adjusted = append(result.Adjusted, "--fingerprint-platform: mac -> macos")
	}
	if langArg, ok := values["--lang"]; ok {
		lang := strings.TrimSpace(strings.TrimPrefix(langArg, "--lang="))
		if lang != "" {
			accept := acceptLanguage(lang)
			existing, exists := values["--accept-lang"]
			changed := !exists || !strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(existing, "--accept-lang=")), accept)
			if !exists {
				order = append(order, "--accept-lang")
			} else if changed {
				result.Warnings = append(result.Warnings, "--accept-lang 与网页首选语言冲突，已按 --lang 重新生成")
			}
			values["--accept-lang"] = "--accept-lang=" + accept
			if changed {
				sources["--accept-lang"] = fingerprintArgSourceCapability
				result.Adjusted = append(result.Adjusted, "根据语言生成 --accept-lang")
			}
		}
	}
	for _, key := range order {
		if value, ok := values[key]; ok {
			result.Args = append(result.Args, value)
			result.Entries = append(result.Entries, FingerprintArgEntry{Arg: value, Source: sources[key]})
		}
	}
	return result
}

func acceptLanguage(lang string) string {
	primary := lang
	if index := strings.Index(lang, "-"); index > 0 {
		primary = lang[:index]
	}
	if strings.EqualFold(primary, lang) {
		return lang
	}
	return lang + "," + primary
}
