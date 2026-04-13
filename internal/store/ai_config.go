package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// AIConfig represents a saved AI provider configuration.
type AIConfig struct {
	ID             int     `db:"id" json:"id"`
	Provider       string  `db:"provider" json:"provider"`
	Name           string  `db:"name" json:"name"`
	BaseURL        *string `db:"base_url" json:"base_url"`
	Model          string  `db:"model" json:"model"`
	APIKeyEncrypted *string `db:"api_key_encrypted" json:"-"`
	IsActive       bool    `db:"is_active" json:"is_active"`
	CreatedAt      int64   `db:"created_at" json:"created_at"`
}

// AIConfigStore provides CRUD for AI provider configurations.
type AIConfigStore struct {
	db       *sqlx.DB
	settings *SettingsStore // for encryption
}

// NewAIConfigStore creates a new AI config store.
func NewAIConfigStore(db *sqlx.DB, settings *SettingsStore) *AIConfigStore {
	return &AIConfigStore{db: db, settings: settings}
}

// ListAIConfigs returns all saved AI configurations.
func (s *AIConfigStore) ListAIConfigs(ctx context.Context) ([]AIConfig, error) {
	var configs []AIConfig
	if err := s.db.SelectContext(ctx, &configs,
		"SELECT id, provider, name, base_url, model, is_active, created_at FROM ai_configs ORDER BY created_at"); err != nil {
		return nil, fmt.Errorf("list ai configs: %w", err)
	}
	return configs, nil
}

// GetActiveAIConfig returns the currently active AI configuration.
func (s *AIConfigStore) GetActiveAIConfig(ctx context.Context) (*AIConfig, error) {
	var config AIConfig
	if err := s.db.GetContext(ctx, &config,
		"SELECT id, provider, name, base_url, model, is_active, created_at FROM ai_configs WHERE is_active = 1 LIMIT 1"); err != nil {
		return nil, fmt.Errorf("get active ai config: %w", err)
	}
	return &config, nil
}

// GetAIConfig returns a specific AI configuration by ID, including decrypted API key.
func (s *AIConfigStore) GetAIConfig(ctx context.Context, id int) (*AIConfig, error) {
	var config AIConfig
	if err := s.db.GetContext(ctx, &config,
		"SELECT * FROM ai_configs WHERE id = ?", id); err != nil {
		return nil, fmt.Errorf("get ai config %d: %w", id, err)
	}
	return &config, nil
}

// CreateAIConfig creates a new AI configuration.
func (s *AIConfigStore) CreateAIConfig(ctx context.Context, provider, name, baseURL, model, apiKey string) (int, error) {
	now := time.Now().Unix()

	var apiKeyEncrypted *string
	if apiKey != "" {
		enc, err := s.settings.EncryptValue(apiKey)
		if err != nil {
			return 0, fmt.Errorf("encrypt api key: %w", err)
		}
		apiKeyEncrypted = &enc
	}

	result, err := s.db.ExecContext(ctx,
		"INSERT INTO ai_configs (provider, name, base_url, model, api_key_encrypted, is_active, created_at) VALUES (?, ?, ?, ?, ?, 0, ?)",
		provider, name, nilIfEmpty(baseURL), model, apiKeyEncrypted, now)
	if err != nil {
		return 0, fmt.Errorf("create ai config: %w", err)
	}

	id, _ := result.LastInsertId()
	return int(id), nil
}

// UpdateAIConfig updates an existing AI configuration.
func (s *AIConfigStore) UpdateAIConfig(ctx context.Context, id int, provider, name, baseURL, model, apiKey string) error {
	// Only update API key if explicitly provided (non-empty)
	if apiKey != "" {
		enc, err := s.settings.EncryptValue(apiKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		_, err = s.db.ExecContext(ctx,
			"UPDATE ai_configs SET provider=?, name=?, base_url=?, model=?, api_key_encrypted=? WHERE id=?",
			provider, name, nilIfEmpty(baseURL), model, &enc, id)
		if err != nil {
			return fmt.Errorf("update ai config %d: %w", id, err)
		}
	} else {
		// Update without touching API key
		_, err := s.db.ExecContext(ctx,
			"UPDATE ai_configs SET provider=?, name=?, base_url=?, model=? WHERE id=?",
			provider, name, nilIfEmpty(baseURL), model, id)
		if err != nil {
			return fmt.Errorf("update ai config %d: %w", id, err)
		}
	}
	return nil
}

// SetActiveAIConfig activates a configuration and deactivates all others.
func (s *AIConfigStore) SetActiveAIConfig(ctx context.Context, id int) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all
	if _, err := tx.ExecContext(ctx, "UPDATE ai_configs SET is_active = 0"); err != nil {
		return fmt.Errorf("deactivate all: %w", err)
	}
	// Activate target
	if _, err := tx.ExecContext(ctx, "UPDATE ai_configs SET is_active = 1 WHERE id = ?", id); err != nil {
		return fmt.Errorf("activate %d: %w", id, err)
	}

	return tx.Commit()
}

// DeleteAIConfig removes a configuration by ID.
func (s *AIConfigStore) DeleteAIConfig(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM ai_configs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete ai config %d: %w", id, err)
	}
	return nil
}

// GetDecryptedAPIKey returns the decrypted API key for a config.
func (s *AIConfigStore) GetDecryptedAPIKey(ctx context.Context, id int) (string, error) {
	var encrypted *string
	if err := s.db.GetContext(ctx, &encrypted, "SELECT api_key_encrypted FROM ai_configs WHERE id = ?", id); err != nil {
		return "", fmt.Errorf("get api key for config %d: %w", id, err)
	}
	if encrypted == nil {
		return "", nil
	}
	return s.settings.DecryptValue(*encrypted)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
