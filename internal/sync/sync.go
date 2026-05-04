package sync

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"personal-kb/internal/nas"
	"personal-kb/internal/store"
)

// NASClient defines the interface for fetching data from the Synology NAS.
type NASClient interface {
	ListNotebooks() ([]nas.Notebook, error)
	ListNotes(offset, limit int) (*nas.NoteListResponse, error)
	GetNote(noteID string) (*nas.Note, error)
}

// Store defines the interface for persisting notes and notebooks.
type Store interface {
	SaveNote(ctx context.Context, note *store.Note) error
	GetNote(ctx context.Context, id string) (*store.Note, error)
	ListNotes(ctx context.Context, notebookID string, offset, limit int) ([]store.Note, error)
	DeleteNote(ctx context.Context, id string) error
	SaveNotebook(ctx context.Context, nb *store.Notebook) error
	ListNotebooks(ctx context.Context) ([]store.Notebook, error)
	DeleteNotebook(ctx context.Context, id string) error
}

// PostSyncHook is called after a sync completes with the IDs of synced notes.
type PostSyncHook func(syncedNoteIDs []string)

// SyncService synchronizes notes from Synology NAS to local SQLite database.
type SyncService struct {
	nasClient    NASClient
	store        Store
	postSyncHook PostSyncHook
}

// NewSyncService creates a new sync service.
func NewSyncService(nasClient *nas.NoteStationClient, store *store.NotesStore) *SyncService {
	return &SyncService{
		nasClient: nasClient,
		store:     store,
	}
}

// SetPostSyncHook sets a callback that runs after sync completes.
func (s *SyncService) SetPostSyncHook(hook PostSyncHook) {
	s.postSyncHook = hook
}

// FullSync performs a full synchronization of all notebooks and notes from NAS to local database.
// It fetches all data from NAS, saves it locally, and removes any local notes that no longer exist on NAS.
func (s *SyncService) FullSync(ctx context.Context) error {
	// Step 1: Fetch and save all notebooks
	nasNotebookIDs, err := s.syncNotebooks(ctx)
	if err != nil {
		return fmt.Errorf("sync notebooks: %w", err)
	}

	// Step 2: Delete local notebooks that no longer exist on NAS
	if err := s.deleteOrphanedNotebooks(ctx, nasNotebookIDs); err != nil {
		return fmt.Errorf("delete orphaned notebooks: %w", err)
	}

	// Step 3: Fetch and save all notes (paginated)
	nasNoteIDs, err := s.syncNotes(ctx)
	if err != nil {
		return fmt.Errorf("sync notes: %w", err)
	}

	// Step 4: Delete local notes that no longer exist on NAS
	if err := s.deleteOrphanedNotes(ctx, nasNoteIDs); err != nil {
		return fmt.Errorf("delete orphaned notes: %w", err)
	}

	// Trigger post-sync hook (e.g., auto-index)
	if s.postSyncHook != nil {
		ids := make([]string, 0, len(nasNoteIDs))
		for id := range nasNoteIDs {
			ids = append(ids, id)
		}
		go s.postSyncHook(ids)
	}

	return nil
}

// syncNotebooks fetches all notebooks from NAS and saves them to the local database.
// Returns the set of notebook IDs that exist on the NAS.
func (s *SyncService) syncNotebooks(ctx context.Context) (map[string]bool, error) {
	notebooks, err := s.nasClient.ListNotebooks()
	if err != nil {
		return nil, fmt.Errorf("fetch notebooks from NAS: %w", err)
	}

	nasIDs := make(map[string]bool, len(notebooks))
	for _, nb := range notebooks {
		nasIDs[nb.ID] = true
		storeNotebook := nasToStoreNotebook(&nb)
		if err := s.store.SaveNotebook(ctx, storeNotebook); err != nil {
			return nil, fmt.Errorf("save notebook %s: %w", nb.ID, err)
		}
	}

	return nasIDs, nil
}

// syncNotes fetches all notes from NAS (paginated) and saves them to the local database.
// Returns the set of note IDs that exist on the NAS.
func (s *SyncService) syncNotes(ctx context.Context) (map[string]bool, error) {
	const limit = 500
	offset := 0
	nasNoteIDs := make(map[string]bool)
	now := time.Now().Unix()

	for {
		resp, err := s.nasClient.ListNotes(offset, limit)
		if err != nil {
			return nil, fmt.Errorf("fetch notes from NAS (offset=%d): %w", offset, err)
		}

		for _, note := range resp.Notes {
			// Skip notes in Synology system notebook (e.g. "1029_#00000000" = drafts/untitled)
			if strings.HasSuffix(note.NotebookID, "_#00000000") {
				continue
			}

			nasNoteIDs[note.ID] = true
			// Fetch full note content
			fullNote, err := s.nasClient.GetNote(note.ID)
			if err != nil {
				return nil, fmt.Errorf("fetch note %s content: %w", note.ID, err)
			}
			storeNote := nasToStoreNote(fullNote, now)
			if err := s.store.SaveNote(ctx, storeNote); err != nil {
				return nil, fmt.Errorf("save note %s: %w", note.ID, err)
			}
		}

		// Check if we've fetched all notes
		if offset+len(resp.Notes) >= resp.Total {
			break
		}
		offset += limit
	}

	return nasNoteIDs, nil
}

// deleteOrphanedNotebooks removes local notebooks that no longer exist on the NAS.
func (s *SyncService) deleteOrphanedNotebooks(ctx context.Context, nasNotebookIDs map[string]bool) error {
	localNotebooks, err := s.store.ListNotebooks(ctx)
	if err != nil {
		return fmt.Errorf("list local notebooks: %w", err)
	}

	for _, nb := range localNotebooks {
		if !nasNotebookIDs[nb.ID] {
			if err := s.store.DeleteNotebook(ctx, nb.ID); err != nil {
				return fmt.Errorf("delete orphaned notebook %s: %w", nb.ID, err)
			}
		}
	}

	return nil
}

// deleteOrphanedNotes removes local notes that no longer exist on the NAS.
func (s *SyncService) deleteOrphanedNotes(ctx context.Context, nasNoteIDs map[string]bool) error {
	// Get all local notes
	localNotes, err := s.store.ListNotes(ctx, "", 0, 1000000) // Large limit to get all notes
	if err != nil {
		return fmt.Errorf("list local notes: %w", err)
	}

	for _, localNote := range localNotes {
		if !nasNoteIDs[localNote.ID] {
			if err := s.store.DeleteNote(ctx, localNote.ID); err != nil {
				return fmt.Errorf("delete orphaned note %s: %w", localNote.ID, err)
			}
		}
	}

	return nil
}

// IncrementalSync performs an incremental synchronization, only updating notes that have changed.
// It compares the modified_time of NAS notes with the local synced_at timestamp.
func (s *SyncService) IncrementalSync(ctx context.Context) error {
	const limit = 500
	offset := 0
	now := time.Now().Unix()
	nasNoteIDs := make(map[string]bool)
	var updatedIDs []string

	for {
		resp, err := s.nasClient.ListNotes(offset, limit)
		if err != nil {
			return fmt.Errorf("fetch notes from NAS (offset=%d): %w", offset, err)
		}

		for _, note := range resp.Notes {
			// Skip notes in Synology system notebook (e.g. "1029_#00000000" = drafts/untitled)
			if strings.HasSuffix(note.NotebookID, "_#00000000") {
				continue
			}

			nasNoteIDs[note.ID] = true

			needsUpdate, err := s.needsUpdate(ctx, &note)
			if err != nil {
				return fmt.Errorf("check if note %s needs update: %w", note.ID, err)
			}

			if needsUpdate {
				// Fetch full note content (list API returns abbreviated data)
				fullNote, err := s.nasClient.GetNote(note.ID)
				if err != nil {
					return fmt.Errorf("fetch note %s content: %w", note.ID, err)
				}
				storeNote := nasToStoreNote(fullNote, now)
				if err := s.store.SaveNote(ctx, storeNote); err != nil {
					return fmt.Errorf("save note %s: %w", note.ID, err)
				}
				updatedIDs = append(updatedIDs, note.ID)
			}
		}

		// Check if we've fetched all notes
		if offset+len(resp.Notes) >= resp.Total {
			break
		}
		offset += limit
	}

	// Delete notes that no longer exist on NAS
	if err := s.deleteOrphanedNotes(ctx, nasNoteIDs); err != nil {
		return fmt.Errorf("delete orphaned notes: %w", err)
	}

	// Trigger post-sync hook for updated notes
	if s.postSyncHook != nil && len(updatedIDs) > 0 {
		go s.postSyncHook(updatedIDs)
	}

	return nil
}

// needsUpdate determines if a note needs to be updated based on its modified time.
func (s *SyncService) needsUpdate(ctx context.Context, nasNote *nas.Note) (bool, error) {
	localNote, err := s.store.GetNote(ctx, nasNote.ID)
	if err != nil {
		// Note doesn't exist locally, needs to be created
		return true, nil
	}

	// If local note has no synced_at timestamp, update it
	if localNote.SyncedAt == nil {
		return true, nil
	}

	// Compare NAS modified_time with local synced_at
	// If NAS modified_time is newer, the note has changed and needs update
	return nasNote.ModifiedTime > *localNote.SyncedAt, nil
}

// nasToStoreNotebook converts a NAS notebook to a store notebook.
func nasToStoreNotebook(nb *nas.Notebook) *store.Notebook {
	var parentID *string
	if nb.ParentID != "" {
		parentID = &nb.ParentID
	}

	return &store.Notebook{
		ID:           nb.ID,
		Title:        nb.Title,
		ParentID:     parentID,
		CreatedTime:  nb.CreatedTime,
		ModifiedTime: nb.ModifiedTime,
	}
}

// nasToStoreNote converts a NAS note to a store note.
func nasToStoreNote(note *nas.Note, syncedAt int64) *store.Note {
	var notebookID *string
	if note.NotebookID != "" {
		notebookID = &note.NotebookID
	}

	// Convert HTML to text
	contentText := nas.HTMLToText(note.ContentHTML)
	var contentTextPtr *string
	if contentText != "" {
		contentTextPtr = &contentText
	}

	// Marshal tags to JSON
	var tagsPtr *string
	if len(note.Tags) > 0 {
		tagsJSON, _ := json.Marshal(note.Tags)
		tagsStr := string(tagsJSON)
		tagsPtr = &tagsStr
	}

	// Compute SHA-256 hash of content text for integrity checking
	var contentHash string
	if contentTextPtr != nil {
		contentHash = fmt.Sprintf("%x", sha256.Sum256([]byte(*contentTextPtr)))
	}

	contentHTML := note.ContentHTML
	return &store.Note{
		ID:           note.ID,
		NotebookID:   notebookID,
		Title:        note.Title,
		ContentHTML:  &contentHTML,
		ContentText:  contentTextPtr,
		Tags:         tagsPtr,
		ContentHash:  contentHash,
		CreatedTime:  note.CreatedTime,
		ModifiedTime: note.ModifiedTime,
		SyncedAt:     &syncedAt,
	}
}
