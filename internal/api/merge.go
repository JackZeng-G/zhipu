package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MergeCandidate represents a pair of concepts that might be merged.
type MergeCandidate struct {
	SourceSlug    string `json:"source_slug"`
	TargetSlug    string `json:"target_slug"`
	SourceTitle   string `json:"source_title"`
	TargetTitle   string `json:"target_title"`
	Similarity    string `json:"similarity"` // "name" or "source_overlap"
	SharedSources int    `json:"shared_sources"`
}

// GetMergeCandidates returns concepts that might be duplicates.
func (h *Handlers) GetMergeCandidates(c *gin.Context) {
	ctx := context.Background()
	concepts, err := h.knowledgeStore.ListWikiConcepts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var candidates []MergeCandidate

	// Find concepts with overlapping source notes
	for i := 0; i < len(concepts); i++ {
		for j := i + 1; j < len(concepts); j++ {
			a, b := concepts[i], concepts[j]

			// Skip if either is a redirect
			if a.RedirectTo != "" || b.RedirectTo != "" {
				continue
			}

			var idsA, idsB []string
			json.Unmarshal([]byte(a.NoteIDs), &idsA)
			json.Unmarshal([]byte(b.NoteIDs), &idsB)

			shared := 0
			setA := make(map[string]bool)
			for _, id := range idsA {
				setA[id] = true
			}
			for _, id := range idsB {
				if setA[id] {
					shared++
				}
			}

			if shared >= 3 {
				candidates = append(candidates, MergeCandidate{
					SourceSlug:    a.Slug,
					TargetSlug:    b.Slug,
					SourceTitle:   a.Title,
					TargetTitle:   b.Title,
					Similarity:    "source_overlap",
					SharedSources: shared,
				})
			}

			// Name similarity (simple Jaccard on bigrams)
			if jaccardSimilarity(a.Title, b.Title) > 0.7 {
				candidates = append(candidates, MergeCandidate{
					SourceSlug:  a.Slug,
					TargetSlug:  b.Slug,
					SourceTitle: a.Title,
					TargetTitle: b.Title,
					Similarity:  "name",
				})
			}
		}
	}

	if candidates == nil {
		candidates = []MergeCandidate{}
	}
	c.JSON(http.StatusOK, gin.H{"candidates": candidates})
}

// ExecuteMerge merges source concept into target concept.
func (h *Handlers) ExecuteMerge(c *gin.Context) {
	var req struct {
		SourceSlug string `json:"source_slug" binding:"required"`
		TargetSlug string `json:"target_slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	if req.SourceSlug == req.TargetSlug {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source and target cannot be the same"})
		return
	}

	source, err := h.knowledgeStore.GetWikiConcept(ctx, req.SourceSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source concept not found"})
		return
	}
	target, err := h.knowledgeStore.GetWikiConcept(ctx, req.TargetSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target concept not found"})
		return
	}

	// Merge note_ids (union)
	var sourceIDs, targetIDs []string
	json.Unmarshal([]byte(source.NoteIDs), &sourceIDs)
	json.Unmarshal([]byte(target.NoteIDs), &targetIDs)
	mergedIDs := unionStrings(sourceIDs, targetIDs)
	mergedIDsJSON, _ := json.Marshal(mergedIDs)

	// Merge aliases (union)
	var sourceAliases, targetAliases []string
	json.Unmarshal([]byte(source.Aliases), &sourceAliases)
	json.Unmarshal([]byte(target.Aliases), &targetAliases)
	mergedAliases := unionStrings(sourceAliases, targetAliases)
	mergedAliasesJSON, _ := json.Marshal(mergedAliases)

	// Merge evolution_log (concatenate)
	var sourceLog, targetLog []map[string]interface{}
	json.Unmarshal([]byte(source.EvolutionLog), &sourceLog)
	json.Unmarshal([]byte(target.EvolutionLog), &targetLog)
	mergedLog := append(targetLog, sourceLog...)
	mergedLogJSON, _ := json.Marshal(mergedLog)

	// Update target
	target.NoteIDs = string(mergedIDsJSON)
	target.Aliases = string(mergedAliasesJSON)
	target.EvolutionLog = string(mergedLogJSON)
	target.SourceCount = len(mergedIDs)
	h.knowledgeStore.SaveWikiConcept(ctx, target)

	// Set source as redirect (keep original NoteIDs for audit trail)
	source.RedirectTo = req.TargetSlug
	h.knowledgeStore.SaveWikiConcept(ctx, source)

	// Update all concept_relations references
	h.knowledgeStore.UpdateConceptRelationSlugs(ctx, req.SourceSlug, req.TargetSlug)

	h.knowledgeStore.LogActivity(ctx, "merge", "concept", req.SourceSlug,
		fmt.Sprintf("合并概念：%s → %s", req.SourceSlug, req.TargetSlug), nil)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("merged %s into %s", req.SourceSlug, req.TargetSlug)})
}

// jaccardSimilarity computes Jaccard similarity on character bigrams.
func jaccardSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	setA := make(map[string]bool)
	setB := make(map[string]bool)
	for i := 0; i < len(a)-1; i++ {
		setA[a[i:i+2]] = true
	}
	for i := 0; i < len(b)-1; i++ {
		setB[b[i:i+2]] = true
	}
	intersection := 0
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// unionStrings returns the union of two string slices, deduplicated.
func unionStrings(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range append(a, b...) {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
