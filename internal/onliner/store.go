package onliner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	path string
	mu   sync.Mutex
	seen map[string]bool
}

func OpenStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create onliner data dir: %w", err)
	}
	s := &Store{
		path: filepath.Join(dataDir, "onliner.json"),
		seen: make(map[string]bool),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) IsSeen(topicID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[topicKey(topicID)]
}

func (s *Store) MarkSeen(topicID int) error {
	s.mu.Lock()
	s.seen[topicKey(topicID)] = true
	s.mu.Unlock()
	return s.save()
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read onliner state: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(b, &s.seen)
}

func (s *Store) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(s.seen, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write onliner state: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func topicKey(topicID int) string {
	return fmt.Sprintf("%d", topicID)
}
