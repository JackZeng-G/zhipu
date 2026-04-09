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

// Message represents a single chat message with a role and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the body sent to POST /api/chat.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// chatResponse is the non-streaming response from /api/chat.
type chatResponse struct {
	Message Message `json:"message"`
}

// Chat sends a non-streaming chat request to Ollama and returns the assistant
// reply text.
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	body := chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama chat: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("ollama chat: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama chat: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama chat: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama chat: decode response: %w", err)
	}
	return result.Message.Content, nil
}

// ChatStream sends a streaming chat request to Ollama. It returns a channel
// that emits text chunks as they arrive. The channel is closed when the
// response ends or an error occurs.
func (c *Client) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	body := chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama chat stream: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ollama chat stream: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama chat stream: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama chat stream: status %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		readChatStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// readChatStream reads NDJSON lines from r, extracts the message.content field,
// and sends each chunk to ch.
func readChatStream(ctx context.Context, r io.Reader, ch chan<- string) {
	type chunk struct {
		Message Message `json:"message"`
		Done    bool    `json:"done"`
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
		if c.Message.Content != "" {
			select {
			case ch <- c.Message.Content:
			case <-ctx.Done():
				return
			}
		}
		if c.Done {
			return
		}
	}
}
