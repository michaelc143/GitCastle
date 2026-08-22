// Package automation fans repository events out to webhooks and the CI
// pipeline. It implements httpapi's Automation interface.
package automation

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/michaelc143/gitcastle/internal/ci"
	"github.com/michaelc143/gitcastle/internal/webhooks"
)

// Config wires the event fan-out.
type Config struct {
	WebhookStore interface {
		webhooks.HookLookup
		webhooks.HookRecorder
	}
	Dispatcher *webhooks.Dispatcher
	CIStore    *ci.Store
	Executor   *ci.Executor
	Logger     *slog.Logger
	// RepoPath resolves a repository id to its bare path.
	RepoPath      func(ctx context.Context, repositoryID int64) (string, error)
	InternalToken string
	ServerURL     string
}

func (c *Config) log(message string, err error) {
	if c.Logger != nil {
		c.Logger.Error(message, "error", err)
	}
}

// PullRequestMerged notifies webhooks and queues CI for the merged commit.
func (c *Config) PullRequestMerged(event MergeEvent) {
	ctx := context.Background()
	if c.Dispatcher != nil && c.WebhookStore != nil {
		c.Dispatcher.Dispatch(ctx, event.RepositoryID, webhooks.EventPullMerge,
			fmt.Sprintf("%s/%s", event.Owner, event.Name), event.Actor, map[string]any{
				"action":       "merged",
				"number":       event.Number,
				"merge_commit": event.MergeCommit,
			})
	}
	c.queueJob(ctx, event.RepositoryID, event.MergeCommit, event.TargetBranch, "pull_request",
		fmt.Sprintf("%d", event.Number))
}

// PushReceived reacts to a git push: webhook fan-out plus CI trigger.
func (c *Config) PushReceived(ctx context.Context, event PushEvent) {
	if c.Dispatcher != nil && c.WebhookStore != nil {
		c.Dispatcher.Dispatch(ctx, event.RepositoryID, webhooks.EventPush,
			fmt.Sprintf("%s/%s", event.Owner, event.Name), event.Actor, map[string]any{
				"branch": event.Branch,
				"after":  event.NewHash,
				"before": event.OldHash,
			})
	}
	if event.NewHash == "" || event.NewHash == "0000000000000000000000000000000000000000" {
		return // branch deletion
	}
	c.queueJob(ctx, event.RepositoryID, event.NewHash, event.Branch, "push", "")
}

// queueJob creates a CI job for the commit when CI is configured. The build
// command comes from .gitcastle.yml in the repo; jobs without config are
// skipped entirely (no-op builds are noise).
func (c *Config) queueJob(ctx context.Context, repositoryID int64, commitHash, branch, kind, ref string) {
	if c.CIStore == nil || c.Executor == nil || c.RepoPath == nil {
		return
	}
	barePath, err := c.RepoPath(ctx, repositoryID)
	if err != nil {
		c.log("resolve repo for CI", err)
		return
	}
	command := readBuildCommand(barePath, commitHash)
	if command == "" {
		return // no CI config in this repo
	}
	job, err := c.CIStore.CreateJob(ctx, repositoryID, commitHash, branch, kind, ref)
	if err != nil {
		c.log("create job", err)
		return
	}
	go func() {
		if err := c.Executor.RunOne(context.Background(), repositoryID, job.ID, commitHash, branch, command); err != nil {
			c.log("run job "+fmt.Sprint(job.ID), err)
		}
	}()
}

// readBuildCommand reads .gitcastle.yml's simple `run:` line from the repo.
// Deliberately minimal: one shell command, no plugin surface.
func readBuildCommand(barePath, commitHash string) string {
	output, err := exec.Command("git", "--git-dir", barePath, "show",
		commitHash+":.gitcastle.yml").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "run:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "run:"))
		}
	}
	return ""
}

