package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/michaelc143/gitcastle/internal/audit"
	"github.com/michaelc143/gitcastle/internal/auth"
)

// Auditor records security-relevant events.
type Auditor interface {
	Record(ctx context.Context, actor, action, target string, details map[string]any, remoteIP string) error
	List(ctx context.Context, actor, action string, limit int) ([]audit.Entry, error)
}

// ProfileManager reads and updates user profiles.
type ProfileManager interface {
	GetProfile(ctx context.Context, username string) (auth.Profile, error)
	UpdateProfile(ctx context.Context, username string, profile auth.Profile) (auth.Profile, error)
}

// BackupObject describes one stored backup.
type BackupObject struct {
	Key      string    `json:"key"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// BackupManager creates and lists repository backups.
type BackupManager interface {
	Backup(ctx context.Context, owner, name string) (string, error)
	List(ctx context.Context, owner, name string) ([]BackupObject, error)
}

func (h Handler) registerHardeningRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/{username}/profile", h.getProfileRoute)
	mux.HandleFunc("PUT /api/v1/users/{username}/profile", h.requireUser(h.updateProfileRoute))
	if h.Audit != nil {
		mux.HandleFunc("GET /api/v1/admin/audit", h.requireUser(h.listAuditRoute))
	}
	if h.Backups != nil {
		mux.HandleFunc("POST /api/v1/repositories/{owner}/{name}/backups", h.requireUser(h.createBackupRoute))
		mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}/backups", h.requireUser(h.listBackupsRoute))
	}
}

// --- profiles ---

func (h Handler) getProfileRoute(w http.ResponseWriter, r *http.Request) {
	if h.Profiles == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "profiles disabled"})
		return
	}
	profile, err := h.Profiles.GetProfile(r.Context(), r.PathValue("username"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h Handler) updateProfileRoute(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())
	// Users may only edit their own profile.
	if user.Username != r.PathValue("username") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "may only edit your own profile"})
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
		Bio         string `json:"bio"`
		Location    string `json:"location"`
		Website     string `json:"website"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	profile, err := h.Profiles.UpdateProfile(r.Context(), user.Username, auth.Profile{
		DisplayName: input.DisplayName,
		Bio:         input.Bio,
		Location:    input.Location,
		Website:     input.Website,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// --- audit ---

func (h Handler) listAuditRoute(w http.ResponseWriter, r *http.Request) {
	actor := r.URL.Query().Get("actor")
	action := r.URL.Query().Get("action")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := h.Audit.List(r.Context(), actor, action, limit)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// --- backups ---

func (h Handler) createBackupRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	user, _ := UserFrom(r.Context())
	key, err := h.Backups.Backup(r.Context(), repository.Owner, repository.Name)
	if err != nil {
		h.writeError(w, err)
		return
	}
	_ = h.Audit // audit is optional; record best-effort below when present
	if h.Audit != nil {
		_ = h.Audit.Record(r.Context(), user.Username, "repository_backed_up",
			repository.Owner+"/"+repository.Name, map[string]any{"key": key}, remoteIP(r))
	}
	writeJSON(w, http.StatusCreated, map[string]string{"key": key})
}

func (h Handler) listBackupsRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	objects, err := h.Backups.List(r.Context(), repository.Owner, repository.Name)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": objects})
}


func remoteIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		for i := 0; i < len(forwarded); i++ {
			if forwarded[i] == ',' {
				return trimSpaces(forwarded[:i])
			}
		}
		return trimSpaces(forwarded)
	}
	return r.RemoteAddr
}

func trimSpaces(value string) string {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}
