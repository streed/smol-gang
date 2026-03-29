package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/middleware"
	"github.com/streed/smol-gang/gateway/internal/models"
)

type RepoHandler struct {
	Queries *db.Queries
	Config  *config.Config
}

func (h *RepoHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Name == "" || req.GitURL == "" || req.GitHubOwner == "" || req.GitHubRepo == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "name, git_url, github_owner, and github_repo are required"})
		return
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	claims := middleware.GetUserFromContext(r.Context())

	repo, err := h.Queries.CreateRepository(r.Context(), models.Repository{
		Name:                 req.Name,
		GitURL:               req.GitURL,
		DefaultBranch:        req.DefaultBranch,
		GitHubOwner:          req.GitHubOwner,
		GitHubRepo:           req.GitHubRepo,
		GitHubInstallationID: req.GitHubInstallationID,
		Config:               req.Config,
		CreatedByID:          claims.UserID,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{Error: "repository already linked or database error", Details: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, repo)
}

func (h *RepoHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)

	repos, total, err := h.Queries.ListRepositories(r.Context(), page, perPage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list repositories"})
		return
	}

	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Items:   repos,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

func (h *RepoHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.Queries.GetRepositoryByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "repository not found"})
		return
	}

	writeJSON(w, http.StatusOK, repo)
}

func (h *RepoHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid repository ID"})
		return
	}

	var req models.UpdateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	repo, err := h.Queries.UpdateRepository(r.Context(), id, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update repository"})
		return
	}

	writeJSON(w, http.StatusOK, repo)
}

func (h *RepoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid repository ID"})
		return
	}

	if err := h.Queries.DeleteRepository(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete repository"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *RepoHandler) SyncConfig(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.Queries.GetRepositoryByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "repository not found"})
		return
	}

	// In production, this would clone/fetch the repo and read .smol-gang.yaml
	// For now, return the current config
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "config synced",
		"repo":    repo,
	})
}

func (h *RepoHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.Queries.GetRepositoryByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "repository not found"})
		return
	}

	claims := middleware.GetUserFromContext(r.Context())
	user, err := h.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user.GitHubAccessToken == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "GitHub account not linked"})
		return
	}

	ghReq, _ := http.NewRequest("GET",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", repo.GitHubOwner, repo.GitHubRepo), nil)
	ghReq.Header.Set("Authorization", "Bearer "+user.GitHubAccessToken)
	ghReq.Header.Set("Accept", "application/json")

	ghResp, err := http.DefaultClient.Do(ghReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: "failed to fetch branches"})
		return
	}
	defer ghResp.Body.Close()

	body, _ := io.ReadAll(ghResp.Body)
	var ghBranches []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(body, &ghBranches)

	branches := make([]string, len(ghBranches))
	for i, b := range ghBranches {
		branches[i] = b.Name
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"branches": branches})
}

func (h *RepoHandler) ListGitHubRepos(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	user, err := h.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	if user.GitHubAccessToken == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "GitHub account not linked. Please sign in with GitHub first."})
		return
	}

	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	ghReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/user/repos?sort=updated&per_page=30&page=%s", page), nil)
	ghReq.Header.Set("Authorization", "Bearer "+user.GitHubAccessToken)
	ghReq.Header.Set("Accept", "application/json")

	ghResp, err := http.DefaultClient.Do(ghReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: "failed to fetch GitHub repos"})
		return
	}
	defer ghResp.Body.Close()

	if ghResp.StatusCode != 200 {
		writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: "GitHub API error"})
		return
	}

	body, _ := io.ReadAll(ghResp.Body)
	var ghRepos []map[string]interface{}
	json.Unmarshal(body, &ghRepos)

	// Filter to relevant fields
	type ghRepoSummary struct {
		FullName      string `json:"full_name"`
		Name          string `json:"name"`
		Owner         string `json:"owner"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
		Description   string `json:"description"`
	}

	var result []ghRepoSummary
	for _, r := range ghRepos {
		owner := ""
		if ownerMap, ok := r["owner"].(map[string]interface{}); ok {
			if login, ok := ownerMap["login"].(string); ok {
				owner = login
			}
		}
		result = append(result, ghRepoSummary{
			FullName:      fmt.Sprintf("%v", r["full_name"]),
			Name:          fmt.Sprintf("%v", r["name"]),
			Owner:         owner,
			CloneURL:      fmt.Sprintf("%v", r["clone_url"]),
			DefaultBranch: fmt.Sprintf("%v", r["default_branch"]),
			Private:       r["private"] == true,
			Description:   fmt.Sprintf("%v", r["description"]),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *RepoHandler) ImportGitHubRepo(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	user, err := h.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	if user.GitHubAccessToken == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "GitHub account not linked"})
		return
	}

	var req models.ImportGitHubRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Owner == "" || req.Repo == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "owner and repo are required"})
		return
	}

	// Fetch repo details from GitHub
	ghReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s", req.Owner, req.Repo), nil)
	ghReq.Header.Set("Authorization", "Bearer "+user.GitHubAccessToken)
	ghReq.Header.Set("Accept", "application/json")

	ghResp, err := http.DefaultClient.Do(ghReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.ErrorResponse{Error: "failed to fetch repo from GitHub"})
		return
	}
	defer ghResp.Body.Close()

	if ghResp.StatusCode != 200 {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "repository not found or no access"})
		return
	}

	body, _ := io.ReadAll(ghResp.Body)
	var ghRepo struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	}
	json.Unmarshal(body, &ghRepo)

	repo, err := h.Queries.CreateRepository(r.Context(), models.Repository{
		Name:          ghRepo.Name,
		GitURL:        ghRepo.CloneURL,
		DefaultBranch: ghRepo.DefaultBranch,
		GitHubOwner:   req.Owner,
		GitHubRepo:    req.Repo,
		CreatedByID:   claims.UserID,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{Error: "failed to import repository", Details: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, repo)
}
