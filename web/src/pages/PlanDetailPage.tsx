import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Play, XCircle, Trash2, ExternalLink } from 'lucide-react';
import { plans as plansApi } from '../api/endpoints';
import type { Plan } from '../types';
import toast from 'react-hot-toast';

const taskStatusColors: Record<string, string> = {
  pending: 'border-gray-600 bg-gray-600/10 text-gray-400',
  ready: 'border-neon-yellow bg-neon-yellow/10 text-neon-yellow',
  claimed: 'border-neon-cyan bg-neon-cyan/10 text-neon-cyan',
  working: 'border-neon-cyan bg-neon-cyan/10 text-neon-cyan animate-pulse',
  pr_open: 'border-neon-purple bg-neon-purple/10 text-neon-purple',
  in_review: 'border-neon-purple bg-neon-purple/10 text-neon-purple',
  changes_requested: 'border-neon-orange bg-neon-orange/10 text-neon-orange',
  merged: 'border-neon-green bg-neon-green/10 text-neon-green',
  failed: 'border-neon-red bg-neon-red/10 text-neon-red',
  cancelled: 'border-gray-600 bg-gray-600/10 text-gray-500',
};

export default function PlanDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [plan, setPlan] = useState<Plan | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchPlan = async () => {
    if (!id) return;
    try {
      const res = await plansApi.get(id);
      setPlan(res.data);
    } catch {
      toast.error('Failed to load plan');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPlan();
    // Poll for updates every 5 seconds
    const interval = setInterval(fetchPlan, 5000);
    return () => clearInterval(interval);
  }, [id]);

  const handleApprove = async () => {
    if (!id) return;
    try {
      await plansApi.approve(id);
      toast.success('Plan approved — execution started');
      fetchPlan();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to approve';
      toast.error(msg);
    }
  };

  const handleReject = async () => {
    if (!id) return;
    try {
      await plansApi.reject(id);
      toast.success('Plan rejected');
      fetchPlan();
    } catch {
      toast.error('Failed to reject');
    }
  };

  const handleRemoveTask = async (taskId: string) => {
    if (!id) return;
    try {
      await plansApi.removeTask(id, taskId);
      toast.success('Task removed');
      fetchPlan();
    } catch {
      toast.error('Failed to remove task');
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-neon-cyan" />
      </div>
    );
  }

  if (!plan) {
    return <div className="text-center py-12 text-gray-500">Plan not found.</div>;
  }

  const tasks = plan.tasks || [];
  const waves = plan.plan_json?.waves || [];
  const isPending = plan.status === 'pending_approval';
  const isAssessing = plan.complexity === 'assessing';

  // Group tasks by wave for DAG visualization
  const taskMap = new Map(tasks.map(t => [t.id, t]));

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-2">
          <h1 className="text-xl font-display font-bold text-gray-100">Plan</h1>
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
            plan.status === 'active' ? 'bg-neon-green/10 text-neon-green border border-neon-green/30' :
            plan.status === 'complete' ? 'bg-neon-green/10 text-neon-green border border-neon-green/30' :
            plan.status === 'pending_approval' ? 'bg-neon-yellow/10 text-neon-yellow border border-neon-yellow/30' :
            plan.status === 'halted' ? 'bg-neon-red/10 text-neon-red border border-neon-red/30' :
            'bg-gray-500/10 text-gray-400 border border-gray-500/30'
          }`}>
            {plan.status.replace('_', ' ')}
          </span>
          <span className="text-xs text-gray-500 font-mono">{plan.complexity}</span>
        </div>
        <p className="text-sm text-gray-400 max-w-3xl">{plan.prompt}</p>
        {plan.complexity_reasoning && (
          <p className="text-xs text-gray-600 mt-1 italic">{plan.complexity_reasoning}</p>
        )}
      </div>

      {/* Actions */}
      {isPending && !isAssessing && (
        <div className="flex items-center gap-3 mb-6">
          <button onClick={handleApprove} className="btn-neon-green flex items-center gap-2">
            <Play className="h-4 w-4" />
            Approve & Start
          </button>
          <button onClick={handleReject} className="btn-neon-red flex items-center gap-2">
            <XCircle className="h-4 w-4" />
            Reject
          </button>
        </div>
      )}

      {isAssessing && (
        <div className="bg-cyber-card border border-neon-cyan/30 rounded-lg p-4 mb-6 flex items-center gap-3">
          <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-neon-cyan" />
          <span className="text-sm text-neon-cyan">Analyzing task complexity and decomposing into subtasks...</span>
        </div>
      )}

      {/* DAG Visualization */}
      {tasks.length > 0 && (
        <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
          <h2 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
            Task Graph ({tasks.length} tasks, {waves.length} waves)
          </h2>

          {/* Render waves horizontally */}
          <div className="space-y-6">
            {waves.map((wave, waveIdx) => (
              <div key={waveIdx}>
                <div className="text-xs text-gray-600 font-mono mb-2">Wave {waveIdx}</div>
                <div className="flex flex-wrap gap-3">
                  {wave.map((taskId) => {
                    const task = taskMap.get(taskId);
                    if (!task) return null;
                    const colors = taskStatusColors[task.status] || taskStatusColors.pending;

                    return (
                      <div
                        key={taskId}
                        className={`border rounded-lg p-3 min-w-[250px] max-w-[400px] ${colors}`}
                      >
                        <div className="flex items-center justify-between mb-1">
                          <span className="font-mono font-bold text-sm">{task.id}</span>
                          <div className="flex items-center gap-1">
                            <span className="text-xs capitalize">{task.status.replace('_', ' ')}</span>
                            {isPending && (
                              <button
                                onClick={(e) => { e.stopPropagation(); handleRemoveTask(task.id); }}
                                className="p-0.5 hover:text-neon-red transition-colors ml-1"
                              >
                                <Trash2 className="h-3 w-3" />
                              </button>
                            )}
                          </div>
                        </div>
                        <p className="text-xs opacity-80 line-clamp-2">{task.description}</p>
                        {task.depends_on.length > 0 && (
                          <div className="mt-1.5 flex flex-wrap gap-1">
                            {task.depends_on.map((dep) => (
                              <span key={dep} className="text-xs bg-black/20 px-1.5 py-0.5 rounded font-mono">
                                &larr; {dep}
                              </span>
                            ))}
                          </div>
                        )}
                        {task.pr_url && (
                          <a
                            href={task.pr_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="flex items-center gap-1 text-xs mt-1.5 hover:underline"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <ExternalLink className="h-3 w-3" />
                            PR
                          </a>
                        )}
                        {task.workstream_id && (
                          <button
                            onClick={(e) => { e.stopPropagation(); navigate(`/workstreams/${task.workstream_id}`); }}
                            className="text-xs mt-1.5 hover:underline"
                          >
                            View Workstream &rarr;
                          </button>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>

          {/* Critical Path */}
          {plan.plan_json?.critical_path && plan.plan_json.critical_path.length > 1 && (
            <div className="mt-4 pt-4 border-t border-cyber-border">
              <span className="text-xs text-gray-500 font-mono">Critical path: </span>
              <span className="text-xs text-neon-cyan font-mono">
                {plan.plan_json.critical_path.join(' \u2192 ')}
              </span>
            </div>
          )}
        </div>
      )}

      {tasks.length === 0 && !isAssessing && (
        <div className="bg-cyber-card border border-cyber-border rounded-lg p-8 text-center text-gray-500">
          No tasks yet. The plan is being processed.
        </div>
      )}
    </div>
  );
}
