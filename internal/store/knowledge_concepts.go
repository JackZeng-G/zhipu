package store

import (
	"context"
	"fmt"
	"time"
)

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
	type noteConcept struct {
		NoteID     string `db:"note_id"`
		EntityName string `db:"entity_name"`
	}
	var pairs []noteConcept
	if err := s.db.SelectContext(ctx, &pairs,
		"SELECT note_id, entity_name FROM note_entities ORDER BY note_id, entity_name"); err != nil {
		return err
	}

	noteConcepts := make(map[string][]string)
	for _, p := range pairs {
		noteConcepts[p.NoteID] = append(noteConcepts[p.NoteID], p.EntityName)
	}

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

	s.db.ExecContext(ctx, "DELETE FROM concept_relations")

	for p, count := range coOcc {
		slugA := Slugify(p.a)
		slugB := Slugify(p.b)
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

// UpdateConceptRelationSlugs replaces all references to oldSlug with newSlug.
// It deletes rows that would create duplicates after the update.
func (s *KnowledgeStore) UpdateConceptRelationSlugs(ctx context.Context, oldSlug, newSlug string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM concept_relations WHERE source_concept_slug = ? AND target_concept_slug IN (SELECT target_concept_slug FROM concept_relations WHERE source_concept_slug = ?)",
		newSlug, oldSlug)
	if err != nil {
		return fmt.Errorf("deduplicate source relations: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		"DELETE FROM concept_relations WHERE target_concept_slug = ? AND source_concept_slug IN (SELECT source_concept_slug FROM concept_relations WHERE target_concept_slug = ?)",
		newSlug, oldSlug)
	if err != nil {
		return fmt.Errorf("deduplicate target relations: %w", err)
	}

	_, err = s.db.ExecContext(ctx, "UPDATE concept_relations SET source_concept_slug = ? WHERE source_concept_slug = ?",
		newSlug, oldSlug)
	if err != nil {
		return fmt.Errorf("update source slug: %w", err)
	}
	_, err = s.db.ExecContext(ctx, "UPDATE concept_relations SET target_concept_slug = ? WHERE target_concept_slug = ?",
		newSlug, oldSlug)
	if err != nil {
		return fmt.Errorf("update target slug: %w", err)
	}
	return nil
}
