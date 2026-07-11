package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anadubesko/go-do-parser/internal/db"
	"github.com/anadubesko/go-do-parser/internal/health"
	"github.com/anadubesko/go-do-parser/internal/onliner"
	"go.uber.org/zap"
)

func main() {
	once := flag.Bool("once", false, "run one sync and exit")
	flag.Parse()

	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	health.Start()

	cfg := onliner.LoadRuntimeConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("open database", zap.Error(err))
	}
	defer database.Close()

	store, err := onliner.OpenStore(cfg.DataDir)
	if err != nil {
		log.Fatal("open onliner store", zap.Error(err))
	}

	scraper := onliner.NewScraper(onliner.Config{
		ForumID:         cfg.ForumID,
		ForumPages:      cfg.ForumPages,
		SearchPages:     cfg.SearchPages,
		SearchQueries:   cfg.SearchQueries,
		RequestDelay:    cfg.RequestDelay,
		MaxTopicAgeDays: cfg.MaxTopicAgeDays,
	}, store, database, log)

	log.Info("onliner scraper started",
		zap.String("data_dir", cfg.DataDir),
		zap.Int("forum_id", cfg.ForumID),
		zap.Int("forum_pages", cfg.ForumPages),
		zap.Int("search_pages", cfg.SearchPages),
		zap.Strings("search_queries", cfg.SearchQueries),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	run := func() error {
		return scraper.SyncOnce(ctx)
	}

	if *once {
		if err := run(); err != nil && err != context.Canceled {
			log.Fatal("sync", zap.Error(err))
		}
		return
	}

	if err := run(); err != nil && err != context.Canceled {
		log.Fatal("initial sync", zap.Error(err))
	}

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := run(); err != nil && err != context.Canceled {
				log.Warn("sync failed", zap.Error(err))
			}
		}
	}
}
