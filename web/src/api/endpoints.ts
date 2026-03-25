import client from './client';
import type {
  User,
  Repository,
  Workstream,
  Message,
  AuditLog,
  PaginatedResponse,
  LoginResponse,
} from '../types';

// Auth
export const auth = {
  login: (email: string, password: string) =>
    client.post<LoginResponse>('/auth/login', { email, password }),

  register: (email: string, password: string, name: string, role: string) =>
    client.post<User>('/auth/register', { email, password, name, role }),

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
    client.post<Message>(`/workstreams/${id}/messages`, { content }),

  getMessages: (id: string, page: number = 1) =>
    client.get<PaginatedResponse<Message>>(`/workstreams/${id}/messages`, {
      params: { page },
    }),

  completeWorkstream: (id: string) =>
    client.post<Workstream>(`/workstreams/${id}/complete`),

  cancelWorkstream: (id: string) =>
    client.post<Workstream>(`/workstreams/${id}/cancel`),

  getLogs: (id: string) =>
    client.get<{ logs: string }>(`/workstreams/${id}/logs`),

  getPorts: (id: string) =>
    client.get<{ port_mappings: Array<{ name: string; url: string; port: number }> }>(
      `/workstreams/${id}/ports`
    ),
};

// Audit Logs
export const audit = {
  listAuditLogs: (params: Record<string, string | number> = {}) =>
    client.get<PaginatedResponse<AuditLog>>('/audit-logs', { params }),
};
