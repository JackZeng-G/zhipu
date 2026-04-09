package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// generateRequest is the body sent to POST /api/generate.
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// generateResponse is the non-streaming response from /api/generate.
type generateResponse struct {
	Response string `json:"response"`
}

// Generate sends a non-streaming generate request to Ollama and returns the
// full response text.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	body := generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama generate: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("ollama generate: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama generate: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama generate: decode response: %w", err)
	}
	return result.Response, nil
}

// GenerateStream sends a streaming generate request to Ollama. It returns a
// channel that emits text chunks as they arrive. The channel is closed when
// the response ends or an error occurs.
func (c *Client) GenerateStream(ctx context.Context, prompt string) (<-chan string, error) {
	body := generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: true,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama generate stream: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ollama generate stream: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama generate stream: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama generate stream: status %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		readGenerateStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// readGenerateStream reads NDJSON lines from r, extracts the response field,
// and sends each chunk to ch.
func readGenerateStream(ctx context.Context, r io.Reader, ch chan<- string) {
	type chunk struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var c chunk
		if err := json.Unmarshal(line, &c); err != nil {
			continue
		}
		if c.Response != "" {
			select {
			case ch <- c.Response:
			case <-ctx.Done():
				return
			}
		}
		if c.Done {
			return
		}
	}
}
