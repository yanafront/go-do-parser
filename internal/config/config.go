package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	App      AppConfig      `yaml:"app"`
}

type TelegramConfig struct {
	APIID       int      `yaml:"api_id"`
	APIHash     string   `yaml:"api_hash"`
	Phone       string   `yaml:"phone"`
	BotToken    string   `yaml:"bot_token"`
	Destination string   `yaml:"destination"`
	Sources     []string `yaml:"sources"`
}

type AppConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
	DataDir      string        `yaml:"data_dir"`
	BatchSize    int           `yaml:"batch_size"`
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
