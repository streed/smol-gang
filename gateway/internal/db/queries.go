package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/streed/smol-cluster/gateway/internal/models"
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
		`INSERT INTO users (email, password_hash, name, role) VALUES ($1, $2, $3, $4)
		 RETURNING id, email, password_hash, name, role, created_at, updated_at`,
		u.Email, u.PasswordHash, u.Name, u.Role,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, created_at, updated_at FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := q.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, created_at, updated_at FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt)
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
		`SELECT id, email, password_hash, name, role, created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		perPage, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
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
		 RETURNING id, email, password_hash, name, role, created_at, updated_at`,
		id, name, role,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt)
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
		`INSERT INTO workstreams (name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, llm_config, created_by_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, created_by_id, created_at, updated_at, completed_at`,
		ws.Name, ws.Description, ws.RepositoryID, ws.BranchName, ws.Status, ws.PodName, ws.ServiceName, portJSON, llmJSON, ws.CreatedByID,
	).Scan(&w.ID, &w.Name, &w.Description, &w.RepositoryID, &w.BranchName, &w.Status, &w.PodName, &w.ServiceName,
		&portBytes, &w.PullRequestURL, &llmBytes, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt)
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
		`SELECT id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, created_by_id, created_at, updated_at, completed_at
		 FROM workstreams WHERE id = $1`, id,
	).Scan(&w.ID, &w.Name, &w.Description, &w.RepositoryID, &w.BranchName, &w.Status, &w.PodName, &w.ServiceName,
		&portBytes, &w.PullRequestURL, &llmBytes, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt)
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

	selectQuery := `SELECT id, name, description, repository_id, branch_name, status, pod_name, service_name, port_mappings, pull_request_url, llm_config, created_by_id, created_at, updated_at, completed_at
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
			&portBytes, &w.PullRequestURL, &llmBytes, &w.CreatedByID, &w.CreatedAt, &w.UpdatedAt, &w.CompletedAt); err != nil {
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

	offset := (page - 1) * perPage
	rows, err := q.Pool.Query(ctx,
		`SELECT id, workstream_id, user_id, source, content, created_at
		 FROM messages WHERE workstream_id = $1 ORDER BY created_at ASC LIMIT $2 OFFSET $3`,
		workstreamID, perPage, offset,
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
