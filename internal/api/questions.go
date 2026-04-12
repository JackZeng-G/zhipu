package api

import (
	"context"
	"net/http"
	"strconv"

	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// CreateQuestion adds a new open question.
func (h *Handlers) CreateQuestion(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	id, err := h.knowledgeStore.SaveQuestion(ctx, req.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.knowledgeStore.LogActivity(ctx, "add_question", "question", strconv.FormatInt(id, 10),
		"添加开放问题: "+req.Question, nil)

	c.JSON(http.StatusOK, gin.H{"id": id, "question": req.Question, "status": "open"})
}

// ListQuestions returns all questions, optionally filtered by status.
func (h *Handlers) ListQuestions(c *gin.Context) {
	status := c.Query("status")
	ctx := context.Background()
	questions, err := h.knowledgeStore.ListAllQuestions(ctx, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if questions == nil {
		questions = []store.Question{}
	}
	c.JSON(http.StatusOK, gin.H{"questions": questions})
}

// ResolveQuestion marks a question as answered.
func (h *Handlers) ResolveQuestion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		OutputSlug string `json:"output_slug"`
	}
	_ = c.ShouldBindJSON(&req)

	ctx := context.Background()
	if err := h.knowledgeStore.ResolveQuestion(ctx, id, req.OutputSlug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.knowledgeStore.LogActivity(ctx, "resolve_question", "question", strconv.Itoa(id),
		"问题已解决", nil)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
