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

	var total int64
	countQuery := "SELECT COUNT(*) FROM onliner_posts " + where
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := fmt.Sprintf(`
SELECT id, topic_id, topic_url, title, body,
       poster_user_id, poster_username, poster_profile_url,
       phone, email, telegram, created_at, parsed_at, posted_at
FROM onliner_posts
%s
ORDER BY COALESCE(posted_at, parsed_at) DESC, id DESC
LIMIT $%d OFFSET $%d
`, where, nextArg, nextArg+1)
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
		); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
