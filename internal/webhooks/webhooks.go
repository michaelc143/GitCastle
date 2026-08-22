// Package webhooks delivers repository events to subscriber URLs with
// HMAC-SHA256 signatures and bounded retries.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Event types subscribers can subscribe to.
const (
	EventPush       = "push"
	EventPullMerge  = "pull_request"
	EventIssue      = "issue"
)

type Hook struct {
	ID     int64
	URL    string
	Secret string
	Events []string
}

type Delivery struct {
	ID        int64
	WebhookID int64
	EventType string
	Payload   json.RawMessage
	Status    int
	Attempts  int
	Delivered bool
}

// Payload is the envelope posted to subscriber URLs.
type Payload struct {
	EventType  string            `json:"event"`
	Repository string            `json:"repository"` // owner/name
	Actor      string            `json:"actor"`
	Timestamp  time.Time         `json:"timestamp"`
	Data       map[string]any    `json:"data,omitempty"`
}

// Store persists hooks and delivery records.
type Store struct {
	Pool *pgxpool.Pool
}

func (s *Store) CreateHook(ctx context.Context, repositoryID int64, url, secret string, events []string) (Hook, error) {
	var hook Hook
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO webhooks(repository_id, url, secret, events)
		VALUES ($1, $2, $3, $4)
		RETURNING id, url, secret, events
	`, repositoryID, url, secret, events).Scan(&hook.ID, &hook.URL, &hook.Secret, &hook.Events)
	if err != nil {
		return Hook{}, fmt.Errorf("insert webhook: %w", err)
	}
	return hook, nil
}

func (s *Store) ListHooks(ctx context.Context, repositoryID int64, eventType string) ([]Hook, error) {
	query := `SELECT id, url, secret, events FROM webhooks WHERE repository_id = $1 AND active`
	args := []any{repositoryID}
	if eventType != "" {
		query += ` AND events @> $2`
		args = append(args, []string{eventType})
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()
	hooks := []Hook{}
	for rows.Next() {
		hook := Hook{}
		if err := rows.Scan(&hook.ID, &hook.URL, &hook.Secret, &hook.Events); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		hooks = append(hooks, hook)
	}
	return hooks, rows.Err()
}

func (s *Store) DeleteHook(ctx context.Context, repositoryID, hookID int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM webhooks WHERE repository_id = $1 AND id = $2`, repositoryID, hookID)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

// RecordDelivery stores one delivery attempt.
func (s *Store) RecordDelivery(ctx context.Context, hookID int64, eventType string, payload []byte, status int) error {
	delivered := status >= 200 && status < 300
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO webhook_deliveries(webhook_id, event_type, payload, status, attempts, delivered_at)
		VALUES ($1, $2, $3, $4, 1, CASE WHEN $5 THEN NOW() ELSE NULL END)
	`, hookID, eventType, payload, status, delivered)
	if err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	return nil
}

func (s *Store) ListDeliveries(ctx context.Context, repositoryID int64) ([]Delivery, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT d.id, d.webhook_id, d.event_type, d.payload, d.status, d.attempts, d.delivered_at IS NOT NULL
		FROM webhook_deliveries d
		JOIN webhooks w ON w.id = d.webhook_id
		WHERE w.repository_id = $1
		ORDER BY d.created_at DESC LIMIT 50
	`, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()
	deliveries := []Delivery{}
	for rows.Next() {
		d := Delivery{}
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload, &d.Status, &d.Attempts, &d.Delivered); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// HookLookup lists active hooks matching an event; HookRecorder persists
// delivery attempts. Interfaces keep the dispatcher testable.
type HookLookup interface {
	ListHooks(ctx context.Context, repositoryID int64, eventType string) ([]Hook, error)
}

type HookRecorder interface {
	RecordDelivery(ctx context.Context, hookID int64, eventType string, payload []byte, status int) error
}

// Dispatcher posts events to matching hooks.
type Dispatcher struct {
	Store  interface {
		HookLookup
		HookRecorder
	}
	Client *http.Client
	Logger *slog.Logger
	MaxRetries int
}

// Dispatch fans an event out to all matching active hooks. Delivery happens
// synchronously here; a production system would queue this.
func (d *Dispatcher) Dispatch(ctx context.Context, repositoryID int64, eventType, repoSlug, actor string, data map[string]any) {
	hooks, err := d.Store.ListHooks(ctx, repositoryID, eventType)
	if err != nil {
		d.log("list hooks failed", err)
		return
	}
	payload, err := json.Marshal(Payload{
		EventType:  eventType,
		Repository: repoSlug,
		Actor:      actor,
		Timestamp:  time.Now().UTC(),
		Data:       data,
	})
	if err != nil {
		d.log("marshal payload", err)
		return
	}

	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	maxRetries := d.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for _, hook := range hooks {
		status := d.deliverWithRetries(ctx, client, hook, payload, maxRetries)
		if err := d.Store.RecordDelivery(ctx, hook.ID, eventType, payload, status); err != nil {
			d.log("record delivery", err)
		}
	}
}

func (d *Dispatcher) deliverWithRetries(ctx context.Context, client *http.Client, hook Hook, payload []byte, maxRetries int) int {
	status := 0
	for attempt := 0; attempt < maxRetries; attempt++ {
		status = d.deliverOnce(ctx, client, hook, payload)
		if status >= 200 && status < 300 {
			return status
		}
		select {
		case <-ctx.Done():
			return status
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return status
}

func (d *Dispatcher) deliverOnce(ctx context.Context, client *http.Client, hook Hook, payload []byte) int {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(payload))
	if err != nil {
		d.log("build request", err)
		return 0
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitCastle-Event", eventTypeHeader(hook))
	signature := Sign(hook.Secret, payload)
	request.Header.Set("X-GitCastle-Signature", signature)

	response, err := client.Do(request)
	if err != nil {
		d.log("deliver to "+hook.URL, err)
		return 0
	}
	defer response.Body.Close()
	return response.StatusCode
}

// eventTypeHeader returns the event name for the header; kept simple since
// the payload already carries it.
func eventTypeHeader(hook Hook) string {
	if len(hook.Events) > 0 {
		return hook.Events[0]
	}
	return ""
}

// Sign computes the HMAC-SHA256 signature for a payload.
func Sign(secret string, payload []byte) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature; empty secrets always pass (unsigned hooks).
func Verify(secret string, payload []byte, signature string) bool {
	if secret == "" {
		return true
	}
	return hmac.Equal([]byte(Sign(secret, payload)), []byte(signature))
}

func (d *Dispatcher) log(message string, err error) {
	if d.Logger != nil {
		d.Logger.Error(message, "error", err)
	}
}
