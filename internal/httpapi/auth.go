package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/michaelc143/gitcastle/internal/auth"
)

const sessionCookie = "gitcastle_session"

// SessionCookieName exposes the cookie name for tests and clients.
const SessionCookieName = sessionCookie

type contextKey struct{}

// Authenticator is the subset of the auth store the HTTP layer needs.
type Authenticator interface {
	CreateUser(ctx context.Context, username, password string) (auth.User, error)
	Authenticate(ctx context.Context, username, password string) (auth.User, error)
	StartSession(ctx context.Context, userID int64) (token string, expires time.Time, err error)
	UserForToken(ctx context.Context, token string) (auth.User, error)
	EndSession(ctx context.Context, token string) error
}

// WithUser stores the authenticated user in the request context.
func WithUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

// UserFrom returns the authenticated user, if any.
func UserFrom(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(contextKey{}).(auth.User)
	return user, ok
}

func (h Handler) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// currentUser resolves the session cookie into a user.
func (h Handler) currentUser(r *http.Request) (auth.User, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return auth.User{}, auth.ErrNoSession
	}
	return h.Auth.UserForToken(r.Context(), cookie.Value)
}

// requireUser wraps a handler so only authenticated requests pass through.
func (h Handler) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := h.currentUser(r)
		if err != nil {
			h.writeError(w, err)
			return
		}
		next(w, r.WithContext(WithUser(r.Context(), user)))
	}
}

// authenticateRequest supports both session cookies (browser) and HTTP basic
// auth (git clients), returning the authenticated user.
func (h Handler) authenticateRequest(r *http.Request) (auth.User, error) {
	username, password, ok := r.BasicAuth()
	if ok {
		return h.Auth.Authenticate(r.Context(), username, password)
	}
	return h.currentUser(r)
}

func (h Handler) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	user, err := h.Auth.CreateUser(r.Context(), input.Username, input.Password)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	user, err := h.Auth.Authenticate(r.Context(), input.Username, input.Password)
	if err != nil {
		h.writeError(w, err)
		return
	}
	token, expires, err := h.Auth.StartSession(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.setSessionCookie(w, token, expires)
	writeJSON(w, http.StatusOK, user)
}

func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		if err := h.Auth.EndSession(r.Context(), cookie.Value); err != nil {
			h.writeError(w, err)
			return
		}
	}
	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not logged in"})
		return
	}
	writeJSON(w, http.StatusOK, user)
}
