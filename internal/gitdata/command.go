package gitdata

import (
	"context"
	"os/exec"
)

func gitCommand(ctx context.Context, path string, args []string) *exec.Cmd {
	command := exec.CommandContext(ctx, "git", "--git-dir", path)
	command.Args = append(command.Args, args...)
	return command
}
