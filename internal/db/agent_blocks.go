package db

import (
	"context"
	"fmt"
	"strings"
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

func (db *DB) RecordSeekerAgentBlock(ctx context.Context, agentID, phone, reason, target string, pausedUntil time.Time) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "default"
	}
	phone = strings.TrimSpace(phone)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "PEER_FLOOD"
	}
	target = strings.TrimSpace(target)
	var until any
	if !pausedUntil.IsZero() {
		until = pausedUntil.UTC()
	}

	_, err := db.sql.ExecContext(ctx, `
INSERT INTO seeker_agent_blocks (agent_id, phone, reason, target, paused_until)
VALUES ($1, $2, $3, $4, $5)
`, agentID, phone, reason, target, until)
	if err != nil {
		return fmt.Errorf("insert seeker agent block: %w", err)
	}

	_, err = db.sql.ExecContext(ctx, `
INSERT INTO seeker_agent_status (agent_id, phone, status, reason, last_target, paused_until, updated_at)
VALUES ($1, $2, 'blocked', $3, $4, $5, NOW())
ON CONFLICT (agent_id) DO UPDATE SET
  phone = EXCLUDED.phone,
  status = 'blocked',
  reason = EXCLUDED.reason,
  last_target = EXCLUDED.last_target,
  paused_until = EXCLUDED.paused_until,
  updated_at = NOW()
`, agentID, phone, reason, target, until)
	if err != nil {
		return fmt.Errorf("upsert seeker agent status: %w", err)
	}
	return nil
}

func (db *DB) SyncSeekerAgentPause(ctx context.Context, agentID, phone, reason string, pausedUntil time.Time) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "default"
	}
	if strings.TrimSpace(reason) == "" {
		reason = "PEER_FLOOD"
	}
	var until any
	if !pausedUntil.IsZero() {
		until = pausedUntil.UTC()
	}
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO seeker_agent_status (agent_id, phone, status, reason, last_target, paused_until, updated_at)
VALUES ($1, $2, 'blocked', $3, '', $4, NOW())
ON CONFLICT (agent_id) DO UPDATE SET
  phone = EXCLUDED.phone,
  status = 'blocked',
  reason = EXCLUDED.reason,
  paused_until = EXCLUDED.paused_until,
  updated_at = NOW()
`, agentID, strings.TrimSpace(phone), reason, until)
	if err != nil {
		return fmt.Errorf("sync seeker agent pause: %w", err)
	}
	return nil
}

func (db *DB) ClearSeekerAgentBlock(ctx context.Context, agentID, phone string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "default"
	}
	phone = strings.TrimSpace(phone)
	_, err := db.sql.ExecContext(ctx, `
INSERT INTO seeker_agent_status (agent_id, phone, status, reason, last_target, paused_until, updated_at)
VALUES ($1, $2, 'ok', '', '', NULL, NOW())
ON CONFLICT (agent_id) DO UPDATE SET
  phone = EXCLUDED.phone,
  status = 'ok',
  reason = '',
  last_target = '',
  paused_until = NULL,
  updated_at = NOW()
`, agentID, phone)
	if err != nil {
		return fmt.Errorf("clear seeker agent status: %w", err)
	}
	return nil
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
	for rows.Next() {
		var s SeekerAgentStatus
		if err := rows.Scan(&s.AgentID, &s.Phone, &s.Status, &s.Reason, &s.LastTarget, &s.PausedUntil, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if s.PausedUntil != nil && !s.PausedUntil.After(time.Now().UTC()) && s.Status == "blocked" {
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
