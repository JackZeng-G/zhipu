package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"personal-kb/internal/ai"
	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// GetNoteSummary returns the persisted AI summary for a note.
func (h *Handlers) GetNoteSummary(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id required"})
		return
	}

	ctx := context.Background()
	summary, err := h.knowledgeStore.GetSummary(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "summary not found"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetNoteEntities returns extracted entities for a note.
func (h *Handlers) GetNoteEntities(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id required"})
		return
	}

	ctx := context.Background()
	entities, err := h.knowledgeStore.GetEntitiesByNote(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entities": entities})
}

// GetRelatedNotes returns notes related to the given note.
func (h *Handlers) GetRelatedNotes(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id required"})
		return
	}

	ctx := context.Background()
	relations, err := h.knowledgeStore.GetRelatedNotes(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build list of related note IDs with relation info
	type relatedNote struct {
		NoteID       string  `json:"note_id"`
		RelationType string  `json:"relation_type"`
		Reason       string  `json:"reason"`
		Confidence   float64 `json:"confidence"`
	}
	var results []relatedNote
	for _, r := range relations {
		noteID := r.TargetNoteID
		if noteID == id {
			noteID = r.SourceNoteID
		}
		// Fetch note title
		note, err := h.notesStore.GetNote(ctx, noteID)
		title := noteID
		if err == nil && note != nil {
			title = note.Title
		}
		results = append(results, relatedNote{
			NoteID:       noteID,
			RelationType: r.RelationType,
			Reason:       r.Reason,
			Confidence:   r.Confidence,
		})
		_ = title // TODO: include title in response
	}

	if results == nil {
		results = []relatedNote{}
	}
	c.JSON(http.StatusOK, gin.H{"related": results})
}

// TriggerIndex triggers indexing for a specific note or all notes.
func (h *Handlers) TriggerIndex(c *gin.Context) {
	var req struct {
		NoteID string `json:"note_id"` // empty = index all
	}
	// BindJSON returns error on malformed JSON, but empty body is OK (index all)
	_ = c.ShouldBindJSON(&req)

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no active AI provider configured"})
		return
	}

	ctx := context.Background()

	if req.NoteID != "" {
		// Index single note - get title for activity log
		note, err := h.notesStore.GetNote(ctx, req.NoteID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "note not found"})
			return
		}
		if err := h.indexNote(ctx, provider, req.NoteID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		h.knowledgeStore.LogActivity(ctx, "index", "note", req.NoteID,
			fmt.Sprintf("indexed note: %s", note.Title), nil)
		c.JSON(http.StatusOK, gin.H{"success": true, "indexed": req.NoteID})
		return
	}

	// Index all notes — run in background
	go h.indexAllNotes(provider)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "full index started in background"})
}

// GetIndexStatus returns the current indexing status.
func (h *Handlers) GetIndexStatus(c *gin.Context) {
	ctx := context.Background()

	summaryCount, _ := h.knowledgeStore.CountSummaries(ctx)
	entityCount, _ := h.knowledgeStore.CountEntities(ctx)
	relationCount, _ := h.knowledgeStore.CountRelations(ctx)
	noteCount, _ := h.notesStore.CountNotes(ctx, "")

	provider := h.getProvider()
	activeProvider := ""
	activeModel := ""
	if provider != nil {
		activeProvider = provider.Name()
		activeModel = provider.Model()
	}

	c.JSON(http.StatusOK, gin.H{
		"total_notes":    noteCount,
		"indexed_notes":  summaryCount,
		"total_entities": entityCount,
		"total_relations": relationCount,
		"active_provider": activeProvider,
		"active_model":   activeModel,
	})
}

// indexNote indexes a single note: extracts concepts and generates summary in one LLM call.
func (h *Handlers) indexNote(ctx context.Context, provider ai.Provider, noteID string) error {
	note, err := h.notesStore.GetNote(ctx, noteID)
	if err != nil {
		return err
	}

	text := note.Title
	if note.ContentText != nil && *note.ContentText != "" {
		content := *note.ContentText
		if len(content) > 3000 {
			content = content[:3000] + "..."
		}
		text += "\n\n" + content
	} else if note.ContentHTML != nil && *note.ContentHTML != "" {
		content := *note.ContentHTML
		if len(content) > 3000 {
			content = content[:3000] + "..."
		}
		text += "\n\n" + stripHTMLTags(content)
	}

	// Single LLM call: extract concepts + generate summary
	prompt := `分析以下笔记内容，同时完成概念提取和摘要生成。

任务1 - 提取核心概念（5-10个）：
识别笔记中的技术概念、工具、框架、项目等知识单元。
每个概念使用标准/常见名称（例如用 "Docker" 而不是 "docker容器技术"）。
返回 JSON 数组，每个元素：{"name": "概念名称", "type": "concept|technology|tool|project|person", "desc": "一句话描述"}

任务2 - 生成摘要：
用中文总结笔记内容，2-3句话概括要点，列出3-5个关键点。

返回JSON对象：
{
  "concepts": [{"name": "...", "type": "...", "desc": "..."}],
  "summary": "总结内容",
  "key_points": ["要点1", "要点2", ...]
}

笔记内容：
` + text

	resp, err := provider.Generate(ctx, prompt)
	if err != nil {
		log.Printf("[index] LLM call failed for %s: %v", noteID, err)
		return err
	}

	// Parse merged response
	concepts, summary, keyPoints := parseMergedIndexResponse(resp)

	if len(concepts) > 0 {
		h.knowledgeStore.SaveEntities(ctx, noteID, concepts)
	}
	if summary != "" {
		h.knowledgeStore.SaveSummary(ctx, noteID, summary, keyPoints)
	}

	h.knowledgeStore.SaveIndexMetadata(ctx, "summary", noteID, 0, provider.Model(), "")
	h.knowledgeStore.LogActivity(ctx, "index", "note", noteID,
		fmt.Sprintf("indexed note: %s", note.Title),
		map[string]interface{}{"provider": provider.Name(), "model": provider.Model(), "concepts": len(concepts)})

	return nil
}

// parseMergedIndexResponse parses the merged concept+summary response.
func parseMergedIndexResponse(resp string) ([]store.NoteEntity, string, []string) {
	jsonStr := extractJSON(resp)
	if jsonStr == "" {
		return nil, resp, nil
	}

	// Try merged format first
	var merged struct {
		Concepts []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Desc string `json:"desc"`
		} `json:"concepts"`
		Summary   string   `json:"summary"`
		KeyPoints []string `json:"key_points"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &merged); err == nil {
		var entities []store.NoteEntity
		for _, c := range merged.Concepts {
			if c.Name == "" {
				continue
			}
			entities = append(entities, store.NoteEntity{
				EntityType:  c.Type,
				EntityName:  c.Name,
				Description: c.Desc,
			})
		}
		return entities, merged.Summary, merged.KeyPoints
	}

	// Fallback: try old array format (concepts only)
	entities := parseEntityResponse(resp)
	if len(entities) > 0 {
		return entities, "", nil
	}

	// Fallback: try summary only
	summary, keyPoints := parseSummaryResponse(resp)
	return nil, summary, keyPoints
}

func (h *Handlers) indexAllNotes(provider ai.Provider) {
	ctx := context.Background()
	notes, err := h.notesStore.ListNotes(ctx, "", 0, 10000)
	if err != nil {
		log.Printf("[index] failed to list notes: %v", err)
		return
	}
	log.Printf("[index] starting full index of %d notes", len(notes))
	var indexedIDs []string
	for _, note := range notes {
		if err := h.indexNote(ctx, provider, note.ID); err != nil {
			continue
		}
		indexedIDs = append(indexedIDs, note.ID)
	}
	h.knowledgeStore.LogActivity(ctx, "index", "", "", "full index completed", map[string]interface{}{"total_notes": len(notes), "indexed": len(indexedIDs)})
	log.Printf("[index] full index completed (%d/%d notes)", len(indexedIDs), len(notes))

	for _, noteID := range indexedIDs {
		h.ingestToWiki(noteID)
	}

	// After full index, build relations between notes
	go h.buildRelationsAsync()
}

func parseEntityResponse(resp string) []store.NoteEntity {
	jsonStr := extractJSON(resp)
	if jsonStr == "" {
		return nil
	}
	var items []struct {
		Type        string "json:\"type\""
		Name        string "json:\"name\""
		Description string "json:\"description\""
	}
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return nil
	}
	var entities []store.NoteEntity
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		entities = append(entities, store.NoteEntity{EntityType: item.Type, EntityName: item.Name, Description: item.Description})
	}
	return entities
}

func parseSummaryResponse(resp string) (string, []string) {
	jsonStr := extractJSON(resp)
	if jsonStr == "" {
		return resp, nil
	}
	var result struct {
		Summary   string   "json:\"summary\""
		KeyPoints []string "json:\"key_points\""
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return resp, nil
	}
	return result.Summary, result.KeyPoints
}

func extractJSON(s string) string {
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end != -1 {
			return strings.TrimSpace(s[:end])
		}
	}
	if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end != -1 {
			return strings.TrimSpace(s[:end])
		}
	}
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) || (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return s
	}
	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start != -1 && end > start {
		return s[start : end+1]
	}
	return ""
}

// IndexNotesAsync indexes a batch of note IDs in the background using the active provider.
// This is intended to be used as a post-sync hook.
func (h *Handlers) IndexNotesAsync(noteIDs []string) {
	provider := h.getProvider()
	if provider == nil {
		log.Printf("[hook] no active AI provider, skipping auto-index")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		log.Printf("[hook] starting auto-index of %d notes", len(noteIDs))
		var indexedIDs []string
		for _, noteID := range noteIDs {
			if err := h.indexNote(ctx, provider, noteID); err != nil {
				continue
			}
			indexedIDs = append(indexedIDs, noteID)
		}

		for _, noteID := range indexedIDs {
			h.ingestToWiki(noteID)
		}

		// After indexing, build relations between notes
		h.buildRelationsAsync()

		h.knowledgeStore.LogActivity(ctx, "auto_index", "", "",
			"auto-index completed after sync",
			map[string]interface{}{"note_count": len(indexedIDs)})
		log.Printf("[hook] auto-index completed (%d notes)", len(indexedIDs))
	}()
}

// buildRelationsAsync discovers and saves relationships between notes.
func (h *Handlers) buildRelationsAsync() {
	provider := h.getProvider()
	if provider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Get all notes with summaries
	notes, err := h.notesStore.ListNotes(ctx, "", 0, 10000)
	if err != nil || len(notes) < 2 {
		return
	}

	// Build a compact note list for the prompt
	type noteInfo struct {
		ID      string
		Title   string
		Summary string
	}
	var infos []noteInfo
	for _, n := range notes {
		info := noteInfo{ID: n.ID, Title: n.Title}
		if summary, err := h.knowledgeStore.GetSummary(ctx, n.ID); err == nil && summary != nil {
			s := summary.Summary
			if len(s) > 200 {
				s = s[:200]
			}
			info.Summary = s
		}
		infos = append(infos, info)
	}

	// Process in batches of 50 notes
	batchSize := 50
	for batchStart := 0; batchStart < len(infos); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(infos) {
			batchEnd = len(infos)
		}
		batch := infos[batchStart:batchEnd]

		var noteList strings.Builder
		for i, n := range batch {
			noteList.WriteString(fmt.Sprintf("[%d] ID: %s, Title: %s", i, n.ID, n.Title))
			if n.Summary != "" {
				noteList.WriteString(fmt.Sprintf(", Summary: %s", n.Summary))
			}
			noteList.WriteString("\n")
		}

		prompt := fmt.Sprintf(`以下是用户的个人笔记列表：
%s

请找出这些笔记之间有意义的关联关系。对于每条关联返回：
- source_idx: 第一篇笔记的索引
- target_idx: 第二篇笔记的索引
- relation_type: 从以下类型中选择："related"（相关）、"depends_on"（依赖）、"explains"（解释说明）、"contrasts"（对比）、"extends"（延伸扩展）
- reason: 用中文说明它们为什么相关（1-2句话）
- confidence: 置信度 0.0 到 1.0

只包含真正有意义的关联关系，返回JSON数组。
只返回JSON数组，不要其他文字。`, noteList.String())

		resp, err := provider.Generate(ctx, prompt)
		if err != nil {
			log.Printf("[relations] LLM failed: %v", err)
			continue
		}

		// Parse response
		jsonStr := extractJSON(resp)
		if jsonStr == "" {
			continue
		}

		var items []struct {
			SourceIdx    int     `json:"source_idx"`
			TargetIdx    int     `json:"target_idx"`
			RelationType string  `json:"relation_type"`
			Reason       string  `json:"reason"`
			Confidence   float64 `json:"confidence"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			log.Printf("[relations] parse failed: %v", err)
			continue
		}

		saved := 0
		for _, item := range items {
			if item.SourceIdx < 0 || item.SourceIdx >= len(batch) || item.TargetIdx < 0 || item.TargetIdx >= len(batch) {
				continue
			}
			if item.SourceIdx == item.TargetIdx {
				continue
			}
			sourceID := batch[item.SourceIdx].ID
			targetID := batch[item.TargetIdx].ID

			h.knowledgeStore.SaveRelations(ctx, sourceID, []store.NoteRelation{{
				SourceNoteID: sourceID,
				TargetNoteID: targetID,
				RelationType: item.RelationType,
				Reason:       item.Reason,
				Confidence:   item.Confidence,
			}})
			saved++
		}
	}

	h.knowledgeStore.LogActivity(ctx, "build_relations", "", "", "auto cross-reference completed", nil)
}

// TranslateRelations translates all English relation reasons to Chinese.
func (h *Handlers) TranslateRelations(c *gin.Context) {
	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no active AI provider configured"})
		return
	}

	go h.translateAllRelations(provider)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "relation translation started in background"})
}

func (h *Handlers) translateAllRelations(provider ai.Provider) {
	ctx := context.Background()

	relations, err := h.knowledgeStore.GetAllRelations(ctx)
	if err != nil || len(relations) == 0 {
		return
	}

	// Filter: only translate relations with English-looking reason
	var toTranslate []store.NoteRelation
	for _, r := range relations {
		if looksEnglish(r.Reason) {
			toTranslate = append(toTranslate, r)
		}
	}

	if len(toTranslate) == 0 {
		return
	}


	// Batch translate: group by 20
	batchSize := 20
	for i := 0; i < len(toTranslate); i += batchSize {
		end := i + batchSize
		if end > len(toTranslate) {
			end = len(toTranslate)
		}
		batch := toTranslate[i:end]

		var lines strings.Builder
		for j, r := range batch {
			lines.WriteString(fmt.Sprintf("[%d] %s\n", j, r.Reason))
		}

		prompt := fmt.Sprintf(`将以下英文关联说明翻译为中文，保持原意，简洁专业。返回JSON数组，每个元素为翻译后的字符串，顺序与输入一致。

%s`, lines.String())

		resp, err := provider.Generate(ctx, prompt)
		if err != nil {
			log.Printf("[translate] LLM failed: %v", err)
			continue
		}

		jsonStr := extractJSON(resp)
		if jsonStr == "" {
			continue
		}

		var translations []string
		if err := json.Unmarshal([]byte(jsonStr), &translations); err != nil {
			log.Printf("[translate] parse failed: %v", err)
			continue
		}

		for j, t := range translations {
			if j >= len(batch) {
				break
			}
			if t != "" {
				h.knowledgeStore.UpdateRelationReason(ctx, batch[j].ID, t)
			}
		}
	}

	h.knowledgeStore.LogActivity(ctx, "translate", "", "",
		fmt.Sprintf("translated %d relation reasons to Chinese", len(toTranslate)), nil)
	log.Printf("[translate] completed (%d relations)", len(toTranslate))
}

// looksEnglish checks if a string looks like English (majority ASCII letters).
func looksEnglish(s string) bool {
	if s == "" {
		return false
	}
	asciiCount := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			asciiCount++
		}
	}
	letters := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 0x4e00 && r <= 0x9fff) {
			letters++
		}
	}
	if letters == 0 {
		return false
	}
	return float64(asciiCount)/float64(letters) > 0.6
}

// ingestToWiki runs after indexing a note.
// Implements the Karpathy INGEST pattern:
// 1. Update source_count for each concept mentioned in this note
// 2. Build co-occurrence relations between concepts in this note
// Uses exact match queries for efficiency (no N+1 LIKE searches).
func (h *Handlers) ingestToWiki(noteID string) {
	ctx := context.Background()

	entities, err := h.knowledgeStore.GetEntitiesByNote(ctx, noteID)
	if err != nil || len(entities) == 0 {
		return
	}

	// Step 1: Update each concept with its source notes
	conceptNames := make(map[string]bool)
	for _, e := range entities {
		conceptNames[e.EntityName] = true
		slug := store.Slugify(e.EntityName)

		// Use exact match query (efficient) instead of LIKE search
		noteIDs, err := h.knowledgeStore.GetNoteIDsByEntityName(ctx, e.EntityName)
		if err != nil {
			continue
		}
		sourceCount := len(noteIDs)

		existing, _ := h.knowledgeStore.GetWikiConcept(ctx, slug)
		noteIDsJSON, _ := json.Marshal(noteIDs)

		confidence := "low"
		confidencePending := 0
		switch {
		case sourceCount >= 5:
			confidence = "medium" // 不再自动晋升 high
			confidencePending = 1 // 标记等待用户确认
		case sourceCount >= 3:
			confidence = "medium"
		}

		var evolutionLog []map[string]interface{}
		if existing != nil {
			json.Unmarshal([]byte(existing.EvolutionLog), &evolutionLog)
		}

		now := time.Now().Format("2006-01-02")
		evolutionLog = append(evolutionLog, map[string]interface{}{
			"date":         now,
			"source_count": sourceCount,
			"action":       "source_added",
			"note_id":      noteID,
		})
		evolutionLogJSON, _ := json.Marshal(evolutionLog)

		concept := &store.WikiConcept{
			Slug:         slug,
			Title:        e.EntityName,
			NoteIDs:      string(noteIDsJSON),
			SourceCount:  sourceCount,
			Confidence:        confidence,
			ConfidencePending: confidencePending,
			EvolutionLog: string(evolutionLogJSON),
		}
		if existing != nil {
			concept.Aliases = existing.Aliases
			concept.Definition = existing.Definition
			concept.KeyPoints = existing.KeyPoints
			concept.Content = existing.Content
			concept.Contradictions = existing.Contradictions
		}
		if err := h.knowledgeStore.SaveWikiConcept(ctx, concept); err != nil {
		}
	}

	// Step 2: Build co-occurrence relations between concepts in this note
	var conceptSlugs []string
	for name := range conceptNames {
		conceptSlugs = append(conceptSlugs, store.Slugify(name))
	}
	for i := 0; i < len(conceptSlugs); i++ {
		for j := i + 1; j < len(conceptSlugs); j++ {
			a, b := conceptSlugs[i], conceptSlugs[j]
			if a > b {
				a, b = b, a
			}
			rel := &store.ConceptRelation{
				SourceConceptSlug: a,
				TargetConceptSlug: b,
				RelationType:      "co_occurs",
				CoOccurrenceCount: 1,
			}
			h.knowledgeStore.SaveConceptRelation(ctx, rel)
		}
	}

		// Check if new concepts match any open questions
		questions, err := h.knowledgeStore.ListOpenQuestions(ctx)
		if err == nil && len(questions) > 0 {
			for _, q := range questions {
				for conceptName := range conceptNames {
					if strings.Contains(strings.ToLower(q.Content), strings.ToLower(conceptName)) {
						h.knowledgeStore.LogActivity(ctx, "question_match", "question", strconv.Itoa(q.ID),
							fmt.Sprintf("新摄入的概念 '%s' 可能回答问题: %s", conceptName, q.Content),
							map[string]interface{}{"concept": conceptName, "question_id": q.ID})
						break
					}
				}
			}
		}

		h.knowledgeStore.LogActivity(ctx, "ingest", "note", noteID,
			fmt.Sprintf("INGEST: updated %d concepts", len(conceptNames)),
			map[string]interface{}{"concepts": len(conceptNames)})
}

// ResetIndexes deletes all index data and wiki pages.
func (h *Handlers) ResetIndexes(c *gin.Context) {
	ctx := context.Background()
	if err := h.knowledgeStore.ResetAllIndexes(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[index] all indexes and wiki data have been reset")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "all indexes and wiki data cleared"})
}

// === Internal: Category Tree Building ===

func (h *Handlers) buildCategoriesInBackground(ctx context.Context, provider ai.Provider, notes []ai.NoteInfo) {
	// Clear existing categories
	h.knowledgeStore.DeleteAllCategories(ctx)

	nodes, err := ai.BuildCategoryTree(ctx, provider, notes)
	if err != nil {
		log.Printf("[categories] build failed: %v", err)
		return
	}

	h.saveCategoryNodes(ctx, nodes, nil, "")
	h.knowledgeStore.RebuildNoteCounts(ctx)

	h.knowledgeStore.LogActivity(ctx, "categorize", "all", "",
		fmt.Sprintf("built category tree with %d root categories", len(nodes)),
		map[string]interface{}{"provider": provider.Name(), "model": provider.Model()})
	log.Printf("[categories] built category tree with %d root categories", len(nodes))
}

func (h *Handlers) saveCategoryNodes(ctx context.Context, nodes []ai.CategoryNode, parentID *int64, parentPath string) {
	for _, node := range nodes {
		path := parentPath + "/" + node.Name
		depth := 0
		if parentPath != "" {
			depth = len(strings.Split(parentPath, "/")) - 1
		}

		cat := &store.Category{
			ParentID: parentID,
			Name:     node.Name,
			Path:     path,
			Depth:    depth,
		}
		catID, err := h.knowledgeStore.SaveCategory(ctx, cat)
		if err != nil {
			log.Printf("[categories] save category %s failed: %v", path, err)
			continue
		}

		for _, noteID := range node.NoteIDs {
			h.knowledgeStore.MapNoteToCategory(ctx, noteID, catID, true)
		}

		if len(node.Children) > 0 {
			h.saveCategoryNodes(ctx, node.Children, &catID, path)
		}
	}
}
