// Package ai provides AI provider abstractions for different LLM backends.
package ai

import (
	"context"
	"errors"
)

var (
	// ErrUnsupportedProvider is returned when the provider type is not recognized.
	ErrUnsupportedProvider = errors.New("unsupported AI provider")

	// ErrEmbeddingNotSupported is returned when the provider doesn't support embeddings.
	ErrEmbeddingNotSupported = errors.New("embeddings not supported by this provider")
)

// Provider defines the interface for AI model providers (Claude, Ollama, etc.).
type Provider interface {
	// Generate sends a prompt and returns the generated text.
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateStream sends a prompt and streams the response via callback.
	// The callback is called for each chunk of text received.
	GenerateStream(ctx context.Context, prompt string, onChunk func(chunk string)) error

	// Embed generates an embedding vector for the given text.
	// Returns nil if the provider doesn't support embeddings.
	Embed(ctx context.Context, text string) ([]float32, error)

	// Name returns the provider name (e.g., "claude", "ollama").
	Name() string

	// Model returns the current model being used.
	Model() string

	// Close releases any resources held by the provider (HTTP connections, etc.).
	Close()
}

// Config holds the configuration for an AI provider.
type Config struct {
	Provider string // "claude" or "ollama"
	BaseURL  string // API base URL (optional for Claude, required for Ollama)
	Model    string // Model name (e.g., "claude-sonnet-4", "qwen2.5")
	APIKey   string // API key (optional for local Ollama, required for Claude)
}

// NewProvider creates a provider instance based on the configuration.
func NewProvider(config Config) (Provider, error) {
	switch config.Provider {
	case "claude":
		return NewClaudeProvider(config)
	case "ollama":
		return NewOllamaProvider(config)
	default:
		return nil, ErrUnsupportedProvider
	}
}
