// Package gitserve exposes bare repositories over Git's smart HTTP protocol
// by delegating to `git http-backend`, the reference CGI implementation.
package gitserve

import (
	"context"
	"net/http"
	"strings"
)

// Access is the permission level a request needs.
type Access int

const (
	AccessRead Access = iota
	AccessWrite
)

// Authorizer decides whether a user may read or write a repository.
// username/password come from basic auth; empty username means anonymous.
type Authorizer interface {
	Authenticate(ctx context.Context, username, password string) (userID int64, err error)
	CheckAccess(ctx context.Context, userID int64, owner, repo string, access Access) error
}

// AuthHandler wraps a git smart-HTTP handler with authentication and
// authorization. Reads require any valid user; writes require the write role.
type AuthHandler struct {
	Backend http.Handler
	Auth    Authorizer
}

func (h AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="GitCastle"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	userID, err := h.Auth.Authenticate(r.Context(), username, password)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="GitCastle"`)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	access := AccessRead
	if wantsWrite(r) {
		access = AccessWrite
	}

	owner, repo := parseOwnerRepo(r.URL.Path)
	if owner != "" {
		if err := h.Auth.CheckAccess(r.Context(), userID, owner, repo, access); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	h.Backend.ServeHTTP(w, r)
}

// wantsWrite reports whether the request mutates the repository.
func wantsWrite(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	service := r.URL.Query().Get("service")
	return service == "git-receive-pack"
}

// parseOwnerRepo extracts owner and repository name from a full request
// path such as /git/{owner}/{repo}.git/..., tolerating a missing .git suffix.
func parseOwnerRepo(path string) (owner, repo string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] == "" {
		return "", ""
	}
	return parts[1], strings.TrimSuffix(parts[2], ".git")
}
