package nas

import "encoding/json"

// Notebook represents a notebook (folder) from Synology Note Station.
type Notebook struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	ParentID      string `json:"parent_id"`
	CreatedTime   int64  `json:"ctime"`
	ModifiedTime  int64  `json:"mtime"`
}

// Note represents a single note from Synology Note Station.
type Note struct {
	ID            string   `json:"id"`
	NotebookID    string   `json:"notebook_id"`
	Title         string   `json:"title"`
	ContentHTML   string   `json:"body"`
	Tags          []string `json:"tags"`
	CreatedTime   int64    `json:"ctime"`
	ModifiedTime  int64    `json:"mtime"`
}

// NoteListResponse is the paginated response from listing notes.
type NoteListResponse struct {
	Total int    `json:"total"`
	Notes []Note `json:"items"`
}

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
