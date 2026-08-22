// Package secrets stores deployment secrets encrypted at rest with AES-256-GCM.
// The key comes from the environment; values never leave the server unencrypted.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
	Key  []byte // 32 bytes for AES-256
}

type Secret struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func NewStore(pool *pgxpool.Pool, key []byte) (*Store, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	return &Store{Pool: pool, Key: key}, nil
}

func (s *Store) cipher() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.Key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Store) encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	aead, err := s.cipher()
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	return aead.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func (s *Store) decrypt(ciphertext, nonce []byte) ([]byte, error) {
	aead, err := s.cipher()
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret (wrong key?): %w", err)
	}
	return plaintext, nil
}

// Set encrypts and upserts a secret value.
func (s *Store) Set(ctx context.Context, repositoryID int64, name, value string) error {
	ciphertext, nonce, err := s.encrypt([]byte(value))
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO deploy_secrets(repository_id, name, ciphertext, nonce)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (repository_id, name)
		DO UPDATE SET ciphertext = EXCLUDED.ciphertext, nonce = EXCLUDED.nonce
	`, repositoryID, name, ciphertext, nonce)
	if err != nil {
		return fmt.Errorf("store secret: %w", err)
	}
	return nil
}

// Get decrypts and returns a secret value.
func (s *Store) Get(ctx context.Context, repositoryID int64, name string) (string, error) {
	var ciphertext, nonce []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT ciphertext, nonce FROM deploy_secrets
		WHERE repository_id = $1 AND name = $2
	`, repositoryID, name).Scan(&ciphertext, &nonce)
	if err != nil {
		return "", fmt.Errorf("load secret: %w", err)
	}
	plaintext, err := s.decrypt(ciphertext, nonce)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// List returns secret names only — never values.
func (s *Store) List(ctx context.Context, repositoryID int64) ([]Secret, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT name, created_at FROM deploy_secrets WHERE repository_id = $1 ORDER BY name
	`, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()
	secrets := []Secret{}
	for rows.Next() {
		var secret Secret
		if err := rows.Scan(&secret.Name, &secret.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan secret: %w", err)
		}
		secrets = append(secrets, secret)
	}
	return secrets, rows.Err()
}

// Delete removes a secret.
func (s *Store) Delete(ctx context.Context, repositoryID int64, name string) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM deploy_secrets WHERE repository_id = $1 AND name = $2
	`, repositoryID, name)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("secret not found")
	}
	return nil
}
