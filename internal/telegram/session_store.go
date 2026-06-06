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
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return "", fmt.Errorf("decode TG_SESSION: %w (проверьте: одна строка без пробелов и кавычек)", err)
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
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func dataDirWritable(dataDir string) bool {
	test := filepath.Join(dataDir, ".write_test")
	if err := os.WriteFile(test, []byte("1"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(test)
	return true
}
