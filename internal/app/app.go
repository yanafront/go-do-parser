package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	outStore        *outreach.Store
	seekerStore     *outreach.Store
	employerRateStore *outreach.Store
	seekerRateStore   *outreach.Store
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

	if dbURL := resolveDatabaseURL(cfg); dbURL != "" {
		cfg.Database.URL = dbURL
		app.tryConnectDB(log)
	} else {
		log.Info("database not configured")
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
		if cfg.Seeker.Active() {
			app.seekerStore, err = outreach.OpenStoreFile(cfg.Seeker.DataDir, "seeker.json")
			if err != nil {
				st.Close()
				return nil, err
			}
		}
		app.employerRateStore, err = outreach.OpenStoreFile(cfg.Outreach.DataDir, "rate.json")
		if err != nil {
			st.Close()
			return nil, err
		}
		if cfg.Seeker.Active() {
			app.seekerRateStore, err = outreach.OpenStoreFile(cfg.Seeker.DataDir, "rate.json")
			if err != nil {
				st.Close()
				return nil, err
			}
		}

		messengerPhone := strings.TrimSpace(cfg.Outreach.Phone)
		if messengerPhone == "" {
			messengerPhone = strings.TrimSpace(cfg.Telegram.Phone)
		}

		app.outreach = outreach.NewService(
			messengerPhone,
			cfg.Outreach.DataDir,
			cfg.App.DataDir,
			cfg.Outreach,
			cfg.Seeker,
			cfg.Telegram.APIID,
			cfg.Telegram.APIHash,
			app.outStore,
			app.seekerStore,
			app.employerRateStore,
			app.seekerRateStore,
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
				zap.String("phone", telegram.MaskPhone(messengerPhone)),
				zap.Int("daily_limit", cfg.Seeker.DailyLimit),
				zap.Duration("delay", cfg.Seeker.Delay),
				zap.String("store", app.seekerStore.Path()),
				zap.Int("message_len", len(cfg.Seeker.Message)),
			)
		} else if cfg.Seeker.Active() {
			log.Error("SEEKER_ENABLED=true but SEEKER_MESSAGE is empty, dm will not be sent")
		}
	} else if cfg.Seeker.Active() {
		log.Warn("seeker enabled but messenger not started",
			zap.Bool("seeker_message_set", strings.TrimSpace(cfg.Seeker.Message) != ""),
			zap.String("hint", "set SEEKER_MESSAGE and TG_PHONE+TG_SESSION or OUTREACH_PHONE+OUTREACH_SESSION"),
		)
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
	if a.employerRateStore != nil {
		defer a.employerRateStore.Close()
	}
	if a.seekerRateStore != nil {
		defer a.seekerRateStore.Close()
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

	go a.dbReconnectLoop(ctx)

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
	a.retryPendingSeekerDMs(ctx)
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
			adUser, adPhone := outreach.SeekerAdContacts(body, post.PosterUsername, post.PosterPhone, a.contactSkip)
			a.saveJobSeekerPost(ctx, channelKeyNorm, source, post, body, adUser, adPhone)
			if a.outreach != nil {
				if a.db != nil {
					text := strings.TrimSpace(body)
					if text == "" {
						text = strings.TrimSpace(post.Text)
					}
					if text == "" {
						text = strings.TrimSpace(post.Caption)
					}
					if t, ok := outreach.SeekerTarget(text, post.PosterUsername, post.PosterPhone, adUser, adPhone, a.contactSkip); ok {
						if contacted, firstSentAt, err := a.db.WasDMContacted(ctx, t.Type, t.Raw); err != nil {
							a.log.Warn("check dm contacted failed",
								zap.String("type", t.Type),
								zap.String("contact", t.Raw),
								zap.Error(err),
							)
						} else if contacted {
							sentAt := time.Now().UTC()
							if firstSentAt != nil {
								sentAt = firstSentAt.UTC()
							}
							if err := a.db.UpdateJobSeekerDM(ctx, channelKeyNorm, post.MessageID, t.Raw, t.Type, sentAt); err != nil {
								a.log.Warn("update job seeker dm failed",
									zap.String("source", channelKeyNorm),
									zap.Int("message_id", post.MessageID),
									zap.Error(err),
								)
							}
							goto seekerDone
						}
					}
				}
				if target := a.outreach.HandleSeekerPost(ctx, outreach.PostInfo{
					SourceChannel:  source,
					MessageID:      post.MessageID,
					Body:           body,
					Text:           post.Text,
					Caption:        post.Caption,
					PosterUsername: post.PosterUsername,
					PosterPhone:    post.PosterPhone,
					AdUsername:     adUser,
					AdPhone:        adPhone,
				}); target != nil {
					a.updateJobSeekerDM(ctx, channelKeyNorm, post.MessageID, *target)
				}
			seekerDone:
			} else if a.cfg.Seeker.Active() {
				a.log.Warn("seeker post skipped: messenger not running",
					zap.String("source", source),
					zap.Int("message_id", post.MessageID),
				)
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
			if a.db != nil {
				text := strings.TrimSpace(body)
				if text == "" {
					text = strings.TrimSpace(post.Text)
				}
				if text == "" {
					text = strings.TrimSpace(post.Caption)
				}
				for _, t := range outreach.ExtractTargets(text) {
					if contacted, firstSentAt, err := a.db.WasDMContacted(ctx, t.Type, t.Raw); err != nil {
						a.log.Warn("check dm contacted failed",
							zap.String("type", t.Type),
							zap.String("contact", t.Raw),
							zap.Error(err),
						)
						break
					} else if contacted {
						sentAt := time.Now().UTC()
						if firstSentAt != nil {
							sentAt = firstSentAt.UTC()
						}
						if err := a.db.UpdateVacancyDM(ctx, channelKeyNorm, post.MessageID, t.Raw, t.Type, sentAt); err != nil {
							a.log.Warn("update vacancy dm failed",
								zap.String("source", channelKeyNorm),
								zap.Int("message_id", post.MessageID),
								zap.Error(err),
							)
						}
						goto vacancyDone
					}
				}
			}
			if target := a.outreach.HandlePost(ctx, outreach.PostInfo{
				SourceChannel: source,
				MessageID:     post.MessageID,
				Text:          post.Text,
				Caption:       post.Caption,
			}); target != nil {
				a.updateVacancyDM(ctx, channelKeyNorm, post.MessageID, *target)
			}
		vacancyDone:
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

func resolveDatabaseURL(cfg *config.Config) string {
	if u := strings.TrimSpace(os.Getenv("DATABASE_PRIVATE_URL")); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("DATABASE_URL")); u != "" {
		return u
	}
	if u := strings.TrimSpace(cfg.Database.URL); u != "" {
		return u
	}
	return db.ResolveURL()
}

func (a *App) tryConnectDB(log *zap.Logger) {
	if a.db != nil {
		return
	}
	connURL := resolveDatabaseURL(a.cfg)
	if strings.TrimSpace(connURL) == "" {
		log.Warn("database url is empty",
			zap.String("hint", "Railway: go-do-parser → Variables → DATABASE_URL = ${{ Postgres.DATABASE_URL }}"),
		)
		return
	}
	database, err := db.Open(connURL)
	if err != nil {
		log.Warn("database unavailable, continuing without db",
			zap.Error(err),
			zap.String("host", db.MaskURL(connURL)),
			zap.String("hint", "Railway: go-do-parser → Variables → DATABASE_URL = ${{ Postgres.DATABASE_URL }}"),
		)
		return
	}
	a.db = database
	a.cfg.Database.URL = connURL
	log.Info("database connected", zap.String("host", db.MaskURL(connURL)))
}

func (a *App) ensureDB() bool {
	if a.db != nil {
		return true
	}
	a.tryConnectDB(a.log)
	return a.db != nil
}

func (a *App) dbReconnectLoop(ctx context.Context) {
	if resolveDatabaseURL(a.cfg) == "" {
		return
	}
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.db != nil {
				continue
			}
			a.tryConnectDB(a.log)
		}
	}
}

func (a *App) saveVacancy(ctx context.Context, sourceChannel string, messageID, destID int, body string) {
	if !a.ensureDB() {
		a.log.Warn("vacancy not saved: database not connected",
			zap.String("source", sourceChannel),
			zap.Int("message_id", messageID),
		)
		return
	}
	adUser, adPhone := outreach.ExtractAdContacts(body, a.contactSkip)
	messageLink := a.reader.MessageLink(ctx, sourceChannel, messageID)
	if err := a.db.SaveVacancy(ctx, db.Vacancy{
		SourceChannel:     sourceChannel,
		SourceMessageID:   messageID,
		SourceMessageLink: messageLink,
		DestMessageID:     destID,
		Body:              body,
		AdUsername:        adUser,
		AdPhone:           adPhone,
		PublishedAt:       time.Now().UTC(),
	}); err != nil {
		a.log.Warn("save vacancy failed",
			zap.String("source", sourceChannel),
			zap.Int("message_id", messageID),
			zap.Error(err),
		)
		return
	}
	a.log.Info("vacancy saved",
		zap.String("source", sourceChannel),
		zap.Int("message_id", messageID),
	)
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

func (a *App) saveJobSeekerPost(ctx context.Context, sourceChannel, source string, post telegram.Post, body, adUser, adPhone string) {
	if !a.ensureDB() {
		return
	}
	messageLink := a.reader.MessageLink(ctx, source, post.MessageID)
	if err := a.db.SaveJobSeekerPost(ctx, db.JobSeekerPost{
		SourceChannel:     sourceChannel,
		SourceMessageID:   post.MessageID,
		SourceMessageLink: messageLink,
		Body:              body,
		PosterUsername:    post.PosterUsername,
		PosterPhone:       post.PosterPhone,
		AdUsername:        adUser,
		AdPhone:           adPhone,
	}); err != nil {
		a.log.Warn("save job seeker post failed",
			zap.String("source", sourceChannel),
			zap.Int("message_id", post.MessageID),
			zap.Error(err),
		)
		return
	}
	a.log.Info("job seeker post saved",
		zap.String("source", sourceChannel),
		zap.Int("message_id", post.MessageID),
	)
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

func (a *App) retryPendingSeekerDMs(ctx context.Context) {
	if a.outreach == nil || !a.cfg.Seeker.Enabled() || a.db == nil {
		return
	}
	pending, err := a.db.ListPendingSeekerDMs(ctx, 1)
	if err != nil {
		a.log.Warn("list pending seeker dms failed", zap.Error(err))
		return
	}
	for _, p := range pending {
		if t, ok := outreach.SeekerTarget(strings.TrimSpace(p.Body), p.PosterUsername, p.PosterPhone, p.AdUsername, p.AdPhone, a.contactSkip); ok {
			if contacted, firstSentAt, err := a.db.WasDMContacted(ctx, t.Type, t.Raw); err != nil {
				a.log.Warn("check dm contacted failed",
					zap.String("type", t.Type),
					zap.String("contact", t.Raw),
					zap.Error(err),
				)
			} else if contacted {
				sentAt := time.Now().UTC()
				if firstSentAt != nil {
					sentAt = firstSentAt.UTC()
				}
				if err := a.db.UpdateJobSeekerDM(ctx, p.SourceChannel, p.SourceMessageID, t.Raw, t.Type, sentAt); err != nil {
					a.log.Warn("update job seeker dm failed",
						zap.String("source", p.SourceChannel),
						zap.Int("message_id", p.SourceMessageID),
						zap.Error(err),
					)
				}
				continue
			}
		}
		if target := a.outreach.HandleSeekerPost(ctx, outreach.PostInfo{
			SourceChannel:  p.SourceChannel,
			MessageID:      p.SourceMessageID,
			Body:           p.Body,
			Text:           p.Body,
			PosterUsername: p.PosterUsername,
			PosterPhone:    p.PosterPhone,
			AdUsername:     p.AdUsername,
			AdPhone:        p.AdPhone,
		}); target != nil {
			a.updateJobSeekerDM(ctx, p.SourceChannel, p.SourceMessageID, *target)
		}
	}
}
