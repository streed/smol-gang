import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  workstreams as wsApi,
  repos as reposApi,
} from '../api/endpoints';
import StatusBadge from '../components/StatusBadge';
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
  const navigate = useNavigate();

  const fetchWorkstreams = async () => {
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
  };

  useEffect(() => {
    reposApi.listRepos(1).then((res) => {
      setRepositories(res.data.items || []);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    fetchWorkstreams();
  }, [page, filterRepo, filterStatus]);

  const repoName = (repoId: string) => {
    const repo = repositories.find((r) => r.id === repoId);
    return repo ? repo.name : repoId.slice(0, 8);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Workstreams</h1>
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
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
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
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
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
      <div className="bg-white rounded-xl border border-gray-200">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Repository
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
              {loading ? (
                <tr>
                  <td colSpan={5} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-600 mx-auto" />
                  </td>
                </tr>
              ) : workstreams.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-8 text-center text-sm text-gray-400"
                  >
                    No workstreams found.
                  </td>
                </tr>
              ) : (
                workstreams.map((ws) => (
                  <tr
                    key={ws.id}
                    onClick={() => navigate(`/workstreams/${ws.id}`)}
                    className="hover:bg-gray-50 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">
                      {ws.name}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {repoName(ws.repository_id)}
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
        {total > 0 && (
          <div className="px-6 border-t border-gray-100">
            <Pagination
              page={page}
              perPage={20}
              total={total}
              onPageChange={setPage}
            />
          </div>
        )}
      </div>
    </div>
  );
}
