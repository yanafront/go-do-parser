package db

import (
	"context"
	"fmt"
	"strings"
)

type ListFilter struct {
	Search        string
	Channel       string
	HasDM         string
	MessageStatus string
	DateFrom      string
	DateTo        string
}

var vacancySearchCols = []string{
	"body",
	"source_channel",
	"COALESCE(source_message_link, '')",
	"COALESCE(ad_username, '')",
	"COALESCE(ad_phone, '')",
	"COALESCE(dm_contact, '')",
}

var jobSeekerSearchCols = []string{
	"body",
	"source_channel",
	"COALESCE(source_message_link, '')",
	"COALESCE(poster_username, '')",
	"COALESCE(poster_phone, '')",
	"COALESCE(ad_username, '')",
	"COALESCE(ad_phone, '')",
	"COALESCE(dm_contact, '')",
}

func (f ListFilter) vacancyWhere(startArg int) (string, []any, int) {
	return f.buildWhere(startArg, vacancySearchCols, vacancyPendingContactSQL)
}

func (f ListFilter) jobSeekerWhere(startArg int) (string, []any, int) {
	return f.buildWhere(startArg, jobSeekerSearchCols, jobSeekerPendingContactSQL)
}

const vacancyPendingContactSQL = `(
	COALESCE(ad_phone, '') <> '' OR
	COALESCE(ad_username, '') <> '' OR
	body ~* '(\\+?\\s*375|8[\\s\\-]*0\\d{2})'
)`

const jobSeekerPendingContactSQL = `(
	COALESCE(ad_username, '') <> '' OR
	COALESCE(ad_phone, '') <> '' OR
	COALESCE(poster_username, '') <> '' OR
	COALESCE(poster_phone, '') <> ''
)`

func (f ListFilter) messageStatus() string {
	if v := strings.TrimSpace(f.MessageStatus); v != "" {
		return v
	}
	switch strings.TrimSpace(f.HasDM) {
	case "yes":
		return "sent"
	case "no":
		return "pending"
	default:
		return strings.TrimSpace(f.HasDM)
	}
}

func (f ListFilter) buildWhere(startArg int, searchCols []string, pendingSQL string) (string, []any, int) {
	var parts []string
	var args []any
	n := startArg

	search := normalizeSearch(f.Search)
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		var conds []string
		for _, col := range searchCols {
			conds = append(conds, fmt.Sprintf("LOWER(%s) LIKE $%d", col, n))
		}
		parts = append(parts, "("+strings.Join(conds, " OR ")+")")
		args = append(args, pattern)
		n++
	}

	channel := normalizeSearch(f.Channel)
	if channel != "" {
		parts = append(parts, fmt.Sprintf("LOWER(source_channel) = LOWER($%d)", n))
		args = append(args, channel)
		n++
	}

	switch strings.TrimSpace(f.messageStatus()) {
	case "sent":
		parts = append(parts, "dm_contact IS NOT NULL AND dm_contact <> '' AND dm_contact <> 'none'")
	case "pending":
		parts = append(parts, "(dm_contact IS NULL OR dm_contact = '')")
		parts = append(parts, "("+pendingSQL+")")
	case "no_contact":
		parts = append(parts, "(dm_contact IS NULL OR dm_contact = '')")
		parts = append(parts, "NOT ("+pendingSQL+")")
	case "skipped":
		parts = append(parts, "dm_contact = 'none'")
	}

	dateFrom := strings.TrimSpace(f.DateFrom)
	if dateFrom != "" {
		parts = append(parts, fmt.Sprintf("created_at >= $%d::date", n))
		args = append(args, dateFrom)
		n++
	}

	dateTo := strings.TrimSpace(f.DateTo)
	if dateTo != "" {
		parts = append(parts, fmt.Sprintf("created_at < ($%d::date + interval '1 day')", n))
		args = append(args, dateTo)
		n++
	}

	where := ""
	if len(parts) > 0 {
		where = "WHERE " + strings.Join(parts, " AND ")
	}
	return where, args, n
}

func (db *DB) ListVacancyChannels(ctx context.Context) ([]string, error) {
	return db.listChannels(ctx, "vacancies")
}

func (db *DB) ListJobSeekerChannels(ctx context.Context) ([]string, error) {
	return db.listChannels(ctx, "job_seeker_posts")
}

func (db *DB) listChannels(ctx context.Context, table string) ([]string, error) {
	query := fmt.Sprintf(`
SELECT DISTINCT source_channel
FROM %s
WHERE source_channel <> ''
ORDER BY source_channel
`, table)
	rows, err := db.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func normalizeSearch(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimPrefix(s, "https://t.me/")
	s = strings.TrimPrefix(s, "t.me/")
	return s
}
