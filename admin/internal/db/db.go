package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	sql *sql.DB
}

func Open(databaseURL string) (*DB, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	var lastErr error
	for _, connURL := range connectionVariants(databaseURL) {
		sqlDB, err := sql.Open("pgx", connURL)
		if err != nil {
			lastErr = err
			continue
		}
		sqlDB.SetMaxOpenConns(5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)

		if err := pingWithRetry(sqlDB, 3); err != nil {
			lastErr = err
			sqlDB.Close()
			continue
		}
		return &DB{sql: sqlDB}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no connection variants")
	}
	return nil, fmt.Errorf("ping database: %w", lastErr)
}

func pingWithRetry(sqlDB *sql.DB, attempts int) error {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		lastErr = sqlDB.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if attempt < attempts {
			time.Sleep(2 * time.Second)
		}
	}
	return lastErr
}

func connectionVariants(raw string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(raw)
	if !strings.Contains(raw, "sslmode=") {
		sep := "?"
		if strings.Contains(raw, "?") {
			sep = "&"
		}
		if strings.Contains(raw, "railway.internal") {
			add(raw + sep + "sslmode=disable")
		}
		add(raw + sep + "sslmode=require")
		add(raw + sep + "sslmode=prefer")
	}
	return out
}

func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	return db.sql.Close()
}
