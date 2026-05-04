package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NoteSummary is a persisted AI-generated summary for a note.
type NoteSummary struct {
	ID          int    `db:"id" json:"id"`
	NoteID      string `db:"note_id" json:"note_id"`
	Summary     string `db:"summary" json:"summary"`
	KeyPoints   string `db:"key_points" json:"key_points"`
	GeneratedAt int64  `db:"generated_at" json:"generated_at"`
}

// SaveSummary upserts a summary for a note.
func (s *KnowledgeStore) SaveSummary(ctx context.Context, noteID, summary string, keyPoints []string) error {
	kpJSON, _ := json.Marshal(keyPoints)
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO note_summaries (note_id, summary, key_points, generated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(note_id) DO UPDATE SET summary=excluded.summary, key_points=excluded.key_points, generated_at=excluded.generated_at`,
		noteID, summary, string(kpJSON), now)
	if err != nil {
		return fmt.Errorf("save summary for note %s: %w", noteID, err)
	}
	return nil
}

// GetSummary returns the summary for a note.
func (s *KnowledgeStore) GetSummary(ctx context.Context, noteID string) (*NoteSummary, error) {
	var summary NoteSummary
	if err := s.db.GetContext(ctx, &summary,
		"SELECT * FROM note_summaries WHERE note_id = ?", noteID); err != nil {
		return nil, fmt.Errorf("get summary for note %s: %w", noteID, err)
	}
	return &summary, nil
}

// CountSummaries returns the number of notes with summaries.
func (s *KnowledgeStore) CountSummaries(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM note_summaries"); err != nil {
		return 0, err
	}
	return count, nil
}

// GetUnindexedNoteIDs returns IDs of notes that don't have a summary yet.
func (s *KnowledgeStore) GetUnindexedNoteIDs(ctx context.Context) ([]string, error) {
	var ids []string
	if err := s.db.SelectContext(ctx, &ids,
		"SELECT id FROM notes WHERE id NOT IN (SELECT note_id FROM note_summaries) ORDER BY modified_time DESC"); err != nil {
		return nil, fmt.Errorf("get unindexed note IDs: %w", err)
	}
	return ids, nil
}
