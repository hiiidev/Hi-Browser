package browser

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestNormalizeProfileIconBadge(t *testing.T) {
	tests := []struct {
		name      string
		badge     string
		color     string
		wantBadge string
		wantColor string
		wantErr   bool
	}{
		{name: "number", badge: "01", color: "#2563eb", wantBadge: "01", wantColor: "#2563EB"},
		{name: "letters", badge: "ab", color: "#059669", wantBadge: "AB", wantColor: "#059669"},
		{name: "Chinese", badge: "美区", color: "#7C3AED", wantBadge: "美区", wantColor: "#7C3AED"},
		{name: "too long", badge: "1234", color: "#2563EB", wantErr: true},
		{name: "invalid character", badge: "A-", color: "#2563EB", wantErr: true},
		{name: "invalid color", badge: "01", color: "blue", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			badge, color, err := NormalizeProfileIconBadge(test.badge, test.color)
			if (err != nil) != test.wantErr {
				t.Fatalf("NormalizeProfileIconBadge(%q, %q) error = %v, wantErr %v", test.badge, test.color, err, test.wantErr)
			}
			if badge != test.wantBadge || color != test.wantColor {
				t.Errorf("NormalizeProfileIconBadge(%q, %q) = (%q, %q), want (%q, %q)", test.badge, test.color, badge, color, test.wantBadge, test.wantColor)
			}
		})
	}
}

func TestNextProfileIconBadgeLocked(t *testing.T) {
	m := &Manager{Profiles: map[string]*Profile{
		"one": {ProfileId: "one", IconBadge: "01"},
		"two": {ProfileId: "two", IconBadge: "03"},
	}}
	badge, err := m.nextProfileIconBadgeLocked("")
	if err != nil {
		t.Fatal(err)
	}
	if badge != "02" {
		t.Errorf("nextProfileIconBadgeLocked() = %q, want %q", badge, "02")
	}
}

func TestEnsureLoadedProfileIconBadgesLocked(t *testing.T) {
	profiles := []*Profile{
		{ProfileId: "first", IconBadgeColor: "#2563EB"},
		{ProfileId: "second", IconBadge: "01", IconBadgeColor: "#7c3aed"},
		{ProfileId: "third", IconBadge: "01", IconBadgeColor: "#DC2626"},
	}
	m := &Manager{Profiles: map[string]*Profile{
		"first": profiles[0], "second": profiles[1], "third": profiles[2],
	}}
	if !m.ensureLoadedProfileIconBadgesLocked(profiles) {
		t.Fatal("ensureLoadedProfileIconBadgesLocked() = false, want true")
	}
	wantBadges := []string{"02", "01", "03"}
	for index, profile := range profiles {
		if profile.IconBadge != wantBadges[index] {
			t.Errorf("profiles[%d].IconBadge = %q, want %q", index, profile.IconBadge, wantBadges[index])
		}
	}
}

func TestAddImportedProfilesAssignsUniqueBadges(t *testing.T) {
	m := NewManager(nil, t.TempDir())
	m.Config = testBadgeConfig()
	m.Profiles["existing"] = &Profile{ProfileId: "existing", IconBadge: "01", IconBadgeColor: "#2563EB"}
	profiles := []*Profile{{ProfileId: "import-1"}, {ProfileId: "import-2"}}
	if err := m.AddImportedProfiles(profiles); err != nil {
		t.Fatal(err)
	}
	if profiles[0].IconBadge != "02" || profiles[1].IconBadge != "03" {
		t.Errorf("imported badges = (%q, %q), want (%q, %q)", profiles[0].IconBadge, profiles[1].IconBadge, "02", "03")
	}
}

func TestUpdatePreservesBadgeForLegacyInput(t *testing.T) {
	m := NewManager(testBadgeConfig(), t.TempDir())
	m.Profiles["profile-1"] = &Profile{
		ProfileId:      "profile-1",
		ProfileName:    "before",
		IconBadge:      "A1",
		IconBadgeColor: "#7C3AED",
	}
	updated, err := m.Update("profile-1", ProfileInput{ProfileName: "after"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.IconBadge != "A1" || updated.IconBadgeColor != "#7C3AED" {
		t.Errorf("updated badge = (%q, %q), want legacy values", updated.IconBadge, updated.IconBadgeColor)
	}
}

func testBadgeConfig() *config.Config {
	return &config.Config{}
}
