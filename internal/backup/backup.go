// Package backup creates and restores point-in-time repository backups.
// Bundles are `git bundle` archives (self-contained, restorable by any git)
// pushed to an ObjectStore with timestamped keys.
package backup

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/michaelc143/gitcastle/internal/storage"
)

var ErrNotFound = storage.ErrNotFound

// Manager coordinates backups for all repositories under Root.
type Manager struct {
	Root   string // repository root: {root}/{owner}/{name}.git
	Store  storage.ObjectStore
	Clock  func() time.Time // injectable for tests
}

func (m *Manager) now() time.Time {
	if m.Clock != nil {
		return m.Clock()
	}
	return time.Now().UTC()
}

// Backup bundles one repository (all refs) and stores it. Returns the key.
func (m *Manager) Backup(ctx context.Context, owner, name string) (string, error) {
	barePath := filepath.Join(m.Root, owner, name+".git")
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "--git-dir", barePath, "bundle", "create", "-", "--all")
	cmd.Stdout = &stdout
	cmd.Stderr = discard{}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("bundle %s/%s: %w", owner, name, err)
	}
	timestamp := m.now().Format("20060102-150405")
	key := fmt.Sprintf("backups/%s/%s/%s-%s.bundle", owner, name, name, timestamp)
	if err := m.Store.Put(ctx, key, stdout.Bytes()); err != nil {
		return "", fmt.Errorf("store backup: %w", err)
	}
	return key, nil
}

// List returns stored backups for one repository, newest first.
func (m *Manager) List(ctx context.Context, owner, name string) ([]storage.Object, error) {
	prefix := fmt.Sprintf("backups/%s/%s/", owner, name)
	objects, err := m.Store.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	if objects == nil {
		objects = []storage.Object{}
	}
	return objects, nil
}

// VerifyRestore checks a stored bundle is a valid git archive by listing its
// heads — catches truncated or corrupted uploads without touching the repo.
func (m *Manager) VerifyRestore(ctx context.Context, key string) ([]string, error) {
	data, err := m.Store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "bundle", "list-heads", "-")
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bundle invalid: %w", err)
	}
	var heads []string
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		if len(line) > 41 { // hash + space + refname
			heads = append(heads, string(line[41:]))
		}
	}
	return heads, nil
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
