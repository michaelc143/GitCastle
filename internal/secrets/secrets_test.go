package secrets

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("GITCASTLE_INTEGRATION") != "1" {
		t.Skip("integration test; set GITCASTLE_INTEGRATION=1")
	}
	pool, err := pgxpool.New(context.Background(), "postgres://gitcastle:gitcastle@localhost:5432/gitcastle?sslmode=disable")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(pool, key)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSetGetRoundTrip(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	repoID := seedRepository(t, store.Pool)
	if err := store.Set(ctx, repoID, "DEPLOY_TOKEN", "s3cr3t-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Get(ctx, repoID, "DEPLOY_TOKEN")
	if err != nil || got != "s3cr3t-value" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	// Overwrite works.
	if err := store.Set(ctx, repoID, "DEPLOY_TOKEN", "rotated"); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Get(ctx, repoID, "DEPLOY_TOKEN"); got != "rotated" {
		t.Fatalf("after rotation = %q", got)
	}

	// Ciphertext must not contain the plaintext.
	var ciphertext []byte
	if err := store.Pool.QueryRow(ctx,
		`SELECT ciphertext FROM deploy_secrets WHERE repository_id = $1 AND name = 'DEPLOY_TOKEN'`,
		repoID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ciphertext), "rotated") {
		t.Fatal("plaintext leaked into stored ciphertext")
	}
}

func TestWrongKeyFailsToDecrypt(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	repoID := seedRepository(t, store.Pool)
	if err := store.Set(ctx, repoID, "KEY", "value"); err != nil {
		t.Fatal(err)
	}

	// Simulate a key change by decrypting with garbage.
	var ciphertext, nonce []byte
	if err := store.Pool.QueryRow(ctx,
		`SELECT ciphertext, nonce FROM deploy_secrets WHERE repository_id = $1 AND name = 'KEY'`,
		repoID).Scan(&ciphertext, &nonce); err != nil {
		t.Fatal(err)
	}
	wrongKey := make([]byte, 32)
	if _, err := rand.Read(wrongKey); err != nil {
		t.Fatal(err)
	}
	broken := &Store{Pool: store.Pool, Key: wrongKey}
	if _, err := broken.decrypt(ciphertext, nonce); err == nil {
		t.Fatal("decryption with wrong key should fail")
	}
}

func TestListNeverReturnsValues(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	repoID := seedRepository(t, store.Pool)
	if err := store.Set(ctx, repoID, "ALPHA", "alpha-secret"); err != nil {
		t.Fatal(err)
	}
	secrets, err := store.List(ctx, repoID)
	if err != nil || len(secrets) == 0 {
		t.Fatalf("List = %+v, %v", secrets, err)
	}
	for _, secret := range secrets {
		if strings.Contains(fmt.Sprint(secret), "alpha-secret") {
			t.Fatal("secret value leaked through List")
		}
	}
}

func seedRepository(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO repositories(owner, name, path)
		VALUES ('secrets-test', 'repo', '/tmp/secrets-test.git')
		ON CONFLICT (owner, name) DO UPDATE SET path = EXCLUDED.path
	`); err != nil {
		t.Fatalf("seed repository (needs migrations applied): %v", err)
	}
	var id int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM repositories WHERE owner='secrets-test' AND name='repo'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id) })
	return id
}
