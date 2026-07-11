package onliner

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type RuntimeConfig struct {
	DataDir       string
	PollInterval  time.Duration
	ForumID       int
	ForumPages    int
	SearchPages   int
	SearchQueries []string
	RequestDelay  time.Duration
	DatabaseURL   string
}

func LoadRuntimeConfig() RuntimeConfig {
	cfg := RuntimeConfig{
		DataDir:      envOr("ONLINER_DATA_DIR", "./data/onliner"),
		PollInterval: 10 * time.Minute,
		ForumID:      34,
		ForumPages:   2,
		SearchPages:  1,
		RequestDelay: 400 * time.Millisecond,
		DatabaseURL:  resolveDatabaseURL(),
	}
	if v := strings.TrimSpace(os.Getenv("ONLINER_POLL_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONLINER_FORUM_PAGES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ForumPages = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONLINER_SEARCH_PAGES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SearchPages = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONLINER_FORUM_ID")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ForumID = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONLINER_REQUEST_DELAY")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RequestDelay = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONLINER_SEARCH_QUERIES")); v != "" {
		cfg.SearchQueries = splitComma(v)
	} else {
		cfg.SearchQueries = []string{"ищу подработку", "ищу работу"}
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitComma(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func resolveDatabaseURL() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_PRIVATE_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return v
	}
	host := strings.TrimSpace(os.Getenv("PGHOST"))
	user := strings.TrimSpace(os.Getenv("PGUSER"))
	if host == "" || user == "" {
		return ""
	}
	password := os.Getenv("PGPASSWORD")
	dbname := strings.TrimSpace(os.Getenv("PGDATABASE"))
	if dbname == "" {
		dbname = "railway"
	}
	port := strings.TrimSpace(os.Getenv("PGPORT"))
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
