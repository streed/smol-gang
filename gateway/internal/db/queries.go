package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/streed/smol-gang/gateway/internal/models"
)

type Queries struct {
	Pool *pgxpool.Pool
}

func NewQueries(pool *pgxpool.Pool) *Queries {
	return &Queries{Pool: pool}
}

// --- Users ---

func (q *Queries) CreateUser(ctx context.Context, u models.User) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name, role, github_id, github_login, github_access_token) VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, email, password_hash, name, role, github_id, github_login, github_access_token, created_at, updated_at`,
		u.Email, u.PasswordHash, u.Name, u.Role, u.GitHubID, u.GitHubLogin, u.GitHubAccessToken,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.GitHubID, &user.GitHubLogin, &user.GitHubAccessToken, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, github_id, github_login, github_access_token, created_at, updated_at FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.GitHubID, &user.GitHubLogin, &user.GitHubAccessToken, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, github_id, github_login, github_access_token, created_at, updated_at FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.GitHubID, &user.GitHubLogin, &user.GitHubAccessToken, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) ListUsers(ctx context.Context, page, perPage int) ([]models.User, int64, error) {
	var total int64
	err := q.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := q.Pool.Query(ctx,
		`SELECT id, email, password_hash, name, role, github_id, github_login, github_access_token, created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		perPage, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.GitHubID, &u.GitHubLogin, &u.GitHubAccessToken, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (q *Queries) UpdateUser(ctx context.Context, id uuid.UUID, name, role *string) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`UPDATE users SET
			name = COALESCE($2, name),
			role = COALESCE($3, role)
		 WHERE id = $1
		 RETURNING id, email, password_hash, name, role, github_id, github_login, github_access_token, created_at, updated_at`,
		id, name, role,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.GitHubID, &user.GitHubLogin, &user.GitHubAccessToken, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (q *Queries) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := q.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (q *Queries) GetUserByGitHubID(ctx context.Context, githubID int64) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, github_id, github_login, github_access_token, created_at, updated_at FROM users WHERE github_id = $1`, githubID,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.GitHubID, &user.GitHubLogin, &user.GitHubAccessToken, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) UpdateUserGitHubToken(ctx context.Context, userID uuid.UUID, token string, login string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE users SET github_access_token = $2, github_login = $3, updated_at = NOW() WHERE id = $1`,
		userID, token, login)
	return err
}

// --- Repositories ---

func (q *Queries) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	configJSON, err := json.Marshal(r.Config)
	if err != nil {
		configJSON = []byte("{}")
	}

	var repo models.Repository
	var configBytes []byte
	err = q.Pool.QueryRow(ctx,
		`INSERT INTO repositories (name, git_url, default_branch, github_owner, github_repo, github_installation_id, config, created_by_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, name, git_url, default_branch, github_owner, github_repo, github_installation_id, config, created_by_id, created_at, updated_at`,
		r.Name, r.GitURL, r.DefaultBranch, r.GitHubOwner, r.GitHubRepo, r.GitHubInstallationID, configJSON, r.CreatedByID,
	).Scan(&repo.ID, &repo.Name, &repo.GitURL, &repo.DefaultBranch, &repo.GitHubOwner, &repo.GitHubRepo,
		&repo.GitHubInstallationID, &configBytes, &repo.CreatedByID, &repo.CreatedAt, &repo.UpdatedAt)
	if err != nil {
		return repo, err
	}
	if len(configBytes) > 0 {
		json.Unmarshal(configBytes, &repo.Config)
	}
	return repo, nil
}

func (q *Queries) GetRepositoryByID(ctx context.Context, id uuid.UUID) (models.Repository, error) {
	var repo models.Repository
	var configBytes []byte
	err := q.Pool.QueryRow(ctx,
		`SELECT id, name, git_url, default_branch, github_owner, github_repo, github_installation_id, config, created_by_id, created_at, updated_at
		 FROM repositories WHERE id = $1`, id,
	).Scan(&repo.ID, &repo.Name, &repo.GitURL, &repo.DefaultBranch, &repo.GitHubOwner, &repo.GitHubRepo,
		&repo.GitHubInstallationID, &configBytes, &repo.CreatedByID, &repo.CreatedAt, &repo.UpdatedAt)
	if err != nil {
		return repo, err
	}
	if len(configBytes) > 0 {
		json.Unmarshal(configBytes, &repo.Config)
	}
	return repo, nil
}

func (q *Queries) ListRepositories(ctx context.Context, page, perPage int) ([]models.Repository, int64, error) {
	var total int64
	err := q.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM repositories`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := q.Pool.Query(ctx,
		`SELECT id, name, git_url, default_branch, github_owner, github_repo, github_installation_id, config, created_by_id, created_at, updated_at
		 FROM repositories ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		perPage, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var repos []models.Repository
	for rows.Next() {
		var r models.Repository
		var configBytes []byte
		if err := rows.Scan(&r.ID, &r.Name, &r.GitURL, &r.DefaultBranch, &r.GitHubOwner, &r.GitHubRepo,
			&r.GitHubInstallationID, &configBytes, &r.CreatedByID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if len(configBytes) > 0 {
			json.Unmarshal(configBytes, &r.Config)
		}
		repos = append(repos, r)
	}
	return repos, total, nil
}

func (q *Queries) UpdateRepository(ctx context.Context, id uuid.UUID, req models.UpdateRepositoryRequest) (models.Repository, error) {
	var configJSON *[]byte
	if req.Config != nil {
		b, err := json.Marshal(req.Config)
		if err != nil {
			return models.Repository{}, err
		}
		configJSON = &b
	}

	var repo models.Repository
	var configBytes []byte
	err := q.Pool.QueryRow(ctx,
		`UPDATE repositories SET
			name = COALESCE($2, name),
			default_branch = COALESCE($3, default_branch),
			config = COALESCE($4, config)
		 WHERE id = $1
		 RETURNING id, name, git_url, default_branch, github_owner, github_repo, github_installation_id, config, created_by_id, created_at, updated_at`,
		id, req.Name, req.DefaultBranch, configJSON,
	).Scan(&repo.ID, &repo.Name, &repo.GitURL, &repo.DefaultBranch, &repo.GitHubOwner, &repo.GitHubRepo,
		&repo.GitHubInstallationID, &configBytes, &repo.CreatedByID, &repo.CreatedAt, &repo.UpdatedAt)
	if err != nil {
		return repo, err
	}
	if len(configBytes) > 0 {
		json.Unmarshal(configBytes, &repo.Config)
	}
	return repo, nil
}

func (q *Queries) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	return err
}

// --- Workstreams ---

func (q *Queries) CreateWorkstream(ctx context.Context, ws models.Workstream) (models.Workstream, error) {
	portJSON, _ := json.Marshal(ws.PortMappings)
	llmJSON, _ := json.Marshal(ws.LLMConfig)

	var w models.Workstream
	var portBytes, llmBytes []byte
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO workstreams (name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, llm_config, parent_workstream_id, created_by_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, parent_workstream_id, created_by_id, created_at, updated_at, completed_at`,
		ws.Name, ws.Description, ws.RepositoryID, ws.BranchName, ws.Status, ws.PodName, ws.ServiceName, portJSON, llmJSON, ws.ParentWorkstreamID, ws.CreatedByID,
	).Scan(&w.ID, &w.Name, &w.Description, &w.RepositoryID, &w.BranchName, &w.Status, &w.PodName, &w.ServiceName,
		&portBytes, &w.PullRequestURL, &llmBytes, &w.ParentWorkstreamID, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt)
	if err != nil {
		return w, err
	}
	json.Unmarshal(portBytes, &w.PortMappings)
	json.Unmarshal(llmBytes, &w.LLMConfig)
	return w, nil
}

func (q *Queries) GetWorkstreamByID(ctx context.Context, id uuid.UUID) (models.Workstream, error) {
	var w models.Workstream
	var portBytes, llmBytes []byte
	err := q.Pool.QueryRow(ctx,
		`SELECT id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, parent_workstream_id, created_by_id, created_at, updated_at, completed_at
		 FROM workstreams WHERE id = $1`, id,
	).Scan(&w.ID, &w.Name, &w.Description, &w.RepositoryID, &w.BranchName, &w.Status, &w.PodName, &w.ServiceName,
		&portBytes, &w.PullRequestURL, &llmBytes, &w.ParentWorkstreamID, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt)
	if err != nil {
		return w, err
	}
	json.Unmarshal(portBytes, &w.PortMappings)
	json.Unmarshal(llmBytes, &w.LLMConfig)
	return w, nil
}

func (q *Queries) ListWorkstreams(ctx context.Context, repoID *uuid.UUID, status string, page, perPage int) ([]models.Workstream, int64, error) {
	query := `SELECT COUNT(*) FROM workstreams WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if repoID != nil {
		query += fmt.Sprintf(` AND repository_id = $%d`, argIdx)
		args = append(args, *repoID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int64
	err := q.Pool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := `SELECT id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, parent_workstream_id, created_by_id, created_at, updated_at, completed_at
		FROM workstreams WHERE 1=1`
	selectArgs := []interface{}{}
	selectIdx := 1

	if repoID != nil {
		selectQuery += fmt.Sprintf(` AND repository_id = $%d`, selectIdx)
		selectArgs = append(selectArgs, *repoID)
		selectIdx++
	}
	if status != "" {
		selectQuery += fmt.Sprintf(` AND status = $%d`, selectIdx)
		selectArgs = append(selectArgs, status)
		selectIdx++
	}

	selectQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, selectIdx, selectIdx+1)
	offset := (page - 1) * perPage
	selectArgs = append(selectArgs, perPage, offset)

	rows, err := q.Pool.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var workstreams []models.Workstream
	for rows.Next() {
		var w models.Workstream
		var portBytes, llmBytes []byte
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.RepositoryID, &w.BranchName, &w.Status, &w.PodName, &w.ServiceName,
			&portBytes, &w.PullRequestURL, &llmBytes, &w.ParentWorkstreamID, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt); err != nil {
			return nil, 0, err
		}
		json.Unmarshal(portBytes, &w.PortMappings)
		json.Unmarshal(llmBytes, &w.LLMConfig)
		workstreams = append(workstreams, w)
	}
	return workstreams, total, nil
}

func (q *Queries) UpdateWorkstreamStatus(ctx context.Context, id uuid.UUID, status string) error {
	sql := `UPDATE workstreams SET status = $2 WHERE id = $1`
	if status == "completed" || status == "failed" || status == "cancelled" {
		sql = `UPDATE workstreams SET status = $2, completed_at = NOW() WHERE id = $1`
	}
	_, err := q.Pool.Exec(ctx, sql, id, status)
	return err
}

func (q *Queries) UpdateWorkstreamPR(ctx context.Context, id uuid.UUID, prURL string) error {
	_, err := q.Pool.Exec(ctx, `UPDATE workstreams SET pull_request_url = $2 WHERE id = $1`, id, prURL)
	return err
}

func (q *Queries) UpdateWorkstreamPod(ctx context.Context, id uuid.UUID, podName, serviceName string) error {
	_, err := q.Pool.Exec(ctx, `UPDATE workstreams SET pod_name = $2, service_name = $3 WHERE id = $1`, id, podName, serviceName)
	return err
}

// ListChildWorkstreams returns all workstreams whose parent is the given workstream ID.
func (q *Queries) ListChildWorkstreams(ctx context.Context, parentID uuid.UUID) ([]models.Workstream, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, parent_workstream_id, created_by_id, created_at, updated_at, completed_at
		 FROM workstreams WHERE parent_workstream_id = $1
		 ORDER BY created_at ASC`, parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workstreams []models.Workstream
	for rows.Next() {
		var w models.Workstream
		var portBytes, llmBytes []byte
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.RepositoryID, &w.BranchName, &w.Status, &w.PodName, &w.ServiceName,
			&portBytes, &w.PullRequestURL, &llmBytes, &w.ParentWorkstreamID, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(portBytes, &w.PortMappings)
		json.Unmarshal(llmBytes, &w.LLMConfig)
		workstreams = append(workstreams, w)
	}
	return workstreams, nil
}

// GetLatestMessageByWorkstreamID returns the most recent message from a given workstream.
func (q *Queries) GetLatestMessageByWorkstreamID(ctx context.Context, workstreamID uuid.UUID) (models.Message, error) {
	var m models.Message
	err := q.Pool.QueryRow(ctx,
		`SELECT id, workstream_id, user_id, source, content, created_at
		 FROM messages WHERE workstream_id = $1 AND source = 'agent'
		 ORDER BY created_at DESC LIMIT 1`, workstreamID,
	).Scan(&m.ID, &m.WorkstreamID, &m.UserID, &m.Source, &m.Content, &m.CreatedAt)
	return m, err
}

// --- Messages ---

func (q *Queries) CreateMessage(ctx context.Context, m models.Message) (models.Message, error) {
	var msg models.Message
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO messages (workstream_id, user_id, source, content) VALUES ($1, $2, $3, $4)
		 RETURNING id, workstream_id, user_id, source, content, created_at`,
		m.WorkstreamID, m.UserID, m.Source, m.Content,
	).Scan(&msg.ID, &msg.WorkstreamID, &msg.UserID, &msg.Source, &msg.Content, &msg.CreatedAt)
	return msg, err
}

func (q *Queries) ListMessages(ctx context.Context, workstreamID uuid.UUID, page, perPage int) ([]models.Message, int64, error) {
	var total int64
	err := q.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE workstream_id = $1`, workstreamID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch the most recent N messages, then return in chronological order
	rows, err := q.Pool.Query(ctx,
		`SELECT id, workstream_id, user_id, source, content, created_at FROM (
			SELECT id, workstream_id, user_id, source, content, created_at
			FROM messages WHERE workstream_id = $1
			ORDER BY created_at DESC LIMIT $2
		) sub ORDER BY created_at ASC`,
		workstreamID, perPage,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.WorkstreamID, &m.UserID, &m.Source, &m.Content, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		messages = append(messages, m)
	}
	return messages, total, nil
}

// --- Audit Logs ---

func (q *Queries) CreateAuditLog(ctx context.Context, log models.AuditLog) error {
	_, err := q.Pool.Exec(ctx,
		`INSERT INTO audit_logs (user_id, workstream_id, action, resource, resource_id, details, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		log.UserID, log.WorkstreamID, log.Action, log.Resource, log.ResourceID, log.Details, log.IPAddress,
	)
	return err
}

func (q *Queries) ListAuditLogs(ctx context.Context, userID *uuid.UUID, workstreamID *uuid.UUID, action string, page, perPage int) ([]models.AuditLog, int64, error) {
	query := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if userID != nil {
		query += fmt.Sprintf(` AND user_id = $%d`, argIdx)
		args = append(args, *userID)
		argIdx++
	}
	if workstreamID != nil {
		query += fmt.Sprintf(` AND workstream_id = $%d`, argIdx)
		args = append(args, *workstreamID)
		argIdx++
	}
	if action != "" {
		query += fmt.Sprintf(` AND action = $%d`, argIdx)
		args = append(args, action)
		argIdx++
	}

	var total int64
	err := q.Pool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := `SELECT id, user_id, workstream_id, action, resource, resource_id, details, ip_address, created_at
		FROM audit_logs WHERE 1=1`
	selectArgs := []interface{}{}
	selectIdx := 1

	if userID != nil {
		selectQuery += fmt.Sprintf(` AND user_id = $%d`, selectIdx)
		selectArgs = append(selectArgs, *userID)
		selectIdx++
	}
	if workstreamID != nil {
		selectQuery += fmt.Sprintf(` AND workstream_id = $%d`, selectIdx)
		selectArgs = append(selectArgs, *workstreamID)
		selectIdx++
	}
	if action != "" {
		selectQuery += fmt.Sprintf(` AND action = $%d`, selectIdx)
		selectArgs = append(selectArgs, action)
		selectIdx++
	}

	selectQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, selectIdx, selectIdx+1)
	offset := (page - 1) * perPage
	selectArgs = append(selectArgs, perPage, offset)

	rows, err := q.Pool.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.WorkstreamID, &l.Action, &l.Resource, &l.ResourceID, &l.Details, &l.IPAddress, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}

// --- Plans ---

func (q *Queries) CreatePlan(ctx context.Context, p models.Plan) (models.Plan, error) {
	var plan models.Plan
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO plans (prompt, plan_json, root_branch, root_pr, status, repository_id, base_branch, complexity, complexity_reasoning, created_by_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, prompt, plan_json, root_branch, root_pr, status, repository_id, base_branch, complexity, complexity_reasoning, conversations, created_by_id, created_at, updated_at`,
		p.Prompt, p.PlanJSON, p.RootBranch, p.RootPR, p.Status, p.RepositoryID, p.BaseBranch, p.Complexity, p.ComplexityReasoning, p.CreatedByID,
	).Scan(&plan.ID, &plan.Prompt, &plan.PlanJSON, &plan.RootBranch, &plan.RootPR, &plan.Status,
		&plan.RepositoryID, &plan.BaseBranch, &plan.Complexity, &plan.ComplexityReasoning, &plan.Conversations, &plan.CreatedByID,
		&plan.CreatedAt, &plan.UpdatedAt)
	return plan, err
}

func (q *Queries) GetPlanByID(ctx context.Context, id uuid.UUID) (models.Plan, error) {
	var plan models.Plan
	err := q.Pool.QueryRow(ctx,
		`SELECT id, prompt, plan_json, root_branch, root_pr, status, repository_id, base_branch, complexity, complexity_reasoning, conversations, created_by_id, created_at, updated_at
		 FROM plans WHERE id = $1`, id,
	).Scan(&plan.ID, &plan.Prompt, &plan.PlanJSON, &plan.RootBranch, &plan.RootPR, &plan.Status,
		&plan.RepositoryID, &plan.BaseBranch, &plan.Complexity, &plan.ComplexityReasoning, &plan.Conversations, &plan.CreatedByID,
		&plan.CreatedAt, &plan.UpdatedAt)
	return plan, err
}

func (q *Queries) UpdatePlanStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := q.Pool.Exec(ctx, `UPDATE plans SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	return err
}

func (q *Queries) UpdatePlanJSON(ctx context.Context, id uuid.UUID, planJSON json.RawMessage) error {
	_, err := q.Pool.Exec(ctx, `UPDATE plans SET plan_json = $2, updated_at = NOW() WHERE id = $1`, id, planJSON)
	return err
}

func (q *Queries) UpdatePlanComplexity(ctx context.Context, id uuid.UUID, complexity, reasoning string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE plans SET complexity = $2, complexity_reasoning = $3, updated_at = NOW() WHERE id = $1`,
		id, complexity, reasoning)
	return err
}

func (q *Queries) UpdatePlanConversations(ctx context.Context, id uuid.UUID, conversations json.RawMessage) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE plans SET conversations = $2, updated_at = NOW() WHERE id = $1`,
		id, conversations)
	return err
}

func (q *Queries) UpdatePlanRootPR(ctx context.Context, id uuid.UUID, rootBranch string, rootPR int) error {
	_, err := q.Pool.Exec(ctx, `UPDATE plans SET root_branch = $2, root_pr = $3, updated_at = NOW() WHERE id = $1`, id, rootBranch, rootPR)
	return err
}

func (q *Queries) ListPlans(ctx context.Context, repoID *uuid.UUID, status string, page, perPage int) ([]models.Plan, int64, error) {
	query := `SELECT COUNT(*) FROM plans WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if repoID != nil {
		query += fmt.Sprintf(` AND repository_id = $%d`, argIdx)
		args = append(args, *repoID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int64
	err := q.Pool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := `SELECT id, prompt, plan_json, root_branch, root_pr, status, repository_id, base_branch, complexity, complexity_reasoning, conversations, created_by_id, created_at, updated_at
		FROM plans WHERE 1=1`
	selectArgs := []interface{}{}
	selectIdx := 1

	if repoID != nil {
		selectQuery += fmt.Sprintf(` AND repository_id = $%d`, selectIdx)
		selectArgs = append(selectArgs, *repoID)
		selectIdx++
	}
	if status != "" {
		selectQuery += fmt.Sprintf(` AND status = $%d`, selectIdx)
		selectArgs = append(selectArgs, status)
		selectIdx++
	}

	selectQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, selectIdx, selectIdx+1)
	offset := (page - 1) * perPage
	selectArgs = append(selectArgs, perPage, offset)

	rows, err := q.Pool.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var plans []models.Plan
	for rows.Next() {
		var p models.Plan
		if err := rows.Scan(&p.ID, &p.Prompt, &p.PlanJSON, &p.RootBranch, &p.RootPR, &p.Status,
			&p.RepositoryID, &p.BaseBranch, &p.Complexity, &p.ComplexityReasoning, &p.Conversations, &p.CreatedByID,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		plans = append(plans, p)
	}
	return plans, total, nil
}

func (q *Queries) DeletePlan(ctx context.Context, id uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `DELETE FROM plans WHERE id = $1`, id)
	return err
}

// --- Tasks ---

func (q *Queries) CreateTask(ctx context.Context, t models.Task) (models.Task, error) {
	depsJSON, _ := json.Marshal(t.DependsOn)
	scopeJSON, _ := json.Marshal(t.FileScope)
	criteriaJSON, _ := json.Marshal(t.AcceptanceCriteria)

	var task models.Task
	var depsBytes, scopeBytes, criteriaBytes []byte
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO tasks (id, plan_id, description, depends_on, file_scope, acceptance_criteria, model_tier, status, branch_name, wave)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, plan_id, description, depends_on, file_scope, acceptance_criteria, model_tier, status, branch_name, pr_number, pr_url, worker_id, lease_expiry, workstream_id, error, wave, created_at, updated_at`,
		t.ID, t.PlanID, t.Description, depsJSON, scopeJSON, criteriaJSON, t.ModelTier, t.Status, t.BranchName, t.Wave,
	).Scan(&task.ID, &task.PlanID, &task.Description, &depsBytes, &scopeBytes, &criteriaBytes,
		&task.ModelTier, &task.Status, &task.BranchName, &task.PRNumber, &task.PRURL,
		&task.WorkerID, &task.LeaseExpiry, &task.WorkstreamID, &task.Error, &task.Wave,
		&task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return task, err
	}
	json.Unmarshal(depsBytes, &task.DependsOn)
	json.Unmarshal(scopeBytes, &task.FileScope)
	json.Unmarshal(criteriaBytes, &task.AcceptanceCriteria)
	return task, nil
}

func (q *Queries) GetTasksByPlanID(ctx context.Context, planID uuid.UUID) ([]models.Task, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, plan_id, description, depends_on, file_scope, acceptance_criteria, model_tier, status, branch_name, pr_number, pr_url, worker_id, lease_expiry, workstream_id, error, wave, created_at, updated_at
		 FROM tasks WHERE plan_id = $1 ORDER BY wave, id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var depsBytes, scopeBytes, criteriaBytes []byte
		if err := rows.Scan(&t.ID, &t.PlanID, &t.Description, &depsBytes, &scopeBytes, &criteriaBytes,
			&t.ModelTier, &t.Status, &t.BranchName, &t.PRNumber, &t.PRURL,
			&t.WorkerID, &t.LeaseExpiry, &t.WorkstreamID, &t.Error, &t.Wave,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(depsBytes, &t.DependsOn)
		json.Unmarshal(scopeBytes, &t.FileScope)
		json.Unmarshal(criteriaBytes, &t.AcceptanceCriteria)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (q *Queries) GetTask(ctx context.Context, taskID string, planID uuid.UUID) (models.Task, error) {
	var t models.Task
	var depsBytes, scopeBytes, criteriaBytes []byte
	err := q.Pool.QueryRow(ctx,
		`SELECT id, plan_id, description, depends_on, file_scope, acceptance_criteria, model_tier, status, branch_name, pr_number, pr_url, worker_id, lease_expiry, workstream_id, error, wave, created_at, updated_at
		 FROM tasks WHERE id = $1 AND plan_id = $2`, taskID, planID,
	).Scan(&t.ID, &t.PlanID, &t.Description, &depsBytes, &scopeBytes, &criteriaBytes,
		&t.ModelTier, &t.Status, &t.BranchName, &t.PRNumber, &t.PRURL,
		&t.WorkerID, &t.LeaseExpiry, &t.WorkstreamID, &t.Error, &t.Wave,
		&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	json.Unmarshal(depsBytes, &t.DependsOn)
	json.Unmarshal(scopeBytes, &t.FileScope)
	json.Unmarshal(criteriaBytes, &t.AcceptanceCriteria)
	return t, nil
}

func (q *Queries) UpdateTaskStatus(ctx context.Context, taskID string, planID uuid.UUID, status string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET status = $3, updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, status)
	return err
}

func (q *Queries) UpdateTaskPR(ctx context.Context, taskID string, planID uuid.UUID, prNumber int, prURL string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET pr_number = $3, pr_url = $4, status = 'pr_open', updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, prNumber, prURL)
	return err
}

func (q *Queries) UpdateTaskDescription(ctx context.Context, taskID string, planID uuid.UUID, desc string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET description = $3, updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, desc)
	return err
}

func (q *Queries) UpdateTaskDeps(ctx context.Context, taskID string, planID uuid.UUID, deps []string) error {
	depsJSON, _ := json.Marshal(deps)
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET depends_on = $3, updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, depsJSON)
	return err
}

func (q *Queries) UpdateTaskCriteria(ctx context.Context, taskID string, planID uuid.UUID, criteria []string) error {
	criteriaJSON, _ := json.Marshal(criteria)
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET acceptance_criteria = $3, updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, criteriaJSON)
	return err
}

func (q *Queries) UpdateTaskBranch(ctx context.Context, taskID string, planID uuid.UUID, branch string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET branch_name = $3, updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, branch)
	return err
}

func (q *Queries) DeleteTask(ctx context.Context, taskID string, planID uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1 AND plan_id = $2`, taskID, planID)
	return err
}

func (q *Queries) ClaimTask(ctx context.Context, taskID string, planID uuid.UUID, workerID string, leaseExpiry time.Time) (bool, error) {
	result, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET worker_id = $3, lease_expiry = $4, status = 'claimed', updated_at = NOW()
		 WHERE id = $1 AND plan_id = $2 AND status = 'ready' AND worker_id IS NULL`,
		taskID, planID, workerID, leaseExpiry)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func (q *Queries) ExtendLease(ctx context.Context, taskID string, planID uuid.UUID, workerID string, newExpiry time.Time) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET lease_expiry = $4, updated_at = NOW()
		 WHERE id = $1 AND plan_id = $2 AND worker_id = $3`,
		taskID, planID, workerID, newExpiry)
	return err
}

func (q *Queries) ReclaimTask(ctx context.Context, taskID string, planID uuid.UUID) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET worker_id = NULL, lease_expiry = NULL, status = 'ready', updated_at = NOW()
		 WHERE id = $1 AND plan_id = $2 AND lease_expiry < NOW()`,
		taskID, planID)
	return err
}

func (q *Queries) FindExpiredLeases(ctx context.Context, planID uuid.UUID) ([]models.Task, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, plan_id, worker_id, status FROM tasks
		 WHERE plan_id = $1 AND lease_expiry < NOW() AND status IN ('claimed', 'working')`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.PlanID, &t.WorkerID, &t.Status); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (q *Queries) LinkTaskWorkstream(ctx context.Context, taskID string, planID uuid.UUID, workstreamID uuid.UUID) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE tasks SET workstream_id = $3, updated_at = NOW() WHERE id = $1 AND plan_id = $2`,
		taskID, planID, workstreamID)
	return err
}

// --- Workstream/Message Deletion ---

func (q *Queries) DeleteWorkstreamMessages(ctx context.Context, workstreamID uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `DELETE FROM messages WHERE workstream_id = $1`, workstreamID)
	return err
}

func (q *Queries) DeleteWorkstream(ctx context.Context, id uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `DELETE FROM workstreams WHERE id = $1`, id)
	return err
}

// --- Task Conversations ---

func (q *Queries) CreateTaskConversation(ctx context.Context, c models.TaskConversation) error {
	_, err := q.Pool.Exec(ctx,
		`INSERT INTO task_conversations (task_id, plan_id, role, content, sequence) VALUES ($1, $2, $3, $4, $5)`,
		c.TaskID, c.PlanID, c.Role, c.Content, c.Sequence)
	return err
}

func (q *Queries) GetTaskConversations(ctx context.Context, taskID string, planID uuid.UUID) ([]models.TaskConversation, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, task_id, plan_id, role, content, sequence, created_at
		 FROM task_conversations WHERE task_id = $1 AND plan_id = $2 ORDER BY sequence`, taskID, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convos []models.TaskConversation
	for rows.Next() {
		var c models.TaskConversation
		if err := rows.Scan(&c.ID, &c.TaskID, &c.PlanID, &c.Role, &c.Content, &c.Sequence, &c.CreatedAt); err != nil {
			return nil, err
		}
		convos = append(convos, c)
	}
	return convos, nil
}

// --- Events ---

func (q *Queries) InsertEvent(ctx context.Context, e models.Event) error {
	_, err := q.Pool.Exec(ctx,
		`INSERT INTO events (plan_id, task_id, event_type, payload) VALUES ($1, $2, $3, $4)`,
		e.PlanID, e.TaskID, e.EventType, e.Payload)
	return err
}

func (q *Queries) EventProcessed(ctx context.Context, eventID uuid.UUID) bool {
	var processed bool
	err := q.Pool.QueryRow(ctx, `SELECT processed FROM events WHERE id = $1`, eventID).Scan(&processed)
	if err != nil {
		return false
	}
	return processed
}

func (q *Queries) MarkEventProcessed(ctx context.Context, eventID uuid.UUID) error {
	_, err := q.Pool.Exec(ctx, `UPDATE events SET processed = TRUE WHERE id = $1`, eventID)
	return err
}

func (q *Queries) GetUnprocessedEvents(ctx context.Context, planID uuid.UUID) ([]models.Event, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, plan_id, task_id, event_type, payload, processed, created_at
		 FROM events WHERE plan_id = $1 AND processed = FALSE ORDER BY created_at`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.PlanID, &e.TaskID, &e.EventType, &e.Payload, &e.Processed, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}
