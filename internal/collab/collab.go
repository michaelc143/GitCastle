// Package collab implements Phase 3 collaboration: issues, comments, pull
// requests, reviews, and branch protection rules.
package collab

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidState   = errors.New("invalid state transition")
	ErrNotMergeable   = errors.New("pull request is not open")
	ErrProtected      = errors.New("branch is protected")
	ErrReviewRequired = errors.New("required approvals missing")
)

type Issue struct {
	Number    int64     `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type PullRequest struct {
	Number        int64     `json:"number"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	Author        string    `json:"author"`
	State         string    `json:"state"` // open | merged | closed
	SourceBranch  string    `json:"source_branch"`
	TargetBranch  string    `json:"target_branch"`
	MergeCommit   *string   `json:"merge_commit,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Review struct {
	Reviewer string    `json:"reviewer"`
	Verdict  string    `json:"verdict"` // approved | changes_requested | commented
	Body     string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type BranchProtection struct {
	Branch           string `json:"branch"`
	RequiredApprovals int   `json:"required_approvals"`
	AllowForcePush   bool   `json:"allow_force_push"`
}

// Store persists collaboration data in PostgreSQL.
type Store struct {
	Pool *pgxpool.Pool
}

// NextNumber allocates the next issue or PR number for a repository within
// one transaction so concurrent creations cannot collide.
func (s *Store) NextNumber(ctx context.Context, repositoryID int64, table string) (int64, error) {
	if table != "issues" && table != "pull_requests" {
		return 0, fmt.Errorf("unknown number sequence table %q", table)
	}
	var number int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(number), 0) + 1 FROM `+table+` WHERE repository_id = $1
	`, repositoryID).Scan(&number)
	if err != nil {
		return 0, fmt.Errorf("allocate number: %w", err)
	}
	return number, nil
}
