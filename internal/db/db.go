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
	databaseURL = normalizeDatabaseURL(databaseURL)

	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		lastErr = sqlDB.PingContext(ctx)
		cancel()
		if lastErr == nil {
			break
		}
		if attempt < 5 {
			time.Sleep(3 * time.Second)
		}
	}
	if lastErr != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", lastErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db := &DB{sql: sqlDB}
	if err := db.Migrate(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func normalizeDatabaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.Contains(raw, "sslmode=") {
		return raw
	}
	if strings.Contains(raw, "?") {
		return raw + "&sslmode=require"
	}
	return raw + "?sslmode=require"
}

func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	return db.sql.Close()
}

func (db *DB) Migrate(ctx context.Context) error {
	_, err := db.sql.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS vacancies (
    id BIGSERIAL PRIMARY KEY,
    source_channel TEXT NOT NULL,
    source_message_id INTEGER NOT NULL,
    dest_message_id INTEGER,
    body TEXT NOT NULL,
    ad_username TEXT,
    ad_phone TEXT,
    dm_contact TEXT,
    dm_contact_type TEXT,
    dm_sent_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_channel, source_message_id)
);

CREATE TABLE IF NOT EXISTS job_seeker_posts (
    id BIGSERIAL PRIMARY KEY,
    source_channel TEXT NOT NULL,
    source_message_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    poster_username TEXT,
    ad_username TEXT,
    ad_phone TEXT,
    dm_contact TEXT,
    dm_contact_type TEXT,
    dm_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_channel, source_message_id)
);
`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
