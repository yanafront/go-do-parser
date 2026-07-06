package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/db"
	"github.com/anadubesko/go-do-parser/internal/outreach"
	"github.com/anadubesko/go-do-parser/internal/store"
	"github.com/anadubesko/go-do-parser/internal/telegram"
	"github.com/anadubesko/go-do-parser/internal/webhook"
	"go.uber.org/zap"
)

type App struct {
	cfg         *config.Config
	store       *store.Store
	db          *db.DB
	contactSkip map[string]bool
	reader      *telegram.Reader
	publisher   *telegram.Publisher
	webhook     *webhook.Client
	outreach    *outreach.Service
	outStore    *outreach.Store
	seekerStore *outreach.Store
	rateStore   *outreach.Store
	log         *zap.Logger
}

func New(cfg *config.Config, log *zap.Logger) (*App, error) {
	st, err := store.Open(cfg.App.DataDir)
	if err != nil {
		return nil, err
	}

	publisher, err := telegram.NewPublisher(cfg.Telegram.BotToken, cfg.Telegram.Destination, cfg.Telegram.MatcherBot, cfg.Telegram.PlatformURL)
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
	if cfg.Telegram.MatcherBot != "" || cfg.Telegram.PlatformURL != "" {
		log.Info("promo enabled",
			zap.String("matcher_bot", cfg.Telegram.MatcherBot),
			zap.String("platform_url", cfg.Telegram.PlatformURL),
			zap.Int("matcher_every", cfg.App.PromoEvery),
			zap.Int("platform_every", cfg.App.PlatformEvery),
			zap.Int("published_so_far", st.TotalPublished()),
		)
	}

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

	app := &App{
		cfg:         cfg,
		store:       st,
		contactSkip: outreach.BuildSkipList(cfg.Telegram.Sources, cfg.Telegram.Destination, cfg.Telegram.MatcherBot),
		reader:      reader,
		publisher:   publisher,
		webhook:     webhook.New(cfg.Telegram.MatcherURL, cfg.Telegram.IngestSecret),
		log:         log,
	}

	if cfg.Database.Enabled() {
		database, err := db.Open(cfg.Database.URL)
		if err != nil {
			st.Close()
			return nil, err
		}
		app.db = database
		log.Info("database connected")
	}

	if cfg.MessengerEnabled() {
		skip := app.contactSkip

		var err error
		if cfg.Outreach.Enabled() {
			app.outStore, err = outreach.OpenStore(cfg.Outreach.DataDir)
			if err != nil {
				st.Close()
				return nil, err
			}
		}
		if cfg.Seeker.Enabled() {
			app.seekerStore, err = outreach.OpenStoreFile(cfg.Seeker.DataDir, "seeker.json")
			if err != nil {
				st.Close()
				return nil, err
			}
		}
		app.rateStore, err = outreach.OpenStoreFile(cfg.Outreach.DataDir, "rate.json")
		if err != nil {
			st.Close()
			return nil, err
		}

		app.outreach = outreach.NewService(
			cfg.Outreach.Phone,
			cfg.Outreach.DataDir,
			cfg.Outreach,
			cfg.Seeker,
			cfg.Telegram.APIID,
			cfg.Telegram.APIHash,
			app.outStore,
			app.seekerStore,
			app.rateStore,
			skip,
			log,
		)

		if cfg.Outreach.Enabled() {
			log.Info("outreach enabled",
				zap.String("phone", telegram.MaskPhone(cfg.Outreach.Phone)),
				zap.Int("daily_limit", cfg.Outreach.DailyLimit),
				zap.Duration("delay", cfg.Outreach.Delay),
				zap.String("store", app.outStore.Path()),
			)
		}
		if cfg.Seeker.Enabled() {
			log.Info("seeker outreach enabled",
				zap.String("phone", telegram.MaskPhone(cfg.Outreach.Phone)),
				zap.Int("daily_limit", cfg.Seeker.DailyLimit),
				zap.Duration("delay", cfg.Seeker.Delay),
				zap.String("store", app.seekerStore.Path()),
			)
		}
	}

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.store.Close()
	if a.db != nil {
		defer a.db.Close()
	}
	if a.outStore != nil {
		defer a.outStore.Close()
	}
	if a.seekerStore != nil {
		defer a.seekerStore.Close()
	}
	if a.rateStore != nil {
		defer a.rateStore.Close()
	}

	ready := make(chan struct{})
	errCh := make(chan error, 2)

	go func() {
		errCh <- a.reader.Connect(ctx, ready)
	}()

	var outreachReadyCh <-chan struct{}
	if a.outreach != nil {
		go func() {
			errCh <- a.outreach.Connect(ctx)
		}()
		outreachReadyCh = a.outreach.Ready()
	}

	readerReady := false
	outreachReady := a.outreach == nil
	for !(readerReady && outreachReady) {
		select {
		case <-ready:
			if !readerReady {
				readerReady = true
				a.log.Info("telegram reader connected")
			}
		case <-outreachReadyCh:
			if !outreachReady {
				outreachReady = true
				a.log.Info("outreach connected")
			}
		case err := <-errCh:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
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

		body := telegram.PostBody(post)
		channelKeyNorm := telegram.NormalizeChannelKey(source)

		if telegram.IsJobSeeker(post) {
			a.saveJobSeekerPost(ctx, channelKeyNorm, post.MessageID, body)
			if a.outreach != nil {
				if target := a.outreach.HandleSeekerPost(ctx, outreach.PostInfo{
					SourceChannel: source,
					MessageID:     post.MessageID,
					Text:          post.Text,
					Caption:       post.Caption,
				}); target != nil {
					a.updateJobSeekerDM(ctx, channelKeyNorm, post.MessageID, *target)
				}
			}
		}

		if telegram.IsBlocked(post) {
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

		if err := a.store.MarkPublished(channelKey, post.MessageID, destID); err != nil {
			return err
		}

		total := a.store.TotalPublished()
		a.log.Info("published",
			zap.String("source", source),
			zap.Int("message_id", post.MessageID),
			zap.Int("dest_message_id", destID),
			zap.Int("publish_total", total),
			zap.Bool("promo_button", total% a.cfg.App.PromoEvery == 0),
		)

		if a.webhook.Enabled() {
			notifyPost := post
			if notifyPost.SourceChannel == "" {
				notifyPost.SourceChannel = source
			}
			if err := a.webhook.Notify(ctx, notifyPost); err != nil {
				a.log.Warn("matcher webhook failed",
					zap.Int("message_id", post.MessageID),
					zap.Error(err),
				)
			}
		}

		a.saveVacancy(ctx, channelKeyNorm, post.MessageID, destID, body)

		if a.outreach != nil {
			if target := a.outreach.HandlePost(ctx, outreach.PostInfo{
				SourceChannel: source,
				MessageID:     post.MessageID,
				Text:          post.Text,
				Caption:       post.Caption,
			}); target != nil {
				a.updateVacancyDM(ctx, channelKeyNorm, post.MessageID, *target)
			}
		}

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

	total := a.store.TotalPublished()
	nextNum := total + 1
	showMatcher := false
	showPlatform := a.cfg.Telegram.PlatformURL != "" && nextNum%a.cfg.App.PlatformEvery == 0
	if showMatcher || showPlatform {
		a.log.Info("promo button",
			zap.Int("publish_num", nextNum),
			zap.Bool("matcher", showMatcher),
			zap.Bool("platform", showPlatform),
		)
	}

	return a.publisher.Publish(post, mediaPath, showMatcher, showPlatform)
}

func (a *App) saveVacancy(ctx context.Context, sourceChannel string, messageID, destID int, body string) {
	if a.db == nil {
		return
	}
	adUser, adPhone := outreach.ExtractAdContacts(body, a.contactSkip)
	if err := a.db.SaveVacancy(ctx, db.Vacancy{
		SourceChannel:   sourceChannel,
		SourceMessageID: messageID,
		DestMessageID:   destID,
		Body:            body,
		AdUsername:      adUser,
		AdPhone:         adPhone,
		PublishedAt:     time.Now().UTC(),
	}); err != nil {
		a.log.Warn("save vacancy failed",
			zap.String("source", sourceChannel),
			zap.Int("message_id", messageID),
			zap.Error(err),
		)
	}
}

func (a *App) updateVacancyDM(ctx context.Context, sourceChannel string, messageID int, target outreach.Target) {
	if a.db == nil {
		return
	}
	sentAt := time.Now().UTC()
	if err := a.db.UpdateVacancyDM(ctx, sourceChannel, messageID, target.Raw, target.Type, sentAt); err != nil {
		a.log.Warn("update vacancy dm failed",
			zap.String("source", sourceChannel),
			zap.Int("message_id", messageID),
			zap.Error(err),
		)
	}
}

func (a *App) saveJobSeekerPost(ctx context.Context, sourceChannel string, messageID int, body string) {
	if a.db == nil {
		return
	}
	poster, adUser, adPhone := outreach.ExtractSeekerContacts(body, a.contactSkip)
	if err := a.db.SaveJobSeekerPost(ctx, db.JobSeekerPost{
		SourceChannel:   sourceChannel,
		SourceMessageID: messageID,
		Body:            body,
		PosterUsername:  poster,
		AdUsername:      adUser,
		AdPhone:         adPhone,
	}); err != nil {
		a.log.Warn("save job seeker post failed",
			zap.String("source", sourceChannel),
			zap.Int("message_id", messageID),
			zap.Error(err),
		)
	}
}

func (a *App) updateJobSeekerDM(ctx context.Context, sourceChannel string, messageID int, target outreach.Target) {
	if a.db == nil {
		return
	}
	sentAt := time.Now().UTC()
	if err := a.db.UpdateJobSeekerDM(ctx, sourceChannel, messageID, target.Raw, target.Type, sentAt); err != nil {
		a.log.Warn("update job seeker dm failed",
			zap.String("source", sourceChannel),
			zap.Int("message_id", messageID),
			zap.Error(err),
		)
	}
}
