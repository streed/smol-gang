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
	Role         string    `json:"role" db:"role"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
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

type RepoConfig struct {
	PortMappings   []PortMapping     `json:"port_mappings"`
	SetupCommands  []string          `json:"setup_commands"`
	EnvVars        map[string]string `json:"env_vars"`
	AgentPrompt    string            `json:"agent_prompt"`
	ResourceLimits *ResourceLimits   `json:"resource_limits,omitempty"`
}

type PortMapping struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Description   string `json:"description"`
}

type ResourceLimits struct {
	CPURequest    string `json:"cpu_request"`
	CPULimit      string `json:"cpu_limit"`
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
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
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
