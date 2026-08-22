package repos

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestServiceCreateInitializesUsableBareGitRepositoryIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	service := Service{
		Store:          &fakeStore{},
		RepositoryRoot: root,
		Git:            CommandGitInitializer{},
	}

	repository, err := service.Create(context.Background(), CreateInput{Owner: "alice", Name: "castle"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repository.Path != filepath.Join(root, "alice", "castle.git") {
		t.Fatalf("repository path = %q", repository.Path)
	}

	command := exec.Command("git", "--git-dir", repository.Path, "symbolic-ref", "HEAD")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("inspect bare repository: %v", err)
	}
	if string(output) != "refs/heads/main\n" {
		t.Fatalf("default branch = %q", output)
	}
}
