package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// testDB opens an in-memory database and runs migrations.
func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func ptrStr(s string) *string { return &s }

// ---- Settings ----

func TestSettingsCRUD(t *testing.T) {
	db := testDB(t)
	s := NewSettingsStore(db)

	// Set and get a plain setting.
	if err := s.SetSetting("theme", "dark"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, err := s.GetSetting("theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "dark" {
		t.Errorf("got %q, want %q", val, "dark")
	}

	// Update an existing setting.
	if err := s.SetSetting("theme", "light"); err != nil {
		t.Fatalf("SetSetting update: %v", err)
	}
	val, err = s.GetSetting("theme")
	if err != nil {
		t.Fatalf("GetSetting after update: %v", err)
	}
	if val != "light" {
		t.Errorf("got %q, want %q", val, "light")
	}

	// Non-existent key should error.
	_, err = s.GetSetting("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestSettingsEncryption(t *testing.T) {
	db := testDB(t)
	s := NewSettingsStore(db)

	secret := "my-super-secret-value"
	if err := s.SetSetting("api_key_encrypted", secret); err != nil {
		t.Fatalf("SetSetting encrypted: %v", err)
	}

	// Verify the raw value in DB is NOT the plaintext.
	var raw string
	if err := db.Get(&raw, "SELECT value FROM settings WHERE key = 'api_key_encrypted'"); err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if raw == secret {
		t.Error("encrypted value stored as plaintext!")
	}

	// Decrypt via GetSetting.
	val, err := s.GetSetting("api_key_encrypted")
	if err != nil {
		t.Fatalf("GetSetting encrypted: %v", err)
	}
	if val != secret {
		t.Errorf("got %q, want %q", val, secret)
	}
}

// ---- Notes & Notebooks ----

func TestSaveAndGetNote(t *testing.T) {
	db := testDB(t)
	s := NewNotesStore(db)
	ctx := context.Background()

	note := &Note{
		ID:           uuid.New().String(),
		Title:        "Test Note",
		ContentHTML:  ptrStr("<p>Hello</p>"),
		ContentText:  ptrStr("Hello"),
		Tags:         ptrStr(`["test"]`),
		CreatedTime:  1000,
		ModifiedTime: 2000,
	}

	if err := s.SaveNote(ctx, note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	got, err := s.GetNote(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got.Title != note.Title {
		t.Errorf("title: got %q, want %q", got.Title, note.Title)
	}
	if got.ContentHTML == nil || *got.ContentHTML != "<p>Hello</p>" {
		t.Errorf("content_html: got %v", got.ContentHTML)
	}
}

func TestUpdateNote(t *testing.T) {
	db := testDB(t)
	s := NewNotesStore(db)
	ctx := context.Background()

	note := &Note{
		ID:           uuid.New().String(),
		Title:        "Original",
		CreatedTime:  1000,
		ModifiedTime: 1000,
	}
	if err := s.SaveNote(ctx, note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	note.Title = "Updated"
	note.ModifiedTime = 2000
	if err := s.SaveNote(ctx, note); err != nil {
		t.Fatalf("SaveNote update: %v", err)
	}

	got, err := s.GetNote(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("got %q, want %q", got.Title, "Updated")
	}
}

func TestListNotes(t *testing.T) {
	db := testDB(t)
	s := NewNotesStore(db)
	ctx := context.Background()

	nbID := uuid.New().String()
	for i := 0; i < 5; i++ {
		if err := s.SaveNote(ctx, &Note{
			ID:           uuid.New().String(),
			NotebookID:   &nbID,
			Title:        "Note",
			CreatedTime:  int64(i),
			ModifiedTime: int64(i),
		}); err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
	}

	// Paginate: first 3.
	notes, err := s.ListNotes(ctx, nbID, 0, 3)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 3 {
		t.Errorf("got %d notes, want 3", len(notes))
	}

	// Paginate: next page.
	notes2, err := s.ListNotes(ctx, nbID, 3, 3)
	if err != nil {
		t.Fatalf("ListNotes page 2: %v", err)
	}
	if len(notes2) != 2 {
		t.Errorf("got %d notes, want 2", len(notes2))
	}

	// List all notebooks filter.
	allNotes, err := s.ListNotes(ctx, "", 0, 100)
	if err != nil {
		t.Fatalf("ListNotes all: %v", err)
	}
	if len(allNotes) != 5 {
		t.Errorf("got %d notes, want 5", len(allNotes))
	}
}

func TestDeleteNote(t *testing.T) {
	db := testDB(t)
	s := NewNotesStore(db)
	ctx := context.Background()

	note := &Note{ID: uuid.New().String(), Title: "To Delete", CreatedTime: 1, ModifiedTime: 1}
	if err := s.SaveNote(ctx, note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := s.DeleteNote(ctx, note.ID); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	if _, err := s.GetNote(ctx, note.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestNotebooks(t *testing.T) {
	db := testDB(t)
	s := NewNotesStore(db)
	ctx := context.Background()

	nb := &Notebook{
		ID:           uuid.New().String(),
		Title:        "My Notebook",
		CreatedTime:  1000,
		ModifiedTime: 1000,
	}
	if err := s.SaveNotebook(ctx, nb); err != nil {
		t.Fatalf("SaveNotebook: %v", err)
	}

	nbs, err := s.ListNotebooks(ctx)
	if err != nil {
		t.Fatalf("ListNotebooks: %v", err)
	}
	if len(nbs) != 1 || nbs[0].Title != "My Notebook" {
		t.Errorf("got %v", nbs)
	}

	// Update.
	nb.Title = "Renamed"
	nb.ModifiedTime = 2000
	if err := s.SaveNotebook(ctx, nb); err != nil {
		t.Fatalf("SaveNotebook update: %v", err)
	}
	nbs, _ = s.ListNotebooks(ctx)
	if nbs[0].Title != "Renamed" {
		t.Errorf("got %q", nbs[0].Title)
	}

	// Delete.
	if err := s.DeleteNotebook(ctx, nb.ID); err != nil {
		t.Fatalf("DeleteNotebook: %v", err)
	}
	nbs, _ = s.ListNotebooks(ctx)
	if len(nbs) != 0 {
		t.Errorf("expected 0 notebooks, got %d", len(nbs))
	}
}

// ---- Conversations & Messages ----

func TestConversationCRUD(t *testing.T) {
	db := testDB(t)
	s := NewConversationsStore(db)
	ctx := context.Background()

	convID, err := s.CreateConversation(ctx, nil, "Test Chat")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if convID <= 0 {
		t.Errorf("expected positive ID, got %d", convID)
	}

	conv, err := s.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if conv.Title != "Test Chat" {
		t.Errorf("got title %q", conv.Title)
	}
}

func TestConversationWithNote(t *testing.T) {
	db := testDB(t)
	s := NewConversationsStore(db)
	ctx := context.Background()

	noteID := "note-123"
	convID, err := s.CreateConversation(ctx, &noteID, "Note Chat")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	convs, err := s.ListConversations(ctx, &noteID)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 1 || convs[0].ID != convID {
		t.Errorf("got %v", convs)
	}

	// Filter by different note should return empty.
	other := "other-note"
	convs2, err := s.ListConversations(ctx, &other)
	if err != nil {
		t.Fatalf("ListConversations other: %v", err)
	}
	if len(convs2) != 0 {
		t.Errorf("expected 0, got %d", len(convs2))
	}
}

func TestMessages(t *testing.T) {
	db := testDB(t)
	s := NewConversationsStore(db)
	ctx := context.Background()

	convID, _ := s.CreateConversation(ctx, nil, "Msg Test")

	if err := s.AddMessage(ctx, convID, "system", "You are helpful."); err != nil {
		t.Fatalf("AddMessage system: %v", err)
	}
	if err := s.AddMessage(ctx, convID, "user", "Hello"); err != nil {
		t.Fatalf("AddMessage user: %v", err)
	}
	if err := s.AddMessage(ctx, convID, "assistant", "Hi there!"); err != nil {
		t.Fatalf("AddMessage assistant: %v", err)
	}

	msgs, err := s.GetMessages(ctx, convID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	// Verify order.
	if msgs[0].Role != "system" || msgs[1].Role != "user" || msgs[2].Role != "assistant" {
		t.Errorf("order wrong: %v", msgs)
	}
}

func TestDeleteConversation(t *testing.T) {
	db := testDB(t)
	s := NewConversationsStore(db)
	ctx := context.Background()

	convID, _ := s.CreateConversation(ctx, nil, "To Delete")
	s.AddMessage(ctx, convID, "user", "bye")

	if err := s.DeleteConversation(ctx, convID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if _, err := s.GetConversation(ctx, convID); err == nil {
		t.Error("expected error for deleted conversation")
	}
	msgs, err := s.GetMessages(ctx, convID)
	if err != nil {
		t.Fatalf("GetMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after delete, got %d", len(msgs))
	}
}

// TestMain sets up a stable encryption key for all tests.
func TestMain(m *testing.M) {
	os.Setenv("KB_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	globalKey = deriveKey()
	os.Exit(m.Run())
}
