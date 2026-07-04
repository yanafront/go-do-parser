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
	cfg       config.OutreachConfig
	apiID     int
	apiHash   string
	store     *Store
	skip      map[string]bool
	log       *zap.Logger
	client    *gotdtelegram.Client
	api       *tg.Client
	sendCh    chan sendJob
	readyCh   chan struct{}
}

type sendJob struct {
	target  Target
	source  string
	msgID   int
	result  chan error
}

type PostInfo struct {
	SourceChannel string
	MessageID     int
	Text          string
	Caption       string
}

func NewService(cfg config.OutreachConfig, apiID int, apiHash string, store *Store, skip map[string]bool, log *zap.Logger) *Service {
	return &Service{
		cfg:     cfg,
		apiID:   apiID,
		apiHash: apiHash,
		store:   store,
		skip:    skip,
		log:     log,
		sendCh:  make(chan sendJob, 16),
		readyCh: make(chan struct{}),
	}
}

func (s *Service) Connect(ctx context.Context) error {
	sessionPath, err := telegram.PrepareOutreachSession(s.cfg.DataDir)
	if err != nil {
		return err
	}
	if os.Getenv("OUTREACH_SESSION") != "" {
		s.log.Info("outreach session restored from OUTREACH_SESSION")
	} else if _, err := os.Stat(sessionPath); err == nil {
		s.log.Info("outreach session loaded from disk", zap.String("path", sessionPath))
	} else {
		return fmt.Errorf("outreach session not found: set OUTREACH_SESSION or login with OUTREACH_PHONE")
	}

	s.client = gotdtelegram.NewClient(s.apiID, s.apiHash, gotdtelegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
	})

	return s.client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(
			constantAuthenticator(
				telegram.NormalizePhone(s.cfg.Phone),
				strings.TrimSpace(os.Getenv("OUTREACH_AUTH_PASSWORD")),
			),
			auth.SendCodeOptions{},
		)
		if err := s.client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("outreach auth: %w", err)
		}

		s.api = s.client.API()
		s.log.Info("outreach authorized", zap.String("phone", telegram.MaskPhone(s.cfg.Phone)))
		close(s.readyCh)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case job := <-s.sendCh:
				job.result <- s.sendOne(ctx, job.target, job.source, job.msgID)
			}
		}
	})
}

func (s *Service) Ready() <-chan struct{} {
	return s.readyCh
}

func (s *Service) HandlePost(ctx context.Context, post PostInfo) {
	if !s.store.CanSendToday(s.cfg.DailyLimit) {
		s.log.Info("outreach daily limit reached", zap.Int("limit", s.cfg.DailyLimit))
		return
	}
	if !s.store.CanSendNow(s.cfg.Delay) {
		s.log.Info("outreach cooldown", zap.Duration("delay", s.cfg.Delay))
		return
	}

	text := strings.TrimSpace(post.Text)
	if text == "" {
		text = strings.TrimSpace(post.Caption)
	}
	targets := ExtractTargets(text)
	if len(targets) == 0 {
		return
	}

	for _, target := range targets {
		if !s.store.CanSendToday(s.cfg.DailyLimit) {
			return
		}
		if !s.store.CanSendNow(s.cfg.Delay) {
			return
		}
		if s.store.WasContacted(target.Key) {
			continue
		}

		if err := s.send(ctx, target, post.SourceChannel, post.MessageID); err != nil {
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
		if err := s.store.MarkSent(target.Key, rec); err != nil {
			s.log.Warn("outreach store failed", zap.Error(err))
		}

		s.log.Info("outreach sent",
			zap.String("target", target.Raw),
			zap.String("type", target.Type),
			zap.String("source", post.SourceChannel),
			zap.Int("message_id", post.MessageID),
			zap.Int("daily_sent", s.store.DailySent()),
		)
		return
	}
}

func (s *Service) send(ctx context.Context, target Target, source string, msgID int) error {
	result := make(chan error, 1)
	select {
	case s.sendCh <- sendJob{target: target, source: source, msgID: msgID, result: result}:
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

func (s *Service) sendOne(ctx context.Context, target Target, source string, msgID int) error {
	peer, err := s.resolvePeer(ctx, target)
	if err != nil {
		return err
	}

	rid, err := randomID()
	if err != nil {
		return err
	}

	text, entities := formatMessage(s.cfg.Message)
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

func constantAuthenticator(phone, password string) auth.UserAuthenticator {
	return auth.Constant(phone, password, auth.CodeAuthenticatorFunc(func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		if v := strings.TrimSpace(os.Getenv("OUTREACH_AUTH_CODE")); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("OUTREACH session expired: login locally and update OUTREACH_SESSION")
	}))
}
