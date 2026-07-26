package db

import (
	"fmt"
	"strings"
)

type OnlinerListFilter struct {
	Search        string
	HasContact    string
	MessageStatus string
	DateFrom      string
	DateTo        string
	SortBy        string
	SortDir       string
}

var onlinerSearchCols = []string{
	"o.title",
	"o.body",
	"COALESCE(o.poster_user_id, '')",
	"COALESCE(o.poster_username, '')",
	"COALESCE(o.phone, '')",
	"COALESCE(o.email, '')",
	"COALESCE(o.telegram, '')",
	"COALESCE(j.dm_contact, '')",
}

const onlinerPendingContactSQL = `(
	COALESCE(o.phone, '') <> '' OR
	COALESCE(o.telegram, '') <> ''
)`

func (f OnlinerListFilter) messageStatus() string {
	return strings.TrimSpace(f.MessageStatus)
}

func (f OnlinerListFilter) fromClause() string {
	return `FROM onliner_posts o
LEFT JOIN job_seeker_posts j ON j.source_channel LIKE 'onliner:%' AND j.source_message_id = o.topic_id`
}

func (f OnlinerListFilter) where(startArg int) (string, []any, int) {
	var parts []string
	var args []any
	n := startArg

	search := normalizeSearch(f.Search)
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		var conds []string
		for _, col := range onlinerSearchCols {
			conds = append(conds, fmt.Sprintf("LOWER(%s) LIKE $%d", col, n))
		}
		parts = append(parts, "("+strings.Join(conds, " OR ")+")")
		args = append(args, pattern)
		n++
	}

	switch strings.TrimSpace(f.HasContact) {
	case "yes":
		parts = append(parts, onlinerPendingContactSQL)
	case "no":
		parts = append(parts, "NOT "+onlinerPendingContactSQL)
	}

	switch strings.TrimSpace(f.messageStatus()) {
	case "sent":
		parts = append(parts, "j.dm_contact IS NOT NULL AND j.dm_contact <> '' AND j.dm_contact <> 'none'")
	case "pending":
		parts = append(parts, "(j.dm_contact IS NULL OR j.dm_contact = '')")
		parts = append(parts, onlinerPendingContactSQL)
	case "no_contact":
		parts = append(parts, "(j.dm_contact IS NULL OR j.dm_contact = '')")
		parts = append(parts, "NOT "+onlinerPendingContactSQL)
	case "skipped":
		parts = append(parts, "j.dm_contact = 'none'")
	}

	dateFrom := strings.TrimSpace(f.DateFrom)
	if dateFrom != "" {
		parts = append(parts, fmt.Sprintf("o.parsed_at >= $%d::date", n))
		args = append(args, dateFrom)
		n++
	}

	dateTo := strings.TrimSpace(f.DateTo)
	if dateTo != "" {
		parts = append(parts, fmt.Sprintf("o.parsed_at < ($%d::date + interval '1 day')", n))
		args = append(args, dateTo)
		n++
	}

	where := ""
	if len(parts) > 0 {
		where = "WHERE " + strings.Join(parts, " AND ")
	}
	return where, args, n
}

func (f OnlinerListFilter) SortAscending() bool {
	return strings.EqualFold(strings.TrimSpace(f.SortDir), "asc")
}

func (f OnlinerListFilter) OrderByDate() string {
	dir := "DESC"
	nulls := "NULLS LAST"
	if f.SortAscending() {
		dir = "ASC"
		nulls = "NULLS FIRST"
	}
	by := strings.ToLower(strings.TrimSpace(f.SortBy))
	if by == "sent" || by == "dm_sent_at" {
		return fmt.Sprintf("ORDER BY j.dm_sent_at %s %s, o.id %s", dir, nulls, dir)
	}
	return fmt.Sprintf("ORDER BY COALESCE(o.posted_at, o.parsed_at) %s, o.id %s", dir, dir)
}
