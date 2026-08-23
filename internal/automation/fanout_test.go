package automation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// readBuildCommand is the CI trigger surface; verify contract behavior.
func TestReadBuildCommand(t *testing.T) {
	root := t.TempDir()
	work := t.TempDir()
	bare := filepath.Join(root, "repo.git")
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
	)
	run := func(dir string, args ...string) {
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = env
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run(work, "init", "-q", "-b", "main", ".")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitcastle.yml", "run: make test\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "ci config")
	if out, err := exec.Command("git", "clone", "--quiet", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %s", out)
	}
	hashOutput, err := exec.Command("git", "--git-dir", bare, "rev-parse", "main").Output()
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.TrimSpace(string(hashOutput))

	if got := readBuildCommand(bare, hash); got != "make test" {
		t.Fatalf("readBuildCommand = %q", got)
	}

	// Missing file yields empty command (job skipped).
	write(".gitcastle.yml", "")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "remove config")
	if out, err := exec.Command("git", "-C", work, "push", "--quiet", bare, "main").CombinedOutput(); err != nil {
		t.Fatalf("push: %s", out)
	}
	hashOutput, _ = exec.Command("git", "--git-dir", bare, "rev-parse", "main").Output()
	newHash := strings.TrimSpace(string(hashOutput))
	if got := readBuildCommand(bare, newHash); got != "" {
		t.Fatalf("expected empty command after removal, got %q", got)
	}

	_ = context.Background
}
