package auth

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/michaelc143/gitcastle/internal/database"
)

func profileStore(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("GITCASTLE_INTEGRATION") != "1" {
		t.Skip("integration test; set GITCASTLE_INTEGRATION=1")
	}
	pool, err := pgxpool.New(context.Background(), databaseTestURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &Store{Pool: pool}
}

func databaseTestURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://gitcastle:gitcastle@localhost:5432/gitcastle?sslmode=disable"
}

func randomSuffix() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	raw := make([]byte, 8)
	for i := range raw {
		raw[i] = charset[rand.Intn(len(charset))]
	}
	return string(raw)
}

func uniqueUser(t *testing.T, store *Store) string {
	t.Helper()
	username := "profiler-" + randomSuffix()
	if _, err := store.CreateUser(context.Background(), username, "password-123"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return username
}

func TestGetProfileDefaults(t *testing.T) {
	store := profileStore(t)
	username := uniqueUser(t, store)

	profile, err := store.GetProfile(context.Background(), username)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Username != username || profile.DisplayName != "" || profile.Bio != "" {
		t.Fatalf("profile = %+v", profile)
	}
	if profile.JoinedAt.IsZero() {
		t.Fatal("joined_at missing")
	}
}

func TestUpdateProfileRoundTrip(t *testing.T) {
	store := profileStore(t)
	username := uniqueUser(t, store)

	updated, err := store.UpdateProfile(context.Background(), username, Profile{
		DisplayName: "Alice of House Corbishley",
		Bio:         "Keeper of the keys.",
		Location:    "The Keep",
		Website:     "https://alice.example.com",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.DisplayName != "Alice of House Corbishley" || updated.Location != "The Keep" {
		t.Fatalf("update lost data: %+v", updated)
	}

	reread, err := store.GetProfile(context.Background(), username)
	if err != nil || reread.Bio != "Keeper of the keys." {
		t.Fatalf("reread = %+v, %v", reread, err)
	}
}

func TestUpdateProfileValidation(t *testing.T) {
	store := profileStore(t)
	username := uniqueUser(t, store)
	ctx := context.Background()

	if _, err := store.UpdateProfile(ctx, username, Profile{Website: "javascript:alert(1)"}); err == nil {
		t.Fatal("javascript: URL should be rejected")
	}
	if _, err := store.UpdateProfile(ctx, username, Profile{Bio: strings.Repeat("x", 501)}); err == nil {
		t.Fatal("over-long bio should be rejected")
	}
	if _, err := store.UpdateProfile(ctx, "ghost-user-missing", Profile{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user error = %v", err)
	}
}

func TestGetProfileMissing(t *testing.T) {
	store := profileStore(t)
	if _, err := store.GetProfile(context.Background(), "definitely-not-here"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
