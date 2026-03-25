package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/streed/smol-cluster/gateway/internal/db"
	"github.com/streed/smol-cluster/gateway/internal/middleware"
	"github.com/streed/smol-cluster/gateway/internal/models"
)

type RepoHandler struct {
	Queries *db.Queries
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

	// In production, this would clone/fetch the repo and read .smol-cluster.yaml
	// For now, return the current config
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "config synced",
		"repo":    repo,
	})
}
