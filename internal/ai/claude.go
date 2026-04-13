package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// hideWindowCmd creates an exec.Cmd with hidden window on Windows.
func hideWindowCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd
}

// ClaudeProvider implements Provider using the local Claude Code CLI or Claude API.
type ClaudeProvider struct {
	config Config
}

// NewClaudeProvider creates a new Claude provider.
// If APIKey is set, it uses the Anthropic API directly.
// If APIKey is empty, it uses the local `claude` CLI command.
func NewClaudeProvider(config Config) (Provider, error) {
	if config.Model == "" {
		config.Model = "claude-sonnet-4-20250514"
	}
	return &ClaudeProvider{config: config}, nil
}

// Generate sends a prompt and returns the response.
// Uses local `claude` CLI with -p flag for one-shot generation.
func (p *ClaudeProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if p.config.APIKey != "" {
		return p.generateViaAPI(ctx, prompt)
	}
	return p.generateViaCLI(ctx, prompt)
}

// generateViaCLI calls the local `claude` CLI command.
// The prompt is piped via stdin to avoid Windows command-line length limits (~8191 chars).
func (p *ClaudeProvider) generateViaCLI(ctx context.Context, prompt string) (string, error) {
	args := []string{"-p", "--model", p.config.Model, "--output-format", "text"}

	cmd := hideWindowCmd(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude cli error: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// generateViaAPI calls the Anthropic API directly using the API key.
func (p *ClaudeProvider) generateViaAPI(ctx context.Context, prompt string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		Messages  []message `json:"messages"`
	}

	body := request{
		Model:     p.config.Model,
		MaxTokens: 4096,
		Messages:  []message{{Role: "user", Content: prompt}},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	baseURL := "https://api.anthropic.com"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	cmd := hideWindowCmd(ctx, "curl", "-s",
		"-X", "POST",
		baseURL+"/v1/messages",
		"-H", "Content-Type: application/json",
		"-H", "x-api-key: "+p.config.APIKey,
		"-H", "anthropic-version: 2023-06-01",
		"-d", string(data),
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("api call error: %w", err)
	}

	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if resp.Error.Message != "" {
		return "", fmt.Errorf("api error: %s", resp.Error.Message)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return resp.Content[0].Text, nil
}

// GenerateStream streams a response using the CLI with --stream flag.
func (p *ClaudeProvider) GenerateStream(ctx context.Context, prompt string, onChunk func(chunk string)) error {
	if p.config.APIKey != "" {
		// For API mode, fall back to non-streaming then chunk it
		resp, err := p.generateViaAPI(ctx, prompt)
		if err != nil {
			return err
		}
		onChunk(resp)
		return nil
	}

	args := []string{"-p", "--model", p.config.Model, "--output-format", "stream-json"}
	cmd := hideWindowCmd(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var evt struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if evt.Type == "content_block_delta" || evt.Type == "assistant" {
			if evt.Text != "" {
				onChunk(evt.Text)
			}
		}
	}

	return cmd.Wait()
}

// Embed is not supported by Claude (use Ollama for embeddings).
func (p *ClaudeProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, ErrEmbeddingNotSupported
}

// Name returns "claude".
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// Model returns the configured model name.
func (p *ClaudeProvider) Model() string {
	return p.config.Model
}

// Close is a no-op for Claude (stateless CLI/API provider).
func (p *ClaudeProvider) Close() {}
