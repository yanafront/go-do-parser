package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/telegram"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	log, _ := zap.NewDevelopment()
	defer log.Sync()

	reader := telegram.NewReader(
		cfg.Telegram.APIID,
		cfg.Telegram.APIHash,
		cfg.Telegram.Phone,
		cfg.App.DataDir,
		log,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ready := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- reader.Connect(ctx, ready)
	}()

	select {
	case <-ready:
		fmt.Println("OK: session saved to", cfg.App.DataDir+"/session.json")
		fmt.Println("Run: ./scripts/encode-session.sh", cfg.App.DataDir+"/session.json")
		cancel()
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
			os.Exit(1)
		}
	case <-ctx.Done():
	}

	<-errCh
}
