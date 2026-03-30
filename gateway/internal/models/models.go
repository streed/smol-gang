package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Domain Models ---

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Name         string    `json:"name" db:"name"`
	Role              string    `json:"role" db:"role"`
	GitHubID          *int64    `json:"github_id,omitempty" db:"github_id"`
	GitHubLogin       string    `json:"github_login,omitempty" db:"github_login"`
	GitHubAccessToken string    `json:"-" db:"github_access_token"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Repository struct {
	ID                   uuid.UUID   `json:"id" db:"id"`
	Name                 string      `json:"name" db:"name"`
	GitURL               string      `json:"git_url" db:"git_url"`
	DefaultBranch        string      `json:"default_branch" db:"default_branch"`
	GitHubOwner          string      `json:"github_owner" db:"github_owner"`
	GitHubRepo           string      `json:"github_repo" db:"github_repo"`
	GitHubInstallationID int64       `json:"github_installation_id" db:"github_installation_id"`
	PortMappings         []PortMapping `json:"port_mappings" db:"-"`
	Config               *RepoConfig `json:"config,omitempty" db:"config"`
	CreatedByID          uuid.UUID   `json:"created_by_id" db:"created_by_id"`
	CreatedAt            time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at" db:"updated_at"`
}

type ServiceDef struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Port    int    `json:"port"`
}

type ComposeDef struct {
	Enabled bool          `json:"enabled"`
	File    string        `json:"file,omitempty"`
	Ports   []ComposePort `json:"ports,omitempty"`
}

type ComposePort struct {
	Service string `json:"service"`
	Port    int    `json:"port"`
}

type RepoConfig struct {
	PortMappings   []PortMapping     `json:"port_mappings"`
	SetupCommands  []string          `json:"setup_commands"`
	EnvVars        map[string]string `json:"env_vars"`
	AgentPrompt    string            `json:"agent_prompt"`
	ResourceLimits *ResourceLimits   `json:"resource_limits,omitempty"`
	Environment    string            `json:"environment,omitempty"`
	Services       []ServiceDef      `json:"services,omitempty"`
	Compose        *ComposeDef       `json:"compose,omitempty"`
}

// DerivePortMappings produces port mappings from services or compose config,
// falling back to the explicit port_mappings field.
func (c *RepoConfig) DerivePortMappings() []PortMapping {
	if c == nil {
		return nil
	}
	if len(c.Services) > 0 {
		var pms []PortMapping
		for _, s := range c.Services {
			pms = append(pms, PortMapping{
				Name:          s.Name,
				ContainerPort: s.Port,
				Protocol:      "HTTP",
				Description:   s.Command,
			})
		}
		return pms
	}
	if c.Compose != nil && c.Compose.Enabled && len(c.Compose.Ports) > 0 {
		var pms []PortMapping
		for _, cp := range c.Compose.Ports {
			pms = append(pms, PortMapping{
				Name:          cp.Service,
				ContainerPort: cp.Port,
				Protocol:      "HTTP",
			})
		}
		return pms
	}
	return c.PortMappings
}

var EnvironmentImages = map[string]string{
	"node-22":     "smol-gang/env-node:22",
	"node-20":     "smol-gang/env-node:20",
	"python-3.12": "smol-gang/env-python:3.12",
	"python-3.11": "smol-gang/env-python:3.11",
	"rust":        "smol-gang/env-rust:latest",
	"go-1.23":     "smol-gang/env-go:1.23",
	"full":        "smol-gang/env-full:latest",
}

type PortMapping struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Description   string `json:"description"`
}

type ResourceLimits struct {
	CPURequest    string `json:"cpu_request"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
	DiskSize      string `json:"disk_size"`
}

type Workstream struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	Name           string        `json:"name" db:"name"`
	Description    string        `json:"description" db:"description"`
	RepositoryID   uuid.UUID     `json:"repository_id" db:"repository_id"`
	BranchName     string        `json:"branch_name" db:"branch_name"`
	Status         string        `json:"status" db:"status"`
	PodName        string        `json:"pod_name" db:"pod_name"`
	ServiceName    string        `json:"service_name" db:"service_name"`
	PortMappings   []PortMapping `json:"port_mappings" db:"-"`
	PullRequestURL string        `json:"pull_request_url,omitempty" db:"pull_request_url"`
	LLMConfig      *LLMConfig    `json:"llm_config,omitempty" db:"llm_config"`
	CreatedByID    uuid.UUID     `json:"created_by_id" db:"created_by_id"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
	CompletedAt    *time.Time    `json:"completed_at,omitempty" db:"completed_at"`
}

type LLMConfig struct {
	APIURL   string `json:"api_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type AuditLog struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	UserID       *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	WorkstreamID *uuid.UUID      `json:"workstream_id,omitempty" db:"workstream_id"`
	Action       string          `json:"action" db:"action"`
	Resource     string          `json:"resource" db:"resource"`
	ResourceID   string          `json:"resource_id" db:"resource_id"`
	Details      json.RawMessage `json:"details,omitempty" db:"details"`
	IPAddress    string          `json:"ip_address" db:"ip_address"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

type Message struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	WorkstreamID uuid.UUID  `json:"workstream_id" db:"workstream_id"`
	UserID       *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Source       string     `json:"source" db:"source"`
	Content      string     `json:"content" db:"content"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// --- Request Types ---

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type UpdateUserRequest struct {
	Name *string `json:"name,omitempty"`
	Role *string `json:"role,omitempty"`
}

type ImportGitHubRepoRequest struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type CreateRepositoryRequest struct {
	Name                 string      `json:"name"`
	GitURL               string      `json:"git_url"`
	DefaultBranch        string      `json:"default_branch"`
	GitHubOwner          string      `json:"github_owner"`
	GitHubRepo           string      `json:"github_repo"`
	GitHubInstallationID int64       `json:"github_installation_id"`
	Config               *RepoConfig `json:"config,omitempty"`
}

type UpdateRepositoryRequest struct {
	Name          *string     `json:"name,omitempty"`
	DefaultBranch *string     `json:"default_branch,omitempty"`
	Config        *RepoConfig `json:"config,omitempty"`
}

type CreateWorkstreamRequest struct {
	RepositoryID uuid.UUID     `json:"repository_id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Prompt       string        `json:"prompt"`
	BranchName   string        `json:"branch_name,omitempty"`
	PortMappings []PortMapping `json:"port_mappings,omitempty"`
	LLMConfig    *LLMConfig    `json:"llm_config,omitempty"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
}

// --- Response Types ---

type PaginatedResponse struct {
	Items   interface{} `json:"items"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// --- DAG Orchestrator Models ---

// Plan status constants
const (
	PlanStatusPendingApproval = "pending_approval"
	PlanStatusActive         = "active"
	PlanStatusHalted         = "halted"
	PlanStatusComplete       = "complete"
	PlanStatusRejected       = "rejected"
)

// Task status constants
const (
	TaskStatusPending          = "pending"
	TaskStatusReady            = "ready"
	TaskStatusClaimed          = "claimed"
	TaskStatusWorking          = "working"
	TaskStatusPROpen           = "pr_open"
	TaskStatusInReview         = "in_review"
	TaskStatusChangesRequested = "changes_requested"
	TaskStatusMerged           = "merged"
	TaskStatusFailed           = "failed"
	TaskStatusCancelled        = "cancelled"
)

type Plan struct {
	ID                  uuid.UUID       `json:"id" db:"id"`
	Prompt              string          `json:"prompt" db:"prompt"`
	PlanJSON            json.RawMessage `json:"plan_json,omitempty" db:"plan_json"`
	RootBranch          string          `json:"root_branch" db:"root_branch"`
	RootPR              int             `json:"root_pr" db:"root_pr"`
	Status              string          `json:"status" db:"status"`
	RepositoryID        uuid.UUID       `json:"repository_id" db:"repository_id"`
	BaseBranch          string          `json:"base_branch" db:"base_branch"`
	Complexity          string          `json:"complexity" db:"complexity"`
	ComplexityReasoning string          `json:"complexity_reasoning" db:"complexity_reasoning"`
	CreatedByID         uuid.UUID       `json:"created_by_id" db:"created_by_id"`
	Conversations       json.RawMessage `json:"conversations,omitempty" db:"conversations"`
	CreatedAt           time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`
	Tasks               []Task          `json:"tasks,omitempty" db:"-"`
}

type Task struct {
	ID                 string          `json:"id" db:"id"`
	PlanID             uuid.UUID       `json:"plan_id" db:"plan_id"`
	Description        string          `json:"description" db:"description"`
	DependsOn          []string        `json:"depends_on" db:"-"`
	DependsOnJSON      json.RawMessage `json:"-" db:"depends_on"`
	FileScope          []string        `json:"file_scope" db:"-"`
	FileScopeJSON      json.RawMessage `json:"-" db:"file_scope"`
	AcceptanceCriteria []string        `json:"acceptance_criteria" db:"-"`
	AcceptCriteriaJSON json.RawMessage `json:"-" db:"acceptance_criteria"`
	ModelTier          string          `json:"model_tier" db:"model_tier"`
	Status             string          `json:"status" db:"status"`
	BranchName         string          `json:"branch_name" db:"branch_name"`
	PRNumber           int             `json:"pr_number" db:"pr_number"`
	PRURL              string          `json:"pr_url" db:"pr_url"`
	WorkerID           *string         `json:"worker_id,omitempty" db:"worker_id"`
	LeaseExpiry        *time.Time      `json:"lease_expiry,omitempty" db:"lease_expiry"`
	WorkstreamID       *uuid.UUID      `json:"workstream_id,omitempty" db:"workstream_id"`
	Error              string          `json:"error,omitempty" db:"error"`
	Wave               int             `json:"wave" db:"wave"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
}

type TaskConversation struct {
	ID        uuid.UUID `json:"id" db:"id"`
	TaskID    string    `json:"task_id" db:"task_id"`
	PlanID    uuid.UUID `json:"plan_id" db:"plan_id"`
	Role      string    `json:"role" db:"role"`
	Content   string    `json:"content" db:"content"`
	Sequence  int       `json:"sequence" db:"sequence"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Event struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	PlanID    uuid.UUID       `json:"plan_id" db:"plan_id"`
	TaskID    string          `json:"task_id" db:"task_id"`
	EventType string          `json:"event_type" db:"event_type"`
	Payload   json.RawMessage `json:"payload,omitempty" db:"payload"`
	Processed bool            `json:"processed" db:"processed"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// --- DAG Request Types ---

type CreatePlanRequest struct {
	RepositoryID uuid.UUID `json:"repository_id"`
	Prompt       string    `json:"prompt"`
	BaseBranch   string    `json:"base_branch,omitempty"`
	AutoApprove  bool      `json:"auto_approve,omitempty"`
}

type UpdateTaskRequest struct {
	Description        *string  `json:"description,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	AddCriteria        []string `json:"add_criteria,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	FileScope          []string `json:"file_scope,omitempty"`
	ModelTier          *string  `json:"model_tier,omitempty"`
}

type AddTaskRequest struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	DependsOn          []string `json:"depends_on"`
	FileScope          []string `json:"file_scope,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	ModelTier          string   `json:"model_tier,omitempty"`
}

// ComplexityVerdict from the Complexity Assessor
type ComplexityVerdict struct {
	Complexity string  `json:"complexity"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
	EstTasks   int     `json:"est_tasks"`
	Suggestion string  `json:"suggestion"`
}

// TaskNode is the decomposer's output format
type TaskNode struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	DependsOn          []string `json:"depends_on"`
	FileScope          []string `json:"file_scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ModelTier          string   `json:"model_tier"`
}

// ExecutionPlan is the scheduler's output
type ExecutionPlan struct {
	Tasks        []TaskNode `json:"tasks"`
	Waves        [][]string `json:"waves"`
	CriticalPath []string   `json:"critical_path"`
}
