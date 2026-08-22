package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/michaelc143/gitcastle/internal/repos"
)

type RepositoryService interface {
	Create(ctx context.Context, input repos.CreateInput) (repos.Repository, error)
	Get(ctx context.Context, owner, name string) (repos.Repository, error)
	List(ctx context.Context) ([]repos.Repository, error)
}

type Handler struct {
	Repositories RepositoryService
	Logger       *slog.Logger
}

func NewHandler(repositoryService RepositoryService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := Handler{Repositories: repositoryService, Logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/v1/repositories", h.listRepositories)
	mux.HandleFunc("POST /api/v1/repositories", h.createRepository)
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}", h.getRepository)
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
	var input repos.CreateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	repository, err := h.Repositories.Create(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
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

func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
