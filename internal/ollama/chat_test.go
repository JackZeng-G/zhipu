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

func TestChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected path /api/chat, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "llama3" {
			t.Errorf("expected model llama3, got %s", req.Model)
		}
		if req.Stream {
			t.Errorf("expected stream false")
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "Hello" {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}

		resp := chatResponse{
			Message: Message{Role: "assistant", Content: "Hi there!"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	messages := []Message{{Role: "user", Content: "Hello"}}
	result, err := c.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hi there!" {
		t.Errorf("expected 'Hi there!', got %q", result)
	}
}

func TestChat_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestChat_CancelledContext(t *testing.T) {
	// Use a port that won't respond, simulating a slow server
	c := NewClient("http://192.0.2.1:12345", "llama3")
	c.httpClient.Timeout = 0 // no client timeout; rely on context only

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.Chat(ctx, []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestChatStream(t *testing.T) {
	chunks := []string{"Hi", " ", "there", "!"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected path /api/chat, got %s", r.URL.Path)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if !req.Stream {
			t.Errorf("expected stream true")
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		for i, chunk := range chunks {
			entry := map[string]any{
				"model": req.Model,
				"message": map[string]string{
					"role":    "assistant",
					"content": chunk,
				},
				"done": i == len(chunks)-1,
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
	messages := []Message{{Role: "user", Content: "Hello"}}
	ch, err := c.ChatStream(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var collected strings.Builder
	for s := range ch {
		collected.WriteString(s)
	}
	result := collected.String()
	expected := "Hi there!"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestChatStream_CancelledContext(t *testing.T) {
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
				"message": map[string]string{
					"role":    "assistant",
					"content": "x",
				},
				"done": false,
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
	ch, err := c.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("expected at least one chunk before cancellation")
	}
}

func TestChat_MultipleMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Messages) != 3 {
			t.Errorf("expected 3 messages, got %d", len(req.Messages))
		}

		resp := chatResponse{
			Message: Message{Role: "assistant", Content: "I understand."},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL, "llama3")
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	}
	result, err := c.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "I understand." {
		t.Errorf("expected 'I understand.', got %q", result)
	}
}
