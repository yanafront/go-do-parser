package db

import (
	"context"
	"time"
)

type Vacancy struct {
	ID              int64      `json:"id"`
	SourceChannel   string     `json:"source_channel"`
	SourceMessageID int        `json:"source_message_id"`
	DestMessageID   *int       `json:"dest_message_id,omitempty"`
	Body            string     `json:"body"`
	AdUsername      *string    `json:"ad_username,omitempty"`
	AdPhone         *string    `json:"ad_phone,omitempty"`
	DMContact       *string    `json:"dm_contact,omitempty"`
	DMContactType   *string    `json:"dm_contact_type,omitempty"`
	DMSentAt        *time.Time `json:"dm_sent_at,omitempty"`
	PublishedAt     time.Time  `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type JobSeekerPost struct {
	ID              int64      `json:"id"`
	SourceChannel   string     `json:"source_channel"`
	SourceMessageID int        `json:"source_message_id"`
	Body            string     `json:"body"`
	PosterUsername  *string    `json:"poster_username,omitempty"`
	AdUsername      *string    `json:"ad_username,omitempty"`
	AdPhone         *string    `json:"ad_phone,omitempty"`
	DMContact       *string    `json:"dm_contact,omitempty"`
	DMContactType   *string    `json:"dm_contact_type,omitempty"`
	DMSentAt        *time.Time `json:"dm_sent_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Stats struct {
	Vacancies   int64 `json:"vacancies"`
	JobSeekers  int64 `json:"job_seekers"`
	DMSent      int64 `json:"dm_sent"`
}

func (db *DB) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM vacancies`).Scan(&s.Vacancies); err != nil {
		return s, err
	}
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_seeker_posts`).Scan(&s.JobSeekers); err != nil {
		return s, err
	}
	if err := db.sql.QueryRowContext(ctx, `
SELECT
  (SELECT COUNT(*) FROM vacancies WHERE dm_contact IS NOT NULL) +
  (SELECT COUNT(*) FROM job_seeker_posts WHERE dm_contact IS NOT NULL)
`).Scan(&s.DMSent); err != nil {
		return s, err
	}
	return s, nil
}

func (db *DB) ListVacancies(ctx context.Context, limit, offset int) ([]Vacancy, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM vacancies`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.sql.QueryContext(ctx, `
SELECT id, source_channel, source_message_id, dest_message_id, body,
       ad_username, ad_phone, dm_contact, dm_contact_type, dm_sent_at,
       published_at, created_at
FROM vacancies
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Vacancy
	for rows.Next() {
		var v Vacancy
		if err := rows.Scan(
			&v.ID, &v.SourceChannel, &v.SourceMessageID, &v.DestMessageID, &v.Body,
			&v.AdUsername, &v.AdPhone, &v.DMContact, &v.DMContactType, &v.DMSentAt,
			&v.PublishedAt, &v.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (db *DB) ListJobSeekers(ctx context.Context, limit, offset int) ([]JobSeekerPost, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_seeker_posts`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.sql.QueryContext(ctx, `
SELECT id, source_channel, source_message_id, body,
       poster_username, ad_username, ad_phone, dm_contact, dm_contact_type, dm_sent_at, created_at
FROM job_seeker_posts
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []JobSeekerPost
	for rows.Next() {
		var p JobSeekerPost
		if err := rows.Scan(
			&p.ID, &p.SourceChannel, &p.SourceMessageID, &p.Body,
			&p.PosterUsername, &p.AdUsername, &p.AdPhone, &p.DMContact, &p.DMContactType, &p.DMSentAt, &p.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
