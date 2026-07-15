package db

import (
	"context"
	"fmt"
	"time"
)

type JobSeekerPost struct {
	SourceChannel     string
	SourceMessageID   int
	SourceMessageLink string
	Body              string
	PosterUsername    string
	PosterPhone       string
	AdUsername        string
	AdPhone         string
	DMContact       string
	DMContactType   string
	DMSentAt        *time.Time
}

func (db *DB) SaveJobSeekerPost(ctx context.Context, p JobSeekerPost) error {
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO job_seeker_posts (
    source_channel, source_message_id, source_message_link, body,
    poster_username, poster_phone, ad_username, ad_phone,
    dm_contact, dm_contact_type, dm_sent_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (source_channel, source_message_id) DO UPDATE SET
    source_message_link = COALESCE(EXCLUDED.source_message_link, job_seeker_posts.source_message_link),
    body = EXCLUDED.body,
    poster_username = COALESCE(NULLIF(EXCLUDED.poster_username, ''), job_seeker_posts.poster_username),
    poster_phone = COALESCE(NULLIF(EXCLUDED.poster_phone, ''), job_seeker_posts.poster_phone),
    ad_username = COALESCE(NULLIF(EXCLUDED.ad_username, ''), job_seeker_posts.ad_username),
    ad_phone = COALESCE(NULLIF(EXCLUDED.ad_phone, ''), job_seeker_posts.ad_phone),
    dm_contact = COALESCE(EXCLUDED.dm_contact, job_seeker_posts.dm_contact),
    dm_contact_type = COALESCE(EXCLUDED.dm_contact_type, job_seeker_posts.dm_contact_type),
    dm_sent_at = COALESCE(EXCLUDED.dm_sent_at, job_seeker_posts.dm_sent_at)
`,
		p.SourceChannel,
		p.SourceMessageID,
		nullStr(p.SourceMessageLink),
		p.Body,
		nullStr(p.PosterUsername),
		nullStr(p.PosterPhone),
		nullStr(p.AdUsername),
		nullStr(p.AdPhone),
		nullStr(p.DMContact),
		nullStr(p.DMContactType),
		p.DMSentAt,
	)
	if err != nil {
		return fmt.Errorf("save job seeker post: %w", err)
	}
	return nil
}

func (db *DB) ListPendingSeekerDMs(ctx context.Context, limit int) ([]JobSeekerPost, error) {
	if limit <= 0 {
		limit = 1
	}
	rows, err := db.sql.QueryContext(ctx, `
SELECT source_channel, source_message_id, COALESCE(source_message_link, ''), body,
       COALESCE(poster_username, ''), COALESCE(poster_phone, ''),
       COALESCE(ad_username, ''), COALESCE(ad_phone, '')
FROM job_seeker_posts
WHERE (dm_contact IS NULL OR dm_contact = '')
ORDER BY created_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending seeker dms: %w", err)
	}
	defer rows.Close()

	var out []JobSeekerPost
	for rows.Next() {
		var p JobSeekerPost
		if err := rows.Scan(
			&p.SourceChannel,
			&p.SourceMessageID,
			&p.SourceMessageLink,
			&p.Body,
			&p.PosterUsername,
			&p.PosterPhone,
			&p.AdUsername,
			&p.AdPhone,
		); err != nil {
			return nil, fmt.Errorf("scan pending seeker dm: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pending seeker dms: %w", err)
	}
	return out, nil
}

func (db *DB) CountPendingSeekerDMs(ctx context.Context) (int, error) {
	var n int
	err := db.sql.QueryRowContext(ctx, `
SELECT COUNT(*) FROM job_seeker_posts
WHERE (dm_contact IS NULL OR dm_contact = '')
`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count pending seeker dms: %w", err)
	}
	return n, nil
}

func (db *DB) UpdateJobSeekerDM(ctx context.Context, sourceChannel string, messageID int, contact, contactType string, sentAt time.Time) error {
	_, err := db.sql.ExecContext(ctx, `
UPDATE job_seeker_posts
SET dm_contact = $1, dm_contact_type = $2, dm_sent_at = $3
WHERE source_channel = $4 AND source_message_id = $5
`,
		contact, contactType, sentAt.UTC(), sourceChannel, messageID,
	)
	if err != nil {
		return fmt.Errorf("update job seeker dm: %w", err)
	}
	return nil
}
