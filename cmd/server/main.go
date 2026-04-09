package main

import (
	"log"
	"os"

	"personal-kb/internal/api"
	"personal-kb/internal/nas"
	"personal-kb/internal/ollama"
	"personal-kb/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	dbPath := envOrDefault("KB_DB_PATH", "knowledge.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Create stores
	notesStore := store.NewNotesStore(db)
	settingsStore := store.NewSettingsStore(db)
	convStore := store.NewConversationsStore(db)

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
		}
	}

	// Create handlers with all dependencies
	handlers := api.NewHandlers(
		notesStore,
		settingsStore,
		convStore,
		nasClient,
		authClient,
		ollamaClient,
		nil, // syncService will be created on connect
	)

	// Setup router
	r := api.SetupRouter(handlers)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
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
