package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	sql *sql.DB
}

func ResolveURL() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_PRIVATE_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return v
	}
	host := strings.TrimSpace(os.Getenv("PGHOST"))
	user := strings.TrimSpace(os.Getenv("PGUSER"))
	password := os.Getenv("PGPASSWORD")
	dbname := strings.TrimSpace(os.Getenv("PGDATABASE"))
	port := strings.TrimSpace(os.Getenv("PGPORT"))
	if host == "" || user == "" {
		return ""
	}
	if dbname == "" {
		dbname = "railway"
	}
	if port == "" {
		port = "5432"
	}
	u := &url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   dbname,
	}
	return u.String()
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
			lastErr = fmt.Errorf("%s: %w", MaskURL(connURL), err)
			continue
		}
		sqlDB.SetMaxOpenConns(5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)

		if err := pingWithRetry(sqlDB, 3); err != nil {
			lastErr = fmt.Errorf("%s: %w", MaskURL(connURL), err)
			sqlDB.Close()
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		db := &DB{sql: sqlDB}
		err = db.Migrate(ctx)
		cancel()
		if err != nil {
			sqlDB.Close()
			lastErr = err
			continue
		}
		return db, nil
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
		add(withQueryParam(raw, "sslmode", "disable"))
		add(withQueryParam(raw, "sslmode", "require"))
		add(withQueryParam(raw, "sslmode", "prefer"))
	}
	return out
}

func withQueryParam(raw, key, value string) string {
	if strings.Contains(raw, key+"=") {
		return raw
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + key + "=" + value
}

func MaskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "invalid database url"
	}
	if u.User != nil {
		if name := u.User.Username(); name != "" {
			u.User = url.UserPassword(name, "***")
		}
	}
	return u.Redacted()
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
    source_message_link TEXT,
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
    source_message_link TEXT,
    body TEXT NOT NULL,
    poster_username TEXT,
    poster_phone TEXT,
    ad_username TEXT,
    ad_phone TEXT,
    dm_contact TEXT,
    dm_contact_type TEXT,
    dm_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_channel, source_message_id)
);

ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS source_message_link TEXT;
ALTER TABLE job_seeker_posts ADD COLUMN IF NOT EXISTS source_message_link TEXT;
ALTER TABLE job_seeker_posts ADD COLUMN IF NOT EXISTS poster_phone TEXT;

CREATE TABLE IF NOT EXISTS onliner_posts (
    id BIGSERIAL PRIMARY KEY,
    topic_id INTEGER NOT NULL UNIQUE,
    topic_url TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL,
    poster_user_id TEXT,
    poster_username TEXT,
    poster_profile_url TEXT,
    phone TEXT,
    email TEXT,
    telegram TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    parsed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
