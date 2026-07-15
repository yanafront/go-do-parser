package onliner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anadubesko/go-do-parser/internal/db"
	"github.com/anadubesko/go-do-parser/internal/filter"
	"go.uber.org/zap"
)

type Config struct {
	ForumID         int
	ForumPages      int
	SearchPages     int
	SearchQueries   []string
	RequestDelay    time.Duration
	MaxTopicAgeDays int
}

type Scraper struct {
	cfg    Config
	client *Client
	store  *Store
	db     *db.DB
	log    *zap.Logger
}

func NewScraper(cfg Config, store *Store, database *db.DB, log *zap.Logger) *Scraper {
	if cfg.ForumID == 0 {
		cfg.ForumID = 34
	}
	if cfg.ForumPages <= 0 {
		cfg.ForumPages = 5
	}
	if cfg.SearchPages <= 0 {
		cfg.SearchPages = 3
	}
	if len(cfg.SearchQueries) == 0 {
		cfg.SearchQueries = []string{"ищу подработку", "ищу работу"}
	}
	if cfg.RequestDelay <= 0 {
		cfg.RequestDelay = 400 * time.Millisecond
	}
	return &Scraper{
		cfg:    cfg,
		client: NewClient(),
		store:  store,
		db:     database,
		log:    log,
	}
}

func (s *Scraper) SyncOnce(ctx context.Context) error {
	if err := s.syncPendingSeekerDMs(ctx); err != nil {
		s.log.Warn("onliner dm backlog sync failed", zap.Error(err))
	}

	refs, err := s.collectTopicRefs()
	if err != nil {
		return err
	}
	s.log.Info("onliner topics discovered", zap.Int("count", len(refs)))

	saved := 0
	for _, ref := range refs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.store.ShouldSkip(ref.ID, ref.UpText) {
			continue
		}
		if savedTopic, err := s.processTopic(ctx, ref); err != nil {
			s.log.Warn("onliner topic failed",
				zap.Int("topic_id", ref.ID),
				zap.Error(err),
			)
			continue
		} else if savedTopic {
			saved++
		}
		time.Sleep(s.cfg.RequestDelay)
	}
	s.log.Info("onliner sync done", zap.Int("saved", saved))
	return nil
}

func (s *Scraper) collectTopicRefs() ([]TopicRef, error) {
	seen := make(map[int]TopicRef)

	for _, query := range s.cfg.SearchQueries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		searchRefs, err := s.client.FetchSearch(query, s.cfg.SearchPages)
		if err != nil {
			return nil, fmt.Errorf("fetch search %q: %w", query, err)
		}
		for _, ref := range searchRefs {
			seen[ref.ID] = ref
		}
		time.Sleep(s.cfg.RequestDelay)
	}

	forumRefs, err := s.client.FetchForum(s.cfg.ForumID, 0, s.cfg.ForumPages)
	if err != nil {
		return nil, fmt.Errorf("fetch forum: %w", err)
	}
	for _, ref := range forumRefs {
		if existing, ok := seen[ref.ID]; ok {
			if ref.Title != "" && existing.Title == "" {
				existing.Title = ref.Title
			}
			if ref.Description != "" && existing.Description == "" {
				existing.Description = ref.Description
			}
			if ref.UpText != "" && existing.UpText == "" {
				existing.UpText = ref.UpText
			}
			if ref.PosterUserID != "" && existing.PosterUserID == "" {
				existing.PosterUserID = ref.PosterUserID
			}
			if ref.PosterUsername != "" && existing.PosterUsername == "" {
				existing.PosterUsername = ref.PosterUsername
			}
			if ref.PosterProfileURL != "" && existing.PosterProfileURL == "" {
				existing.PosterProfileURL = ref.PosterProfileURL
			}
			seen[ref.ID] = existing
			continue
		}
		seen[ref.ID] = ref
	}

	return sortRefsByIDDesc(seen), nil
}

func (s *Scraper) processTopic(ctx context.Context, ref TopicRef) (bool, error) {
	topic, err := s.client.FetchTopic(ref.ID)
	if err != nil {
		if strings.Contains(err.Error(), "empty body") && (strings.TrimSpace(ref.Description) != "" || strings.TrimSpace(ref.Title) != "") {
			topic = Topic{
				ID:               ref.ID,
				Title:            ref.Title,
				Body:             strings.TrimSpace(ref.Description),
				PosterUserID:     ref.PosterUserID,
				PosterUsername:   ref.PosterUsername,
				PosterProfileURL: ref.PosterProfileURL,
				Link:             fmt.Sprintf("%s/viewtopic.php?t=%d", baseURL, ref.ID),
				PostedAt:         nil,
			}
		} else {
			return false, err
		}
	}
	if ref.Title != "" && topic.Title == "" {
		topic.Title = ref.Title
	}
	if strings.TrimSpace(topic.Body) == "" && strings.TrimSpace(ref.Description) != "" {
		topic.Body = strings.TrimSpace(ref.Description)
	}
	if topic.PosterUserID == "" && ref.PosterUserID != "" {
		topic.PosterUserID = ref.PosterUserID
	}
	if topic.PosterUsername == "" && ref.PosterUsername != "" {
		topic.PosterUsername = ref.PosterUsername
	}
	if topic.PosterProfileURL == "" && ref.PosterProfileURL != "" {
		topic.PosterProfileURL = ref.PosterProfileURL
	}

	text := topicSearchText(topic)
	if !filter.IsJobSeekerText(text) {
		if err := s.store.MarkSeen(ref.ID, ref.UpText); err != nil {
			return false, err
		}
		return false, nil
	}

	if s.cfg.MaxTopicAgeDays > 0 && topic.PostedAt != nil {
		cutoff := time.Now().AddDate(0, 0, -s.cfg.MaxTopicAgeDays)
		if topic.PostedAt.Before(cutoff) {
			if err := s.store.MarkSeen(ref.ID, ref.UpText); err != nil {
				return false, err
			}
			return false, nil
		}
	}

	contacts := ExtractContacts(topicSearchText(topic))

	if err := s.db.SaveOnlinerPost(ctx, db.OnlinerPost{
		TopicID:          topic.ID,
		TopicURL:         topic.Link,
		Title:            topic.Title,
		Body:             topic.Body,
		PosterUserID:     topic.PosterUserID,
		PosterUsername:   topic.PosterUsername,
		PosterProfileURL: topic.PosterProfileURL,
		Phone:            contacts.Phone,
		Email:            contacts.Email,
		Telegram:         contacts.Telegram,
		PostedAt:         topic.PostedAt,
	}); err != nil {
		return false, err
	}

	if err := s.enqueueSeekerDM(ctx, topic, contacts); err != nil {
		return false, err
	}

	if err := s.store.MarkSeen(topic.ID, ref.UpText); err != nil {
		return false, err
	}

	s.log.Info("onliner post saved",
		zap.Int("topic_id", topic.ID),
		zap.String("link", topic.Link),
	)
	return true, nil
}

func (s *Scraper) enqueueSeekerDM(ctx context.Context, topic Topic, contacts Contacts) error {
	if strings.TrimSpace(contacts.Phone) == "" && strings.TrimSpace(contacts.Telegram) == "" {
		return nil
	}
	return s.db.SaveJobSeekerPost(ctx, db.JobSeekerPost{
		SourceChannel:     fmt.Sprintf("onliner:%d", s.cfg.ForumID),
		SourceMessageID:     topic.ID,
		SourceMessageLink:   topic.Link,
		Body:                topicSearchText(topic),
		PosterUsername:      topic.PosterUsername,
		AdUsername:          contacts.Telegram,
		AdPhone:             contacts.Phone,
	})
}

func (s *Scraper) syncPendingSeekerDMs(ctx context.Context) error {
	posts, err := s.db.ListOnlinerPostsPendingDM(ctx, s.cfg.ForumID, 100)
	if err != nil {
		return err
	}
	for _, p := range posts {
		if err := s.db.SaveJobSeekerPost(ctx, db.JobSeekerPost{
			SourceChannel:     fmt.Sprintf("onliner:%d", s.cfg.ForumID),
			SourceMessageID:     p.TopicID,
			SourceMessageLink:   p.TopicURL,
			Body:                strings.TrimSpace(p.Title + "\n" + p.Body),
			PosterUsername:      p.PosterUsername,
			AdUsername:          p.Telegram,
			AdPhone:             p.Phone,
		}); err != nil {
			return err
		}
	}
	if len(posts) > 0 {
		s.log.Info("onliner dm backlog synced", zap.Int("count", len(posts)))
	}
	return nil
}
