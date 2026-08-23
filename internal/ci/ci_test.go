package ci

import (
	"context"
	"strings"
	"testing"
)

func testContext() context.Context { return context.Background() }

func TestStatusError(t *testing.T) {
	if statusError(StatusSuccess) != nil {
		t.Fatal("success should not be an error")
	}
	if statusError(StatusFailed) == nil {
		t.Fatal("failed should be an error")
	}
}

func TestBranchLabel(t *testing.T) {
	hash := "abcdef1234567890"
	cases := []struct{ branch, want string }{
		{"main", "main@abcdef12"},
		{"", "abcdef12"},
	}
	for _, tc := range cases {
		if got := branchLabel(hash, tc.branch); got != tc.want {
			t.Fatalf("branchLabel(%q) = %q, want %q", tc.branch, got, tc.want)
		}
	}
}

func TestShortHash(t *testing.T) {
	if got := shortHash("abcdefghijklmnop"); got != "abcdefgh" {
		t.Fatalf("shortHash = %q", got)
	}
	if got := shortHash("abc"); got != "abc" {
		t.Fatalf("short shortHash = %q", got)
	}
}

func TestRunnerRequiresImage(t *testing.T) {
	runner := &Runner{}
	if _, err := runner.Run(testContext(), "/tmp/does-not-matter.git", "abcdef1234567890", "main", "true"); err == nil ||
		!strings.Contains(err.Error(), "image not configured") {
		t.Fatalf("err = %v, want image-not-configured", err)
	}
}

func TestStoreValidation(t *testing.T) {
	// MarkFinished rejects invalid statuses without touching the database.
	store := &Store{} // nil pool is fine; validation happens first
	if err := store.MarkFinished(testContext(), 1, "bogus", "", 0); err == nil {
		t.Fatal("invalid status should be rejected before any DB call")
	}
}
