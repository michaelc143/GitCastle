// Package ci — executor ties the store and runner together: it claims queued
// jobs, runs them in Docker, and records results.
package ci

import (
	"context"
	"log/slog"
	"strings"
)

// Executor processes queued jobs using a Runner.
type Executor struct {
	Store  *Store
	Runner *Runner
	Logger *slog.Logger
	// RepoPath maps repository id to bare repo path on disk.
	RepoPath func(ctx context.Context, repositoryID int64) (string, error)
}

// RunOne executes a single job by id (already claimed by caller).
func (e *Executor) RunOne(ctx context.Context, repositoryID, jobID int64, commitHash, branch, command string) error {
	if err := e.Store.MarkRunning(ctx, jobID); err != nil {
		return err
	}
	barePath, err := e.RepoPath(ctx, repositoryID)
	if err != nil {
		_ = e.Store.MarkFinished(ctx, jobID, StatusFailed, "resolve repository: "+err.Error(), -1)
		return err
	}
	result, runErr := e.Runner.Run(ctx, barePath, commitHash, branch, command)
	status := StatusSuccess
	if runErr != nil || result.ExitCode != 0 {
		status = StatusFailed
	}
	output := result.Output
	if runErr != nil && !strings.HasSuffix(output, "\"") && output == "" {
		output = "runner error: " + runErr.Error()
	}
	if err := e.Store.MarkFinished(ctx, jobID, status, output, result.ExitCode); err != nil {
		return err
	}
	return runErr
}
