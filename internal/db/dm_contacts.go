package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type DMContactInfo struct {
	Contact string
	Type    string
	SentAt  time.Time
}

func normalizeDMContact(contactType, contact string) (string, string) {
	contactType = strings.TrimSpace(contactType)
	contact = strings.TrimSpace(contact)
	if contactType == "username" {
		contact = strings.ToLower(strings.TrimPrefix(contact, "@"))
	}
	return contactType, contact
}

func (db *DB) FirstDMSentAt(ctx context.Context, contactType, contact string) (*time.Time, error) {
	if db == nil || db.sql == nil {
		return nil, nil
	}
	contactType, contact = normalizeDMContact(contactType, contact)
	if contactType == "" || contact == "" {
		return nil, nil
	}

	var sentAt sql.NullTime
	err := db.sql.QueryRowContext(ctx, `
SELECT MIN(dm_sent_at) FROM (
  SELECT dm_sent_at FROM vacancies
  WHERE dm_sent_at IS NOT NULL
    AND dm_contact_type = $1
    AND (
      ($1 = 'username' AND lower(coalesce(dm_contact, '')) = $2) OR
      ($1 <> 'username' AND coalesce(dm_contact, '') = $2)
    )
  UNION ALL
  SELECT dm_sent_at FROM job_seeker_posts
  WHERE dm_sent_at IS NOT NULL
    AND dm_contact_type = $1
    AND (
      ($1 = 'username' AND lower(coalesce(dm_contact, '')) = $2) OR
      ($1 <> 'username' AND coalesce(dm_contact, '') = $2)
    )
) t
`, contactType, contact).Scan(&sentAt)
	if err != nil {
		return nil, fmt.Errorf("first dm sent at: %w", err)
	}
	if !sentAt.Valid || sentAt.Time.IsZero() {
		return nil, nil
	}
	t := sentAt.Time.UTC()
	return &t, nil
}

func (db *DB) WasDMContacted(ctx context.Context, contactType, contact string) (bool, *time.Time, error) {
	sentAt, err := db.FirstDMSentAt(ctx, contactType, contact)
	if err != nil {
		return false, nil, err
	}
	return sentAt != nil, sentAt, nil
}

