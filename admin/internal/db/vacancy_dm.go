package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (db *DB) UpdateVacancyDMStatus(ctx context.Context, id int64, status, changedBy string) (*Vacancy, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "sent", "pending":
	default:
		return nil, fmt.Errorf("invalid status")
	}
	changedBy = strings.TrimSpace(strings.TrimPrefix(changedBy, "@"))

	var v Vacancy
	err := db.sql.QueryRowContext(ctx, `
SELECT vacancies.id, vacancies.source_channel, COALESCE(c.title, ''),
       vacancies.source_message_id, vacancies.source_message_link, vacancies.dest_message_id, vacancies.body,
       vacancies.ad_username, vacancies.ad_phone, vacancies.dm_contact, vacancies.dm_contact_type, vacancies.dm_sent_at,
       vacancies.dm_status_changed_by, vacancies.published_at, vacancies.created_at
FROM vacancies
LEFT JOIN channels c ON c.username = LOWER(vacancies.source_channel)
WHERE vacancies.id = $1
`, id).Scan(
		&v.ID, &v.SourceChannel, &v.SourceChannelTitle, &v.SourceMessageID, &v.SourceMessageLink, &v.DestMessageID, &v.Body,
		&v.AdUsername, &v.AdPhone, &v.DMContact, &v.DMContactType, &v.DMSentAt,
		&v.DMStatusChangedBy, &v.PublishedAt, &v.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, err
	}

	if status == "pending" {
		_, err = db.sql.ExecContext(ctx, `
UPDATE vacancies
SET dm_contact = NULL, dm_contact_type = NULL, dm_sent_at = NULL, dm_status_changed_by = $2
WHERE id = $1
`, id, nullIfEmpty(changedBy))
		if err != nil {
			return nil, err
		}
		v.DMContact = nil
		v.DMContactType = nil
		v.DMSentAt = nil
		if changedBy != "" {
			v.DMStatusChangedBy = &changedBy
		} else {
			v.DMStatusChangedBy = nil
		}
		return &v, nil
	}

	contact, contactType := resolveVacancyContact(v)
	if contact == "" {
		contact = "manual"
		contactType = "manual"
	}
	sentAt := time.Now().UTC()
	_, err = db.sql.ExecContext(ctx, `
UPDATE vacancies
SET dm_contact = $1, dm_contact_type = $2, dm_sent_at = $3, dm_status_changed_by = $4
WHERE id = $5
`, contact, contactType, sentAt, nullIfEmpty(changedBy), id)
	if err != nil {
		return nil, err
	}
	v.DMContact = &contact
	v.DMContactType = &contactType
	v.DMSentAt = &sentAt
	if changedBy != "" {
		v.DMStatusChangedBy = &changedBy
	} else {
		v.DMStatusChangedBy = nil
	}
	return &v, nil
}

func resolveVacancyContact(v Vacancy) (contact, contactType string) {
	if v.DMContact != nil {
		c := strings.TrimSpace(*v.DMContact)
		if c != "" && c != "none" {
			t := "username"
			if v.DMContactType != nil && strings.TrimSpace(*v.DMContactType) != "" {
				t = strings.TrimSpace(*v.DMContactType)
			}
			return c, t
		}
	}
	if v.AdPhone != nil {
		if ph := strings.TrimSpace(*v.AdPhone); ph != "" {
			return ph, "phone"
		}
	}
	if v.AdUsername != nil {
		if u := strings.TrimSpace(*v.AdUsername); u != "" {
			return strings.TrimPrefix(u, "@"), "username"
		}
	}
	return "", ""
}
