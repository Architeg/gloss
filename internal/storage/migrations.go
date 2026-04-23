package storage

import (
	"database/sql"
	"fmt"
)

const schemaV1 = `
CREATE TABLE IF NOT EXISTS entries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	command TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '[]',
	type TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	target TEXT NOT NULL DEFAULT '',
	managed_alias INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

// Migrate applies the embedded schema. Safe to call on every open.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schemaV1); err != nil {
		return fmt.Errorf("migrate entries: %w", err)
	}
	return nil
}
