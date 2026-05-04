package store

import (
	"context"
	"fmt"
	"time"
)

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
