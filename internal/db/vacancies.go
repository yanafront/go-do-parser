package db

import (
	"context"
	"fmt"
	"time"
)

type Vacancy struct {
	SourceChannel   string
	SourceMessageID int
	DestMessageID   int
	Body            string
	AdUsername      string
	AdPhone         string
	DMContact       string
	DMContactType   string
	DMSentAt        *time.Time
	PublishedAt     time.Time
}

func (db *DB) SaveVacancy(ctx context.Context, v Vacancy) error {
	if v.PublishedAt.IsZero() {
		v.PublishedAt = time.Now().UTC()
	}
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO vacancies (
    source_channel, source_message_id, dest_message_id, body,
    ad_username, ad_phone, dm_contact, dm_contact_type, dm_sent_at, published_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (source_channel, source_message_id) DO UPDATE SET
    dest_message_id = EXCLUDED.dest_message_id,
    body = EXCLUDED.body,
    ad_username = EXCLUDED.ad_username,
    ad_phone = EXCLUDED.ad_phone,
    published_at = EXCLUDED.published_at,
    dm_contact = COALESCE(EXCLUDED.dm_contact, vacancies.dm_contact),
    dm_contact_type = COALESCE(EXCLUDED.dm_contact_type, vacancies.dm_contact_type),
    dm_sent_at = COALESCE(EXCLUDED.dm_sent_at, vacancies.dm_sent_at)
`,
		v.SourceChannel,
		v.SourceMessageID,
		nullInt(v.DestMessageID),
		v.Body,
		nullStr(v.AdUsername),
		nullStr(v.AdPhone),
		nullStr(v.DMContact),
		nullStr(v.DMContactType),
		v.DMSentAt,
		v.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("save vacancy: %w", err)
	}
	return nil
}

func (db *DB) UpdateVacancyDM(ctx context.Context, sourceChannel string, messageID int, contact, contactType string, sentAt time.Time) error {
	_, err := db.sql.ExecContext(ctx, `
UPDATE vacancies
SET dm_contact = $1, dm_contact_type = $2, dm_sent_at = $3
WHERE source_channel = $4 AND source_message_id = $5
`,
		contact, contactType, sentAt.UTC(), sourceChannel, messageID,
	)
	if err != nil {
		return fmt.Errorf("update vacancy dm: %w", err)
	}
	return nil
}
