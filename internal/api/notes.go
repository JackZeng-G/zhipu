package api

import (
	"context"
	"net/http"
	"strconv"

	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// ListNotebooks returns all notebooks.
func (h *Handlers) ListNotebooks(c *gin.Context) {
	ctx := context.Background()
	notebooks, err := h.notesStore.ListNotebooks(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notebooks: " + err.Error()})
		return
	}
	if notebooks == nil {
		notebooks = []store.Notebook{}
	}
	c.JSON(http.StatusOK, notebooks)
}

// listNotesResponse is the paginated response for listing notes.
type listNotesResponse struct {
	Total int         `json:"total"`
	Items []noteItem  `json:"items"`
}

// noteItem is a summary of a note for list views.
type noteItem struct {
	ID            string  `json:"id"`
	NotebookID    *string `json:"notebook_id"`
	Title         string  `json:"title"`
	Tags          *string `json:"tags"`
	CreatedTime   int64   `json:"created_time"`
	ModifiedTime  int64   `json:"modified_time"`
}

// ListNotes returns paginated notes, optionally filtered by notebook.
func (h *Handlers) ListNotes(c *gin.Context) {
	notebookID := c.Query("notebook_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	ctx := context.Background()

	notes, err := h.notesStore.ListNotes(ctx, notebookID, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notes: " + err.Error()})
		return
	}

	// Get total count by fetching with a large limit
	allNotes, _ := h.notesStore.ListNotes(ctx, notebookID, 0, 1000000)
	total := len(allNotes)

	items := make([]noteItem, 0, len(notes))
	for _, n := range notes {
		items = append(items, noteItem{
			ID:           n.ID,
			NotebookID:   n.NotebookID,
			Title:        n.Title,
			Tags:         n.Tags,
			CreatedTime:  n.CreatedTime,
			ModifiedTime: n.ModifiedTime,
		})
	}

	if items == nil {
		items = []noteItem{}
	}

	c.JSON(http.StatusOK, listNotesResponse{
		Total: total,
		Items: items,
	})
}

// GetNote returns a single note by ID with full content.
func (h *Handlers) GetNote(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id is required"})
		return
	}

	ctx := context.Background()
	note, err := h.notesStore.GetNote(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
		return
	}

	c.JSON(http.StatusOK, note)
}
