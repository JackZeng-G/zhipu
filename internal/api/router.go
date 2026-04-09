package api

import (
	"net/http"
	stdsync "sync"
	"time"

	"personal-kb/internal/nas"
	"personal-kb/internal/ollama"
	"personal-kb/internal/store"
	"personal-kb/internal/sync"

	"github.com/gin-gonic/gin"
)

// Handlers holds all dependencies for API handlers.
type Handlers struct {
	notesStore  *store.NotesStore
	settingsStore *store.SettingsStore
	convStore   *store.ConversationsStore
	nasClient   *nas.NoteStationClient
	authClient  *nas.AuthClient
	ollamaClient *ollama.Client
	syncService *sync.SyncService

	// summaryCache stores AI-generated summaries keyed by note ID.
	summaryCache map[string]string
	summaryMu    stdsync.RWMutex
}

// NewHandlers creates a new Handlers instance with all dependencies.
func NewHandlers(
	notesStore *store.NotesStore,
	settingsStore *store.SettingsStore,
	convStore *store.ConversationsStore,
	nasClient *nas.NoteStationClient,
	authClient *nas.AuthClient,
	ollamaClient *ollama.Client,
	syncService *sync.SyncService,
) *Handlers {
	return &Handlers{
		notesStore:    notesStore,
		settingsStore: settingsStore,
		convStore:     convStore,
		nasClient:     nasClient,
		authClient:    authClient,
		ollamaClient:  ollamaClient,
		syncService:   syncService,
		summaryCache:  make(map[string]string),
	}
}

// SetupRouter creates and configures the Gin engine with all API routes.
func SetupRouter(h *Handlers) *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		// NAS connection endpoints
		api.POST("/nas/connect", h.NASConnect)
		api.POST("/nas/disconnect", h.NASDisconnect)
		api.GET("/nas/status", h.NASStatus)
		api.POST("/nas/sync", h.NASSync)

		// Notes endpoints
		api.GET("/notebooks", h.ListNotebooks)
		api.GET("/notes", h.ListNotes)
		api.GET("/notes/:id", h.GetNote)

		// AI endpoints
		ai := api.Group("/ai")
		{
			ai.POST("/summarize/:id", h.AISummarize)
			ai.POST("/search", h.AISearch)
			ai.POST("/edit", h.AIEdit)
			ai.GET("/conversations", h.ListConversations)
			ai.POST("/conversations", h.CreateConversation)
			ai.POST("/conversations/:id/messages", h.SendMessage)
		}

		// Settings endpoints
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.UpdateSettings)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Unix()})
	})

	return r
}
