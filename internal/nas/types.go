package nas

import "encoding/json"

// Notebook represents a notebook (folder) from Synology Note Station.
type Notebook struct {
	ID            string `json:"object_id"`
	Title         string `json:"title"`
	ParentID      string `json:"-"`
	CreatedTime   int64  `json:"ctime"`
	ModifiedTime  int64  `json:"mtime"`
}

// Note represents a single note from Synology Note Station.
type Note struct {
	ID            string   `json:"object_id"`
	NotebookID    string   `json:"-"`
	Title         string   `json:"title"`
	ContentHTML   string   `json:"body"`
	Tags          []string `json:"tags"`
	CreatedTime   int64    `json:"ctime"`
	ModifiedTime  int64    `json:"mtime"`
	Ver           string   `json:"ver"`
	LinkID        string   `json:"link_id"`
	Attachments   map[string]Attachment `json:"attachment"`
}

// Attachment represents a file attachment in a note.
type Attachment struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Ref      string `json:"ref"`
	Type     string `json:"type"`
	Ext      string `json:"ext"`
	Size     int64  `json:"size"`
}

// NoteListResponse is the paginated response from listing notes.
type NoteListResponse struct {
	Total int    `json:"total"`
	Notes []Note `json:"items"`
}

// notebookListResponse matches the actual Synology API response for notebooks.
type notebookListResponse struct {
	Notebooks []rawNotebook `json:"notebooks"`
	Total     int           `json:"total"`
	Offset    int           `json:"offset"`
}

// rawNotebook matches the actual JSON structure from Synology Note Station.
type rawNotebook struct {
	ObjectID     string `json:"object_id"`
	Title        string `json:"title"`
	CTime        int64  `json:"ctime"`
	MTime        int64  `json:"mtime"`
	Category     string `json:"category"`
	Stack        string `json:"stack"`
}

// noteListResponseWrapper matches the actual Synology API response for notes.
type noteListResponseWrapper struct {
	Total  int       `json:"total"`
	Offset int       `json:"offset"`
	Notes  []rawNote `json:"notes"`
}

// rawNote matches the actual JSON structure from Synology Note Station notes.
type rawNote struct {
	ObjectID   string `json:"object_id"`
	ParentID   string `json:"parent_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Body       string `json:"body"`
	Brief      string `json:"brief"`
	CTime      int64  `json:"ctime"`
	MTime      int64  `json:"mtime"`
	Ver        string `json:"ver"`
	LinkID     string `json:"link_id"`
}

// parseNotebooks converts raw API response to Notebook slice.
func parseNotebooks(data json.RawMessage) ([]Notebook, error) {
	var raw notebookListResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	notebooks := make([]Notebook, len(raw.Notebooks))
	for i, r := range raw.Notebooks {
		notebooks[i] = Notebook{
			ID:            r.ObjectID,
			Title:         r.Title,
			CreatedTime:   r.CTime,
			ModifiedTime:  r.MTime,
		}
	}
	return notebooks, nil
}

// parseNotes converts raw API response to NoteListResponse.
func parseNotes(data json.RawMessage) (*NoteListResponse, error) {
	var raw noteListResponseWrapper
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	notes := make([]Note, len(raw.Notes))
	for i, r := range raw.Notes {
		content := r.Content
		if content == "" {
			content = r.Body
		}
		notes[i] = Note{
			ID:            r.ObjectID,
			NotebookID:    r.ParentID,
			Title:         r.Title,
			ContentHTML:   content,
			CreatedTime:   r.CTime,
			ModifiedTime:  r.MTime,
		}
	}
	return &NoteListResponse{Total: raw.Total, Notes: notes}, nil
}

// ErrOTPRequired is returned when the NAS requires 2FA OTP code.
type ErrOTPRequired struct{}

func (e *ErrOTPRequired) Error() string { return "2FA OTP code required" }

// synoResponse is the common envelope for all Synology API responses.
type synoResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *synoError      `json:"error,omitempty"`
}

// synoError represents an error returned by the Synology API.
type synoError struct {
	Code int `json:"code"`
}

// authData is the data returned on successful login.
type authData struct {
	Did string `json:"did"`
	SID string `json:"sid"`
}
