package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/db"
	"github.com/streed/smol-cluster/gateway/internal/k8s"
	"github.com/streed/smol-cluster/gateway/internal/middleware"
	"github.com/streed/smol-cluster/gateway/internal/models"
	"github.com/streed/smol-cluster/gateway/internal/ws"
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

	// Generate branch name from workstream name
	branchName := fmt.Sprintf("smol/%s", sanitizeBranchName(req.Name))

	// Merge port mappings: repo config defaults + request overrides
	portMappings := req.PortMappings
	if len(portMappings) == 0 && repo.Config != nil {
		portMappings = repo.Config.PortMappings
	}

	// Merge LLM config: request override > repo default > global default
	llmConfig := req.LLMConfig
	if llmConfig == nil {
		llmConfig = &models.LLMConfig{
			APIURL: h.Config.LLMApiURL,
			APIKey: h.Config.LLMApiKey,
			Model:  h.Config.LLMModel,
		}
	}

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

	// Provision K8s pod asynchronously
	go func() {
		_ = h.Queries.UpdateWorkstreamStatus(r.Context(), workstream.ID, "provisioning")

		podName, serviceName, provErr := h.K8s.CreateAgentPod(r.Context(), workstream, repo)
		if provErr != nil {
			_ = h.Queries.UpdateWorkstreamStatus(r.Context(), workstream.ID, "failed")
			h.Hub.BroadcastToWorkstream(workstream.ID.String(), ws.Message{
				Type:    "status_change",
				Content: fmt.Sprintf("provisioning failed: %v", provErr),
			})
			return
		}

		_ = h.Queries.UpdateWorkstreamPod(r.Context(), workstream.ID, podName, serviceName)
		_ = h.Queries.UpdateWorkstreamStatus(r.Context(), workstream.ID, "running")

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
	if err == nil && workstream.PodName != "" {
		go h.K8s.SendMessageToAgent(r.Context(), workstream.ServiceName, req.Content)
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

	// Tell agent to wrap up, push, and create PR
	go func() {
		prURL, completeErr := h.K8s.CompleteWorkstream(r.Context(), workstream)
		if completeErr != nil {
			_ = h.Queries.UpdateWorkstreamStatus(r.Context(), id, "failed")
			return
		}

		if prURL != "" {
			_ = h.Queries.UpdateWorkstreamPR(r.Context(), id, prURL)
		}

		// Clean up K8s resources
		_ = h.K8s.DeleteAgentPod(r.Context(), workstream.PodName, workstream.ServiceName)
		_ = h.Queries.UpdateWorkstreamStatus(r.Context(), id, "completed")

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
	if workstream.PodName != "" {
		go h.K8s.DeleteAgentPod(r.Context(), workstream.PodName, workstream.ServiceName)
	}

	_ = h.Queries.UpdateWorkstreamStatus(r.Context(), id, "cancelled")

	h.Hub.BroadcastToWorkstream(id.String(), ws.Message{
		Type:    "status_change",
		Content: "cancelled",
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
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

	logs, err := h.K8s.GetPodLogs(r.Context(), workstream.PodName)
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

func sanitizeBranchName(name string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "")
	return strings.ToLower(replacer.Replace(name))
}
