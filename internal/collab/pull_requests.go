package collab

import (
	"context"
	"fmt"
)

// CreatePullRequest opens a PR with the next repository-scoped number.
// Issue and PR numbers share no sequence; each table counts independently.
func (s *Store) CreatePullRequest(ctx context.Context, repositoryID int64, author, title, body, sourceBranch, targetBranch string) (PullRequest, error) {
	pr := PullRequest{
		Title: title, Body: body, Author: author, State: "open",
		SourceBranch: sourceBranch, TargetBranch: targetBranch,
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO pull_requests(repository_id, number, title, body, author, source_branch, target_branch)
		SELECT $1, COALESCE(MAX(number), 0) + 1, $2, $3, $4, $5, $6 FROM pull_requests WHERE repository_id = $1
		RETURNING number, created_at, updated_at
	`, repositoryID, title, body, author, sourceBranch, targetBranch).Scan(&pr.Number, &pr.CreatedAt, &pr.UpdatedAt)
	if err != nil {
		return PullRequest{}, fmt.Errorf("insert pull request: %w", err)
	}
	return pr, nil
}

func (s *Store) ListPullRequests(ctx context.Context, repositoryID int64, state string) ([]PullRequest, error) {
	query := `
		SELECT number, title, body, author, state, source_branch, target_branch, merge_commit, created_at, updated_at
		FROM pull_requests
		WHERE repository_id = $1`
	args := []any{repositoryID}
	if state == "open" || state == "merged" || state == "closed" {
		query += ` AND state = $2`
		args = append(args, state)
	}
	query += ` ORDER BY number DESC`
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}
	defer rows.Close()
	prs := []PullRequest{}
	for rows.Next() {
		pr, err := scanPullRequest(rows)
		if err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, rows.Err()
}

func (s *Store) GetPullRequest(ctx context.Context, repositoryID, number int64) (PullRequest, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT number, title, body, author, state, source_branch, target_branch, merge_commit, created_at, updated_at
		FROM pull_requests WHERE repository_id = $1 AND number = $2
	`, repositoryID, number)
	pr, err := scanPullRequest(row)
	if err != nil {
		return PullRequest{}, err
	}
	return pr, nil
}

// SetPullRequestState transitions a PR: open→merged (with merge commit hash)
// or open→closed. Terminal states cannot change.
func (s *Store) SetPullRequestState(ctx context.Context, repositoryID, number int64, state, mergeCommit string) (PullRequest, error) {
	if state != "merged" && state != "closed" {
		return PullRequest{}, ErrInvalidState
	}
	var pr PullRequest
	var merge *string
	if mergeCommit != "" {
		merge = &mergeCommit
	}
	err := s.Pool.QueryRow(ctx, `
		UPDATE pull_requests
		SET state = $3, merge_commit = $4, updated_at = NOW()
		WHERE repository_id = $1 AND number = $2 AND state = 'open'
		RETURNING number, title, body, author, state, source_branch, target_branch, merge_commit, created_at, updated_at
	`, repositoryID, number, state, merge).Scan(&pr.Number, &pr.Title, &pr.Body, &pr.Author, &pr.State, &pr.SourceBranch, &pr.TargetBranch, &pr.MergeCommit, &pr.CreatedAt, &pr.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			// Distinguish missing PR from non-open PR.
			if _, getErr := s.GetPullRequest(ctx, repositoryID, number); getErr == nil {
				return PullRequest{}, ErrNotMergeable
			}
			return PullRequest{}, ErrNotFound
		}
		return PullRequest{}, fmt.Errorf("update pull request state: %w", err)
	}
	return pr, nil
}

// --- reviews ---

// UpsertReview records or replaces a reviewer's verdict on a PR.
func (s *Store) UpsertReview(ctx context.Context, repositoryID, prNumber int64, reviewer, verdict, body string) (Review, error) {
	switch verdict {
	case "approved", "changes_requested", "commented":
	default:
		return Review{}, ErrInvalidState
	}
	pr, err := s.GetPullRequest(ctx, repositoryID, prNumber)
	if err != nil {
		return Review{}, err
	}
	if pr.State == "merged" || pr.State == "closed" {
		return Review{}, ErrNotMergeable
	}
	// reviews.pull_request_id references pull_requests(id); resolve the
	// human-readable number to the row id first.
	var rowID int64
	if err := s.Pool.QueryRow(ctx, `
		SELECT id FROM pull_requests WHERE repository_id = $1 AND number = $2
	`, repositoryID, prNumber).Scan(&rowID); err != nil {
		if isNoRows(err) {
			return Review{}, ErrNotFound
		}
		return Review{}, fmt.Errorf("resolve pull request id: %w", err)
	}
	var review Review
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO reviews(pull_request_id, reviewer, verdict, body)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (pull_request_id, reviewer)
		DO UPDATE SET verdict = EXCLUDED.verdict, body = EXCLUDED.body, created_at = NOW()
		RETURNING reviewer, verdict, body, created_at
	`, rowID, reviewer, verdict, body).Scan(&review.Reviewer, &review.Verdict, &review.Body, &review.CreatedAt)
	if err != nil {
		return Review{}, fmt.Errorf("upsert review: %w", err)
	}
	return review, nil
}

func (s *Store) ListReviews(ctx context.Context, repositoryID, prNumber int64) ([]Review, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.reviewer, r.verdict, r.body, r.created_at
		FROM reviews r
		JOIN pull_requests p ON p.id = r.pull_request_id
		WHERE p.repository_id = $1 AND p.number = $2
		ORDER BY r.created_at ASC
	`, repositoryID, prNumber)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()
	reviews := []Review{}
	for rows.Next() {
		var review Review
		if err := rows.Scan(&review.Reviewer, &review.Verdict, &review.Body, &review.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func scanPullRequest(row pgxRow) (PullRequest, error) {
	var pr PullRequest
	err := row.Scan(&pr.Number, &pr.Title, &pr.Body, &pr.Author, &pr.State, &pr.SourceBranch, &pr.TargetBranch, &pr.MergeCommit, &pr.CreatedAt, &pr.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return PullRequest{}, ErrNotFound
		}
		return PullRequest{}, fmt.Errorf("scan pull request: %w", err)
	}
	return pr, nil
}

type pgxRow interface{ Scan(dest ...any) error }
