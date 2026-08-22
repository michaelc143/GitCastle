// Package ci runs build jobs in isolated Docker containers and tracks their
// lifecycle: queued → running → success | failed.
package ci

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

type Job struct {
	ID          int64      `json:"id"`
	CommitHash  string     `json:"commit_hash"`
	Branch      string     `json:"branch"`
	TriggerKind string     `json:"trigger_kind"`
	TriggerRef  string     `json:"trigger_ref"`
	Status      string     `json:"status"`
	Output      string     `json:"output"`
	ExitCode    *int       `json:"exit_code"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

// Store persists build jobs.
type Store struct {
	Pool *pgxpool.Pool
}

func (s *Store) CreateJob(ctx context.Context, repositoryID int64, commitHash, branch, triggerKind, triggerRef string) (Job, error) {
	job := Job{CommitHash: commitHash, Branch: branch, TriggerKind: triggerKind, TriggerRef: triggerRef, Status: StatusQueued}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO build_jobs(repository_id, commit_hash, branch, trigger_kind, trigger_ref)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status, created_at
	`, repositoryID, commitHash, branch, triggerKind, triggerRef).Scan(&job.ID, &job.Status, &job.CreatedAt)
	if err != nil {
		return Job{}, fmt.Errorf("insert build job: %w", err)
	}
	return job, nil
}

func (s *Store) ListJobs(ctx context.Context, repositoryID int64, limit int) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, commit_hash, branch, trigger_kind, trigger_ref, status, output, exit_code, created_at, started_at, finished_at
		FROM build_jobs WHERE repository_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, repositoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("list build jobs: %w", err)
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.CommitHash, &job.Branch, &job.TriggerKind, &job.TriggerRef,
			&job.Status, &job.Output, &job.ExitCode, &job.CreatedAt, &job.StartedAt, &job.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) MarkRunning(ctx context.Context, jobID int64) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE build_jobs SET status = 'running', started_at = NOW() WHERE id = $1`, jobID)
	return err
}

func (s *Store) MarkFinished(ctx context.Context, jobID int64, status string, output string, exitCode int) error {
	if status != StatusSuccess && status != StatusFailed {
		return fmt.Errorf("invalid final status %q", status)
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE build_jobs SET status = $2, output = $3, exit_code = $4, finished_at = NOW()
		WHERE id = $1`, jobID, status, output, exitCode)
	return err
}
