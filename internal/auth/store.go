// Package auth provides user accounts, password hashing, and cookie-backed
// sessions for GitCastle.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sessionCookieName = "gitcastle_session"
	sessionDuration   = 14 * 24 * time.Hour
)

var (
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidUsername = errors.New("invalid username")
	ErrWeakPassword    = errors.New("password must be at least 8 characters")
	ErrBadCredentials  = errors.New("invalid username or password")
	ErrNoSession       = errors.New("no valid session")
)

// User is a registered account. Password hashes never leave the store.
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	Pool *pgxpool.Pool
}

func validateUsername(username string) error {
	if len(username) < 1 || len(username) > 40 {
		return ErrInvalidUsername
	}
	for _, r := range username {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return ErrInvalidUsername
		}
	}
	return nil
}

// CreateUser registers a new account with a bcrypt-hashed password.
func (s *Store) CreateUser(ctx context.Context, username, password string) (User, error) {
	if err := validateUsername(username); err != nil {
		return User{}, err
	}
	if len(password) < 8 {
		return User{}, ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	user := User{Username: username}
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO users(username, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at
	`, username, string(hash)).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserExists
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

// Authenticate verifies credentials and returns the user on success.
func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	var (
		user        User
		passwordHash string
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &passwordHash, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Burn comparable time so missing users are not distinguishable.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5B0X0mE9wUcJc1lZGRhG1vWc8u6Iy"), []byte(password))
		return User{}, ErrBadCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return User{}, ErrBadCredentials
	}
	return user, nil
}

func newSessionToken() (token, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256Sum(token)
	return token, sum, nil
}

// StartSession mints a session for the user and returns the raw cookie value.
// Only the SHA-256 hash of the token is persisted.
func (s *Store) StartSession(ctx context.Context, userID int64) (token string, expires time.Time, err error) {
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires = time.Now().Add(sessionDuration)
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO sessions(token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, tokenHash, userID, expires)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert session: %w", err)
	}
	return token, expires, nil
}

// UserForToken resolves a raw session token to its user, if the session is live.
func (s *Store) UserForToken(ctx context.Context, token string) (User, error) {
	var user User
	err := s.Pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()
	`, sha256Sum(token)).Scan(&user.ID, &user.Username, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("resolve session: %w", err)
	}
	return user, nil
}

// UserForID looks up a user by primary key.
func (s *Store) UserForID(ctx context.Context, userID int64) (User, error) {
	var user User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, username, created_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Username, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup user by id: %w", err)
	}
	return user, nil
}

// EndSession deletes the session row for a logout.
func (s *Store) EndSession(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, sha256Sum(token))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
