package browser

import "testing"

func TestProfileBundleIdentifierIsStableAndDistinct(t *testing.T) {
	first := ProfileBundleIdentifier("profile-1")
	if first != ProfileBundleIdentifier("profile-1") {
		t.Fatal("bundle identifier is not stable")
	}
	if first == ProfileBundleIdentifier("profile-2") {
		t.Fatal("bundle identifiers are not distinct")
	}
	if len(first) != 42 {
		t.Fatalf("bundle identifier length = %d, want 42: %s", len(first), first)
	}
}
