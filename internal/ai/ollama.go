package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"personal-kb/internal/ollama"
)

// OllamaProvider implements Provider using the local Ollama service.
type OllamaProvider struct {
	config     Config
	client     *ollama.Client
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(config Config) (Provider, error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	model := config.Model
	if model == "" {
		model = "qwen2"
	}
	return &OllamaProvider{
		config:     config,
		client:     ollama.NewClient(baseURL, model),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Generate sends a prompt to Ollama and returns the response.
func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return p.client.Generate(ctx, prompt)
}

// GenerateStream streams a response from Ollama via callback.
func (p *OllamaProvider) GenerateStream(ctx context.Context, prompt string, onChunk func(chunk string)) error {
	ch, err := p.client.GenerateStream(ctx, prompt)
	if err != nil {
		return err
	}
	for chunk := range ch {
		onChunk(chunk)
	}
	return nil
}

// Embed generates an embedding vector for the given text using Ollama's embeddings API.
func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Use the model name with -embed suffix if not already an embedding model
	model := p.config.Model
	if !strings.Contains(model, "embed") {
		// Try to use a compatible embedding model
		model = "nomic-embed-text"
	}

	url := p.getBaseURL() + "/api/embeddings"
	reqBody := map[string]string{
		"model": model,
		"prompt": text,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("ollama embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: status %d", resp.StatusCode)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed: decode response: %w", err)
	}
	return result.Embedding, nil
}

// Close closes the underlying Ollama HTTP client connections.
func (p *OllamaProvider) Close() {
	if p.client != nil {
		p.client.Close()
	}
	if transport, ok := p.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// Name returns "ollama".
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Model returns the configured model name.
func (p *OllamaProvider) Model() string {
	return p.config.Model
}

// getBaseURL extracts base URL from config or client.
func (p *OllamaProvider) getBaseURL() string {
	if p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "http://localhost:11434"
}
