package backup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/michaelc143/gitcastle/internal/storage"
)

func seedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	work := t.TempDir()
	bare := filepath.Join(root, "alice", "castle.git")
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
	write := func(name, content string) {
		full := filepath.Join(work, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run(work, "init", "-q", "-b", "main", ".")
	write("README.md", "# castle\n")
	run(work, "add", ".")
	run(work, "commit", "-q", "-m", "base")
	run(work, "branch", "feature")
	if out, err := exec.Command("git", "clone", "--quiet", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %v: %s", err, out)
	}
	return root
}

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestBackupStoresBundleAndListsNewestFirst(t *testing.T) {
	root := seedRepo(t)
	store := &storage.LocalStore{Root: t.TempDir()}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	manager := &Manager{Root: root, Store: store, Clock: fixedClock(base)}
	ctx := context.Background()

	firstKey, err := manager.Backup(ctx, "alice", "castle")
	if err != nil {
		t.Fatalf("first Backup: %v", err)
	}
	wantFirst := "backups/alice/castle/castle-20260101-120000.bundle"
	if firstKey != wantFirst {
		t.Fatalf("key = %q, want %q", firstKey, wantFirst)
	}

	manager.Clock = fixedClock(base.Add(time.Hour))
	secondKey, err := manager.Backup(ctx, "alice", "castle")
	if err != nil {
		t.Fatalf("second Backup: %v", err)
	}

	objects, err := manager.List(ctx, "alice", "castle")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("backups = %d, want 2", len(objects))
	}
	if objects[0].Key != secondKey || objects[1].Key != firstKey {
		t.Fatalf("not newest-first: [%s, %s]", objects[0].Key, objects[1].Key)
	}
}

func TestVerifyRestoreValidatesBundle(t *testing.T) {
	root := seedRepo(t)
	store := &storage.LocalStore{Root: t.TempDir()}
	manager := &Manager{Root: root, Store: store}
	ctx := context.Background()

	key, err := manager.Backup(ctx, "alice", "castle")
	if err != nil {
		t.Fatal(err)
	}
	heads, err := manager.VerifyRestore(ctx, key)
	if err != nil {
		t.Fatalf("VerifyRestore: %v", err)
	}
	foundMain, foundFeature := false, false
	for _, head := range heads {
		switch head {
		case "refs/heads/main":
			foundMain = true
		case "refs/heads/feature":
			foundFeature = true
		}
	}
	if !foundMain || !foundFeature {
		t.Fatalf("heads missing; got %v", heads)
	}

	// Corrupt data must be rejected.
	if err := store.Put(ctx, "backups/alice/castle/broken.bundle", []byte("not a bundle")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.VerifyRestore(ctx, "backups/alice/castle/broken.bundle"); err == nil {
		t.Fatal("corrupt bundle should fail verification")
	}
}

func TestListEmptyRepositoryReturnsNoError(t *testing.T) {
	root := seedRepo(t)
	store := &storage.LocalStore{Root: t.TempDir()}
	manager := &Manager{Root: root, Store: store}
	objects, err := manager.List(context.Background(), "ghost", "missing")
	if err != nil {
		t.Fatalf("List on empty prefix errored: %v", err)
	}
	if len(objects) != 0 {
		t.Fatalf("expected no objects, got %d", len(objects))
	}
}
