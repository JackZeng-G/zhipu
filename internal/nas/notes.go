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
		return nil, fmt.Errorf("decode notebooks response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("list notebooks failed: %s", synoErrorMessage(result.Error))
	}

	var notebooks []Notebook
	if err := json.Unmarshal(result.Data, &notebooks); err != nil {
		return nil, fmt.Errorf("parse notebooks data: %w", err)
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
		return nil, fmt.Errorf("decode notes response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("list notes failed: %s", synoErrorMessage(result.Error))
	}

	var noteResp NoteListResponse
	if err := json.Unmarshal(result.Data, &noteResp); err != nil {
		return nil, fmt.Errorf("parse notes data: %w", err)
	}

	return &noteResp, nil
}

// GetNote retrieves a single note by its object ID from Synology Note Station.
func (c *NoteStationClient) GetNote(noteID string) (*Note, error) {
	if !c.auth.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in")
	}

	params := url.Values{}
	params.Set("api", "SYNO.NoteStation.Note")
	params.Set("version", "3")
	params.Set("method", "get")
	params.Set("object_id", noteID)

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

	var note Note
	if err := json.Unmarshal(result.Data, &note); err != nil {
		return nil, fmt.Errorf("parse note data: %w", err)
	}

	return &note, nil
}
