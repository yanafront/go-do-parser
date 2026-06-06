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

	publisher, err := telegram.NewPublisher(cfg.Telegram.BotToken, cfg.Telegram.Destination, cfg.Telegram.MatcherBot)
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

	channels := st.Snapshot()
	if len(channels) > 0 {
		log.Info(fmt.Sprintf("state loaded from %s: %v", st.Path(), channels))
	} else {
		log.Info(fmt.Sprintf("state empty, file=%s data_dir=%s", st.Path(), cfg.App.DataDir))
	}

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
		if err := a.initChannel(ctx, source, channelKey); err != nil {
			return err
		}
		lastID, err = a.store.LastMessageID(channelKey)
		if err != nil {
			return err
		}
	}

	fetched, err := a.reader.FetchNewPosts(ctx, source, lastID, a.cfg.App.BatchSize)
	if err != nil {
		return err
	}

	a.log.Info(fmt.Sprintf("channel polled %s last_id=%d new_messages=%d posts=%d",
		source, lastID, fetched.MaxID-lastID, len(fetched.Posts)),
		zap.String("channel", source),
		zap.Int("last_id", lastID),
		zap.Int("new_messages", fetched.MaxID-lastID),
		zap.Int("posts_to_publish", len(fetched.Posts)),
	)

	return a.processPosts(ctx, source, channelKey, lastID, fetched)
}

func (a *App) initChannel(ctx context.Context, source, channelKey string) error {
	latestID, err := a.reader.LatestMessageID(ctx, source)
	if err != nil {
		return err
	}
	if latestID == 0 {
		a.log.Info("channel is empty, waiting for first post", zap.String("channel", source))
		return nil
	}

	a.log.Info(fmt.Sprintf("channel baseline set %s last_message_id=%d", source, latestID),
		zap.String("channel", source),
		zap.Int("last_message_id", latestID),
	)
	if err := a.store.SetLastMessageID(channelKey, latestID); err != nil {
		return fmt.Errorf("save baseline: %w", err)
	}
	saved, err := a.store.LastMessageID(channelKey)
	if err != nil {
		return err
	}
	a.log.Info(fmt.Sprintf("baseline saved %s last_id=%d", source, saved))
	return nil
}

func (a *App) processPosts(ctx context.Context, source, channelKey string, lastID int, fetched telegram.FetchResult) error {
	maxID := fetched.MaxID
	if maxID <= lastID {
		return nil
	}

	for _, post := range fetched.Posts {
		published, err := a.store.IsPublished(channelKey, post.MessageID)
		if err != nil {
			return err
		}
		if published {
			continue
		}

		if post.GroupedID != 0 {
			continue
		}

		if !telegram.HasContact(post) {
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

		if _, err := a.store.BumpPublishCount(); err != nil {
			return err
		}

		if err := a.store.MarkPublished(channelKey, post.MessageID, destID); err != nil {
			return err
		}

		a.log.Info("published",
			zap.String("source", source),
			zap.Int("message_id", post.MessageID),
			zap.Int("dest_message_id", destID),
		)

		time.Sleep(1500 * time.Millisecond)
	}

	if err := a.store.SetLastMessageID(channelKey, maxID); err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}
	a.log.Info(fmt.Sprintf("cursor saved %s last_id=%d", source, maxID))
	return nil
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

	count, err := a.store.PublishCount()
	if err != nil {
		return 0, err
	}
	attachPromo := a.cfg.Telegram.MatcherBot != "" && (count+1)%a.cfg.App.PromoEvery == 0
	if attachPromo {
		a.log.Info("matcher promo button attached", zap.Int("publish_num", count+1))
	}

	return a.publisher.Publish(post, mediaPath, attachPromo)
}
