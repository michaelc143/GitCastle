package ci

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner executes build commands inside an isolated Docker container with a
// checkout of the repository at the job's commit.
type Runner struct {
	// Image runs the build; must have git and a shell.
	Image string
	// Timeout bounds each job.
	Timeout time.Duration
	// WorkRoot holds per-job checkouts and is cleaned after each run.
	WorkRoot string
}

// Result captures the outcome of one job execution.
type Result struct {
	ExitCode int
	Output   string
}

// Run checks out commitHash from barePath into a fresh container-mounted
// directory, executes command inside the container, and returns the result.
func (r *Runner) Run(ctx context.Context, barePath, commitHash, branch, command string) (Result, error) {
	if r.Image == "" {
		return Result{}, fmt.Errorf("runner image not configured")
	}
	if r.WorkRoot == "" {
		r.WorkRoot = os.TempDir()
	}
	// Ensure the work root exists; the server may not have created it.
	if err := os.MkdirAll(r.WorkRoot, 0o700); err != nil {
		return Result{}, fmt.Errorf("create work root: %w", err)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workDir, err := os.MkdirTemp(r.WorkRoot, "gitcastle-job-")
	if err != nil {
		return Result{}, fmt.Errorf("create work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	// Clone outside the container (host git), then hand the tree to Docker.
	if out, err := hostGit(runCtx, "--git-dir", barePath, "worktree", "add",
		"--detach", workDir+"/src", commitHash); err != nil {
		_, _ = hostGit(runCtx, "--git-dir", barePath, "worktree", "prune")
		return Result{}, fmt.Errorf("checkout %s: %w: %s", branchLabel(commitHash, branch), err, out)
	}

	containerName := fmt.Sprintf("gitcastle-build-%d-%s", os.Getpid(), commitHash[:8])
	dockerArgs := []string{
		"run", "--rm",
		"--name", containerName,
		"--network", "none",           // no network access for builds by default
		"-v", workDir + "/src:/workspace:ro",
		"-w", "/workspace",
		r.Image,
		"/bin/sh", "-c", command,
	}
	docker := exec.CommandContext(runCtx, "docker", dockerArgs...)
	var output strings.Builder
	docker.Stdout = &output
	docker.Stderr = &output

	err = docker.Run()
	exitCode := 0
	status := StatusSuccess
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		status = StatusFailed
	} else if err != nil {
		// Docker itself failed (daemon down, image missing...).
		output.WriteString("\n[docker error] " + err.Error())
		return Result{ExitCode: -1, Output: output.String()}, fmt.Errorf("docker run: %w", err)
	}

	cleanup := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	_ = cleanup.Run() // best effort; --rm handles it on success

	return Result{ExitCode: exitCode, Output: output.String()}, statusError(status)
}

func statusError(status string) error {
	if status == StatusFailed {
		return fmt.Errorf("build failed")
	}
	return nil
}

func branchLabel(hash, branch string) string {
	if branch != "" {
		return branch + "@" + shortHash(hash)
	}
	return shortHash(hash)
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func hostGit(ctx context.Context, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	out, err := command.CombinedOutput()
	return string(out), err
}
