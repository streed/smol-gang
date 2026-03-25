package slack

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/db"
	"github.com/streed/smol-cluster/gateway/internal/ws"
)

type Bot interface {
	Start(ctx context.Context) error
	SendMessage(channel, text string) error
}

type SlackBot struct {
	config  *config.Config
	queries *db.Queries
	hub     *ws.Hub
}

func NewBot(cfg *config.Config, queries *db.Queries, hub *ws.Hub) Bot {
	return &SlackBot{
		config:  cfg,
		queries: queries,
		hub:     hub,
	}
}

func (b *SlackBot) Start(ctx context.Context) error {
	if b.config.SlackBotToken == "" {
		log.Println("slack: no bot token configured, skipping")
		return nil
	}

	log.Println("slack: bot started (integration ready)")
	// In production, this would use the Slack Events API or Socket Mode
	// to listen for slash commands and DMs:
	// /smol list - list active workstreams
	// /smol status <workstream> - get workstream status
	// /smol message <workstream> <text> - send message to agent
	// /smol start <repo> <name> - start a new workstream

	<-ctx.Done()
	return nil
}

func (b *SlackBot) SendMessage(channel, text string) error {
	if b.config.SlackBotToken == "" {
		return fmt.Errorf("slack not configured")
	}

	// In production, use Slack API to post message
	log.Printf("slack: would send to %s: %s", channel, text)
	return nil
}

func HandleSlackCommand(command, text string, queries *db.Queries) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Usage: /smol [list|status|message|start]"
	}

	switch parts[0] {
	case "list":
		workstreams, _, err := queries.ListWorkstreams(context.Background(), nil, "running", 1, 10)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		if len(workstreams) == 0 {
			return "No active workstreams"
		}
		var sb strings.Builder
		sb.WriteString("*Active Workstreams:*\n")
		for _, ws := range workstreams {
			sb.WriteString(fmt.Sprintf("- `%s` (%s) - %s\n", ws.Name, ws.ID.String()[:8], ws.Status))
		}
		return sb.String()

	case "status":
		return "Usage: /smol status <workstream-id>"

	case "help":
		return "*smol-cluster commands:*\n" +
			"- `/smol list` - List active workstreams\n" +
			"- `/smol status <id>` - Get workstream status\n" +
			"- `/smol message <id> <text>` - Send message to agent\n" +
			"- `/smol start <repo> <name>` - Start new workstream"

	default:
		return fmt.Sprintf("Unknown command: %s. Try `/smol help`", parts[0])
	}
}
