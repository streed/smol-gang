import { create } from 'zustand';
import { auth as authApi } from '../api/endpoints';
import type { User } from '../types';

interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
  loginWithToken: (token: string) => Promise<void>;
  logout: () => void;
  initialize: () => void;
  isAuthenticated: () => boolean;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  token: null,
  loading: true,

  loginWithToken: async (token: string) => {
    localStorage.setItem('token', token);
    set({ token });
    try {
      const response = await authApi.getMe();
      const user = response.data;
      localStorage.setItem('user', JSON.stringify(user));
      set({ user });
    } catch {
      localStorage.removeItem('token');
      set({ token: null, user: null });
      throw new Error('Failed to authenticate');
    }
  },

  logout: () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    set({ token: null, user: null });
    window.location.href = '/login';
  },

  initialize: () => {
    const token = localStorage.getItem('token');
    const userStr = localStorage.getItem('user');
    let user: User | null = null;
    if (userStr) {
      try {
        user = JSON.parse(userStr);
      } catch {
        user = null;
      }
    }
    set({ token, user, loading: false });
  },

  isAuthenticated: () => {
    const state = get();
    return !!state.token && !!state.user;
  },
}));
