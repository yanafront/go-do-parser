package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/store"
	"github.com/anadubesko/go-do-parser/internal/telegram"
	"go.uber.org/zap"
)

type App struct {
	cfg       *config.Config
	store     *store.Store
	reader    *telegram.Reader
	publisher *telegram.Publisher
	log       *zap.Logger
}

func New(cfg *config.Config, log *zap.Logger) (*App, error) {
	st, err := store.Open(cfg.App.DataDir)
	if err != nil {
		return nil, err
	}

	publisher, err := telegram.NewPublisher(cfg.Telegram.BotToken, cfg.Telegram.Destination)
	if err != nil {
		st.Close()
		return nil, err
	}
	if err := publisher.ValidateAccess(); err != nil {
		st.Close()
		return nil, err
	}

	log.Info("destination channel ok",
		zap.String("channel", publisher.Destination()),
		zap.Int64("chat_id", publisher.ChatID()),
	)

	tmpDir := filepath.Join(cfg.App.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		st.Close()
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}

	reader := telegram.NewReader(
		cfg.Telegram.APIID,
		cfg.Telegram.APIHash,
		cfg.Telegram.Phone,
		cfg.App.DataDir,
		log,
	)

	return &App{
		cfg:       cfg,
		store:     st,
		reader:    reader,
		publisher: publisher,
		log:       log,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.store.Close()

	ready := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- a.reader.Connect(ctx, ready)
	}()

	select {
	case <-ready:
		a.log.Info("telegram reader connected")
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}

	if err := a.syncOnce(ctx); err != nil {
		a.log.Warn("initial sync error", zap.Error(err))
	}

	ticker := time.NewTicker(a.cfg.App.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && err != context.Canceled {
				return err
			}
			return nil
		case <-ticker.C:
			if err := a.syncOnce(ctx); err != nil {
				a.log.Warn("sync error", zap.Error(err))
			}
		}
	}
}

func (a *App) syncOnce(ctx context.Context) error {
	for _, source := range a.cfg.Telegram.Sources {
		if err := a.syncChannel(ctx, source); err != nil {
			a.log.Warn("channel sync failed",
				zap.String("channel", source),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (a *App) syncChannel(ctx context.Context, source string) error {
	channelKey := telegram.NormalizeChannelKey(source)
	lastID, err := a.store.LastMessageID(channelKey)
	if err != nil {
		return err
	}

	if lastID == 0 {
		return a.backfillChannel(ctx, source, channelKey)
	}

	posts, err := a.reader.FetchNewPosts(ctx, source, lastID, a.cfg.App.BatchSize)
	if err != nil {
		return err
	}

	_, err = a.processPosts(ctx, source, channelKey, lastID, posts)
	return err
}

func (a *App) backfillChannel(ctx context.Context, source, channelKey string) error {
	offsetID := 0
	maxID := 0

	for {
		posts, err := a.reader.FetchHistoricalPage(ctx, source, offsetID, a.cfg.App.BatchSize)
		if err != nil {
			return err
		}
		if len(posts) == 0 {
			break
		}

		oldestID := posts[0].MessageID
		newMax, err := a.processPosts(ctx, source, channelKey, maxID, posts)
		if err != nil {
			return err
		}
		if newMax > maxID {
			maxID = newMax
		}

		if oldestID == offsetID {
			break
		}
		offsetID = oldestID
	}

	if maxID > 0 {
		return a.store.SetLastMessageID(channelKey, maxID)
	}
	return nil
}

func (a *App) processPosts(ctx context.Context, source, channelKey string, lastID int, posts []telegram.Post) (int, error) {
	if len(posts) == 0 {
		return lastID, nil
	}

	maxID := lastID
	for _, post := range posts {
		published, err := a.store.IsPublished(channelKey, post.MessageID)
		if err != nil {
			return maxID, err
		}
		if published {
			if post.MessageID > maxID {
				maxID = post.MessageID
			}
			continue
		}

		if post.GroupedID != 0 {
			if post.MessageID > maxID {
				maxID = post.MessageID
			}
			continue
		}

		destID, err := a.publishPost(ctx, post)
		if err != nil {
			a.log.Warn("publish failed",
				zap.String("source", source),
				zap.String("destination", a.publisher.Destination()),
				zap.Int("message_id", post.MessageID),
				zap.Error(err),
			)
			continue
		}

		if err := a.store.MarkPublished(channelKey, post.MessageID, destID); err != nil {
			return maxID, err
		}

		if post.MessageID > maxID {
			maxID = post.MessageID
		}

		a.log.Info("published",
			zap.String("source", source),
			zap.Int("message_id", post.MessageID),
			zap.Int("dest_message_id", destID),
		)

		time.Sleep(1500 * time.Millisecond)
	}

	if maxID > lastID {
		if err := a.store.SetLastMessageID(channelKey, maxID); err != nil {
			return maxID, err
		}
	}
	return maxID, nil
}

func (a *App) publishPost(ctx context.Context, post telegram.Post) (int, error) {
	mediaPath := ""
	if post.HasMedia {
		mediaPath = telegram.TempMediaPath(a.cfg.App.DataDir, post.SourceChannel, post.MessageID)
		if err := a.reader.DownloadMedia(ctx, post.SourceChannel, post.MessageID, mediaPath); err != nil {
			return 0, err
		}
		defer telegram.CleanupMedia(mediaPath)
	}

	return a.publisher.Publish(post, mediaPath)
}
