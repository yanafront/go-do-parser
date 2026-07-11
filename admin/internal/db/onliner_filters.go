package db

import (
	"fmt"
	"strings"
)

type OnlinerListFilter struct {
	Search      string
	HasContact  string
	DateFrom    string
	DateTo      string
}

var onlinerSearchCols = []string{
	"title",
	"body",
	"COALESCE(poster_user_id, '')",
	"COALESCE(poster_username, '')",
	"COALESCE(phone, '')",
	"COALESCE(email, '')",
	"COALESCE(telegram, '')",
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
		parts = append(parts, `(
			COALESCE(phone, '') <> '' OR
			COALESCE(email, '') <> '' OR
			COALESCE(telegram, '') <> ''
		)`)
	case "no":
		parts = append(parts, `(
			COALESCE(phone, '') = '' AND
			COALESCE(email, '') = '' AND
			COALESCE(telegram, '') = ''
		)`)
	}

	dateFrom := strings.TrimSpace(f.DateFrom)
	if dateFrom != "" {
		parts = append(parts, fmt.Sprintf("parsed_at >= $%d::date", n))
		args = append(args, dateFrom)
		n++
	}

	dateTo := strings.TrimSpace(f.DateTo)
	if dateTo != "" {
		parts = append(parts, fmt.Sprintf("parsed_at < ($%d::date + interval '1 day')", n))
		args = append(args, dateTo)
		n++
	}

	where := ""
	if len(parts) > 0 {
		where = "WHERE " + strings.Join(parts, " AND ")
	}
	return where, args, n
}
