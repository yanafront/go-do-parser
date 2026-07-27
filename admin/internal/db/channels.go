package db

import (
	"context"
	"fmt"
	"strings"
)

type Channel struct {
	Username string `json:"username"`
	Title    string `json:"title,omitempty"`
}

func (db *DB) ListVacancyChannels(ctx context.Context) ([]Channel, error) {
	return db.listChannels(ctx, "vacancies")
}

func (db *DB) ListJobSeekerChannels(ctx context.Context) ([]Channel, error) {
	return db.listChannels(ctx, "job_seeker_posts")
}

func (db *DB) listChannels(ctx context.Context, table string) ([]Channel, error) {
	query := fmt.Sprintf(`
SELECT DISTINCT t.source_channel, COALESCE(c.title, '')
FROM %s t
LEFT JOIN channels c ON LOWER(c.username) = LOWER(t.source_channel)
WHERE t.source_channel <> '' AND t.source_channel NOT LIKE 'onliner:%%'
ORDER BY COALESCE(NULLIF(c.title, ''), t.source_channel)
`, table)
	rows, err := db.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.Username, &ch.Title); err != nil {
			return nil, err
		}
		ch.Username = strings.TrimSpace(ch.Username)
		ch.Title = strings.TrimSpace(ch.Title)
		out = append(out, ch)
	}
	return out, rows.Err()
}
