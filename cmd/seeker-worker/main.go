package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/anadubesko/go-do-parser/internal/db"
	"github.com/anadubesko/go-do-parser/internal/health"
	"github.com/anadubesko/go-do-parser/internal/seekerworker"
	"go.uber.org/zap"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	health.Start()

	cfg, err := seekerworker.LoadConfig()
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("open database", zap.Error(err))
	}
	defer database.Close()

	log.Info("seeker worker starting",
		zap.Int("agents", len(cfg.Agents)),
		zap.Duration("poll", cfg.PollInterval),
		zap.Int("daily_limit_per_agent", cfg.Seeker.DailyLimit),
		zap.Duration("delay_min", cfg.Seeker.MinDelay()),
		zap.Duration("delay_max", cfg.Seeker.MaxDelay()),
		zap.String("data_dir", cfg.DataDir),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool := seekerworker.NewPool(cfg, database, log)
	if err := pool.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal("run", zap.Error(err))
	}
}
