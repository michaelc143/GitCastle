package audit

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/michaelc143/gitcastle/internal/database"
)

func databaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://gitcastle:gitcastle@localhost:5432/gitcastle?sslmode=disable"
}

func integrationStore(t *testing.T) *Store {
	t.Helper()
	if env := os.Getenv("GITCASTLE_INTEGRATION"); env != "1" {
		t.Skip("integration test; set GITCASTLE_INTEGRATION=1")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &Store{Pool: pool}
}

func TestRecordAndList(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()

	if err := store.Record(ctx, "alice", EventLogin, "alice", map[string]any{"mfa": false}, "10.0.0.1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := store.Record(ctx, "bob", EventRepoCreated, "bob/fort", nil, ""); err != nil {
		t.Fatalf("Record without details: %v", err)
	}

	all, err := store.List(ctx, "", "", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(all))
	}
	// Newest first.
	if all[0].Action != EventRepoCreated || all[0].Actor != "bob" {
		t.Fatalf("newest entry = %+v", all[0])
	}
	if all[1].Details["mfa"] != false || all[1].RemoteIP != "10.0.0.1" {
		t.Fatalf("details/ip not persisted: %+v", all[1])
	}

	byActor, err := store.List(ctx, "alice", "", 50)
	if err != nil || len(byActor) == 0 {
		t.Fatalf("actor filter = %d entries, %v", len(byActor), err)
	}
	for _, entry := range byActor {
		if entry.Actor != "alice" {
			t.Fatalf("actor filter leaked %q", entry.Actor)
		}
	}

	byAction, err := store.List(ctx, "", EventRepoCreated, 50)
	if err != nil || len(byAction) == 0 {
		t.Fatalf("action filter = %d entries, %v", len(byAction), err)
	}
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()

	if _, err := store.Pool.Exec(ctx,
		`UPDATE audit_log SET actor = 'tampered' WHERE actor = 'alice'`); err == nil {
		t.Fatal("UPDATE should be blocked by trigger")
	}
	if _, err := store.Pool.Exec(ctx,
		`DELETE FROM audit_log WHERE action = $1`, EventLogin); err == nil {
		t.Fatal("DELETE should be blocked by trigger")
	}
}
