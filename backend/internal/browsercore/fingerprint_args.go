package browsercore

import "strings"

type FingerprintArgResult struct {
	Args     []string `json:"args"`
	Warnings []string `json:"warnings"`
	Adjusted []string `json:"adjusted"`
}

func NormalizeFingerprintArgs(args []string, chromiumMajor int) FingerprintArgResult {
	values := make(map[string]string)
	order := make([]string, 0, len(args)+1)
	result := FingerprintArgResult{}
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
	}
	if platform, ok := values["--fingerprint-platform"]; ok && strings.EqualFold(strings.TrimPrefix(platform, "--fingerprint-platform="), "mac") {
		values["--fingerprint-platform"] = "--fingerprint-platform=macos"
		result.Adjusted = append(result.Adjusted, "--fingerprint-platform: mac -> macos")
	}
	if langArg, ok := values["--lang"]; ok {
		lang := strings.TrimSpace(strings.TrimPrefix(langArg, "--lang="))
		if lang != "" {
			accept := acceptLanguage(lang)
			if _, exists := values["--accept-lang"]; !exists {
				order = append(order, "--accept-lang")
			}
			values["--accept-lang"] = "--accept-lang=" + accept
			result.Adjusted = append(result.Adjusted, "根据语言生成 --accept-lang")
		}
	}
	if chromiumMajor >= 144 {
		removed := false
		for _, key := range []string{"--fingerprint-gpu-vendor", "--fingerprint-gpu-renderer", "--disable-gpu-fingerprint"} {
			if _, ok := values[key]; ok {
				delete(values, key)
				removed = true
			}
		}
		if removed {
			if _, ok := values["--disable-spoofing"]; !ok {
				order = append(order, "--disable-spoofing")
			}
			values["--disable-spoofing"] = "--disable-spoofing=gpu"
			result.Adjusted = append(result.Adjusted, "Chromium 144+ 已移除旧 GPU 参数并使用 --disable-spoofing=gpu")
		}
	}
	for _, key := range order {
		if value, ok := values[key]; ok {
			result.Args = append(result.Args, value)
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
