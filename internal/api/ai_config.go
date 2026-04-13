package api

import (
	"context"
	"net/http"

	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

// ListAIConfigs returns all saved AI configurations.
func (h *Handlers) ListAIConfigs(c *gin.Context) {
	ctx := context.Background()
	configs, err := h.aiConfigStore.ListAIConfigs(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if configs == nil {
		configs = []store.AIConfig{}
	}
	c.JSON(http.StatusOK, gin.H{"configs": configs})
}

// CreateAIConfig creates a new AI provider configuration.
func (h *Handlers) CreateAIConfig(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required"`
		Name     string `json:"name" binding:"required"`
		BaseURL  string `json:"base_url"`
		Model    string `json:"model" binding:"required"`
		APIKey   string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	id, err := h.aiConfigStore.CreateAIConfig(ctx, req.Provider, req.Name, req.BaseURL, req.Model, req.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateAIConfig updates an existing AI configuration.
func (h *Handlers) UpdateAIConfig(c *gin.Context) {
	id, err := paramInt(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Provider string `json:"provider" binding:"required"`
		Name     string `json:"name" binding:"required"`
		BaseURL  string `json:"base_url"`
		Model    string `json:"model" binding:"required"`
		APIKey   string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := h.aiConfigStore.UpdateAIConfig(ctx, id, req.Provider, req.Name, req.BaseURL, req.Model, req.APIKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Refresh active provider if this was the active config
	config, _ := h.aiConfigStore.GetActiveAIConfig(ctx)
	if config != nil && config.ID == id {
		_ = h.refreshActiveProvider()
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ActivateAIConfig sets a configuration as the active one.
func (h *Handlers) ActivateAIConfig(c *gin.Context) {
	id, err := paramInt(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := h.aiConfigStore.SetActiveAIConfig(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Refresh the active provider
	if err := h.refreshActiveProvider(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteAIConfig removes a configuration.
func (h *Handlers) DeleteAIConfig(c *gin.Context) {
	id, err := paramInt(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := h.aiConfigStore.DeleteAIConfig(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestAIConfig tests the connection to an AI provider.
func (h *Handlers) TestAIConfig(c *gin.Context) {
	id, err := paramInt(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	config, err := h.aiConfigStore.GetAIConfig(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	// Get decrypted API key
	apiKey, _ := h.aiConfigStore.GetDecryptedAPIKey(ctx, id)

	baseURL := ""
	if config.BaseURL != nil {
		baseURL = *config.BaseURL
	}

	providerCfg := aiConfigFromStore(config, baseURL, apiKey)
	provider, err := newProvider(providerCfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer provider.Close()

	// Simple test: generate a short response
	resp, err := provider.Generate(ctx, "Say 'OK' in one word.")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "response": resp})
}
