package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	stdsync "sync"
	"time"

	"personal-kb/internal/ai"
	"personal-kb/internal/nas"
	"personal-kb/internal/ollama"
	"personal-kb/internal/store"
	"personal-kb/internal/sync"

	"github.com/gin-gonic/gin"
)

// Handlers holds all dependencies for API handlers.
type Handlers struct {
	notesStore     *store.NotesStore
	settingsStore  *store.SettingsStore
	convStore      *store.ConversationsStore
	aiConfigStore  *store.AIConfigStore
	knowledgeStore *store.KnowledgeStore
	nasClient      *nas.NoteStationClient
	authClient     *nas.AuthClient
	ollamaClient   *ollama.Client
	syncService    *sync.SyncService

	// activeProvider is the currently active AI provider (may be nil).
	activeProvider ai.Provider
	providerMu     stdsync.RWMutex

	// summaryCache stores AI-generated summaries keyed by note ID.
	summaryCache map[string]string
	summaryMu    stdsync.RWMutex
}

// NewHandlers creates a new Handlers instance with all dependencies.
func NewHandlers(
	notesStore *store.NotesStore,
	settingsStore *store.SettingsStore,
	convStore *store.ConversationsStore,
	aiConfigStore *store.AIConfigStore,
	knowledgeStore *store.KnowledgeStore,
	nasClient *nas.NoteStationClient,
	authClient *nas.AuthClient,
	ollamaClient *ollama.Client,
	syncService *sync.SyncService,
) *Handlers {
	h := &Handlers{
		notesStore:     notesStore,
		settingsStore:  settingsStore,
		convStore:      convStore,
		aiConfigStore:  aiConfigStore,
		knowledgeStore: knowledgeStore,
		nasClient:      nasClient,
		authClient:     authClient,
		ollamaClient:   ollamaClient,
		syncService:    syncService,
		summaryCache:   make(map[string]string),
	}

	// Try to initialize active AI provider from saved config
	if err := h.refreshActiveProvider(); err != nil {
		log.Printf("[api] no active AI provider: %v", err)
	}

	return h
}

// refreshActiveProvider loads the active AI config from DB and creates a provider.
func (h *Handlers) refreshActiveProvider() error {
	ctx := context.Background()
	config, err := h.aiConfigStore.GetActiveAIConfig(ctx)
	if err != nil {
		return err
	}

	apiKey, _ := h.aiConfigStore.GetDecryptedAPIKey(ctx, config.ID)
	baseURL := ""
	if config.BaseURL != nil {
		baseURL = *config.BaseURL
	}

	providerCfg := aiConfigFromStore(config, baseURL, apiKey)
	provider, err := newProvider(providerCfg)
	if err != nil {
		return err
	}

	h.providerMu.Lock()
	old := h.activeProvider
	h.activeProvider = provider
	h.providerMu.Unlock()

	if old != nil {
		old.Close()
	}

	log.Printf("[api] active AI provider: %s/%s", provider.Name(), provider.Model())
	return nil
}

// getProvider returns the active AI provider.
func (h *Handlers) getProvider() ai.Provider {
	h.providerMu.RLock()
	defer h.providerMu.RUnlock()
	return h.activeProvider
}

// GetProvider returns the active AI provider (exported for external callers).
func (h *Handlers) GetProvider() ai.Provider {
	return h.getProvider()
}

// AutoIndexUnindexedNotes finds notes without summaries and indexes them.
func (h *Handlers) AutoIndexUnindexedNotes() {
	provider := h.getProvider()
	if provider == nil {
		return
	}
	ctx := context.Background()
	noteIDs, err := h.knowledgeStore.GetUnindexedNoteIDs(ctx)
	if err != nil {
		log.Printf("[auto-index] failed to query unindexed notes: %v", err)
		return
	}
	if len(noteIDs) == 0 {
		log.Printf("[auto-index] all notes already indexed")
		return
	}
	log.Printf("[auto-index] found %d unindexed notes, starting auto-index", len(noteIDs))
	h.IndexNotesAsync(noteIDs)
}

// newProvider is a wrapper around ai.NewProvider for use in handlers.
func newProvider(cfg ai.Config) (ai.Provider, error) {
	return ai.NewProvider(cfg)
}

// aiConfigFromStore converts a store AIConfig to an ai.Config.
func aiConfigFromStore(config *store.AIConfig, baseURL, apiKey string) ai.Config {
	return ai.Config{
		Provider: config.Provider,
		BaseURL:  baseURL,
		Model:    config.Model,
		APIKey:   apiKey,
	}
}

// paramInt parses an integer path parameter.
func paramInt(c *gin.Context, param string) (int, error) {
	s := c.Param(param)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// SetupRouter creates and configures the Gin engine with all API routes.
func SetupRouter(h *Handlers) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false
		if origin == "" {
			allowed = true
		} else {
			// Allow localhost and same-origin for personal use
			for _, prefix := range []string{"http://localhost", "http://127.0.0.1"} {
				if strings.HasPrefix(origin, prefix) {
					allowed = true
					break
				}
			}
		}
		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	api := r.Group("/api")

	// Health endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Unix()})
	})

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
		api.GET("/nas/image", h.NASImageProxy)

		// AI config endpoints
		api.GET("/ai/configs", h.ListAIConfigs)
		api.POST("/ai/configs", h.CreateAIConfig)
		api.PUT("/ai/configs/:id", h.UpdateAIConfig)
		api.PUT("/ai/configs/:id/activate", h.ActivateAIConfig)
		api.DELETE("/ai/configs/:id", h.DeleteAIConfig)
		api.POST("/ai/configs/:id/test", h.TestAIConfig)

		// AI endpoints
		ai := api.Group("/ai")
		{
			ai.POST("/summarize/:id", h.AISummarize)
			ai.POST("/search", h.AISearch)
			ai.POST("/edit", h.AIEdit)
			ai.POST("/index", h.TriggerIndex)
			ai.DELETE("/index", h.ResetIndexes)
			ai.GET("/index/status", h.GetIndexStatus)
			ai.GET("/notes/:id/related", h.GetRelatedNotes)
			ai.GET("/notes/:id/entities", h.GetNoteEntities)
			ai.GET("/conversations", h.ListConversations)
			ai.POST("/conversations", h.CreateConversation)
			ai.DELETE("/conversations/:id", h.DeleteConversation)
			ai.POST("/conversations/:id/messages", h.SendMessage)
			ai.POST("/query", h.RunQuery)
			ai.POST("/reflect", h.RunReflect)
			ai.GET("/reflect/status", h.GetReflectStatus)
			ai.GET("/outputs", h.ListOutputs)
			ai.GET("/outputs/:slug", h.GetOutput)
			ai.DELETE("/outputs/:slug", h.DeleteOutput)
		}

		// Notes detail endpoints
		api.GET("/notes/:id/summary", h.GetNoteSummary)

		// Wiki endpoints
		api.GET("/wiki/pages", h.ListWikiPages)
		api.GET("/wiki/pages/:slug", h.GetWikiPage)
		api.GET("/wiki/catalog", h.GetWikiCatalog)
		api.POST("/wiki/generate", h.GenerateWikiPage)
		api.DELETE("/wiki/pages/:slug", h.DeleteWikiPage)
		api.GET("/wiki/concepts", h.ListConcepts)
		api.GET("/wiki/concepts/:slug", h.GetConcept)
			api.DELETE("/wiki/concepts/:slug", h.DeleteConcept)
			api.PUT("/wiki/concepts/:slug/confirm-confidence", h.ConfirmConceptConfidence)
			api.GET("/wiki/graph", h.GetConceptGraph)
			api.POST("/wiki/graph/refresh", h.RefreshConceptGraph)
			api.GET("/wiki/entities", h.ListEntities)
			api.POST("/wiki/auto", h.AutoGenerateWiki)
			api.GET("/wiki/merge/candidates", h.GetMergeCandidates)
			api.POST("/wiki/merge", h.ExecuteMerge)

		// Questions endpoints
		api.POST("/questions", h.CreateQuestion)
		api.GET("/questions", h.ListQuestions)
		api.PUT("/questions/:id/resolve", h.ResolveQuestion)

		// Lint & Knowledge health
		api.GET("/ai/lint", h.RunLint)
		api.POST("/ai/lint/fix", h.FixLintIssues)
		api.GET("/ai/activities", h.GetActivityLog)
		api.POST("/ai/insights", h.SaveChatInsight)
		api.POST("/ai/insights/analyze", h.AnalyzeChatForInsights)
			api.POST("/ai/relations/translate", h.TranslateRelations)

		// Settings endpoints
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.UpdateSettings)
	}

	return r
}
