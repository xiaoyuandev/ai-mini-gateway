package migration

import "database/sql"

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
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
