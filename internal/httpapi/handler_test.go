package httpapi

import (
	"context"
	"errors"
	"io"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michaelc143/gitcastle/internal/auth"
	"github.com/michaelc143/gitcastle/internal/repos"
)

type fakeAuthenticator struct {
	users        map[string]string // username -> password
	sessionToken string
	err          error
}

func (f *fakeAuthenticator) CreateUser(_ context.Context, username, password string) (auth.User, error) {
	if f.err != nil {
		return auth.User{}, f.err
	}
	if f.users == nil {
		f.users = map[string]string{}
	}
	if _, exists := f.users[username]; exists {
		return auth.User{}, auth.ErrUserExists
	}
	f.users[username] = password
	return auth.User{ID: 1, Username: username, CreatedAt: time.Now()}, nil
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, username, password string) (auth.User, error) {
	if stored, ok := f.users[username]; ok && stored == password {
		return auth.User{ID: 1, Username: username}, nil
	}
	return auth.User{}, auth.ErrBadCredentials
}

func (f *fakeAuthenticator) StartSession(context.Context, int64) (string, time.Time, error) {
	return f.sessionToken, time.Now().Add(time.Hour), nil
}

func (f *fakeAuthenticator) UserForToken(_ context.Context, token string) (auth.User, error) {
	if token == f.sessionToken {
		return auth.User{ID: 1, Username: "alice"}, nil
	}
	return auth.User{}, auth.ErrNoSession
}

func (f *fakeAuthenticator) EndSession(context.Context, string) error { return nil }

type fakePermissions struct {
	grants map[string]string // "repoID:username" -> role
	err    error
}

func (f *fakePermissions) Grant(_ context.Context, repositoryID int64, username, role string) error {
	if f.err != nil {
		return f.err
	}
	if f.grants == nil {
		f.grants = map[string]string{}
	}
	f.grants[fmt.Sprintf("%d:%s", repositoryID, username)] = role
	return nil
}

func (f *fakePermissions) RoleFor(_ context.Context, repositoryID int64, username string) (string, error) {
	if role, ok := f.grants[fmt.Sprintf("%d:%s", repositoryID, username)]; ok {
		return role, nil
	}
	return "", auth.ErrNotFound
}

func newTestHandler(service *fakeRepositoryService, authenticator Authenticator) http.Handler {
	if authenticator == nil {
		authenticator = &fakeAuthenticator{sessionToken: "test-token"}
	}
	return NewHandler(service, authenticator, &fakePermissions{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}


func authenticatedRequest(method, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "test-token"})
	return request
}

type fakeRepositoryService struct {
	created repos.Repository
	list    []repos.Repository
	get     repos.Repository
	err     error
}

func (s *fakeRepositoryService) Create(_ context.Context, input repos.CreateInput) (repos.Repository, error) {
	if s.err != nil {
		return repos.Repository{}, s.err
	}
	s.created = repos.Repository{ID: 1, Owner: input.Owner, Name: input.Name, Path: "/tmp/" + input.Name + ".git"}
	return s.created, nil
}

func (s *fakeRepositoryService) Get(context.Context, string, string) (repos.Repository, error) {
	if s.err != nil {
		return repos.Repository{}, s.err
	}
	return s.get, nil
}

func (s *fakeRepositoryService) List(context.Context) ([]repos.Repository, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

func TestCreateRepository(t *testing.T) {
	service := &fakeRepositoryService{}
	handler := newTestHandler(service, nil)
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"owner":"alice","name":"castle"}`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.created.Owner != "alice" || service.created.Name != "castle" {
		t.Fatalf("created repository = %+v", service.created)
	}
}

func TestCreateRepositoryRejectsMalformedJSON(t *testing.T) {
	handler := newTestHandler(&fakeRepositoryService{}, nil)
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"owner":`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetRepositoryMapsNotFound(t *testing.T) {
	service := &fakeRepositoryService{err: repos.ErrNotFound}
	handler := newTestHandler(service, nil)
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/v1/repositories/alice/missing", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestServiceFailureMapsToInternalServerError(t *testing.T) {
	service := &fakeRepositoryService{err: errors.New("unexpected failure")}
	handler := newTestHandler(service, nil)
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/v1/repositories", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRepositoriesRequireAuthentication(t *testing.T) {
	authenticator := &fakeAuthenticator{sessionToken: ""}
	service := &fakeRepositoryService{}
	handler := NewHandler(service, authenticator, &fakePermissions{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/repositories", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterAndLoginFlow(t *testing.T) {
	authenticator := &fakeAuthenticator{sessionToken: "session-123"}
	handler := newTestHandler(&fakeRepositoryService{}, authenticator)

	// Register
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(`{"username":"alice","password":"correct horse"}`)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	// Login sets the session cookie
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(`{"username":"alice","password":"correct horse"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Value != "session-123" {
		t.Fatalf("expected session cookie, got %+v", cookies)
	}

	// /me with that cookie
	recorder = httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	meRequest.AddCookie(cookies[0])
	handler.ServeHTTP(recorder, meRequest)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"alice"`) {
		t.Fatalf("me status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	authenticator := &fakeAuthenticator{users: map[string]string{"alice": "correct horse"}}
	handler := newTestHandler(&fakeRepositoryService{}, authenticator)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(`{"username":"alice","password":"wrong"}`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateRepositoryGrantsAdminToCreator(t *testing.T) {
	service := &fakeRepositoryService{}
	permissions := &fakePermissions{}
	authenticator := &fakeAuthenticator{sessionToken: "test-token"}
	handler := NewHandler(service, authenticator, permissions, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"castle"}`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if role := permissions.grants["1:alice"]; role != auth.RoleAdmin {
		t.Fatalf("creator role = %q, want %q", role, auth.RoleAdmin)
	}
}
