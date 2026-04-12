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
	db.SetMaxOpenConns(1)
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

	CREATE TABLE IF NOT EXISTS ai_configs (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		provider         TEXT NOT NULL,
		name             TEXT NOT NULL,
		base_url         TEXT,
		model            TEXT NOT NULL,
		api_key_encrypted TEXT,
		is_active        INTEGER DEFAULT 0,
		created_at       INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS index_metadata (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		entity_type   TEXT NOT NULL,
		target_id     TEXT NOT NULL,
		ai_config_id  INTEGER REFERENCES ai_configs(id),
		model_name    TEXT NOT NULL,
		generated_at  INTEGER NOT NULL,
		content_hash  TEXT
	);

	CREATE TABLE IF NOT EXISTS note_entities (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id       TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		entity_type   TEXT NOT NULL,
		entity_name   TEXT NOT NULL,
		description   TEXT,
		created_at    INTEGER NOT NULL,
		UNIQUE(note_id, entity_type, entity_name)
	);

	CREATE TABLE IF NOT EXISTS note_summaries (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id       TEXT NOT NULL UNIQUE REFERENCES notes(id) ON DELETE CASCADE,
		summary       TEXT NOT NULL,
		key_points    TEXT,
		generated_at  INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS note_relations (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		source_note_id  TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		target_note_id  TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		relation_type   TEXT NOT NULL,
		reason          TEXT,
		confidence      REAL,
		created_at      INTEGER NOT NULL,
		UNIQUE(source_note_id, target_note_id, relation_type)
	);

	CREATE TABLE IF NOT EXISTS activity_log (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		activity_type TEXT NOT NULL,
		target_type   TEXT,
		target_id     TEXT,
		description   TEXT,
		metadata      TEXT,
		created_at    INTEGER NOT NULL
	);

	-- Legacy wiki_pages (kept for backward compatibility)
	CREATE TABLE IF NOT EXISTS wiki_pages (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		slug            TEXT NOT NULL UNIQUE,
		title           TEXT NOT NULL,
		content         TEXT NOT NULL,
		source_note_ids TEXT,
		page_type       TEXT NOT NULL,
		generated_at    INTEGER NOT NULL,
		updated_at      INTEGER NOT NULL
	);

	-- Concept pages: the core of the knowledge graph
	-- Each concept aggregates knowledge from ALL notes mentioning it
	CREATE TABLE IF NOT EXISTS wiki_concepts (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		slug            TEXT NOT NULL UNIQUE,
		title           TEXT NOT NULL,
		aliases         TEXT DEFAULT '[]',
		definition      TEXT DEFAULT '',
		key_points      TEXT DEFAULT '',
		content         TEXT DEFAULT '',
		note_ids        TEXT DEFAULT '[]',
		source_count    INTEGER DEFAULT 0,
		confidence      TEXT DEFAULT 'low',
		evolution_log   TEXT DEFAULT '[]',
		created_at      INTEGER NOT NULL,
		updated_at      INTEGER NOT NULL
	);

	-- Concept-to-concept relationships (knowledge graph edges)
	CREATE TABLE IF NOT EXISTS concept_relations (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		source_concept_slug TEXT NOT NULL,
		target_concept_slug TEXT NOT NULL,
		relation_type       TEXT DEFAULT 'related',
		reason              TEXT DEFAULT '',
		co_occurrence_count INTEGER DEFAULT 0,
		created_at          INTEGER NOT NULL,
		UNIQUE(source_concept_slug, target_concept_slug)
	);

	-- Synthesis pages: cross-concept deep analysis
	CREATE TABLE IF NOT EXISTS wiki_synthesis (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		slug            TEXT NOT NULL UNIQUE,
		title           TEXT NOT NULL,
		content         TEXT NOT NULL DEFAULT '',
		concept_slugs   TEXT DEFAULT '[]',
		confidence_notes TEXT DEFAULT '',
		created_at      INTEGER NOT NULL,
		updated_at      INTEGER NOT NULL
	);

	-- Open questions tracker
	CREATE TABLE IF NOT EXISTS questions (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		content              TEXT NOT NULL,
		status               TEXT DEFAULT 'open',
		opened_at            INTEGER NOT NULL,
		answered_at          INTEGER,
		answer_synthesis_slug TEXT
	);

	-- Category tree for note classification
	CREATE TABLE IF NOT EXISTS categories (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_id  INTEGER REFERENCES categories(id) ON DELETE CASCADE,
		name       TEXT NOT NULL,
		path       TEXT NOT NULL UNIQUE,
		note_count INTEGER DEFAULT 0,
		depth      INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

		-- Persisted query answers and analysis outputs
		CREATE TABLE IF NOT EXISTS wiki_outputs (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			slug             TEXT NOT NULL UNIQUE,
			title            TEXT NOT NULL,
			content          TEXT NOT NULL,
			output_type      TEXT DEFAULT 'query',
			source_concepts  TEXT,
			confidence_notes TEXT,
			created_at       INTEGER DEFAULT (strftime('%s','now')),
			updated_at       INTEGER DEFAULT (strftime('%s','now'))
		);

	-- Note-Category mapping (many-to-many)
	CREATE TABLE IF NOT EXISTS note_categories (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id     TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
		is_primary  INTEGER DEFAULT 0,
		UNIQUE(note_id, category_id)
	);
	CREATE INDEX IF NOT EXISTS idx_note_categories_note ON note_categories(note_id);
	CREATE INDEX IF NOT EXISTS idx_note_categories_cat ON note_categories(category_id);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Run incremental migrations for existing tables
	migrations := []string{
		"ALTER TABLE wiki_concepts ADD COLUMN contradictions TEXT DEFAULT ''",
		"ALTER TABLE wiki_concepts ADD COLUMN domain_volatility TEXT DEFAULT 'medium'",
		"ALTER TABLE wiki_concepts ADD COLUMN last_reviewed INTEGER",
		"ALTER TABLE wiki_concepts ADD COLUMN confidence_pending INTEGER DEFAULT 0",
		"ALTER TABLE wiki_concepts ADD COLUMN redirect_to TEXT DEFAULT ''",
		"ALTER TABLE notes ADD COLUMN content_hash TEXT",
	}
	for _, m := range migrations {
		db.Exec(m) // ignore error if column already exists
	}

	return nil
}
