package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ActivityLogEntry represents a logged activity.
type ActivityLogEntry struct {
	ID           int    `db:"id" json:"id"`
	ActivityType string `db:"activity_type" json:"activity_type"`
	TargetType   string `db:"target_type" json:"target_type"`
	TargetID     string `db:"target_id" json:"target_id"`
	Description  string `db:"description" json:"description"`
	Metadata     string `db:"metadata" json:"metadata"`
	CreatedAt    int64  `db:"created_at" json:"created_at"`
}

// LogActivity writes an activity log entry.
func (s *KnowledgeStore) LogActivity(ctx context.Context, activityType, targetType, targetID, description string, metadata map[string]interface{}) error {
	metaJSON, _ := json.Marshal(metadata)
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO activity_log (activity_type, target_type, target_id, description, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		activityType, targetType, targetID, description, string(metaJSON), now)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}

// GetRecentActivities returns the most recent N activity log entries.
func (s *KnowledgeStore) GetRecentActivities(ctx context.Context, limit int) ([]ActivityLogEntry, error) {
	var entries []ActivityLogEntry
	if err := s.db.SelectContext(ctx, &entries,
		"SELECT * FROM activity_log ORDER BY created_at DESC LIMIT ?", limit); err != nil {
		return nil, fmt.Errorf("get recent activities: %w", err)
	}
	return entries, nil
}
