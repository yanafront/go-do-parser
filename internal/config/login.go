package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type LoginConfig struct {
	APIID   int
	APIHash string
	Phone   string
	DataDir string
}

func LoadLogin(path string) (*LoginConfig, error) {
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
	if cfg.Telegram.APIID == 0 {
		return nil, fmt.Errorf("TG_API_ID is required")
	}
	if cfg.Telegram.APIHash == "" {
		return nil, fmt.Errorf("TG_API_HASH is required")
	}
	if cfg.Telegram.Phone == "" {
		return nil, fmt.Errorf("TG_PHONE is required")
	}
	return &LoginConfig{
		APIID:   cfg.Telegram.APIID,
		APIHash: cfg.Telegram.APIHash,
		Phone:   cfg.Telegram.Phone,
		DataDir: cfg.App.DataDir,
	}, nil
}
