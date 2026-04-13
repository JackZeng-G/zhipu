package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"personal-kb/internal/ollama"
	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	dbPath := t.TempDir() + "/test.db"
	db, err := store.Open(dbPath)
	require.NoError(t, err)
	err = store.Migrate(db)
	require.NoError(t, err)
	return db
}

func setupTestRouter(t *testing.T) (*gin.Engine, *sqlx.DB) {
	db := setupTestDB(t)

	notesStore := store.NewNotesStore(db)
	settingsStore := store.NewSettingsStore(db)
	convStore := store.NewConversationsStore(db)
	aiConfigStore := store.NewAIConfigStore(db, settingsStore)
	knowledgeStore := store.NewKnowledgeStore(db)
	ollamaClient := ollama.NewClient("http://localhost:11434", "test-model")

	handlers := NewHandlers(
		notesStore,
		settingsStore,
		convStore,
		aiConfigStore,
		knowledgeStore,
		nil, // nasClient
		nil, // authClient
		ollamaClient,
		nil, // syncService
	)

	gin.SetMode(gin.TestMode)
	r := SetupRouter(handlers)
	return r, db
}

func TestHealthEndpoint(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.NotNil(t, response["time"])
}

func TestListNotebooks_Empty(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/notebooks", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var notebooks []store.Notebook
	err := json.Unmarshal(w.Body.Bytes(), &notebooks)
	require.NoError(t, err)
	assert.Empty(t, notebooks)
}

func TestListNotebooks_WithData(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	ctx := context.Background()
	notesStore := store.NewNotesStore(db)
	err := notesStore.SaveNotebook(ctx, &store.Notebook{
		ID:    "nb-1",
		Title: "Test Notebook",
	})
	require.NoError(t, err)
	// Save a note in the notebook so ListNotebooksWithNotes returns it
	nbID := "nb-1"
	err = notesStore.SaveNote(ctx, &store.Note{
		ID:          "note-1",
		Title:       "Test Note",
		NotebookID:  &nbID,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/notebooks", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var notebooks []store.Notebook
	err = json.Unmarshal(w.Body.Bytes(), &notebooks)
	require.NoError(t, err)
	assert.Len(t, notebooks, 1)
	assert.Equal(t, "nb-1", notebooks[0].ID)
	assert.Equal(t, "Test Notebook", notebooks[0].Title)
}

func TestListNotes_Empty(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/notes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response listNotesResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 0, response.Total)
	assert.Empty(t, response.Items)
}

func TestListNotes_WithData(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	ctx := context.Background()
	notesStore := store.NewNotesStore(db)
	err := notesStore.SaveNote(ctx, &store.Note{
		ID:    "note-1",
		Title: "Test Note",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/notes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response listNotesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 1, response.Total)
	assert.Len(t, response.Items, 1)
	assert.Equal(t, "note-1", response.Items[0].ID)
	assert.Equal(t, "Test Note", response.Items[0].Title)
}

func TestGetNote_NotFound(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/notes/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetNote_Found(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	ctx := context.Background()
	notesStore := store.NewNotesStore(db)
	err := notesStore.SaveNote(ctx, &store.Note{
		ID:    "note-1",
		Title: "Test Note",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/notes/note-1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var note store.Note
	err = json.Unmarshal(w.Body.Bytes(), &note)
	require.NoError(t, err)
	assert.Equal(t, "note-1", note.ID)
	assert.Equal(t, "Test Note", note.Title)
}

func TestGetSettings(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/settings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response settingsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", response.OllamaURL)
	assert.Equal(t, "qwen2", response.OllamaModel)
}

func TestUpdateSettings(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	payload := updateSettingsRequest{
		OllamaURL:   "http://192.168.1.100:11434",
		OllamaModel: "llama2",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	// Verify settings were saved
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/settings", nil)
	r.ServeHTTP(w, req)

	var settings settingsResponse
	err = json.Unmarshal(w.Body.Bytes(), &settings)
	require.NoError(t, err)
	assert.Equal(t, "http://192.168.1.100:11434", settings.OllamaURL)
	assert.Equal(t, "llama2", settings.OllamaModel)
}

func TestNASStatus_NotConnected(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/nas/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["connected"].(bool))
}

func TestNASDisconnect(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/nas/disconnect", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestNASSync_NotConnected(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/nas/sync", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListConversations_Empty(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/ai/conversations", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var conversations []store.Conversation
	err := json.Unmarshal(w.Body.Bytes(), &conversations)
	require.NoError(t, err)
	assert.Empty(t, conversations)
}

func TestCreateConversation(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	ctx := context.Background()
	notesStore := store.NewNotesStore(db)
	err := notesStore.SaveNote(ctx, &store.Note{
		ID:    "note-1",
		Title: "Test Note",
	})
	require.NoError(t, err)

	payload := createConversationRequest{NoteID: "note-1"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/ai/conversations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var conv store.Conversation
	err = json.Unmarshal(w.Body.Bytes(), &conv)
	require.NoError(t, err)
	assert.NotZero(t, conv.ID)
	assert.Equal(t, "Chat about Test Note", conv.Title)
}

func TestCORSHeaders(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/api/notes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestNASConnect_InvalidRequest(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	body := []byte(`{"host": "192.168.1.100"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/nas/connect", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAISearch_InvalidRequest(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/ai/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAIEdit_InvalidRequest(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	body := []byte(`{"note_id": "note-1"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/ai/edit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSendMessage_InvalidConversationID(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	body := []byte(`{"content": "Hello"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/ai/conversations/invalid/messages", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSendMessage_ConversationNotFound(t *testing.T) {
	r, db := setupTestRouter(t)
	defer db.Close()

	body := []byte(`{"content": "Hello"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/ai/conversations/999/messages", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Returns 400 because no AI provider is configured (checked before conversation lookup)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseSearchResults(t *testing.T) {
	notes := []store.Note{
		{ID: "note-1", Title: "Note 1"},
		{ID: "note-2", Title: "Note 2"},
	}

	tests := []struct {
		name     string
		response string
		wantLen  int
	}{
		{
			name:     "valid JSON array",
			response: `[{"note_id": "note-1", "reason": "relevant"}]`,
			wantLen:  1,
		},
		{
			name:     "valid with extra text",
			response: `Here are the results: [{"note_id": "note-1", "reason": "relevant"}]`,
			wantLen:  1,
		},
		{
			name:     "empty JSON array",
			response: `[]`,
			wantLen:  0,
		},
		{
			name:     "invalid JSON",
			response: `not valid json`,
			wantLen:  0,
		},
		{
			name:     "non-existent note_id",
			response: `[{"note_id": "nonexistent", "reason": "relevant"}]`,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseSearchResults(tt.response, notes)
			assert.Len(t, results, tt.wantLen)
		})
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`simple text`, `simple text`},
		{`text with "quotes"`, `text with \"quotes\"`},
		{`text with \ backslash`, `text with \\ backslash`},
		{"text with\nnewline", `text with\nnewline`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildNoteContent(t *testing.T) {
	textContent := "This is the text content"
	htmlContent := "<p>This is HTML content</p>"

	tests := []struct {
		name     string
		note     *store.Note
		expected string
	}{
		{
			name: "with text content",
			note: &store.Note{
				ID:          "note-1",
				Title:       "Test Note",
				ContentText: &textContent,
			},
			expected: "Test Note\n\nThis is the text content",
		},
		{
			name: "with HTML content only",
			note: &store.Note{
				ID:          "note-1",
				Title:       "Test Note",
				ContentHTML: &htmlContent,
			},
			expected: "Test Note\n\nThis is HTML content",
		},
		{
			name: "title only",
			note: &store.Note{
				ID:    "note-1",
				Title: "Test Note",
			},
			expected: "Test Note",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildNoteContent(tt.note)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
