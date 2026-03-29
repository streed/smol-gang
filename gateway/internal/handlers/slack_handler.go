package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/slack"
)

type SlackHandler struct {
	Config  *config.Config
	Queries *db.Queries
	K8s     slack.SlackK8sActions
}

// HandleSlashCommand handles POST /api/v1/slack/command
// This is the endpoint configured as the Slash Command Request URL in Slack
func (h *SlackHandler) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	// Verify the request came from Slack
	if !slack.VerifySlackSignature(r, h.Config.SlackSigningSecret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	cmd := slack.ParseSlashCommand(r)
	response := slack.HandleSlashCommand(cmd, h.Queries, h.K8s)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleInteraction handles POST /api/v1/slack/interact
// This is the endpoint configured as the Interactivity Request URL in Slack
func (h *SlackHandler) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	// Verify the request came from Slack
	if !slack.VerifySlackSignature(r, h.Config.SlackSigningSecret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		http.Error(w, "missing payload", http.StatusBadRequest)
		return
	}

	var payload slack.InteractionPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		log.Printf("slack: failed to parse interaction payload: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	response := slack.HandleInteraction(payload, h.Queries, h.K8s)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
