package migration

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", t.TempDir()+"/gateway.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := Apply(db); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := Apply(db); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	for _, table := range []string{
		"schema_migrations",
		"model_sources",
		"selected_models",
		"model_source_exposed_models",
	} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatalf("query sqlite_master for %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("expected table %s to exist once, got %d", table, count)
		}
	}
}
