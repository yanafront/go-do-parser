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
		cfg.ForumPages = 2
	}
	if cfg.SearchPages <= 0 {
		cfg.SearchPages = 1
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
		if s.store.IsSeen(ref.ID) {
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
		if existing, ok := seen[ref.ID]; ok && ref.Title != "" && existing.Title == "" {
			existing.Title = ref.Title
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
		return false, err
	}
	if ref.Title != "" && topic.Title == "" {
		topic.Title = ref.Title
	}

	text := topicSearchText(topic)
	if !filter.IsJobSeekerText(text) {
		if err := s.store.MarkSeen(ref.ID); err != nil {
			return false, err
		}
		return false, nil
	}

	if s.cfg.MaxTopicAgeDays > 0 && topic.PostedAt != nil {
		cutoff := time.Now().AddDate(0, 0, -s.cfg.MaxTopicAgeDays)
		if topic.PostedAt.Before(cutoff) {
			if err := s.store.MarkSeen(ref.ID); err != nil {
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

	if err := s.store.MarkSeen(topic.ID); err != nil {
		return false, err
	}

	s.log.Info("onliner post saved",
		zap.Int("topic_id", topic.ID),
		zap.String("link", topic.Link),
	)
	return true, nil
}
