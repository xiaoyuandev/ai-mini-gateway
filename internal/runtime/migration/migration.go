package migration

import (
	"database/sql"
	"fmt"
)

func Apply(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS model_sources (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			provider_type TEXT NOT NULL,
			default_model_id TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			position INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS selected_models (
			model_id TEXT PRIMARY KEY,
			position INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS model_source_exposed_models (
			source_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			PRIMARY KEY (source_id, model_id)
		)`,
		`CREATE TABLE IF NOT EXISTS runtime_metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	if err := ensureColumn(db, "model_sources", "external_id", "TEXT"); err != nil {
		return err
	}

	return nil
}

func ensureColumn(db *sql.DB, table string, column string, definition string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			typ        string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}
