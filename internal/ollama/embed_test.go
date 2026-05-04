package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embeddings", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"embedding": [0.1, 0.2, 0.3]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	vec, err := client.Embed(context.Background(), "nomic-embed-text", "hello world")
	require.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vec)
}

func TestEmbed_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	_, err := client.Embed(context.Background(), "nomic-embed-text", "hello")
	assert.Error(t, err)
}
