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
	PosterUsername  string
	AdUsername      string
	AdPhone         string
	DMContact       string
	DMContactType   string
	DMSentAt        *time.Time
}

func (db *DB) SaveJobSeekerPost(ctx context.Context, p JobSeekerPost) error {
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO job_seeker_posts (
    source_channel, source_message_id, source_message_link, body,
    poster_username, ad_username, ad_phone,
    dm_contact, dm_contact_type, dm_sent_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (source_channel, source_message_id) DO UPDATE SET
    source_message_link = COALESCE(EXCLUDED.source_message_link, job_seeker_posts.source_message_link),
    body = EXCLUDED.body,
    poster_username = EXCLUDED.poster_username,
    ad_username = EXCLUDED.ad_username,
    ad_phone = EXCLUDED.ad_phone,
    dm_contact = COALESCE(EXCLUDED.dm_contact, job_seeker_posts.dm_contact),
    dm_contact_type = COALESCE(EXCLUDED.dm_contact_type, job_seeker_posts.dm_contact_type),
    dm_sent_at = COALESCE(EXCLUDED.dm_sent_at, job_seeker_posts.dm_sent_at)
`,
		p.SourceChannel,
		p.SourceMessageID,
		nullStr(p.SourceMessageLink),
		p.Body,
		nullStr(p.PosterUsername),
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
