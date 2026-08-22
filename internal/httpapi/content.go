package httpapi

import (
	"context"
	"net/http"

	"github.com/michaelc143/gitcastle/internal/gitdata"
)

// ContentService exposes repository content reads for the web interface.
type ContentService interface {
	Repo(owner, name string) (gitdata.Repo, error)
}

// DiskContent resolves repositories to their on-disk bare paths.
type DiskContent struct {
	Repositories RepositoryService
}

func (d DiskContent) Repo(owner, name string) (gitdata.Repo, error) {
	repository, err := d.Repositories.Get(context.Background(), owner, name)
	if err != nil {
		return gitdata.Repo{}, err
	}
	return gitdata.Repo{Path: repository.Path}, nil
}

// registerContentRoutes mounts the read-only content endpoints; all require
// an authenticated user. Revision and in-repository path are passed
// separately: {rev} is one path segment (a branch, tag, or hash) and the
// location inside the tree travels in the "path" query parameter.
func (h Handler) registerContentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}/refs", h.requireUser(h.listRefs))
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}/tree/{rev}", h.requireUser(h.listTreeRoute))
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}/blob/{rev}", h.requireUser(h.getBlobRoute))
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}/commits/{rev}", h.requireUser(h.listCommitsRoute))
	mux.HandleFunc("GET /api/v1/repositories/{owner}/{name}/commit/{hash}", h.requireUser(h.getCommitRoute))
}

func (h Handler) openRepo(w http.ResponseWriter, r *http.Request) (gitdata.Repo, bool) {
	repo, err := h.Content.Repo(r.PathValue("owner"), r.PathValue("name"))
	if err != nil {
		h.writeError(w, err)
		return gitdata.Repo{}, false
	}
	return repo, true
}

func (h Handler) listRefs(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.openRepo(w, r)
	if !ok {
		return
	}
	refs, head, err := repo.ListRefs(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"refs": refs, "head": head})
}

func (h Handler) listTreeRoute(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.openRepo(w, r)
	if !ok {
		return
	}
	entries, err := repo.ListTree(r.Context(), r.PathValue("rev"), r.URL.Query().Get("path"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (h Handler) getBlobRoute(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.openRepo(w, r)
	if !ok {
		return
	}
	content, tooLarge, err := repo.Blob(r.Context(), r.PathValue("rev"), r.URL.Query().Get("path"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	if tooLarge {
		writeJSON(w, http.StatusOK, map[string]any{"too_large": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": string(content), "too_large": false})
}

func (h Handler) listCommitsRoute(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.openRepo(w, r)
	if !ok {
		return
	}
	commits, err := repo.ListCommits(r.Context(), r.PathValue("rev"), "", 50)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commits": commits})
}

func (h Handler) getCommitRoute(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.openRepo(w, r)
	if !ok {
		return
	}
	commit, patch, err := repo.GetCommit(r.Context(), r.PathValue("hash"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commit": commit, "patch": patch})
}
