package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (db *DB) UpdateJobSeekerDMStatus(ctx context.Context, id int64, status, changedBy string) (*JobSeekerPost, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "sent", "pending":
	default:
		return nil, fmt.Errorf("invalid status")
	}
	changedBy = strings.TrimSpace(strings.TrimPrefix(changedBy, "@"))

	var p JobSeekerPost
	err := db.sql.QueryRowContext(ctx, `
SELECT id, source_channel, source_message_id, source_message_link, body,
       poster_username, poster_phone, ad_username, ad_phone,
       dm_contact, dm_contact_type, dm_sent_at, dm_message, dm_status_changed_by, created_at
FROM job_seeker_posts
WHERE id = $1 AND source_channel NOT LIKE 'onliner:%'
`, id).Scan(
		&p.ID, &p.SourceChannel, &p.SourceMessageID, &p.SourceMessageLink, &p.Body,
		&p.PosterUsername, &p.PosterPhone, &p.AdUsername, &p.AdPhone,
		&p.DMContact, &p.DMContactType, &p.DMSentAt, &p.DMMessage, &p.DMStatusChangedBy, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, err
	}

	if status == "pending" {
		_, err = db.sql.ExecContext(ctx, `
UPDATE job_seeker_posts
SET dm_contact = NULL, dm_contact_type = NULL, dm_sent_at = NULL, dm_message = NULL,
    dm_claimed_by = NULL, dm_claimed_at = NULL, dm_status_changed_by = $2
WHERE id = $1
`, id, nullIfEmpty(changedBy))
		if err != nil {
			return nil, err
		}
		p.DMContact = nil
		p.DMContactType = nil
		p.DMSentAt = nil
		p.DMMessage = nil
		if changedBy != "" {
			p.DMStatusChangedBy = &changedBy
		} else {
			p.DMStatusChangedBy = nil
		}
		return &p, nil
	}

	contact, contactType := resolveSeekerContact(p)
	if contact == "" {
		contact = "manual"
		contactType = "manual"
	}
	sentAt := time.Now().UTC()
	_, err = db.sql.ExecContext(ctx, `
UPDATE job_seeker_posts
SET dm_contact = $1, dm_contact_type = $2, dm_sent_at = $3, dm_status_changed_by = $4
WHERE id = $5
`, contact, contactType, sentAt, nullIfEmpty(changedBy), id)
	if err != nil {
		return nil, err
	}
	p.DMContact = &contact
	p.DMContactType = &contactType
	p.DMSentAt = &sentAt
	if changedBy != "" {
		p.DMStatusChangedBy = &changedBy
	} else {
		p.DMStatusChangedBy = nil
	}
	return &p, nil
}

func resolveSeekerContact(p JobSeekerPost) (contact, contactType string) {
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
	if p.AdUsername != nil {
		if u := strings.TrimSpace(*p.AdUsername); u != "" {
			return strings.TrimPrefix(u, "@"), "username"
		}
	}
	if p.AdPhone != nil {
		if ph := strings.TrimSpace(*p.AdPhone); ph != "" {
			return ph, "phone"
		}
	}
	if p.PosterUsername != nil {
		if u := strings.TrimSpace(*p.PosterUsername); u != "" {
			return strings.TrimPrefix(u, "@"), "username"
		}
	}
	if p.PosterPhone != nil {
		if ph := strings.TrimSpace(*p.PosterPhone); ph != "" {
			return ph, "phone"
		}
	}
	return "", ""
}

func nullIfEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
