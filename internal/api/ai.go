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

// AISummarize generates and caches a summary for a note.
func (h *Handlers) AISummarize(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id is required"})
		return
	}

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured, please add one in Settings"})
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
	if len(content) <= len(note.Title)+5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note has no content to summarize"})
		return
	}
	prompt := fmt.Sprintf("请用中文简洁总结以下笔记内容，2-3句话概括要点：\n\n%s", content)

	summary, err := provider.Generate(ctx, prompt)
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

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured, please add one in Settings"})
		return
	}

	ctx := context.Background()

	// Step 1: Search category index
	var candidateNotes []store.Note
	if catNoteIDs, err := h.knowledgeStore.SearchNotesByCategoryQuery(ctx, req.Query); err == nil && len(catNoteIDs) > 0 {
		seen := make(map[string]bool)
		for _, nid := range catNoteIDs {
			if seen[nid] {
				continue
			}
			seen[nid] = true
			if note, err := h.notesStore.GetNote(ctx, nid); err == nil {
				candidateNotes = append(candidateNotes, *note)
			}
		}
	}

	// Step 2: Search entity index
	if entityResults := h.entityBasedSearch(ctx, req.Query); len(entityResults) > 0 {
		seen := make(map[string]bool)
		for _, n := range candidateNotes {
			seen[n.ID] = true
		}
		for _, n := range entityResults {
			if !seen[n.ID] {
				candidateNotes = append(candidateNotes, n)
			}
		}
	}

	// Step 3: If we have candidates, refine with LLM
	if len(candidateNotes) > 0 {
		results := h.refineSearchWithLLM(ctx, provider, req.Query, candidateNotes)
		if len(results) > 0 {
			c.JSON(http.StatusOK, gin.H{"results": results})
			return
		}
	}

	// Fall back to full note list search
	notes, err := h.notesStore.ListNotes(ctx, "", 0, 10000)
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

	response, err := provider.Generate(ctx, prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI search failed: " + err.Error()})
		return
	}

	// Parse the AI response
	results := parseSearchResults(response, notes)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// entityBasedSearch finds notes via entity index matching.
func (h *Handlers) entityBasedSearch(ctx context.Context, query string) []store.Note {
	entities, err := h.knowledgeStore.SearchEntities(ctx, query)
	if err != nil || len(entities) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var notes []store.Note
	for _, e := range entities {
		if seen[e.NoteID] {
			continue
		}
		seen[e.NoteID] = true
		note, err := h.notesStore.GetNote(ctx, e.NoteID)
		if err == nil {
			notes = append(notes, *note)
		}
	}
	return notes
}

// refineSearchWithLLM uses LLM to refine entity-matched results.
func (h *Handlers) refineSearchWithLLM(ctx context.Context, provider interface {
	Generate(context.Context, string) (string, error)
}, query string, notes []store.Note) []aiSearchResult {

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

	prompt := fmt.Sprintf(`Given these candidate notes:
%s

Which notes are relevant to the query "%s"?

Return a JSON array where each element has "note_id" and "reason" fields.
Only include notes that are genuinely relevant.
Return ONLY the JSON array, no other text.`, noteList.String(), query)

	response, err := provider.Generate(ctx, prompt)
	if err != nil {
		return nil
	}
	return parseSearchResults(response, notes)
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

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured, please add one in Settings"})
		return
	}

	ctx := context.Background()

	prompt := fmt.Sprintf(`Instruction: %s

Text to edit:
%s

Return ONLY the edited text, nothing else.`, req.Instruction, req.SelectedText)

	editedText, err := provider.Generate(ctx, prompt)
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

// DeleteConversation deletes a conversation and all its messages.
func (h *Handlers) DeleteConversation(c *gin.Context) {
	id, err := paramInt(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := h.convStore.DeleteConversation(ctx, int64(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete conversation: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
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

	provider := h.getProvider()
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no AI provider configured, please add one in Settings"})
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

	// Build prompt from conversation history
	var promptBuilder strings.Builder

	// If conversation has a note, prepend as system context
	if conv.NoteID != nil && *conv.NoteID != "" {
		note, err := h.notesStore.GetNote(ctx, *conv.NoteID)
		if err == nil {
			content := buildNoteContent(note)
			promptBuilder.WriteString(fmt.Sprintf("[System]: You are discussing this note:\n\n%s\n\n", content))
		}
	}

	// Get previous messages (includes the just-saved user message)
	prevMsgs, err := h.convStore.GetMessages(ctx, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get messages: " + err.Error()})
		return
	}

	for _, msg := range prevMsgs {
		switch msg.Role {
		case "user":
			promptBuilder.WriteString(fmt.Sprintf("[User]: %s\n\n", msg.Content))
		case "assistant":
			promptBuilder.WriteString(fmt.Sprintf("[Assistant]: %s\n\n", msg.Content))
		case "system":
			promptBuilder.WriteString(fmt.Sprintf("[System]: %s\n\n", msg.Content))
		}
	}

	// Stream response via SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	streamCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var fullResponse strings.Builder
	err = provider.GenerateStream(streamCtx, promptBuilder.String(), func(chunk string) {
		fullResponse.WriteString(chunk)
		data, _ := json.Marshal(map[string]string{"chunk": chunk})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	})

	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"%s\"}\n\n", escapeJSON(err.Error()))
		c.Writer.Flush()
		return
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
		text := *note.ContentText
		if len(text) > 3000 {
			text = text[:3000] + "..."
		}
		sb.WriteString("\n\n")
		sb.WriteString(text)
	} else if note.ContentHTML != nil && *note.ContentHTML != "" {
		html := *note.ContentHTML
		if len(html) > 3000 {
			html = html[:3000] + "..."
		}
		// Strip HTML tags for cleaner AI prompt
		text := stripHTMLTags(html)
		sb.WriteString("\n\n")
		sb.WriteString(text)
	}
	return sb.String()
}

// stripHTMLTags removes HTML tags from a string.
func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(result.String())
}

// escapeJSON escapes a string for safe inclusion in a JSON value.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
