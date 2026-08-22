package collab

import (
	"context"
	"fmt"
)

// SetBranchProtection creates or updates the protection rule for one branch.
func (s *Store) SetBranchProtection(ctx context.Context, repositoryID int64, protection BranchProtection) error {
	if protection.Branch == "" {
		return fmt.Errorf("branch name is required")
	}
	if protection.RequiredApprovals < 0 {
		return fmt.Errorf("required_approvals must be >= 0")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO branch_protection(repository_id, branch, required_approvals, allow_force_push)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (repository_id, branch)
		DO UPDATE SET required_approvals = EXCLUDED.required_approvals,
		              allow_force_push = EXCLUDED.allow_force_push
	`, repositoryID, protection.Branch, protection.RequiredApprovals, protection.AllowForcePush)
	if err != nil {
		return fmt.Errorf("set branch protection: %w", err)
	}
	return nil
}

func (s *Store) GetBranchProtection(ctx context.Context, repositoryID int64, branch string) (BranchProtection, error) {
	var protection BranchProtection
	err := s.Pool.QueryRow(ctx, `
		SELECT branch, required_approvals, allow_force_push
		FROM branch_protection
		WHERE repository_id = $1 AND branch = $2
	`, repositoryID, branch).Scan(&protection.Branch, &protection.RequiredApprovals, &protection.AllowForcePush)
	if err != nil {
		if isNoRows(err) {
			return BranchProtection{}, ErrNotFound
		}
		return BranchProtection{}, fmt.Errorf("get branch protection: %w", err)
	}
	return protection, nil
}

func (s *Store) DeleteBranchProtection(ctx context.Context, repositoryID int64, branch string) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM branch_protection WHERE repository_id = $1 AND branch = $2
	`, repositoryID, branch)
	if err != nil {
		return fmt.Errorf("delete branch protection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MergeCheck reports whether a pull request may merge under the target
// branch's protection rules.
type MergeCheck struct {
	Mergable          bool     `json:"mergable"`
	Blockers          []string `json:"blockers"`
	RequiredApprovals int      `json:"required_approvals"`
	CurrentApprovals  int      `json:"current_approvals"`
}

// EvaluateMerge applies target-branch protection to a PR's reviews.
func (s *Store) EvaluateMerge(ctx context.Context, repositoryID, prNumber int64) (MergeCheck, error) {
	pr, err := s.GetPullRequest(ctx, repositoryID, prNumber)
	if err != nil {
		return MergeCheck{}, err
	}
	check := MergeCheck{Blockers: []string{}}
	if pr.State != "open" {
		check.Blockers = append(check.Blockers, "pull request is not open")
		check.Mergable = false
		// Still report approval counts for context.
	}
	protection, err := s.GetBranchProtection(ctx, repositoryID, pr.TargetBranch)
	hasRule := err == nil
	if hasRule {
		check.RequiredApprovals = protection.RequiredApprovals
	}
	reviews, err := s.ListReviews(ctx, repositoryID, prNumber)
	if err != nil {
		return MergeCheck{}, err
	}
	latest := map[string]string{}
	for _, review := range reviews {
		latest[review.Reviewer] = review.Verdict
	}
	for _, verdict := range latest {
		if verdict == "approved" {
			check.CurrentApprovals++
		}
	}
	if check.CurrentApprovals < check.RequiredApprovals {
		check.Blockers = append(check.Blockers,
			fmt.Sprintf("requires %d approvals, has %d", check.RequiredApprovals, check.CurrentApprovals))
	}
	check.Mergable = len(check.Blockers) == 0
	return check, nil
}
