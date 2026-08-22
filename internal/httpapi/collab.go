package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/michaelc143/gitcastle/internal/collab"
	"github.com/michaelc143/gitcastle/internal/repos"
)

// Collaboration provides issue, PR, review, and protection operations.
type Collaboration interface {
	CreateIssue(ctx context.Context, repositoryID int64, author, title, body string) (collab.Issue, error)
	ListIssues(ctx context.Context, repositoryID int64, state string) ([]collab.Issue, error)
	GetIssue(ctx context.Context, repositoryID, number int64) (collab.Issue, error)
	SetIssueState(ctx context.Context, repositoryID, number int64, state string) (collab.Issue, error)

	AddComment(ctx context.Context, repositoryID int64, subjectType string, subjectNumber int64, author, body string) (collab.Comment, error)
	ListComments(ctx context.Context, repositoryID int64, subjectType string, subjectNumber int64) ([]collab.Comment, error)

	CreatePullRequest(ctx context.Context, repositoryID int64, author, title, body, sourceBranch, targetBranch string) (collab.PullRequest, error)
	GetPullRequest(ctx context.Context, repositoryID, number int64) (collab.PullRequest, error)
	ListPullRequests(ctx context.Context, repositoryID int64, state string) ([]collab.PullRequest, error)
	SetPullRequestState(ctx context.Context, repositoryID, number int64, state, mergeCommit string) (collab.PullRequest, error)
	UpsertReview(ctx context.Context, repositoryID, prNumber int64, reviewer, verdict, body string) (collab.Review, error)
	ListReviews(ctx context.Context, repositoryID, prNumber int64) ([]collab.Review, error)

	SetBranchProtection(ctx context.Context, repositoryID int64, protection collab.BranchProtection) error
	GetBranchProtection(ctx context.Context, repositoryID int64, branch string) (collab.BranchProtection, error)
	DeleteBranchProtection(ctx context.Context, repositoryID int64, branch string) error
	EvaluateMerge(ctx context.Context, repositoryID, prNumber int64) (collab.MergeCheck, error)
}

// registerCollabRoutes mounts the collaboration API. All routes require an
// authenticated user.
func (h Handler) registerCollabRoutes(mux *http.ServeMux) {
	issue := "/api/v1/repositories/{owner}/{name}/issues"
	pr := "/api/v1/repositories/{owner}/{name}/pulls"

	mux.HandleFunc("GET "+issue, h.requireUser(h.listIssuesRoute))
	mux.HandleFunc("POST "+issue, h.requireUser(h.createIssueRoute))
	mux.HandleFunc("GET "+issue+"/{number}", h.requireUser(h.getIssueRoute))
	mux.HandleFunc("PATCH "+issue+"/{number}", h.requireUser(h.patchIssueRoute))
	mux.HandleFunc("GET "+issue+"/{number}/comments", h.requireUser(h.listCommentsRoute))
	mux.HandleFunc("POST "+issue+"/{number}/comments", h.requireUser(h.addIssueCommentRoute))

	mux.HandleFunc("GET "+pr, h.requireUser(h.listPullRequestsRoute))
	mux.HandleFunc("POST "+pr, h.requireUser(h.createPullRequestRoute))
	mux.HandleFunc("GET "+pr+"/{number}", h.requireUser(h.getPullRequestRoute))
	mux.HandleFunc("POST "+pr+"/{number}/merge", h.requireUser(h.mergePullRequestRoute))
	mux.HandleFunc("GET "+pr+"/{number}/comments", h.requireUser(h.listPRCommentsRoute))
	mux.HandleFunc("POST "+pr+"/{number}/comments", h.requireUser(h.addPRCommentRoute))
	mux.HandleFunc("PUT "+pr+"/{number}/review", h.requireUser(h.putReviewRoute))
	mux.HandleFunc("GET "+pr+"/{number}/reviews", h.requireUser(h.listReviewsRoute))

	protection := "/api/v1/repositories/{owner}/{name}/branches/{branch}/protection"
	mux.HandleFunc("GET "+protection, h.requireUser(h.getProtectionRoute))
	mux.HandleFunc("PUT "+protection, h.requireUser(h.putProtectionRoute))
	mux.HandleFunc("DELETE "+protection, h.requireUser(h.deleteProtectionRoute))
}

// resolveRepo loads the repository row for the URL owner/name.
func (h Handler) resolveRepo(w http.ResponseWriter, r *http.Request) (repos.Repository, bool) {
	repository, err := h.Repositories.Get(r.Context(), r.PathValue("owner"), r.PathValue("name"))
	if err != nil {
		h.writeError(w, err)
		return repos.Repository{}, false
	}
	return repository, true
}

// parseNumber extracts a positive integer {number} path parameter.
func parseNumber(r *http.Request) (int64, bool) {
	number, err := strconv.ParseInt(r.PathValue("number"), 10, 64)
	return number, err == nil && number > 0
}

func badNumber(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid number"})
}

// --- issues ---

func (h Handler) createIssueRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	user, _ := UserFrom(r.Context())
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	issue, err := h.Collab.CreateIssue(r.Context(), repository.ID, user.Username, input.Title, input.Body)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issue)
}

func (h Handler) listIssuesRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	issues, err := h.Collab.ListIssues(r.Context(), repository.ID, r.URL.Query().Get("state"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": issues})
}

func (h Handler) getIssueRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	issue, err := h.Collab.GetIssue(r.Context(), repository.ID, number)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (h Handler) patchIssueRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	var input struct {
		State string `json:"state"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	issue, err := h.Collab.SetIssueState(r.Context(), repository.ID, number, input.State)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

// --- comments (shared between issues and pull requests) ---

func (h Handler) addComment(w http.ResponseWriter, r *http.Request, subjectType string) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	user, _ := UserFrom(r.Context())
	var input struct {
		Body string `json:"body"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	comment, err := h.Collab.AddComment(r.Context(), repository.ID, subjectType, number, user.Username, input.Body)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (h Handler) listComments(w http.ResponseWriter, r *http.Request, subjectType string) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	comments, err := h.Collab.ListComments(r.Context(), repository.ID, subjectType, number)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (h Handler) addIssueCommentRoute(w http.ResponseWriter, r *http.Request) {
	h.addComment(w, r, "issue")
}

func (h Handler) listCommentsRoute(w http.ResponseWriter, r *http.Request) {
	h.listComments(w, r, "issue")
}

func (h Handler) addPRCommentRoute(w http.ResponseWriter, r *http.Request) {
	h.addComment(w, r, "pull_request")
}

func (h Handler) listPRCommentsRoute(w http.ResponseWriter, r *http.Request) {
	h.listComments(w, r, "pull_request")
}

// --- pull requests ---

func (h Handler) createPullRequestRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	user, _ := UserFrom(r.Context())
	var input struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	pr, err := h.Collab.CreatePullRequest(r.Context(), repository.ID, user.Username,
		input.Title, input.Body, input.SourceBranch, input.TargetBranch)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pr)
}

func (h Handler) listPullRequestsRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	prs, err := h.Collab.ListPullRequests(r.Context(), repository.ID, r.URL.Query().Get("state"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pull_requests": prs})
}

func (h Handler) getPullRequestRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	pr, err := h.Collab.GetPullRequest(r.Context(), repository.ID, number)
	if err != nil {
		h.writeError(w, err)
		return
	}
	check, err := h.Collab.EvaluateMerge(r.Context(), repository.ID, number)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pull_request": pr, "merge_check": check})
}

func (h Handler) mergePullRequestRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	check, err := h.Collab.EvaluateMerge(r.Context(), repository.ID, number)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if !check.Mergable {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":    "merge blocked",
			"blockers": check.Blockers,
		})
		return
	}
	pr, err := h.Collab.SetPullRequestState(r.Context(), repository.ID, number, "merged", "")
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

// --- reviews ---

func (h Handler) putReviewRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	user, _ := UserFrom(r.Context())
	var input struct {
		Verdict string `json:"verdict"`
		Body    string `json:"body"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	review, err := h.Collab.UpsertReview(r.Context(), repository.ID, number, user.Username, input.Verdict, input.Body)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (h Handler) listReviewsRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	number, valid := parseNumber(r)
	if !valid {
		badNumber(w)
		return
	}
	reviews, err := h.Collab.ListReviews(r.Context(), repository.ID, number)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

// --- branch protection ---

func (h Handler) getProtectionRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	protection, err := h.Collab.GetBranchProtection(r.Context(), repository.ID, r.PathValue("branch"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, protection)
}

func (h Handler) putProtectionRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	var input struct {
		RequiredApprovals int  `json:"required_approvals"`
		AllowForcePush    bool `json:"allow_force_push"`
	}
	if !h.decodeJSON(w, r, &input) {
		return
	}
	protection := collab.BranchProtection{
		Branch:            r.PathValue("branch"),
		RequiredApprovals: input.RequiredApprovals,
		AllowForcePush:    input.AllowForcePush,
	}
	if err := h.Collab.SetBranchProtection(r.Context(), repository.ID, protection); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, protection)
}

func (h Handler) deleteProtectionRoute(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	if err := h.Collab.DeleteBranchProtection(r.Context(), repository.ID, r.PathValue("branch")); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
