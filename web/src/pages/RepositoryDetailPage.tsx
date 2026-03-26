import { useEffect, useState, FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { GitFork, Plus, ExternalLink } from 'lucide-react';
import {
  repos as reposApi,
  workstreams as wsApi,
} from '../api/endpoints';
import StatusBadge from '../components/StatusBadge';
import Modal from '../components/Modal';
import type { Repository, Workstream } from '../types';
import toast from 'react-hot-toast';

export default function RepositoryDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [repo, setRepo] = useState<Repository | null>(null);
  const [workstreams, setWorkstreams] = useState<Workstream[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [wsForm, setWsForm] = useState({
    name: '',
    description: '',
    branch_name: '',
  });
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    const fetchData = async () => {
      if (!id) return;
      try {
        const [repoRes, wsRes] = await Promise.all([
          reposApi.getRepo(id),
          wsApi.listWorkstreams({ repository_id: id }),
        ]);
        setRepo(repoRes.data);
        setWorkstreams(wsRes.data.items || []);
      } catch {
        toast.error('Failed to load repository');
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [id]);

  const handleCreateWorkstream = async (e: FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setSubmitting(true);
    try {
      const res = await wsApi.createWorkstream({
        ...wsForm,
        repository_id: id,
      });
      toast.success('Workstream created');
      navigate(`/workstreams/${res.data.id}`);
    } catch {
      toast.error('Failed to create workstream');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-neon-cyan" />
      </div>
    );
  }

  if (!repo) {
    return (
      <div className="text-center py-12 text-gray-500">
        Repository not found.
      </div>
    );
  }

  const activeWorkstreams = workstreams.filter((w) =>
    ['running', 'provisioning', 'pending'].includes(w.status)
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <GitFork className="h-7 w-7 text-neon-cyan" />
          <div>
            <h1 className="text-xl font-display font-bold text-gray-100">{repo.name}</h1>
            <p className="text-sm font-mono text-gray-400">
              {repo.github_owner}/{repo.github_repo}
            </p>
          </div>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="btn-neon-cyan flex items-center gap-2"
        >
          <Plus className="h-4 w-4" />
          Start Workstream
        </button>
      </div>

      {/* Info Card */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
          <h2 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
            Repository Info
          </h2>
          <dl className="space-y-3">
            <div className="flex justify-between">
              <dt className="text-sm font-mono text-gray-500">Git URL</dt>
              <dd className="text-sm text-gray-300 font-mono">{repo.git_url}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm font-mono text-gray-500">Default Branch</dt>
              <dd className="text-sm text-gray-300 font-mono">{repo.default_branch}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm font-mono text-gray-500">Created</dt>
              <dd className="text-sm text-gray-300 font-mono">
                {new Date(repo.created_at).toLocaleDateString()}
              </dd>
            </div>
          </dl>
        </div>

        {/* Config */}
        <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
          <h2 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
            Configuration
          </h2>
          {repo.config ? (
            <dl className="space-y-3">
              {repo.config.setup_commands && repo.config.setup_commands.length > 0 && (
                <div>
                  <dt className="text-sm font-mono text-gray-500 mb-1">Setup Commands</dt>
                  <dd className="bg-cyber-bg text-neon-green text-xs font-mono p-2 rounded">
                    {repo.config.setup_commands.join('\n')}
                  </dd>
                </div>
              )}
              {repo.config.agent_prompt && (
                <div>
                  <dt className="text-sm font-mono text-gray-500 mb-1">Agent Prompt</dt>
                  <dd className="bg-cyber-bg text-neon-green text-xs font-mono p-2 rounded">
                    {repo.config.agent_prompt}
                  </dd>
                </div>
              )}
            </dl>
          ) : (
            <p className="text-sm text-gray-500">No configuration set.</p>
          )}
        </div>
      </div>

      {/* Port Mappings */}
      {repo.port_mappings && repo.port_mappings.length > 0 && (
        <div className="bg-cyber-card border border-cyber-border rounded-lg p-6 mb-8">
          <h2 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
            Port Mappings
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {repo.port_mappings.map((pm, idx) => (
              <div
                key={idx}
                className="bg-cyber-surface rounded-lg p-3 border border-cyber-border"
              >
                <p className="text-sm font-medium text-gray-300">{pm.name}</p>
                <p className="text-xs text-gray-500">
                  Port {pm.container_port} ({pm.protocol})
                </p>
                {pm.description && (
                  <p className="text-xs text-gray-500 mt-1">{pm.description}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Active Workstreams */}
      <div className="bg-cyber-card border border-cyber-border rounded-lg">
        <div className="px-6 py-4 border-b border-cyber-border">
          <h2 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider">
            Active Workstreams ({activeWorkstreams.length})
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="table-cyber w-full">
            <thead>
              <tr className="border-b border-cyber-border">
                <th className="text-left text-xs font-mono font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-mono font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Branch
                </th>
                <th className="text-left text-xs font-mono font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Status
                </th>
                <th className="text-left text-xs font-mono font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-cyber-border">
              {activeWorkstreams.length === 0 ? (
                <tr>
                  <td
                    colSpan={4}
                    className="px-6 py-8 text-center text-sm text-gray-500"
                  >
                    No active workstreams.
                  </td>
                </tr>
              ) : (
                activeWorkstreams.map((ws) => (
                  <tr
                    key={ws.id}
                    onClick={() => navigate(`/workstreams/${ws.id}`)}
                    className="hover:bg-neon-cyan/5 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-300">
                      {ws.name}
                    </td>
                    <td className="px-6 py-4 text-sm font-mono text-gray-500">
                      {ws.branch_name}
                    </td>
                    <td className="px-6 py-4">
                      <StatusBadge status={ws.status} />
                    </td>
                    <td className="px-6 py-4 text-sm font-mono text-gray-500">
                      {new Date(ws.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Create Workstream Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title="Start Workstream"
      >
        <form onSubmit={handleCreateWorkstream} className="space-y-4">
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Name
            </label>
            <input
              type="text"
              value={wsForm.name}
              onChange={(e) =>
                setWsForm({ ...wsForm, name: e.target.value })
              }
              required
              className="input-cyber w-full"
              placeholder="fix-login-bug"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Description
            </label>
            <textarea
              value={wsForm.description}
              onChange={(e) =>
                setWsForm({ ...wsForm, description: e.target.value })
              }
              rows={3}
              className="input-cyber w-full"
              placeholder="Describe what this workstream should accomplish..."
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Branch Name
            </label>
            <input
              type="text"
              value={wsForm.branch_name}
              onChange={(e) =>
                setWsForm({ ...wsForm, branch_name: e.target.value })
              }
              className="input-cyber w-full"
              placeholder="feature/fix-login-bug"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowModal(false)}
              className="btn-cyber text-gray-400 border-cyber-border hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="btn-neon-cyan disabled:opacity-50"
            >
              {submitting ? 'Creating...' : 'Start Workstream'}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
