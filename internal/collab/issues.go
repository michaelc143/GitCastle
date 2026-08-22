package collab

import (
	"context"
	"fmt"
)

// CreateIssue opens a new issue with the next repository-scoped number.
func (s *Store) CreateIssue(ctx context.Context, repositoryID int64, author, title, body string) (Issue, error) {
	issue := Issue{Title: title, Body: body, Author: author, State: "open"}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO issues(repository_id, number, title, body, author)
		SELECT $1, COALESCE(MAX(number), 0) + 1, $2, $3, $4 FROM issues WHERE repository_id = $1
		RETURNING number, created_at, updated_at
	`, repositoryID, title, body, author).Scan(&issue.Number, &issue.CreatedAt, &issue.UpdatedAt)
	if err != nil {
		return Issue{}, fmt.Errorf("insert issue: %w", err)
	}
	return issue, nil
}

func (s *Store) ListIssues(ctx context.Context, repositoryID int64, state string) ([]Issue, error) {
	query := `
		SELECT number, title, body, author, state, created_at, updated_at
		FROM issues
		WHERE repository_id = $1`
	args := []any{repositoryID}
	if state == "open" || state == "closed" {
		query += ` AND state = $2`
		args = append(args, state)
	}
	query += ` ORDER BY number DESC`
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()
	issues := []Issue{}
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.Number, &issue.Title, &issue.Body, &issue.Author, &issue.State, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (s *Store) GetIssue(ctx context.Context, repositoryID, number int64) (Issue, error) {
	var issue Issue
	err := s.Pool.QueryRow(ctx, `
		SELECT number, title, body, author, state, created_at, updated_at
		FROM issues WHERE repository_id = $1 AND number = $2
	`, repositoryID, number).Scan(&issue.Number, &issue.Title, &issue.Body, &issue.Author, &issue.State, &issue.CreatedAt, &issue.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return Issue{}, ErrNotFound
		}
		return Issue{}, fmt.Errorf("get issue: %w", err)
	}
	return issue, nil
}

// SetIssueState transitions an issue between open and closed.
func (s *Store) SetIssueState(ctx context.Context, repositoryID, number int64, state string) (Issue, error) {
	if state != "open" && state != "closed" {
		return Issue{}, ErrInvalidState
	}
	var issue Issue
	err := s.Pool.QueryRow(ctx, `
		UPDATE issues SET state = $3, updated_at = NOW()
		WHERE repository_id = $1 AND number = $2
		RETURNING number, title, body, author, state, created_at, updated_at
	`, repositoryID, number, state).Scan(&issue.Number, &issue.Title, &issue.Body, &issue.Author, &issue.State, &issue.CreatedAt, &issue.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return Issue{}, ErrNotFound
		}
		return Issue{}, fmt.Errorf("update issue state: %w", err)
	}
	return issue, nil
}

// --- comments ---

func (s *Store) AddComment(ctx context.Context, repositoryID int64, subjectType string, subjectNumber int64, author, body string) (Comment, error) {
	if subjectType != "issue" && subjectType != "pull_request" {
		return Comment{}, ErrInvalidState
	}
	var comment Comment
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO comments(repository_id, subject_type, subject_number, author, body)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, author, body, created_at
	`, repositoryID, subjectType, subjectNumber, author, body).Scan(&comment.ID, &comment.Author, &comment.Body, &comment.CreatedAt)
	if err != nil {
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	return comment, nil
}

func (s *Store) ListComments(ctx context.Context, repositoryID int64, subjectType string, subjectNumber int64) ([]Comment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, author, body, created_at
		FROM comments
		WHERE repository_id = $1 AND subject_type = $2 AND subject_number = $3
		ORDER BY created_at ASC, id ASC
	`, repositoryID, subjectType, subjectNumber)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.Author, &comment.Body, &comment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func isNoRows(err error) bool {
	return err.Error() == "no rows in result set"
}
