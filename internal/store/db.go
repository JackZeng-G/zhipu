package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// Open creates a connection to the SQLite database at the given path.
func Open(path string) (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite supports one writer at a time
	return db, nil
}

// Migrate creates all required tables if they do not already exist.
func Migrate(db *sqlx.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS notebooks (
		id             TEXT PRIMARY KEY,
		title          TEXT NOT NULL,
		parent_id      TEXT,
		created_time   INTEGER,
		modified_time  INTEGER
	);

	CREATE TABLE IF NOT EXISTS notes (
		id             TEXT PRIMARY KEY,
		notebook_id    TEXT,
		title          TEXT NOT NULL,
		content_html   TEXT,
		content_text   TEXT,
		tags           TEXT,
		created_time   INTEGER,
		modified_time  INTEGER,
		synced_at      INTEGER
	);

	CREATE TABLE IF NOT EXISTS conversations (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id     TEXT,
		title       TEXT,
		created_at  INTEGER
	);

	CREATE TABLE IF NOT EXISTS messages (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id  INTEGER REFERENCES conversations(id),
		role             TEXT NOT NULL,
		content          TEXT NOT NULL,
		created_at       INTEGER
	);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
