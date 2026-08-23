package gitdata

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Regression: a freshly initialized repository has zero refs. The refs list
// must serialize as [] (not null) so API clients never crash on it, and the
// default branch name should still be reported.
func TestListRefsOnEmptyRepositoryReturnsEmptySlice(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "empty.git")
	command := exec.Command("git", "init", "--quiet", "--bare", "--initial-branch=main", bare)
	command.Env = append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}

	repo := Repo{Path: bare}
	refs, head, err := repo.ListRefs(context.Background())
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if refs == nil {
		t.Fatal("refs is nil; must be an empty slice for JSON []")
	}
	if len(refs) != 0 {
		t.Fatalf("refs = %+v, want empty", refs)
	}
	if head != "main" {
		t.Fatalf("head = %q, want main", head)
	}

	encoded, err := json.Marshal(map[string]any{"refs": refs})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"refs":[]}` {
		t.Fatalf("serialized = %s, want {\"refs\":[]}", encoded)
	}
}
