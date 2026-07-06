package outreach

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anadubesko/go-do-parser/internal/config"
	"github.com/anadubesko/go-do-parser/internal/telegram"
	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"go.uber.org/zap"
)

type Service struct {
	phone         string
	dataDir       string
	parserDataDir string
	employerCfg   config.OutreachConfig
	seekerCfg   config.SeekerConfig
	apiID       int
	apiHash     string
	employerStore *Store
	seekerStore   *Store
	rateStore     *Store
	skip          map[string]bool
	log           *zap.Logger
	client        *gotdtelegram.Client
	api           *tg.Client
	sendCh        chan sendJob
	readyCh       chan struct{}
}

type sendJob struct {
	target  Target
	source  string
	msgID   int
	message string
	result  chan error
}

type PostInfo struct {
	SourceChannel string
	MessageID     int
	Text          string
	Caption       string
}

func NewService(
	phone, dataDir, parserDataDir string,
	employerCfg config.OutreachConfig,
	seekerCfg config.SeekerConfig,
	apiID int,
	apiHash string,
	employerStore, seekerStore, rateStore *Store,
	skip map[string]bool,
	log *zap.Logger,
) *Service {
	return &Service{
		phone:         phone,
		dataDir:       dataDir,
		parserDataDir: parserDataDir,
		employerCfg:   employerCfg,
		seekerCfg:     seekerCfg,
		apiID:         apiID,
		apiHash:       apiHash,
		employerStore: employerStore,
		seekerStore:   seekerStore,
		rateStore:     rateStore,
		skip:          skip,
		log:           log,
		sendCh:        make(chan sendJob, 16),
		readyCh:       make(chan struct{}),
	}
}

func (s *Service) Connect(ctx context.Context) error {
	sessionPath, err := s.resolveSessionPath()
	if err != nil {
		return err
	}

	s.client = gotdtelegram.NewClient(s.apiID, s.apiHash, gotdtelegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
	})

	return s.client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(
			constantAuthenticator(
				telegram.NormalizePhone(s.phone),
				s.authPassword(),
			),
			auth.SendCodeOptions{},
		)
		if err := s.client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("outreach auth: %w", err)
		}

		s.api = s.client.API()
		s.log.Info("outreach authorized", zap.String("phone", telegram.MaskPhone(s.phone)))
		close(s.readyCh)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case job := <-s.sendCh:
				job.result <- s.sendOne(ctx, job.target, job.message)
			}
		}
	})
}

func (s *Service) Ready() <-chan struct{} {
	return s.readyCh
}

func (s *Service) HandlePost(ctx context.Context, post PostInfo) *Target {
	if !s.employerCfg.Enabled() || s.employerStore == nil {
		return nil
	}
	if !s.employerStore.CanSendToday(s.employerCfg.DailyLimit) {
		s.log.Info("outreach daily limit reached", zap.Int("limit", s.employerCfg.DailyLimit))
		return nil
	}
	if !s.rateStore.CanSendNow(s.employerCfg.Delay) {
		s.log.Info("outreach cooldown", zap.Duration("delay", s.employerCfg.Delay))
		return nil
	}

	text := postText(post)
	targets := ExtractTargets(text)
	if len(targets) == 0 {
		return nil
	}

	for _, target := range targets {
		if !s.employerStore.CanSendToday(s.employerCfg.DailyLimit) {
			return nil
		}
		if !s.rateStore.CanSendNow(s.employerCfg.Delay) {
			return nil
		}
		if s.employerStore.WasContacted(target.Key) {
			continue
		}

		if err := s.send(ctx, target, s.employerCfg.Message); err != nil {
			s.log.Warn("outreach send failed",
				zap.String("target", target.Raw),
				zap.String("type", target.Type),
				zap.Error(err),
			)
			continue
		}

		rec := Record{
			Target:  target.Raw,
			Type:    target.Type,
			SentAt:  time.Now().UTC().Format(time.RFC3339),
			Source:  post.SourceChannel,
			Message: post.MessageID,
		}
		if err := s.employerStore.MarkSent(target.Key, rec); err != nil {
			s.log.Warn("outreach store failed", zap.Error(err))
		}
		if err := s.rateStore.TouchLastSent(); err != nil {
			s.log.Warn("outreach rate store failed", zap.Error(err))
		}

		s.log.Info("outreach sent",
			zap.String("target", target.Raw),
			zap.String("type", target.Type),
			zap.String("source", post.SourceChannel),
			zap.Int("message_id", post.MessageID),
			zap.Int("daily_sent", s.employerStore.DailySent()),
		)
		return &target
	}
	return nil
}

func (s *Service) HandleSeekerPost(ctx context.Context, post PostInfo) *Target {
	if !s.seekerCfg.Enabled() || s.seekerStore == nil {
		return nil
	}
	if !telegram.IsJobSeeker(telegram.Post{Text: post.Text, Caption: post.Caption}) {
		return nil
	}
	if !s.seekerStore.CanSendToday(s.seekerCfg.DailyLimit) {
		s.log.Info("seeker daily limit reached", zap.Int("limit", s.seekerCfg.DailyLimit))
		return nil
	}
	if !s.rateStore.CanSendNow(s.seekerCfg.Delay) {
		s.log.Info("seeker cooldown", zap.Duration("delay", s.seekerCfg.Delay))
		return nil
	}

	text := postText(post)
	target, ok := ExtractSeekerTarget(text, s.skip)
	if !ok {
		s.log.Info("seeker skipped: no contact in post",
			zap.String("source", post.SourceChannel),
			zap.Int("message_id", post.MessageID),
		)
		return nil
	}
	if s.seekerStore.WasContacted(target.Key) {
		s.log.Info("seeker skipped: already contacted",
			zap.String("target", target.Raw),
			zap.String("type", target.Type),
		)
		return nil
	}

	if err := s.send(ctx, target, s.seekerCfg.Message); err != nil {
		s.log.Warn("seeker send failed",
			zap.String("target", target.Raw),
			zap.Error(err),
		)
		return nil
	}

	rec := Record{
		Target:  target.Raw,
		Type:    target.Type,
		SentAt:  time.Now().UTC().Format(time.RFC3339),
		Source:  post.SourceChannel,
		Message: post.MessageID,
	}
	if err := s.seekerStore.MarkSent(target.Key, rec); err != nil {
		s.log.Warn("seeker store failed", zap.Error(err))
	}
	if err := s.rateStore.TouchLastSent(); err != nil {
		s.log.Warn("seeker rate store failed", zap.Error(err))
	}

	s.log.Info("seeker sent",
		zap.String("target", target.Raw),
		zap.String("source", post.SourceChannel),
		zap.Int("message_id", post.MessageID),
		zap.Int("daily_sent", s.seekerStore.DailySent()),
	)
	return &target
}

func postText(post PostInfo) string {
	text := strings.TrimSpace(post.Text)
	if text == "" {
		text = strings.TrimSpace(post.Caption)
	}
	return text
}

func (s *Service) send(ctx context.Context, target Target, message string) error {
	result := make(chan error, 1)
	select {
	case s.sendCh <- sendJob{target: target, message: message, result: result}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) sendOne(ctx context.Context, target Target, message string) error {
	peer, err := s.resolvePeer(ctx, target)
	if err != nil {
		return err
	}

	rid, err := randomID()
	if err != nil {
		return err
	}

	text, entities := formatMessage(message)
	req := &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: rid,
	}
	if len(entities) > 0 {
		req.SetEntities(entities)
	}

	_, err = s.api.MessagesSendMessage(ctx, req)
	if err != nil {
		if wait, ok := tgerr.AsFloodWait(err); ok {
			return fmt.Errorf("flood wait %v: %w", wait, err)
		}
		return err
	}
	return nil
}

func (s *Service) resolvePeer(ctx context.Context, target Target) (tg.InputPeerClass, error) {
	switch target.Type {
	case "username":
		resolved, err := s.api.ContactsResolveUsername(ctx, target.Raw)
		if err != nil {
			return nil, fmt.Errorf("resolve @%s: %w", target.Raw, err)
		}
		for _, u := range resolved.Users {
			user, ok := u.(*tg.User)
			if !ok || user.Bot {
				continue
			}
			if user.Username != "" && strings.EqualFold(user.Username, target.Raw) {
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
			}
		}
		for _, u := range resolved.Users {
			user, ok := u.(*tg.User)
			if ok && !user.Bot {
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
			}
		}
		return nil, fmt.Errorf("@%s is not a user account", target.Raw)
	case "phone":
		clientID, err := randomID()
		if err != nil {
			return nil, err
		}
		imported, err := s.api.ContactsImportContacts(ctx, []tg.InputPhoneContact{
			{
				ClientID:  clientID,
				Phone:     target.Raw,
				FirstName: "Contact",
				LastName:  "",
			},
		})
		if err != nil {
			return nil, fmt.Errorf("import phone %s: %w", target.Raw, err)
		}
		if len(imported.Users) == 0 {
			return nil, fmt.Errorf("phone %s not found in Telegram", target.Raw)
		}
		user, ok := imported.Users[0].(*tg.User)
		if !ok || user.Bot {
			return nil, fmt.Errorf("phone %s is not a user account", target.Raw)
		}
		return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
	default:
		return nil, fmt.Errorf("unknown target type %q", target.Type)
	}
}

func randomID() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	id := int64(binary.LittleEndian.Uint64(b[:]))
	if id == 0 {
		id = 1
	}
	return id, nil
}

func (s *Service) resolveSessionPath() (string, error) {
	sessionPath, err := telegram.PrepareOutreachSession(s.dataDir)
	if err != nil {
		return "", err
	}
	if os.Getenv("OUTREACH_SESSION") != "" {
		s.log.Info("messenger session from OUTREACH_SESSION")
		return sessionPath, nil
	}
	if _, err := os.Stat(sessionPath); err == nil {
		s.log.Info("messenger session from outreach disk", zap.String("path", sessionPath))
		return sessionPath, nil
	}
	parserPath, err := telegram.PrepareParserSession(s.parserDataDir)
	if err != nil {
		return "", err
	}
	if os.Getenv("TG_SESSION") != "" {
		s.log.Info("messenger session from TG_SESSION")
		return parserPath, nil
	}
	if telegram.ParserSessionExists(s.parserDataDir) {
		s.log.Info("messenger session from parser disk", zap.String("path", parserPath))
		return parserPath, nil
	}
	return "", fmt.Errorf("messenger session not found: set OUTREACH_SESSION or TG_SESSION")
}

func (s *Service) authPassword() string {
	if v := strings.TrimSpace(os.Getenv("OUTREACH_AUTH_PASSWORD")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("TG_AUTH_PASSWORD"))
}

func constantAuthenticator(phone, password string) auth.UserAuthenticator {
	return auth.Constant(phone, password, auth.CodeAuthenticatorFunc(func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		if v := strings.TrimSpace(os.Getenv("OUTREACH_AUTH_CODE")); v != "" {
			return v, nil
		}
		if v := strings.TrimSpace(os.Getenv("TG_AUTH_CODE")); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("messenger session expired: update OUTREACH_SESSION or TG_SESSION")
	}))
}
