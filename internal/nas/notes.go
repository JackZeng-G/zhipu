package nas

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ListNotebooks retrieves all notebooks from Synology Note Station.
func (c *NoteStationClient) ListNotebooks() ([]Notebook, error) {
	if !c.auth.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in")
	}

	params := url.Values{}
	params.Set("api", "SYNO.NoteStation.Notebook")
	params.Set("version", "2")
	params.Set("method", "list")

	resp, err := c.auth.get(params)
	if err != nil {
		return nil, fmt.Errorf("list notebooks request: %w", err)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read notebooks response: %w", err)
	}

	var result synoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode notebooks response: %w, body: %s", err, string(body))
	}

	if !result.Success {
		return nil, fmt.Errorf("list notebooks failed: %s", synoErrorMessage(result.Error))
	}

	notebooks, err := parseNotebooks(result.Data)
	if err != nil {
		return nil, fmt.Errorf("parse notebooks: %w", err)
	}
	return notebooks, nil
}

// ListNotes retrieves a paginated list of notes from Synology Note Station.
func (c *NoteStationClient) ListNotes(offset, limit int) (*NoteListResponse, error) {
	if !c.auth.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in")
	}

	params := url.Values{}
	params.Set("api", "SYNO.NoteStation.Note")
	params.Set("version", "3")
	params.Set("method", "list")
	params.Set("offset", strconv.Itoa(offset))
	params.Set("limit", strconv.Itoa(limit))
	// Request all data fields
	params.Set("additional", `["count"]`)

	resp, err := c.auth.get(params)
	if err != nil {
		return nil, fmt.Errorf("list notes request: %w", err)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read notes response: %w", err)
	}

	var result synoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode notes response: %w, body: %s", err, string(body))
	}

	if !result.Success {
		return nil, fmt.Errorf("list notes failed: %s, body: %s", synoErrorMessage(result.Error), string(body))
	}

	parsed, err := parseNotes(result.Data)
	if err != nil {
		return nil, fmt.Errorf("parse notes: %w", err)
	}
	return parsed, nil
}

// GetNote retrieves a single note by its object ID from Synology Note Station.
func (c *NoteStationClient) GetNote(noteID string) (*Note, error) {
	return c.getNote(noteID, false)
}

// GetNoteWithAttachments retrieves a single note with full attachment metadata.
func (c *NoteStationClient) GetNoteWithAttachments(noteID string) (*Note, error) {
	return c.getNote(noteID, true)
}

// getNote retrieves a single note, optionally including attachment metadata.
func (c *NoteStationClient) getNote(noteID string, withAttachments bool) (*Note, error) {
	if !c.auth.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in")
	}

	params := url.Values{
		"api":       {"SYNO.NoteStation.Note"},
		"version":   {"4"},
		"method":    {"get"},
		"object_id": {noteID},
	}
	if withAttachments {
		params.Set("additional", `["attachment"]`)
	}

	resp, err := c.auth.get(params)
	if err != nil {
		return nil, fmt.Errorf("get note request: %w", err)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read note response: %w", err)
	}

	var result synoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode note response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("get note failed: %s", synoErrorMessage(result.Error))
	}

	var raw rawNote
	if err := json.Unmarshal(result.Data, &raw); err != nil {
		return nil, fmt.Errorf("parse note data: %w", err)
	}

	content := raw.Content
	if content == "" {
		content = raw.Body
	}

	note := &Note{
		ID:            raw.ObjectID,
		NotebookID:    raw.ParentID,
		Title:         raw.Title,
		ContentHTML:   content,
		CreatedTime:   raw.CTime,
		ModifiedTime:  raw.MTime,
		Ver:           raw.Ver,
		LinkID:        raw.LinkID,
	}

	// Parse attachment data when requested (returned as map when additional=["attachment"])
	if withAttachments {
		var fullData struct {
			Ver         string                            `json:"ver"`
			Attachments map[string]map[string]interface{} `json:"attachment"`
		}
		if err := json.Unmarshal(result.Data, &fullData); err == nil {
			note.Ver = fullData.Ver
			if len(fullData.Attachments) > 0 {
				note.Attachments = make(map[string]Attachment, len(fullData.Attachments))
				for key, att := range fullData.Attachments {
					a := Attachment{Key: key}
					if v, ok := att["name"].(string); ok {
						a.Name = v
					}
					if v, ok := att["ref"].(string); ok {
						a.Ref = v
					}
					if v, ok := att["type"].(string); ok {
						a.Type = v
					}
					if v, ok := att["ext"].(string); ok {
						a.Ext = v
					}
					if v, ok := att["size"].(float64); ok {
						a.Size = int64(v)
					}
					note.Attachments[key] = a
				}
			}
		}
	}

	return note, nil
}

// GetNoteInfo retrieves the NoteStation info (hash, uid, username).
func (c *NoteStationClient) GetInfo() (hash string, uid int, username string, err error) {
	if !c.auth.IsLoggedIn() {
		return "", 0, "", fmt.Errorf("not logged in")
	}

	params := url.Values{
		"api":     {"SYNO.NoteStation.Info"},
		"version": {"1"},
		"method":  {"get"},
	}

	resp, err := c.auth.get(params)
	if err != nil {
		return "", 0, "", fmt.Errorf("get info request: %w", err)
	}

	body, err := readBody(resp)
	if err != nil {
		return "", 0, "", fmt.Errorf("read info response: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Hash     string `json:"hash"`
			UID      int    `json:"uid"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, "", fmt.Errorf("decode info response: %w", err)
	}
	if !result.Success {
		return "", 0, "", fmt.Errorf("get info failed")
	}

	return result.Data.Hash, result.Data.UID, result.Data.Username, nil
}

