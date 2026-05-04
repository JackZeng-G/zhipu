package ai

import (
	"context"
	"strings"

	"personal-kb/internal/ollama"
)

// OllamaProvider implements Provider using the local Ollama service.
type OllamaProvider struct {
	config Config
	client *ollama.Client
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
		config: config,
		client: ollama.NewClient(baseURL, model),
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
	model := p.config.Model
	if !strings.Contains(model, "embed") {
		model = "nomic-embed-text"
	}
	return p.client.Embed(ctx, model, text)
}

// Close closes the underlying Ollama HTTP client connections.
func (p *OllamaProvider) Close() {
	if p.client != nil {
		p.client.Close()
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
