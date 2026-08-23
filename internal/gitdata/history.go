package gitdata

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ErrNotFound is returned when a revision, path, or object does not exist.
var ErrNotFound = fmt.Errorf("not found")

func (r Repo) runBytes(ctx context.Context, args ...string) ([]byte, error) {
	command := gitCommand(ctx, r.Path, args)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Commit summarizes one commit.
type Commit struct {
	Hash    string   `json:"hash"`
	Author  string   `json:"author"`
	Email   string   `json:"email"`
	Date    string   `json:"date"`
	Message string   `json:"message"`
	Parents []string `json:"parents"`
}

const commitFormat = "%H%x00%an%x00%ae%x00%aI%x00%P%x00%B%x1e"

func parseCommits(output string) []Commit {
	commits := []Commit{}
	for _, record := range strings.Split(output, "\x1e") {
		record = strings.TrimLeft(record, "\n")
		if strings.TrimSpace(record) == "" {
			continue
		}
		parts := strings.SplitN(record, "\x00", 6)
		if len(parts) < 6 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Author:  parts[1],
			Email:   parts[2],
			Date:    parts[3],
			Message: strings.TrimRight(parts[5], "\n"),
			Parents: strings.Fields(parts[4]),
		})
	}
	return commits
}

// ListCommits returns commits newest-first. When path is non-empty only
// commits touching that path are included.
func (r Repo) ListCommits(ctx context.Context, rev, path string, limit int) ([]Commit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []string{"log", "--format=" + commitFormat, "-n", strconv.Itoa(limit)}
	if path != "" {
		args = append(args, "--", path)
	}
	output, err := r.run(ctx, args...)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") || strings.Contains(err.Error(), "bad revision") || strings.Contains(err.Error(), "does not have any commits") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return parseCommits(output), nil
}

// GetCommit returns a single commit with its diff patch.
func (r Repo) GetCommit(ctx context.Context, hash string) (Commit, string, error) {
	output, err := r.run(ctx, "show", "--format="+commitFormat, "--patch", "--no-color", hash)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") || strings.Contains(err.Error(), "bad revision") {
			return Commit{}, "", ErrNotFound
		}
		return Commit{}, "", err
	}
	header, rest, found := strings.Cut(output, "\x1e")
	if !found {
		return Commit{}, "", fmt.Errorf("unexpected show output for %s", hash)
	}
	commits := parseCommits(header + "\x1e")
	if len(commits) == 0 {
		return Commit{}, "", fmt.Errorf("could not parse commit %s", hash)
	}
	patch := strings.TrimPrefix(rest, "\n")
	return commits[0], patch, nil
}

// DefaultBranch falls back to main or master when HEAD is unborn.
func (r Repo) ResolveRev(ctx context.Context, rev string) (string, error) {
	hash, err := r.run(ctx, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimSpace(hash), nil
}
