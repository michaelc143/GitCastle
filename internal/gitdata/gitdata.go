// Package gitdata reads repositories on disk through git plumbing commands,
// providing the data behind the web interface: refs, trees, blobs, commit
// history, and diffs.
package gitdata

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Repo points at a bare repository path on disk.
type Repo struct {
	Path string
}

func (r Repo) run(ctx context.Context, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", "--git-dir", r.Path)
	command.Args = append(command.Args, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Ref is a branch or tag.
type Ref struct {
	Name   string `json:"name"`
	Hash   string `json:"hash"`
	IsTag  bool   `json:"is_tag"`
}

// ListRefs returns all branches and tags, default branch first.
func (r Repo) ListRefs(ctx context.Context) ([]Ref, string, error) {
	output, err := r.run(ctx, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(objecttype)", "refs/heads", "refs/tags")
	if err != nil {
		return nil, "", err
	}

	var headBranch string
	if head, headErr := r.run(ctx, "symbolic-ref", "--short", "HEAD"); headErr == nil {
		headBranch = strings.TrimSpace(head)
	}

	refs := []Ref{} // never nil: JSON must serialize as [], not null
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) < 3 {
			continue
		}
		// refname:short loses the branch/tag distinction for lightweight
		// tags, so classify by the full refname instead.
		isTag := strings.HasPrefix(parts[0], "refs/tags/")
		name := strings.TrimPrefix(strings.TrimPrefix(parts[0], "refs/heads/"), "refs/tags/")
		refs = append(refs, Ref{Name: name, Hash: parts[1], IsTag: isTag})
	}
	return refs, headBranch, nil
}

// TreeEntry is one row of a directory listing.
type TreeEntry struct {
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob" or "tree"
	Hash string `json:"hash"`
	Path string `json:"path"`
}

// ListTree lists entries under a path at a given revision.
// dir uses forward slashes and may be empty for the root.
func (r Repo) ListTree(ctx context.Context, rev, dir string) ([]TreeEntry, error) {
	spec := rev
	if dir != "" {
		spec = rev + ":" + dir
	}
	output, err := r.run(ctx, "ls-tree", "-z", spec)
	if err != nil {
		if strings.Contains(err.Error(), "Not a valid object name") || strings.Contains(err.Error(), "does not exist") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	entries := []TreeEntry{}
	for _, record := range strings.Split(output, "\x00") {
		if record == "" {
			continue
		}
		meta, name, found := strings.Cut(record, "\t")
		if !found {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) < 3 {
			continue
		}
		entries = append(entries, TreeEntry{Mode: fields[0], Type: fields[1], Hash: fields[2], Path: name})
	}
	return entries, nil
}

// Blob returns file contents at a revision. Large files are rejected.
func (r Repo) Blob(ctx context.Context, rev, path string) ([]byte, bool, error) {
	maxSize := int64(1 << 20) // 1 MiB display cap
	spec := rev + ":" + path
	sizeOutput, err := r.run(ctx, "cat-file", "-s", spec)
	if err != nil {
		if strings.Contains(err.Error(), "Not a valid object name") || strings.Contains(err.Error(), "does not exist") {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}
	var size int64
	if _, err := fmt.Sscanf(strings.TrimSpace(sizeOutput), "%d", &size); err != nil {
		return nil, false, fmt.Errorf("parse blob size: %w", err)
	}
	if size > maxSize {
		return nil, true, nil // too large to display
	}
	data, err := r.runBytes(ctx, "cat-file", "blob", spec)
	if err != nil {
		return nil, false, err
	}
	return data, false, nil
}
