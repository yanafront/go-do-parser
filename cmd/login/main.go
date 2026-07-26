package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/telegram"
)

func main() {
	loadDotEnv(".env")
	os.Unsetenv("TG_SESSION")

	agentID, phoneOverride, fresh := parseArgs(os.Args[1:])

	cfg, err := config.LoadLogin("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if phoneOverride != "" {
		cfg.Phone = phoneOverride
	}
	if agentID != "" {
		cfg.DataDir = filepath.Join(cfg.DataDir, "agents", agentID)
	}

	sessionPath := filepath.Join(cfg.DataDir, "session.json")
	if fresh {
		_ = os.Remove(sessionPath)
		fmt.Println("Старая session.json удалена")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := telegram.Login(ctx, cfg.APIID, cfg.APIHash, cfg.Phone, cfg.DataDir); err != nil {
		if err != context.Canceled {
			fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("OK: session saved to", sessionPath)
	if agentID != "" {
		key := strings.ToUpper(strings.ReplaceAll(agentID, "-", "_"))
		fmt.Println("Encode:")
		fmt.Println("  ./scripts/encode-session.sh", sessionPath)
		fmt.Println("Then set in .env / Railway:")
		fmt.Printf("  SEEKER_AGENT_%s_PHONE=%s\n", key, cfg.Phone)
		fmt.Printf("  SEEKER_AGENT_%s_SESSION=<base64 from encode-session.sh>\n", key)
		return
	}
	fmt.Println("Run: ./scripts/encode-session.sh", sessionPath)
}

func parseArgs(args []string) (agentID, phone string, fresh bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--fresh":
			fresh = true
		case a == "--agent" && i+1 < len(args):
			i++
			agentID = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--agent="):
			agentID = strings.TrimSpace(strings.TrimPrefix(a, "--agent="))
		case a == "--phone" && i+1 < len(args):
			i++
			phone = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--phone="):
			phone = strings.TrimSpace(strings.TrimPrefix(a, "--phone="))
		}
	}
	return agentID, phone, fresh
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
