package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		migrationCreateExtensions,
		migrationCreateUsers,
		migrationCreateRepositories,
		migrationCreateWorkstreams,
		migrationCreateAuditLogs,
		migrationCreateMessages,
		migrationCreateIndexes,
		migrationAddGitHubToUsers,
		migrationCreateDAGTables,
		migrationAddPlanConversations,
		migrationAddBackgroundAgents,
	}

	for i, migration := range migrations {
		if _, err := pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
		log.Printf("Migration %d applied successfully", i)
	}

	return nil
}

const migrationCreateExtensions = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
`

const migrationCreateUsers = `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const migrationCreateRepositories = `
CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    git_url TEXT NOT NULL,
    default_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    github_owner VARCHAR(255) NOT NULL DEFAULT '',
    github_repo VARCHAR(255) NOT NULL DEFAULT '',
    github_installation_id BIGINT NOT NULL DEFAULT 0,
    config JSONB,
    created_by_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const migrationCreateWorkstreams = `
CREATE TABLE IF NOT EXISTS workstreams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    repository_id UUID NOT NULL REFERENCES repositories(id),
    branch_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    pod_name VARCHAR(255) NOT NULL DEFAULT '',
    service_name VARCHAR(255) NOT NULL DEFAULT '',
    port_mappings JSONB DEFAULT '[]',
    pull_request_url TEXT NOT NULL DEFAULT '',
    llm_config JSONB,
    created_by_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
`

const migrationCreateAuditLogs = `
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    workstream_id UUID REFERENCES workstreams(id),
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255) NOT NULL DEFAULT '',
    details JSONB,
    ip_address VARCHAR(45) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const migrationCreateMessages = `
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workstream_id UUID NOT NULL REFERENCES workstreams(id),
    user_id UUID REFERENCES users(id),
    source VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const migrationCreateIndexes = `
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_workstreams_repo ON workstreams(repository_id);
CREATE INDEX IF NOT EXISTS idx_workstreams_status ON workstreams(status);
CREATE INDEX IF NOT EXISTS idx_workstreams_created_by ON workstreams(created_by_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_workstream ON audit_logs(workstream_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_workstream ON messages(workstream_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
`

const migrationAddGitHubToUsers = `
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_id BIGINT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_login VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_access_token TEXT NOT NULL DEFAULT '';
ALTER TABLE users ALTER COLUMN password_hash SET DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id) WHERE github_id IS NOT NULL;
`

const migrationCreateDAGTables = `
-- Plans table: top-level execution unit
CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    prompt TEXT NOT NULL,
    plan_json JSONB,
    root_branch VARCHAR(255) NOT NULL DEFAULT '',
    root_pr INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'pending_approval',
    repository_id UUID NOT NULL REFERENCES repositories(id),
    base_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    complexity VARCHAR(20) NOT NULL DEFAULT 'simple',
    complexity_reasoning TEXT NOT NULL DEFAULT '',
    created_by_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tasks table: individual work items within a plan
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(255) NOT NULL,
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    description TEXT NOT NULL DEFAULT '',
    depends_on JSONB DEFAULT '[]',
    file_scope JSONB DEFAULT '[]',
    acceptance_criteria JSONB DEFAULT '[]',
    model_tier VARCHAR(20) NOT NULL DEFAULT 'auto',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    branch_name VARCHAR(255) NOT NULL DEFAULT '',
    pr_number INTEGER NOT NULL DEFAULT 0,
    pr_url TEXT NOT NULL DEFAULT '',
    worker_id VARCHAR(255),
    lease_expiry TIMESTAMPTZ,
    workstream_id UUID REFERENCES workstreams(id),
    error TEXT NOT NULL DEFAULT '',
    wave INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, plan_id)
);

-- Task conversations: LLM conversation history per task
CREATE TABLE IF NOT EXISTS task_conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id VARCHAR(255) NOT NULL,
    plan_id UUID NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    sequence INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (task_id, plan_id) REFERENCES tasks(id, plan_id) ON DELETE CASCADE
);

-- Events: durable event log for idempotent processing
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    task_id VARCHAR(255) NOT NULL DEFAULT '',
    event_type VARCHAR(100) NOT NULL,
    payload JSONB,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_plans_status ON plans(status);
CREATE INDEX IF NOT EXISTS idx_plans_repo ON plans(repository_id);
CREATE INDEX IF NOT EXISTS idx_plans_created_by ON plans(created_by_id);
CREATE INDEX IF NOT EXISTS idx_tasks_plan ON tasks(plan_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(plan_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_worker ON tasks(worker_id);
CREATE INDEX IF NOT EXISTS idx_tasks_lease ON tasks(lease_expiry) WHERE lease_expiry IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_task_conversations_task ON task_conversations(task_id, plan_id, sequence);
CREATE INDEX IF NOT EXISTS idx_events_plan ON events(plan_id, processed);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
`

const migrationAddPlanConversations = `
ALTER TABLE plans ADD COLUMN IF NOT EXISTS conversations JSONB DEFAULT '[]';
`

const migrationAddBackgroundAgents = `
-- Parent-child workstream relationship for background agent spawning
ALTER TABLE workstreams ADD COLUMN IF NOT EXISTS parent_workstream_id UUID REFERENCES workstreams(id);
CREATE INDEX IF NOT EXISTS idx_workstreams_parent ON workstreams(parent_workstream_id) WHERE parent_workstream_id IS NOT NULL;
`
