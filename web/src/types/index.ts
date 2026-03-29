export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  github_id?: number;
  github_login?: string;
  created_at: string;
  updated_at: string;
}

export interface PortMapping {
  name: string;
  container_port: number;
  protocol: string;
  description: string;
}

export interface ServiceDef {
  name: string;
  command: string;
  port: number;
}

export interface ComposeDef {
  enabled: boolean;
  file?: string;
  ports?: ComposePort[];
}

export interface ComposePort {
  service: string;
  port: number;
}

export interface RepoConfig {
  port_mappings: PortMapping[];
  setup_commands: string[];
  env_vars: Record<string, string>;
  agent_prompt: string;
  resource_limits: Record<string, string>;
  environment?: string;
  services?: ServiceDef[];
  compose?: ComposeDef;
}

export interface Repository {
  id: string;
  name: string;
  git_url: string;
  default_branch: string;
  github_owner: string;
  github_repo: string;
  config: RepoConfig;
  port_mappings: PortMapping[];
  created_at: string;
}

export interface LLMConfig {
  api_url: string;
  api_key: string;
  model: string;
}

export interface Workstream {
  id: string;
  name: string;
  description: string;
  repository_id: string;
  branch_name: string;
  status: string;
  pod_name: string;
  service_name: string;
  port_mappings: PortMapping[];
  pull_request_url: string;
  llm_config: LLMConfig;
  created_at: string;
  completed_at: string;
}

export interface Message {
  id: string;
  workstream_id: string;
  user_id: string;
  source: string;
  content: string;
  created_at: string;
}

export interface AuditLog {
  id: string;
  user_id: string;
  workstream_id: string;
  action: string;
  resource: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface GitHubRepo {
  full_name: string;
  name: string;
  owner: string;
  clone_url: string;
  default_branch: string;
  private: boolean;
  description: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

// --- DAG Plan Types ---

export interface Plan {
  id: string;
  prompt: string;
  plan_json?: ExecutionPlan;
  root_branch: string;
  root_pr: number;
  status: string;
  repository_id: string;
  base_branch: string;
  complexity: string;
  complexity_reasoning: string;
  created_by_id: string;
  conversations?: unknown[];
  created_at: string;
  updated_at: string;
  tasks?: PlanTask[];
}

export interface PlanTask {
  id: string;
  plan_id: string;
  description: string;
  depends_on: string[];
  file_scope: string[];
  acceptance_criteria: string[];
  model_tier: string;
  status: string;
  branch_name: string;
  pr_number: number;
  pr_url: string;
  worker_id?: string;
  workstream_id?: string;
  error?: string;
  wave: number;
  created_at: string;
  updated_at: string;
}

export interface ExecutionPlan {
  tasks: TaskNode[];
  waves: string[][];
  critical_path: string[];
}

export interface TaskNode {
  id: string;
  description: string;
  depends_on: string[];
  file_scope: string[];
  acceptance_criteria: string[];
  model_tier: string;
}
