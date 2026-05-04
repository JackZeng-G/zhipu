package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// embedRequest is the body sent to POST /api/embeddings.
type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// embedResponse is the response from /api/embeddings.
type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed sends an embedding request and returns the embedding vector.
func (c *Client) Embed(ctx context.Context, model, prompt string) ([]float32, error) {
	body := embedRequest{
		Model:  model,
		Prompt: prompt,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ollama embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed: decode response: %w", err)
	}
	return result.Embedding, nil
}
