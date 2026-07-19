package db

import (
	"context"
	"fmt"
)

func (db *DB) ListOnlinerPosts(ctx context.Context, filter OnlinerListFilter, limit, offset int) ([]OnlinerPost, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where, args, nextArg := filter.where(1)
	from := filter.fromClause()

	var total int64
	countQuery := "SELECT COUNT(*) " + from + " " + where
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := fmt.Sprintf(`
SELECT o.id, o.topic_id, o.topic_url, o.title, o.body,
       o.poster_user_id, o.poster_username, o.poster_profile_url,
       o.phone, o.email, o.telegram, o.created_at, o.parsed_at, o.posted_at,
       j.dm_contact, j.dm_contact_type, j.dm_sent_at
%s
%s
ORDER BY COALESCE(o.posted_at, o.parsed_at) DESC, o.id DESC
LIMIT $%d OFFSET $%d
`, from, where, nextArg, nextArg+1)
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.sql.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []OnlinerPost
	for rows.Next() {
		var p OnlinerPost
		if err := rows.Scan(
			&p.ID, &p.TopicID, &p.TopicURL, &p.Title, &p.Body,
			&p.PosterUserID, &p.PosterUsername, &p.PosterProfileURL,
			&p.Phone, &p.Email, &p.Telegram, &p.CreatedAt, &p.ParsedAt, &p.PostedAt,
			&p.DMContact, &p.DMContactType, &p.DMSentAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
