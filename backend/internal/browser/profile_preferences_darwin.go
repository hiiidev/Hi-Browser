//go:build darwin

package browser

import (
	"fmt"
	"os/exec"
	"strings"
)

func cleanupPlatformProfilePreferences(profileID string) error {
	domain := ProfileBundleIdentifier(profileID)
	output, err := exec.Command("defaults", "delete", domain).CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if strings.Contains(strings.ToLower(detail), "does not exist") {
		return nil
	}
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, detail)
}
