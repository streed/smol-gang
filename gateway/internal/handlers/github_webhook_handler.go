package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/k8s"
	"github.com/streed/smol-gang/gateway/internal/models"
)

type GitHubWebhookHandler struct {
	Config  *config.Config
	Queries *db.Queries
	K8s     *k8s.Client
}

func (h *GitHubWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature if secret is configured
	if h.Config.GitHubWebhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(body, sig, h.Config.GitHubWebhookSecret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	log.Printf("github webhook: event=%s delivery=%s", event, r.Header.Get("X-GitHub-Delivery"))

	switch event {
	case "ping":
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return

	case "pull_request":
		h.handlePullRequest(w, r, body)

	case "pull_request_review":
		h.handlePullRequestReview(w, r, body)

	case "pull_request_review_comment":
		h.handlePRComment(w, r, body)

	case "issue_comment":
		h.handleIssueComment(w, r, body)

	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "event": event})
	}
}

type prEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number    int    `json:"number"`
		State     string `json:"state"`
		Merged    bool   `json:"merged"`
		Title     string `json:"title"`
		HTMLURL   string `json:"html_url"`
		Head      struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
}

type prReviewEvent struct {
	Action string `json:"action"`
	Review struct {
		State string `json:"state"`
		Body  string `json:"body"`
	} `json:"review"`
	PullRequest struct {
		Number int    `json:"number"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
}

func (h *GitHubWebhookHandler) handlePullRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	var evt prEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	branch := evt.PullRequest.Head.Ref
	log.Printf("github webhook: PR #%d action=%s branch=%s merged=%v",
		evt.PullRequest.Number, evt.Action, branch, evt.PullRequest.Merged)

	// Check if this is a plan's root PR
	if evt.Action == "closed" {
		if evt.PullRequest.Merged {
			// Root PR merged into main — plan complete, clean up all branches
			if h.completePlanByRootPR(r, evt.PullRequest.Number, branch) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "plan_completed"})
				return
			}
		} else {
			// Root PR closed without merge — cancel plan
			if h.cancelPlanByRootPR(r, evt.PullRequest.Number, branch) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "plan_cancelled"})
				return
			}
		}
	}

	// Find tasks matching this branch
	if evt.Action == "closed" && evt.PullRequest.Merged {
		// Task PR merged — mark as merged, promote downstream tasks
		h.updateTaskByBranch(r, branch, models.TaskStatusMerged, evt.PullRequest.HTMLURL, evt.PullRequest.Number)
		h.promoteDownstreamTasks(r, branch)

		// Also update workstream if linked
		h.updateWorkstreamByBranch(r, branch, "completed", evt.PullRequest.HTMLURL)
	} else if evt.Action == "closed" && !evt.PullRequest.Merged {
		// Task PR closed without merge — task failed
		h.updateTaskByBranch(r, branch, models.TaskStatusFailed, "", evt.PullRequest.Number)
		h.updateWorkstreamByBranch(r, branch, "failed", "")
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHubWebhookHandler) handlePullRequestReview(w http.ResponseWriter, r *http.Request, body []byte) {
	var evt prReviewEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	branch := evt.PullRequest.Head.Ref
	log.Printf("github webhook: review action=%s state=%s branch=%s", evt.Action, evt.Review.State, branch)

	if evt.Action == "submitted" && evt.Review.State == "changes_requested" {
		h.updateTaskByBranch(r, branch, models.TaskStatusChangesRequested, "", evt.PullRequest.Number)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHubWebhookHandler) updateTaskByBranch(r *http.Request, branch, status, prURL string, prNumber int) {
	// Search all active plans for a task with this branch
	plans, _, err := h.Queries.ListPlans(r.Context(), nil, models.PlanStatusActive, 1, 100)
	if err != nil {
		return
	}

	for _, plan := range plans {
		tasks, err := h.Queries.GetTasksByPlanID(r.Context(), plan.ID)
		if err != nil {
			continue
		}
		for _, task := range tasks {
			if task.BranchName == branch {
				h.Queries.UpdateTaskStatus(r.Context(), task.ID, plan.ID, status)
				if prURL != "" || prNumber > 0 {
					h.Queries.UpdateTaskPR(r.Context(), task.ID, plan.ID, prNumber, prURL)
				}
				log.Printf("github webhook: updated task %s in plan %s → %s", task.ID, plan.ID, status)

				// Insert event for the coordinator
				payload, _ := json.Marshal(map[string]interface{}{
					"branch":    branch,
					"status":    status,
					"pr_url":    prURL,
					"pr_number": prNumber,
				})
				h.Queries.InsertEvent(r.Context(), models.Event{
					PlanID:    plan.ID,
					TaskID:    task.ID,
					EventType: fmt.Sprintf("pr_%s", strings.ReplaceAll(status, "_", "")),
					Payload:   payload,
				})
				return
			}
		}
	}
}

func (h *GitHubWebhookHandler) updateWorkstreamByBranch(r *http.Request, branch, status, prURL string) {
	// Search workstreams for matching branch
	workstreams, _, err := h.Queries.ListWorkstreams(r.Context(), nil, "", 1, 100)
	if err != nil {
		return
	}
	for _, ws := range workstreams {
		if ws.BranchName == branch {
			h.Queries.UpdateWorkstreamStatus(r.Context(), ws.ID, status)
			if prURL != "" {
				h.Queries.UpdateWorkstreamPR(r.Context(), ws.ID, prURL)
			}
			log.Printf("github webhook: updated workstream %s → %s", ws.ID, status)
			return
		}
	}
}

type prCommentEvent struct {
	Action  string `json:"action"`
	Comment struct {
		ID              int64  `json:"id"`
		Body            string `json:"body"`
		Path            string `json:"path"`
		Line            int    `json:"line"`
		OriginalLine    int    `json:"original_line"`
		Side            string `json:"side"`
		DiffHunk        string `json:"diff_hunk"`
		CommitID        string `json:"commit_id"`
		User            struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

type issueCommentEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int `json:"number"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	} `json:"issue"`
	Comment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Repository struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
}

func (h *GitHubWebhookHandler) handlePRComment(w http.ResponseWriter, r *http.Request, body []byte) {
	var evt prCommentEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if evt.Action != "created" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	branch := evt.PullRequest.Head.Ref
	reviewer := evt.Comment.User.Login
	comment := evt.Comment.Body
	filePath := evt.Comment.Path
	diffHunk := evt.Comment.DiffHunk
	line := evt.Comment.Line

	log.Printf("github webhook: PR comment on %s by %s: %s", branch, reviewer, comment[:min(len(comment), 80)])

	// Build a detailed prompt with full review context
	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("## PR Review Comment from @%s\n\n", reviewer))
	if filePath != "" {
		prompt.WriteString(fmt.Sprintf("**File:** `%s`", filePath))
		if line > 0 {
			prompt.WriteString(fmt.Sprintf(" (line %d)", line))
		}
		prompt.WriteString("\n\n")
	}
	if diffHunk != "" {
		prompt.WriteString("**Code context:**\n```diff\n")
		prompt.WriteString(diffHunk)
		prompt.WriteString("\n```\n\n")
	}
	prompt.WriteString("**Comment:**\n")
	prompt.WriteString(comment)
	prompt.WriteString("\n\n---\n\nPlease address this review feedback. Read the file, understand the context, make the requested changes, and push them.")
	prompt.WriteString(fmt.Sprintf("\n\n<!-- smol-gang-meta: pr=%d comment_id=%d -->", evt.PullRequest.Number, evt.Comment.ID))

	h.forwardToAgent(r, branch, prompt.String())
	writeJSON(w, http.StatusOK, map[string]string{"status": "forwarded"})
}

func (h *GitHubWebhookHandler) handleIssueComment(w http.ResponseWriter, r *http.Request, body []byte) {
	var evt issueCommentEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Only handle comments on PRs, not issues
	if evt.Issue.PullRequest == nil || evt.Action != "created" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	reviewer := evt.Comment.User.Login
	comment := evt.Comment.Body
	prNumber := evt.Issue.Number

	log.Printf("github webhook: issue comment on PR #%d by %s: %s", prNumber, reviewer, comment[:min(len(comment), 80)])

	prompt := fmt.Sprintf("Comment on PR from @%s:\n\n%s\n\nPlease address this feedback and push the changes.\n\n<!-- smol-gang-meta: pr=%d comment_id=%d type=issue_comment -->", reviewer, comment, prNumber, evt.Comment.ID)

	h.forwardToAgentByPR(r, prNumber, prompt)
	writeJSON(w, http.StatusOK, map[string]string{"status": "forwarded"})
}

// forwardToAgent finds the agent pod for a branch and sends a message to it.
func (h *GitHubWebhookHandler) forwardToAgent(r *http.Request, branch, prompt string) {
	if h.K8s == nil {
		log.Printf("github webhook: k8s not available, cannot forward to agent")
		return
	}

	plans, _, _ := h.Queries.ListPlans(r.Context(), nil, models.PlanStatusActive, 1, 100)
	for _, plan := range plans {
		tasks, _ := h.Queries.GetTasksByPlanID(r.Context(), plan.ID)
		for _, task := range tasks {
			if task.BranchName == branch && task.WorkstreamID != nil {
				ws, err := h.Queries.GetWorkstreamByID(r.Context(), *task.WorkstreamID)
				if err != nil || ws.PodName == "" {
					continue
				}
				log.Printf("github webhook: forwarding comment to pod %s for task %s", ws.PodName, task.ID)
				go h.K8s.SendMessageToAgent(context.Background(), ws.PodName, prompt)

				// Also save as a message in the workstream
				h.Queries.CreateMessage(r.Context(), models.Message{
					WorkstreamID: *task.WorkstreamID,
					Source:       "github",
					Content:      prompt,
				})
				return
			}
		}
	}
	log.Printf("github webhook: no agent found for branch %s", branch)
}

// forwardToAgentByPR finds the agent pod by PR number and sends a message.
func (h *GitHubWebhookHandler) forwardToAgentByPR(r *http.Request, prNumber int, prompt string) {
	if h.K8s == nil {
		return
	}

	plans, _, _ := h.Queries.ListPlans(r.Context(), nil, models.PlanStatusActive, 1, 100)
	for _, plan := range plans {
		tasks, _ := h.Queries.GetTasksByPlanID(r.Context(), plan.ID)
		for _, task := range tasks {
			if task.PRNumber == prNumber && task.WorkstreamID != nil {
				ws, err := h.Queries.GetWorkstreamByID(r.Context(), *task.WorkstreamID)
				if err != nil || ws.PodName == "" {
					continue
				}
				log.Printf("github webhook: forwarding PR comment to pod %s for task %s", ws.PodName, task.ID)
				go h.K8s.SendMessageToAgent(context.Background(), ws.PodName, prompt)

				h.Queries.CreateMessage(r.Context(), models.Message{
					WorkstreamID: *task.WorkstreamID,
					Source:       "github",
					Content:      prompt,
				})
				return
			}
		}
	}
}

// promoteDownstreamTasks immediately marks pending tasks as ready if all their deps are now merged.
func (h *GitHubWebhookHandler) promoteDownstreamTasks(r *http.Request, mergedBranch string) {
	plans, _, err := h.Queries.ListPlans(r.Context(), nil, models.PlanStatusActive, 1, 100)
	if err != nil {
		return
	}

	for _, plan := range plans {
		tasks, err := h.Queries.GetTasksByPlanID(r.Context(), plan.ID)
		if err != nil {
			continue
		}

		taskMap := make(map[string]*models.Task)
		for i := range tasks {
			taskMap[tasks[i].ID] = &tasks[i]
		}

		for _, t := range tasks {
			if t.Status != models.TaskStatusPending {
				continue
			}
			allMet := true
			for _, depID := range t.DependsOn {
				dep, ok := taskMap[depID]
				if !ok || dep.Status != models.TaskStatusMerged {
					allMet = false
					break
				}
			}
			if allMet {
				h.Queries.UpdateTaskStatus(r.Context(), t.ID, plan.ID, models.TaskStatusReady)
				log.Printf("github webhook: promoted task %s to ready (deps met after merge)", t.ID)
			}
		}
	}
}

// cancelPlanByRootPR checks if the closed PR is a plan's root PR and cancels the entire plan.
// Returns true if a plan was cancelled.
// completePlanByRootPR checks if the merged PR is a plan's root PR.
// If so, marks the plan complete and deletes all task branches.
func (h *GitHubWebhookHandler) completePlanByRootPR(r *http.Request, prNumber int, branch string) bool {
	plans, _, err := h.Queries.ListPlans(r.Context(), nil, models.PlanStatusActive, 1, 100)
	if err != nil {
		return false
	}

	for _, plan := range plans {
		if (plan.RootPR > 0 && plan.RootPR == prNumber) || (plan.RootBranch != "" && plan.RootBranch == branch) {
			log.Printf("github webhook: root PR #%d merged for plan %s — completing plan and cleaning up branches", prNumber, plan.ID)

			// Get user token for GitHub API calls
			user, _ := h.Queries.GetUserByID(r.Context(), plan.CreatedByID)
			token := ""
			if user.GitHubAccessToken != "" {
				token = user.GitHubAccessToken
			}

			repo, _ := h.Queries.GetRepositoryByID(r.Context(), plan.RepositoryID)

			// Delete all task branches and clean up pods
			tasks, _ := h.Queries.GetTasksByPlanID(r.Context(), plan.ID)
			for _, t := range tasks {
				if t.BranchName != "" && token != "" && repo.GitHubOwner != "" {
					h.githubDeleteBranch(repo.GitHubOwner, repo.GitHubRepo, t.BranchName, token)
				}
				if t.Status != models.TaskStatusMerged {
					h.Queries.UpdateTaskStatus(r.Context(), t.ID, plan.ID, models.TaskStatusMerged)
				}
				if t.WorkstreamID != nil {
					h.Queries.UpdateWorkstreamStatus(r.Context(), *t.WorkstreamID, "completed")
					// Delete pod
					if h.K8s != nil {
						ws, err := h.Queries.GetWorkstreamByID(r.Context(), *t.WorkstreamID)
						if err == nil && ws.PodName != "" {
							h.K8s.DeleteAgentPod(r.Context(), ws.PodName, ws.ServiceName)
							log.Printf("webhook: deleted pod %s for completed task %s", ws.PodName, t.ID)
						}
					}
				}
			}

			// Delete the root branch itself (already merged into main)
			if plan.RootBranch != "" && token != "" && repo.GitHubOwner != "" {
				h.githubDeleteBranch(repo.GitHubOwner, repo.GitHubRepo, plan.RootBranch, token)
			}

			// Mark plan complete
			h.Queries.UpdatePlanStatus(r.Context(), plan.ID, models.PlanStatusComplete)

			// Log event
			payload, _ := json.Marshal(map[string]interface{}{
				"reason":    "root_pr_merged",
				"pr_number": prNumber,
				"branch":    branch,
			})
			h.Queries.InsertEvent(r.Context(), models.Event{
				PlanID:    plan.ID,
				EventType: "plan_completed",
				Payload:   payload,
			})

			return true
		}
	}
	return false
}

// githubDeleteBranch deletes a branch on GitHub.
func (h *GitHubWebhookHandler) githubDeleteBranch(owner, repo, branch, token string) {
	req, _ := http.NewRequest("DELETE",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", owner, repo, branch), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("webhook: failed to delete branch %s: %v", branch, err)
		return
	}
	resp.Body.Close()
	log.Printf("webhook: deleted branch %s (status %d)", branch, resp.StatusCode)
}

func (h *GitHubWebhookHandler) cancelPlanByRootPR(r *http.Request, prNumber int, branch string) bool {
	// Search active plans for one whose root PR or root branch matches
	plans, _, err := h.Queries.ListPlans(r.Context(), nil, models.PlanStatusActive, 1, 100)
	if err != nil {
		return false
	}

	for _, plan := range plans {
		if (plan.RootPR > 0 && plan.RootPR == prNumber) || (plan.RootBranch != "" && plan.RootBranch == branch) {
			log.Printf("github webhook: root PR #%d closed for plan %s — cancelling entire plan", prNumber, plan.ID)

			// Cancel all non-terminal tasks and clean up pods
			tasks, _ := h.Queries.GetTasksByPlanID(r.Context(), plan.ID)
			for _, t := range tasks {
				if t.Status != models.TaskStatusMerged && t.Status != models.TaskStatusCancelled && t.Status != models.TaskStatusFailed {
					h.Queries.UpdateTaskStatus(r.Context(), t.ID, plan.ID, models.TaskStatusCancelled)

					// Cancel linked workstream and delete pod
					if t.WorkstreamID != nil {
						h.Queries.UpdateWorkstreamStatus(r.Context(), *t.WorkstreamID, "cancelled")
						if h.K8s != nil {
							ws, err := h.Queries.GetWorkstreamByID(r.Context(), *t.WorkstreamID)
							if err == nil && ws.PodName != "" {
								h.K8s.DeleteAgentPod(r.Context(), ws.PodName, ws.ServiceName)
								log.Printf("webhook: deleted pod %s for cancelled task %s", ws.PodName, t.ID)
							}
						}
					}
				}
			}

			// Mark plan as halted
			h.Queries.UpdatePlanStatus(r.Context(), plan.ID, models.PlanStatusHalted)

			// Insert event
			payload, _ := json.Marshal(map[string]interface{}{
				"reason":    "root_pr_closed",
				"pr_number": prNumber,
				"branch":    branch,
			})
			h.Queries.InsertEvent(r.Context(), models.Event{
				PlanID:    plan.ID,
				EventType: "plan_cancelled",
				Payload:   payload,
			})

			return true
		}
	}

	return false
}

func verifySignature(payload []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig, err := hex.DecodeString(signature[7:])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}
