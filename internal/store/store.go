package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "state.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS channel_state (
			channel_key TEXT PRIMARY KEY,
			last_message_id INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS published (
			source_channel TEXT NOT NULL,
			message_id INTEGER NOT NULL,
			dest_message_id INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (source_channel, message_id)
		);
	`)
	return err
}

func (s *Store) LastMessageID(channelKey string) (int, error) {
	var id int
	err := s.db.QueryRow(
		`SELECT last_message_id FROM channel_state WHERE channel_key = ?`,
		channelKey,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

func (s *Store) SetLastMessageID(channelKey string, messageID int) error {
	_, err := s.db.Exec(`
		INSERT INTO channel_state (channel_key, last_message_id) VALUES (?, ?)
		ON CONFLICT(channel_key) DO UPDATE SET last_message_id = excluded.last_message_id
	`, channelKey, messageID)
	return err
}

func (s *Store) IsPublished(sourceChannel string, messageID int) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM published WHERE source_channel = ? AND message_id = ?`,
		sourceChannel, messageID,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) MarkPublished(sourceChannel string, messageID, destMessageID int) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO published (source_channel, message_id, dest_message_id)
		VALUES (?, ?, ?)
	`, sourceChannel, messageID, destMessageID)
	return err
}
