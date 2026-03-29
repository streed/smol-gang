package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/models"
)

type AuditHandler struct {
	Queries *db.Queries
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)

	var userID *uuid.UUID
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		id, err := uuid.Parse(uid)
		if err == nil {
			userID = &id
		}
	}

	var workstreamID *uuid.UUID
	if wid := r.URL.Query().Get("workstream_id"); wid != "" {
		id, err := uuid.Parse(wid)
		if err == nil {
			workstreamID = &id
		}
	}

	action := r.URL.Query().Get("action")

	logs, total, err := h.Queries.ListAuditLogs(r.Context(), userID, workstreamID, action, page, perPage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list audit logs"})
		return
	}

	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Items:   logs,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}
