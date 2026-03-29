import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Layers, GitFork, CheckCircle, Plus } from 'lucide-react';
import { workstreams as wsApi, repos as reposApi } from '../api/endpoints';
import StatusBadge from '../components/StatusBadge';
import type { Workstream, Repository } from '../types';

export default function DashboardPage() {
  const [recentWorkstreams, setRecentWorkstreams] = useState<Workstream[]>([]);
  const [totalRepos, setTotalRepos] = useState(0);
  const [activeCount, setActiveCount] = useState(0);
  const [completedToday, setCompletedToday] = useState(0);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [wsRes, repoRes] = await Promise.all([
          wsApi.listWorkstreams({ page: 1, per_page: 10 }),
          reposApi.listRepos(1),
        ]);
        const items = wsRes.data.items || [];
        setRecentWorkstreams(items);
        setTotalRepos(repoRes.data.total || 0);
        setActiveCount(
          items.filter((w: Workstream) =>
            ['running', 'provisioning', 'pending'].includes(w.status)
          ).length
        );
        const today = new Date().toISOString().slice(0, 10);
        setCompletedToday(
          items.filter(
            (w: Workstream) =>
              w.status === 'completed' &&
              w.completed_at?.slice(0, 10) === today
          ).length
        );
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const stats = [
    {
      label: 'Active Workstreams',
      value: activeCount,
      icon: Layers,
      color: 'text-neon-green',
      bg: 'bg-neon-green/10',
      border: 'border-neon-green/30',
    },
    {
      label: 'Total Repositories',
      value: totalRepos,
      icon: GitFork,
      color: 'text-neon-cyan',
      bg: 'bg-neon-cyan/10',
      border: 'border-neon-cyan/30',
    },
    {
      label: 'Completed Today',
      value: completedToday,
      icon: CheckCircle,
      color: 'text-neon-purple',
      bg: 'bg-neon-purple/10',
      border: 'border-neon-purple/30',
    },
  ];

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
        <h1 className="text-xl font-display font-bold text-gray-100 uppercase tracking-wider">
          Dashboard
        </h1>
        <button
          onClick={() => navigate('/workstreams')}
          className="btn-neon-cyan flex items-center gap-2"
        >
          <Plus className="h-4 w-4" />
          New Workstream
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        {stats.map((stat) => (
          <div
            key={stat.label}
            className={`bg-cyber-card border ${stat.border} rounded-lg p-6 flex items-center gap-4`}
          >
            <div className={`p-3 rounded-lg ${stat.bg}`}>
              <stat.icon className={`h-6 w-6 ${stat.color}`} />
            </div>
            <div>
              <p className="text-sm text-gray-500">{stat.label}</p>
              <p className={`text-2xl font-bold ${stat.color}`}>{stat.value}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Recent Workstreams */}
      <div className="bg-cyber-card border border-cyber-border rounded-lg">
        <div className="px-6 py-4 border-b border-cyber-border">
          <h2 className="text-neon-cyan font-mono text-lg font-semibold">
            Recent Workstreams
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="table-cyber w-full">
            <thead>
              <tr className="border-b border-cyber-border">
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
            <tbody className="divide-y divide-cyber-border">
              {recentWorkstreams.length === 0 ? (
                <tr>
                  <td
                    colSpan={4}
                    className="px-6 py-8 text-center text-sm text-gray-500"
                  >
                    No workstreams yet.
                  </td>
                </tr>
              ) : (
                recentWorkstreams.map((ws) => (
                  <tr
                    key={ws.id}
                    onClick={() => navigate(`/workstreams/${ws.id}`)}
                    className="hover:bg-neon-cyan/5 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-300">
                      {ws.name}
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
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
