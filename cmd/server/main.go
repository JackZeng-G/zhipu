package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"personal-kb/internal/api"
	"personal-kb/internal/nas"
	"personal-kb/internal/ollama"
	"personal-kb/internal/store"
	"personal-kb/internal/sync"
	"personal-kb/web"

	"github.com/gin-gonic/gin"
)

func main() {
	// 将日志重定向到文件，避免在 windowsgui 模式下弹出窗口
	logFile, err := os.OpenFile(filepath.Join("data", "server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
	}

	gin.SetMode(gin.ReleaseMode)

	dbPath := envOrDefault("KB_DB_PATH", "knowledge.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Ensure data directories exist for image cache
	os.MkdirAll(filepath.Join("data", "images"), 0755)

	// Create stores
	notesStore := store.NewNotesStore(db)
	settingsStore := store.NewSettingsStore(db)
	convStore := store.NewConversationsStore(db)
	aiConfigStore := store.NewAIConfigStore(db, settingsStore)
	knowledgeStore := store.NewKnowledgeStore(db)

	// Restore NAS settings from environment (fallback for initial setup)
	nasHostEnv := envOrDefault("KB_NAS_HOST", "")
	if nasHostEnv != "" {
		if nasHost, _ := settingsStore.GetSetting("nas_host"); nasHost == "" {
			settingsStore.SetSetting("nas_host", nasHostEnv)
			log.Printf("[main] set NAS host from env: %s", nasHostEnv)
		}
	}

	// Create Ollama client with default settings
	ollamaURL, _ := settingsStore.GetSetting("ollama_url")
	ollamaModel, _ := settingsStore.GetSetting("ollama_model")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if ollamaModel == "" {
		ollamaModel = "qwen2"
	}
	ollamaClient := ollama.NewClient(ollamaURL, ollamaModel)

	// Try to restore NAS session from settings
	var authClient *nas.AuthClient
	var nasClient *nas.NoteStationClient
	var scheduler *sync.Scheduler
	var syncService *sync.SyncService
	nasHost, _ := settingsStore.GetSetting("nas_host")
	nasPort, _ := settingsStore.GetSetting("nas_port")
	nasUsername, _ := settingsStore.GetSetting("nas_username")
	nasPassword, _ := settingsStore.GetSetting("nas_password_encrypted")

	if nasHost != "" && nasUsername != "" && nasPassword != "" {
		scheme := "https"
		baseURL := scheme + "://" + nasHost
		if nasPort != "" {
			baseURL = baseURL + ":" + nasPort
		}
		authClient = nas.NewAuthClient(baseURL, true)
		if err := authClient.Login(nasUsername, nasPassword); err != nil {
			log.Printf("[main] failed to restore NAS session: %v", err)
			authClient = nil
		} else {
			nasClient = nas.NewNoteStationClient(authClient)
			log.Printf("[main] restored NAS session for %s", nasHost)

			// Start auto-sync scheduler
			syncService = sync.NewSyncService(nasClient, notesStore)
			scheduler = sync.NewScheduler(syncService, sync.DefaultInterval)
			scheduler.Start()
		}
	} else {
		log.Printf("[main] no NAS host configured, skipping auto-connect")
	}

	// Create handlers with all dependencies
	handlers := api.NewHandlers(
		notesStore,
		settingsStore,
		convStore,
		aiConfigStore,
		knowledgeStore,
		nasClient,
		authClient,
		ollamaClient,
		syncService,
	)

	// Wire up auto-indexing: after sync completes, index updated notes
	if syncService != nil {
		syncService.SetPostSyncHook(handlers.IndexNotesAsync)
	}

	// Auto-index unindexed notes on startup if AI provider is active
	if handlers.GetProvider() != nil {
		go handlers.AutoIndexUnindexedNotes()
	}

	// Setup router
	r := api.SetupRouter(handlers)

	// Serve embedded frontend static files with SPA fallback
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("failed to create sub filesystem for static assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Skip API routes — let them return 404 naturally
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// Try to serve the static file
		f, err := distFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		// SPA fallback: serve index.html for all non-file routes
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	addr := ":" + envOrDefault("KB_PORT", "8080")
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
