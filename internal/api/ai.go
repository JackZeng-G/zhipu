package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"personal-kb/internal/ollama"
	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// AISummarize generates and caches a summary for a note.
func (h *Handlers) AISummarize(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id is required"})
		return
	}

	// Check cache first
	h.summaryMu.RLock()
	if summary, ok := h.summaryCache[id]; ok {
		h.summaryMu.RUnlock()
		c.JSON(http.StatusOK, gin.H{"summary": summary})
		return
	}
	h.summaryMu.RUnlock()

	// Get note from store
	ctx := context.Background()
	note, err := h.notesStore.GetNote(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
		return
	}

	content := buildNoteContent(note)
	prompt := fmt.Sprintf("Summarize this note concisely in 2-3 sentences:\n\n%s", content)

	summary, err := h.ollamaClient.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed: " + err.Error()})
		return
	}

	// Cache the summary
	h.summaryMu.Lock()
	h.summaryCache[id] = summary
	h.summaryMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// aiSearchRequest is the request body for POST /api/ai/search.
type aiSearchRequest struct {
	Query string `json:"query" binding:"required"`
}

// aiSearchResult represents a single search result with relevance reasoning.
type aiSearchResult struct {
	NoteID string `json:"note_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// AISearch uses AI to find notes relevant to a query.
func (h *Handlers) AISearch(c *gin.Context) {
	var req aiSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := context.Background()

	// Get all notes (limited to title + first 200 chars of content)
	notes, err := h.notesStore.ListNotes(ctx, "", 0, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notes: " + err.Error()})
		return
	}

	if len(notes) == 0 {
		c.JSON(http.StatusOK, gin.H{"results": []aiSearchResult{}})
		return
	}

	// Build the note list for the prompt, limiting context size
	var noteList strings.Builder
	for i, n := range notes {
		content := ""
		if n.ContentText != nil {
			content = *n.ContentText
			if len(content) > 200 {
				content = content[:200]
			}
		}
		noteList.WriteString(fmt.Sprintf("[%d] ID: %s, Title: %s, Content: %s\n", i, n.ID, n.Title, content))
	}

	prompt := fmt.Sprintf(`Given these notes:
%s

Which notes are relevant to the query "%s"?

Return a JSON array where each element has "note_id" and "reason" fields.
Only include notes that are genuinely relevant.
Example format: [{"note_id": "some-id", "reason": "This note discusses..."}]

Return ONLY the JSON array, no other text.`, noteList.String(), req.Query)

	response, err := h.ollamaClient.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI search failed: " + err.Error()})
		return
	}

	// Parse the AI response
	results := parseSearchResults(response, notes)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// parseSearchResults extracts search results from the AI response text.
func parseSearchResults(response string, notes []store.Note) []aiSearchResult {
	// Try to extract JSON array from the response
	response = strings.TrimSpace(response)

	// Find the JSON array in the response
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		return []aiSearchResult{}
	}

	jsonStr := response[start : end+1]

	var rawResults []struct {
		NoteID string `json:"note_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &rawResults); err != nil {
		return []aiSearchResult{}
	}

	// Build a map for quick title lookup
	noteTitles := make(map[string]string)
	for _, n := range notes {
		noteTitles[n.ID] = n.Title
	}

	results := make([]aiSearchResult, 0, len(rawResults))
	for _, r := range rawResults {
		if title, ok := noteTitles[r.NoteID]; ok {
			results = append(results, aiSearchResult{
				NoteID: r.NoteID,
				Title:  title,
				Reason: r.Reason,
			})
		}
	}

	return results
}

// aiEditRequest is the request body for POST /api/ai/edit.
type aiEditRequest struct {
	NoteID       string `json:"note_id"`
	SelectedText string `json:"selected_text" binding:"required"`
	Instruction  string `json:"instruction" binding:"required"`
}

// AIEdit sends selected text to AI for editing based on an instruction.
func (h *Handlers) AIEdit(c *gin.Context) {
	var req aiEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := context.Background()

	prompt := fmt.Sprintf(`Instruction: %s

Text to edit:
%s

Return ONLY the edited text, nothing else.`, req.Instruction, req.SelectedText)

	editedText, err := h.ollamaClient.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI edit failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"edited_text": editedText})
}

// ListConversations returns all AI conversations.
func (h *Handlers) ListConversations(c *gin.Context) {
	ctx := context.Background()
	convs, err := h.convStore.ListConversations(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list conversations: " + err.Error()})
		return
	}
	if convs == nil {
		convs = []store.Conversation{}
	}
	c.JSON(http.StatusOK, convs)
}

// createConversationRequest is the request body for POST /api/ai/conversations.
type createConversationRequest struct {
	NoteID string `json:"note_id"`
}

// CreateConversation creates a new AI conversation, optionally tied to a note.
func (h *Handlers) CreateConversation(c *gin.Context) {
	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Body is optional; create without note_id if parsing fails
		req = createConversationRequest{}
	}

	ctx := context.Background()

	title := "New Conversation"
	if req.NoteID != "" {
		note, err := h.notesStore.GetNote(ctx, req.NoteID)
		if err == nil {
			title = "Chat about " + note.Title
		}
	}

	var noteID *string
	if req.NoteID != "" {
		noteID = &req.NoteID
	}

	id, err := h.convStore.CreateConversation(ctx, noteID, title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create conversation: " + err.Error()})
		return
	}

	conv, err := h.convStore.GetConversation(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get conversation: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// sendMessageRequest is the request body for POST /api/ai/conversations/:id/messages.
type sendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// SendMessage sends a message in a conversation and streams the AI response via SSE.
func (h *Handlers) SendMessage(c *gin.Context) {
	convIDStr := c.Param("id")
	if convIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation id is required"})
		return
	}

	var convID int64
	if _, err := fmt.Sscanf(convIDStr, "%d", &convID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := context.Background()

	// Get the conversation
	conv, err := h.convStore.GetConversation(ctx, convID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	// Save the user message
	if err := h.convStore.AddMessage(ctx, convID, "user", req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message: " + err.Error()})
		return
	}

	// Build message list for Ollama
	var messages []ollama.Message

	// If conversation has a note, prepend as system context
	if conv.NoteID != nil && *conv.NoteID != "" {
		note, err := h.notesStore.GetNote(ctx, *conv.NoteID)
		if err == nil {
			content := buildNoteContent(note)
			messages = append(messages, ollama.Message{
				Role: "system",
				Content: fmt.Sprintf("You are discussing this note:\n\n%s", content),
			})
		}
	}

	// Get previous messages
	prevMsgs, err := h.convStore.GetMessages(ctx, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get messages: " + err.Error()})
		return
	}

	for _, msg := range prevMsgs {
		messages = append(messages, ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Stream response via SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	streamCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	ch, err := h.ollamaClient.ChatStream(streamCtx, messages)
	if err != nil {
		// Write error as SSE event
		fmt.Fprintf(c.Writer, "data: {\"error\": \"%s\"}\n\n", escapeJSON(err.Error()))
		c.Writer.Flush()
		return
	}

	var fullResponse strings.Builder
	for chunk := range ch {
		fullResponse.WriteString(chunk)

		data, _ := json.Marshal(map[string]string{"chunk": chunk})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	// Save assistant response
	if fullResponse.Len() > 0 {
		_ = h.convStore.AddMessage(ctx, convID, "assistant", fullResponse.String())
	}

	// Send done event
	fmt.Fprintf(c.Writer, "event: done\n\n")
	c.Writer.Flush()
}

// buildNoteContent creates a readable string from a note for AI prompts.
func buildNoteContent(note *store.Note) string {
	var sb strings.Builder
	sb.WriteString(note.Title)
	if note.ContentText != nil && *note.ContentText != "" {
		sb.WriteString("\n\n")
		sb.WriteString(*note.ContentText)
	} else if note.ContentHTML != nil && *note.ContentHTML != "" {
		sb.WriteString("\n\n")
		// Truncate very long HTML content
		html := *note.ContentHTML
		if len(html) > 2000 {
			html = html[:2000] + "..."
		}
		sb.WriteString(html)
	}
	return sb.String()
}

// escapeJSON escapes a string for safe inclusion in a JSON value.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
