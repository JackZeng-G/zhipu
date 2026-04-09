package nas

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupTestServer creates a test server and a logged-in NoteStationClient.
func setupTestServer(handler http.HandlerFunc) (*httptest.Server, *NoteStationClient) {
	server := httptest.NewServer(handler)
	auth := NewAuthClient(server.URL, true)
	auth.sessionID = "test-session-id"
	client := NewNoteStationClient(auth)
	return server, client
}

func TestListNotebooks(t *testing.T) {
	notebooks := []Notebook{
		{ID: "nb1", Title: "Personal", ParentID: "", CreatedTime: 1700000000, ModifiedTime: 1700001000},
		{ID: "nb2", Title: "Work", ParentID: "", CreatedTime: 1700002000, ModifiedTime: 1700003000},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/webapi/entry.cgi" {
			t.Errorf("path = %s, want /webapi/entry.cgi", r.URL.Path)
		}

		q := r.URL.Query()
		if q.Get("api") != "SYNO.NoteStation.Notebook" {
			t.Errorf("api = %s, want SYNO.NoteStation.Notebook", q.Get("api"))
		}
		if q.Get("version") != "2" {
			t.Errorf("version = %s, want 2", q.Get("version"))
		}
		if q.Get("method") != "list" {
			t.Errorf("method = %s, want list", q.Get("method"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    notebooks,
		})
	})
	defer server.Close()

	result, err := client.ListNotebooks()
	if err != nil {
		t.Fatalf("ListNotebooks error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d notebooks, want 2", len(result))
	}
	if result[0].ID != "nb1" || result[0].Title != "Personal" {
		t.Errorf("notebook[0] = %+v, want ID=nb1 Title=Personal", result[0])
	}
	if result[1].ID != "nb2" || result[1].Title != "Work" {
		t.Errorf("notebook[1] = %+v, want ID=nb2 Title=Work", result[1])
	}
}

func TestListNotebooks_NotLoggedIn(t *testing.T) {
	auth := NewAuthClient("http://unused", true)
	client := NewNoteStationClient(auth)
	_, err := client.ListNotebooks()
	if err == nil {
		t.Error("expected error when not logged in")
	}
}

func TestListNotes(t *testing.T) {
	notes := []Note{
		{ID: "n1", NotebookID: "nb1", Title: "First Note", CreatedTime: 1700000000, ModifiedTime: 1700001000},
		{ID: "n2", NotebookID: "nb1", Title: "Second Note", CreatedTime: 1700002000, ModifiedTime: 1700003000},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("api") != "SYNO.NoteStation.Note" {
			t.Errorf("api = %s, want SYNO.NoteStation.Note", q.Get("api"))
		}
		if q.Get("version") != "3" {
			t.Errorf("version = %s, want 3", q.Get("version"))
		}
		if q.Get("method") != "list" {
			t.Errorf("method = %s, want list", q.Get("method"))
		}
		if q.Get("offset") != "0" {
			t.Errorf("offset = %s, want 0", q.Get("offset"))
		}
		if q.Get("limit") != "100" {
			t.Errorf("limit = %s, want 100", q.Get("limit"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"total": 2,
				"items": notes,
			},
		})
	})
	defer server.Close()

	result, err := client.ListNotes(0, 100)
	if err != nil {
		t.Fatalf("ListNotes error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if len(result.Notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(result.Notes))
	}
	if result.Notes[0].ID != "n1" || result.Notes[0].Title != "First Note" {
		t.Errorf("notes[0] = %+v, want ID=n1 Title=First Note", result.Notes[0])
	}
}

func TestListNotes_NotLoggedIn(t *testing.T) {
	auth := NewAuthClient("http://unused", true)
	client := NewNoteStationClient(auth)
	_, err := client.ListNotes(0, 10)
	if err == nil {
		t.Error("expected error when not logged in")
	}
}

func TestListNotes_APIError(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   map[string]int{"code": 120},
		})
	})
	defer server.Close()

	_, err := client.ListNotes(0, 100)
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

func TestGetNote(t *testing.T) {
	note := Note{
		ID:            "n42",
		NotebookID:    "nb1",
		Title:         "Test Note",
		ContentHTML:   "<h1>Hello</h1><p>World</p>",
		Tags:          []string{"test", "important"},
		CreatedTime:   1700000000,
		ModifiedTime:  1700001000,
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("api") != "SYNO.NoteStation.Note" {
			t.Errorf("api = %s, want SYNO.NoteStation.Note", q.Get("api"))
		}
		if q.Get("method") != "get" {
			t.Errorf("method = %s, want get", q.Get("method"))
		}
		if q.Get("object_id") != "n42" {
			t.Errorf("object_id = %s, want n42", q.Get("object_id"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    note,
		})
	})
	defer server.Close()

	result, err := client.GetNote("n42")
	if err != nil {
		t.Fatalf("GetNote error: %v", err)
	}

	if result.ID != "n42" {
		t.Errorf("ID = %s, want n42", result.ID)
	}
	if result.Title != "Test Note" {
		t.Errorf("Title = %s, want Test Note", result.Title)
	}
	if result.ContentHTML != "<h1>Hello</h1><p>World</p>" {
		t.Errorf("ContentHTML = %s", result.ContentHTML)
	}
	if len(result.Tags) != 2 || result.Tags[0] != "test" || result.Tags[1] != "important" {
		t.Errorf("Tags = %v, want [test important]", result.Tags)
	}
}

func TestGetNote_NotLoggedIn(t *testing.T) {
	auth := NewAuthClient("http://unused", true)
	client := NewNoteStationClient(auth)
	_, err := client.GetNote("n1")
	if err == nil {
		t.Error("expected error when not logged in")
	}
}

func TestGetNote_APIError(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   map[string]int{"code": 403},
		})
	})
	defer server.Close()

	_, err := client.GetNote("nonexistent")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}
