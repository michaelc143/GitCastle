package automation

import "context"

// MergeEvent describes a completed pull request merge.
type MergeEvent struct {
	RepositoryID int64
	Owner        string
	Name         string
	Number       int64
	Actor        string
	MergeCommit  string
	SourceBranch string
	TargetBranch string
}

// PushEvent describes a branch push received by the git smart-HTTP handler.
type PushEvent struct {
	RepositoryID int64
	Owner        string
	Name         string
	Actor        string
	Branch       string
	OldHash      string
	NewHash      string
}

// Handler reacts to repository events by fanning out to webhooks and CI.
type Handler interface {
	PullRequestMerged(event MergeEvent)
	PushReceived(ctx context.Context, event PushEvent)
}
