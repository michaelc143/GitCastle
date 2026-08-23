// Package storage abstracts blob storage for repository backups. The local
// filesystem implementation ships by default; an S3-compatible client slots
// in via the same interface.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"path/filepath"
	"time"
)

// ObjectStore persists immutable blobs addressed by key.
type ObjectStore interface {
	// Put stores the object; keys use forward-slash pseudo-paths.
	Put(ctx context.Context, key string, data []byte) error
	// Get returns the stored bytes or ErrNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
	// List returns keys under prefix, newest first.
	List(ctx context.Context, prefix string) ([]Object, error)
	// Delete removes a single object.
	Delete(ctx context.Context, key string) error
}

var ErrNotFound = fmt.Errorf("object not found")

type Object struct {
	Key      string    `json:"key"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// LocalStore implements ObjectStore under a root directory.
type LocalStore struct {
	Root string
}

func (s *LocalStore) path(key string) (string, error) {
	if strings.Contains(key, "\x00") {
		return "", fmt.Errorf("invalid object key")
	}
	// Anchor then clean to collapse traversal segments.
	clean := filepath.Clean("/" + key)
	// Clean of an anchored path keeps a leading separator; anything still
	// containing ".." attempted to escape and is rejected outright.
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("object key escapes storage root: %q", key)
	}
	return filepath.Join(s.Root, clean), nil
}

func (s *LocalStore) Put(_ context.Context, key string, data []byte) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create object dir: %w", err)
	}
	// Write-then-rename for atomicity.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write object: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (s *LocalStore) Get(_ context.Context, key string) ([]byte, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

func (s *LocalStore) List(ctx context.Context, prefix string) ([]Object, error) {
	root, err := s.path(prefix)
	if err != nil {
		return nil, err
	}
	var objects []Object
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return io.EOF // nothing stored yet
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, err := filepath.Rel(s.Root, path)
		if err != nil {
			return err
		}
		objects = append(objects, Object{
			Key:      filepath.ToSlash(rel),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
		return nil
	})
	if err != nil && err != io.EOF {
		if os.IsNotExist(err) {
			return []Object{}, nil
		}
		return nil, fmt.Errorf("list objects: %w", err)
	}
	// Newest first.
	for i := 1; i < len(objects); i++ {
		for j := i; j > 0 && objects[j].Modified.After(objects[j-1].Modified); j-- {
			objects[j], objects[j-1] = objects[j-1], objects[j]
		}
	}
	return objects, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
