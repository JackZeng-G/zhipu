package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// RunQuery executes the full QUERY pipeline: search -> read -> synthesize -> attribute -> persist.
func (h *Handlers) RunQuery(c *gin.Context) {
	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured"})
		return
	}

	var req struct {
		Query string `json:"query" binding:"required"`
		Save  bool   `json:"save"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Stage Q1-Q2: Find relevant concepts
	concepts, err := h.knowledgeStore.ListWikiConcepts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var conceptCatalog strings.Builder
	var relevantSlugs []string
	for _, c := range concepts {
		if c.SourceCount >= 1 {
			conceptCatalog.WriteString(fmt.Sprintf("- %s (slug: %s, sources: %d, confidence: %s)\n",
				c.Title, c.Slug, c.SourceCount, c.Confidence))
			relevantSlugs = append(relevantSlugs, c.Slug)
		}
	}

	// Stage Q3: Synthesize answer
	prompt := fmt.Sprintf(`基于以下知识库概念目录，回答用户的问题。
要求：
1. 每个核心结论必须引用具体的概念名称
2. 标注各来源的 confidence 级别
3. 如果来源之间有矛盾，显式标注分歧
4. 用中文回答

知识库概念目录：
%s

用户问题：%s`, conceptCatalog.String(), req.Query)

	answer, err := provider.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed: " + err.Error()})
		return
	}

	// Stage Q4: Source attribution
	sourceConceptsJSON, _ := json.Marshal(relevantSlugs[:min(10, len(relevantSlugs))])
	confidenceNotes := buildConfidenceNotes(concepts)

	// Stage Q5: Persist if requested
	slug := ""
	if req.Save {
		title := req.Query
		if len(title) > 80 {
			title = title[:80]
		}
		slug = store.Slugify(title) + "-" + time.Now().Format("2006-01-02")

		output := &store.WikiOutput{
			Slug:            slug,
			Title:           title,
			Content:         answer,
			OutputType:      "query",
			SourceConcepts:  string(sourceConceptsJSON),
			ConfidenceNotes: confidenceNotes,
		}
		if err := h.knowledgeStore.SaveWikiOutput(ctx, output); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed: " + err.Error()})
			return
		}

		h.knowledgeStore.LogActivity(ctx, "query_persist", "output", slug,
			fmt.Sprintf("QUERY 答案持久化: %s", title), map[string]interface{}{
				"query":   req.Query,
				"sources": len(relevantSlugs),
			})
	}

	c.JSON(http.StatusOK, gin.H{
		"answer":           answer,
		"source_concepts":  relevantSlugs[:min(10, len(relevantSlugs))],
		"confidence_notes": confidenceNotes,
		"saved":            req.Save,
		"slug":             slug,
	})
}

// ListOutputs returns all persisted outputs.
func (h *Handlers) ListOutputs(c *gin.Context) {
	outputType := c.Query("type")
	ctx := context.Background()
	outputs, err := h.knowledgeStore.ListWikiOutputs(ctx, outputType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if outputs == nil {
		outputs = []store.WikiOutput{}
	}
	c.JSON(http.StatusOK, gin.H{"outputs": outputs})
}

// GetOutput returns a single output by slug.
func (h *Handlers) GetOutput(c *gin.Context) {
	slug := c.Param("slug")
	ctx := context.Background()
	output, err := h.knowledgeStore.GetWikiOutput(ctx, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "output not found"})
		return
	}
	c.JSON(http.StatusOK, output)
}

// DeleteOutput deletes an output by slug.
func (h *Handlers) DeleteOutput(c *gin.Context) {
	slug := c.Param("slug")
	ctx := context.Background()
	if err := h.knowledgeStore.DeleteWikiOutput(ctx, slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// buildConfidenceNotes summarizes confidence levels of relevant concepts.
func buildConfidenceNotes(concepts []store.WikiConcept) string {
	var notes []string
	for _, c := range concepts {
		if c.SourceCount >= 1 {
			notes = append(notes, fmt.Sprintf("%s: %s (%d sources)", c.Title, c.Confidence, c.SourceCount))
		}
	}
	if len(notes) > 20 {
		notes = notes[:20]
	}
	return strings.Join(notes, "; ")
}
