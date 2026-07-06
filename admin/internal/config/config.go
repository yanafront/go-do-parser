package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port          string
	DatabaseURL   string
	AdminPassword string
	JWTSecret     string
}

func Load() (Config, error) {
	cfg := Config{
		Port:          envOr("PORT", "8080"),
		DatabaseURL:   resolveDatabaseURL(),
		AdminPassword: strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")),
		JWTSecret:     strings.TrimSpace(os.Getenv("ADMIN_JWT_SECRET")),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AdminPassword == "" {
		return cfg, fmt.Errorf("ADMIN_PASSWORD is required")
	}
	if cfg.JWTSecret == "" {
		return cfg, fmt.Errorf("ADMIN_JWT_SECRET is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func resolveDatabaseURL() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_PRIVATE_URL")); v != "" {
		return normalizeURL(v)
	}
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return normalizeURL(v)
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
	return normalizeURL(u.String())
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "sslmode=") {
		return raw
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	if strings.Contains(raw, "railway.internal") {
		return raw + sep + "sslmode=disable"
	}
	return raw + sep + "sslmode=require"
}
