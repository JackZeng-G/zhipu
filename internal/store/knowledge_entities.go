package store

import (
	"context"
	"fmt"
	"time"
)

// NoteEntity represents an extracted entity from a note.
type NoteEntity struct {
	ID          int    `db:"id" json:"id"`
	NoteID      string `db:"note_id" json:"note_id"`
	EntityType  string `db:"entity_type" json:"entity_type"`
	EntityName  string `db:"entity_name" json:"entity_name"`
	Description string `db:"description" json:"description"`
	CreatedAt   int64  `db:"created_at" json:"created_at"`
}

// SaveEntities replaces all entities for a note.
func (s *KnowledgeStore) SaveEntities(ctx context.Context, noteID string, entities []NoteEntity) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM note_entities WHERE note_id = ?", noteID); err != nil {
		return fmt.Errorf("delete old entities: %w", err)
	}
	now := time.Now().Unix()
	for _, e := range entities {
		_, err := s.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO note_entities (note_id, entity_type, entity_name, description, created_at) VALUES (?, ?, ?, ?, ?)",
			noteID, e.EntityType, e.EntityName, e.Description, now)
		if err != nil {
			return fmt.Errorf("save entity: %w", err)
		}
	}
	return nil
}

// GetEntitiesByNote returns all entities for a note.
func (s *KnowledgeStore) GetEntitiesByNote(ctx context.Context, noteID string) ([]NoteEntity, error) {
	var entities []NoteEntity
	if err := s.db.SelectContext(ctx, &entities,
		"SELECT * FROM note_entities WHERE note_id = ? ORDER BY entity_type, entity_name", noteID); err != nil {
		return nil, fmt.Errorf("get entities for note %s: %w", noteID, err)
	}
	return entities, nil
}

// SearchEntities searches entities by name (case-insensitive LIKE).
func (s *KnowledgeStore) SearchEntities(ctx context.Context, query string) ([]NoteEntity, error) {
	var entities []NoteEntity
	pattern := "%" + query + "%"
	if err := s.db.SelectContext(ctx, &entities,
		"SELECT * FROM note_entities WHERE entity_name LIKE ? ORDER BY entity_name", pattern); err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	return entities, nil
}

// GetNoteIDsByEntityName returns distinct note IDs that mention this exact entity.
func (s *KnowledgeStore) GetNoteIDsByEntityName(ctx context.Context, name string) ([]string, error) {
	var ids []string
	if err := s.db.SelectContext(ctx, &ids,
		"SELECT DISTINCT note_id FROM note_entities WHERE entity_name = ? ORDER BY note_id", name); err != nil {
		return nil, fmt.Errorf("get note IDs by entity %s: %w", name, err)
	}
	return ids, nil
}

// GetAllEntityNames returns distinct entity names with their types.
func (s *KnowledgeStore) GetAllEntityNames(ctx context.Context) ([]NoteEntity, error) {
	var entities []NoteEntity
	if err := s.db.SelectContext(ctx, &entities,
		"SELECT DISTINCT entity_type, entity_name FROM note_entities ORDER BY entity_type, entity_name"); err != nil {
		return nil, fmt.Errorf("get all entity names: %w", err)
	}
	return entities, nil
}

// CountEntities returns the total number of entities.
func (s *KnowledgeStore) CountEntities(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM note_entities"); err != nil {
		return 0, err
	}
	return count, nil
}
