package backend

import "strings"

func fingerprintLanguage(args []string) string {
	for index := len(args) - 1; index >= 0; index-- {
		arg := strings.TrimSpace(args[index])
		if strings.HasPrefix(strings.ToLower(arg), "--lang=") {
			return strings.TrimSpace(arg[len("--lang="):])
		}
	}
	return ""
}
