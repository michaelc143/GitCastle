package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/michaelc143/gitcastle/internal/repos"
)

func TestPostgresRepositoryStoreIntegration(t *testing.T) {
	if os.Getenv("GITCASTLE_INTEGRATION") != "1" {
		t.Skip("set GITCASTLE_INTEGRATION=1 to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://gitcastle:gitcastle@localhost:5432/gitcastle?sslmode=disable"
	}
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	owner := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	name := "repository"
	store := repos.PostgresStore{Pool: pool}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM repositories WHERE owner = $1 AND name = $2", owner, name)
	})

	created, err := store.Create(ctx, repos.Repository{Owner: owner, Name: name, Path: "/tmp/" + owner + "/" + name + ".git"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	found, err := store.Get(ctx, owner, name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found.ID != created.ID || found.Path != created.Path {
		t.Fatalf("found repository = %+v, created = %+v", found, created)
	}
}
