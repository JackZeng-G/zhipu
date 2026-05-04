package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// settingsResponse is the response body for GET /api/settings.
type settingsResponse struct {
	OllamaURL   string `json:"ollama_url"`
	OllamaModel string `json:"ollama_model"`
	NASHost     string `json:"nas_host,omitempty"`
	NASUsername string `json:"nas_username,omitempty"`
	NASPort     string `json:"nas_port,omitempty"`
}

// updateSettingsRequest is the request body for PUT /api/settings.
type updateSettingsRequest struct {
	OllamaURL   string `json:"ollama_url"`
	OllamaModel string `json:"ollama_model"`
}

// GetSettings returns current application settings.
// Note: NAS password is never returned.
func (h *Handlers) GetSettings(c *gin.Context) {
	ollamaURL, _ := h.settingsStore.GetSetting("ollama_url")
	ollamaModel, _ := h.settingsStore.GetSetting("ollama_model")
	nasHost, _ := h.settingsStore.GetSetting("nas_host")
	nasUsername, _ := h.settingsStore.GetSetting("nas_username")
	nasPort, _ := h.settingsStore.GetSetting("nas_port")

	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if ollamaModel == "" {
		ollamaModel = "qwen2"
	}

	c.JSON(http.StatusOK, settingsResponse{
		OllamaURL:   ollamaURL,
		OllamaModel: ollamaModel,
		NASHost:     nasHost,
		NASUsername: nasUsername,
		NASPort:     nasPort,
	})
}

// UpdateSettings updates application settings.
func (h *Handlers) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.OllamaURL != "" {
		if err := h.settingsStore.SetSetting("ollama_url", req.OllamaURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save ollama_url: " + err.Error()})
			return
		}
	}

	if req.OllamaModel != "" {
		if err := h.settingsStore.SetSetting("ollama_model", req.OllamaModel); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save ollama_model: " + err.Error()})
			return
		}
	}

	if err := h.refreshActiveProvider(); err != nil {
		log.Printf("[settings] failed to refresh provider: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
