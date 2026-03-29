import { useEffect, useState, useCallback, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Trash2 } from 'lucide-react';
import {
  workstreams as wsApi,
  repos as reposApi,
} from '../api/endpoints';
import StatusBadge from '../components/StatusBadge';
import Modal from '../components/Modal';
import Pagination from '../components/Pagination';
import type { Workstream, Repository } from '../types';
import toast from 'react-hot-toast';

const STATUS_OPTIONS = [
  'all',
  'pending',
  'provisioning',
  'running',
  'completing',
  'completed',
  'failed',
  'cancelled',
];

export default function WorkstreamsPage() {
  const [workstreams, setWorkstreams] = useState<Workstream[]>([]);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [filterRepo, setFilterRepo] = useState('');
  const [filterStatus, setFilterStatus] = useState('all');
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createForm, setCreateForm] = useState({
    repository_id: '',
    name: '',
    prompt: '',
    branch_name: '',
  });
  const [branches, setBranches] = useState<string[]>([]);
  const [branchSearch, setBranchSearch] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();

  const fetchBranches = async (repoId: string) => {
    if (!repoId) { setBranches([]); return; }
    try {
      const res = await reposApi.listBranches(repoId);
      setBranches((res.data.branches || []).sort());
    } catch {
      setBranches([]);
    }
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      const res = await wsApi.createWorkstream({
        ...createForm,
        description: createForm.prompt,
      });
      toast.success('Workstream created');
      setShowCreateModal(false);
      navigate(`/workstreams/${res.data.id}`);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to create workstream';
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  };

  const fetchWorkstreams = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { page, per_page: 20 };
      if (filterRepo) params.repository_id = filterRepo;
      if (filterStatus !== 'all') params.status = filterStatus;
      const res = await wsApi.listWorkstreams(params);
      setWorkstreams(res.data.items || []);
      setTotal(res.data.total || 0);
    } catch {
      toast.error('Failed to load workstreams');
    } finally {
      setLoading(false);
    }
  }, [page, filterRepo, filterStatus]);

  useEffect(() => {
    reposApi.listRepos(1).then((res) => {
      setRepositories(res.data.items || []);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    fetchWorkstreams();
  }, [fetchWorkstreams]);

  const repoName = (repoId: string) => {
    const repo = repositories.find((r) => r.id === repoId);
    return repo ? repo.name : repoId.slice(0, 8);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-display font-bold text-gray-100 uppercase tracking-wider">Workstreams</h1>
        <button
          onClick={() => setShowCreateModal(true)}
          disabled={repositories.length === 0}
          className="btn-neon-cyan flex items-center gap-2"
        >
          <Plus className="h-4 w-4" />
          New Workstream
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 mb-4">
        <div>
          <select
            value={filterRepo}
            onChange={(e) => {
              setFilterRepo(e.target.value);
              setPage(1);
            }}
            className="input-cyber bg-cyber-bg/80 border border-cyber-border rounded px-3 py-2 text-sm text-gray-200 font-mono focus:outline-none focus:border-neon-cyan/50 focus:shadow-neon-cyan"
          >
            <option value="">All Repositories</option>
            {repositories.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <select
            value={filterStatus}
            onChange={(e) => {
              setFilterStatus(e.target.value);
              setPage(1);
            }}
            className="input-cyber bg-cyber-bg/80 border border-cyber-border rounded px-3 py-2 text-sm text-gray-200 font-mono focus:outline-none focus:border-neon-cyan/50 focus:shadow-neon-cyan"
          >
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s === 'all' ? 'All Statuses' : s.charAt(0).toUpperCase() + s.slice(1)}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Table */}
      <div className="bg-cyber-card border border-cyber-border rounded-lg">
        <div className="overflow-x-auto">
          <table className="table-cyber w-full">
            <thead>
              <tr className="border-b border-cyber-border">
                <th className="text-left text-xs font-medium text-gray-400 uppercase tracking-wider px-6 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-medium text-gray-400 uppercase tracking-wider px-6 py-3">
                  Repository
                </th>
                <th className="text-left text-xs font-medium text-gray-400 uppercase tracking-wider px-6 py-3">
                  Branch
                </th>
                <th className="text-left text-xs font-medium text-gray-400 uppercase tracking-wider px-6 py-3">
                  Status
                </th>
                <th className="text-left text-xs font-medium text-gray-400 uppercase tracking-wider px-6 py-3">
                  Created
                </th>
                <th className="text-right text-xs font-medium text-gray-400 uppercase tracking-wider px-6 py-3">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-cyber-border">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-neon-cyan mx-auto" />
                  </td>
                </tr>
              ) : workstreams.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-8 text-center text-sm text-gray-500"
                  >
                    No workstreams found.
                  </td>
                </tr>
              ) : (
                workstreams.map((ws) => (
                  <tr
                    key={ws.id}
                    onClick={() => navigate(`/workstreams/${ws.id}`)}
                    className="hover:bg-neon-cyan/5 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-300">
                      {ws.name}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {repoName(ws.repository_id)}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {ws.branch_name}
                    </td>
                    <td className="px-6 py-4">
                      <StatusBadge status={ws.status} />
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {new Date(ws.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button
                        onClick={async (e) => {
                          e.stopPropagation();
                          try {
                            await wsApi.deleteWorkstream(ws.id);
                            setWorkstreams(prev => prev.filter(w => w.id !== ws.id));
                            toast.success('Workstream deleted');
                          } catch { toast.error('Failed to delete workstream'); }
                        }}
                        className="p-1 text-gray-600 hover:text-neon-red transition-colors"
                        title="Delete workstream"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {total > 0 && (
          <div className="px-6 border-t border-cyber-border">
            <Pagination
              page={page}
              perPage={20}
              total={total}
              onPageChange={setPage}
            />
          </div>
        )}
      </div>
      {/* Create Workstream Modal */}
      <Modal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="New Workstream"
      >
        <form onSubmit={handleCreate} className="space-y-4">
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Repository
            </label>
            <select
              value={createForm.repository_id}
              onChange={(e) => {
                setCreateForm({ ...createForm, repository_id: e.target.value, branch_name: '' });
                setBranchSearch('');
                fetchBranches(e.target.value);
              }}
              required
              className="input-cyber w-full"
            >
              <option value="">Select a repository</option>
              {repositories.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.github_owner}/{r.github_repo}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Branch <span className="text-gray-600">(optional — leave empty to create new)</span>
            </label>
            <input
              type="text"
              value={branchSearch}
              onChange={(e) => {
                setBranchSearch(e.target.value);
                setCreateForm({ ...createForm, branch_name: e.target.value });
              }}
              className="input-cyber w-full"
              placeholder="Search or type a branch name..."
              list="branch-list"
            />
            <datalist id="branch-list">
              {branches
                .filter((b) => !branchSearch || b.toLowerCase().includes(branchSearch.toLowerCase()))
                .map((b) => (
                  <option key={b} value={b} />
                ))}
            </datalist>
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Name
            </label>
            <input
              type="text"
              value={createForm.name}
              onChange={(e) =>
                setCreateForm({ ...createForm, name: e.target.value })
              }
              required
              className="input-cyber w-full"
              placeholder="fix-login-bug"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Prompt
            </label>
            <textarea
              value={createForm.prompt}
              onChange={(e) =>
                setCreateForm({ ...createForm, prompt: e.target.value })
              }
              required
              rows={4}
              className="input-cyber w-full"
              placeholder="Tell the agent what to do..."
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowCreateModal(false)}
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
