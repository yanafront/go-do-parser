package db

import (
	"context"
	"fmt"
	"time"
)

type OnlinerPost struct {
	TopicID          int
	TopicURL         string
	Title            string
	Body             string
	PosterUserID     string
	PosterUsername   string
	PosterProfileURL string
	Phone            string
	Email            string
	Telegram         string
	PostedAt         *time.Time
}

func (db *DB) SaveOnlinerPost(ctx context.Context, p OnlinerPost) error {
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO onliner_posts (
    topic_id, topic_url, title, body,
    poster_user_id, poster_username, poster_profile_url,
    phone, email, telegram, posted_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (topic_id) DO UPDATE SET
    topic_url = EXCLUDED.topic_url,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    poster_user_id = COALESCE(NULLIF(EXCLUDED.poster_user_id, ''), onliner_posts.poster_user_id),
    poster_username = COALESCE(NULLIF(EXCLUDED.poster_username, ''), onliner_posts.poster_username),
    poster_profile_url = COALESCE(NULLIF(EXCLUDED.poster_profile_url, ''), onliner_posts.poster_profile_url),
    phone = COALESCE(NULLIF(EXCLUDED.phone, ''), onliner_posts.phone),
    email = COALESCE(NULLIF(EXCLUDED.email, ''), onliner_posts.email),
    telegram = COALESCE(NULLIF(EXCLUDED.telegram, ''), onliner_posts.telegram),
    posted_at = COALESCE(EXCLUDED.posted_at, onliner_posts.posted_at),
    parsed_at = NOW()
`,
		p.TopicID,
		nullStr(p.TopicURL),
		p.Title,
		p.Body,
		nullStr(p.PosterUserID),
		nullStr(p.PosterUsername),
		nullStr(p.PosterProfileURL),
		nullStr(p.Phone),
		nullStr(p.Email),
		nullStr(p.Telegram),
		nullTime(p.PostedAt),
	)
	if err != nil {
		return fmt.Errorf("save onliner post: %w", err)
	}
	return nil
}

type OnlinerPendingDM struct {
	TopicID          int
	TopicURL         string
	Title            string
	Body             string
	PosterUsername   string
	Phone            string
	Telegram         string
}

func (db *DB) ListOnlinerPostsPendingDM(ctx context.Context, forumID, limit int) ([]OnlinerPendingDM, error) {
	if limit <= 0 {
		limit = 100
	}
	sourceChannel := fmt.Sprintf("onliner:%d", forumID)
	rows, err := db.sql.QueryContext(ctx, `
SELECT o.topic_id,
       COALESCE(o.topic_url, ''),
       COALESCE(o.title, ''),
       COALESCE(o.body, ''),
       COALESCE(o.poster_username, ''),
       COALESCE(o.phone, ''),
       COALESCE(o.telegram, '')
FROM onliner_posts o
WHERE (COALESCE(o.phone, '') <> '' OR COALESCE(o.telegram, '') <> '')
  AND NOT EXISTS (
    SELECT 1 FROM job_seeker_posts j
    WHERE j.source_channel = $1 AND j.source_message_id = o.topic_id
  )
ORDER BY o.parsed_at ASC
LIMIT $2
`, sourceChannel, limit)
	if err != nil {
		return nil, fmt.Errorf("list onliner pending dm: %w", err)
	}
	defer rows.Close()

	var out []OnlinerPendingDM
	for rows.Next() {
		var p OnlinerPendingDM
		if err := rows.Scan(
			&p.TopicID,
			&p.TopicURL,
			&p.Title,
			&p.Body,
			&p.PosterUsername,
			&p.Phone,
			&p.Telegram,
		); err != nil {
			return nil, fmt.Errorf("scan onliner pending dm: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list onliner pending dm: %w", err)
	}
	return out, nil
}
