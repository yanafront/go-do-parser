package db

import (
	"context"
	"fmt"
	"time"
)

type SeekerAgentBlock struct {
	ID          int64      `json:"id"`
	AgentID     string     `json:"agent_id"`
	Phone       string     `json:"phone"`
	Reason      string     `json:"reason"`
	Target      string     `json:"target"`
	PausedUntil *time.Time `json:"paused_until,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type SeekerAgentStatus struct {
	AgentID     string     `json:"agent_id"`
	Phone       string     `json:"phone"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason"`
	LastTarget  string     `json:"last_target"`
	PausedUntil *time.Time `json:"paused_until,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (db *DB) ListSeekerAgentStatuses(ctx context.Context) ([]SeekerAgentStatus, error) {
	rows, err := db.sql.QueryContext(ctx, `
SELECT agent_id, COALESCE(phone, ''), status, COALESCE(reason, ''), COALESCE(last_target, ''),
       paused_until, updated_at
FROM seeker_agent_status
ORDER BY
  CASE WHEN status = 'blocked' THEN 0 ELSE 1 END,
  updated_at DESC,
  agent_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list seeker agent status: %w", err)
	}
	defer rows.Close()

	var out []SeekerAgentStatus
	now := time.Now().UTC()
	for rows.Next() {
		var s SeekerAgentStatus
		if err := rows.Scan(&s.AgentID, &s.Phone, &s.Status, &s.Reason, &s.LastTarget, &s.PausedUntil, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if s.Status == "blocked" && s.PausedUntil != nil && !s.PausedUntil.After(now) {
			s.Status = "ok"
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (db *DB) ListSeekerAgentBlocks(ctx context.Context, limit int) ([]SeekerAgentBlock, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.sql.QueryContext(ctx, `
SELECT id, agent_id, COALESCE(phone, ''), reason, COALESCE(target, ''), paused_until, created_at
FROM seeker_agent_blocks
ORDER BY created_at DESC, id DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list seeker agent blocks: %w", err)
	}
	defer rows.Close()

	var out []SeekerAgentBlock
	for rows.Next() {
		var b SeekerAgentBlock
		if err := rows.Scan(&b.ID, &b.AgentID, &b.Phone, &b.Reason, &b.Target, &b.PausedUntil, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
