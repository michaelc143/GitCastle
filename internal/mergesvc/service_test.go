package mergesvc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// seedRepo creates a bare repo with main + a divergent feature branch.
func seedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	work := t.TempDir()
	bare := filepath.Join(root, "alice", "castle.git")
	if err := os.MkdirAll(filepath.Dir(bare), 0o700); err != nil {
		t.Fatal(err)
	}

	run := func(dir string, args ...string) {
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=a@b.c",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=a@b.c",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
			"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
		)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	run(work, "init", "-q", "-b", "main", ".")
	write := func(name, content string) {
		full := filepath.Join(work, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# base\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "base")
	run(work, "checkout", "-qb", "feature")
	write("feature.txt", "new\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "feature work")
	run(work, "checkout", "-q", "main")
	write("main.txt", "mainline\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "main work")

	clone := exec.Command("git", "clone", "--quiet", "--bare", work, bare)
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %v: %s", err, out)
	}
	// The bare clone only carries the checked-out branch; push feature too.
	push := exec.Command("git", "-C", work, "push", "--quiet", bare, "feature")
	if out, err := push.CombinedOutput(); err != nil {
		t.Fatalf("push feature: %v: %s", err, out)
	}
	return root
}

func TestMergePRCreatesMergeCommitAndPushes(t *testing.T) {
	root := seedRepo(t)
	events := make(chan Event, 1)
	service := &Service{Root: root, Events: events}

	hash, err := service.MergePR(context.Background(), "alice", "castle", 1, "feature", "main", "alice")
	if err != nil {
		t.Fatalf("MergePR: %v", err)
	}
	if len(hash) != 40 {
		t.Fatalf("merge hash = %q", hash)
	}

	select {
	case event := <-events:
		if event.MergeCommit != hash || event.SourceBranch != "feature" || event.MergedBy != "alice" {
			t.Fatalf("event = %+v", event)
		}
	default:
		t.Fatal("expected a merge event")
	}

	// Verify the merge actually landed on main in the bare repo.
	log := exec.Command("git", "--git-dir", filepath.Join(root, "alice", "castle.git"),
		"log", "--format=%s", "-n", "1", "main")
	output, err := log.Output()
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(string(output), "Merge pull request #1") {
		t.Fatalf("expected merge commit on main, got: %s", output)
	}
}

func TestMergePRReportsConflict(t *testing.T) {
	root := t.TempDir()
	work := t.TempDir()
	bare := filepath.Join(root, "alice", "clash.git")
	if err := os.MkdirAll(filepath.Dir(bare), 0o700); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=a@b.c",
		"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=a@b.c",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
	)
	run := func(dir string, args ...string) {
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = env
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run(work, "init", "-q", "-b", "main", ".")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("shared.txt", "base\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "base")
	run(work, "checkout", "-qb", "feature")
	write("shared.txt", "feature version\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "feature edit")
	run(work, "checkout", "-q", "main")
	write("shared.txt", "main version\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "main edit")
	if out, err := exec.Command("git", "clone", "--quiet", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", work, "push", "--quiet", bare, "feature").CombinedOutput(); err != nil {
		t.Fatalf("push feature: %v: %s", err, out)
	}

	service := &Service{Root: root}
	if _, err := service.MergePR(context.Background(), "alice", "clash", 2, "feature", "main", "alice"); err == nil {
		t.Fatal("expected conflicting merge to fail")
	}
}
