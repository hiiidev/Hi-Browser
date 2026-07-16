package database

import (
	"path/filepath"
	"testing"
)

func TestProfileIconBadgeMigration(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.conn.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, desc TEXT NOT NULL DEFAULT '', applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO schema_migrations(version,desc) VALUES(13,'legacy');
		CREATE TABLE browser_profiles(profile_id TEXT PRIMARY KEY);
		INSERT INTO browser_profiles(profile_id) VALUES('legacy')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	var badge, badgeColor string
	if err := db.conn.QueryRow(`SELECT icon_badge, icon_badge_color FROM browser_profiles WHERE profile_id = 'legacy'`).Scan(&badge, &badgeColor); err != nil {
		t.Fatal(err)
	}
	if badge != "" || badgeColor != "#2563EB" {
		t.Errorf("migrated badge = (%q, %q), want (%q, %q)", badge, badgeColor, "", "#2563EB")
	}
}
