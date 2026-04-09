package ollama

import (
	"fmt"
	"net/http"
	"time"
)

// Client communicates with a local Ollama service.
type Client struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// NewClient creates a Client that talks to Ollama at baseURL using model.
func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		model: model,
	}
}

// SetModel changes the model used for subsequent requests.
func (c *Client) SetModel(model string) {
	c.model = model
}

// SetBaseURL changes the Ollama endpoint.
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// Ping checks whether the Ollama service is reachable by requesting GET /api/tags.
func (c *Client) Ping() error {
	url := c.baseURL + "/api/tags"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("ollama ping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama ping: unexpected status %d", resp.StatusCode)
	}
	return nil
}
