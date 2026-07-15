package onliner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type seenEntry struct {
	UpText string `json:"up_text,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
	seen map[string]seenEntry
}

func OpenStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create onliner data dir: %w", err)
	}
	s := &Store{
		path: filepath.Join(dataDir, "onliner.json"),
		seen: make(map[string]seenEntry),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ShouldSkip(topicID int, upText string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.seen[topicKey(topicID)]
	if !ok {
		return false
	}
	upText = strings.TrimSpace(upText)
	if upText != "" && upText != strings.TrimSpace(entry.UpText) {
		return false
	}
	if isRecentBump(upText) {
		return false
	}
	return true
}

func (s *Store) MarkSeen(topicID int, upText string) error {
	s.mu.Lock()
	s.seen[topicKey(topicID)] = seenEntry{UpText: strings.TrimSpace(upText)}
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
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range raw {
		var entry seenEntry
		if err := json.Unmarshal(v, &entry); err != nil {
			var seen bool
			if json.Unmarshal(v, &seen) == nil && seen {
				s.seen[k] = seenEntry{}
				continue
			}
			return fmt.Errorf("parse onliner state key %s: %w", k, err)
		}
		s.seen[k] = entry
	}
	return nil
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
