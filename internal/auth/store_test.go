package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	cases := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"simple", "alice", false},
		{"with dashes", "a-b_c", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 41), true},
		{"spaces", "has space", true},
		{"path traversal", "../etc", true},
		{"unicode", "ünicode", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUsername(tc.username)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateUsername(%q) error = %v, wantErr = %v", tc.username, err, tc.wantErr)
			}
		})
	}
}

func TestCreateUserRejectsShortPassword(t *testing.T) {
	store := &Store{}
	_, err := store.CreateUser(context.Background(), "alice", "short")
	if !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("CreateUser() error = %v, want ErrWeakPassword", err)
	}
}

func TestCreateUserRejectsInvalidUsername(t *testing.T) {
	store := &Store{}
	_, err := store.CreateUser(context.Background(), "bad user!", "long enough password")
	if !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("CreateUser() error = %v, want ErrInvalidUsername", err)
	}
}

func TestHasAtLeast(t *testing.T) {
	cases := []struct {
		role     string
		required string
		want     bool
	}{
		{RoleAdmin, RoleRead, true},
		{RoleWrite, RoleWrite, true},
		{RoleRead, RoleWrite, false},
		{RoleRead, RoleAdmin, false},
		{"", RoleRead, false},
	}
	for _, tc := range cases {
		if got := HasAtLeast(tc.role, tc.required); got != tc.want {
			t.Fatalf("HasAtLeast(%q, %q) = %v, want %v", tc.role, tc.required, got, tc.want)
		}
	}
}

func TestSHA256Sum(t *testing.T) {
	if sha256Sum("abc") == sha256Sum("abd") {
		t.Fatal("different inputs produced the same hash")
	}
	if len(sha256Sum("abc")) != 64 {
		t.Fatalf("hash length = %d, want 64 hex chars", len(sha256Sum("abc")))
	}
}
