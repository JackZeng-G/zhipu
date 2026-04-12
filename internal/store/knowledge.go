package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
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

// NoteSummary is a persisted AI-generated summary for a note.
type NoteSummary struct {
	ID          int    `db:"id" json:"id"`
	NoteID      string `db:"note_id" json:"note_id"`
	Summary     string `db:"summary" json:"summary"`
	KeyPoints   string `db:"key_points" json:"key_points"`
	GeneratedAt int64  `db:"generated_at" json:"generated_at"`
}

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

// ActivityLogEntry represents a logged activity.
type ActivityLogEntry struct {
	ID           int    `db:"id" json:"id"`
	ActivityType string `db:"activity_type" json:"activity_type"`
	TargetType   string `db:"target_type" json:"target_type"`
	TargetID     string `db:"target_id" json:"target_id"`
	Description  string `db:"description" json:"description"`
	Metadata     string `db:"metadata" json:"metadata"`
	CreatedAt    int64  `db:"created_at" json:"created_at"`
}

// WikiPage represents a legacy wiki page (kept for backward compatibility).
type WikiPage struct {
	ID            int    `db:"id" json:"id"`
	Slug          string `db:"slug" json:"slug"`
	Title         string `db:"title" json:"title"`
	Content       string `db:"content" json:"content"`
	SourceNoteIDs string `db:"source_note_ids" json:"source_note_ids"`
	PageType      string `db:"page_type" json:"page_type"`
	GeneratedAt   int64  `db:"generated_at" json:"generated_at"`
	UpdatedAt     int64  `db:"updated_at" json:"updated_at"`
}

// WikiConcept represents a concept page in the knowledge base.
// This is the CORE unit: each concept aggregates knowledge from ALL notes mentioning it.
type WikiConcept struct {
	ID           int    `db:"id" json:"id"`
	Slug         string `db:"slug" json:"slug"`
	Title        string `db:"title" json:"title"`
	Aliases      string `db:"aliases" json:"aliases"`
	Definition   string `db:"definition" json:"definition"`
	KeyPoints    string `db:"key_points" json:"key_points"`
	Content      string `db:"content" json:"content"`
	NoteIDs      string `db:"note_ids" json:"note_ids"`
	SourceCount  int    `db:"source_count" json:"source_count"`
	Confidence   string `db:"confidence" json:"confidence"`
	EvolutionLog   string `db:"evolution_log" json:"evolution_log"`
	Contradictions string `db:"contradictions" json:"contradictions"`
	CreatedAt    int64  `db:"created_at" json:"created_at"`
	UpdatedAt    int64  `db:"updated_at" json:"updated_at"`
	DomainVolatility  string `db:"domain_volatility" json:"domain_volatility"`
	LastReviewed      *int64 `db:"last_reviewed" json:"last_reviewed"`
	ConfidencePending int    `db:"confidence_pending" json:"confidence_pending"`
	RedirectTo        string `db:"redirect_to" json:"redirect_to"`
}

// ConceptSummary is a lightweight subset of WikiConcept used for list views.
// It excludes heavy columns (content, key_points, aliases, note_ids,
// evolution_log, contradictions) to keep response payloads small.
type ConceptSummary struct {
	ID          int    `db:"id" json:"id"`
	Slug        string `db:"slug" json:"slug"`
	Title       string `db:"title" json:"title"`
	Definition  string `db:"definition" json:"definition"`
	SourceCount int    `db:"source_count" json:"source_count"`
	Confidence  string `db:"confidence" json:"confidence"`
	ConfidencePending int    `db:"confidence_pending" json:"confidence_pending"`
	UpdatedAt   int64  `db:"updated_at" json:"updated_at"`
}

// ConceptRelation represents an edge in the knowledge graph between two concepts.
type ConceptRelation struct {
	ID                 int    `db:"id" json:"id"`
	SourceConceptSlug  string `db:"source_concept_slug" json:"source_concept_slug"`
	TargetConceptSlug  string `db:"target_concept_slug" json:"target_concept_slug"`
	RelationType       string `db:"relation_type" json:"relation_type"`
	Reason             string `db:"reason" json:"reason"`
	CoOccurrenceCount  int    `db:"co_occurrence_count" json:"co_occurrence_count"`
	CreatedAt          int64  `db:"created_at" json:"created_at"`
}

// KnowledgeStore provides CRUD for knowledge index data.
type KnowledgeStore struct {
	db *sqlx.DB
}

// NewKnowledgeStore creates a new knowledge store.
func NewKnowledgeStore(db *sqlx.DB) *KnowledgeStore {
	return &KnowledgeStore{db: db}
}

// === Entities ===

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

// === Summaries ===

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

// === Relations ===

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

// === Activity Log ===

// LogActivity writes an activity log entry.
func (s *KnowledgeStore) LogActivity(ctx context.Context, activityType, targetType, targetID, description string, metadata map[string]interface{}) error {
	metaJSON, _ := json.Marshal(metadata)
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO activity_log (activity_type, target_type, target_id, description, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		activityType, targetType, targetID, description, string(metaJSON), now)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}

// GetRecentActivities returns the most recent N activity log entries.
func (s *KnowledgeStore) GetRecentActivities(ctx context.Context, limit int) ([]ActivityLogEntry, error) {
	var entries []ActivityLogEntry
	if err := s.db.SelectContext(ctx, &entries,
		"SELECT * FROM activity_log ORDER BY created_at DESC LIMIT ?", limit); err != nil {
		return nil, fmt.Errorf("get recent activities: %w", err)
	}
	return entries, nil
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

// CountEntities returns the total number of entities.
func (s *KnowledgeStore) CountEntities(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM note_entities"); err != nil {
		return 0, err
	}
	return count, nil
}

// CountRelations returns the total number of note relations.
func (s *KnowledgeStore) CountRelations(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM note_relations"); err != nil {
		return 0, err
	}
	return count, nil
}

// === Index Metadata ===

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

// === Legacy Wiki Pages (kept for backward compatibility) ===

// SaveWikiPage upserts a wiki page by slug.
func (s *KnowledgeStore) SaveWikiPage(ctx context.Context, page *WikiPage) error {
	now := time.Now().Unix()
	page.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wiki_pages (slug, title, content, source_note_ids, page_type, generated_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET title=excluded.title, content=excluded.content, source_note_ids=excluded.source_note_ids, page_type=excluded.page_type, updated_at=excluded.updated_at`,
		page.Slug, page.Title, page.Content, page.SourceNoteIDs, page.PageType, now, now)
	if err != nil {
		return fmt.Errorf("save wiki page: %w", err)
	}
	return nil
}

// GetWikiPage returns a wiki page by slug.
func (s *KnowledgeStore) GetWikiPage(ctx context.Context, slug string) (*WikiPage, error) {
	var page WikiPage
	if err := s.db.GetContext(ctx, &page,
		"SELECT * FROM wiki_pages WHERE slug = ?", slug); err != nil {
		return nil, fmt.Errorf("get wiki page %s: %w", slug, err)
	}
	return &page, nil
}

// ListWikiPages returns all wiki pages.
func (s *KnowledgeStore) ListWikiPages(ctx context.Context) ([]WikiPage, error) {
	var pages []WikiPage
	if err := s.db.SelectContext(ctx, &pages,
		"SELECT * FROM wiki_pages ORDER BY page_type, title"); err != nil {
		return nil, fmt.Errorf("list wiki pages: %w", err)
	}
	return pages, nil
}

// DeleteWikiPage deletes a wiki page by slug.
func (s *KnowledgeStore) DeleteWikiPage(ctx context.Context, slug string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM wiki_pages WHERE slug = ?", slug)
	return err
}

// === Wiki Concepts (Knowledge Graph Core) ===

// SaveWikiConcept upserts a concept page by slug.
func (s *KnowledgeStore) SaveWikiConcept(ctx context.Context, c *WikiConcept) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wiki_concepts (slug, title, aliases, definition, key_points, content, note_ids, source_count, confidence, evolution_log, contradictions, created_at, updated_at, domain_volatility, last_reviewed, confidence_pending, redirect_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title=excluded.title, aliases=excluded.aliases, definition=excluded.definition,
			key_points=excluded.key_points, content=excluded.content, note_ids=excluded.note_ids,
			source_count=excluded.source_count, confidence=excluded.confidence,
			evolution_log=excluded.evolution_log, contradictions=excluded.contradictions, updated_at=excluded.updated_at,
			domain_volatility=excluded.domain_volatility, last_reviewed=excluded.last_reviewed,
			confidence_pending=excluded.confidence_pending, redirect_to=excluded.redirect_to`,
		c.Slug, c.Title, c.Aliases, c.Definition, c.KeyPoints, c.Content,
		c.NoteIDs, c.SourceCount, c.Confidence, c.EvolutionLog, c.Contradictions, now, now,
		c.DomainVolatility, c.LastReviewed, c.ConfidencePending, c.RedirectTo)
	if err != nil {
		return fmt.Errorf("save wiki concept: %w", err)
	}
	return nil
}

// GetWikiConcept returns a concept page by slug.
func (s *KnowledgeStore) GetWikiConcept(ctx context.Context, slug string) (*WikiConcept, error) {
	var c WikiConcept
	if err := s.db.GetContext(ctx, &c,
		"SELECT * FROM wiki_concepts WHERE slug = ?", slug); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListWikiConcepts returns all concept pages ordered by source_count.
func (s *KnowledgeStore) ListWikiConcepts(ctx context.Context) ([]WikiConcept, error) {
	var concepts []WikiConcept
	if err := s.db.SelectContext(ctx, &concepts,
		"SELECT * FROM wiki_concepts ORDER BY source_count DESC, title"); err != nil {
		return nil, err
	}
	return concepts, nil
}

// ListConceptSummaries returns lightweight concept summaries (no content/key_points/etc.) ordered by source_count.
func (s *KnowledgeStore) ListConceptSummaries(ctx context.Context) ([]ConceptSummary, error) {
	var summaries []ConceptSummary
	if err := s.db.SelectContext(ctx, &summaries,
		"SELECT id, slug, title, definition, source_count, confidence, confidence_pending, updated_at FROM wiki_concepts ORDER BY source_count DESC, title"); err != nil {
		return nil, err
	}
	return summaries, nil
}

// DeleteWikiConcept deletes a concept page by slug.
func (s *KnowledgeStore) DeleteWikiConcept(ctx context.Context, slug string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM wiki_concepts WHERE slug = ?", slug)
	return err
}

// CountWikiConcepts returns the total number of concept pages.
func (s *KnowledgeStore) CountWikiConcepts(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM wiki_concepts"); err != nil {
		return 0, err
	}
	return count, nil
}

// === Concept Relations (Knowledge Graph Edges) ===

// SaveConceptRelation upserts a concept-to-concept relation.
func (s *KnowledgeStore) SaveConceptRelation(ctx context.Context, r *ConceptRelation) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO concept_relations (source_concept_slug, target_concept_slug, relation_type, reason, co_occurrence_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_concept_slug, target_concept_slug) DO UPDATE SET
			relation_type=excluded.relation_type, reason=excluded.reason,
			co_occurrence_count=co_occurrence_count + 1`,
		r.SourceConceptSlug, r.TargetConceptSlug, r.RelationType, r.Reason, r.CoOccurrenceCount, now)
	if err != nil {
		return fmt.Errorf("save concept relation: %w", err)
	}
	return nil
}

// GetConceptRelations returns all relations involving a concept.
func (s *KnowledgeStore) GetConceptRelations(ctx context.Context, slug string) ([]ConceptRelation, error) {
	var rels []ConceptRelation
	if err := s.db.SelectContext(ctx, &rels,
		"SELECT * FROM concept_relations WHERE source_concept_slug = ? OR target_concept_slug = ? ORDER BY co_occurrence_count DESC",
		slug, slug); err != nil {
		return nil, err
	}
	return rels, nil
}

// ListAllConceptRelations returns all concept relations.
func (s *KnowledgeStore) ListAllConceptRelations(ctx context.Context) ([]ConceptRelation, error) {
	var rels []ConceptRelation
	if err := s.db.SelectContext(ctx, &rels,
		"SELECT * FROM concept_relations ORDER BY co_occurrence_count DESC"); err != nil {
		return nil, err
	}
	return rels, nil
}

// BuildCoOccurrenceRelations builds concept relations based on co-occurrence in notes.
// Two concepts are related if they appear in the same note.
func (s *KnowledgeStore) BuildCoOccurrenceRelations(ctx context.Context) error {
	// Get all entities grouped by note_id
	type noteConcept struct {
		NoteID     string `db:"note_id"`
		EntityName string `db:"entity_name"`
	}
	var pairs []noteConcept
	if err := s.db.SelectContext(ctx, &pairs,
		"SELECT note_id, entity_name FROM note_entities ORDER BY note_id, entity_name"); err != nil {
		return err
	}

	// Group concepts by note
	noteConcepts := make(map[string][]string)
	for _, p := range pairs {
		noteConcepts[p.NoteID] = append(noteConcepts[p.NoteID], p.EntityName)
	}

	// Count co-occurrences
	type pair struct {
		a, b string
	}
	coOcc := make(map[pair]int)
	for _, concepts := range noteConcepts {
		for i := 0; i < len(concepts); i++ {
			for j := i + 1; j < len(concepts); j++ {
				a, b := concepts[i], concepts[j]
				if a > b {
					a, b = b, a
				}
				coOcc[pair{a, b}]++
			}
		}
	}

	// Clear and rebuild relations
	s.db.ExecContext(ctx, "DELETE FROM concept_relations")

	// Save relations with co-occurrence >= 1
	for p, count := range coOcc {
		slugA := slugifyConcept(p.a)
		slugB := slugifyConcept(p.b)
		if slugA == "" || slugB == "" || slugA == slugB {
			continue
		}
		rel := &ConceptRelation{
			SourceConceptSlug: slugA,
			TargetConceptSlug: slugB,
			RelationType:      "co_occurs",
			Reason:            fmt.Sprintf("共同出现在 %d 篇笔记中", count),
			CoOccurrenceCount: count,
		}
		s.SaveConceptRelation(ctx, rel)
	}

	return nil
}

// slugifyConcept converts a concept name to a URL-friendly slug.
func slugifyConcept(s string) string {
	// Simple slug: lowercase, replace spaces/special chars with hyphens
	result := make([]byte, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			result = append(result, byte(r))
		} else if r >= 'A' && r <= 'Z' {
			result = append(result, byte(r+32))
		} else if r >= 0x4e00 && r <= 0x9fff {
			// Keep Chinese characters
			result = append(result, []byte(string(r))...)
		} else {
			if len(result) > 0 && result[len(result)-1] != '-' {
				result = append(result, '-')
			}
		}
	}
	// Trim trailing hyphen
	if len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	if len(result) > 100 {
		result = result[:100]
	}
	return string(result)
}

// === Wiki Outputs ===

// WikiOutput represents a persisted query answer or analysis result.
type WikiOutput struct {
	ID              int    `db:"id" json:"id"`
	Slug            string `db:"slug" json:"slug"`
	Title           string `db:"title" json:"title"`
	Content         string `db:"content" json:"content"`
	OutputType      string `db:"output_type" json:"output_type"`
	SourceConcepts  string `db:"source_concepts" json:"source_concepts"`
	ConfidenceNotes string `db:"confidence_notes" json:"confidence_notes"`
	CreatedAt       int64  `db:"created_at" json:"created_at"`
	UpdatedAt       int64  `db:"updated_at" json:"updated_at"`
}

// SaveWikiOutput upserts an output by slug.
func (s *KnowledgeStore) SaveWikiOutput(ctx context.Context, o *WikiOutput) error {
	now := time.Now().Unix()
	o.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wiki_outputs (slug, title, content, output_type, source_concepts, confidence_notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title=excluded.title, content=excluded.content, output_type=excluded.output_type,
			source_concepts=excluded.source_concepts, confidence_notes=excluded.confidence_notes, updated_at=excluded.updated_at`,
		o.Slug, o.Title, o.Content, o.OutputType, o.SourceConcepts, o.ConfidenceNotes, now, now)
	if err != nil {
		return fmt.Errorf("save wiki output: %w", err)
	}
	return nil
}

// GetWikiOutput returns an output by slug.
func (s *KnowledgeStore) GetWikiOutput(ctx context.Context, slug string) (*WikiOutput, error) {
	var o WikiOutput
	if err := s.db.GetContext(ctx, &o, "SELECT * FROM wiki_outputs WHERE slug = ?", slug); err != nil {
		return nil, err
	}
	return &o, nil
}

// ListWikiOutputs returns all outputs, optionally filtered by type.
func (s *KnowledgeStore) ListWikiOutputs(ctx context.Context, outputType string) ([]WikiOutput, error) {
	var outputs []WikiOutput
	query := "SELECT * FROM wiki_outputs"
	var args []interface{}
	if outputType != "" {
		query += " WHERE output_type = ?"
		args = append(args, outputType)
	}
	query += " ORDER BY created_at DESC"
	if err := s.db.SelectContext(ctx, &outputs, query, args...); err != nil {
		return nil, err
	}
	return outputs, nil
}

// DeleteWikiOutput deletes an output by slug.
func (s *KnowledgeStore) DeleteWikiOutput(ctx context.Context, slug string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM wiki_outputs WHERE slug = ?", slug)
	return err
}

// CountWikiOutputs returns the total number of outputs.
func (s *KnowledgeStore) CountWikiOutputs(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM wiki_outputs"); err != nil {
		return 0, err
	}
	return count, nil
}

// === Wiki Synthesis ===

// SaveWikiSynthesis upserts a synthesis page.
func (s *KnowledgeStore) SaveWikiSynthesis(ctx context.Context, syn *WikiSynthesis) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wiki_synthesis (slug, title, content, concept_slugs, confidence_notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title=excluded.title, content=excluded.content, concept_slugs=excluded.concept_slugs,
			confidence_notes=excluded.confidence_notes, updated_at=excluded.updated_at`,
		syn.Slug, syn.Title, syn.Content, syn.ConceptSlugs, syn.ConfidenceNotes, now, now)
	if err != nil {
		return fmt.Errorf("save wiki synthesis: %w", err)
	}
	return nil
}

// WikiSynthesis represents a cross-concept synthesis page.
type WikiSynthesis struct {
	ID              int    `db:"id" json:"id"`
	Slug            string `db:"slug" json:"slug"`
	Title           string `db:"title" json:"title"`
	Content         string `db:"content" json:"content"`
	ConceptSlugs    string `db:"concept_slugs" json:"concept_slugs"`
	ConfidenceNotes string `db:"confidence_notes" json:"confidence_notes"`
	CreatedAt       int64  `db:"created_at" json:"created_at"`
	UpdatedAt       int64  `db:"updated_at" json:"updated_at"`
}

// === Questions ===

// SaveQuestion creates a new question.
func (s *KnowledgeStore) SaveQuestion(ctx context.Context, content string) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO questions (content, status, opened_at) VALUES (?, 'open', ?)",
		content, now)
	if err != nil {
		return 0, fmt.Errorf("save question: %w", err)
	}
	return res.LastInsertId()
}

// ListOpenQuestions returns all open questions.
func (s *KnowledgeStore) ListOpenQuestions(ctx context.Context) ([]Question, error) {
	var qs []Question
	if err := s.db.SelectContext(ctx, &qs,
		"SELECT * FROM questions WHERE status = 'open' ORDER BY opened_at DESC"); err != nil {
		return nil, err
	}
	return qs, nil
}

// ListAllQuestions returns all questions, optionally filtered by status.
func (s *KnowledgeStore) ListAllQuestions(ctx context.Context, status string) ([]Question, error) {
	var qs []Question
	query := "SELECT * FROM questions"
	var args []interface{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY opened_at DESC"
	if err := s.db.SelectContext(ctx, &qs, query, args...); err != nil {
		return nil, err
	}
	return qs, nil
}

// ResolveQuestion marks a question as answered.
func (s *KnowledgeStore) ResolveQuestion(ctx context.Context, id int, outputSlug string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		"UPDATE questions SET status = 'answered', answered_at = ?, answer_synthesis_slug = ? WHERE id = ?",
		now, outputSlug, id)
	return err
}

// Question represents an open question in the knowledge base.
type Question struct {
	ID                   int    `db:"id" json:"id"`
	Content              string `db:"content" json:"content"`
	Status               string `db:"status" json:"status"`
	OpenedAt             int64  `db:"opened_at" json:"opened_at"`
	AnsweredAt           *int64 `db:"answered_at" json:"answered_at"`
	AnswerSynthesisSlug  string `db:"answer_synthesis_slug" json:"answer_synthesis_slug"`
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

// === Categories (Note Classification Tree) ===

// Category represents a node in the classification tree.
type Category struct {
	ID        int64  `db:"id" json:"id"`
	ParentID  *int64 `db:"parent_id" json:"parent_id"`
	Name      string `db:"name" json:"name"`
	Path      string `db:"path" json:"path"`
	NoteCount int    `db:"note_count" json:"note_count"`
	Depth     int    `db:"depth" json:"depth"`
	CreatedAt int64  `db:"created_at" json:"created_at"`
	UpdatedAt int64  `db:"updated_at" json:"updated_at"`
	// Computed fields for tree building
	Children []Category `db:"-" json:"children,omitempty"`
}

// NoteCategoryMapping represents a note-category mapping.
type NoteCategoryMapping struct {
	ID         int64  `db:"id" json:"id"`
	NoteID     string `db:"note_id" json:"note_id"`
	CategoryID int64  `db:"category_id" json:"category_id"`
	IsPrimary  bool   `db:"is_primary" json:"is_primary"`
}

// SaveCategory inserts a new category and returns its ID.
func (s *KnowledgeStore) SaveCategory(ctx context.Context, cat *Category) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO categories (parent_id, name, path, note_count, depth, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cat.ParentID, cat.Name, cat.Path, cat.NoteCount, cat.Depth, now, now)
	if err != nil {
		return 0, fmt.Errorf("save category: %w", err)
	}
	return res.LastInsertId()
}

// GetCategoryTree returns all categories ordered by path.
func (s *KnowledgeStore) GetCategoryTree(ctx context.Context) ([]Category, error) {
	var cats []Category
	if err := s.db.SelectContext(ctx, &cats,
		"SELECT * FROM categories ORDER BY path"); err != nil {
		return nil, fmt.Errorf("get category tree: %w", err)
	}
	return cats, nil
}

// MapNoteToCategory maps a note to a category.
func (s *KnowledgeStore) MapNoteToCategory(ctx context.Context, noteID string, categoryID int64, isPrimary bool) error {
	primary := 0
	if isPrimary {
		primary = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO note_categories (note_id, category_id, is_primary) VALUES (?, ?, ?)`,
		noteID, categoryID, primary)
	if err != nil {
		return fmt.Errorf("map note to category: %w", err)
	}
	return nil
}

// GetNotesByCategory returns all note IDs in a category (including sub-categories).
func (s *KnowledgeStore) GetNotesByCategory(ctx context.Context, categoryID int) ([]string, error) {
	// First get the path of the target category
	var path string
	if err := s.db.GetContext(ctx, &path, "SELECT path FROM categories WHERE id = ?", categoryID); err != nil {
		return nil, fmt.Errorf("get category path: %w", err)
	}

	// Get all category IDs that are this category or its children
	var catIDs []int64
	if err := s.db.SelectContext(ctx, &catIDs,
		"SELECT id FROM categories WHERE path = ? OR path LIKE ?", path, path+"/%"); err != nil {
		return nil, fmt.Errorf("get sub-categories: %w", err)
	}

	if len(catIDs) == 0 {
		return nil, nil
	}

	// Get note IDs for all these categories
	query, args, _ := sqlx.In("SELECT DISTINCT note_id FROM note_categories WHERE category_id IN (?)", catIDs)
	var noteIDs []string
	if err := s.db.SelectContext(ctx, &noteIDs, query, args...); err != nil {
		return nil, fmt.Errorf("get notes by category: %w", err)
	}
	return noteIDs, nil
}

// GetCategoriesByNote returns all categories for a note.
func (s *KnowledgeStore) GetCategoriesByNote(ctx context.Context, noteID string) ([]Category, error) {
	var cats []Category
	if err := s.db.SelectContext(ctx, &cats,
		`SELECT c.* FROM categories c
		JOIN note_categories nc ON c.id = nc.category_id
		WHERE nc.note_id = ?
		ORDER BY nc.is_primary DESC, c.path`, noteID); err != nil {
		return nil, fmt.Errorf("get categories for note %s: %w", noteID, err)
	}
	return cats, nil
}

// SearchCategories searches categories by name or path.
func (s *KnowledgeStore) SearchCategories(ctx context.Context, query string) ([]Category, error) {
	var cats []Category
	pattern := "%" + query + "%"
	if err := s.db.SelectContext(ctx, &cats,
		"SELECT * FROM categories WHERE name LIKE ? OR path LIKE ? ORDER BY path", pattern, pattern); err != nil {
		return nil, fmt.Errorf("search categories: %w", err)
	}
	return cats, nil
}

// SearchNotesByCategoryQuery searches for notes matching a query via category index.
// Returns note IDs from matching categories.
func (s *KnowledgeStore) SearchNotesByCategoryQuery(ctx context.Context, query string) ([]string, error) {
	pattern := "%" + query + "%"
	// Get category IDs matching the query
	var catIDs []int64
	if err := s.db.SelectContext(ctx, &catIDs,
		"SELECT id FROM categories WHERE name LIKE ? OR path LIKE ?", pattern, pattern); err != nil {
		return nil, err
	}
	if len(catIDs) == 0 {
		return nil, nil
	}

	// Also get sub-categories
	var allCatIDs []int64
	for _, id := range catIDs {
		var path string
		if err := s.db.GetContext(ctx, &path, "SELECT path FROM categories WHERE id = ?", id); err != nil {
			continue
		}
		var subIDs []int64
		s.db.SelectContext(ctx, &subIDs, "SELECT id FROM categories WHERE path LIKE ?", path+"/%")
		allCatIDs = append(allCatIDs, id)
		allCatIDs = append(allCatIDs, subIDs...)
	}

	sqlQuery, args, _ := sqlx.In("SELECT DISTINCT note_id FROM note_categories WHERE category_id IN (?)", allCatIDs)
	var noteIDs []string
	if err := s.db.SelectContext(ctx, &noteIDs, sqlQuery, args...); err != nil {
		return nil, err
	}
	return noteIDs, nil
}

// RebuildNoteCounts recalculates note_count for all categories.
func (s *KnowledgeStore) RebuildNoteCounts(ctx context.Context) error {
	cats, err := s.GetCategoryTree(ctx)
	if err != nil {
		return err
	}
	for _, cat := range cats {
		// Count notes directly in this category + sub-categories
		var count int
		s.db.GetContext(ctx, &count,
			`SELECT COUNT(DISTINCT nc.note_id) FROM note_categories nc
			JOIN categories c ON nc.category_id = c.id
			WHERE c.path = ? OR c.path LIKE ?`,
			cat.Path, cat.Path+"/%")
		s.db.ExecContext(ctx, "UPDATE categories SET note_count = ? WHERE id = ?", count, cat.ID)
	}
	return nil
}

// DeleteAllCategories removes all category data.
func (s *KnowledgeStore) DeleteAllCategories(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM note_categories"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM categories"); err != nil {
		return err
	}
	return nil
}

// CountCategories returns the total number of categories.
func (s *KnowledgeStore) CountCategories(ctx context.Context) (int, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM categories"); err != nil {
		return 0, err
	}
	return count, nil
}
