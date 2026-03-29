package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/streed/smol-gang/gateway/internal/models"
	"github.com/streed/smol-gang/gateway/internal/scheduler"
)

const decomposerSystemPrompt = `You are a task decomposition engine for a software development agent system.

Given a complex task description and optional repository context, break the task into independent, parallelizable subtasks suitable for implementation as separate Git branches and Pull Requests.

RULES:
1. Each task MUST be independently implementable as a single PR.
2. Each task MUST have a clear, specific scope — not vague or open-ended.
3. Tasks MUST declare dependencies explicitly. A task depends on another ONLY if it needs the code/artifacts from that task to exist first.
4. Minimize dependencies. Prefer independent tasks that can run in parallel.
5. Each task should touch a distinct set of files when possible. If two tasks must modify the same file, one MUST depend on the other.
6. Task descriptions must be specific enough for a coding agent to implement without ambiguity. Include file paths, function names, and expected behavior.
7. Include acceptance criteria for each task — concrete conditions that indicate the task is done.
8. The task graph MUST be a valid Directed Acyclic Graph (no circular dependencies).
9. Aim for 3-12 tasks. Fewer than 3 suggests the task doesn't need decomposition. More than 12 suggests over-decomposition.
10. Each task ID must be a short kebab-case identifier (e.g., "db-schema", "auth-middleware").

Respond with ONLY valid JSON matching the schema below. No markdown, no explanation.
JSON schema: {"plan_summary":"string","tasks":[{"id":"string","description":"string","depends_on":["string"],"file_scope":["string"],"acceptance_criteria":["string"],"model_tier":"heavy|light|auto"}]}`

type decomposerResponse struct {
	PlanSummary string            `json:"plan_summary"`
	Tasks       []models.TaskNode `json:"tasks"`
}

// Decompose breaks a complex task into a DAG of subtasks.
// It validates the result and retries up to 2 times if the DAG is invalid.
func (c *Client) Decompose(ctx context.Context, prompt string, repoContext string) ([]models.TaskNode, string, error) {
	userMsg := fmt.Sprintf("TASK:\n%s", prompt)
	if repoContext != "" {
		userMsg += fmt.Sprintf("\n\nREPOSITORY CONTEXT:\n%s", repoContext)
	}
	userMsg += "\n\nDecompose this task into independent, parallelizable subtasks. Each subtask will be implemented as a separate Pull Request by an autonomous coding agent.\n\nReturn ONLY valid JSON matching the provided schema."

	messages := []ChatMessage{
		{Role: "system", Content: decomposerSystemPrompt},
		{Role: "user", Content: userMsg},
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		response, err := c.Chat(ctx, messages, true)
		if err != nil {
			return nil, "", fmt.Errorf("decomposition LLM call: %w", err)
		}

		var result decomposerResponse
		if err := json.Unmarshal([]byte(response), &result); err != nil {
			lastErr = fmt.Errorf("parse decomposition (attempt %d): %w", attempt+1, err)
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: response},
				ChatMessage{Role: "user", Content: fmt.Sprintf("Your response was not valid JSON: %s\n\nPlease fix and respond with ONLY valid JSON.", err.Error())},
			)
			continue
		}

		if len(result.Tasks) == 0 {
			lastErr = fmt.Errorf("decomposition returned 0 tasks")
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: response},
				ChatMessage{Role: "user", Content: "You returned 0 tasks. Please decompose the task into at least 2 subtasks."},
			)
			continue
		}

		// Validate the DAG
		if err := scheduler.ValidateDAG(result.Tasks); err != nil {
			lastErr = fmt.Errorf("DAG validation (attempt %d): %w", attempt+1, err)
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: response},
				ChatMessage{Role: "user", Content: fmt.Sprintf("The task DAG is invalid: %s\n\nPlease fix the dependencies and respond with valid JSON.", err.Error())},
			)
			continue
		}

		// Ensure model_tier defaults
		for i := range result.Tasks {
			if result.Tasks[i].ModelTier == "" {
				result.Tasks[i].ModelTier = "auto"
			}
			if result.Tasks[i].DependsOn == nil {
				result.Tasks[i].DependsOn = []string{}
			}
			if result.Tasks[i].FileScope == nil {
				result.Tasks[i].FileScope = []string{}
			}
			if result.Tasks[i].AcceptanceCriteria == nil {
				result.Tasks[i].AcceptanceCriteria = []string{}
			}
		}

		return result.Tasks, result.PlanSummary, nil
	}

	return nil, "", fmt.Errorf("decomposition failed after 3 attempts: %w", lastErr)
}
