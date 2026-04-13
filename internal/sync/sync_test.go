package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"personal-kb/internal/nas"
	"personal-kb/internal/store"
)

// ---- Mock NAS Client ----

type mockNASClient struct {
	notebooks []nas.Notebook
	notes     []nas.Note
	listErr   error
}

func (m *mockNASClient) ListNotebooks() ([]nas.Notebook, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.notebooks, nil
}

func (m *mockNASClient) ListNotes(offset, limit int) (*nas.NoteListResponse, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	total := len(m.notes)
	if offset >= total {
		return &nas.NoteListResponse{Total: total, Notes: []nas.Note{}}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return &nas.NoteListResponse{
		Total: total,
		Notes: m.notes[offset:end],
	}, nil
}

func (m *mockNASClient) GetNote(noteID string) (*nas.Note, error) {
	for _, n := range m.notes {
		if n.ID == noteID {
			return &n, nil
		}
	}
	return nil, fmt.Errorf("note not found: %s", noteID)
}

// ---- Mock Store ----

type mockStore struct {
	notebooks map[string]*store.Notebook
	notes     map[string]*store.Note
}

func newMockStore() *mockStore {
	return &mockStore{
		notebooks: make(map[string]*store.Notebook),
		notes:     make(map[string]*store.Note),
	}
}

func (m *mockStore) SaveNote(_ context.Context, note *store.Note) error {
	m.notes[note.ID] = note
	return nil
}

func (m *mockStore) GetNote(_ context.Context, id string) (*store.Note, error) {
	note, ok := m.notes[id]
	if !ok {
		return nil, fmt.Errorf("note not found: %s", id)
	}
	return note, nil
}

func (m *mockStore) ListNotes(_ context.Context, _ string, offset, limit int) ([]store.Note, error) {
	var all []store.Note
	for _, n := range m.notes {
		all = append(all, *n)
	}
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (m *mockStore) DeleteNote(_ context.Context, id string) error {
	delete(m.notes, id)
	return nil
}

func (m *mockStore) SaveNotebook(_ context.Context, nb *store.Notebook) error {
	m.notebooks[nb.ID] = nb
	return nil
}

func (m *mockStore) ListNotebooks(_ context.Context) ([]store.Notebook, error) {
	var all []store.Notebook
	for _, nb := range m.notebooks {
		all = append(all, *nb)
	}
	return all, nil
}

func (m *mockStore) DeleteNotebook(_ context.Context, id string) error {
	delete(m.notebooks, id)
	return nil
}

// ---- Helper: create a sync service with mocks ----

func newTestSyncService(nasClient *mockNASClient, store *mockStore) *SyncService {
	return &SyncService{
		nasClient: nasClient,
		store:     store,
	}
}

// ---- Tests ----

func TestFullSync_SyncNotebooks(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{
			{ID: "nb1", Title: "Personal", CreatedTime: 1000, ModifiedTime: 2000},
			{ID: "nb2", Title: "Work", ParentID: "nb1", CreatedTime: 3000, ModifiedTime: 4000},
		},
		notes: []nas.Note{},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	if err := svc.FullSync(ctx); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	if len(st.notebooks) != 2 {
		t.Errorf("expected 2 notebooks, got %d", len(st.notebooks))
	}

	nb1, ok := st.notebooks["nb1"]
	if !ok {
		t.Fatal("notebook nb1 not found")
	}
	if nb1.Title != "Personal" {
		t.Errorf("nb1 title = %q, want %q", nb1.Title, "Personal")
	}
	if nb1.ParentID != nil {
		t.Errorf("nb1 parent_id should be nil, got %q", *nb1.ParentID)
	}

	nb2, ok := st.notebooks["nb2"]
	if !ok {
		t.Fatal("notebook nb2 not found")
	}
	if nb2.ParentID == nil || *nb2.ParentID != "nb1" {
		t.Errorf("nb2 parent_id = %v, want %q", nb2.ParentID, "nb1")
	}
}

func TestFullSync_SyncNotes(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes: []nas.Note{
			{
				ID: "n1", NotebookID: "nb1", Title: "First",
				ContentHTML: "<h1>Hello</h1><p>World</p>",
				Tags:        []string{"test", "important"},
				CreatedTime: 1000, ModifiedTime: 2000,
			},
			{
				ID: "n2", NotebookID: "nb1", Title: "Second",
				ContentHTML: "<p>Just text</p>",
				CreatedTime: 3000, ModifiedTime: 4000,
			},
		},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	if err := svc.FullSync(ctx); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	if len(st.notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(st.notes))
	}

	note1, ok := st.notes["n1"]
	if !ok {
		t.Fatal("note n1 not found")
	}
	if note1.Title != "First" {
		t.Errorf("note1 title = %q, want %q", note1.Title, "First")
	}
	if note1.ContentHTML == nil || *note1.ContentHTML != "<h1>Hello</h1><p>World</p>" {
		t.Errorf("note1 content_html = %v", note1.ContentHTML)
	}
	if note1.ContentText == nil || *note1.ContentText == "" {
		t.Error("note1 content_text should not be empty")
	}
	if note1.Tags == nil {
		t.Error("note1 tags should not be nil")
	} else {
		var tags []string
		if err := json.Unmarshal([]byte(*note1.Tags), &tags); err != nil {
			t.Fatalf("unmarshal tags: %v", err)
		}
		if len(tags) != 2 || tags[0] != "test" || tags[1] != "important" {
			t.Errorf("tags = %v, want [test important]", tags)
		}
	}
	if note1.NotebookID == nil || *note1.NotebookID != "nb1" {
		t.Errorf("note1 notebook_id = %v, want nb1", note1.NotebookID)
	}
	if note1.SyncedAt == nil {
		t.Error("note1 synced_at should be set")
	}
}

func TestFullSync_DeleteOrphanedNotes(t *testing.T) {
	st := newMockStore()
	ctx := context.Background()

	// Pre-populate a local note that won't exist on NAS
	orphanID := "orphan-note"
	syncedAt := int64(1000)
	st.notes[orphanID] = &store.Note{
		ID: orphanID, Title: "Orphan", SyncedAt: &syncedAt,
		CreatedTime: 1000, ModifiedTime: 1000,
	}
	// Pre-populate a note that will exist on NAS
	st.notes["n1"] = &store.Note{
		ID: "n1", Title: "Keep Me", SyncedAt: &syncedAt,
		CreatedTime: 1000, ModifiedTime: 1000,
	}

	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes: []nas.Note{
			{ID: "n1", Title: "Keep Me", CreatedTime: 1000, ModifiedTime: 2000},
		},
	}

	svc := newTestSyncService(nasClient, st)

	if err := svc.FullSync(ctx); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	// Orphan should be deleted
	if _, ok := st.notes[orphanID]; ok {
		t.Error("orphan note should have been deleted")
	}

	// n1 should still exist
	if _, ok := st.notes["n1"]; !ok {
		t.Error("note n1 should still exist")
	}
}

func TestFullSync_Pagination(t *testing.T) {
	// Create 250 notes to test pagination (3 pages: 100+100+50)
	var notes []nas.Note
	for i := 0; i < 250; i++ {
		notes = append(notes, nas.Note{
			ID: fmt.Sprintf("note-%d", i),
			Title: fmt.Sprintf("Note %d", i),
			CreatedTime: int64(i),
			ModifiedTime: int64(i),
		})
	}

	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes:     notes,
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	if err := svc.FullSync(ctx); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	if len(st.notes) != 250 {
		t.Errorf("expected 250 notes, got %d", len(st.notes))
	}
}

func TestFullSync_NASError(t *testing.T) {
	nasClient := &mockNASClient{
		listErr: fmt.Errorf("NAS unavailable"),
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	err := svc.FullSync(ctx)
	if err == nil {
		t.Fatal("expected error when NAS is unavailable")
	}
}

func TestIncrementalSync_NewNote(t *testing.T) {
	st := newMockStore()
	ctx := context.Background()

	// Pre-populate an existing note with synced_at in the future
	syncedAt := int64(5000)
	st.notes["existing"] = &store.Note{
		ID: "existing", Title: "Existing Note",
		SyncedAt: &syncedAt, CreatedTime: 1000, ModifiedTime: 2000,
	}

	nasClient := &mockNASClient{
		notes: []nas.Note{
			{
				ID: "existing", Title: "Existing Note",
				ModifiedTime: 2000, // Same as local, no update needed
			},
			{
				ID: "new-note", Title: "New Note", NotebookID: "nb1",
				ContentHTML: "<p>New content</p>",
				ModifiedTime: 3000,
			},
		},
	}

	svc := newTestSyncService(nasClient, st)

	if err := svc.IncrementalSync(ctx); err != nil {
		t.Fatalf("IncrementalSync error: %v", err)
	}

	// New note should be added
	if _, ok := st.notes["new-note"]; !ok {
		t.Error("new note should have been added")
	}

	// Existing note should NOT be updated (modified_time not newer than synced_at)
	existing := st.notes["existing"]
	if existing.Title != "Existing Note" {
		t.Errorf("existing note title should not change, got %q", existing.Title)
	}
}

func TestIncrementalSync_UpdatedNote(t *testing.T) {
	st := newMockStore()
	ctx := context.Background()

	// Pre-populate a note with synced_at = 1000
	syncedAt := int64(1000)
	st.notes["n1"] = &store.Note{
		ID: "n1", Title: "Old Title",
		SyncedAt: &syncedAt, CreatedTime: 500, ModifiedTime: 800,
	}

	nasClient := &mockNASClient{
		notes: []nas.Note{
			{
				ID: "n1", Title: "Updated Title",
				ContentHTML: "<p>Updated</p>",
				ModifiedTime: 2000, // Newer than synced_at
			},
		},
	}

	svc := newTestSyncService(nasClient, st)

	if err := svc.IncrementalSync(ctx); err != nil {
		t.Fatalf("IncrementalSync error: %v", err)
	}

	// Note should be updated
	note := st.notes["n1"]
	if note.Title != "Updated Title" {
		t.Errorf("note title = %q, want %q", note.Title, "Updated Title")
	}
	if note.ContentHTML == nil || *note.ContentHTML != "<p>Updated</p>" {
		t.Errorf("note content should be updated")
	}
}

func TestIncrementalSync_DeleteRemovedNotes(t *testing.T) {
	st := newMockStore()
	ctx := context.Background()

	syncedAt := int64(5000)
	st.notes["keep"] = &store.Note{
		ID: "keep", Title: "Keep", SyncedAt: &syncedAt,
		CreatedTime: 1000, ModifiedTime: 1000,
	}
	st.notes["remove"] = &store.Note{
		ID: "remove", Title: "Remove", SyncedAt: &syncedAt,
		CreatedTime: 1000, ModifiedTime: 1000,
	}

	nasClient := &mockNASClient{
		notes: []nas.Note{
			{ID: "keep", Title: "Keep", ModifiedTime: 1000},
		},
	}

	svc := newTestSyncService(nasClient, st)

	if err := svc.IncrementalSync(ctx); err != nil {
		t.Fatalf("IncrementalSync error: %v", err)
	}

	if _, ok := st.notes["remove"]; ok {
		t.Error("removed note should have been deleted")
	}
	if _, ok := st.notes["keep"]; !ok {
		t.Error("kept note should still exist")
	}
}

func TestIncrementalSync_NoChanges(t *testing.T) {
	st := newMockStore()
	ctx := context.Background()

	syncedAt := int64(5000)
	st.notes["n1"] = &store.Note{
		ID: "n1", Title: "Unchanged",
		SyncedAt: &syncedAt, CreatedTime: 1000, ModifiedTime: 2000,
	}

	nasClient := &mockNASClient{
		notes: []nas.Note{
			{ID: "n1", Title: "Unchanged", ModifiedTime: 2000}, // Same time, no update
		},
	}

	svc := newTestSyncService(nasClient, st)

	if err := svc.IncrementalSync(ctx); err != nil {
		t.Fatalf("IncrementalSync error: %v", err)
	}

	// Title should remain unchanged (SaveNote not called for unchanged note)
	note := st.notes["n1"]
	if note.Title != "Unchanged" {
		t.Errorf("note title should be unchanged, got %q", note.Title)
	}
}

func TestNasToStoreNotebook_EmptyParentID(t *testing.T) {
	nb := &nas.Notebook{ID: "nb1", Title: "Test", ParentID: ""}
	result := nasToStoreNotebook(nb)

	if result.ParentID != nil {
		t.Errorf("ParentID should be nil for empty string, got %v", result.ParentID)
	}
}

func TestNasToStoreNotebook_WithParentID(t *testing.T) {
	nb := &nas.Notebook{ID: "nb1", Title: "Test", ParentID: "parent1"}
	result := nasToStoreNotebook(nb)

	if result.ParentID == nil || *result.ParentID != "parent1" {
		t.Errorf("ParentID = %v, want parent1", result.ParentID)
	}
}

func TestNasToStoreNote_EmptyContent(t *testing.T) {
	note := &nas.Note{
		ID: "n1", Title: "Empty", ContentHTML: "",
		CreatedTime: 1000, ModifiedTime: 2000,
	}
	result := nasToStoreNote(note, 3000)

	if result.ContentText != nil {
		t.Errorf("ContentText should be nil for empty content, got %v", result.ContentText)
	}
	if result.Tags != nil {
		t.Errorf("Tags should be nil for empty tags, got %v", result.Tags)
	}
}

func TestNasToStoreNote_WithTags(t *testing.T) {
	note := &nas.Note{
		ID: "n1", Title: "Tagged", ContentHTML: "<p>text</p>",
		Tags: []string{"go", "test"},
		CreatedTime: 1000, ModifiedTime: 2000,
	}
	result := nasToStoreNote(note, 3000)

	if result.Tags == nil {
		t.Fatal("Tags should not be nil")
	}
	var tags []string
	if err := json.Unmarshal([]byte(*result.Tags), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "go" || tags[1] != "test" {
		t.Errorf("tags = %v, want [go test]", tags)
	}
}

func TestNasToStoreNote_HTMLToText(t *testing.T) {
	note := &nas.Note{
		ID: "n1", Title: "HTML Note",
		ContentHTML: "<h1>Title</h1><p>Paragraph</p>",
		CreatedTime: 1000, ModifiedTime: 2000,
	}
	result := nasToStoreNote(note, 3000)

	if result.ContentText == nil {
		t.Fatal("ContentText should not be nil")
	}
	if *result.ContentText == "" {
		t.Error("ContentText should contain extracted text")
	}
}

func TestNeedsUpdate_NewNote(t *testing.T) {
	st := newMockStore()
	nasClient := &mockNASClient{}
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	nasNote := &nas.Note{ID: "new-note", ModifiedTime: 1000}
	needsUpdate, err := svc.needsUpdate(ctx, nasNote)
	if err != nil {
		t.Fatalf("needsUpdate error: %v", err)
	}
	if !needsUpdate {
		t.Error("new note should need update")
	}
}

func TestNeedsUpdate_NoSyncedAt(t *testing.T) {
	st := newMockStore()
	st.notes["n1"] = &store.Note{ID: "n1", Title: "Test", SyncedAt: nil}
	nasClient := &mockNASClient{}
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	nasNote := &nas.Note{ID: "n1", ModifiedTime: 1000}
	needsUpdate, err := svc.needsUpdate(ctx, nasNote)
	if err != nil {
		t.Fatalf("needsUpdate error: %v", err)
	}
	if !needsUpdate {
		t.Error("note with no synced_at should need update")
	}
}

func TestNeedsUpdate_ModifiedTimeNewer(t *testing.T) {
	syncedAt := int64(1000)
	st := newMockStore()
	st.notes["n1"] = &store.Note{ID: "n1", Title: "Test", SyncedAt: &syncedAt}
	nasClient := &mockNASClient{}
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	nasNote := &nas.Note{ID: "n1", ModifiedTime: 2000}
	needsUpdate, err := svc.needsUpdate(ctx, nasNote)
	if err != nil {
		t.Fatalf("needsUpdate error: %v", err)
	}
	if !needsUpdate {
		t.Error("note with newer modified_time should need update")
	}
}

func TestNeedsUpdate_ModifiedTimeOlder(t *testing.T) {
	syncedAt := int64(3000)
	st := newMockStore()
	st.notes["n1"] = &store.Note{ID: "n1", Title: "Test", SyncedAt: &syncedAt}
	nasClient := &mockNASClient{}
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	nasNote := &nas.Note{ID: "n1", ModifiedTime: 2000}
	needsUpdate, err := svc.needsUpdate(ctx, nasNote)
	if err != nil {
		t.Fatalf("needsUpdate error: %v", err)
	}
	if needsUpdate {
		t.Error("note with older modified_time should not need update")
	}
}

func TestNeedsUpdate_ModifiedTimeEqual(t *testing.T) {
	syncedAt := int64(2000)
	st := newMockStore()
	st.notes["n1"] = &store.Note{ID: "n1", Title: "Test", SyncedAt: &syncedAt}
	nasClient := &mockNASClient{}
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	nasNote := &nas.Note{ID: "n1", ModifiedTime: 2000}
	needsUpdate, err := svc.needsUpdate(ctx, nasNote)
	if err != nil {
		t.Fatalf("needsUpdate error: %v", err)
	}
	if needsUpdate {
		t.Error("note with equal modified_time should not need update")
	}
}

func TestFullSync_EmptyNAS(t *testing.T) {
	nasClient := &mockNASClient{
		notebooks: []nas.Notebook{},
		notes:     []nas.Note{},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	// Pre-populate a local note
	syncedAt := int64(1000)
	st.notes["local-only"] = &store.Note{
		ID: "local-only", Title: "Local", SyncedAt: &syncedAt,
		CreatedTime: 500, ModifiedTime: 500,
	}

	if err := svc.FullSync(ctx); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	// Local-only note should be deleted since NAS is empty
	if len(st.notes) != 0 {
		t.Errorf("expected 0 notes after full sync with empty NAS, got %d", len(st.notes))
	}
}

func TestFullSync_NoteWithoutNotebookID(t *testing.T) {
	nasClient := &mockNASClient{
		notes: []nas.Note{
			{ID: "n1", Title: "No Notebook", NotebookID: "", ContentHTML: "<p>test</p>"},
		},
	}
	st := newMockStore()
	svc := newTestSyncService(nasClient, st)
	ctx := context.Background()

	if err := svc.FullSync(ctx); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	note, ok := st.notes["n1"]
	if !ok {
		t.Fatal("note n1 not found")
	}
	if note.NotebookID != nil {
		t.Errorf("NotebookID should be nil for empty notebook_id, got %v", note.NotebookID)
	}
}
