package telegram

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PrepareParserSession(dataDir string) (sessionPath string, err error) {
	return prepareSession(dataDir)
}

func prepareSession(dataDir string) (sessionPath string, err error) {
	return prepareSessionFromEnv(dataDir, "TG_SESSION", "session.json")
}

func ParserSessionExists(dataDir string) bool {
	return sessionExists(filepath.Join(dataDir, "session.json"))
}

func PrepareOutreachSession(dataDir string) (sessionPath string, err error) {
	return prepareSessionFromEnv(dataDir, "OUTREACH_SESSION", "session.json")
}

func prepareSessionFromEnv(dataDir, envKey, fileName string) (sessionPath string, err error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}

	sessionPath = filepath.Join(dataDir, fileName)

	if raw := os.Getenv(envKey); raw != "" {
		raw = normalizeBase64Env(raw)
		if raw == "" {
			return "", fmt.Errorf("decode %s: empty after cleanup", envKey)
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return "", fmt.Errorf("decode %s: %w", envKey, err)
		}
		if len(data) < 2 || data[0] != '{' {
			return "", fmt.Errorf("decode %s: invalid session data", envKey)
		}
		if err := os.WriteFile(sessionPath, data, 0o600); err != nil {
			return "", fmt.Errorf("write session from %s: %w", envKey, err)
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
