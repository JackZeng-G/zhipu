package main

import (
	"log"
	"os"

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

	r := gin.Default()

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
