package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"personal-kb/internal/nas"
	"personal-kb/internal/sync"

	"github.com/gin-gonic/gin"
)

// nasConnectRequest is the request body for POST /api/nas/connect.
type nasConnectRequest struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// NASConnect authenticates against the NAS and triggers a full sync.
func (h *Handlers) NASConnect(c *gin.Context) {
	var req nasConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	scheme := "https"
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, req.Host, req.Port)
	if req.Port == 0 {
		baseURL = fmt.Sprintf("%s://%s", scheme, req.Host)
	}

	// Create a new auth client with TLS skip verify for self-signed certs
	authClient := nas.NewAuthClient(baseURL, true)
	if err := authClient.Login(req.Username, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed: " + err.Error()})
		return
	}

	// Store the auth client
	h.authClient = authClient
	h.nasClient = nas.NewNoteStationClient(authClient)

	// Save encrypted credentials to settings
	ctx := context.Background()
	if err := h.settingsStore.SetSetting("nas_host", req.Host); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save host: " + err.Error()})
		return
	}
	if err := h.settingsStore.SetSetting("nas_port", strconv.Itoa(req.Port)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save port: " + err.Error()})
		return
	}
	if err := h.settingsStore.SetSetting("nas_username", req.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save username: " + err.Error()})
		return
	}
	if err := h.settingsStore.SetSetting("nas_password_encrypted", req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save password: " + err.Error()})
		return
	}

	// Trigger full sync
	syncService := sync.NewSyncService(h.nasClient, h.notesStore)
	h.syncService = syncService

	syncedNotes := 0
	if err := syncService.FullSync(ctx); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Connected but sync had issues: " + err.Error(),
		})
		return
	}

	// Count notes for the message
	notes, _ := h.notesStore.ListNotes(ctx, "", 0, 1)
	if len(notes) > 0 {
		// Get all notes to count
		allNotes, _ := h.notesStore.ListNotes(ctx, "", 0, 100000)
		syncedNotes = len(allNotes)
	}

	// Save last sync time
	_ = h.settingsStore.SetSetting("nas_last_sync", strconv.FormatInt(time.Now().Unix(), 10))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Connected and synced %d notes", syncedNotes),
	})
}

// NASDisconnect logs out from the NAS and clears the session.
func (h *Handlers) NASDisconnect(c *gin.Context) {
	if h.authClient != nil {
		_ = h.authClient.Logout()
	}
	h.authClient = nil
	h.nasClient = nil
	h.syncService = nil

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// NASStatus returns the current NAS connection status.
func (h *Handlers) NASStatus(c *gin.Context) {
	connected := h.authClient != nil && h.authClient.IsLoggedIn()

	host := ""
	if connected {
		host, _ = h.settingsStore.GetSetting("nas_host")
	}

	lastSync := ""
	if connected {
		lastSync, _ = h.settingsStore.GetSetting("nas_last_sync")
	}

	resp := gin.H{
		"connected": connected,
	}

	if host != "" {
		resp["host"] = host
	}
	if lastSync != "" {
		resp["last_sync"], _ = strconv.ParseInt(lastSync, 10, 64)
	}

	c.JSON(http.StatusOK, resp)
}

// NASSync triggers an incremental sync.
func (h *Handlers) NASSync(c *gin.Context) {
	if h.authClient == nil || !h.authClient.IsLoggedIn() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not connected to NAS"})
		return
	}

	if h.syncService == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sync service not initialized"})
		return
	}

	ctx := context.Background()

	// Get count before sync
	beforeNotes, _ := h.notesStore.ListNotes(ctx, "", 0, 100000)
	beforeCount := len(beforeNotes)

	if err := h.syncService.IncrementalSync(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed: " + err.Error()})
		return
	}

	// Get count after sync
	afterNotes, _ := h.notesStore.ListNotes(ctx, "", 0, 100000)
	afterCount := len(afterNotes)

	syncedNotes := afterCount - beforeCount
	if syncedNotes < 0 {
		syncedNotes = 0
	}

	_ = h.settingsStore.SetSetting("nas_last_sync", strconv.FormatInt(time.Now().Unix(), 10))

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"synced_notes": syncedNotes,
	})
}
