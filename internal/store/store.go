package store

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
	data diskState
}

type diskState struct {
	Channels  map[string]int            `json:"channels"`
	Published map[string]map[string]int `json:"published"`
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	s := &Store{
		path: filepath.Join(dataDir, "state.json"),
		data: diskState{
			Channels:  make(map[string]int),
			Published: make(map[string]map[string]int),
		},
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.save()
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(b, &s.data)
}

func (s *Store) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *Store) LastMessageID(channelKey string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Channels[channelKey], nil
}

func (s *Store) SetLastMessageID(channelKey string, messageID int) error {
	s.mu.Lock()
	s.data.Channels[channelKey] = messageID
	s.mu.Unlock()
	return s.save()
}

func (s *Store) IsPublished(sourceChannel string, messageID int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.data.Published[sourceChannel]
	if ch == nil {
		return false, nil
	}
	_, ok := ch[fmt.Sprintf("%d", messageID)]
	return ok, nil
}

func (s *Store) MarkPublished(sourceChannel string, messageID, destMessageID int) error {
	s.mu.Lock()
	if s.data.Published[sourceChannel] == nil {
		s.data.Published[sourceChannel] = make(map[string]int)
	}
	s.data.Published[sourceChannel][fmt.Sprintf("%d", messageID)] = destMessageID
	s.mu.Unlock()
	return s.save()
}
