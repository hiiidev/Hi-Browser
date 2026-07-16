package browser

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const DefaultProfileIconBadgeColor = "#2563EB"

var profileIconBadgePalette = []string{
	"#2563EB",
	"#7C3AED",
	"#DB2777",
	"#DC2626",
	"#EA580C",
	"#CA8A04",
	"#059669",
	"#0891B2",
}

// NormalizeProfileIconBadge validates and normalizes a system icon badge.
func NormalizeProfileIconBadge(badge, badgeColor string) (string, string, error) {
	badge = strings.TrimSpace(badge)
	if badge != "" {
		if utf8.RuneCountInString(badge) > 3 {
			return "", "", fmt.Errorf("实例角标最多支持 3 个字符")
		}
		for _, r := range badge {
			if unicode.IsSpace(r) || unicode.IsControl(r) || (!unicode.IsLetter(r) && !unicode.IsDigit(r)) {
				return "", "", fmt.Errorf("实例角标仅支持字母、数字或中文")
			}
		}
		badge = strings.ToUpper(badge)
	}

	badgeColor = strings.ToUpper(strings.TrimSpace(badgeColor))
	if badgeColor == "" {
		badgeColor = defaultProfileIconBadgeColor(badge)
	}
	if len(badgeColor) != 7 || badgeColor[0] != '#' {
		return "", "", fmt.Errorf("实例角标颜色必须是 #RRGGBB 格式")
	}
	if _, err := strconv.ParseUint(badgeColor[1:], 16, 24); err != nil {
		return "", "", fmt.Errorf("实例角标颜色必须是 #RRGGBB 格式")
	}
	return badge, badgeColor, nil
}

func defaultProfileIconBadgeColor(badge string) string {
	if badge == "" {
		return DefaultProfileIconBadgeColor
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(badge))
	return profileIconBadgePalette[int(h.Sum32())%len(profileIconBadgePalette)]
}

func (m *Manager) normalizeProfileIconBadgeInputLocked(badge, badgeColor, currentProfileID string) (string, string, error) {
	normalizedBadge, normalizedColor, err := NormalizeProfileIconBadge(badge, badgeColor)
	if err != nil {
		return "", "", err
	}
	if normalizedBadge == "" {
		normalizedBadge, err = m.nextProfileIconBadgeLocked(currentProfileID)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(badgeColor) == "" {
			normalizedColor = defaultProfileIconBadgeColor(normalizedBadge)
		}
	}
	for profileID, profile := range m.Profiles {
		if profile == nil || profileID == currentProfileID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(profile.IconBadge), normalizedBadge) {
			return "", "", fmt.Errorf("实例角标 %s 已被其他实例使用", normalizedBadge)
		}
	}
	return normalizedBadge, normalizedColor, nil
}

func (m *Manager) nextProfileIconBadgeLocked(currentProfileID string) (string, error) {
	used := make(map[string]struct{}, len(m.Profiles))
	for profileID, profile := range m.Profiles {
		if profile == nil || profileID == currentProfileID {
			continue
		}
		badge := strings.ToUpper(strings.TrimSpace(profile.IconBadge))
		if badge != "" {
			used[badge] = struct{}{}
		}
	}
	for number := 1; number <= 999; number++ {
		width := 2
		if number >= 100 {
			width = 3
		}
		candidate := fmt.Sprintf("%0*d", width, number)
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("实例角标编号已用尽")
}

func (m *Manager) ensureLoadedProfileIconBadgesLocked(profiles []*Profile) bool {
	changed := false
	used := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		badge, color, err := NormalizeProfileIconBadge(profile.IconBadge, profile.IconBadgeColor)
		if err != nil || badge == "" {
			profile.IconBadge = ""
			continue
		}
		if _, duplicate := used[badge]; duplicate {
			profile.IconBadge = ""
			changed = true
			continue
		}
		used[badge] = struct{}{}
		if badge != profile.IconBadge || color != profile.IconBadgeColor {
			profile.IconBadge = badge
			profile.IconBadgeColor = color
			changed = true
		}
	}
	for _, profile := range profiles {
		if profile == nil || profile.IconBadge != "" {
			continue
		}
		badge, err := nextProfileIconBadgeFromSet(used)
		if err != nil {
			continue
		}
		profile.IconBadge = badge
		profile.IconBadgeColor = defaultProfileIconBadgeColor(badge)
		used[badge] = struct{}{}
		changed = true
	}
	return changed
}

func nextProfileIconBadgeFromSet(used map[string]struct{}) (string, error) {
	for number := 1; number <= 999; number++ {
		width := 2
		if number >= 100 {
			width = 3
		}
		candidate := fmt.Sprintf("%0*d", width, number)
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("实例角标编号已用尽")
}

// AddImportedProfiles assigns fresh unique icon badges and adds imported profiles atomically.
func (m *Manager) AddImportedProfiles(profiles []*Profile) error {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	added := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		badge, color, err := m.normalizeProfileIconBadgeInputLocked("", "", profile.ProfileId)
		if err != nil {
			for _, profileID := range added {
				delete(m.Profiles, profileID)
			}
			return err
		}
		profile.IconBadge = badge
		profile.IconBadgeColor = color
		m.Profiles[profile.ProfileId] = profile
		added = append(added, profile.ProfileId)
	}
	return nil
}
