package telegram

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func prepareSession(dataDir string) (sessionPath string, err error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}

	sessionPath = filepath.Join(dataDir, "session.json")

	if raw := os.Getenv("TG_SESSION"); raw != "" {
		raw = normalizeBase64Env(raw)
		if raw == "" {
			return "", fmt.Errorf("decode TG_SESSION: empty after cleanup")
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return "", fmt.Errorf("decode TG_SESSION: %w (len=%d, нужна одна строка ~5600 символов)", err, len(raw))
		}
		if len(data) < 2 || data[0] != '{' {
			return "", fmt.Errorf("decode TG_SESSION: decoded %d bytes, but this is not session.json — перекодируйте ./data/session.json", len(data))
		}
		if err := os.WriteFile(sessionPath, data, 0o600); err != nil {
			return "", fmt.Errorf("write session from TG_SESSION: %w", err)
		}
	}

	return sessionPath, nil
}

func normalizeBase64Env(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dataDirWritable(dataDir string) bool {
	test := filepath.Join(dataDir, ".write_test")
	if err := os.WriteFile(test, []byte("1"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(test)
	return true
}
