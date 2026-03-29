import client from './client';
import type {
  User,
  Repository,
  Workstream,
  Message,
  AuditLog,
  PaginatedResponse,
  GitHubRepo,
  Plan,
  PlanTask,
} from '../types';

// Auth (GitHub OAuth only)
export const auth = {
  getGitHubLoginUrl: () => client.get<{ url: string }>('/auth/github'),
  getMe: () => client.get<User>('/auth/me'),
};

// Users
export const users = {
  listUsers: (page: number = 1) =>
    client.get<PaginatedResponse<User>>('/users', { params: { page } }),

  getUser: (id: string) => client.get<User>(`/users/${id}`),

  updateUser: (id: string, data: Partial<User>) =>
    client.put<User>(`/users/${id}`, data),

  deleteUser: (id: string) => client.delete(`/users/${id}`),
};

// Repositories
export const repos = {
  listRepos: (page: number = 1) =>
    client.get<PaginatedResponse<Repository>>('/repositories', { params: { page } }),

  getRepo: (id: string) => client.get<Repository>(`/repositories/${id}`),

  createRepo: (data: Partial<Repository>) =>
    client.post<Repository>('/repositories', data),

  updateRepo: (id: string, data: Partial<Repository>) =>
    client.put<Repository>(`/repositories/${id}`, data),

  deleteRepo: (id: string) => client.delete(`/repositories/${id}`),

  listBranches: (id: string) =>
    client.get<{ branches: string[] }>(`/repositories/${id}/branches`),
};

// GitHub
export const github = {
  listRepos: (page: number = 1) =>
    client.get<GitHubRepo[]>('/github/repos', { params: { page } }),
  importRepo: (owner: string, repo: string) =>
    client.post<Repository>('/github/repos/import', { owner, repo }),
};

// Workstreams
export const workstreams = {
  listWorkstreams: (params: Record<string, string | number> = {}) =>
    client.get<PaginatedResponse<Workstream>>('/workstreams', { params }),

  getWorkstream: (id: string) =>
    client.get<Workstream>(`/workstreams/${id}`),

  createWorkstream: (data: Partial<Workstream>) =>
    client.post<Workstream>('/workstreams', data),

  sendMessage: (id: string, content: string) =>
    client.post<Message>(`/workstreams/${id}/message`, { content }),

  getMessages: (id: string, page: number = 1, perPage: number = 200) =>
    client.get<PaginatedResponse<Message>>(`/workstreams/${id}/messages`, {
      params: { page, per_page: perPage },
    }),

  completeWorkstream: (id: string) =>
    client.post<Workstream>(`/workstreams/${id}/complete`),

  cancelWorkstream: (id: string) =>
    client.post<Workstream>(`/workstreams/${id}/cancel`),

  getLogs: (id: string, container?: string) =>
    client.get<{ logs: string }>(`/workstreams/${id}/logs`, { params: container ? { container } : {} }),

  getPorts: (id: string) =>
    client.get<{ port_mappings: Array<{ name: string; url: string; port: number }> }>(
      `/workstreams/${id}/ports`
    ),

  getDiff: (id: string) =>
    client.get<{ stat: string; diff: string }>(`/workstreams/${id}/diff`),

  deleteWorkstream: (id: string) => client.delete(`/workstreams/${id}`),
};

// Audit Logs
export const audit = {
  listAuditLogs: (params: Record<string, string | number> = {}) =>
    client.get<PaginatedResponse<AuditLog>>('/audit-logs', { params }),
};

// Plans (DAG orchestrator)
export const plans = {
  list: (params: Record<string, string | number> = {}) =>
    client.get<PaginatedResponse<Plan>>('/plans', { params }),

  get: (id: string) =>
    client.get<Plan>(`/plans/${id}`),

  create: (data: { repository_id: string; prompt: string; base_branch?: string; auto_approve?: boolean }) =>
    client.post<Plan>('/plans', data),

  addTask: (planId: string, task: { id: string; description: string; depends_on: string[]; file_scope?: string[]; acceptance_criteria?: string[]; model_tier?: string }) =>
    client.post<PlanTask>(`/plans/${planId}/tasks`, task),

  updateTask: (planId: string, taskId: string, data: Partial<{ description: string; depends_on: string[]; acceptance_criteria: string[]; add_criteria: string[]; file_scope: string[]; model_tier: string }>) =>
    client.put<PlanTask>(`/plans/${planId}/tasks/${taskId}`, data),

  removeTask: (planId: string, taskId: string) =>
    client.delete(`/plans/${planId}/tasks/${taskId}`),

  approve: (planId: string) =>
    client.post<{ status: string; plan_id: string }>(`/plans/${planId}/approve`),

  reject: (planId: string) =>
    client.post(`/plans/${planId}/reject`),

  complete: (planId: string) =>
    client.post(`/plans/${planId}/complete`),

  restart: (planId: string) =>
    client.post(`/plans/${planId}/restart`),

  saveConversations: (planId: string, messages: unknown[]) =>
    client.put(`/plans/${planId}/conversations`, messages),

  delete: (id: string) => client.delete(`/plans/${id}`),
};
