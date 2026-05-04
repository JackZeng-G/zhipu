package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// KnowledgeStore provides CRUD for knowledge index data.
type KnowledgeStore struct {
	db *sqlx.DB
}

// NewKnowledgeStore creates a new knowledge store.
func NewKnowledgeStore(db *sqlx.DB) *KnowledgeStore {
	return &KnowledgeStore{db: db}
}

// SaveIndexMetadata records what model indexed what content.
func (s *KnowledgeStore) SaveIndexMetadata(ctx context.Context, entityType, targetID string, aiConfigID int, modelName, contentHash string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO index_metadata (entity_type, target_id, ai_config_id, model_name, generated_at, content_hash) VALUES (?, ?, ?, ?, ?, ?)`,
		entityType, targetID, aiConfigID, modelName, now, contentHash)
	if err != nil {
		return fmt.Errorf("save index metadata: %w", err)
	}
	return nil
}

// ResetAllIndexes deletes all index data: entities, summaries, relations, concepts, wiki pages.
func (s *KnowledgeStore) ResetAllIndexes(ctx context.Context) error {
	tables := []string{
		"note_entities",
		"note_summaries",
		"note_relations",
		"index_metadata",
		"wiki_concepts",
		"concept_relations",
		"wiki_pages",
		"wiki_synthesis",
		"questions",
		"activity_log",
		"note_categories",
		"categories",
	}
	for _, t := range tables {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return fmt.Errorf("clear table %s: %w", t, err)
		}
	}
	return nil
}
