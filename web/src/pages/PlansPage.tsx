import { useEffect, useState, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Clock, CheckCircle, XCircle, AlertTriangle, Loader } from 'lucide-react';
import { plans as plansApi, repos as reposApi } from '../api/endpoints';
import type { Plan, Repository } from '../types';
import toast from 'react-hot-toast';

const statusConfig: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
  pending_approval: { icon: <Clock className="h-3.5 w-3.5" />, color: 'text-neon-yellow', label: 'Pending Approval' },
  assessing: { icon: <Loader className="h-3.5 w-3.5 animate-spin" />, color: 'text-neon-cyan', label: 'Assessing...' },
  active: { icon: <Loader className="h-3.5 w-3.5 animate-spin" />, color: 'text-neon-green', label: 'Active' },
  complete: { icon: <CheckCircle className="h-3.5 w-3.5" />, color: 'text-neon-green', label: 'Complete' },
  halted: { icon: <AlertTriangle className="h-3.5 w-3.5" />, color: 'text-neon-red', label: 'Halted' },
  rejected: { icon: <XCircle className="h-3.5 w-3.5" />, color: 'text-gray-500', label: 'Rejected' },
};

export default function PlansPage() {
  const [plansList, setPlansList] = useState<Plan[]>([]);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ repository_id: '', prompt: '', auto_approve: false });
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    Promise.all([
      plansApi.list({ per_page: 50 }),
      reposApi.listRepos(1),
    ]).then(([plansRes, reposRes]) => {
      setPlansList(plansRes.data.items || []);
      setRepositories(reposRes.data.items || []);
    }).catch(() => toast.error('Failed to load')).finally(() => setLoading(false));
  }, []);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      const res = await plansApi.create(form);
      toast.success('Plan created');
      navigate(`/plans/${res.data.id}`);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to create plan';
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  };

  const getStatus = (status: string) => statusConfig[status] || statusConfig.pending_approval;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-neon-cyan" />
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-display font-bold text-gray-100 uppercase tracking-wider">Plans</h1>
        <button onClick={() => setShowCreate(!showCreate)} className="btn-neon-cyan flex items-center gap-2">
          <Plus className="h-4 w-4" />
          New Plan
        </button>
      </div>

      {showCreate && (
        <div className="bg-cyber-card border border-cyber-border rounded-lg p-6 mb-6">
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-sm font-mono text-gray-400 mb-1">Repository</label>
              <select
                value={form.repository_id}
                onChange={(e) => setForm({ ...form, repository_id: e.target.value })}
                required
                className="input-cyber w-full"
              >
                <option value="">Select a repository</option>
                {repositories.map((r) => (
                  <option key={r.id} value={r.id}>{r.github_owner}/{r.github_repo}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-mono text-gray-400 mb-1">What do you want to build?</label>
              <textarea
                value={form.prompt}
                onChange={(e) => setForm({ ...form, prompt: e.target.value })}
                required
                rows={4}
                className="input-cyber w-full"
                placeholder="Describe the feature, bug fix, or refactor you want..."
              />
            </div>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 text-sm text-gray-400 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.auto_approve}
                  onChange={(e) => setForm({ ...form, auto_approve: e.target.checked })}
                  className="rounded border-cyber-border"
                />
                Auto-approve (skip review)
              </label>
              <div className="flex-1" />
              <button type="submit" disabled={submitting} className="btn-neon-cyan disabled:opacity-50">
                {submitting ? 'Creating...' : 'Create Plan'}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="space-y-3">
        {plansList.length === 0 && (
          <p className="text-center text-gray-500 py-8">No plans yet. Create one to get started.</p>
        )}
        {plansList.map((plan) => {
          const st = getStatus(plan.status);
          return (
            <div
              key={plan.id}
              onClick={() => navigate(`/plans/${plan.id}`)}
              className="bg-cyber-card border border-cyber-border rounded-lg p-4 hover:border-gray-600 cursor-pointer transition-colors"
            >
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-gray-300 line-clamp-2">{plan.prompt}</p>
                  <div className="flex items-center gap-3 mt-2 text-xs text-gray-500">
                    <span className={`flex items-center gap-1 ${st.color}`}>
                      {st.icon}
                      {st.label}
                    </span>
                    <span className="font-mono">{plan.complexity}</span>
                    <span>{new Date(plan.created_at).toLocaleDateString()}</span>
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
