package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Permission roles, ordered from least to most access.
const (
	RoleRead  = "read"
	RoleWrite = "write"
	RoleAdmin = "admin"
)

var ErrNotFound = errors.New("permission not found")

// Permissions manages per-user access to repositories.
type Permissions struct {
	Pool *pgxpool.Pool
}

// Grant gives a user a role on a repository, replacing any existing role.
func (p *Permissions) Grant(ctx context.Context, repositoryID int64, username, role string) error {
	switch role {
	case RoleRead, RoleWrite, RoleAdmin:
	default:
		return fmt.Errorf("invalid role %q", role)
	}
	_, err := p.Pool.Exec(ctx, `
		INSERT INTO repository_permissions(repository_id, username, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (repository_id, username) DO UPDATE SET role = EXCLUDED.role
	`, repositoryID, username, role)
	if err != nil {
		return fmt.Errorf("grant permission: %w", err)
	}
	return nil
}

// Revoke removes a user's access to a repository.
func (p *Permissions) Revoke(ctx context.Context, repositoryID int64, username string) error {
	_, err := p.Pool.Exec(ctx, `
		DELETE FROM repository_permissions
		WHERE repository_id = $1 AND username = $2
	`, repositoryID, username)
	if err != nil {
		return fmt.Errorf("revoke permission: %w", err)
	}
	return nil
}

// RoleFor returns the user's role on a repository, or ErrNotFound when the
// user has no grant. Admin implies write; write implies read.
func (p *Permissions) RoleFor(ctx context.Context, repositoryID int64, username string) (string, error) {
	var role string
	err := p.Pool.QueryRow(ctx, `
		SELECT role
		FROM repository_permissions
		WHERE repository_id = $1 AND username = $2
	`, repositoryID, username).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lookup permission: %w", err)
	}
	return role, nil
}

// HasAtLeast reports whether the user's role meets the minimum required role.
func HasAtLeast(role, required string) bool {
	rank := map[string]int{RoleRead: 1, RoleWrite: 2, RoleAdmin: 3}
	return rank[role] >= rank[required]
}
