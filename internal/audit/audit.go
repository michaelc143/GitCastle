// Package audit provides an append-only trail of security-relevant events:
// logins, repository changes, permission grants, protection edits.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Event kinds recorded by the system.
const (
	EventLogin           = "login"
	EventLoginFailed     = "login_failed"
	EventLogout          = "logout"
	EventUserRegistered  = "user_registered"
	EventRepoCreated     = "repository_created"
	EventPermissionGrant = "permission_granted"
	EventProtectionSet   = "branch_protection_set"
	EventSecretChanged   = "secret_changed"
	EventWebhookChanged  = "webhook_changed"
	EventPRMerged        = "pull_request_merged"
)

// Entry is one immutable audit record.
type Entry struct {
	ID       int64          `json:"id"`
	Time     time.Time      `json:"time"`
	Actor    string         `json:"actor"`
	Action   string         `json:"action"`
	Target   string         `json:"target"`
	Details  map[string]any `json:"details,omitempty"`
	RemoteIP string         `json:"remote_ip,omitempty"`
}

// Store persists audit entries. Rows are never updated or deleted through
// this API.
type Store struct {
	Pool *pgxpool.Pool
}

func (s *Store) Record(ctx context.Context, actor, action, target string, details map[string]any, remoteIP string) error {
	var payload []byte
	if details != nil {
		encoded, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("encode audit details: %w", err)
		}
		payload = encoded
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO audit_log(actor, action, target, details, remote_ip)
		VALUES ($1, $2, $3, $4::jsonb, $5)
	`, actor, action, target, payload, remoteIP)
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}

// List returns the most recent entries, optionally filtered by actor or
// action. limit is capped at 500.
func (s *Store) List(ctx context.Context, actor, action string, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, occurred_at, actor, action, target, details, COALESCE(remote_ip, '')
		FROM audit_log WHERE true`
	args := []any{}
	if actor != "" {
		args = append(args, actor)
		query += fmt.Sprintf(` AND actor = $%d`, len(args))
	}
	if action != "" {
		args = append(args, action)
		query += fmt.Sprintf(` AND action = $%d`, len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY occurred_at DESC LIMIT $%d`, len(args))

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		var rawDetails []byte
		if err := rows.Scan(&entry.ID, &entry.Time, &entry.Actor, &entry.Action, &entry.Target, &rawDetails, &entry.RemoteIP); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		if len(rawDetails) > 0 {
			if err := json.Unmarshal(rawDetails, &entry.Details); err != nil {
				return nil, fmt.Errorf("decode audit details: %w", err)
			}
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
