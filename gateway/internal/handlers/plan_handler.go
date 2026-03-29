package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/llm"
	"github.com/streed/smol-gang/gateway/internal/middleware"
	"github.com/streed/smol-gang/gateway/internal/models"
	"github.com/streed/smol-gang/gateway/internal/scheduler"
	"k8s.io/client-go/kubernetes"
)

type PlanHandler struct {
	Queries *db.Queries
	K8s     kubernetes.Interface
	Config  *config.Config
	LLM     *llm.Client
}

// Create creates a new plan. For now, the frontend provides tasks directly.
// Later, the Complexity Assessor + Decomposer will generate them from a prompt.
func (h *PlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Prompt == "" || req.RepositoryID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "prompt and repository_id are required"})
		return
	}

	if req.BaseBranch == "" {
		req.BaseBranch = "main"
	}

	claims := middleware.GetUserFromContext(r.Context())

	plan, err := h.Queries.CreatePlan(r.Context(), models.Plan{
		Prompt:       req.Prompt,
		Status:       models.PlanStatusPendingApproval,
		RepositoryID: req.RepositoryID,
		BaseBranch:   req.BaseBranch,
		Complexity:   "assessing",
		CreatedByID:  claims.UserID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create plan", Details: err.Error()})
		return
	}

	// Run assessment + decomposition in background
	if h.LLM != nil {
		go h.assessAndDecompose(plan, req.AutoApprove, claims.UserID)
	}

	writeJSON(w, http.StatusCreated, plan)
}

// addThinking appends a system message to the plan's conversations
func (h *PlanHandler) addThinking(ctx context.Context, planID uuid.UUID, msg string) {
	log.Printf("plan %s: %s", planID, msg)
	// Load existing conversations, append, save
	plan, err := h.Queries.GetPlanByID(ctx, planID)
	if err != nil {
		return
	}
	var convos []map[string]interface{}
	if plan.Conversations != nil {
		json.Unmarshal(plan.Conversations, &convos)
	}
	convos = append(convos, map[string]interface{}{
		"id":        uuid.New().String(),
		"role":      "system",
		"content":   msg,
		"timestamp": time.Now(),
	})
	data, _ := json.Marshal(convos)
	h.Queries.UpdatePlanConversations(ctx, planID, data)
}

func (h *PlanHandler) assessAndDecompose(plan models.Plan, autoApprove bool, userID uuid.UUID) {
	ctx := context.Background()

	// Step 0: Quick repo context for assessment (basic file tree)
	h.addThinking(ctx, plan.ID, "🔍 Fetching repository context...")

	repoContext := ""
	repo, err := h.Queries.GetRepositoryByID(ctx, plan.RepositoryID)
	user, _ := h.Queries.GetUserByID(ctx, userID)
	gitToken := user.GitHubAccessToken

	if err == nil && h.K8s != nil {
		repoCtx, err := llm.ExploreRepoBasic(ctx, h.K8s, repo.GitURL, gitToken, repo.DefaultBranch, h.Config.K8sNamespace)
		if err != nil {
			h.addThinking(ctx, plan.ID, "⚠️ Could not scan repo: "+err.Error())
		} else {
			repoContext = repoCtx
			h.addThinking(ctx, plan.ID, "✅ Repository structure loaded")
		}
	}

	// Step 1: Assess complexity
	h.addThinking(ctx, plan.ID, "🧠 Assessing task complexity...")

	verdict, err := h.LLM.Assess(ctx, plan.Prompt, repoContext)
	if err != nil {
		h.addThinking(ctx, plan.ID, "⚠️ Assessment failed: "+err.Error()+". Falling back to single task.")
		h.Queries.UpdatePlanStatus(ctx, plan.ID, models.PlanStatusPendingApproval)
		h.createSingleTask(ctx, plan)
		return
	}

	// Update plan with assessment
	h.Queries.UpdatePlanComplexity(ctx, plan.ID, verdict.Complexity, verdict.Reasoning)
	h.addThinking(ctx, plan.ID, fmt.Sprintf("📊 Complexity: **%s** (confidence: %.0f%%)\n\n%s", verdict.Complexity, verdict.Confidence*100, verdict.Reasoning))

	if verdict.Complexity == "simple" {
		h.addThinking(ctx, plan.ID, "✅ Simple task — creating single workstream")
		h.createSingleTask(ctx, plan)
		if autoApprove {
			h.autoApprove(ctx, plan.ID)
		}
		return
	}

	// Step 2: Deep exploration via smol-agent (complex tasks only)
	if h.K8s != nil && repo.GitURL != "" {
		h.addThinking(ctx, plan.ID, "🤖 Launching smol-agent to explore the repository in depth...")
		agentAnalysis, err := llm.ExploreWithAgent(ctx, h.K8s, h.Config, repo.GitURL, gitToken, repo.DefaultBranch, h.Config.K8sNamespace)
		if err != nil {
			h.addThinking(ctx, plan.ID, "⚠️ Agent exploration failed: "+err.Error()+". Using basic context.")
		} else {
			// Trim very long output
			if len(agentAnalysis) > 8000 {
				agentAnalysis = agentAnalysis[:8000] + "\n...(truncated)"
			}
			repoContext = repoContext + "\n\n## Agent Analysis\n" + agentAnalysis
			h.addThinking(ctx, plan.ID, "✅ Agent exploration complete — deep analysis available for decomposition")
		}
	}

	// Step 3: Decompose into DAG
	h.addThinking(ctx, plan.ID, fmt.Sprintf("🔨 Decomposing into ~%d subtasks...", verdict.EstTasks))

	tasks, summary, err := h.LLM.Decompose(ctx, plan.Prompt, repoContext)
	if err != nil {
		h.addThinking(ctx, plan.ID, "⚠️ Decomposition failed: "+err.Error()+". Falling back to single task.")
		h.createSingleTask(ctx, plan)
		return
	}

	h.addThinking(ctx, plan.ID, fmt.Sprintf("📋 Plan: %s", summary))

	// Build execution plan and persist
	execPlan, err := scheduler.BuildExecutionPlan(tasks)
	if err != nil {
		log.Printf("plan %s: scheduling failed: %v", plan.ID, err)
		h.createSingleTask(ctx, plan)
		return
	}

	planJSON, _ := json.Marshal(execPlan)
	h.Queries.UpdatePlanJSON(ctx, plan.ID, planJSON)

	waveMap := scheduler.AssignWaveNumbers(execPlan.Waves)

	// Create tasks in DB and report each one
	var taskSummary string
	for _, tn := range tasks {
		h.Queries.CreateTask(ctx, models.Task{
			ID:                 tn.ID,
			PlanID:             plan.ID,
			Description:        tn.Description,
			DependsOn:          tn.DependsOn,
			FileScope:          tn.FileScope,
			AcceptanceCriteria: tn.AcceptanceCriteria,
			ModelTier:          tn.ModelTier,
			Status:             models.TaskStatusPending,
			Wave:               waveMap[tn.ID],
		})
		deps := ""
		if len(tn.DependsOn) > 0 {
			deps = " ← depends on: " + strings.Join(tn.DependsOn, ", ")
		}
		taskSummary += fmt.Sprintf("- **%s** (wave %d): %s%s\n", tn.ID, waveMap[tn.ID], tn.Description[:min(len(tn.Description), 100)], deps)
	}

	h.addThinking(ctx, plan.ID, fmt.Sprintf("✅ Created %d tasks in %d waves:\n\n%s\nCritical path: %s\n\nReview the graph above. Refine via chat or approve to start execution.",
		len(tasks), len(execPlan.Waves), taskSummary, strings.Join(execPlan.CriticalPath, " → ")))

	if autoApprove {
		h.autoApprove(ctx, plan.ID)
	}
}

func (h *PlanHandler) createSingleTask(ctx context.Context, plan models.Plan) {
	h.Queries.CreateTask(ctx, models.Task{
		ID:                 "main",
		PlanID:             plan.ID,
		Description:        plan.Prompt,
		DependsOn:          []string{},
		FileScope:          []string{},
		AcceptanceCriteria: []string{"Task completed as described in the prompt"},
		ModelTier:          "auto",
		Status:             models.TaskStatusPending,
		Wave:               0,
	})

	execPlan := &models.ExecutionPlan{
		Tasks: []models.TaskNode{{
			ID:                 "main",
			Description:        plan.Prompt,
			DependsOn:          []string{},
			FileScope:          []string{},
			AcceptanceCriteria: []string{"Task completed as described in the prompt"},
			ModelTier:          "auto",
		}},
		Waves:        [][]string{{"main"}},
		CriticalPath: []string{"main"},
	}
	planJSON, _ := json.Marshal(execPlan)
	h.Queries.UpdatePlanJSON(ctx, plan.ID, planJSON)
}

func (h *PlanHandler) autoApprove(ctx context.Context, planID uuid.UUID) {
	tasks, _ := h.Queries.GetTasksByPlanID(ctx, planID)
	taskNodes := tasksToNodes(tasks)
	waves, _ := scheduler.ComputeWaves(taskNodes)
	if len(waves) > 0 {
		for _, taskID := range waves[0] {
			h.Queries.UpdateTaskStatus(ctx, taskID, planID, models.TaskStatusReady)
		}
	}
	h.Queries.UpdatePlanStatus(ctx, planID, models.PlanStatusActive)
	log.Printf("plan %s: auto-approved", planID)
}

// Get returns a plan with its tasks.
func (h *PlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	plan, err := h.Queries.GetPlanByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}

	tasks, err := h.Queries.GetTasksByPlanID(r.Context(), id)
	if err == nil {
		plan.Tasks = tasks
	}

	writeJSON(w, http.StatusOK, plan)
}

// List returns paginated plans.
func (h *PlanHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)

	var repoID *uuid.UUID
	if rid := r.URL.Query().Get("repository_id"); rid != "" {
		id, err := uuid.Parse(rid)
		if err == nil {
			repoID = &id
		}
	}
	status := r.URL.Query().Get("status")

	plans, total, err := h.Queries.ListPlans(r.Context(), repoID, status, page, perPage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list plans"})
		return
	}

	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Items:   plans,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// AddTask adds a task to a pending plan.
func (h *PlanHandler) AddTask(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusPendingApproval {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan is not editable (must be pending_approval)"})
		return
	}

	var req models.AddTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.ID == "" || req.Description == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "id and description are required"})
		return
	}

	if req.ModelTier == "" {
		req.ModelTier = "auto"
	}

	// Validate the new task doesn't break the DAG
	existingTasks, _ := h.Queries.GetTasksByPlanID(r.Context(), planID)
	taskNodes := tasksToNodes(existingTasks)
	taskNodes = append(taskNodes, models.TaskNode{
		ID:                 req.ID,
		Description:        req.Description,
		DependsOn:          req.DependsOn,
		FileScope:          req.FileScope,
		AcceptanceCriteria: req.AcceptanceCriteria,
		ModelTier:          req.ModelTier,
	})

	execPlan, err := scheduler.BuildExecutionPlan(taskNodes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid DAG after adding task", Details: err.Error()})
		return
	}

	waveMap := scheduler.AssignWaveNumbers(execPlan.Waves)

	task, err := h.Queries.CreateTask(r.Context(), models.Task{
		ID:                 req.ID,
		PlanID:             planID,
		Description:        req.Description,
		DependsOn:          req.DependsOn,
		FileScope:          req.FileScope,
		AcceptanceCriteria: req.AcceptanceCriteria,
		ModelTier:          req.ModelTier,
		Status:             models.TaskStatusPending,
		Wave:               waveMap[req.ID],
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{Error: "failed to create task", Details: err.Error()})
		return
	}

	// Update plan JSON with new execution plan
	planJSON, _ := json.Marshal(execPlan)
	h.Queries.UpdatePlanJSON(r.Context(), planID, planJSON)

	writeJSON(w, http.StatusCreated, task)
}

// UpdateTask modifies a task in a pending plan.
func (h *PlanHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}
	taskID := chi.URLParam(r, "taskID")

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusPendingApproval {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan is not editable"})
		return
	}

	var req models.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Description != nil {
		h.Queries.UpdateTaskDescription(r.Context(), taskID, planID, *req.Description)
	}
	if req.DependsOn != nil {
		h.Queries.UpdateTaskDeps(r.Context(), taskID, planID, req.DependsOn)
	}
	if req.AcceptanceCriteria != nil {
		h.Queries.UpdateTaskCriteria(r.Context(), taskID, planID, req.AcceptanceCriteria)
	}
	if len(req.AddCriteria) > 0 {
		task, err := h.Queries.GetTask(r.Context(), taskID, planID)
		if err == nil {
			combined := append(task.AcceptanceCriteria, req.AddCriteria...)
			h.Queries.UpdateTaskCriteria(r.Context(), taskID, planID, combined)
		}
	}

	// Re-validate DAG
	allTasks, _ := h.Queries.GetTasksByPlanID(r.Context(), planID)
	taskNodes := tasksToNodes(allTasks)
	execPlan, err := scheduler.BuildExecutionPlan(taskNodes)
	if err != nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{Error: "DAG invalid after edit", Details: err.Error()})
		return
	}

	planJSON, _ := json.Marshal(execPlan)
	h.Queries.UpdatePlanJSON(r.Context(), planID, planJSON)

	task, _ := h.Queries.GetTask(r.Context(), taskID, planID)
	writeJSON(w, http.StatusOK, task)
}

// RemoveTask removes a task from a pending plan.
func (h *PlanHandler) RemoveTask(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}
	taskID := chi.URLParam(r, "taskID")

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusPendingApproval {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan is not editable"})
		return
	}

	if err := h.Queries.DeleteTask(r.Context(), taskID, planID); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete task"})
		return
	}

	// Re-validate remaining DAG (remove deps on deleted task)
	allTasks, _ := h.Queries.GetTasksByPlanID(r.Context(), planID)
	for _, t := range allTasks {
		newDeps := []string{}
		for _, dep := range t.DependsOn {
			if dep != taskID {
				newDeps = append(newDeps, dep)
			}
		}
		if len(newDeps) != len(t.DependsOn) {
			h.Queries.UpdateTaskDeps(r.Context(), t.ID, planID, newDeps)
		}
	}

	// Rebuild execution plan
	allTasks, _ = h.Queries.GetTasksByPlanID(r.Context(), planID)
	if len(allTasks) > 0 {
		taskNodes := tasksToNodes(allTasks)
		execPlan, _ := scheduler.BuildExecutionPlan(taskNodes)
		if execPlan != nil {
			planJSON, _ := json.Marshal(execPlan)
			h.Queries.UpdatePlanJSON(r.Context(), planID, planJSON)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// Approve transitions a plan from pending_approval to active.
func (h *PlanHandler) Approve(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusPendingApproval {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan is not pending approval"})
		return
	}

	// Final validation
	allTasks, _ := h.Queries.GetTasksByPlanID(r.Context(), planID)
	if len(allTasks) == 0 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan has no tasks"})
		return
	}

	taskNodes := tasksToNodes(allTasks)
	if err := scheduler.ValidateDAG(taskNodes); err != nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{Error: "DAG validation failed", Details: err.Error()})
		return
	}

	// Mark all tasks with no unmet dependencies as ready (not just wave-0)
	for _, t := range allTasks {
		if len(t.DependsOn) == 0 {
			h.Queries.UpdateTaskStatus(r.Context(), t.ID, planID, models.TaskStatusReady)
		}
	}

	if err := h.Queries.UpdatePlanStatus(r.Context(), planID, models.PlanStatusActive); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to activate plan"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "plan_id": planID.String()})
}

// Reject discards a plan.
func (h *PlanHandler) Reject(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusPendingApproval {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan is not pending approval"})
		return
	}

	if err := h.Queries.UpdatePlanStatus(r.Context(), planID, models.PlanStatusRejected); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to reject plan"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Complete marks an active plan as complete.
func (h *PlanHandler) Complete(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusActive {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan is not active"})
		return
	}

	if err := h.Queries.UpdatePlanStatus(r.Context(), planID, models.PlanStatusComplete); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to complete plan"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "complete"})
}

// Restart resets a halted/failed plan back to pending_approval so it can be re-approved.
// Cleans up task branches and PRs on GitHub.
func (h *PlanHandler) Restart(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	plan, err := h.Queries.GetPlanByID(r.Context(), planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "plan not found"})
		return
	}
	if plan.Status != models.PlanStatusHalted && plan.Status != models.PlanStatusComplete && plan.Status != models.PlanStatusRejected {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "plan must be halted, complete, or rejected to restart"})
		return
	}

	repo, _ := h.Queries.GetRepositoryByID(r.Context(), plan.RepositoryID)
	claims := middleware.GetUserFromContext(r.Context())
	user, _ := h.Queries.GetUserByID(r.Context(), claims.UserID)
	token := user.GitHubAccessToken

	tasks, _ := h.Queries.GetTasksByPlanID(r.Context(), planID)

	// Close PRs and delete task branches on GitHub
	if token != "" && repo.GitHubOwner != "" {
		for _, t := range tasks {
			// Close PR if open
			if t.PRNumber > 0 {
				h.githubClosePR(repo.GitHubOwner, repo.GitHubRepo, t.PRNumber, token)
			}
			// Delete task branch
			if t.BranchName != "" {
				h.githubDeleteBranch(repo.GitHubOwner, repo.GitHubRepo, t.BranchName, token)
			}
		}
		// Close root PR if open
		if plan.RootPR > 0 {
			h.githubClosePR(repo.GitHubOwner, repo.GitHubRepo, plan.RootPR, token)
		}
		// Delete root branch
		if plan.RootBranch != "" {
			h.githubDeleteBranch(repo.GitHubOwner, repo.GitHubRepo, plan.RootBranch, token)
		}
	}

	// Reset failed/cancelled tasks to pending, clear branch names and PR info
	for _, t := range tasks {
		if t.Status == models.TaskStatusFailed || t.Status == models.TaskStatusCancelled || t.Status == models.TaskStatusMerged {
			h.Queries.UpdateTaskStatus(r.Context(), t.ID, planID, models.TaskStatusPending)
		}
		h.Queries.UpdateTaskBranch(r.Context(), t.ID, planID, "")
		h.Queries.UpdateTaskPR(r.Context(), t.ID, planID, 0, "")
	}

	// Clear root branch so coordinator creates a fresh one
	h.Queries.UpdatePlanRootPR(r.Context(), planID, "", 0)
	h.Queries.UpdatePlanStatus(r.Context(), planID, models.PlanStatusPendingApproval)

	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (h *PlanHandler) githubClosePR(owner, repo string, prNumber int, token string) {
	body, _ := json.Marshal(map[string]string{"state": "closed"})
	req, _ := http.NewRequest("PATCH",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber),
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("restart: failed to close PR #%d: %v", prNumber, err)
		return
	}
	resp.Body.Close()
	log.Printf("restart: closed PR #%d (status %d)", prNumber, resp.StatusCode)
}

func (h *PlanHandler) githubDeleteBranch(owner, repo, branch, token string) {
	req, _ := http.NewRequest("DELETE",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", owner, repo, branch), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("restart: failed to delete branch %s: %v", branch, err)
		return
	}
	resp.Body.Close()
	log.Printf("restart: deleted branch %s (status %d)", branch, resp.StatusCode)
}

// SaveConversations persists the chat messages for a plan.
func (h *PlanHandler) SaveConversations(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid body"})
		return
	}

	if err := h.Queries.UpdatePlanConversations(r.Context(), planID, body); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save conversations"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete removes a plan entirely.
func (h *PlanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid plan ID"})
		return
	}

	if err := h.Queries.DeletePlan(r.Context(), planID); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete plan"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// helper: convert Task models to TaskNode for scheduler
func tasksToNodes(tasks []models.Task) []models.TaskNode {
	nodes := make([]models.TaskNode, len(tasks))
	for i, t := range tasks {
		nodes[i] = models.TaskNode{
			ID:                 t.ID,
			Description:        t.Description,
			DependsOn:          t.DependsOn,
			FileScope:          t.FileScope,
			AcceptanceCriteria: t.AcceptanceCriteria,
			ModelTier:          t.ModelTier,
		}
	}
	return nodes
}
