export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  created_at: string;
  updated_at: string;
}

export interface PortMapping {
  name: string;
  container_port: number;
  protocol: string;
  description: string;
}

export interface RepoConfig {
  port_mappings: PortMapping[];
  setup_commands: string[];
  env_vars: Record<string, string>;
  agent_prompt: string;
  resource_limits: Record<string, string>;
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

export interface LoginResponse {
  token: string;
  user: User;
}
