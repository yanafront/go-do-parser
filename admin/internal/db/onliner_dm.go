package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (db *DB) UpdateOnlinerDMStatus(ctx context.Context, id int64, status string) (*OnlinerPost, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "sent", "pending":
	default:
		return nil, fmt.Errorf("invalid status")
	}

	p, err := db.getOnlinerPostByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sourceChannel, err := db.onlinerSourceChannel(ctx, p.TopicID)
	if err != nil {
		return nil, err
	}

	body := strings.TrimSpace(p.Title)
	if b := strings.TrimSpace(p.Body); b != "" {
		if body != "" {
			body += "\n" + b
		} else {
			body = b
		}
	}

	_, err = db.sql.ExecContext(ctx, `
INSERT INTO job_seeker_posts (
    source_channel, source_message_id, source_message_link, body,
    poster_username, ad_username, ad_phone
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (source_channel, source_message_id) DO UPDATE SET
    source_message_link = COALESCE(EXCLUDED.source_message_link, job_seeker_posts.source_message_link),
    body = EXCLUDED.body,
    poster_username = COALESCE(NULLIF(EXCLUDED.poster_username, ''), job_seeker_posts.poster_username),
    ad_username = COALESCE(NULLIF(EXCLUDED.ad_username, ''), job_seeker_posts.ad_username),
    ad_phone = COALESCE(NULLIF(EXCLUDED.ad_phone, ''), job_seeker_posts.ad_phone)
`,
		sourceChannel,
		p.TopicID,
		p.TopicURL,
		body,
		nullStrPtr(p.PosterUsername),
		nullStrPtr(p.Telegram),
		nullStrPtr(p.Phone),
	)
	if err != nil {
		return nil, err
	}

	if status == "pending" {
		_, err = db.sql.ExecContext(ctx, `
UPDATE job_seeker_posts
SET dm_contact = NULL, dm_contact_type = NULL, dm_sent_at = NULL
WHERE source_channel = $1 AND source_message_id = $2
`, sourceChannel, p.TopicID)
		if err != nil {
			return nil, err
		}
		return db.getOnlinerPostByID(ctx, id)
	}

	contact, contactType := resolveOnlinerContact(*p)
	if contact == "" {
		contact = "manual"
		contactType = "manual"
	}
	sentAt := time.Now().UTC()
	_, err = db.sql.ExecContext(ctx, `
UPDATE job_seeker_posts
SET dm_contact = $1, dm_contact_type = $2, dm_sent_at = $3
WHERE source_channel = $4 AND source_message_id = $5
`, contact, contactType, sentAt, sourceChannel, p.TopicID)
	if err != nil {
		return nil, err
	}
	return db.getOnlinerPostByID(ctx, id)
}

func (db *DB) getOnlinerPostByID(ctx context.Context, id int64) (*OnlinerPost, error) {
	var p OnlinerPost
	err := db.sql.QueryRowContext(ctx, `
SELECT o.id, o.topic_id, o.topic_url, o.title, o.body,
       o.poster_user_id, o.poster_username, o.poster_profile_url,
       o.phone, o.email, o.telegram, o.created_at, o.parsed_at, o.posted_at,
       j.dm_contact, j.dm_contact_type, j.dm_sent_at
FROM onliner_posts o
LEFT JOIN job_seeker_posts j ON j.source_channel LIKE 'onliner:%' AND j.source_message_id = o.topic_id
WHERE o.id = $1
`, id).Scan(
		&p.ID, &p.TopicID, &p.TopicURL, &p.Title, &p.Body,
		&p.PosterUserID, &p.PosterUsername, &p.PosterProfileURL,
		&p.Phone, &p.Email, &p.Telegram, &p.CreatedAt, &p.ParsedAt, &p.PostedAt,
		&p.DMContact, &p.DMContactType, &p.DMSentAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) onlinerSourceChannel(ctx context.Context, topicID int) (string, error) {
	var ch string
	err := db.sql.QueryRowContext(ctx, `
SELECT source_channel FROM job_seeker_posts
WHERE source_message_id = $1 AND source_channel LIKE 'onliner:%'
LIMIT 1
`, topicID).Scan(&ch)
	if err == nil && strings.TrimSpace(ch) != "" {
		return ch, nil
	}
	err = db.sql.QueryRowContext(ctx, `
SELECT source_channel FROM job_seeker_posts
WHERE source_channel LIKE 'onliner:%'
GROUP BY source_channel
ORDER BY COUNT(*) DESC
LIMIT 1
`).Scan(&ch)
	if err == nil && strings.TrimSpace(ch) != "" {
		return ch, nil
	}
	return "onliner:34", nil
}

func resolveOnlinerContact(p OnlinerPost) (contact, contactType string) {
	if p.DMContact != nil {
		c := strings.TrimSpace(*p.DMContact)
		if c != "" && c != "none" {
			t := "username"
			if p.DMContactType != nil && strings.TrimSpace(*p.DMContactType) != "" {
				t = strings.TrimSpace(*p.DMContactType)
			}
			return c, t
		}
	}
	if p.Telegram != nil {
		if u := strings.TrimSpace(*p.Telegram); u != "" {
			return strings.TrimPrefix(u, "@"), "username"
		}
	}
	if p.Phone != nil {
		if ph := strings.TrimSpace(*p.Phone); ph != "" {
			return ph, "phone"
		}
	}
	return "", ""
}

func nullStrPtr(s *string) any {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return v
}
