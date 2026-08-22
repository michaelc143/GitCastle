package gitdata

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newTestRepo creates a bare repository seeded with two commits and a branch.
func newTestRepo(t *testing.T) Repo {
	t.Helper()
	work := t.TempDir()
	bare := filepath.Join(t.TempDir(), "test.git")

	run := func(args ...string) string {
		command := exec.Command("git", args...)
		command.Dir = work
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
			// Bypass the user's global hooksPath, which rewrites messages.
			"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return string(output)
	}

	run("init", "-q", "-b", "main", ".")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# Test\n")
	if err := os.MkdirAll(filepath.Join(work, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join("src", "main.go"), "package main\n")
	run("add", ".")
	run("commit", "-q", "-m", "first commit")
	write("README.md", "# Test\n\ngrown\n")
	run("add", ".")
	run("commit", "-q", "-m", "second commit")
	run("branch", "feature")
	run("tag", "v1")

	initOutput, err := exec.Command("git", "clone", "--bare", "--quiet", work, bare).CombinedOutput()
	if err != nil {
		t.Fatalf("clone --bare: %v: %s", err, initOutput)
	}
	return Repo{Path: bare}
}

func TestListRefs(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	refs, head, err := repo.ListRefs(ctx)
	if err != nil {
		t.Fatalf("ListRefs() error = %v", err)
	}
	if head != "main" {
		t.Fatalf("head = %q, want main", head)
	}
	names := make(map[string]Ref)
	for _, ref := range refs {
		names[ref.Name] = ref
	}
	if _, ok := names["main"]; !ok {
		t.Fatalf("missing main branch in %+v", refs)
	}
	if _, ok := names["feature"]; !ok {
		t.Fatal("missing feature branch")
	}
	if tag, ok := names["v1"]; !ok || !tag.IsTag {
		t.Fatalf("missing v1 tag or IsTag wrong: %+v", tag)
	}
}

func TestListTreeAndBlob(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	root, err := repo.ListTree(ctx, "main", "")
	if err != nil {
		t.Fatalf("ListTree(root) error = %v", err)
	}
	if len(root) != 2 {
		t.Fatalf("root entries = %d, want 2 (README.md + src)", len(root))
	}

	src, err := repo.ListTree(ctx, "main", "src")
	if err != nil {
		t.Fatalf("ListTree(src) error = %v", err)
	}
	if len(src) != 1 || src[0].Path != "main.go" {
		t.Fatalf("src entries = %+v", src)
	}

	content, tooLarge, err := repo.Blob(ctx, "main", "README.md")
	if err != nil || tooLarge {
		t.Fatalf("Blob() error = %v tooLarge = %v", err, tooLarge)
	}
	if string(content) != "# Test\n\ngrown\n" {
		t.Fatalf("blob content = %q", content)
	}

	if _, _, err := repo.Blob(ctx, "main", "missing.txt"); err != ErrNotFound {
		t.Fatalf("Blob(missing) error = %v, want ErrNotFound", err)
	}
}

func TestListCommitsAndGetCommit(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	commits, err := repo.ListCommits(ctx, "main", "", 10)
	if err != nil {
		t.Fatalf("ListCommits() error = %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("commits = %d, want 2", len(commits))
	}
	if commits[0].Message != "second commit" || commits[1].Message != "first commit" {
		t.Fatalf("unexpected order: %+v", commits)
	}
	if commits[0].Author != "Alice" || commits[0].Email != "alice@example.com" {
		t.Fatalf("author = %s <%s>", commits[0].Author, commits[0].Email)
	}

	byPath, err := repo.ListCommits(ctx, "main", "src/main.go", 10)
	if err != nil {
		t.Fatalf("ListCommits(path) error = %v", err)
	}
	if len(byPath) != 1 || byPath[0].Message != "first commit" {
		t.Fatalf("path-filtered commits = %+v", byPath)
	}

	head := commits[0].Hash
	commit, patch, err := repo.GetCommit(ctx, head)
	if err != nil {
		t.Fatalf("GetCommit() error = %v", err)
	}
	if commit.Message != "second commit" {
		t.Fatalf("commit message = %q", commit.Message)
	}
	if len(commit.Parents) != 1 {
		t.Fatalf("parents = %v", commit.Parents)
	}
	if patch == "" || !contains(patch, "+grown") {
		t.Fatalf("patch missing added line:\n%s", patch)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && stringsContains(haystack, needle)
}
