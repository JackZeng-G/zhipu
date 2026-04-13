package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"personal-kb/internal/ai"
	"personal-kb/internal/store"
	"personal-kb/internal/util"

	"github.com/gin-gonic/gin"
)

// ListWikiPages returns all wiki pages.
func (h *Handlers) ListWikiPages(c *gin.Context) {
	ctx := context.Background()
	pages, err := h.knowledgeStore.ListWikiPages(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if pages == nil {
		pages = []store.WikiPage{}
	}
	c.JSON(http.StatusOK, gin.H{"pages": pages})
}

// GetWikiPage returns a single wiki page by slug.
func (h *Handlers) GetWikiPage(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	ctx := context.Background()
	page, err := h.knowledgeStore.GetWikiPage(ctx, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "wiki page not found"})
		return
	}
	c.JSON(http.StatusOK, page)
}

// buildWikiPrompt builds the note context and prompt for wiki page generation.
func buildWikiPrompt(title string, notes []store.Note) string {
	var noteContents strings.Builder
	for i, n := range notes {
		noteContents.WriteString(fmt.Sprintf("--- 笔记 %d: %s ---\n", i+1, n.Title))
		if n.ContentText != nil && *n.ContentText != "" {
			noteContents.WriteString(*n.ContentText)
		} else if n.ContentHTML != nil && *n.ContentHTML != "" {
			noteContents.WriteString(stripHTMLTags(*n.ContentHTML))
		}
		noteContents.WriteString("\n\n")
	}

	prompt := "你是一个知识管理助手。以下是与「" + title + "」相关的多条笔记内容，" +
		"请将它们汇聚整合为一篇完整的 Wiki 知识页面。\n\n" +
		"相关笔记：\n" + noteContents.String() + "\n" +
		"要求：\n" +
		"- 使用 Markdown 格式\n" +
		"- 第一行为一级标题作为页面标题\n" +
		"- 根据笔记内容自由组织结构，不要套用固定模板\n" +
		"- 充分整合所有笔记中的信息，保留关键细节和实际内容\n" +
		"- 如果有代码、命令或配置示例，用代码块展示\n" +
		"- 用中文撰写\n" +
		"- 只返回 Markdown 内容，不要额外说明"
	return prompt
}

// GenerateWikiPage generates a wiki page from notes or an entity.
func (h *Handlers) GenerateWikiPage(c *gin.Context) {
	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured"})
		return
	}

	var req struct {
		NoteIDs  []string `json:"note_ids"`
		Entity   string   `json:"entity"`
		PageType string   `json:"page_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	var notes []store.Note

	if req.Entity != "" {
		entities, err := h.knowledgeStore.SearchEntities(ctx, req.Entity)
		if err != nil || len(entities) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no notes found for entity: " + req.Entity})
			return
		}
		seen := make(map[string]bool)
		for _, e := range entities {
			if !seen[e.NoteID] {
				seen[e.NoteID] = true
				note, err := h.notesStore.GetNote(ctx, e.NoteID)
				if err == nil {
					notes = append(notes, *note)
				}
			}
		}
		if len(notes) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no notes found"})
			return
		}
	} else if len(req.NoteIDs) > 0 {
		for _, id := range req.NoteIDs {
			note, err := h.notesStore.GetNote(ctx, id)
			if err == nil {
				notes = append(notes, *note)
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide note_ids or entity"})
		return
	}

	pageType := req.PageType
	if pageType == "" {
		if req.Entity != "" {
			pageType = "concept"
		} else {
			pageType = "topic"
		}
	}

	title := req.Entity
	if title == "" && len(notes) > 0 {
		title = notes[0].Title
	}

	noteIDList := make([]string, 0, len(notes))
	for _, n := range notes {
		noteIDList = append(noteIDList, n.ID)
	}

	prompt := buildWikiPrompt(title, notes)

	content, err := provider.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed: " + err.Error()})
		return
	}

	// Extract title from first heading
	generatedTitle := title
	if lines := strings.Split(content, "\n"); len(lines) > 0 {
		firstLine := strings.TrimLeft(lines[0], "# ")
		if firstLine != "" {
			generatedTitle = firstLine
		}
	}

	slug := util.Slugify(generatedTitle)
	noteIDsJSON, _ := json.Marshal(noteIDList)

	page := &store.WikiPage{
		Slug:          slug,
		Title:         generatedTitle,
		Content:       content,
		SourceNoteIDs: string(noteIDsJSON),
		PageType:      pageType,
	}

	if err := h.knowledgeStore.SaveWikiPage(ctx, page); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed: " + err.Error()})
		return
	}

	h.knowledgeStore.LogActivity(ctx, "generate_wiki", "wiki", slug,
		fmt.Sprintf("generated wiki page: %s", generatedTitle),
		map[string]interface{}{"note_count": len(notes), "page_type": pageType})

	c.JSON(http.StatusOK, page)
}

// DeleteWikiPage deletes a wiki page.
func (h *Handlers) DeleteWikiPage(c *gin.Context) {
	slug := c.Param("slug")
	ctx := context.Background()
	if err := h.knowledgeStore.DeleteWikiPage(ctx, slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListEntities returns all distinct entity names from indexed notes.
func (h *Handlers) ListEntities(c *gin.Context) {
	ctx := context.Background()
	entities, err := h.knowledgeStore.GetAllEntityNames(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entities == nil {
		entities = []store.NoteEntity{}
	}
	c.JSON(http.StatusOK, gin.H{"entities": entities})
}

// GetWikiCatalog returns a catalog of all wiki pages with one-line summaries (index.md style).
func (h *Handlers) GetWikiCatalog(c *gin.Context) {
	ctx := context.Background()
	pages, err := h.knowledgeStore.ListWikiPages(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type CatalogEntry struct {
		Slug        string `json:"slug"`
		Title       string `json:"title"`
		PageType    string `json:"page_type"`
		Summary     string `json:"summary"`
		NoteCount   int    `json:"note_count"`
		UpdatedAt   int64  `json:"updated_at"`
	}

	entries := make([]CatalogEntry, 0, len(pages))
	for _, p := range pages {
		// Extract first line of content as summary (skip markdown heading)
		summary := ""
		lines := strings.Split(p.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Take first non-heading sentence as summary
			if len(line) > 150 {
				summary = line[:150] + "..."
			} else {
				summary = line
			}
			break
		}

		// Count source notes
		noteCount := 0
		if p.SourceNoteIDs != "" {
			var ids []string
			json.Unmarshal([]byte(p.SourceNoteIDs), &ids)
			noteCount = len(ids)
		}

		entries = append(entries, CatalogEntry{
			Slug:      p.Slug,
			Title:     p.Title,
			PageType:  p.PageType,
			Summary:   summary,
			NoteCount: noteCount,
			UpdatedAt: p.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(entries),
		"entries": entries,
	})
}

// AutoGenerateWiki scans entities and auto-generates wiki pages for the most common ones.
func (h *Handlers) AutoGenerateWiki(c *gin.Context) {
	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured"})
		return
	}

	go func() {
		ctx := context.Background()

		entities, err := h.knowledgeStore.GetAllEntityNames(ctx)
		if err != nil || len(entities) == 0 {
			log.Printf("[wiki] no entities found for auto-wiki")
			return
		}

		generated := 0
		for _, entity := range entities {
			matching, err := h.knowledgeStore.SearchEntities(ctx, entity.EntityName)
			if err != nil || len(matching) < 2 {
				continue
			}

			slug := util.Slugify(entity.EntityName)
			if _, err := h.knowledgeStore.GetWikiPage(ctx, slug); err == nil {
				continue
			}

			seen := make(map[string]bool)
			var notes []store.Note
			for _, e := range matching {
				if !seen[e.NoteID] {
					seen[e.NoteID] = true
					note, err := h.notesStore.GetNote(ctx, e.NoteID)
					if err == nil {
						notes = append(notes, *note)
					}
				}
			}
			if len(notes) == 0 {
				continue
			}

			noteIDList := make([]string, 0, len(notes))
			for _, n := range notes {
				noteIDList = append(noteIDList, n.ID)
			}

			prompt := buildWikiPrompt(entity.EntityName, notes)

			content, err := provider.Generate(ctx, prompt)
			if err != nil {
				log.Printf("[wiki] failed to generate page for %s: %v", entity.EntityName, err)
				continue
			}

			generatedTitle := entity.EntityName
			if lines := strings.Split(content, "\n"); len(lines) > 0 {
				firstLine := strings.TrimLeft(lines[0], "# ")
				if firstLine != "" {
					generatedTitle = firstLine
				}
			}

			noteIDsJSON, _ := json.Marshal(noteIDList)
			page := &store.WikiPage{
				Slug:          slug,
				Title:         generatedTitle,
				Content:       content,
				SourceNoteIDs: string(noteIDsJSON),
				PageType:      "concept",
			}

			if err := h.knowledgeStore.SaveWikiPage(ctx, page); err != nil {
				log.Printf("[wiki] failed to save page for %s: %v", entity.EntityName, err)
				continue
			}

			log.Printf("[wiki] auto-generated page for entity: %s (%d notes)", entity.EntityName, len(matching))
			generated++
		}

		h.knowledgeStore.LogActivity(ctx, "auto_wiki", "wiki", "",
			fmt.Sprintf("auto-wiki scan complete, generated %d pages", generated),
			map[string]interface{}{"generated": generated})
	}()

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "auto-wiki generation started in background"})
}

// === Concept-centric Wiki API ===

// ListConcepts returns lightweight concept summaries ordered by source_count.
// Only id, slug, title, definition, source_count, confidence, updated_at are returned
// to keep the response payload small for list views.
func (h *Handlers) ListConcepts(c *gin.Context) {
	ctx := context.Background()
	concepts, err := h.knowledgeStore.ListConceptSummaries(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if concepts == nil {
		concepts = []store.ConceptSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"concepts": concepts})
}

// GetConcept returns a single concept page by slug.
func (h *Handlers) GetConcept(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	ctx := context.Background()
	concept, err := h.knowledgeStore.GetWikiConcept(ctx, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "concept not found"})
		return
	}

	// Check if content needs generation (on-demand generation)
	if concept.Content == "" || concept.Definition == "" {
		provider := h.getProvider()
		if provider != nil {
			h.generateConceptContent(ctx, concept, provider)
			// Reload after generation
			concept, _ = h.knowledgeStore.GetWikiConcept(ctx, slug)
		}
	}

	c.JSON(http.StatusOK, concept)
}

// generateConceptContent generates the concept page content on-demand using LLM.
func (h *Handlers) generateConceptContent(ctx context.Context, c *store.WikiConcept, provider ai.Provider) error {
	var noteIDs []string
	if err := json.Unmarshal([]byte(c.NoteIDs), &noteIDs); err != nil {
		return err
	}

	// Collect content from all notes mentioning this concept
	var noteContents strings.Builder
	for i, noteID := range noteIDs {
		note, err := h.notesStore.GetNote(ctx, noteID)
		if err != nil {
			continue
		}
		content := note.Title
		if note.ContentText != nil {
			content += "\n" + *note.ContentText
		}
		noteContents.WriteString(fmt.Sprintf("--- Note %d: %s ---\n%s\n\n", i+1, note.Title, content[:min(len(content), 500)]))
	}

	prompt := fmt.Sprintf(`你是一个知识管理助手。以下是与概念「%s」相关的笔记片段，请生成一个结构化的概念页面。

相关笔记：
%s

请用中文生成以下内容：

## 定义
给出该概念的简明定义。

## 关键要点
列出 3-7 个关键要点。

## 矛盾或分歧（如有）
如果不同笔记对该概念有不同描述，请说明。如果没有，写"无分歧"。

## 来源统计
此概念出现在 %d 篇笔记中。

只返回以上格式的 Markdown 内容，不要其他说明。`, c.Title, noteContents.String(), c.SourceCount)

	content, err := provider.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate concept content: %w", err)
	}

	// Parse content into fields
	definition := ""
	keyPoints := ""
	contradictions := ""

	// Simple parsing
	if strings.Contains(content, "## 定义") {
		parts := strings.Split(content, "##")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "定义") {
				lines := strings.Split(part, "\n")
				if len(lines) > 1 {
					definition = strings.TrimSpace(lines[1])
				}
			}
			if strings.HasPrefix(part, "矛盾") {
				contradictions = strings.TrimSpace(strings.TrimPrefix(part, "矛盾或分歧（如有）"))
				contradictions = strings.TrimSpace(strings.TrimPrefix(contradictions, "或分歧（如有）"))
			}
		}
	}
	keyPoints = content

	c.Definition = definition
	c.KeyPoints = keyPoints
	c.Content = content
	c.Contradictions = contradictions

	return h.knowledgeStore.SaveWikiConcept(ctx, c)
}

// GetConceptGraph returns the full knowledge graph data (nodes and edges).
func (h *Handlers) GetConceptGraph(c *gin.Context) {
	ctx := context.Background()

	// Get all concepts
	concepts, err := h.knowledgeStore.ListWikiConcepts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get all relations
	relations, err := h.knowledgeStore.ListAllConceptRelations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Format for visualization (minimal fields to reduce payload)
	type Node struct {
		ID          string `json:"id"`
		Label       string `json:"label"`
		SourceCount int    `json:"source_count"`
	}
	type Edge struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Weight int    `json:"weight"`
	}

	var nodes []Node
	var edges []Edge

	for _, c := range concepts {
		nodes = append(nodes, Node{
			ID:          c.Slug,
			Label:       c.Title,
			SourceCount: c.SourceCount,
		})
	}

	for _, r := range relations {
		// Skip weak edges (weight < 2) — frontend filters these anyway
		if r.CoOccurrenceCount < 2 {
			continue
		}
		edges = append(edges, Edge{
			Source: r.SourceConceptSlug,
			Target: r.TargetConceptSlug,
			Weight: r.CoOccurrenceCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"edges": edges,
	})
}

// RefreshConceptGraph rebuilds co-occurrence relations from scratch.
func (h *Handlers) RefreshConceptGraph(c *gin.Context) {
	ctx := context.Background()
	if err := h.knowledgeStore.BuildCoOccurrenceRelations(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "concept graph refreshed"})
}

// DeleteConcept deletes a concept page by slug.
func (h *Handlers) DeleteConcept(c *gin.Context) {
	slug := c.Param("slug")
	ctx := context.Background()
	if err := h.knowledgeStore.DeleteWikiConcept(ctx, slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ConfirmConceptConfidence allows a user to confirm a concept's confidence as "high".
func (h *Handlers) ConfirmConceptConfidence(c *gin.Context) {
	slug := c.Param("slug")
	ctx := context.Background()

	concept, err := h.knowledgeStore.GetWikiConcept(ctx, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "concept not found"})
		return
	}

	if concept.ConfidencePending != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "concept is not pending confirmation"})
		return
	}

	concept.Confidence = "high"
	concept.ConfidencePending = 0
	now := time.Now().Unix()
	concept.LastReviewed = &now

	if err := h.knowledgeStore.SaveWikiConcept(ctx, concept); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.knowledgeStore.LogActivity(ctx, "confirm_confidence", "concept", slug,
		fmt.Sprintf("用户确认概念 %s 为高置信度", concept.Title), nil)

	c.JSON(http.StatusOK, gin.H{"success": true, "confidence": "high"})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
