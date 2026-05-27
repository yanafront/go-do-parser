package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/anadubesko/go-do-parser/internal/app"
	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/health"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", os.Getenv("CONFIG_PATH"), "path to config file (optional)")
	flag.Parse()

	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	health.Start()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal("init app", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := application.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal("run", zap.Error(err))
	}
}
