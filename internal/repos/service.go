package repos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,99}$`)

type GitInitializer interface {
	InitBare(ctx context.Context, path string) error
}

type Service struct {
	Store          Store
	RepositoryRoot string
	Git            GitInitializer
}

type CommandGitInitializer struct{}

func (CommandGitInitializer) InitBare(ctx context.Context, path string) error {
	command := exec.CommandContext(ctx, "git", "init", "--bare", "--initial-branch=main", path)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("initialize bare repository: %w: %s", err, output)
	}
	// Allow pushes over the smart HTTP transport; bare repos reject
	// receive-pack by default.
	config := exec.CommandContext(ctx, "git", "-C", path, "config", "http.receivepack", "true")
	if output, err := config.CombinedOutput(); err != nil {
		return fmt.Errorf("enable http.receivepack: %w: %s", err, output)
	}
	return nil
}

func (s Service) Create(ctx context.Context, input CreateInput) (Repository, error) {
	if err := validateName("owner", input.Owner); err != nil {
		return Repository{}, err
	}
	if err := validateName("repository", input.Name); err != nil {
		return Repository{}, err
	}
	if s.Store == nil || s.Git == nil {
		return Repository{}, errors.New("repository service is not configured")
	}

	ownerDirectory := filepath.Join(s.RepositoryRoot, input.Owner)
	repositoryPath := filepath.Join(ownerDirectory, input.Name+".git")
	if err := os.MkdirAll(ownerDirectory, 0o750); err != nil {
		return Repository{}, fmt.Errorf("create owner directory: %w", err)
	}
	if _, err := os.Stat(repositoryPath); err == nil {
		return Repository{}, ErrAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return Repository{}, fmt.Errorf("check repository path: %w", err)
	}

	if err := s.Git.InitBare(ctx, repositoryPath); err != nil {
		return Repository{}, err
	}
	repository, err := s.Store.Create(ctx, Repository{Owner: input.Owner, Name: input.Name, Path: repositoryPath})
	if err != nil {
		_ = os.RemoveAll(repositoryPath)
		return Repository{}, err
	}
	return repository, nil
}

func (s Service) Get(ctx context.Context, owner, name string) (Repository, error) {
	if err := validateName("owner", owner); err != nil {
		return Repository{}, err
	}
	if err := validateName("repository", name); err != nil {
		return Repository{}, err
	}
	return s.Store.Get(ctx, owner, name)
}

func (s Service) List(ctx context.Context) ([]Repository, error) {
	return s.Store.List(ctx)
}

func validateName(kind, value string) error {
	if !validName.MatchString(value) {
		return fmt.Errorf("invalid %s name: use 1-100 letters, numbers, dots, dashes, or underscores", kind)
	}
	return nil
}
