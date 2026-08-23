package automation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPostReceiveHookWritesExecutableScript(t *testing.T) {
	root := t.TempDir()
	barePath := filepath.Join(root, "repo.git")
	if err := os.MkdirAll(filepath.Join(barePath, "hooks"), 0o700); err != nil {
		t.Fatal(err)
	}

	env := HookEnv{
		ServerURL: "http://localhost:8090",
		Token:     "secret-token",
		Owner:     "alice",
		Name:      "castle",
	}
	if err := InstallPostReceiveHook(barePath, env); err != nil {
		t.Fatalf("InstallPostReceiveHook: %v", err)
	}

	info, err := os.Stat(filepath.Join(barePath, "hooks", "post-receive"))
	if err != nil {
		t.Fatalf("hook missing: %v", err)
	}
	if info.Mode()&0o100 == 0 {
		t.Fatal("hook is not executable")
	}
	content, err := os.ReadFile(filepath.Join(barePath, "hooks", "post-receive"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	for _, want := range []string{"http://localhost:8090", "secret-token", "alice", "castle", "notify-push"} {
		if !contains(script, want) {
			t.Fatalf("hook script missing %q:\n%s", want, script)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
