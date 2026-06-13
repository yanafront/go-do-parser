package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/telegram"
)

func main() {
	loadDotEnv(".env")
	os.Unsetenv("TG_SESSION")

	cfg, err := config.LoadLogin("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	sessionPath := cfg.DataDir + "/session.json"
	if hasArg(os.Args, "--fresh") {
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
	fmt.Println("Run: ./scripts/encode-session.sh", sessionPath)
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

func hasArg(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}
