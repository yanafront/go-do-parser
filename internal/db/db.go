package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	sql *sql.DB
}

func Open(databaseURL string) (*DB, error) {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{sql: sqlDB}
	if err := db.Migrate(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
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
