package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Database DatabaseConfig `yaml:"database"`
	Outreach OutreachConfig `yaml:"outreach"`
	Seeker   SeekerConfig   `yaml:"seeker"`
	App      AppConfig      `yaml:"app"`
}

type TelegramConfig struct {
	APIID       int      `yaml:"api_id"`
	APIHash     string   `yaml:"api_hash"`
	Phone       string   `yaml:"phone"`
	BotToken    string   `yaml:"bot_token"`
	Destination string   `yaml:"destination"`
	Sources     []string `yaml:"sources"`
	MatcherBot  string   `yaml:"matcher_bot"`
	PlatformURL string   `yaml:"platform_url"`
	MatcherURL  string   `yaml:"matcher_url"`
	IngestSecret string  `yaml:"ingest_secret"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

func (c DatabaseConfig) Enabled() bool {
	return strings.TrimSpace(c.URL) != ""
}

type OutreachConfig struct {
	Phone             string
	Session           string
	Message           string
	DailyLimit        int
	Delay             time.Duration
	DataDir           string
	ExplicitlyEnabled bool
}

func (c OutreachConfig) Enabled() bool {
	if !c.ExplicitlyEnabled {
		return false
	}
	return strings.TrimSpace(c.Phone) != "" && strings.TrimSpace(c.Message) != ""
}

type SeekerConfig struct {
	Message           string
	DailyLimit        int
	Delay             time.Duration
	DataDir           string
	ExplicitlyEnabled bool
}

func (c SeekerConfig) Enabled() bool {
	if !c.ExplicitlyEnabled {
		return false
	}
	return strings.TrimSpace(c.Message) != ""
}

func (c *Config) MessengerEnabled() bool {
	return strings.TrimSpace(c.Outreach.Phone) != "" && (c.Outreach.Enabled() || c.Seeker.Enabled())
}

type AppConfig struct {
	PollInterval  time.Duration `yaml:"poll_interval"`
	DataDir       string        `yaml:"data_dir"`
	BatchSize     int           `yaml:"batch_size"`
	PromoEvery    int           `yaml:"promo_every"`
	PlatformEvery int           `yaml:"platform_every"`
}

func Load(path string) (*Config, error) {
	var cfg Config

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parse config: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg.applyEnv()
	cfg.setDefaults()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyEnv() {
	if v := os.Getenv("TG_API_ID"); v != "" {
		var id int
		if _, err := fmt.Sscanf(v, "%d", &id); err == nil {
			c.Telegram.APIID = id
		}
	}
	if v := os.Getenv("TG_API_HASH"); v != "" {
		c.Telegram.APIHash = v
	}
	if v := os.Getenv("TG_PHONE"); v != "" {
		c.Telegram.Phone = v
	}
	if v := os.Getenv("TG_BOT_TOKEN"); v != "" {
		c.Telegram.BotToken = v
	}
	if v := os.Getenv("TG_DESTINATION"); v != "" {
		c.Telegram.Destination = v
	}
	if v := os.Getenv("TG_SOURCES"); v != "" {
		c.Telegram.Sources = splitSources(v)
	}
	if v := os.Getenv("MATCHER_BOT"); v != "" {
		c.Telegram.MatcherBot = v
	}
	if v := os.Getenv("PLATFORM_URL"); v != "" {
		c.Telegram.PlatformURL = v
	}
	if v := os.Getenv("MATCHER_URL"); v != "" {
		c.Telegram.MatcherURL = v
	}
	if v := os.Getenv("INGEST_SECRET"); v != "" {
		c.Telegram.IngestSecret = v
	}
	if v := os.Getenv("DATABASE_PRIVATE_URL"); v != "" {
		c.Database.URL = v
	} else if v := os.Getenv("DATABASE_URL"); v != "" {
		c.Database.URL = v
	}
	if c.Database.URL == "" {
		c.Database.URL = postgresURLFromEnv()
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		c.App.DataDir = v
	}
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.App.PollInterval = d
		}
	}
	if v := os.Getenv("BATCH_SIZE"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			c.App.BatchSize = n
		}
	}
	if v := os.Getenv("PROMO_EVERY"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			c.App.PromoEvery = n
		}
	}
	if v := os.Getenv("PLATFORM_EVERY"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			c.App.PlatformEvery = n
		}
	}
	if v := os.Getenv("OUTREACH_PHONE"); v != "" {
		c.Outreach.Phone = v
	}
	if v := os.Getenv("OUTREACH_SESSION"); v != "" {
		c.Outreach.Session = v
	}
	if v := os.Getenv("OUTREACH_MESSAGE"); v != "" {
		c.Outreach.Message = unescapeEnv(v)
	}
	if v := os.Getenv("OUTREACH_DATA_DIR"); v != "" {
		c.Outreach.DataDir = v
	}
	if v := os.Getenv("OUTREACH_DAILY_LIMIT"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			c.Outreach.DailyLimit = n
		}
	}
	if v := os.Getenv("OUTREACH_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Outreach.Delay = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("OUTREACH_ENABLED")); v == "true" || v == "1" || strings.EqualFold(v, "yes") {
		c.Outreach.ExplicitlyEnabled = true
	}
	if v := os.Getenv("SEEKER_MESSAGE"); v != "" {
		c.Seeker.Message = unescapeEnv(v)
	}
	if v := os.Getenv("SEEKER_DATA_DIR"); v != "" {
		c.Seeker.DataDir = v
	}
	if v := os.Getenv("SEEKER_DAILY_LIMIT"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			c.Seeker.DailyLimit = n
		}
	}
	if v := os.Getenv("SEEKER_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Seeker.Delay = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("SEEKER_ENABLED")); v == "true" || v == "1" || strings.EqualFold(v, "yes") {
		c.Seeker.ExplicitlyEnabled = true
	}
}

func (c *Config) setDefaults() {
	if c.App.PollInterval == 0 {
		c.App.PollInterval = 2 * time.Minute
	}
	if c.App.DataDir == "" {
		c.App.DataDir = "./data"
	}
	if c.App.BatchSize == 0 {
		c.App.BatchSize = 50
	}
	if c.App.PromoEvery == 0 {
		c.App.PromoEvery = 10
	}
	if c.App.PlatformEvery == 0 {
		c.App.PlatformEvery = 5
	}
	if c.Telegram.PlatformURL == "" {
		c.Telegram.PlatformURL = "https://platform.alcan.by/"
	}
	if c.Outreach.DataDir == "" {
		c.Outreach.DataDir = c.App.DataDir + "/outreach"
	}
	if c.Outreach.DailyLimit == 0 {
		c.Outreach.DailyLimit = 5
	}
	if c.Outreach.Delay == 0 {
		c.Outreach.Delay = 10 * time.Minute
	}
	if c.Seeker.DataDir == "" {
		c.Seeker.DataDir = c.App.DataDir + "/seeker"
	}
	if c.Seeker.DailyLimit == 0 {
		c.Seeker.DailyLimit = 5
	}
	if c.Seeker.Delay == 0 {
		c.Seeker.Delay = 10 * time.Minute
	}
}

func (c *Config) validate() error {
	if c.Telegram.APIID == 0 {
		return fmt.Errorf("TG_API_ID is required")
	}
	if c.Telegram.APIHash == "" {
		return fmt.Errorf("TG_API_HASH is required")
	}
	if c.Telegram.Phone == "" {
		return fmt.Errorf("TG_PHONE is required")
	}
	if c.Telegram.BotToken == "" {
		return fmt.Errorf("TG_BOT_TOKEN is required")
	}
	if c.Telegram.Destination == "" {
		return fmt.Errorf("TG_DESTINATION is required")
	}
	if len(c.Telegram.Sources) == 0 {
		return fmt.Errorf("TG_SOURCES must not be empty")
	}
	return nil
}

func splitSources(raw string) []string {
	var out []string
	for _, part := range splitComma(raw) {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func unescapeEnv(s string) string {
	return strings.NewReplacer("\\n", "\n", "\\t", "\t").Replace(s)
}

func postgresURLFromEnv() string {
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
