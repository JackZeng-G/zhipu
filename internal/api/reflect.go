package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"personal-kb/internal/ai"
	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

var reflectRunning atomic.Bool

// RunReflect starts the 4-stage REFLECT pipeline in the background.
func (h *Handlers) RunReflect(c *gin.Context) {
	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured"})
		return
	}

	if !reflectRunning.CompareAndSwap(false, true) {
		c.JSON(http.StatusConflict, gin.H{"error": "REFLECT already running"})
		return
	}

	go h.executeReflect(provider)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "REFLECT started"})
}

// GetReflectStatus returns the current REFLECT progress.
func (h *Handlers) GetReflectStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"running": reflectRunning.Load()})
}

func (h *Handlers) executeReflect(provider ai.Provider) {
	defer reflectRunning.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Printf("[reflect] starting REFLECT pipeline")

	log.Printf("[reflect] Stage 0: counter-evidence search")
	h.reflectStage0(ctx, provider)

	log.Printf("[reflect] Stage 1: pattern scan")
	patterns := h.reflectStage1(ctx)

	log.Printf("[reflect] Stage 2: deep synthesis")
	h.reflectStage2(ctx, provider, patterns)

	log.Printf("[reflect] Stage 3: gap analysis")
	h.reflectStage3(ctx)

	h.knowledgeStore.LogActivity(ctx, "reflect", "", "",
		"REFLECT 完成（4 阶段）", nil)
	log.Printf("[reflect] REFLECT pipeline completed")
}

func (h *Handlers) reflectStage0(ctx context.Context, provider ai.Provider) {
	concepts, err := h.knowledgeStore.ListWikiConcepts(ctx)
	if err != nil || len(concepts) == 0 {
		return
	}

	for _, concept := range concepts {
		if concept.Confidence == "" || concept.Confidence == "low" {
			continue
		}

		var noteIDs []string
		if err := json.Unmarshal([]byte(concept.NoteIDs), &noteIDs); err != nil {
			continue
		}

		var noteContents strings.Builder
		for i, id := range noteIDs {
			if i >= 5 {
				break
			}
			note, err := h.notesStore.GetNote(ctx, id)
			if err != nil {
				continue
			}
			text := note.Title
			if note.ContentText != nil {
				text += "\n" + *note.ContentText
			}
			if len(text) > 500 {
				text = text[:500]
			}
			noteContents.WriteString(fmt.Sprintf("--- 来源 %d ---\n%s\n\n", i+1, text))
		}

		prompt := fmt.Sprintf(`分析以下关于「%s」的来源笔记。

概念当前定义：%s

来源笔记：
%s

任务：
1. 检查来源中是否存在与当前定义矛盾的内容
2. 如果有矛盾，描述矛盾内容
3. 如果没有矛盾，回答"无矛盾"

只返回分析结果，不要其他说明。`, concept.Title, concept.Definition, noteContents.String())

		resp, err := provider.Generate(ctx, prompt)
		if err != nil {
			log.Printf("[reflect] Stage 0 failed for %s: %v", concept.Slug, err)
			continue
		}

		if strings.Contains(resp, "无矛盾") {
			if concept.Contradictions == "" {
				concept.Contradictions = "⚠ 回音室风险：未找到反驳来源，结论可能存在确认偏差"
			}
		} else {
			concept.Contradictions = resp
		}

		h.classifyEvolutionLog(ctx, provider, concept)

		now := time.Now().Unix()
		concept.LastReviewed = &now
		h.knowledgeStore.SaveWikiConcept(ctx, &concept)
	}
}

func (h *Handlers) classifyEvolutionLog(ctx context.Context, provider ai.Provider, concept store.WikiConcept) {
	var logEntries []map[string]interface{}
	if err := json.Unmarshal([]byte(concept.EvolutionLog), &logEntries); err != nil {
		return
	}

	hasUnclassified := false
	for _, entry := range logEntries {
		if action, ok := entry["action"].(string); ok && action == "source_added" {
			hasUnclassified = true
			break
		}
	}
	if !hasUnclassified {
		return
	}

	prompt := fmt.Sprintf(`对概念「%s」的以下 evolution log 条目进行语义分类。
每个条目从 "source_added" 改为以下之一：
- "reinforce"：新来源与现有定义一致（强化）
- "revise"：新来源修正了现有定义（修正）
- "contradict"：新来源与现有定义矛盾（分歧）

当前 evolution log：
%s

返回完整的 JSON 数组，保持其他字段不变，只改 action 字段。`, concept.Title, concept.EvolutionLog)

	resp, err := provider.Generate(ctx, prompt)
	if err != nil {
		return
	}
	jsonStr := extractJSON(resp)
	if jsonStr == "" {
		return
	}
	var classified []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &classified); err != nil {
		return
	}
	classifiedJSON, _ := json.Marshal(classified)
	concept.EvolutionLog = string(classifiedJSON)
}

func (h *Handlers) reflectStage1(ctx context.Context) []string {
	var patterns []string

	concepts, _ := h.knowledgeStore.ListWikiConcepts(ctx)
	thirtyDaysAgo := time.Now().Unix() - 30*86400
	for _, c := range concepts {
		if c.SourceCount == 1 && c.CreatedAt < thirtyDaysAgo {
			patterns = append(patterns, c.Slug)
		}
	}

	relations, _ := h.knowledgeStore.ListAllConceptRelations(ctx)
	for _, r := range relations {
		if r.CoOccurrenceCount >= 3 {
			patterns = append(patterns,
				fmt.Sprintf("%s+%s (共现 %d 次)", r.SourceConceptSlug, r.TargetConceptSlug, r.CoOccurrenceCount))
		}
	}

	return patterns
}

func (h *Handlers) reflectStage2(ctx context.Context, provider ai.Provider, patterns []string) {
	concepts, _ := h.knowledgeStore.ListWikiConcepts(ctx)
	for _, c := range concepts {
		if c.SourceCount < 3 || c.Content == "" {
			continue
		}

		rels, err := h.knowledgeStore.GetConceptRelations(ctx, c.Slug)
		if err != nil || len(rels) == 0 {
			continue
		}

		var relatedNames []string
		for _, r := range rels {
			if r.CoOccurrenceCount >= 2 {
				otherSlug := r.TargetConceptSlug
				if otherSlug == c.Slug {
					otherSlug = r.SourceConceptSlug
				}
				relatedNames = append(relatedNames, otherSlug)
			}
		}

		if len(relatedNames) == 0 {
			continue
		}

		contentPreview := c.Content
		if len(contentPreview) > 1000 {
			contentPreview = contentPreview[:1000]
		}

		prompt := fmt.Sprintf(`基于概念「%s」及其关联概念，生成一篇跨概念综合分析。

当前概念内容：
%s

关联概念：%s

请生成综合分析，包含：
## 核心论点
## 证据支持
## 反证与局限
## 综合结论
## Confidence Notes
## 局限性

用中文撰写。`, c.Title, contentPreview, strings.Join(relatedNames, ", "))

		content, err := provider.Generate(ctx, prompt)
		if err != nil {
			continue
		}

		slug := c.Slug + "-synthesis"
		slugs := append([]string{c.Slug}, relatedNames[:min(5, len(relatedNames))]...)
		slugsJSON, _ := json.Marshal(slugs)

		syn := &store.WikiSynthesis{
			Slug:         slug,
			Title:        fmt.Sprintf("综合分析：%s", c.Title),
			Content:      content,
			ConceptSlugs: string(slugsJSON),
		}

		h.knowledgeStore.SaveWikiSynthesis(ctx, syn)
		log.Printf("[reflect] Stage 2: generated synthesis for %s", c.Slug)
	}
}

func (h *Handlers) reflectStage3(ctx context.Context) {
	concepts, _ := h.knowledgeStore.ListWikiConcepts(ctx)

	thirtyDaysAgo := time.Now().Unix() - 30*86400
	var gaps []string

	for _, c := range concepts {
		if c.SourceCount == 1 && c.CreatedAt < thirtyDaysAgo {
			gaps = append(gaps, fmt.Sprintf("- 孤立概念：**%s**（仅 1 个来源，超过 30 天未补充）", c.Title))
		}
		if c.Content == "" && c.SourceCount >= 2 {
			gaps = append(gaps, fmt.Sprintf("- 空概念页：**%s**（%d 个来源但无内容，需生成）", c.Title, c.SourceCount))
		}
	}

	if len(gaps) == 0 {
		gaps = append(gaps, "- 知识库状态良好，未发现明显空白")
	}

	content := fmt.Sprintf("# Gap Analysis 报告\n\n生成时间：%s\n\n## 发现的空白\n\n%s\n\n## 建议\n\n- 优先为孤立概念补充新来源\n- 为空概念页触发生成",
		time.Now().Format("2006-01-02 15:04"), strings.Join(gaps, "\n"))

	slug := fmt.Sprintf("gap-report-%s", time.Now().Format("2006-01-02"))
	output := &store.WikiOutput{
		Slug:       slug,
		Title:      fmt.Sprintf("Gap Analysis %s", time.Now().Format("2006-01-02")),
		Content:    content,
		OutputType: "gap",
	}
	h.knowledgeStore.SaveWikiOutput(ctx, output)
	log.Printf("[reflect] Stage 3: gap report saved as %s", slug)
}
