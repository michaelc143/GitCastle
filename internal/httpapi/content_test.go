package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/michaelc143/gitcastle/internal/gitdata"
)

type fakeContent struct {
	repo gitdata.Repo
}

func (f *fakeContent) Repo(_, _ string) (gitdata.Repo, error) {
	return f.repo, nil
}

// newContentTestRepo seeds a real bare repository using the same helper the
// gitdata package tests use.
func newContentTestRepo(t *testing.T) gitdata.Repo {
	t.Helper()
	return newGitdataTestRepo(t)
}

func newContentHandlerWithRepo(t *testing.T) (http.Handler, *fakePermissions) {
	t.Helper()
	content := &fakeContent{repo: newContentTestRepo(t)}
	permissions := &fakePermissions{}
	authenticator := &fakeAuthenticator{sessionToken: "test-token"}
	handler := NewHandler(&fakeRepositoryService{}, authenticator, permissions, discardLogger(), content)
	return handler, permissions
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func getContent(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, path, nil))
	return recorder
}

// newGitdataTestRepo mirrors gitdata's newTestRepo; it lives here because Go
// test helpers cannot be imported across packages.
func newGitdataTestRepo(t *testing.T) gitdata.Repo {
	t.Helper()

	work := t.TempDir()
	bare := filepath.Join(t.TempDir(), "test.git")

	run := func(args ...string) string {
		command := gitCommandForTest(args...)
		command.Dir = work
		command.Env = append(
			os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
			"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return string(output)
	}

	run("init", "-q", "-b", "main", ".")
	write := func(name, content string) {
		full := filepath.Join(work, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# Test\n")
	write("src/main.go", "package main\n")
	run("add", ".")
	run("commit", "-q", "-m", "first commit")
	write("README.md", "# Test\n\ngrown\n")
	run("add", ".")
	run("commit", "-q", "-m", "second commit")
	run("branch", "feature")
	run("tag", "v1")

	if output, err := execCommandCombined("git", "clone", "--bare", "--quiet", work, bare); err != nil {
		t.Fatalf("clone --bare: %v: %s", err, output)
	}
	return gitdata.Repo{Path: bare}
}

func extractJSONString(t *testing.T, body, key string) string {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(body), &document); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	first, ok := document[key].([]any)
	if !ok || len(first) == 0 {
		t.Fatalf("no %s array in body:\n%s", key, body)
	}
	entry, ok := first[0].(map[string]any)
	if !ok {
		t.Fatalf("%s[0] is not an object:\n%s", key, body)
	}
	hash, _ := entry[key].(string)
	if hash == "" {
		t.Fatalf("missing %s in %+v", key, entry)
	}
	return hash
}

func gitCommandForTest(args ...string) *exec.Cmd {
	return exec.Command("git", args...)
}

func execCommandCombined(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}


func TestRefsEndpointRequiresAuth(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/repositories/alice/castle/refs", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestRefsEndpointListsBranchesAndTags(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	recorder := getContent(t, handler, "/api/v1/repositories/alice/castle/refs")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`"head":"main"`, `"name":"main"`, `"name":"feature"`, `"name":"v1"`, `"is_tag":true`} {
		if !stringsContains(body, want) {
			t.Fatalf("body missing %s:\n%s", want, body)
		}
	}
}

func TestTreeEndpointListsRoot(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	recorder := getContent(t, handler, "/api/v1/repositories/alice/castle/tree/main")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !stringsContains(body, `"path":"README.md"`) || !stringsContains(body, `"type":"tree"`) {
		t.Fatalf("unexpected tree body:\n%s", body)
	}
}

func TestBlobEndpointReturnsContents(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	recorder := getContent(t, handler, "/api/v1/repositories/alice/castle/blob/main?path=README.md")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !stringsContains(recorder.Body.String(), "# Test") {
		t.Fatalf("blob content missing:\n%s", recorder.Body.String())
	}
}

func TestCommitsEndpointListsHistory(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	recorder := getContent(t, handler, "/api/v1/repositories/alice/castle/commits/main")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !stringsContains(body, "second commit") || !stringsContains(body, "first commit") {
		t.Fatalf("commits missing:\n%s", body)
	}
}

func TestCommitEndpointReturnsPatch(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	listBody := getContent(t, handler, "/api/v1/repositories/alice/castle/commits/main").Body.String()
	var document struct {
		Commits []struct{ Hash string `json:"hash"` } `json:"commits"`
	}
	if err := json.Unmarshal([]byte(listBody), &document); err != nil || len(document.Commits) == 0 {
		t.Fatalf("could not read commits: %v %s", err, listBody)
	}
	headHash := document.Commits[0].Hash

	recorder := getContent(t, handler, "/api/v1/repositories/alice/castle/commit/"+headHash)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !stringsContains(recorder.Body.String(), "+grown") {
		t.Fatalf("patch missing added line:\n%s", recorder.Body.String())
	}
}

func TestMissingRevisionMapsTo404(t *testing.T) {
	handler, _ := newContentHandlerWithRepo(t)
	recorder := getContent(t, handler, "/api/v1/repositories/alice/castle/tree/nope")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", recorder.Code, recorder.Body.String())
	}
}

func stringsContains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
