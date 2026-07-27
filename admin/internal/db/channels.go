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
SELECT source_channel, title
FROM (
    SELECT t.source_channel AS source_channel,
           COALESCE(MAX(c.title), '') AS title
    FROM %s t
    LEFT JOIN channels c ON c.username = LOWER(t.source_channel)
    WHERE t.source_channel <> '' AND t.source_channel NOT LIKE 'onliner:%%'
    GROUP BY t.source_channel
) x
ORDER BY COALESCE(NULLIF(title, ''), source_channel)
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
