package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Conversation represents an AI conversation thread.
type Conversation struct {
	ID        int64   `db:"id" json:"id"`
	NoteID    *string `db:"note_id" json:"note_id"`
	Title     string  `db:"title" json:"title"`
	CreatedAt int64   `db:"created_at" json:"created_at"`
}

// Message represents a single message within a conversation.
type Message struct {
	ID             int64  `db:"id" json:"id"`
	ConversationID int64  `db:"conversation_id" json:"conversation_id"`
	Role           string `db:"role" json:"role"`
	Content        string `db:"content" json:"content"`
	CreatedAt      int64  `db:"created_at" json:"created_at"`
}

// ConversationsStore provides CRUD for conversations and messages.
type ConversationsStore struct {
	db *sqlx.DB
}

// NewConversationsStore creates a conversations store.
func NewConversationsStore(db *sqlx.DB) *ConversationsStore {
	return &ConversationsStore{db: db}
}

// CreateConversation inserts a new conversation and returns its ID.
func (s *ConversationsStore) CreateConversation(ctx context.Context, noteID *string, title string) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO conversations (note_id, title, created_at) VALUES (?, ?, strftime('%s','now'))",
		noteID, title)
	if err != nil {
		return 0, fmt.Errorf("create conversation: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}
	return id, nil
}

// GetConversation retrieves a conversation by ID.
func (s *ConversationsStore) GetConversation(ctx context.Context, id int64) (*Conversation, error) {
	var conv Conversation
	if err := s.db.GetContext(ctx, &conv, "SELECT * FROM conversations WHERE id = ?", id); err != nil {
		return nil, fmt.Errorf("get conversation %d: %w", id, err)
	}
	return &conv, nil
}

// ListConversations returns conversations, optionally filtered by noteID.
func (s *ConversationsStore) ListConversations(ctx context.Context, noteID *string) ([]Conversation, error) {
	var convs []Conversation
	var err error
	if noteID != nil {
		err = s.db.SelectContext(ctx, &convs,
			"SELECT * FROM conversations WHERE note_id = ? ORDER BY created_at DESC", *noteID)
	} else {
		err = s.db.SelectContext(ctx, &convs,
			"SELECT * FROM conversations ORDER BY created_at DESC")
	}
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	return convs, nil
}

// DeleteConversation removes a conversation and its messages.
func (s *ConversationsStore) DeleteConversation(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM messages WHERE conversation_id = ?", id); err != nil {
		return fmt.Errorf("delete messages for conversation %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM conversations WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete conversation %d: %w", id, err)
	}
	return tx.Commit()
}

// AddMessage inserts a new message into a conversation.
func (s *ConversationsStore) AddMessage(ctx context.Context, convID int64, role, content string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO messages (conversation_id, role, content, created_at) VALUES (?, ?, ?, strftime('%s','now'))",
		convID, role, content)
	if err != nil {
		return fmt.Errorf("add message to conversation %d: %w", convID, err)
	}
	return nil
}

// GetMessages returns all messages for a conversation, ordered by creation time.
func (s *ConversationsStore) GetMessages(ctx context.Context, convID int64) ([]Message, error) {
	var msgs []Message
	if err := s.db.SelectContext(ctx, &msgs,
		"SELECT * FROM messages WHERE conversation_id = ? ORDER BY created_at ASC", convID); err != nil {
		return nil, fmt.Errorf("get messages for conversation %d: %w", convID, err)
	}
	return msgs, nil
}
