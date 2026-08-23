package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Profile is the public view of a user account.
type Profile struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	Location    string    `json:"location"`
	Website     string    `json:"website"`
	JoinedAt    time.Time `json:"joined_at"`
}

const maxProfileField = 500

// GetProfile loads a user's public profile by username.
func (s *Store) GetProfile(ctx context.Context, username string) (Profile, error) {
	if err := validateUsername(username); err != nil {
		return Profile{}, err
	}
	var profile Profile
	err := s.Pool.QueryRow(ctx, `
		SELECT username, display_name, bio, location, website, created_at
		FROM users WHERE username = $1
	`, username).Scan(&profile.Username, &profile.DisplayName, &profile.Bio,
		&profile.Location, &profile.Website, &profile.JoinedAt)
	if err != nil {
		if isNoRows(err) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return profile, nil
}

// UpdateProfile updates the caller's own profile. Username is immutable.
func (s *Store) UpdateProfile(ctx context.Context, username string, profile Profile) (Profile, error) {
	for _, field := range []string{profile.DisplayName, profile.Bio, profile.Location} {
		if len(field) > maxProfileField {
			return Profile{}, fmt.Errorf("profile field exceeds %d characters", maxProfileField)
		}
	}
	if profile.Website != "" && !isPlausibleURL(profile.Website) {
		return Profile{}, fmt.Errorf("website must be an http(s) URL")
	}
	var updated Profile
	err := s.Pool.QueryRow(ctx, `
		UPDATE users SET display_name = $2, bio = $3, location = $4, website = $5
		WHERE username = $1
		RETURNING username, display_name, bio, location, website, created_at
	`, username, strings.TrimSpace(profile.DisplayName), profile.Bio,
		strings.TrimSpace(profile.Location), strings.TrimSpace(profile.Website)).Scan(
		&updated.Username, &updated.DisplayName, &updated.Bio,
		&updated.Location, &updated.Website, &updated.JoinedAt)
	if err != nil {
		if isNoRows(err) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return updated, nil
}

func isPlausibleURL(raw string) bool {
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	if strings.ContainsAny(raw, " \t\r\n\"'<>()") {
		return false
	}
	return len(raw) <= 2000
}

func isNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}
