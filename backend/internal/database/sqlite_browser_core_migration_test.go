package database

import (
	"path/filepath"
	"testing"
)

func TestBrowserCoreLegacyRecordMigration(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.conn.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, desc TEXT NOT NULL DEFAULT '', applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO schema_migrations(version,desc) VALUES(12,'legacy');
		CREATE TABLE browser_cores(core_id TEXT PRIMARY KEY,core_name TEXT NOT NULL,core_path TEXT NOT NULL,is_default INTEGER NOT NULL DEFAULT 0,sort_order INTEGER NOT NULL DEFAULT 0,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO browser_cores(core_id,core_name,core_path,is_default) VALUES('legacy','Legacy','chrome/legacy',1)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	var name, path, provider string
	var managed int
	if err := db.conn.QueryRow(`SELECT core_name,core_path,provider,managed_by_app FROM browser_cores WHERE core_id='legacy'`).Scan(&name, &path, &provider, &managed); err != nil {
		t.Fatal(err)
	}
	if name != "Legacy" || path != "chrome/legacy" || provider != "" || managed != 0 {
		t.Fatalf("unexpected migrated row %q %q %q %d", name, path, provider, managed)
	}
}
