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
	mux.HandleFunc("POST /api/v1/internal/notify-push", h.notifyPushRoute)
	base := "/api/v1/repositories/{owner}/{name}"
	mux.HandleFunc("GET "+base+"/webhooks", h.requireUser(h.listWebhooksRoute))
	mux.HandleFunc("POST "+base+"/webhooks", h.requireUser(h.createWebhookRoute))
	mux.HandleFunc("DELETE "+base+"/webhooks/{id}", h.requireUser(h.deleteWebhookRoute))
	mux.HandleFunc("GET "+base+"/webhooks/deliveries", h.requireUser(h.listDeliveriesRoute))
	mux.HandleFunc("GET "+base+"/jobs", h.requireUser(h.listJobsRoute))
	mux.HandleFunc("GET "+base+"/secrets", h.requireUser(h.listSecretsRoute))
	mux.HandleFunc("PUT "+base+"/secrets/{secretName}", h.requireUser(h.putSecretRoute))
	mux.HandleFunc("DELETE "+base+"/secrets/{secretName}", h.requireUser(h.deleteSecretRoute))
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
	name := r.PathValue("secretName")
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
	if err := h.Secrets.Delete(r.Context(), repository.ID, r.PathValue("secretName")); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}


// PushNotifier receives push notifications from repository post-receive hooks.
type PushNotifier interface {
	PushReceived(ctx context.Context, event PushEvent)
}

// hookPushPayload is the JSON body posted by post-receive hooks.
type hookPushPayload struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	OldHash string `json:"old_hash"`
	NewHash string `json:"new_hash"`
}

// PushEvent is the enriched notification sent to PushNotifier.
type PushEvent struct {
	RepositoryID int64
	Owner        string
	Name         string
	Branch       string
	OldHash      string
	NewHash      string
}

// notifyPushRoute accepts authenticated post-receive callbacks.
func (h Handler) notifyPushRoute(w http.ResponseWriter, r *http.Request) {
	if h.Pushes == nil || h.InternalToken == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "push notifications disabled"})
		return
	}
	if r.Header.Get("X-GitCastle-Internal-Token") != h.InternalToken {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	var input hookPushPayload
	if !h.decodeJSON(w, r, &input) {
		return
	}
	if input.Owner == "" || input.Name == "" || input.Branch == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner, name, branch required"})
		return
	}
	repository, err := h.Repositories.Get(r.Context(), input.Owner, input.Name)
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.Pushes.PushReceived(r.Context(), PushEvent{
		RepositoryID: repository.ID,
		Owner:        input.Owner,
		Name:         input.Name,
		Branch:       input.Branch,
		OldHash:      input.OldHash,
		NewHash:      input.NewHash,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "notified"})
}
