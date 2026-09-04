package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// ListNotebooks returns all all notebooks.
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

// ListStacks returns all distinct stack names.
func (h *Handlers) ListStacks(c *gin.Context) {
	ctx := context.Background()
	stacks, err := h.notesStore.ListStacks(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if stacks == nil {
		stacks = []string{}
	}
	c.JSON(http.StatusOK, stacks)
}

// CreateNotebook creates a new notebook on NAS and saves locally.
func (h *Handlers) CreateNotebook(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Stack string `json:"stack"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	now := time.Now().Unix()
	var nbID string

	if h.nasClient != nil && h.authClient != nil {
		if !h.authClient.IsLoggedIn() {
			h.reconnectNAS()
		}
		if h.authClient.IsLoggedIn() {
			id, err := h.nasClient.CreateNotebook(req.Title, req.Stack)
			if err != nil {
				log.Printf("[api] NAS create notebook failed: %v", err)
				h.reconnectNAS()
				if h.authClient.IsLoggedIn() {
					id, err = h.nasClient.CreateNotebook(req.Title, req.Stack)
				}
			}
			if err == nil {
				nbID = id
			} else {
				log.Printf("[api] NAS create notebook failed after retry: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "NAS 操作失败"})
				return
			}
		}
	}
	if nbID == "" {
		nbID = fmt.Sprintf("local_nb_%d", now)
	}

	var stackPtr *string
	if req.Stack != "" {
		stackPtr = &req.Stack
	}
	nb := &store.Notebook{
		ID:            nbID,
		Title:         req.Title,
		Stack:         stackPtr,
		CreatedTime:   now,
		ModifiedTime:  now,
	}
	if err := h.notesStore.SaveNotebook(ctx, nb); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nb)
}

// RenameStack renames a stack across all notebooks (NAS + local).
func (h *Handlers) RenameStack(c *gin.Context) {
	var req struct {
		OldName string `json:"old_name" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if h.nasClient != nil && h.authClient != nil && h.authClient.IsLoggedIn() {
		notebooks, err := h.notesStore.GetNotebooksByStack(ctx, req.OldName)
		if err == nil {
			for _, nb := range notebooks {
				if err := h.nasClient.EditNotebook(nb.ID, nb.Title, req.NewName); err != nil {
					log.Printf("[api] NAS rename stack for notebook %s failed: %v", nb.ID, err)
				}
			}
		}
	}

	affected, err := h.notesStore.RenameStack(ctx, req.OldName, req.NewName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "affected": affected})
}

// DeleteStack removes stack association from all notebooks.
func (h *Handlers) DeleteStack(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stack name is required"})
		return
	}

	ctx := context.Background()
	if h.nasClient != nil && h.authClient != nil && h.authClient.IsLoggedIn() {
		notebooks, err := h.notesStore.GetNotebooksByStack(ctx, name)
		if err == nil {
			for _, nb := range notebooks {
				if err := h.nasClient.EditNotebook(nb.ID, nb.Title, ""); err != nil {
					log.Printf("[api] NAS clear stack for notebook %s failed: %v", nb.ID, err)
				}
			}
		}
	}

	affected, err := h.notesStore.RenameStack(ctx, name, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "affected": affected})
}

// MoveNotebookToStack changes which stack a notebook belongs to.
func (h *Handlers) MoveNotebookToStack(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notebook id is required"})
		return
	}

	var req struct {
		Stack string `json:"stack"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.nasClient != nil && h.authClient != nil {
		if !h.authClient.IsLoggedIn() {
			h.reconnectNAS()
		}
		if h.authClient.IsLoggedIn() {
			if err := h.nasClient.EditNotebook(id, "", req.Stack); err != nil {
				log.Printf("[api] NAS move notebook %s to stack failed: %v", id, err)
			}
		}
	}

	ctx := context.Background()
	var stackPtr *string
	if req.Stack != "" {
		stackPtr = &req.Stack
	}
	if err := h.notesStore.UpdateNotebookStack(ctx, id, stackPtr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// reconnectNAS tries to re-login to NAS using saved credentials.
func (h *Handlers) reconnectNAS() bool {
	if h.authClient == nil {
		return false
	}
	username, _ := h.settingsStore.GetSetting("nas_username")
	password, _ := h.settingsStore.GetSetting("nas_password_encrypted")
	if username == "" || password == "" {
		return false
	}
	if err := h.authClient.Login(username, password); err != nil {
		log.Printf("[api] NAS auto-reconnect failed: %v", err)
		return false
	}
	log.Printf("[api] NAS auto-reconnect succeeded")
	return true
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
	if pageSize < 1 || pageSize > 10000 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	ctx := context.Background()

	notes, err := h.notesStore.ListNotes(ctx, notebookID, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notes: " + err.Error()})
		return
	}

	total, _ := h.notesStore.CountNotes(ctx, notebookID)

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

	// Rewrite image URLs to use proxy
	if note.ContentHTML != nil && *note.ContentHTML != "" {
		host, _ := h.settingsStore.GetSetting("nas_host")
		rewritten := rewriteImageURLs(*note.ContentHTML, host, id)
		note.ContentHTML = &rewritten
	}

	c.JSON(http.StatusOK, note)
}
