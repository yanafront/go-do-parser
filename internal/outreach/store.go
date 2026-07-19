package outreach

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Record struct {
	Target  string `json:"target"`
	Type    string `json:"type"`
	SentAt  string `json:"sent_at"`
	Source  string `json:"source"`
	Message int    `json:"message_id"`
}

type Store struct {
	path string
	mu   sync.Mutex
	data diskState
}

type diskState struct {
	Contacted   map[string]Record `json:"contacted"`
	Daily       map[string]int    `json:"daily"`
	LastSentAt  string            `json:"last_sent_at"`
	NextSendAt  string            `json:"next_send_at,omitempty"`
	PausedUntil string            `json:"paused_until,omitempty"`
}

func OpenStore(dataDir string) (*Store, error) {
	return openStore(dataDir, "outreach.json")
}

func OpenStoreFile(dataDir, filename string) (*Store, error) {
	return openStore(dataDir, filename)
}

func openStore(dataDir, filename string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		path: filepath.Join(dataDir, filename),
		data: diskState{
			Contacted: make(map[string]Record),
			Daily:     make(map[string]int),
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.Unmarshal(b, &s.data); err != nil {
		return err
	}
	s.ensure()
	return nil
}

func (s *Store) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) ensure() {
	if s.data.Contacted == nil {
		s.data.Contacted = make(map[string]Record)
	}
	if s.data.Daily == nil {
		s.data.Daily = make(map[string]int)
	}
}

func todayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

func (s *Store) WasContacted(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	_, ok := s.data.Contacted[key]
	return ok
}

func (s *Store) Lookup(key string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	rec, ok := s.data.Contacted[key]
	return rec, ok
}

func (s *Store) DailySent() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	return s.data.Daily[todayKey()]
}

func (s *Store) CanSendToday(limit int) bool {
	return s.DailySent() < limit
}

func (s *Store) CanSendNow(minDelay time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	now := time.Now()
	if s.data.PausedUntil != "" {
		if t, err := time.Parse(time.RFC3339, s.data.PausedUntil); err == nil && now.Before(t) {
			return false
		}
	}
	if s.data.NextSendAt != "" {
		if t, err := time.Parse(time.RFC3339, s.data.NextSendAt); err == nil {
			return !now.Before(t)
		}
	}
	if s.data.LastSentAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, s.data.LastSentAt)
	if err != nil {
		return true
	}
	return now.Sub(t) >= minDelay
}

func (s *Store) NextSendDelay() (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if s.data.NextSendAt == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, s.data.NextSendAt)
	if err != nil {
		return 0, false
	}
	d := time.Until(t)
	if d < 0 {
		return 0, false
	}
	return d, true
}

func (s *Store) IsPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if s.data.PausedUntil == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, s.data.PausedUntil)
	if err != nil {
		return false
	}
	return time.Now().Before(t)
}

func (s *Store) PausedUntil() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if s.data.PausedUntil == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s.data.PausedUntil)
	if err != nil {
		return time.Time{}, false
	}
	if !time.Now().Before(t) {
		return time.Time{}, false
	}
	return t, true
}

func (s *Store) PauseUntil(until time.Time) error {
	s.mu.Lock()
	s.ensure()
	s.data.PausedUntil = until.UTC().Format(time.RFC3339)
	s.mu.Unlock()
	return s.save()
}

func (s *Store) MarkSent(key string, rec Record) error {
	s.mu.Lock()
	s.ensure()
	s.data.Contacted[key] = rec
	day := todayKey()
	s.data.Daily[day]++
	now := time.Now().UTC()
	s.data.LastSentAt = now.Format(time.RFC3339)
	s.mu.Unlock()
	return s.save()
}

func (s *Store) ScheduleNext(minDelay, maxDelay time.Duration) error {
	if minDelay < 0 {
		minDelay = 0
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	delay := minDelay
	if maxDelay > minDelay {
		n := int64(maxDelay - minDelay)
		delay = minDelay + time.Duration(rand.Int63n(n+1))
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.ensure()
	s.data.LastSentAt = now.Format(time.RFC3339)
	s.data.NextSendAt = now.Add(delay).Format(time.RFC3339)
	s.mu.Unlock()
	return s.save()
}

func (s *Store) MarkSkipped(key string, rec Record) error {
	s.mu.Lock()
	s.ensure()
	s.data.Contacted[key] = rec
	s.mu.Unlock()
	return s.save()
}

func (s *Store) TouchLastSent() error {
	s.mu.Lock()
	s.ensure()
	s.data.LastSentAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Unlock()
	return s.save()
}

func (s *Store) Close() error {
	return s.save()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Stats() string {
	return fmt.Sprintf("contacted=%d today=%d", len(s.data.Contacted), s.DailySent())
}
