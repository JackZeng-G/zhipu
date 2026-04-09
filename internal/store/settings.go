package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/jmoiron/sqlx"
)

// deriveKey generates a 32-byte AES key from the environment variable
// KB_ENCRYPTION_KEY. If not set, a random key is generated (data will be
// lost across restarts unless persisted elsewhere).
func deriveKey() []byte {
	envKey := os.Getenv("KB_ENCRYPTION_KEY")
	if envKey == "" {
		// Return random 32 bytes; this means encrypted data is session-only
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic("failed to generate random key: " + err.Error())
		}
		return key
	}
	// Derive 32-byte key from user-provided string via hex decode if valid,
	// otherwise hash it using SHA-256 truncated to 32 bytes.
	key, err := hex.DecodeString(envKey)
	if err != nil || len(key) != 32 {
		// Simple fallback: pad/truncate string to 32 bytes
		key = make([]byte, 32)
		copy(key, []byte(envKey))
	}
	return key
}

var globalKey = deriveKey()

// encrypt encrypts plaintext using AES-GCM with a random nonce.
func encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(globalKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts base64-encoded ciphertext using AES-GCM.
func decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	block, err := aes.NewCipher(globalKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, cipherText := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

// SettingsStore provides CRUD for encrypted settings.
type SettingsStore struct {
	db *sqlx.DB
}

// NewSettingsStore creates a settings store.
func NewSettingsStore(db *sqlx.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

// GetSetting retrieves a setting value. If the key is marked as sensitive
// (key suffix "_encrypted"), it is automatically decrypted.
func (s *SettingsStore) GetSetting(key string) (string, error) {
	var val string
	if err := s.db.Get(&val, "SELECT value FROM settings WHERE key = ?", key); err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	if len(key) > 10 && key[len(key)-10:] == "_encrypted" {
		decrypted, err := decrypt(val)
		if err != nil {
			return "", fmt.Errorf("decrypt setting %s: %w", key, err)
		}
		return decrypted, nil
	}
	return val, nil
}

// SetSetting stores a setting value. If the key ends with "_encrypted",
// the value is encrypted before storage.
func (s *SettingsStore) SetSetting(key string, value string) error {
	if len(key) > 10 && key[len(key)-10:] == "_encrypted" {
		encrypted, err := encrypt(value)
		if err != nil {
			return fmt.Errorf("encrypt setting %s: %w", key, err)
		}
		value = encrypted
	}
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}
