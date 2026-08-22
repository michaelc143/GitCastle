package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/michaelc143/gitcastle/internal/repos"
)

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
	s.created = repos.Repository{Owner: input.Owner, Name: input.Name, Path: "/tmp/" + input.Name + ".git"}
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
	handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"owner":"alice","name":"castle"}`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.created.Owner != "alice" || service.created.Name != "castle" {
		t.Fatalf("created repository = %+v", service.created)
	}
}

func TestCreateRepositoryRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(&fakeRepositoryService{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"owner":`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetRepositoryMapsNotFound(t *testing.T) {
	service := &fakeRepositoryService{err: repos.ErrNotFound}
	handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/repositories/alice/missing", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestServiceFailureMapsToInternalServerError(t *testing.T) {
	service := &fakeRepositoryService{err: errors.New("unexpected failure")}
	handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/repositories", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
