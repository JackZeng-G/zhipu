package store

import (
	"context"
	"fmt"
	"time"
)

// NoteRelation represents a relationship between two notes.
type NoteRelation struct {
	ID           int     `db:"id" json:"id"`
	SourceNoteID string  `db:"source_note_id" json:"source_note_id"`
	TargetNoteID string  `db:"target_note_id" json:"target_note_id"`
	RelationType string  `db:"relation_type" json:"relation_type"`
	Reason       string  `db:"reason" json:"reason"`
	Confidence   float64 `db:"confidence" json:"confidence"`
	CreatedAt    int64   `db:"created_at" json:"created_at"`
}

// SaveRelations replaces all relations where the note is the source.
func (s *KnowledgeStore) SaveRelations(ctx context.Context, noteID string, relations []NoteRelation) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM note_relations WHERE source_note_id = ?", noteID); err != nil {
		return fmt.Errorf("delete old relations: %w", err)
	}
	now := time.Now().Unix()
	for _, r := range relations {
		_, err := s.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO note_relations (source_note_id, target_note_id, relation_type, reason, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			r.SourceNoteID, r.TargetNoteID, r.RelationType, r.Reason, r.Confidence, now)
		if err != nil {
			return fmt.Errorf("save relation: %w", err)
		}
	}
	return nil
}

// GetRelatedNotes returns all notes related to the given note (in either direction).
func (s *KnowledgeStore) GetRelatedNotes(ctx context.Context, noteID string) ([]NoteRelation, error) {
	var relations []NoteRelation
	if err := s.db.SelectContext(ctx, &relations,
		"SELECT * FROM note_relations WHERE source_note_id = ? OR target_note_id = ? ORDER BY confidence DESC", noteID, noteID); err != nil {
		return nil, fmt.Errorf("get related notes for %s: %w", noteID, err)
	}
	return relations, nil
}

// GetAllRelations returns all note relations.
func (s *KnowledgeStore) GetAllRelations(ctx context.Context) ([]NoteRelation, error) {
	var relations []NoteRelation
	if err := s.db.SelectContext(ctx, &relations, "SELECT * FROM note_relations ORDER BY id"); err != nil {
		return nil, fmt.Errorf("get all relations: %w", err)
	}
	return relations, nil
}

// UpdateRelationReason updates the reason field of a relation.
func (s *KnowledgeStore) UpdateRelationReason(ctx context.Context, id int, reason string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE note_relations SET reason = ? WHERE id = ?", reason, id)
	return err
}

// CountRelations returns the total number of note relations.
func (s *KnowledgeStore) CountRelations(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM note_relations"); err != nil {
		return 0, err
	}
	return count, nil
}
