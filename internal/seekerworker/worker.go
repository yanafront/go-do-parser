package seekerworker

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
	"github.com/anadubesko/go-do-parser/internal/telegram"
	"go.uber.org/zap"
)

type AgentConfig struct {
	ID       string
	Phone    string
	Session  string
	Password string
}

type Config struct {
	APIID        int
	APIHash      string
	DataDir      string
	DatabaseURL  string
	PollInterval time.Duration
	ClaimStale   time.Duration
	Seeker       config.SeekerConfig
	Agents       []AgentConfig
	Skip         map[string]bool
}

func LoadConfig() (Config, error) {
	cfg := Config{
		APIID:        envInt("TG_API_ID", 0),
		APIHash:      strings.TrimSpace(os.Getenv("TG_API_HASH")),
		DataDir:      strings.TrimSpace(os.Getenv("DATA_DIR")),
		DatabaseURL:  db.ResolveURL(),
		PollInterval: envDuration("SEEKER_WORKER_POLL", 30*time.Second),
		ClaimStale:   envDuration("SEEKER_CLAIM_STALE", 15*time.Minute),
		Seeker: config.SeekerConfig{
			Message:           unescape(os.Getenv("SEEKER_MESSAGE")),
			DailyLimit:        envInt("SEEKER_DAILY_LIMIT", 20),
			Delay:             envDuration("SEEKER_DELAY", 15*time.Minute),
			DelayMin:          envDuration("SEEKER_DELAY_MIN", 15*time.Minute),
			DelayMax:          envDuration("SEEKER_DELAY_MAX", 30*time.Minute),
			ExplicitlyEnabled: true,
		},
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.Seeker.DelayMin == 0 {
		cfg.Seeker.DelayMin = cfg.Seeker.Delay
	}
	if cfg.Seeker.DelayMax == 0 {
		cfg.Seeker.DelayMax = 30 * time.Minute
	}
	if cfg.Seeker.DelayMax < cfg.Seeker.DelayMin {
		cfg.Seeker.DelayMax = cfg.Seeker.DelayMin
	}

	sources := splitCSV(os.Getenv("TG_SOURCES"))
	cfg.Skip = outreach.BuildSkipList(sources, os.Getenv("TG_DESTINATION"), os.Getenv("MATCHER_BOT"))
	cfg.Agents = loadAgents()

	if cfg.APIID == 0 || cfg.APIHash == "" {
		return cfg, fmt.Errorf("TG_API_ID and TG_API_HASH are required")
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.Agents) == 0 {
		return cfg, fmt.Errorf("no seeker agents: set SEEKER_AGENTS=a1,a2 and SEEKER_AGENT_A1_PHONE/SESSION")
	}
	return cfg, nil
}

func loadAgents() []AgentConfig {
	ids := splitCSV(os.Getenv("SEEKER_AGENTS"))
	var out []AgentConfig
	for _, id := range ids {
		key := strings.ToUpper(strings.ReplaceAll(id, "-", "_"))
		phone := firstNonEmpty(
			os.Getenv("SEEKER_AGENT_"+key+"_PHONE"),
			os.Getenv("SEEKER_AGENT_"+id+"_PHONE"),
		)
		session := firstNonEmpty(
			os.Getenv("SEEKER_AGENT_"+key+"_SESSION"),
			os.Getenv("SEEKER_AGENT_"+id+"_SESSION"),
		)
		password := firstNonEmpty(
			os.Getenv("SEEKER_AGENT_"+key+"_PASSWORD"),
			os.Getenv("SEEKER_AGENT_"+id+"_PASSWORD"),
		)
		if phone == "" || session == "" {
			continue
		}
		out = append(out, AgentConfig{
			ID:       id,
			Phone:    phone,
			Session:  session,
			Password: password,
		})
	}
	if len(out) > 0 {
		return out
	}

	phone := firstNonEmpty(os.Getenv("OUTREACH_PHONE"), os.Getenv("TG_PHONE"))
	session := firstNonEmpty(os.Getenv("OUTREACH_SESSION"), os.Getenv("TG_SESSION"))
	if phone != "" && session != "" {
		out = append(out, AgentConfig{
			ID:       firstNonEmpty(os.Getenv("SEEKER_AGENT_ID"), "default"),
			Phone:    phone,
			Session:  session,
			Password: firstNonEmpty(os.Getenv("OUTREACH_AUTH_PASSWORD"), os.Getenv("TG_AUTH_PASSWORD")),
		})
	}
	return out
}

type Pool struct {
	cfg Config
	db  *db.DB
	log *zap.Logger
}

func NewPool(cfg Config, database *db.DB, log *zap.Logger) *Pool {
	return &Pool{cfg: cfg, db: database, log: log}
}

func (p *Pool) Run(ctx context.Context) error {
	errCh := make(chan error, len(p.cfg.Agents))
	for _, agent := range p.cfg.Agents {
		ag := agent
		go func() {
			errCh <- p.runAgent(ctx, ag)
		}()
		p.log.Info("seeker agent started",
			zap.String("agent", ag.ID),
			zap.String("phone", telegram.MaskPhone(ag.Phone)),
		)
	}

	var first error
	remaining := len(p.cfg.Agents)
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			remaining--
			if err != nil && err != context.Canceled && first == nil {
				first = err
			}
		}
	}
	return first
}

func (p *Pool) runAgent(ctx context.Context, agent AgentConfig) error {
	log := p.log.With(zap.String("agent", agent.ID))
	agentDir := filepath.Join(p.cfg.DataDir, "agents", agent.ID)
	seekerDir := filepath.Join(agentDir, "seeker")

	seekerCfg := p.cfg.Seeker
	seekerCfg.DataDir = seekerDir

	seekerStore, err := outreach.OpenStoreFile(seekerDir, "seeker.json")
	if err != nil {
		return fmt.Errorf("agent %s seeker store: %w", agent.ID, err)
	}
	defer seekerStore.Close()

	rateStore, err := outreach.OpenStoreFile(seekerDir, "rate.json")
	if err != nil {
		return fmt.Errorf("agent %s rate store: %w", agent.ID, err)
	}
	defer rateStore.Close()

	svc := outreach.NewService(
		agent.Phone,
		agentDir,
		p.cfg.DataDir,
		config.OutreachConfig{},
		seekerCfg,
		p.cfg.APIID,
		p.cfg.APIHash,
		nil,
		seekerStore,
		nil,
		rateStore,
		p.cfg.Skip,
		log,
	)
	svc.SetAgent(agent.ID, agent.Session, agent.Password)

	if until, ok := rateStore.PausedUntil(); ok {
		log.Warn("seeker agent paused by Telegram", zap.Time("paused_until", until))
	}

	goErr := make(chan error, 1)
	go func() {
		goErr <- svc.Connect(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-goErr:
		return err
	case <-svc.Ready():
		log.Info("seeker agent authorized")
	}

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	p.tick(ctx, agent, svc, seekerStore, rateStore, log)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-goErr:
			if err != nil && err != context.Canceled {
				return err
			}
			return nil
		case <-ticker.C:
			p.tick(ctx, agent, svc, seekerStore, rateStore, log)
		}
	}
}

func (p *Pool) tick(
	ctx context.Context,
	agent AgentConfig,
	svc *outreach.Service,
	seekerStore, rateStore *outreach.Store,
	log *zap.Logger,
) {
	if pending, err := p.db.CountPendingSeekerDMs(ctx); err == nil && pending > 0 {
		log.Info("seeker queue", zap.Int("pending", pending))
	}

	minDelay := p.cfg.Seeker.MinDelay()
	if !seekerStore.CanSendToday(p.cfg.Seeker.DailyLimit) {
		log.Info("seeker daily limit reached", zap.Int("limit", p.cfg.Seeker.DailyLimit))
		return
	}
	if !rateStore.CanSendNow(minDelay) {
		if rateStore.IsPaused() {
			log.Info("seeker paused by Telegram limit")
		} else if d, ok := rateStore.NextSendDelay(); ok {
			log.Info("seeker cooldown", zap.Duration("next_in", d))
		} else {
			log.Info("seeker cooldown", zap.Duration("delay_min", minDelay))
		}
		return
	}

	claimed, err := p.db.ClaimPendingSeekerDM(ctx, agent.ID, p.cfg.ClaimStale)
	if err != nil {
		log.Warn("claim pending seeker failed", zap.Error(err))
		return
	}
	if claimed == nil {
		return
	}

	processed := false
	defer func() {
		if !processed {
			_ = p.db.ReleaseSeekerDMClaim(ctx, claimed.SourceChannel, claimed.SourceMessageID, agent.ID)
		}
	}()

	t, ok := outreach.SeekerTarget(
		strings.TrimSpace(claimed.Body),
		claimed.PosterUsername, claimed.PosterPhone,
		claimed.AdUsername, claimed.AdPhone,
		p.cfg.Skip,
	)
	if !ok {
		if err := p.db.UpdateJobSeekerDM(ctx, claimed.SourceChannel, claimed.SourceMessageID, "none", "skipped", time.Now().UTC()); err != nil {
			log.Warn("mark seeker without contact failed", zap.Error(err))
			return
		}
		processed = true
		return
	}

	if contacted, firstSentAt, err := p.db.WasDMContacted(ctx, t.Type, t.Raw); err != nil {
		log.Warn("check dm contacted failed", zap.Error(err))
	} else if contacted {
		sentAt := time.Now().UTC()
		if firstSentAt != nil {
			sentAt = firstSentAt.UTC()
		}
		if err := p.db.UpdateJobSeekerDM(ctx, claimed.SourceChannel, claimed.SourceMessageID, t.Raw, t.Type, sentAt); err != nil {
			log.Warn("update job seeker dm failed", zap.Error(err))
			return
		}
		processed = true
		return
	}

	if seekerStore.WasContacted(t.Key) {
		contact, contactType := t.Raw, t.Type
		if rec, ok := seekerStore.Lookup(t.Key); ok {
			if rec.Type != "" {
				contactType = rec.Type
			}
			if rec.Target != "" {
				contact = rec.Target
			}
		}
		if err := p.db.UpdateJobSeekerDM(ctx, claimed.SourceChannel, claimed.SourceMessageID, contact, contactType, time.Now().UTC()); err != nil {
			log.Warn("update job seeker dm failed", zap.Error(err))
			return
		}
		processed = true
		return
	}

	target := svc.HandleSeekerPost(ctx, outreach.PostInfo{
		SourceChannel:  claimed.SourceChannel,
		SourceLink:     claimed.SourceMessageLink,
		MessageID:      claimed.SourceMessageID,
		Body:           claimed.Body,
		Text:           claimed.Body,
		PosterUsername: claimed.PosterUsername,
		PosterPhone:    claimed.PosterPhone,
		AdUsername:     claimed.AdUsername,
		AdPhone:        claimed.AdPhone,
	})
	if target == nil {
		return
	}
	if err := p.db.UpdateJobSeekerDM(ctx, claimed.SourceChannel, claimed.SourceMessageID, target.Raw, target.Type, time.Now().UTC()); err != nil {
		log.Warn("update job seeker dm failed", zap.Error(err))
		return
	}
	processed = true
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func unescape(s string) string {
	return strings.NewReplacer("\\n", "\n", "\\t", "\t").Replace(s)
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
		return n
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
