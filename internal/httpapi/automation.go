package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/michaelc143/gitcastle/internal/ci"
	"github.com/michaelc143/gitcastle/internal/secrets"
	"github.com/michaelc143/gitcastle/internal/webhooks"
)

// WebhookManager manages repository webhooks and records deliveries.
type WebhookManager interface {
	CreateHook(ctx context.Context, repositoryID int64, url, secret string, events []string) (webhooks.Hook, error)
	ListAllHooks(ctx context.Context, repositoryID int64) ([]webhooks.Hook, error)
	DeleteHook(ctx context.Context, repositoryID, hookID int64) error
	ListDeliveries(ctx context.Context, repositoryID int64) ([]webhooks.Delivery, error)
}

// JobStore lists build jobs for a repository.
type JobStore interface {
	ListJobs(ctx context.Context, repositoryID int64, limit int) ([]ci.Job, error)
}

// SecretManager stores encrypted deployment secrets.
type SecretManager interface {
	Set(ctx context.Context, repositoryID int64, name, value string) error
	List(ctx context.Context, repositoryID int64) ([]secrets.Secret, error)
	Delete(ctx context.Context, repositoryID int64, name string) error
}

func (h Handler) registerAutomationRoutes(mux *http.ServeMux) {
	base := "/api/v1/repositories/{owner}/{name}"
	mux.HandleFunc("GET "+base+"/webhooks", h.requireUser(h.listWebhooksRoute))
	mux.HandleFunc("POST "+base+"/webhooks", h.requireUser(h.createWebhookRoute))
	mux.HandleFunc("DELETE "+base+"/webhooks/{id}", h.requireUser(h.deleteWebhookRoute))
	mux.HandleFunc("GET "+base+"/webhooks/deliveries", h.requireUser(h.listDeliveriesRoute))
	mux.HandleFunc("GET "+base+"/jobs", h.requireUser(h.listJobsRoute))
	mux.HandleFunc("GET "+base+"/secrets", h.requireUser(h.listSecretsRoute))
	mux.HandleFunc("PUT "+base+"/secrets/{name}", h.requireUser(h.putSecretRoute))
	mux.HandleFunc("DELETE "+base+"/secrets/{name}", h.requireUser(h.deleteSecretRoute))
}

func parseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (h Handler) listWebhooksRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	hooks, err := h.Webhooks.ListAllHooks(r.Context(), repository.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": hooks})
}

func (h Handler) createWebhookRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	var input struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	if len(input.Events) == 0 {
		input.Events = []string{"push", "pull_request"}
	}
	hook, err := h.Webhooks.CreateHook(r.Context(), repository.ID, input.URL, input.Secret, input.Events)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, hook)
}

func (h Handler) deleteWebhookRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	id, valid := parseID(r)
	if !valid {
		badNumber(w)
		return
	}
	if err := h.Webhooks.DeleteHook(r.Context(), repository.ID, id); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h Handler) listDeliveriesRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	deliveries, err := h.Webhooks.ListDeliveries(r.Context(), repository.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deliveries": deliveries})
}

func (h Handler) listJobsRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	jobs, err := h.Jobs.ListJobs(r.Context(), repository.ID, 20)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h Handler) listSecretsRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	secretsList, err := h.Secrets.List(r.Context(), repository.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": secretsList})
}

func (h Handler) putSecretRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		badNumber(w)
		return
	}
	var input struct {
		Value string `json:"value"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	if err := h.Secrets.Set(r.Context(), repository.ID, name, input.Value); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": name})
}

func (h Handler) deleteSecretRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	if err := h.Secrets.Delete(r.Context(), repository.ID, r.PathValue("name")); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
