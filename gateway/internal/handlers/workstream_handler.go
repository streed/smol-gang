package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/auth"
	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/k8s"
	"github.com/streed/smol-gang/gateway/internal/middleware"
	"github.com/streed/smol-gang/gateway/internal/models"
	"github.com/streed/smol-gang/gateway/internal/ws"
)

type WorkstreamHandler struct {
	Queries *db.Queries
	K8s     *k8s.Client
	Config  *config.Config
	Hub     *ws.Hub
}

func (h *WorkstreamHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateWorkstreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Name == "" || req.RepositoryID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "name and repository_id are required"})
		return
	}

	repo, err := h.Queries.GetRepositoryByID(r.Context(), req.RepositoryID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "repository not found"})
		return
	}

	claims := middleware.GetUserFromContext(r.Context())

	// Use provided branch name or generate a new one
	branchName := req.BranchName
	if branchName == "" {
		shortID := uuid.New().String()[:8]
		branchName = fmt.Sprintf("smol/%s-%s", sanitizeBranchName(req.Name), shortID)
	}

	// Merge port mappings: repo config defaults + request overrides
	portMappings := req.PortMappings
	if len(portMappings) == 0 && repo.Config != nil {
		portMappings = repo.Config.PortMappings
	}

	// LLM config: only store if explicitly provided in request
	// Pod template handles defaults (in-cluster Ollama, global config)
	llmConfig := req.LLMConfig

	workstream, err := h.Queries.CreateWorkstream(r.Context(), models.Workstream{
		Name:         req.Name,
		Description:  req.Description,
		RepositoryID: req.RepositoryID,
		BranchName:   branchName,
		Status:       "pending",
		PortMappings: portMappings,
		LLMConfig:    llmConfig,
		CreatedByID:  claims.UserID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create workstream", Details: err.Error()})
		return
	}

	// Fetch user's GitHub token for repo cloning
	user, _ := h.Queries.GetUserByID(r.Context(), claims.UserID)
	gitToken := user.GitHubAccessToken

	// Generate a long-lived gateway token for the agent to call back
	gatewayToken, _ := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, h.Config.JWTSecret, 7*24*time.Hour)

	// Provision K8s pod asynchronously (use background context since request will end)
	go func() {
		bgCtx := context.Background()
		_ = h.Queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "provisioning")

		if h.K8s == nil {
			_ = h.Queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "failed")
			h.Hub.BroadcastToWorkstream(workstream.ID.String(), ws.Message{
				Type:    "status_change",
				Content: "provisioning failed: k8s client not initialized",
			})
			return
		}

		extraEnv := map[string]string{
			"GIT_TOKEN":     gitToken,
			"GATEWAY_TOKEN": gatewayToken,
		}

		podName, serviceName, provErr := h.K8s.CreateAgentPod(bgCtx, workstream, repo, extraEnv)
		if provErr != nil {
			_ = h.Queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "failed")
			h.Hub.BroadcastToWorkstream(workstream.ID.String(), ws.Message{
				Type:    "status_change",
				Content: fmt.Sprintf("provisioning failed: %v", provErr),
			})
			return
		}

		_ = h.Queries.UpdateWorkstreamPod(bgCtx, workstream.ID, podName, serviceName)
		_ = h.Queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "running")

		h.Hub.BroadcastToWorkstream(workstream.ID.String(), ws.Message{
			Type:    "status_change",
			Content: "agent is running",
		})
	}()

	writeJSON(w, http.StatusCreated, workstream)
}

func (h *WorkstreamHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)

	var repoID *uuid.UUID
	if rid := r.URL.Query().Get("repository_id"); rid != "" {
		id, err := uuid.Parse(rid)
		if err == nil {
			repoID = &id
		}
	}
	status := r.URL.Query().Get("status")

	workstreams, total, err := h.Queries.ListWorkstreams(r.Context(), repoID, status, page, perPage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list workstreams"})
		return
	}

	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Items:   workstreams,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

func (h *WorkstreamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	writeJSON(w, http.StatusOK, workstream)
}

func (h *WorkstreamHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	claims := middleware.GetUserFromContext(r.Context())

	msg, err := h.Queries.CreateMessage(r.Context(), models.Message{
		WorkstreamID: id,
		UserID:       &claims.UserID,
		Source:       "user",
		Content:      req.Content,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save message"})
		return
	}

	// Forward message to agent pod
	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err == nil && workstream.PodName != "" && h.K8s != nil {
		go h.K8s.SendMessageToAgent(context.Background(), workstream.PodName, req.Content)
	}

	// Broadcast to WebSocket clients
	h.Hub.BroadcastToWorkstream(id.String(), ws.Message{
		Type:    "chat",
		Content: req.Content,
		Source:  "user",
	})

	writeJSON(w, http.StatusOK, msg)
}

func (h *WorkstreamHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	page, perPage := parsePagination(r)

	messages, total, err := h.Queries.ListMessages(r.Context(), id, page, perPage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list messages"})
		return
	}

	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Items:   messages,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

func (h *WorkstreamHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	if workstream.Status != "running" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "workstream is not running"})
		return
	}

	_ = h.Queries.UpdateWorkstreamStatus(r.Context(), id, "completing")

	// Tell agent to wrap up, push, and create PR (use background context)
	go func() {
		bgCtx := context.Background()

		if h.K8s == nil {
			_ = h.Queries.UpdateWorkstreamStatus(bgCtx, id, "failed")
			return
		}

		prURL, completeErr := h.K8s.CompleteWorkstream(bgCtx, workstream)
		if completeErr != nil {
			_ = h.Queries.UpdateWorkstreamStatus(bgCtx, id, "failed")
			return
		}

		if prURL != "" {
			_ = h.Queries.UpdateWorkstreamPR(bgCtx, id, prURL)
		}

		// Clean up K8s resources
		_ = h.K8s.DeleteAgentPod(bgCtx, workstream.PodName, workstream.ServiceName)
		_ = h.Queries.UpdateWorkstreamStatus(bgCtx, id, "completed")

		h.Hub.BroadcastToWorkstream(id.String(), ws.Message{
			Type:    "status_change",
			Content: fmt.Sprintf("completed, PR: %s", prURL),
		})
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "completing"})
}

func (h *WorkstreamHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	// Clean up K8s resources
	if workstream.PodName != "" && h.K8s != nil {
		go h.K8s.DeleteAgentPod(context.Background(), workstream.PodName, workstream.ServiceName)
	}

	_ = h.Queries.UpdateWorkstreamStatus(r.Context(), id, "cancelled")

	h.Hub.BroadcastToWorkstream(id.String(), ws.Message{
		Type:    "status_change",
		Content: "cancelled",
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *WorkstreamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	// Delete associated messages first
	h.Queries.DeleteWorkstreamMessages(r.Context(), id)

	if err := h.Queries.DeleteWorkstream(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete workstream"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkstreamHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	if workstream.PodName == "" {
		writeJSON(w, http.StatusOK, map[string]string{"logs": ""})
		return
	}

	container := r.URL.Query().Get("container")
	logs, err := h.K8s.GetPodLogs(r.Context(), workstream.PodName, container)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get pod logs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (h *WorkstreamHandler) GetPorts(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	if workstream.ServiceName == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ports": []interface{}{}})
		return
	}

	endpoints, err := h.K8s.GetServiceEndpoints(r.Context(), workstream.ServiceName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get service endpoints"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ports": endpoints})
}

// AgentMessage handles callbacks from agent pods
func (h *WorkstreamHandler) AgentMessage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	msg, err := h.Queries.CreateMessage(r.Context(), models.Message{
		WorkstreamID: id,
		Source:       "agent",
		Content:      req.Content,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save message"})
		return
	}

	h.Hub.BroadcastToWorkstream(id.String(), ws.Message{
		Type:    "chat",
		Content: req.Content,
		Source:  "agent",
	})

	writeJSON(w, http.StatusOK, msg)
}

// AgentStatusUpdate handles status updates from agent pods
func (h *WorkstreamHandler) AgentStatusUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	var req struct {
		Status string `json:"status"`
		PRURL  string `json:"pull_request_url,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Status != "" {
		_ = h.Queries.UpdateWorkstreamStatus(r.Context(), id, req.Status)
	}
	if req.PRURL != "" {
		_ = h.Queries.UpdateWorkstreamPR(r.Context(), id, req.PRURL)
	}

	h.Hub.BroadcastToWorkstream(id.String(), ws.Message{
		Type:    "status_change",
		Content: req.Status,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Terminal provides an interactive web terminal into a workstream's container.
// Defaults to the "app" container, but can target "agent" or "dind" via URL param.
func (h *WorkstreamHandler) Terminal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	if workstream.PodName == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "workstream has no pod"})
		return
	}

	container := chi.URLParam(r, "container")
	if container == "" {
		container = "app" // default to app container for user interaction
	}

	// Validate container name
	validContainers := map[string]bool{"app": true, "agent": true, "dind": true}
	if !validContainers[container] {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid container name, must be: app, agent, or dind"})
		return
	}

	if err := h.K8s.HandleTerminal(w, r, workstream.PodName, container); err != nil {
		// WebSocket already upgraded, can't send HTTP error
		return
	}
}

func (h *WorkstreamHandler) GetDiff(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid workstream ID"})
		return
	}

	workstream, err := h.Queries.GetWorkstreamByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "workstream not found"})
		return
	}

	if workstream.PodName == "" || h.K8s == nil {
		writeJSON(w, http.StatusOK, map[string]string{"stat": "", "diff": ""})
		return
	}

	respBody, err := h.K8s.ProxyGet(r.Context(), workstream.PodName, "diff")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"stat": "", "diff": ""})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

func sanitizeBranchName(name string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "")
	return strings.ToLower(replacer.Replace(name))
}
