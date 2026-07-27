package db

import (
	"context"
	"fmt"
	"time"
)

type Vacancy struct {
	ID                 int64      `json:"id"`
	SourceChannel      string     `json:"source_channel"`
	SourceChannelTitle string     `json:"source_channel_title,omitempty"`
	SourceMessageID    int        `json:"source_message_id"`
	SourceMessageLink  *string    `json:"source_message_link,omitempty"`
	DestMessageID      *int       `json:"dest_message_id,omitempty"`
	Body               string     `json:"body"`
	AdUsername         *string    `json:"ad_username,omitempty"`
	AdPhone            *string    `json:"ad_phone,omitempty"`
	DMContact          *string    `json:"dm_contact,omitempty"`
	DMContactType      *string    `json:"dm_contact_type,omitempty"`
	DMSentAt           *time.Time `json:"dm_sent_at,omitempty"`
	PublishedAt        time.Time  `json:"published_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

type JobSeekerPost struct {
	ID                 int64      `json:"id"`
	SourceChannel      string     `json:"source_channel"`
	SourceChannelTitle string     `json:"source_channel_title,omitempty"`
	SourceMessageID    int        `json:"source_message_id"`
	SourceMessageLink  *string    `json:"source_message_link,omitempty"`
	Body               string     `json:"body"`
	PosterUsername     *string    `json:"poster_username,omitempty"`
	PosterPhone        *string    `json:"poster_phone,omitempty"`
	AdUsername         *string    `json:"ad_username,omitempty"`
	AdPhone            *string    `json:"ad_phone,omitempty"`
	DMContact          *string    `json:"dm_contact,omitempty"`
	DMContactType      *string    `json:"dm_contact_type,omitempty"`
	DMSentAt           *time.Time `json:"dm_sent_at,omitempty"`
	DMMessage          *string    `json:"dm_message,omitempty"`
	DMStatusChangedBy  *string    `json:"dm_status_changed_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type Stats struct {
	Vacancies  int64 `json:"vacancies"`
	JobSeekers int64 `json:"job_seekers"`
	Onliner    int64 `json:"onliner"`
	DMSent     int64 `json:"dm_sent"`
}

type OnlinerPost struct {
	ID               int64      `json:"id"`
	TopicID          int        `json:"topic_id"`
	TopicURL         string     `json:"topic_url"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	PosterUserID     *string    `json:"poster_user_id,omitempty"`
	PosterUsername   *string    `json:"poster_username,omitempty"`
	PosterProfileURL *string    `json:"poster_profile_url,omitempty"`
	Phone            *string    `json:"phone,omitempty"`
	Email            *string    `json:"email,omitempty"`
	Telegram         *string    `json:"telegram,omitempty"`
	DMContact        *string    `json:"dm_contact,omitempty"`
	DMContactType    *string    `json:"dm_contact_type,omitempty"`
	DMSentAt         *time.Time `json:"dm_sent_at,omitempty"`
	DMStatusChangedBy *string   `json:"dm_status_changed_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	ParsedAt         time.Time  `json:"parsed_at"`
	PostedAt         *time.Time `json:"posted_at,omitempty"`
}

func (db *DB) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM vacancies`).Scan(&s.Vacancies); err != nil {
		return s, err
	}
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_seeker_posts WHERE source_channel NOT LIKE 'onliner:%'`).Scan(&s.JobSeekers); err != nil {
		return s, err
	}
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM onliner_posts`).Scan(&s.Onliner); err != nil {
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

func (db *DB) ListVacancies(ctx context.Context, filter ListFilter, limit, offset int) ([]Vacancy, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where, args, nextArg := filter.vacancyWhere(1)

	var total int64
	countQuery := "SELECT COUNT(*) FROM vacancies " + where
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := fmt.Sprintf(`
SELECT vacancies.id, vacancies.source_channel, COALESCE(c.title, ''),
       vacancies.source_message_id, vacancies.source_message_link, vacancies.dest_message_id, vacancies.body,
       vacancies.ad_username, vacancies.ad_phone, vacancies.dm_contact, vacancies.dm_contact_type, vacancies.dm_sent_at,
       vacancies.published_at, vacancies.created_at
FROM vacancies
LEFT JOIN channels c ON c.username = LOWER(vacancies.source_channel)
%s
%s
LIMIT $%d OFFSET $%d
`, where, filter.OrderByPublished("id"), nextArg, nextArg+1)
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.sql.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Vacancy
	for rows.Next() {
		var v Vacancy
		if err := rows.Scan(
			&v.ID, &v.SourceChannel, &v.SourceChannelTitle, &v.SourceMessageID, &v.SourceMessageLink, &v.DestMessageID, &v.Body,
			&v.AdUsername, &v.AdPhone, &v.DMContact, &v.DMContactType, &v.DMSentAt,
			&v.PublishedAt, &v.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (db *DB) ListJobSeekers(ctx context.Context, filter ListFilter, limit, offset int) ([]JobSeekerPost, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where, args, nextArg := filter.jobSeekerWhere(1)
	if where == "" {
		where = "WHERE source_channel NOT LIKE 'onliner:%'"
	} else {
		where += " AND source_channel NOT LIKE 'onliner:%'"
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM job_seeker_posts WHERE source_channel NOT LIKE 'onliner:%'"
	if where != "" {
		countQuery = "SELECT COUNT(*) FROM job_seeker_posts " + where
	}
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := fmt.Sprintf(`
SELECT job_seeker_posts.id, job_seeker_posts.source_channel, COALESCE(c.title, ''),
       job_seeker_posts.source_message_id, job_seeker_posts.source_message_link, job_seeker_posts.body,
       job_seeker_posts.poster_username, job_seeker_posts.poster_phone, job_seeker_posts.ad_username, job_seeker_posts.ad_phone,
       job_seeker_posts.dm_contact, job_seeker_posts.dm_contact_type, job_seeker_posts.dm_sent_at,
       job_seeker_posts.dm_message, job_seeker_posts.dm_status_changed_by, job_seeker_posts.created_at
FROM job_seeker_posts
LEFT JOIN channels c ON c.username = LOWER(job_seeker_posts.source_channel)
%s
%s
LIMIT $%d OFFSET $%d
`, where, filter.OrderByCreated("id"), nextArg, nextArg+1)
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.sql.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []JobSeekerPost
	for rows.Next() {
		var p JobSeekerPost
		if err := rows.Scan(
			&p.ID, &p.SourceChannel, &p.SourceChannelTitle, &p.SourceMessageID, &p.SourceMessageLink, &p.Body,
			&p.PosterUsername, &p.PosterPhone, &p.AdUsername, &p.AdPhone, &p.DMContact, &p.DMContactType, &p.DMSentAt, &p.DMMessage, &p.DMStatusChangedBy, &p.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
