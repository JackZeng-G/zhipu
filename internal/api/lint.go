package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// LintIssue represents a detected knowledge quality issue.
type LintIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"` // info, warning, error
	TargetID    string `json:"target_id"`
	TargetType  string `json:"target_type"` // note, entity, summary
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// LintResult contains all detected issues and statistics.
type LintResult struct {
	Issues         []LintIssue `json:"issues"`
	TotalNotes     int         `json:"total_notes"`
	IndexedNotes   int         `json:"indexed_notes"`
	OrphanedNotes  int         `json:"orphaned_notes"`
	StaleSummaries int         `json:"stale_summaries"`
}

// RunLint scans the knowledge base for quality issues.
func (h *Handlers) RunLint(c *gin.Context) {
	ctx := context.Background()

	result := LintResult{
		Issues: []LintIssue{},
	}

	// Get all notes
	notes, err := h.notesStore.ListNotes(ctx, "", 0, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result.TotalNotes = len(notes)

	// Check each note
	for _, note := range notes {
		// Check if note has entities
		entities, err := h.knowledgeStore.GetEntitiesByNote(ctx, note.ID)
		if err != nil || len(entities) == 0 {
			result.OrphanedNotes++
			result.Issues = append(result.Issues, LintIssue{
				Type:        "no_entities",
				Severity:    "info",
				TargetID:    note.ID,
				TargetType:  "note",
				Title:       note.Title,
				Description: "笔记没有提取到任何实体",
				Suggestion:  `点击"索引此笔记"按钮来提取实体`,
			})
		}

		// Check if note has relations
		relations, err := h.knowledgeStore.GetRelatedNotes(ctx, note.ID)
		if err != nil || len(relations) == 0 {
			if len(entities) > 0 {
				result.Issues = append(result.Issues, LintIssue{
					Type:        "no_relations",
					Severity:    "info",
					TargetID:    note.ID,
					TargetType:  "note",
					Title:       note.Title,
					Description: "笔记没有与其他笔记建立关联",
					Suggestion:  "等待自动关联分析完成，或手动触发索引",
				})
			}
		}

		// Check for stale summary
		summary, err := h.knowledgeStore.GetSummary(ctx, note.ID)
		if err == nil && summary != nil {
			result.IndexedNotes++
			// If note was modified after summary was generated
			if note.ModifiedTime > summary.GeneratedAt {
				result.StaleSummaries++
				result.Issues = append(result.Issues, LintIssue{
					Type:        "stale_summary",
					Severity:    "warning",
					TargetID:    note.ID,
					TargetType:  "summary",
					Title:       note.Title,
					Description: "笔记内容已更新，但摘要已过期",
					Suggestion:  "重新索引此笔记以更新摘要",
				})
			}
		}
	}

	// Check for entities with conflicting descriptions
	entityMap := make(map[string][]string)
	allEntities, _ := h.knowledgeStore.GetAllEntityNames(ctx)
	for _, e := range allEntities {
		key := fmt.Sprintf("%s:%s", e.EntityType, e.EntityName)
		entityMap[key] = append(entityMap[key], e.NoteID)
	}

	// Add stats for entities appearing in multiple notes (potential conflicts or rich context)
	for key, noteIDs := range entityMap {
		if len(noteIDs) > 3 {
			result.Issues = append(result.Issues, LintIssue{
				Type:        "popular_entity",
				Severity:    "info",
				TargetID:    key,
				TargetType:  "entity",
				Title:       key,
				Description: fmt.Sprintf("实体出现在 %d 篇笔记中", len(noteIDs)),
				Suggestion:  "可以为此实体生成 Wiki 页面",
			})
		}
	}

	// Check 4: Stub concepts (empty content or < 50 chars)
	concepts, err := h.knowledgeStore.ListWikiConcepts(ctx)
	if err == nil {
		for _, c := range concepts {
			if c.Content == "" || len(c.Content) < 50 {
				result.Issues = append(result.Issues, LintIssue{
					Type:        "stub_concept",
					Severity:    "warning",
					TargetID:    c.Slug,
					TargetType:  "concept",
					Title:       c.Title,
					Description: fmt.Sprintf("概念页内容为空或过短（%d 字符）", len(c.Content)),
					Suggestion:  "查看此概念并触发内容生成",
				})
			}
		}

		// Check 5: Stale concepts (not updated within domain_volatility threshold)
		now := time.Now().Unix()
		for _, c := range concepts {
			if c.LastReviewed != nil && *c.LastReviewed > 0 {
				threshold := int64(180 * 86400) // default medium
				switch c.DomainVolatility {
				case "high":
					threshold = 90 * 86400
				case "low":
					threshold = 365 * 86400
				}
				if now-*c.LastReviewed > threshold {
					result.Issues = append(result.Issues, LintIssue{
						Type:        "stale_concept",
						Severity:    "info",
						TargetID:    c.Slug,
						TargetType:  "concept",
						Title:       c.Title,
						Description: fmt.Sprintf("概念超过 %d 天未审阅（domain_volatility: %s）", threshold/86400, c.DomainVolatility),
						Suggestion:  "运行 REFLECT 更新此概念",
					})
				}
			}
		}

		// Check 6: Orphan concepts (source_count = 0)
		for _, c := range concepts {
			if c.SourceCount == 0 {
				result.Issues = append(result.Issues, LintIssue{
					Type:        "orphan_concept",
					Severity:    "warning",
					TargetID:    c.Slug,
					TargetType:  "concept",
					Title:       c.Title,
					Description: "概念无任何来源笔记（source_count = 0）",
					Suggestion:  "考虑删除此孤立概念，或为其补充来源",
				})
			}
		}
	}

	// Check 7: Content hash mismatch
	notes, _ = h.notesStore.ListNotes(ctx, "", 0, 10000)
	for _, note := range notes {
		if note.ContentHash == "" {
			continue
		}
		if note.ContentText == nil {
			continue
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(*note.ContentText)))
		if hash != note.ContentHash {
			result.Issues = append(result.Issues, LintIssue{
				Type:        "content_hash_mismatch",
				Severity:    "error",
				TargetID:    note.ID,
				TargetType:  "note",
				Title:       note.Title,
				Description: "笔记内容哈希与上次索引时不一致（内容可能被修改）",
				Suggestion:  "重新索引此笔记以更新摘要和实体",
			})
		}
	}

	c.JSON(http.StatusOK, result)
}

// FixLintIssues attempts to auto-fix lint issues.
func (h *Handlers) FixLintIssues(c *gin.Context) {
	var req struct {
		Issues []LintIssue `json:"issues"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured"})
		return
	}

	ctx := context.Background()
	fixed := 0

	for _, issue := range req.Issues {
		switch issue.Type {
		case "stale_summary", "no_entities":
			// Re-index the note
			if err := h.indexNote(ctx, provider, issue.TargetID); err == nil {
				fixed++
			}
		}
	}

	h.knowledgeStore.LogActivity(ctx, "lint_fix", "", "",
		"auto-fix completed",
		map[string]interface{}{"fixed_count": fixed})

	c.JSON(http.StatusOK, gin.H{"fixed": fixed})
}

// GetActivityLog returns recent activity log entries.
func (h *Handlers) GetActivityLog(c *gin.Context) {
	ctx := context.Background()
	limit := 50

	entries, err := h.knowledgeStore.GetRecentActivities(ctx, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []store.ActivityLogEntry{}
	}

	c.JSON(http.StatusOK, gin.H{"activities": entries})
}

// SaveChatInsight saves an insight extracted from chat.
func (h *Handlers) SaveChatInsight(c *gin.Context) {
	var req struct {
		Content    string `json:"content" binding:"required"`
		NoteID     string `json:"note_id"`
		RelatedIDs []string `json:"related_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	metadata := map[string]interface{}{
		"note_id":     req.NoteID,
		"related_ids": req.RelatedIDs,
	}

	err := h.knowledgeStore.LogActivity(ctx, "chat_insight", "insight", "",
		req.Content, metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AnalyzeChatForInsights uses AI to extract insights from chat content.
func (h *Handlers) AnalyzeChatForInsights(c *gin.Context) {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build conversation text
	var convo strings.Builder
	for _, m := range req.Messages {
		convo.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	prompt := fmt.Sprintf(`Analyze this conversation and identify if there are any valuable knowledge insights worth saving.

Conversation:
%s

If there are insights, return a JSON object:
{
  "has_insight": true,
  "title": "brief title",
  "content": "detailed insight content",
  "related_topics": ["topic1", "topic2"]
}

If no valuable insights, return {"has_insight": false}
Return ONLY the JSON object.`, convo.String())

	resp, err := provider.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	jsonStr := extractJSON(resp)
	if jsonStr == "" {
		c.JSON(http.StatusOK, gin.H{"has_insight": false})
		return
	}

	var result struct {
		HasInsight    bool     `json:"has_insight"`
		Title         string   `json:"title"`
		Content       string   `json:"content"`
		RelatedTopics []string `json:"related_topics"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		c.JSON(http.StatusOK, gin.H{"has_insight": false, "raw": resp})
		return
	}

	c.JSON(http.StatusOK, result)
}
