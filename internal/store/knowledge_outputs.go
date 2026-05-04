package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

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

// Question represents an open question in the knowledge base.
type Question struct {
	ID                   int    `db:"id" json:"id"`
	Content              string `db:"content" json:"content"`
	Status               string `db:"status" json:"status"`
	OpenedAt             int64  `db:"opened_at" json:"opened_at"`
	AnsweredAt           *int64 `db:"answered_at" json:"answered_at"`
	AnswerSynthesisSlug  string `db:"answer_synthesis_slug" json:"answer_synthesis_slug"`
}

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
	Children  []Category `db:"-" json:"children,omitempty"`
}

// NoteCategoryMapping represents a note-category mapping.
type NoteCategoryMapping struct {
	ID         int64  `db:"id" json:"id"`
	NoteID     string `db:"note_id" json:"note_id"`
	CategoryID int64  `db:"category_id" json:"category_id"`
	IsPrimary  bool   `db:"is_primary" json:"is_primary"`
}

// === Wiki Outputs ===

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

// === Categories (Note Classification Tree) ===

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
	var path string
	if err := s.db.GetContext(ctx, &path, "SELECT path FROM categories WHERE id = ?", categoryID); err != nil {
		return nil, fmt.Errorf("get category path: %w", err)
	}

	var catIDs []int64
	if err := s.db.SelectContext(ctx, &catIDs,
		"SELECT id FROM categories WHERE path = ? OR path LIKE ?", path, path+"/%"); err != nil {
		return nil, fmt.Errorf("get sub-categories: %w", err)
	}

	if len(catIDs) == 0 {
		return nil, nil
	}

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
	var catIDs []int64
	if err := s.db.SelectContext(ctx, &catIDs,
		"SELECT id FROM categories WHERE name LIKE ? OR path LIKE ?", pattern, pattern); err != nil {
		return nil, err
	}
	if len(catIDs) == 0 {
		return nil, nil
	}

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
