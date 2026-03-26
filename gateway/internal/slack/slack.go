package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/db"
	"github.com/streed/smol-cluster/gateway/internal/models"
	"github.com/streed/smol-cluster/gateway/internal/ws"
)

// Bot is the interface for the Slack integration
type Bot interface {
	Start(ctx context.Context) error
	SendMessage(channel, text string) error
	SendBlocks(channel string, blocks []Block) error
	NotifyWorkstreamEvent(channel string, event WorkstreamEvent) error
}

// WorkstreamEvent represents a workstream lifecycle event for Slack notifications
type WorkstreamEvent struct {
	Type           string // "created", "running", "completed", "failed", "cancelled", "message"
	WorkstreamName string
	WorkstreamID   string
	RepoName       string
	BranchName     string
	UserName       string
	Details        string // PR URL, error message, etc.
}

// SlackK8sActions is the interface for K8s operations needed by Slack commands
type SlackK8sActions interface {
	SendMessageToAgent(ctx context.Context, serviceName, message string) error
	CompleteWorkstream(ctx context.Context, ws models.Workstream) (string, error)
	DeleteAgentPod(ctx context.Context, podName, serviceName string) error
}

// SlackBot implements the Bot interface
type SlackBot struct {
	config  *config.Config
	queries *db.Queries
	hub     *ws.Hub
	client  *http.Client
}

func NewBot(cfg *config.Config, queries *db.Queries, hub *ws.Hub) Bot {
	return &SlackBot{
		config:  cfg,
		queries: queries,
		hub:     hub,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *SlackBot) Start(ctx context.Context) error {
	if b.config.SlackBotToken == "" {
		log.Println("slack: no bot token configured, skipping")
		return nil
	}

	log.Println("slack: bot started - listening for slash commands via webhook")
	<-ctx.Done()
	return nil
}

// SendMessage posts a simple text message to a Slack channel
func (b *SlackBot) SendMessage(channel, text string) error {
	if b.config.SlackBotToken == "" {
		return nil
	}

	payload := map[string]string{
		"channel": channel,
		"text":    text,
	}
	return b.postSlackAPI("chat.postMessage", payload)
}

// SendBlocks posts a Block Kit message to a Slack channel
func (b *SlackBot) SendBlocks(channel string, blocks []Block) error {
	if b.config.SlackBotToken == "" {
		return nil
	}

	payload := map[string]interface{}{
		"channel": channel,
		"blocks":  blocks,
	}
	return b.postSlackAPI("chat.postMessage", payload)
}

// NotifyWorkstreamEvent sends a rich notification about a workstream event
func (b *SlackBot) NotifyWorkstreamEvent(channel string, event WorkstreamEvent) error {
	if b.config.SlackBotToken == "" {
		return nil
	}

	blocks := buildEventBlocks(event)
	return b.SendBlocks(channel, blocks)
}

func (b *SlackBot) postSlackAPI(method string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+b.config.SlackBotToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: API request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("slack: failed to decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack: API error: %s", result.Error)
	}
	return nil
}

// --- Slack Block Kit Types ---

type Block struct {
	Type     string       `json:"type"`
	Text     *TextObject  `json:"text,omitempty"`
	Fields   []TextObject `json:"fields,omitempty"`
	Elements []Element    `json:"elements,omitempty"`
	BlockID  string       `json:"block_id,omitempty"`
}

type TextObject struct {
	Type string `json:"type"` // "mrkdwn" or "plain_text"
	Text string `json:"text"`
}

type Element struct {
	Type     string      `json:"type"`
	Text     *TextObject `json:"text,omitempty"`
	ActionID string      `json:"action_id,omitempty"`
	Value    string      `json:"value,omitempty"`
	Style    string      `json:"style,omitempty"`
}

// --- Slash Command Handler ---

// SlashCommandRequest represents an incoming Slack slash command
type SlashCommandRequest struct {
	Command     string
	Text        string
	UserID      string
	UserName    string
	ChannelID   string
	ChannelName string
	ResponseURL string
	TriggerID   string
}

// SlashCommandResponse is the JSON response sent back to Slack
type SlashCommandResponse struct {
	ResponseType    string  `json:"response_type"` // "in_channel" or "ephemeral"
	Text            string  `json:"text,omitempty"`
	Blocks          []Block `json:"blocks,omitempty"`
	ReplaceOriginal bool    `json:"replace_original,omitempty"`
}

// ParseSlashCommand parses the form-encoded slash command body
func ParseSlashCommand(r *http.Request) SlashCommandRequest {
	return SlashCommandRequest{
		Command:     r.FormValue("command"),
		Text:        r.FormValue("text"),
		UserID:      r.FormValue("user_id"),
		UserName:    r.FormValue("user_name"),
		ChannelID:   r.FormValue("channel_id"),
		ChannelName: r.FormValue("channel_name"),
		ResponseURL: r.FormValue("response_url"),
		TriggerID:   r.FormValue("trigger_id"),
	}
}

// VerifySlackSignature verifies the request came from Slack using HMAC-SHA256
func VerifySlackSignature(r *http.Request, signingSecret string) bool {
	if signingSecret == "" {
		return true // skip verification in dev
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return false
	}

	// Reject requests older than 5 minutes to prevent replay attacks
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 300 {
		return false
	}

	// Read and restore the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Compute expected signature: v0=HMAC-SHA256(signing_secret, "v0:timestamp:body")
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// HandleSlashCommand processes /smol slash commands and returns a Slack response
func HandleSlashCommand(cmd SlashCommandRequest, queries *db.Queries, k8s SlackK8sActions) SlashCommandResponse {
	parts := strings.Fields(cmd.Text)
	if len(parts) == 0 {
		return buildHelpResponse()
	}

	switch parts[0] {
	case "list":
		return handleList(queries, parts[1:])
	case "status":
		return handleStatus(queries, parts[1:])
	case "message", "msg":
		return handleMessage(queries, k8s, parts[1:], cmd.UserName)
	case "start":
		return handleStart(queries, parts[1:], cmd.UserName)
	case "complete":
		return handleComplete(queries, parts[1:])
	case "cancel":
		return handleCancel(queries, parts[1:])
	case "repos":
		return handleRepos(queries)
	case "help":
		return buildHelpResponse()
	default:
		return SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf(":warning: Unknown command: `%s`. Try `/smol help`", parts[0]),
		}
	}
}

// --- Command Implementations ---

func handleList(queries *db.Queries, args []string) SlashCommandResponse {
	status := "running"
	if len(args) > 0 {
		status = args[0]
		if status == "all" {
			status = ""
		}
	}

	workstreams, _, err := queries.ListWorkstreams(context.Background(), nil, status, 1, 10)
	if err != nil {
		return errorResponse("Failed to fetch workstreams: " + err.Error())
	}

	if len(workstreams) == 0 {
		label := status
		if label == "" {
			label = "any"
		}
		return SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("No %s workstreams found.", label),
		}
	}

	blocks := []Block{
		{
			Type: "header",
			Text: &TextObject{Type: "plain_text", Text: fmt.Sprintf("Workstreams (%d)", len(workstreams))},
		},
		{Type: "divider"},
	}

	for _, w := range workstreams {
		emoji := statusToEmoji(w.Status)
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("%s *%s*\n`%s` | Branch: `%s` | Status: %s", emoji, w.Name, w.ID.String()[:8], w.BranchName, w.Status),
			},
		})

		if w.Status == "running" {
			blocks = append(blocks, Block{
				Type: "actions",
				Elements: []Element{
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "View Status"},
						ActionID: "view_status",
						Value:    w.ID.String(),
					},
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "Send Message"},
						ActionID: "send_message_prompt",
						Value:    w.ID.String(),
					},
				},
			})
		}
	}

	return SlashCommandResponse{
		ResponseType: "in_channel",
		Blocks:       blocks,
	}
}

func handleStatus(queries *db.Queries, args []string) SlashCommandResponse {
	if len(args) == 0 {
		return errorResponse("Usage: `/smol status <workstream-id or name>`")
	}

	w, err := findWorkstream(queries, args[0])
	if err != nil {
		return errorResponse(err.Error())
	}

	emoji := statusToEmoji(w.Status)
	fields := []TextObject{
		{Type: "mrkdwn", Text: fmt.Sprintf("*Status:*\n%s %s", emoji, w.Status)},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Branch:*\n`%s`", w.BranchName)},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Created:*\n%s", w.CreatedAt.Format("Jan 2 15:04 UTC"))},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Pod:*\n`%s`", orDefault(w.PodName, "none"))},
	}

	blocks := []Block{
		{
			Type: "header",
			Text: &TextObject{Type: "plain_text", Text: w.Name},
		},
		{Type: "divider"},
		{Type: "section", Fields: fields},
	}

	if w.Description != "" {
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{Type: "mrkdwn", Text: fmt.Sprintf("*Description:*\n%s", w.Description)},
		})
	}

	if w.PullRequestURL != "" {
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{Type: "mrkdwn", Text: fmt.Sprintf("*Pull Request:* <%s|View PR>", w.PullRequestURL)},
		})
	}

	// Action buttons for active workstreams
	if w.Status == "running" {
		blocks = append(blocks, Block{
			Type: "actions",
			Elements: []Element{
				{
					Type:     "button",
					Text:     &TextObject{Type: "plain_text", Text: "Complete"},
					ActionID: "complete_workstream",
					Value:    w.ID.String(),
					Style:    "primary",
				},
				{
					Type:     "button",
					Text:     &TextObject{Type: "plain_text", Text: "Cancel"},
					ActionID: "cancel_workstream",
					Value:    w.ID.String(),
					Style:    "danger",
				},
			},
		})
	}

	return SlashCommandResponse{
		ResponseType: "in_channel",
		Blocks:       blocks,
	}
}

func handleMessage(queries *db.Queries, k8s SlackK8sActions, args []string, userName string) SlashCommandResponse {
	if len(args) < 2 {
		return errorResponse("Usage: `/smol message <workstream-id> <your message>`")
	}

	w, err := findWorkstream(queries, args[0])
	if err != nil {
		return errorResponse(err.Error())
	}

	if w.Status != "running" {
		return errorResponse(fmt.Sprintf("Workstream `%s` is not running (status: %s)", w.Name, w.Status))
	}

	message := strings.Join(args[1:], " ")

	// Save message to database
	_, err = queries.CreateMessage(context.Background(), models.Message{
		WorkstreamID: w.ID,
		Source:       "user",
		Content:      fmt.Sprintf("[slack:%s] %s", userName, message),
	})
	if err != nil {
		return errorResponse("Failed to save message")
	}

	// Forward to agent
	if w.ServiceName != "" {
		go func() {
			if err := k8s.SendMessageToAgent(context.Background(), w.ServiceName, message); err != nil {
				log.Printf("slack: failed to forward message to agent: %v", err)
			}
		}()
	}

	return SlashCommandResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf(":speech_balloon: *%s* sent to *%s*: _%s_", userName, w.Name, message),
	}
}

func handleStart(queries *db.Queries, args []string, userName string) SlashCommandResponse {
	if len(args) < 2 {
		return errorResponse("Usage: `/smol start <repo-name> <workstream-name> [description...]`\n\nUse `/smol repos` to see available repositories.")
	}

	repoName := args[0]
	wsName := args[1]
	description := ""
	if len(args) > 2 {
		description = strings.Join(args[2:], " ")
	}

	// Find repo by name
	repos, _, err := queries.ListRepositories(context.Background(), 1, 100)
	if err != nil {
		return errorResponse("Failed to fetch repositories")
	}

	var repo *models.Repository
	for i := range repos {
		if strings.EqualFold(repos[i].Name, repoName) {
			repo = &repos[i]
			break
		}
	}
	if repo == nil {
		return errorResponse(fmt.Sprintf("Repository `%s` not found. Use `/smol repos` to see available repositories.", repoName))
	}

	blocks := []Block{
		{
			Type: "section",
			Text: &TextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf(":rocket: Starting workstream *%s* on repo *%s*...", wsName, repoName),
			},
		},
	}

	if description != "" {
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{Type: "mrkdwn", Text: fmt.Sprintf("_%s_", description)},
		})
	}

	blocks = append(blocks, Block{
		Type: "context",
		Elements: []Element{
			{Type: "mrkdwn", Text: &TextObject{Type: "mrkdwn", Text: fmt.Sprintf("Requested by %s | Repo: %s/%s", userName, repo.GitHubOwner, repo.GitHubRepo)}},
		},
	})

	return SlashCommandResponse{
		ResponseType: "in_channel",
		Blocks:       blocks,
	}
}

func handleComplete(queries *db.Queries, args []string) SlashCommandResponse {
	if len(args) == 0 {
		return errorResponse("Usage: `/smol complete <workstream-id>`")
	}

	w, err := findWorkstream(queries, args[0])
	if err != nil {
		return errorResponse(err.Error())
	}

	if w.Status != "running" {
		return errorResponse(fmt.Sprintf("Workstream `%s` is not running (status: %s)", w.Name, w.Status))
	}

	return SlashCommandResponse{
		ResponseType: "in_channel",
		Blocks: []Block{
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: fmt.Sprintf(":warning: Complete workstream *%s*? The agent will push changes and create a PR.", w.Name),
				},
			},
			{
				Type: "actions",
				Elements: []Element{
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "Yes, Complete"},
						ActionID: "confirm_complete",
						Value:    w.ID.String(),
						Style:    "primary",
					},
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "No, Keep Running"},
						ActionID: "dismiss_cancel",
						Value:    w.ID.String(),
					},
				},
			},
		},
	}
}

func handleCancel(queries *db.Queries, args []string) SlashCommandResponse {
	if len(args) == 0 {
		return errorResponse("Usage: `/smol cancel <workstream-id>`")
	}

	w, err := findWorkstream(queries, args[0])
	if err != nil {
		return errorResponse(err.Error())
	}

	if !isActive(w.Status) {
		return errorResponse(fmt.Sprintf("Workstream `%s` is not active (status: %s)", w.Name, w.Status))
	}

	return SlashCommandResponse{
		ResponseType: "in_channel",
		Blocks: []Block{
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: fmt.Sprintf(":warning: Are you sure you want to cancel *%s*? This will delete the pod and stop the agent.", w.Name),
				},
			},
			{
				Type: "actions",
				Elements: []Element{
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "Yes, Cancel"},
						ActionID: "confirm_cancel",
						Value:    w.ID.String(),
						Style:    "danger",
					},
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "No, Keep Running"},
						ActionID: "dismiss_cancel",
						Value:    w.ID.String(),
					},
				},
			},
		},
	}
}

func handleRepos(queries *db.Queries) SlashCommandResponse {
	repos, _, err := queries.ListRepositories(context.Background(), 1, 20)
	if err != nil {
		return errorResponse("Failed to fetch repositories: " + err.Error())
	}

	if len(repos) == 0 {
		return SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "No repositories linked yet. Add repositories via the web UI.",
		}
	}

	blocks := []Block{
		{
			Type: "header",
			Text: &TextObject{Type: "plain_text", Text: fmt.Sprintf("Linked Repositories (%d)", len(repos))},
		},
		{Type: "divider"},
	}

	for _, r := range repos {
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s*\n`%s/%s` | Default branch: `%s`", r.Name, r.GitHubOwner, r.GitHubRepo, r.DefaultBranch),
			},
		})
	}

	return SlashCommandResponse{
		ResponseType: "ephemeral",
		Blocks:       blocks,
	}
}

func buildHelpResponse() SlashCommandResponse {
	return SlashCommandResponse{
		ResponseType: "ephemeral",
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{Type: "plain_text", Text: "smol-cluster Commands"},
			},
			{Type: "divider"},
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: "*Workstream Management:*\n" +
						"`/smol list [status]` - List workstreams (default: running)\n" +
						"`/smol status <id|name>` - Get detailed workstream status\n" +
						"`/smol start <repo> <name> [desc]` - Start a new workstream\n" +
						"`/smol complete <id>` - Complete workstream (push + PR)\n" +
						"`/smol cancel <id>` - Cancel a workstream",
				},
			},
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: "*Communication:*\n" +
						"`/smol message <id> <text>` - Send message to agent\n" +
						"`/smol msg <id> <text>` - Shorthand for message",
				},
			},
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: "*Info:*\n" +
						"`/smol repos` - List linked repositories\n" +
						"`/smol help` - Show this help message",
				},
			},
			{Type: "divider"},
			{
				Type: "context",
				Elements: []Element{
					{Type: "mrkdwn", Text: &TextObject{Type: "mrkdwn", Text: ":bulb: Tip: You can use the first 8 characters of a workstream ID or its name to identify workstreams."}},
				},
			},
		},
	}
}

// --- Interaction Handler ---

// InteractionPayload represents a Slack interactive message callback
type InteractionPayload struct {
	Type        string `json:"type"`
	TriggerID   string `json:"trigger_id"`
	ResponseURL string `json:"response_url"`
	User        struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
	Actions []InteractionAction `json:"actions"`
	Channel struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channel"`
}

type InteractionAction struct {
	ActionID string `json:"action_id"`
	Value    string `json:"value"`
	Type     string `json:"type"`
}

// HandleInteraction processes interactive message actions (button clicks)
func HandleInteraction(payload InteractionPayload, queries *db.Queries, k8s SlackK8sActions) SlashCommandResponse {
	if len(payload.Actions) == 0 {
		return errorResponse("No action received")
	}

	action := payload.Actions[0]

	switch action.ActionID {
	case "view_status":
		return handleStatus(queries, []string{action.Value})

	case "complete_workstream":
		return handleComplete(queries, []string{action.Value})

	case "confirm_complete":
		wsID, err := uuid.Parse(action.Value)
		if err != nil {
			return errorResponse("Invalid workstream ID")
		}
		w, err := queries.GetWorkstreamByID(context.Background(), wsID)
		if err != nil {
			return errorResponse("Workstream not found")
		}
		if w.Status != "running" {
			return errorResponse(fmt.Sprintf("Workstream is %s, not running", w.Status))
		}

		_ = queries.UpdateWorkstreamStatus(context.Background(), wsID, "completing")
		go func() {
			prURL, completeErr := k8s.CompleteWorkstream(context.Background(), w)
			if completeErr != nil {
				_ = queries.UpdateWorkstreamStatus(context.Background(), wsID, "failed")
				return
			}
			if prURL != "" {
				_ = queries.UpdateWorkstreamPR(context.Background(), wsID, prURL)
			}
			_ = k8s.DeleteAgentPod(context.Background(), w.PodName, w.ServiceName)
			_ = queries.UpdateWorkstreamStatus(context.Background(), wsID, "completed")
		}()

		return SlashCommandResponse{
			ResponseType:    "in_channel",
			Text:            fmt.Sprintf(":arrows_counterclockwise: Completing *%s*... Will push changes and create a PR.", w.Name),
			ReplaceOriginal: true,
		}

	case "cancel_workstream":
		return handleCancel(queries, []string{action.Value})

	case "confirm_cancel":
		wsID, err := uuid.Parse(action.Value)
		if err != nil {
			return errorResponse("Invalid workstream ID")
		}
		w, err := queries.GetWorkstreamByID(context.Background(), wsID)
		if err != nil {
			return errorResponse("Workstream not found")
		}

		if w.PodName != "" {
			go func() {
				if err := k8s.DeleteAgentPod(context.Background(), w.PodName, w.ServiceName); err != nil {
					log.Printf("slack: failed to delete pod: %v", err)
				}
			}()
		}
		_ = queries.UpdateWorkstreamStatus(context.Background(), wsID, "cancelled")

		return SlashCommandResponse{
			ResponseType:    "in_channel",
			Text:            fmt.Sprintf(":no_entry_sign: Workstream *%s* has been cancelled.", w.Name),
			ReplaceOriginal: true,
		}

	case "dismiss_cancel":
		return SlashCommandResponse{
			ResponseType:    "ephemeral",
			Text:            "Action dismissed. Workstream is still running.",
			ReplaceOriginal: true,
		}

	case "send_message_prompt":
		return SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("To send a message, use:\n`/smol msg %s <your message>`", action.Value[:8]),
		}

	default:
		return errorResponse("Unknown action: " + action.ActionID)
	}
}

// --- Helper Functions ---

func findWorkstream(queries *db.Queries, idOrName string) (models.Workstream, error) {
	// Try UUID parse first
	if id, err := uuid.Parse(idOrName); err == nil {
		ws, err := queries.GetWorkstreamByID(context.Background(), id)
		if err != nil {
			return models.Workstream{}, fmt.Errorf("workstream `%s` not found", idOrName)
		}
		return ws, nil
	}

	// Search by short ID prefix or name
	workstreams, _, err := queries.ListWorkstreams(context.Background(), nil, "", 1, 100)
	if err != nil {
		return models.Workstream{}, fmt.Errorf("failed to search workstreams")
	}

	for _, ws := range workstreams {
		if strings.HasPrefix(ws.ID.String(), idOrName) || strings.EqualFold(ws.Name, idOrName) {
			return ws, nil
		}
	}

	return models.Workstream{}, fmt.Errorf("workstream `%s` not found. Use `/smol list` to see active workstreams", idOrName)
}

func statusToEmoji(status string) string {
	switch status {
	case "pending":
		return ":hourglass:"
	case "provisioning":
		return ":gear:"
	case "running":
		return ":green_circle:"
	case "completing":
		return ":arrows_counterclockwise:"
	case "completed":
		return ":white_check_mark:"
	case "failed":
		return ":red_circle:"
	case "cancelled":
		return ":no_entry_sign:"
	default:
		return ":question:"
	}
}

func isActive(status string) bool {
	return status == "running" || status == "provisioning" || status == "pending"
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func errorResponse(msg string) SlashCommandResponse {
	return SlashCommandResponse{
		ResponseType: "ephemeral",
		Text:         ":warning: " + msg,
	}
}

func buildEventBlocks(event WorkstreamEvent) []Block {
	var emoji, title string

	switch event.Type {
	case "created":
		emoji = ":rocket:"
		title = "Workstream Created"
	case "running":
		emoji = ":green_circle:"
		title = "Agent is Running"
	case "completed":
		emoji = ":white_check_mark:"
		title = "Workstream Completed"
	case "failed":
		emoji = ":red_circle:"
		title = "Workstream Failed"
	case "cancelled":
		emoji = ":no_entry_sign:"
		title = "Workstream Cancelled"
	case "message":
		emoji = ":speech_balloon:"
		title = "Agent Message"
	default:
		emoji = ":information_source:"
		title = "Workstream Update"
	}

	blocks := []Block{
		{
			Type: "section",
			Text: &TextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("%s *%s* - *%s*", emoji, title, event.WorkstreamName),
			},
		},
	}

	fields := []TextObject{}
	if event.RepoName != "" {
		fields = append(fields, TextObject{Type: "mrkdwn", Text: fmt.Sprintf("*Repo:* %s", event.RepoName)})
	}
	if event.BranchName != "" {
		fields = append(fields, TextObject{Type: "mrkdwn", Text: fmt.Sprintf("*Branch:* `%s`", event.BranchName)})
	}
	if event.UserName != "" {
		fields = append(fields, TextObject{Type: "mrkdwn", Text: fmt.Sprintf("*By:* %s", event.UserName)})
	}
	if len(fields) > 0 {
		blocks = append(blocks, Block{Type: "section", Fields: fields})
	}

	if event.Details != "" {
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{Type: "mrkdwn", Text: event.Details},
		})
	}

	return blocks
}
