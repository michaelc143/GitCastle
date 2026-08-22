package repos

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAlreadyExists = errors.New("repository already exists")
var ErrNotFound = errors.New("repository not found")

type Store interface {
	Create(ctx context.Context, repository Repository) (Repository, error)
	Get(ctx context.Context, owner, name string) (Repository, error)
	List(ctx context.Context) ([]Repository, error)
}

type PostgresStore struct {
	Pool *pgxpool.Pool
}

func (s PostgresStore) Create(ctx context.Context, repository Repository) (Repository, error) {
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO repositories(owner, name, path)
		VALUES ($1, $2, $3)
		RETURNING id, owner, name, path, created_at
	`, repository.Owner, repository.Name, repository.Path).Scan(
		&repository.ID,
		&repository.Owner,
		&repository.Name,
		&repository.Path,
		&repository.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Repository{}, ErrAlreadyExists
		}
		return Repository{}, fmt.Errorf("insert repository: %w", err)
	}
	return repository, nil
}

func (s PostgresStore) Get(ctx context.Context, owner, name string) (Repository, error) {
	var repository Repository
	err := s.Pool.QueryRow(ctx, `
		SELECT id, owner, name, path, created_at
		FROM repositories
		WHERE owner = $1 AND name = $2
	`, owner, name).Scan(
		&repository.ID,
		&repository.Owner,
		&repository.Name,
		&repository.Path,
		&repository.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, fmt.Errorf("get repository: %w", err)
	}
	return repository, nil
}

func (s PostgresStore) List(ctx context.Context) ([]Repository, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, owner, name, path, created_at
		FROM repositories
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()

	result := make([]Repository, 0)
	for rows.Next() {
		var repository Repository
		if err := rows.Scan(&repository.ID, &repository.Owner, &repository.Name, &repository.Path, &repository.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}
		result = append(result, repository)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repositories: %w", err)
	}
	return result, nil
}
