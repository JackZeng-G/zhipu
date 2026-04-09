package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected path /api/generate, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "llama3" {
			t.Errorf("expected model llama3, got %s", req.Model)
		}
		if req.Stream {
			t.Errorf("expected stream false")
		}

		resp := generateResponse{Response: "Hello, world!"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	result, err := c.Generate(context.Background(), "Say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", result)
	}
}

func TestGenerate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	_, err := c.Generate(context.Background(), "Say hello")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestGenerate_CancelledContext(t *testing.T) {
	c := NewClient("http://192.0.2.1:12345", "llama3")
	c.httpClient.Timeout = 0 // no client timeout; rely on context only

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.Generate(ctx, "Say hello")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestGenerateStream(t *testing.T) {
	chunks := []string{"Hello", ", ", "world", "!"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected path /api/generate, got %s", r.URL.Path)
		}

		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if !req.Stream {
			t.Errorf("expected stream true")
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		for i, chunk := range chunks {
			entry := map[string]any{
				"model":    req.Model,
				"response": chunk,
				"done":     i == len(chunks)-1,
			}
			data, _ := json.Marshal(entry)
			w.Write(data)
			w.Write([]byte("\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	ch, err := c.GenerateStream(context.Background(), "Say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var collected strings.Builder
	for s := range ch {
		collected.WriteString(s)
	}
	result := collected.String()
	expected := "Hello, world!"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGenerateStream_CancelledContext(t *testing.T) {
	// Use a done channel so the handler can exit promptly when the test ends
	handlerDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		w.Header().Set("Content-Type", "application/x-ndjson")
		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			entry := map[string]any{
				"response": "x",
				"done":     false,
			}
			data, _ := json.Marshal(entry)
			w.Write(data)
			w.Write([]byte("\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()
	defer func() { <-handlerDone }()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c := NewClient(server.URL, "llama3")
	ch, err := c.GenerateStream(ctx, "Say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Drain the channel — it should close after context cancellation
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("expected at least one chunk before cancellation")
	}
}
