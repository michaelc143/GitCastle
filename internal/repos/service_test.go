package repos

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeStore struct {
	created   Repository
	createErr error
}

func (s *fakeStore) Create(_ context.Context, repository Repository) (Repository, error) {
	if s.createErr != nil {
		return Repository{}, s.createErr
	}
	s.created = repository
	repository.ID = 1
	return repository, nil
}

func (s *fakeStore) Get(context.Context, string, string) (Repository, error) {
	return Repository{}, ErrNotFound
}
func (s *fakeStore) List(context.Context) ([]Repository, error) { return nil, nil }

type fakeGit struct {
	initializedPath string
	err             error
}

func (g *fakeGit) InitBare(_ context.Context, path string) error {
	g.initializedPath = path
	if g.err != nil {
		return g.err
	}
	return os.MkdirAll(path, 0o750)
}

func TestServiceCreateInitializesBareRepositoryAndPersistsMetadata(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{}
	git := &fakeGit{}
	service := Service{Store: store, RepositoryRoot: root, Git: git}

	repository, err := service.Create(context.Background(), CreateInput{Owner: "alice", Name: "castle"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	wantPath := filepath.Join(root, "alice", "castle.git")
	if repository.Path != wantPath || store.created.Path != wantPath || git.initializedPath != wantPath {
		t.Fatalf("repository path mismatch: repository=%q store=%q git=%q", repository.Path, store.created.Path, git.initializedPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected repository directory to exist: %v", err)
	}
}

func TestServiceRejectsPathTraversalNames(t *testing.T) {
	service := Service{Store: &fakeStore{}, RepositoryRoot: t.TempDir(), Git: &fakeGit{}}

	_, err := service.Create(context.Background(), CreateInput{Owner: "../outside", Name: "castle"})
	if err == nil {
		t.Fatal("Create() expected an error")
	}
	if !strings.Contains(err.Error(), "invalid owner name") {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestServiceRemovesRepositoryWhenStoreFails(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{createErr: errors.New("database unavailable")}
	service := Service{Store: store, RepositoryRoot: root, Git: &fakeGit{}}

	_, err := service.Create(context.Background(), CreateInput{Owner: "alice", Name: "castle"})
	if err == nil {
		t.Fatal("Create() expected an error")
	}
	if _, statErr := os.Stat(filepath.Join(root, "alice", "castle.git")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("repository directory still exists, stat error = %v", statErr)
	}
}
