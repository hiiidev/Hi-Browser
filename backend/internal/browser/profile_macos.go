package browser

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ProfileBundleIdentifier is stable for the lifetime of a profile and is shared
// by the derived macOS app bundle and its isolated CFPreferences domain.
func ProfileBundleIdentifier(profileID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(profileID)))
	return fmt.Sprintf("com.hibrowser.profile.%x", digest)[:42]
}
