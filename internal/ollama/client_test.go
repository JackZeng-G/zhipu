package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:11434", "llama3")
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("expected baseURL http://localhost:11434, got %s", c.baseURL)
	}
	if c.model != "llama3" {
		t.Errorf("expected model llama3, got %s", c.model)
	}
	if c.httpClient == nil {
		t.Error("expected httpClient to be set")
	}
}

func TestSetModel(t *testing.T) {
	c := NewClient("http://localhost:11434", "llama3")
	c.SetModel("mistral")
	if c.model != "mistral" {
		t.Errorf("expected model mistral, got %s", c.model)
	}
}

func TestSetBaseURL(t *testing.T) {
	c := NewClient("http://localhost:11434", "llama3")
	c.SetBaseURL("http://192.168.1.100:11434")
	if c.baseURL != "http://192.168.1.100:11434" {
		t.Errorf("expected baseURL http://192.168.1.100:11434, got %s", c.baseURL)
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/tags" {
			t.Errorf("expected path /api/tags, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"models": []any{},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	if err := c.Ping(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPing_NotOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	if err := c.Ping(); err == nil {
		t.Error("expected error for non-OK status")
	}
}

func TestPing_ConnectionRefused(t *testing.T) {
	c := NewClient("http://localhost:0", "llama3")
	if err := c.Ping(); err == nil {
		t.Error("expected error for connection refused")
	}
}
