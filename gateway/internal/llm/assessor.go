package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/streed/smol-gang/gateway/internal/models"
)

const assessorSystemPrompt = `You are a complexity assessor for a software development agent system.

Given a task description and optional repository context, classify whether this task should be executed as a single Pull Request or decomposed into multiple parallel subtasks.

CLASSIFICATION RULES:

A task is SIMPLE if ALL of these are true:
- It touches 1-3 files
- It involves a single logical concern (one cohesive change)
- There are no internal ordering dependencies
- A single developer could complete it in one sitting without context-switching

A task is COMPLEX if ANY of these are true:
- It touches 4+ files across multiple modules, layers, or services
- It involves multiple distinct concerns that could be worked on independently
- It has internal ordering dependencies (X must exist before Y can be built)
- A human team would naturally split it into multiple PRs

EDGE CASES — default to SIMPLE:
- If the task is ambiguous but could reasonably be done in one PR, classify as SIMPLE.

Respond with ONLY valid JSON. No markdown, no explanation.
JSON schema: {"complexity":"simple|complex","reasoning":"string","confidence":0.0-1.0,"est_tasks":int,"suggestion":"string"}`

// Assess determines whether a task is simple or complex.
func (c *Client) Assess(ctx context.Context, prompt string, repoContext string) (*models.ComplexityVerdict, error) {
	userMsg := fmt.Sprintf("TASK:\n%s", prompt)
	if repoContext != "" {
		userMsg += fmt.Sprintf("\n\nREPOSITORY CONTEXT:\n%s", repoContext)
	}

	response, err := c.Chat(ctx, []ChatMessage{
		{Role: "system", Content: assessorSystemPrompt},
		{Role: "user", Content: userMsg},
	}, true)
	if err != nil {
		return nil, fmt.Errorf("complexity assessment: %w", err)
	}

	var verdict models.ComplexityVerdict
	if err := json.Unmarshal([]byte(response), &verdict); err != nil {
		return nil, fmt.Errorf("parse verdict: %w (response: %s)", err, response)
	}

	// Default to complex if confidence is low
	if verdict.Confidence < 0.6 {
		verdict.Complexity = "complex"
	}

	if verdict.EstTasks < 1 {
		verdict.EstTasks = 1
	}

	return &verdict, nil
}
