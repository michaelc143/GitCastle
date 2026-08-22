package gitserve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeAuthorizer struct {
	userID      int64
	authErr     error
	accessErr   error
	lastAccess  Access
	lastRepo    string
	checkCalled bool
}

func (f *fakeAuthorizer) Authenticate(_ context.Context, _, _ string) (int64, error) {
	return f.userID, f.authErr
}

func (f *fakeAuthorizer) CheckAccess(_ context.Context, _ int64, _, repo string, access Access) error {
	f.checkCalled = true
	f.lastRepo = repo
	f.lastAccess = access
	return f.accessErr
}

func TestAuthHandlerRejectsAnonymous(t *testing.T) {
	handler := AuthHandler{Backend: http.NotFoundHandler(), Auth: &fakeAuthorizer{}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/git/alice/castle.git/info/refs", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatal("missing WWW-Authenticate challenge")
	}
}

func TestAuthHandlerRejectsBadCredentials(t *testing.T) {
	handler := AuthHandler{Backend: http.NotFoundHandler(), Auth: &fakeAuthorizer{authErr: errors.New("bad")}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/git/alice/castle.git/info/refs", nil)
	request.SetBasicAuth("alice", "wrong")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestAuthHandlerChecksReadOnFetch(t *testing.T) {
	authorizer := &fakeAuthorizer{userID: 7}
	handler := AuthHandler{Backend: http.NotFoundHandler(), Auth: authorizer}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/git/alice/castle.git/git-upload-pack", nil)
	request.SetBasicAuth("alice", "secret")

	handler.ServeHTTP(recorder, request)

	if !authorizer.checkCalled {
		t.Fatal("CheckAccess was not called")
	}
	if authorizer.lastAccess != AccessRead {
		t.Fatalf("access = %v, want read", authorizer.lastAccess)
	}
	if authorizer.lastRepo != "castle" {
		t.Fatalf("repo = %q, want castle", authorizer.lastRepo)
	}
}

func TestAuthHandlerRequiresWriteForPush(t *testing.T) {
	authorizer := &fakeAuthorizer{userID: 7, accessErr: ErrAccessDenied}
	handler := AuthHandler{Backend: http.NotFoundHandler(), Auth: authorizer}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/git/alice/castle.git/git-receive-pack?service=git-receive-pack", nil)
	request.SetBasicAuth("alice", "secret")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for denied write", recorder.Code)
	}
	if authorizer.lastAccess != AccessWrite {
		t.Fatalf("access = %v, want write", authorizer.lastAccess)
	}
}

func TestParseOwnerRepo(t *testing.T) {
	cases := []struct {
		path          string
		owner, repo   string
	}{
		{"/git/alice/castle.git/info/refs", "alice", "castle"},
		{"/git/alice/castle/info/refs", "alice", "castle"},
		{"/git/alice/castle.git/git-receive-pack", "alice", "castle"},
		{"/git/onlyonepath", "", ""},
	}
	for _, tc := range cases {
		owner, repo := parseOwnerRepo(tc.path)
		if owner != tc.owner || repo != tc.repo {
			t.Fatalf("parseOwnerRepo(%q) = (%q, %q), want (%q, %q)", tc.path, owner, repo, tc.owner, tc.repo)
		}
	}
}
