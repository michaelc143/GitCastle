package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/michaelc143/gitcastle/internal/auth"
	"github.com/michaelc143/gitcastle/internal/collab"
	"github.com/michaelc143/gitcastle/internal/gitdata"
	"github.com/michaelc143/gitcastle/internal/repos"
)

type RepositoryService interface {
	Create(ctx context.Context, input repos.CreateInput) (repos.Repository, error)
	Get(ctx context.Context, owner, name string) (repos.Repository, error)
	List(ctx context.Context) ([]repos.Repository, error)
}

type PermissionGranter interface {
	Grant(ctx context.Context, repositoryID int64, username, role string) error
	RoleFor(ctx context.Context, repositoryID int64, username string) (string, error)
}

type Handler struct {
	Repositories RepositoryService
	Auth         Authenticator
	Permissions  PermissionGranter
	Content      ContentService
	Collab       Collaboration
	Merger       Merger
	Automation   Automation
	Webhooks     WebhookManager
	Jobs         JobStore
	Secrets      SecretManager
	Pushes       PushNotifier
	InternalToken string
	Logger       *slog.Logger
}

// Options carries the optional services; zero values fall back to defaults.
type Options struct {
	Content    ContentService // defaults to DiskContent over repositoryService
	Collab     Collaboration  // nil disables the collaboration routes
	Merger     Merger         // nil makes merges state-only (no git operation)
	Automation Automation     // nil disables event fan-out after merges
	Webhooks   WebhookManager // nil disables webhook management routes
	Jobs       JobStore       // nil disables job listing routes
	Secrets    SecretManager  // nil disables secret management routes
	Pushes     PushNotifier   // nil disables push notification intake
	InternalToken string      // shared secret for post-receive hooks
}

func NewHandler(repositoryService RepositoryService, authenticator Authenticator, permissions PermissionGranter, logger *slog.Logger, opts Options) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	contentService := opts.Content
	if contentService == nil {
		contentService = DiskContent{Repositories: repositoryService}
	}
	h := Handler{
		Repositories: repositoryService,
		Auth:         authenticator,
		Permissions:  permissions,
		Content:      contentService,
		Collab:       opts.Collab,
		Merger:       opts.Merger,
		Automation:   opts.Automation,
		Webhooks:     opts.Webhooks,
		Jobs:         opts.Jobs,
		Secrets:      opts.Secrets,
		Pushes:       opts.Pushes,
		InternalToken: opts.InternalToken,
		Logger:       logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /api/v1/register", h.register)
	mux.HandleFunc("POST /api/v1/login", h.login)
	mux.HandleFunc("POST /api/v1/logout", h.logout)
	mux.HandleFunc("GET /api/v1/me", h.requireUser(h.me))
	mux.HandleFunc("GET /api/v1/repositories", h.requireUser(h.listRepositories))
	mux.HandleFunc("POST /api/v1/repositories", h.requireUser(h.createRepository))
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}", h.requireUser(h.getRepository))
	h.registerContentRoutes(mux)
	if h.Collab != nil {
		h.registerCollabRoutes(mux)
	}
	if h.Webhooks != nil {
		h.registerAutomationRoutes(mux)
	}
	return loggingMiddleware(mux, logger)
}

func (h Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h Handler) listRepositories(w http.ResponseWriter, r *http.Request) {
	repositories, err := h.Repositories.List(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repositories})
}

func (h Handler) createRepository(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())
	var input repos.CreateInput
	if !h.decodeJSON(w, r, &input) {
		return
	}
	if input.Owner == "" {
		input.Owner = user.Username
	}
	repository, err := h.Repositories.Create(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	// The creator becomes the repository admin.
	if h.Permissions != nil {
		if err := h.Permissions.Grant(r.Context(), repository.ID, user.Username, auth.RoleAdmin); err != nil {
			h.writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, repository)
}

func (h Handler) getRepository(w http.ResponseWriter, r *http.Request) {
	repository, err := h.Repositories.Get(r.Context(), r.PathValue("owner"), r.PathValue("name"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repository)
}

func (h Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	switch {
	case errors.Is(err, repos.ErrAlreadyExists):
		status = http.StatusConflict
		message = "repository already exists"
	case errors.Is(err, repos.ErrNotFound):
		status = http.StatusNotFound
		message = "repository not found"
	case errors.Is(err, gitdata.ErrNotFound):
		status = http.StatusNotFound
		message = "revision or path not found"
	case errors.Is(err, collab.ErrNotFound):
		status = http.StatusNotFound
		message = "not found"
	case errors.Is(err, collab.ErrInvalidState):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, collab.ErrNotMergeable), errors.Is(err, collab.ErrReviewRequired):
		status = http.StatusUnprocessableEntity
		message = err.Error()
	case errors.Is(err, collab.ErrProtected):
		status = http.StatusForbidden
		message = "branch is protected"
	case errors.Is(err, auth.ErrUserExists):
		status = http.StatusConflict
		message = "user already exists"
	case errors.Is(err, auth.ErrInvalidUsername):
		status = http.StatusBadRequest
		message = "invalid username: use 1-40 letters, numbers, dashes, or underscores"
	case errors.Is(err, auth.ErrWeakPassword):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, auth.ErrBadCredentials):
		status = http.StatusUnauthorized
		message = "invalid username or password"
	case errors.Is(err, auth.ErrNoSession):
		status = http.StatusUnauthorized
		message = "authentication required"
	case strings.HasPrefix(err.Error(), "invalid "):
		status = http.StatusBadRequest
		message = err.Error()
	}
	h.Logger.Error("request failed", "error", err, "status", status)
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// decodeJSON parses a bounded JSON body into target, replying with a 400 on
// malformed input.
func (h Handler) decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
