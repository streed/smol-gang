package coordinator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/auth"
	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/k8s"
	"github.com/streed/smol-gang/gateway/internal/models"
	"github.com/streed/smol-gang/gateway/internal/ws"
)

type Coordinator struct {
	queries *db.Queries
	k8s     *k8s.Client
	config  *config.Config
	hub     *ws.Hub
}

func New(queries *db.Queries, k8sClient *k8s.Client, cfg *config.Config, hub *ws.Hub) *Coordinator {
	return &Coordinator{
		queries: queries,
		k8s:     k8sClient,
		config:  cfg,
		hub:     hub,
	}
}

// Run starts the coordinator loop. Call this as a goroutine from main.
func (c *Coordinator) Run(ctx context.Context) {
	log.Println("coordinator: started")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("coordinator: shutting down")
			return
		case <-ticker.C:
			c.tick(ctx)
		}
	}
}

func (c *Coordinator) tick(ctx context.Context) {
	// Monitor workstream completions and update task statuses
	c.MonitorWorkstreams(ctx)

	// Find all active plans and process them
	plans, _, err := c.queries.ListPlans(ctx, nil, models.PlanStatusActive, 1, 100)
	if err != nil {
		log.Printf("coordinator: error listing active plans: %v", err)
		return
	}

	for _, plan := range plans {
		c.processPlan(ctx, plan)
	}
}

func (c *Coordinator) processPlan(ctx context.Context, plan models.Plan) {
	// Ensure plan has a root branch and PR
	if plan.RootBranch == "" {
		rootBranch := fmt.Sprintf("smol-gang/plan-%s", plan.ID.String()[:8])
		plan.RootBranch = rootBranch

		// Create the branch and open a PR on GitHub
		tasks, _ := c.queries.GetTasksByPlanID(ctx, plan.ID)
		prNumber := c.createRootBranchAndPR(ctx, plan, rootBranch, tasks)
		_ = c.queries.UpdatePlanRootPR(ctx, plan.ID, rootBranch, prNumber)
		log.Printf("coordinator: created root branch %s with PR #%d for plan %s", rootBranch, prNumber, plan.ID)
	}

	tasks, err := c.queries.GetTasksByPlanID(ctx, plan.ID)
	if err != nil {
		log.Printf("coordinator: error getting tasks for plan %s: %v", plan.ID, err)
		return
	}

	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}

	allDone := true
	anyFailed := false

	for i := range tasks {
		t := &tasks[i]

		switch t.Status {
		case models.TaskStatusReady:
			// Provision this task
			c.provisionTask(ctx, plan, t)
			allDone = false

		case models.TaskStatusPending:
			// Check if all dependencies are met
			if c.depsAreMet(t, taskMap) {
				log.Printf("coordinator: task %s in plan %s — deps met, marking ready", t.ID, plan.ID)
				_ = c.queries.UpdateTaskStatus(ctx, t.ID, plan.ID, models.TaskStatusReady)
			}
			allDone = false

		case models.TaskStatusClaimed, models.TaskStatusWorking, models.TaskStatusPROpen, models.TaskStatusInReview, models.TaskStatusChangesRequested:
			allDone = false

		case models.TaskStatusFailed:
			anyFailed = true
			// Cancel downstream tasks
			c.cancelDownstream(ctx, plan.ID, t.ID, tasks)

		case models.TaskStatusMerged:
			// Already done — check if this unlocked anything (handled by pending check above)

		case models.TaskStatusCancelled:
			// Skip
		}
	}

	if anyFailed {
		_ = c.queries.UpdatePlanStatus(ctx, plan.ID, models.PlanStatusHalted)
		log.Printf("coordinator: plan %s halted due to task failure", plan.ID)
		return
	}

	if allDone {
		_ = c.queries.UpdatePlanStatus(ctx, plan.ID, models.PlanStatusComplete)
		log.Printf("coordinator: plan %s complete", plan.ID)
	}
}

func (c *Coordinator) depsAreMet(task *models.Task, taskMap map[string]*models.Task) bool {
	for _, depID := range task.DependsOn {
		dep, ok := taskMap[depID]
		if !ok {
			return false
		}
		if dep.Status != models.TaskStatusMerged {
			return false
		}
	}
	return true
}

func (c *Coordinator) cancelDownstream(ctx context.Context, planID uuid.UUID, failedTaskID string, allTasks []models.Task) {
	// Find all tasks that depend (directly or transitively) on the failed task
	dependents := make(map[string]bool)
	changed := true
	for changed {
		changed = false
		for _, t := range allTasks {
			if dependents[t.ID] {
				continue
			}
			for _, dep := range t.DependsOn {
				if dep == failedTaskID || dependents[dep] {
					dependents[t.ID] = true
					changed = true
					break
				}
			}
		}
	}

	for id := range dependents {
		task := findTask(allTasks, id)
		if task != nil && task.Status != models.TaskStatusCancelled && task.Status != models.TaskStatusMerged && task.Status != models.TaskStatusFailed {
			_ = c.queries.UpdateTaskStatus(ctx, id, planID, models.TaskStatusCancelled)
			log.Printf("coordinator: cancelled downstream task %s in plan %s", id, planID)
		}
	}
}

func (c *Coordinator) provisionTask(ctx context.Context, plan models.Plan, task *models.Task) {
	if c.k8s == nil {
		log.Printf("coordinator: k8s not available, cannot provision task %s", task.ID)
		_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusFailed)
		return
	}

	// Mark as claimed by coordinator
	_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusClaimed)

	repo, err := c.queries.GetRepositoryByID(ctx, plan.RepositoryID)
	if err != nil {
		log.Printf("coordinator: repo not found for plan %s: %v", plan.ID, err)
		_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusFailed)
		return
	}

	// Generate branch name for this task
	branchName := fmt.Sprintf("smol/%s-%s", sanitizeBranchName(task.ID), plan.ID.String()[:8])
	if task.BranchName != "" {
		branchName = task.BranchName
	} else {
		// Store the generated branch name
		_ = c.queries.UpdateTaskBranch(ctx, task.ID, plan.ID, branchName)
	}

	// Build the agent prompt from task description + acceptance criteria
	prompt := task.Description
	if len(task.AcceptanceCriteria) > 0 {
		prompt += "\n\nAcceptance Criteria:\n"
		for _, ac := range task.AcceptanceCriteria {
			prompt += "- " + ac + "\n"
		}
	}
	if len(task.FileScope) > 0 {
		prompt += "\nFiles to focus on: " + fmt.Sprintf("%v", task.FileScope)
	}

	// Create a workstream for this task
	workstream, err := c.queries.CreateWorkstream(ctx, models.Workstream{
		Name:         fmt.Sprintf("[%s] %s", plan.ID.String()[:8], task.ID),
		Description:  prompt,
		RepositoryID: plan.RepositoryID,
		BranchName:   branchName,
		Status:       "pending",
		CreatedByID:  plan.CreatedByID,
	})
	if err != nil {
		log.Printf("coordinator: failed to create workstream for task %s: %v", task.ID, err)
		_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusFailed)
		return
	}

	// Link task to workstream
	_ = c.queries.LinkTaskWorkstream(ctx, task.ID, plan.ID, workstream.ID)
	_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusWorking)

	// Get user's GitHub token
	user, err := c.queries.GetUserByID(ctx, plan.CreatedByID)
	if err != nil {
		log.Printf("coordinator: failed to get user for plan %s: %v", plan.ID, err)
		return
	}

	gatewayToken, _ := auth.GenerateToken(plan.CreatedByID, user.Email, user.Role, c.config.JWTSecret, 7*24*time.Hour)

	// Provision K8s pod
	go func() {
		bgCtx := context.Background()
		_ = c.queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "provisioning")

		extraEnv := map[string]string{
			"GIT_TOKEN":     user.GitHubAccessToken,
			"GATEWAY_TOKEN": gatewayToken,
			"BASE_BRANCH":   plan.RootBranch,
		}

		podName, serviceName, provErr := c.k8s.CreateAgentPod(bgCtx, workstream, repo, extraEnv)
		if provErr != nil {
			_ = c.queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "failed")
			_ = c.queries.UpdateTaskStatus(bgCtx, task.ID, plan.ID, models.TaskStatusFailed)
			log.Printf("coordinator: provisioning failed for task %s: %v", task.ID, provErr)
			return
		}

		_ = c.queries.UpdateWorkstreamPod(bgCtx, workstream.ID, podName, serviceName)
		_ = c.queries.UpdateWorkstreamStatus(bgCtx, workstream.ID, "running")
		log.Printf("coordinator: task %s provisioned as pod %s", task.ID, podName)
	}()
}

// MonitorWorkstreams checks workstream status changes and updates corresponding tasks.
func (c *Coordinator) MonitorWorkstreams(ctx context.Context) {
	plans, _, err := c.queries.ListPlans(ctx, nil, models.PlanStatusActive, 1, 100)
	if err != nil {
		return
	}

	for _, plan := range plans {
		tasks, err := c.queries.GetTasksByPlanID(ctx, plan.ID)
		if err != nil {
			continue
		}

		for _, task := range tasks {
			if task.WorkstreamID == nil {
				continue
			}
			if task.Status != models.TaskStatusWorking && task.Status != models.TaskStatusClaimed {
				continue
			}

			workstream, err := c.queries.GetWorkstreamByID(ctx, *task.WorkstreamID)
			if err != nil {
				continue
			}

			switch workstream.Status {
			case "completed":
				_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusMerged)
				if workstream.PullRequestURL != "" {
					_ = c.queries.UpdateTaskPR(ctx, task.ID, plan.ID, 0, workstream.PullRequestURL)
				}
				log.Printf("coordinator: task %s completed (workstream %s)", task.ID, workstream.ID)

			case "failed":
				_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusFailed)
				log.Printf("coordinator: task %s failed (workstream %s)", task.ID, workstream.ID)

			case "cancelled":
				_ = c.queries.UpdateTaskStatus(ctx, task.ID, plan.ID, models.TaskStatusCancelled)
			}
		}
	}
}

func findTask(tasks []models.Task, id string) *models.Task {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

// createRootBranchAndPR creates the plan's root branch on GitHub and opens a PR.
func (c *Coordinator) createRootBranchAndPR(ctx context.Context, plan models.Plan, rootBranch string, tasks []models.Task) int {
	repo, err := c.queries.GetRepositoryByID(ctx, plan.RepositoryID)
	if err != nil {
		log.Printf("coordinator: repo not found for PR creation: %v", err)
		return 0
	}

	user, err := c.queries.GetUserByID(ctx, plan.CreatedByID)
	if err != nil || user.GitHubAccessToken == "" {
		log.Printf("coordinator: no GitHub token for PR creation")
		return 0
	}

	token := user.GitHubAccessToken
	owner := repo.GitHubOwner
	repoName := repo.GitHubRepo
	apiBase := "https://api.github.com"

	// Step 1: Get the SHA of the base branch (main)
	mainSHA := c.getGitHubRef(apiBase, owner, repoName, plan.BaseBranch, token)
	if mainSHA == "" {
		log.Printf("coordinator: could not get SHA for %s", plan.BaseBranch)
		return 0
	}

	// Step 2: Create the root branch
	createRefBody, _ := json.Marshal(map[string]string{
		"ref": "refs/heads/" + rootBranch,
		"sha": mainSHA,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/repos/%s/%s/git/refs", apiBase, owner, repoName),
		bytes.NewReader(createRefBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("coordinator: failed to create branch: %v", err)
		return 0
	}
	resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 422 { // 422 = already exists
		log.Printf("coordinator: create branch returned %d", resp.StatusCode)
	}

	// Step 3: Create an initial commit on the root branch (plan file)
	// This is needed because GitHub won't allow a PR with no commits difference
	planFileContent := fmt.Sprintf("# smol-gang Plan\n\n## Prompt\n\n%s\n\n## Tasks\n\n", plan.Prompt)
	for _, t := range tasks {
		planFileContent += fmt.Sprintf("- **%s** (wave %d): %s\n", t.ID, t.Wave, t.Description)
	}
	planFileContent += fmt.Sprintf("\n\n*Plan ID: %s*\n", plan.ID)

	encodedContent := base64Encode(planFileContent)
	filePayload, _ := json.Marshal(map[string]string{
		"message": fmt.Sprintf("plan: %s", plan.Prompt[:min(len(plan.Prompt), 60)]),
		"content": encodedContent,
		"branch":  rootBranch,
	})
	fileReq, _ := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/repos/%s/%s/contents/.smol-gang-plan.md", apiBase, owner, repoName),
		bytes.NewReader(filePayload))
	fileReq.Header.Set("Authorization", "Bearer "+token)
	fileReq.Header.Set("Content-Type", "application/json")
	fileResp, err := http.DefaultClient.Do(fileReq)
	if err != nil {
		log.Printf("coordinator: failed to create plan file: %v", err)
	} else {
		fileResp.Body.Close()
		log.Printf("coordinator: created plan file on %s (status %d)", rootBranch, fileResp.StatusCode)
	}

	// Step 5: Build PR body with plan details
	var prBody strings.Builder
	prBody.WriteString("## Plan\n\n")
	prBody.WriteString(plan.Prompt)
	prBody.WriteString("\n\n---\n\n")

	if len(tasks) > 0 {
		prBody.WriteString("## Tasks\n\n")
		for _, t := range tasks {
			status := "⬜"
			if t.Status == models.TaskStatusMerged {
				status = "✅"
			}
			prBody.WriteString(fmt.Sprintf("- %s **%s** (wave %d): %s\n", status, t.ID, t.Wave, t.Description))
			if len(t.AcceptanceCriteria) > 0 {
				for _, ac := range t.AcceptanceCriteria {
					prBody.WriteString(fmt.Sprintf("  - [ ] %s\n", ac))
				}
			}
		}
		prBody.WriteString("\n---\n\n")
	}

	prBody.WriteString("*Automated by smol-gang*\n")

	// Step 6: Create the PR
	prPayload, _ := json.Marshal(map[string]interface{}{
		"title": fmt.Sprintf("[smol-gang] %s", plan.Prompt[:min(len(plan.Prompt), 80)]),
		"body":  prBody.String(),
		"head":  rootBranch,
		"base":  plan.BaseBranch,
		"draft": true,
	})
	prReq, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/repos/%s/%s/pulls", apiBase, owner, repoName),
		bytes.NewReader(prPayload))
	prReq.Header.Set("Authorization", "Bearer "+token)
	prReq.Header.Set("Content-Type", "application/json")
	prResp, err := http.DefaultClient.Do(prReq)
	if err != nil {
		log.Printf("coordinator: failed to create PR: %v", err)
		return 0
	}
	defer prResp.Body.Close()

	body, _ := io.ReadAll(prResp.Body)
	var prResult struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	json.Unmarshal(body, &prResult)

	if prResult.Number > 0 {
		log.Printf("coordinator: created PR #%d for plan %s: %s", prResult.Number, plan.ID, prResult.HTMLURL)
	} else {
		log.Printf("coordinator: PR creation response (%d): %s", prResp.StatusCode, string(body[:min(len(body), 200)]))
	}

	return prResult.Number
}

func (c *Coordinator) getGitHubRef(apiBase, owner, repo, branch, token string) string {
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", apiBase, owner, repo, branch), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	json.Unmarshal(body, &result)
	return result.Object.SHA
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func sanitizeBranchName(name string) string {
	result := make([]byte, 0, len(name))
	for _, b := range []byte(name) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' {
			result = append(result, b)
		} else if b == ' ' {
			result = append(result, '-')
		}
	}
	s := string(result)
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}
