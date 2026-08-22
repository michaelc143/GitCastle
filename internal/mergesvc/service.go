// Package mergesvc performs real git merges for pull requests: it clones the
// bare repository to a scratch worktree, merges the source branch, and pushes
// the result back. Emits events on success.
package mergesvc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Event describes a successful merge for downstream automation.
type Event struct {
	RepositoryID   int64
	Owner          string
	Name           string
	PullNumber     int64
	MergeCommit    string
	TargetBranch   string
	SourceBranch   string
	MergedBy       string
}

// Service executes merges against repositories under Root.
type Service struct {
	Root     string // repository root; repos live at {root}/{owner}/{name}.git
	Events   chan<- Event
	Committer struct {
		Name  string
		Email string
	}
}

func (s *Service) git(ctx context.Context, dir string, args ...string) (string, error) {
	committerName := s.Committer.Name
	if committerName == "" {
		committerName = "GitCastle"
	}
	committerEmail := s.Committer.Email
	if committerEmail == "" {
		committerEmail = "noreply@gitcastle.local"
	}
	command := exec.CommandContext(ctx, "git", "-C", dir)
	command.Args = append(command.Args, args...)
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+committerName,
		"GIT_AUTHOR_EMAIL="+committerEmail,
		"GIT_COMMITTER_NAME="+committerName,
		"GIT_COMMITTER_EMAIL="+committerEmail,
	)
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}
	return stdout.String(), nil
}

// MergePR merges sourceBranch into targetBranch using a temporary clone and
// returns the merge commit hash.
func (s *Service) MergePR(ctx context.Context, owner, name string, pullNumber int64, sourceBranch, targetBranch, mergedBy string) (string, error) {
	barePath := filepath.Join(s.Root, owner, name+".git")
	work := filepath.Join(os.TempDir(), fmt.Sprintf("gitcastle-merge-%d-%d", os.Getpid(), pullNumber))
	defer func() { _ = os.RemoveAll(work) }()

	if out, err := s.git(ctx, ".", "clone", "--quiet", "--branch", targetBranch, barePath, work); err != nil {
		return "", fmt.Errorf("clone target branch %s: %w: %s", targetBranch, err, out)
	}
	if out, err := s.git(ctx, work, "fetch", "--quiet", "origin", sourceBranch); err != nil {
		return "", fmt.Errorf("fetch source branch %s: %w: %s", sourceBranch, err, out)
	}
	message := fmt.Sprintf("Merge pull request #%d from %s/%s\n\nMerged by %s",
		pullNumber, owner, sourceBranch, mergedBy)
	if out, err := s.git(ctx, work, "merge", "--no-ff", "--quiet", "-m", message, "origin/"+sourceBranch); err != nil {
		return "", fmt.Errorf("merge %s into %s (conflict?): %w: %s", sourceBranch, targetBranch, err, out)
	}
	if _, err := s.git(ctx, work, "push", "--quiet", "origin", targetBranch); err != nil {
		return "", fmt.Errorf("push merged %s: %w", targetBranch, err)
	}
	hashOutput, err := s.git(ctx, work, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	hash := trim(hashOutput)

	if s.Events != nil {
		s.Events <- Event{
			RepositoryID: 0, // caller fills via SetRepositoryID if needed
			Owner:        owner,
			Name:         name,
			PullNumber:   pullNumber,
			MergeCommit:  hash,
			TargetBranch: targetBranch,
			SourceBranch: sourceBranch,
			MergedBy:     mergedBy,
		}
	}
	return hash, nil
}

func trim(value string) string {
	for len(value) > 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == ' ' || value[len(value)-1] == '\t') {
		value = value[:len(value)-1]
	}
	return value
}
