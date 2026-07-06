package db

import (
	"context"
	"fmt"
	"strings"
)

type ListFilter struct {
	Search   string
	Channel  string
	HasDM    string
	DateFrom string
	DateTo   string
}

func (f ListFilter) vacancyWhere(startArg int) (string, []any, int) {
	return f.buildWhere(startArg, vacancySearchCols)
}

func (f ListFilter) jobSeekerWhere(startArg int) (string, []any, int) {
	return f.buildWhere(startArg, jobSeekerSearchCols)
}

var vacancySearchCols = []string{
	"body",
	"source_channel",
	"COALESCE(ad_username, '')",
	"COALESCE(ad_phone, '')",
	"COALESCE(dm_contact, '')",
}

var jobSeekerSearchCols = []string{
	"body",
	"source_channel",
	"COALESCE(poster_username, '')",
	"COALESCE(ad_username, '')",
	"COALESCE(ad_phone, '')",
	"COALESCE(dm_contact, '')",
}

func (f ListFilter) buildWhere(startArg int, searchCols []string) (string, []any, int) {
	var parts []string
	var args []any
	n := startArg

	search := strings.TrimSpace(f.Search)
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

	channel := strings.TrimSpace(strings.TrimPrefix(f.Channel, "@"))
	if channel != "" {
		parts = append(parts, fmt.Sprintf("source_channel = $%d", n))
		args = append(args, channel)
		n++
	}

	switch strings.TrimSpace(f.HasDM) {
	case "yes":
		parts = append(parts, "dm_contact IS NOT NULL AND dm_contact <> ''")
	case "no":
		parts = append(parts, "(dm_contact IS NULL OR dm_contact = '')")
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
