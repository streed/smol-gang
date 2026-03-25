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
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
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
          <GitFork className="h-7 w-7 text-indigo-600" />
          <div>
            <h1 className="text-2xl font-bold text-gray-900">{repo.name}</h1>
            <p className="text-sm text-gray-500">
              {repo.github_owner}/{repo.github_repo}
            </p>
          </div>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
        >
          <Plus className="h-4 w-4" />
          Start Workstream
        </button>
      </div>

      {/* Info Card */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">
            Repository Info
          </h2>
          <dl className="space-y-3">
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Git URL</dt>
              <dd className="text-sm text-gray-900 font-mono">{repo.git_url}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Default Branch</dt>
              <dd className="text-sm text-gray-900">{repo.default_branch}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Created</dt>
              <dd className="text-sm text-gray-900">
                {new Date(repo.created_at).toLocaleDateString()}
              </dd>
            </div>
          </dl>
        </div>

        {/* Config */}
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">
            Configuration
          </h2>
          {repo.config ? (
            <dl className="space-y-3">
              {repo.config.setup_commands && repo.config.setup_commands.length > 0 && (
                <div>
                  <dt className="text-sm text-gray-500 mb-1">Setup Commands</dt>
                  <dd className="text-xs font-mono bg-gray-50 p-2 rounded-lg">
                    {repo.config.setup_commands.join('\n')}
                  </dd>
                </div>
              )}
              {repo.config.agent_prompt && (
                <div>
                  <dt className="text-sm text-gray-500 mb-1">Agent Prompt</dt>
                  <dd className="text-sm text-gray-900 bg-gray-50 p-2 rounded-lg">
                    {repo.config.agent_prompt}
                  </dd>
                </div>
              )}
            </dl>
          ) : (
            <p className="text-sm text-gray-400">No configuration set.</p>
          )}
        </div>
      </div>

      {/* Port Mappings */}
      {repo.port_mappings && repo.port_mappings.length > 0 && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 mb-8">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">
            Port Mappings
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {repo.port_mappings.map((pm, idx) => (
              <div
                key={idx}
                className="bg-gray-50 rounded-lg p-3 border border-gray-100"
              >
                <p className="text-sm font-medium text-gray-900">{pm.name}</p>
                <p className="text-xs text-gray-500">
                  Port {pm.container_port} ({pm.protocol})
                </p>
                {pm.description && (
                  <p className="text-xs text-gray-400 mt-1">{pm.description}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Active Workstreams */}
      <div className="bg-white rounded-xl border border-gray-200">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-semibold text-gray-900">
            Active Workstreams ({activeWorkstreams.length})
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Branch
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Status
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {activeWorkstreams.length === 0 ? (
                <tr>
                  <td
                    colSpan={4}
                    className="px-6 py-8 text-center text-sm text-gray-400"
                  >
                    No active workstreams.
                  </td>
                </tr>
              ) : (
                activeWorkstreams.map((ws) => (
                  <tr
                    key={ws.id}
                    onClick={() => navigate(`/workstreams/${ws.id}`)}
                    className="hover:bg-gray-50 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">
                      {ws.name}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {ws.branch_name}
                    </td>
                    <td className="px-6 py-4">
                      <StatusBadge status={ws.status} />
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
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
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Name
            </label>
            <input
              type="text"
              value={wsForm.name}
              onChange={(e) =>
                setWsForm({ ...wsForm, name: e.target.value })
              }
              required
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              placeholder="fix-login-bug"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Description
            </label>
            <textarea
              value={wsForm.description}
              onChange={(e) =>
                setWsForm({ ...wsForm, description: e.target.value })
              }
              rows={3}
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              placeholder="Describe what this workstream should accomplish..."
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Branch Name
            </label>
            <input
              type="text"
              value={wsForm.branch_name}
              onChange={(e) =>
                setWsForm({ ...wsForm, branch_name: e.target.value })
              }
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              placeholder="feature/fix-login-bug"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowModal(false)}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 disabled:opacity-50"
            >
              {submitting ? 'Creating...' : 'Start Workstream'}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
