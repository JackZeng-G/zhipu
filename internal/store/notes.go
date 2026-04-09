package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Notebook represents a notebook (folder) that contains notes.
type Notebook struct {
	ID            string `db:"id" json:"id"`
	Title         string `db:"title" json:"title"`
	ParentID      *string `db:"parent_id" json:"parent_id"`
	CreatedTime   int64  `db:"created_time" json:"created_time"`
	ModifiedTime  int64  `db:"modified_time" json:"modified_time"`
}

// Note represents a single note within a notebook.
type Note struct {
	ID            string  `db:"id" json:"id"`
	NotebookID    *string `db:"notebook_id" json:"notebook_id"`
	Title         string  `db:"title" json:"title"`
	ContentHTML   *string `db:"content_html" json:"content_html"`
	ContentText   *string `db:"content_text" json:"content_text"`
	Tags          *string `db:"tags" json:"tags"`
	CreatedTime   int64   `db:"created_time" json:"created_time"`
	ModifiedTime  int64   `db:"modified_time" json:"modified_time"`
	SyncedAt      *int64  `db:"synced_at" json:"synced_at"`
}

// NotesStore provides CRUD for notes and notebooks.
type NotesStore struct {
	db *sqlx.DB
}

// NewNotesStore creates a notes store.
func NewNotesStore(db *sqlx.DB) *NotesStore {
	return &NotesStore{db: db}
}

// SaveNote inserts or updates a note by ID (upsert).
func (s *NotesStore) SaveNote(ctx context.Context, note *Note) error {
	_, err := s.db.NamedExecContext(ctx, `
		INSERT INTO notes (id, notebook_id, title, content_html, content_text, tags, created_time, modified_time, synced_at)
		VALUES (:id, :notebook_id, :title, :content_html, :content_text, :tags, :created_time, :modified_time, :synced_at)
		ON CONFLICT(id) DO UPDATE SET
			notebook_id    = excluded.notebook_id,
			title          = excluded.title,
			content_html   = excluded.content_html,
			content_text   = excluded.content_text,
			tags           = excluded.tags,
			created_time   = excluded.created_time,
			modified_time  = excluded.modified_time,
			synced_at      = excluded.synced_at
	`, note)
	if err != nil {
		return fmt.Errorf("save note %s: %w", note.ID, err)
	}
	return nil
}

// GetNote retrieves a single note by its ID.
func (s *NotesStore) GetNote(ctx context.Context, id string) (*Note, error) {
	var note Note
	if err := s.db.GetContext(ctx, &note, "SELECT * FROM notes WHERE id = ?", id); err != nil {
		return nil, fmt.Errorf("get note %s: %w", id, err)
	}
	return &note, nil
}

// ListNotes returns notes in a notebook with pagination.
// If notebookID is empty, notes across all notebooks are returned.
func (s *NotesStore) ListNotes(ctx context.Context, notebookID string, offset, limit int) ([]Note, error) {
	var notes []Note
	var err error
	if notebookID != "" {
		err = s.db.SelectContext(ctx, &notes,
			"SELECT * FROM notes WHERE notebook_id = ? ORDER BY modified_time DESC LIMIT ? OFFSET ?",
			notebookID, limit, offset)
	} else {
		err = s.db.SelectContext(ctx, &notes,
			"SELECT * FROM notes ORDER BY modified_time DESC LIMIT ? OFFSET ?",
			limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	return notes, nil
}

// DeleteNote removes a note by ID.
func (s *NotesStore) DeleteNote(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM notes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete note %s: %w", id, err)
	}
	return nil
}

// ListNotebooks returns all notebooks.
func (s *NotesStore) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	var notebooks []Notebook
	if err := s.db.SelectContext(ctx, &notebooks, "SELECT * FROM notebooks ORDER BY title"); err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}
	return notebooks, nil
}

// SaveNotebook inserts or updates a notebook by ID (upsert).
func (s *NotesStore) SaveNotebook(ctx context.Context, nb *Notebook) error {
	_, err := s.db.NamedExecContext(ctx, `
		INSERT INTO notebooks (id, title, parent_id, created_time, modified_time)
		VALUES (:id, :title, :parent_id, :created_time, :modified_time)
		ON CONFLICT(id) DO UPDATE SET
			title          = excluded.title,
			parent_id      = excluded.parent_id,
			created_time   = excluded.created_time,
			modified_time  = excluded.modified_time
	`, nb)
	if err != nil {
		return fmt.Errorf("save notebook %s: %w", nb.ID, err)
	}
	return nil
}

// DeleteNotebook removes a notebook by ID.
func (s *NotesStore) DeleteNotebook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM notebooks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete notebook %s: %w", id, err)
	}
	return nil
}
