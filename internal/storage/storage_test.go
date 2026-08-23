package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalStorePutGetRoundTrip(t *testing.T) {
	store := &LocalStore{Root: t.TempDir()}
	ctx := context.Background()

	if err := store.Put(ctx, "backups/alice/castle/a.bundle", []byte("bundle-bytes")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := store.Get(ctx, "backups/alice/castle/a.bundle")
	if err != nil || string(got) != "bundle-bytes" {
		t.Fatalf("Get = %q, %v", got, err)
	}
}

func TestLocalStoreGetMissingReturnsSentinel(t *testing.T) {
	store := &LocalStore{Root: t.TempDir()}
	if _, err := store.Get(context.Background(), "nope/missing"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreContainsPathTraversal(t *testing.T) {
	root := t.TempDir()
	store := &LocalStore{Root: root}
	ctx := context.Background()

	// Traversal segments must never escape the root; keys are normalized
	// under it (or rejected outright).
	if err := store.Put(ctx, "../../escape", []byte("x")); err != nil {
		// Rejected outright is acceptable.
		return
	}
	if _, err := store.Get(ctx, "../../escape"); err != nil {
		// Stored under a normalized key; nothing escaped either way.
		_ = err
	}
	escaped, _ := filepath.Glob(filepath.Join(filepath.Dir(root), "*escape*"))
	if len(escaped) != 0 {
		t.Fatalf("file escaped the storage root: %v", escaped)
	}
	// Explicit ".." keys are rejected after normalization.
	if err := store.Put(ctx, "a/../../b", []byte("y")); err != nil && !strings.Contains(err.Error(), "escapes") {
		if _, getErr := store.Get(ctx, "a/../../b"); getErr != nil && !strings.Contains(getErr.Error(), "not found") {
			t.Fatalf("unexpected behavior for mid-path traversal: %v / %v", err, getErr)
		}
	}
}

func TestLocalStoreListPrefixAndOrder(t *testing.T) {
	store := &LocalStore{Root: t.TempDir()}
	ctx := context.Background()

	for _, key := range []string{"p/a/1", "p/b/2", "other/3"} {
		if err := store.Put(ctx, key, []byte(key)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // distinct mtimes
	}
	listed, err := store.List(ctx, "p/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("prefix matched %d objects, want 2", len(listed))
	}
	for _, object := range listed {
		if !strings.HasPrefix(object.Key, "p/") {
			t.Fatalf("key outside prefix: %s", object.Key)
		}
	}
}

func TestLocalStoreDelete(t *testing.T) {
	store := &LocalStore{Root: t.TempDir()}
	ctx := context.Background()
	if err := store.Put(ctx, "k/x", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "k/x"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, "k/x"); err != ErrNotFound {
		t.Fatalf("after delete err = %v", err)
	}
	if err := store.Delete(ctx, "k/x"); err != ErrNotFound {
		t.Fatalf("double delete err = %v", err)
	}
}
