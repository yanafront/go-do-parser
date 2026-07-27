package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (db *DB) UpsertChannel(ctx context.Context, username, title string) error {
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	title = strings.TrimSpace(title)
	if username == "" {
		return nil
	}
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO channels (username, title, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (username) DO UPDATE SET
    title = CASE
        WHEN EXCLUDED.title <> '' THEN EXCLUDED.title
        ELSE channels.title
    END,
    updated_at = EXCLUDED.updated_at
`, strings.ToLower(username), title, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert channel: %w", err)
	}
	return nil
}
