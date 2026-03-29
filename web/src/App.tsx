import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Toaster } from 'react-hot-toast';
import { useAuthStore } from './store/auth';
import Layout from './components/Layout';
import ProtectedRoute from './components/ProtectedRoute';
import AdminRoute from './components/AdminRoute';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import RepositoriesPage from './pages/RepositoriesPage';
import RepositoryDetailPage from './pages/RepositoryDetailPage';
import WorkstreamsPage from './pages/WorkstreamsPage';
import WorkstreamDetailPage from './pages/WorkstreamDetailPage';
import UsersPage from './pages/UsersPage';
import AuditLogPage from './pages/AuditLogPage';
import OrchestratorPage from './pages/OrchestratorPage';
import GitHubCallbackPage from './pages/GitHubCallbackPage';

function App() {
  const initialize = useAuthStore((s) => s.initialize);
  const loading = useAuthStore((s) => s.loading);

  useEffect(() => {
    initialize();
  }, [initialize]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-neon-cyan" />
      </div>
    );
  }

  return (
    <BrowserRouter>
      <Toaster position="top-right" />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/auth/github/callback" element={<GitHubCallbackPage />} />
        <Route element={<ProtectedRoute />}>
          {/* Orchestrator is full-screen — no sidebar Layout */}
          <Route path="/" element={<Navigate to="/orchestrator" replace />} />
          <Route path="/orchestrator" element={<OrchestratorPage />} />
          <Route path="/orchestrator/:planId" element={<OrchestratorPage />} />

          {/* Other pages use the sidebar Layout */}
          <Route element={<Layout />}>
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/repositories" element={<RepositoriesPage />} />
            <Route path="/repositories/:id" element={<RepositoryDetailPage />} />
            <Route path="/workstreams" element={<WorkstreamsPage />} />
            <Route path="/workstreams/:id" element={<WorkstreamDetailPage />} />
            <Route element={<AdminRoute />}>
              <Route path="/users" element={<UsersPage />} />
              <Route path="/audit-log" element={<AuditLogPage />} />
            </Route>
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
