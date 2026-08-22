package collab

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/michaelc143/gitcastle/internal/database"
)

// integrationTestStore connects to the docker-compose Postgres and seeds one
// repository row. Skipped unless GITCASTLE_INTEGRATION=1.
func integrationTestStore(t *testing.T) (*Store, int64) {
	t.Helper()
	if os.Getenv("GITCASTLE_INTEGRATION") != "1" {
		t.Skip("integration test; set GITCASTLE_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://gitcastle:gitcastle@localhost:5432/gitcastle?sslmode=disable")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO repositories(owner, name, path)
		VALUES ('collab-test', 'repo', '/tmp/collab-test.git')
		ON CONFLICT (owner, name) DO UPDATE SET path = EXCLUDED.path
	`); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	var repoID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM repositories WHERE owner = 'collab-test' AND name = 'repo'`).Scan(&repoID); err != nil {
		t.Fatalf("lookup repository: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, repoID)
	})
	return &Store{Pool: pool}, repoID
}

func TestIssueLifecycle(t *testing.T) {
	store, repoID := integrationTestStore(t)
	ctx := context.Background()

	first, err := store.CreateIssue(ctx, repoID, "alice", "Walls are damp", "Need better mortar.")
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	second, err := store.CreateIssue(ctx, repoID, "bob", "Drawbridge squeaks", "")
	if err != nil {
		t.Fatalf("CreateIssue second: %v", err)
	}
	if second.Number != first.Number+1 {
		t.Fatalf("numbers not sequential: %d then %d", first.Number, second.Number)
	}

	got, err := store.GetIssue(ctx, repoID, first.Number)
	if err != nil || got.Title != "Walls are damp" {
		t.Fatalf("GetIssue = %+v, %v", got, err)
	}

	closed, err := store.SetIssueState(ctx, repoID, first.Number, "closed")
	if err != nil || closed.State != "closed" {
		t.Fatalf("SetIssueState = %+v, %v", closed, err)
	}
	if _, err := store.SetIssueState(ctx, repoID, first.Number, "bogus"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("invalid state error = %v", err)
	}

	open, err := store.ListIssues(ctx, repoID, "open")
	if err != nil || len(open) != 1 || open[0].Number != second.Number {
		t.Fatalf("ListIssues(open) = %+v, %v", open, err)
	}
	all, err := store.ListIssues(ctx, repoID, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListIssues(all) = %d items, %v", len(all), err)
	}
}

func TestCommentsOnIssueAndPR(t *testing.T) {
	store, repoID := integrationTestStore(t)
	ctx := context.Background()

	issue, err := store.CreateIssue(ctx, repoID, "alice", "Moat is empty", "")
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	comment, err := store.AddComment(ctx, repoID, "issue", issue.Number, "bob", "Filling it now.")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	comments, err := store.ListComments(ctx, repoID, "issue", issue.Number)
	if err != nil || len(comments) != 1 || comments[0].Body != comment.Body {
		t.Fatalf("ListComments = %+v, %v", comments, err)
	}
	// PR comments must not leak into issue threads.
	prComments, err := store.ListComments(ctx, repoID, "pull_request", issue.Number)
	if err != nil || len(prComments) != 0 {
		t.Fatalf("cross-subject leakage: %+v, %v", prComments, err)
	}
}

func TestPullRequestLifecycleAndReviews(t *testing.T) {
	store, repoID := integrationTestStore(t)
	ctx := context.Background()

	pr, err := store.CreatePullRequest(ctx, repoID, "alice", "Raise walls", "", "feature/walls", "main")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if _, err := store.UpsertReview(ctx, repoID, pr.Number, "bob", "approved", "lgtm"); err != nil {
		t.Fatalf("UpsertReview: %v", err)
	}
	// Bob changes his mind — latest review wins.
	if _, err := store.UpsertReview(ctx, repoID, pr.Number, "bob", "changes_requested", "needs battlements"); err != nil {
		t.Fatalf("UpsertReview replace: %v", err)
	}
	reviews, err := store.ListReviews(ctx, repoID, pr.Number)
	if err != nil || len(reviews) != 1 || reviews[0].Verdict != "changes_requested" {
		t.Fatalf("reviews after re-review = %+v, %v", reviews, err)
	}

	merged, err := store.SetPullRequestState(ctx, repoID, pr.Number, "merged", "abc1234")
	if err != nil || merged.State != "merged" || merged.MergeCommit == nil {
		t.Fatalf("merge = %+v, %v", merged, err)
	}
	// Terminal state cannot transition again.
	if _, err := store.SetPullRequestState(ctx, repoID, pr.Number, "closed", ""); !errors.Is(err, ErrNotMergeable) {
		t.Fatalf("re-merge error = %v, want ErrNotMergeable", err)
	}
	// Reviews survive the merge.
	if _, err := store.UpsertReview(ctx, repoID, pr.Number, "carol", "approved", ""); err == nil {
		t.Fatal("expected reviewing a merged PR to fail")
	}
}

func TestBranchProtectionAndMergeGate(t *testing.T) {
	store, repoID := integrationTestStore(t)
	ctx := context.Background()

	pr, err := store.CreatePullRequest(ctx, repoID, "alice", "Fortify gate", "", "fortify", "main")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}

	// No protection rule: merge check passes for an open PR.
	check, err := store.EvaluateMerge(ctx, repoID, pr.Number)
	if err != nil || !check.Mergable {
		t.Fatalf("unprotected merge check = %+v, %v", check, err)
	}

	if err := store.SetBranchProtection(ctx, repoID, BranchProtection{
		Branch: "main", RequiredApprovals: 2,
	}); err != nil {
		t.Fatalf("SetBranchProtection: %v", err)
	}

	check, err = store.EvaluateMerge(ctx, repoID, pr.Number)
	if err != nil || check.Mergable || check.CurrentApprovals != 0 {
		t.Fatalf("protected check should block: %+v, %v", check, err)
	}

	if _, err := store.UpsertReview(ctx, repoID, pr.Number, "bob", "approved", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	check, err = store.EvaluateMerge(ctx, repoID, pr.Number)
	if err != nil || check.Mergable || check.CurrentApprovals != 1 {
		t.Fatalf("one approval should still block: %+v, %v", check, err)
	}

	if _, err := store.UpsertReview(ctx, repoID, pr.Number, "carol", "approved", ""); err != nil {
		t.Fatalf("second approve: %v", err)
	}
	check, err = store.EvaluateMerge(ctx, repoID, pr.Number)
	if err != nil || !check.Mergable || check.CurrentApprovals != 2 {
		t.Fatalf("two approvals should pass: %+v, %v", check, err)
	}

	// A later changes_requested from carol revokes her approval.
	if _, err := store.UpsertReview(ctx, repoID, pr.Number, "carol", "changes_requested", "hmm"); err != nil {
		t.Fatalf("revoke approval: %v", err)
	}
	check, err = store.EvaluateMerge(ctx, repoID, pr.Number)
	if err != nil || check.Mergable || check.CurrentApprovals != 1 {
		t.Fatalf("revoked approval should block again: %+v, %v", check, err)
	}

	if err := store.DeleteBranchProtection(ctx, repoID, "main"); err != nil {
		t.Fatalf("DeleteBranchProtection: %v", err)
	}
	if _, err := store.GetBranchProtection(ctx, repoID, "main"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetBranchProtection after delete = %v, want ErrNotFound", err)
	}
}
