import { useEffect, useState, useCallback } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { audit as auditApi, users as usersApi } from '../api/endpoints';
import Pagination from '../components/Pagination';
import type { AuditLog, User } from '../types';
import toast from 'react-hot-toast';

export default function AuditLogPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [usersList, setUsersList] = useState<User[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [filterUser, setFilterUser] = useState('');
  const [filterAction, setFilterAction] = useState('');
  const [filterDateFrom, setFilterDateFrom] = useState('');
  const [filterDateTo, setFilterDateTo] = useState('');
  const [expandedRow, setExpandedRow] = useState<string | null>(null);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { page, per_page: 20 };
      if (filterUser) params.user_id = filterUser;
      if (filterAction) params.action = filterAction;
      if (filterDateFrom) params.date_from = filterDateFrom;
      if (filterDateTo) params.date_to = filterDateTo;
      const res = await auditApi.listAuditLogs(params);
      setLogs(res.data.items || []);
      setTotal(res.data.total || 0);
    } catch {
      toast.error('Failed to load audit logs');
    } finally {
      setLoading(false);
    }
  }, [page, filterUser, filterAction, filterDateFrom, filterDateTo]);

  useEffect(() => {
    usersApi
      .listUsers(1)
      .then((res) => setUsersList(res.data.items || []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  const userName = (userId: string) => {
    const user = usersList.find((u) => u.id === userId);
    return user ? user.name || user.email : userId.slice(0, 8);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-display font-bold text-gray-100 uppercase tracking-wider">Audit Log</h1>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-4 mb-4">
        <select
          value={filterUser}
          onChange={(e) => {
            setFilterUser(e.target.value);
            setPage(1);
          }}
          className="bg-cyber-bg/80 border border-cyber-border rounded px-3 py-2 text-sm text-gray-200 font-mono focus:outline-none focus:border-neon-cyan/50 focus:border-neon-cyan/40 transition-all"
        >
          <option value="">All Users</option>
          {usersList.map((u) => (
            <option key={u.id} value={u.id}>
              {u.name || u.email}
            </option>
          ))}
        </select>
        <input
          type="text"
          value={filterAction}
          onChange={(e) => {
            setFilterAction(e.target.value);
            setPage(1);
          }}
          placeholder="Filter by action..."
          className="bg-cyber-bg/80 border border-cyber-border rounded px-3 py-2 text-sm text-gray-200 font-mono focus:outline-none focus:border-neon-cyan/50 focus:border-neon-cyan/40 transition-all"
        />
        <input
          type="date"
          value={filterDateFrom}
          onChange={(e) => {
            setFilterDateFrom(e.target.value);
            setPage(1);
          }}
          className="bg-cyber-bg/80 border border-cyber-border rounded px-3 py-2 text-sm text-gray-200 font-mono focus:outline-none focus:border-neon-cyan/50 focus:border-neon-cyan/40 transition-all [color-scheme:dark]"
        />
        <span className="text-gray-500 font-mono">to</span>
        <input
          type="date"
          value={filterDateTo}
          onChange={(e) => {
            setFilterDateTo(e.target.value);
            setPage(1);
          }}
          className="bg-cyber-bg/80 border border-cyber-border rounded px-3 py-2 text-sm text-gray-200 font-mono focus:outline-none focus:border-neon-cyan/50 focus:border-neon-cyan/40 transition-all [color-scheme:dark]"
        />
      </div>

      <div className="bg-cyber-card border border-cyber-border rounded-lg">
        <div className="overflow-x-auto">
          <table className="table-cyber w-full">
            <thead>
              <tr className="border-b border-cyber-border">
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Timestamp
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  User
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Action
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Resource
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Details
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-cyber-border">
              {loading ? (
                <tr>
                  <td colSpan={5} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-neon-cyan mx-auto" />
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-8 text-center text-sm text-gray-500"
                  >
                    No audit logs found.
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr
                    key={log.id}
                    className="hover:bg-neon-cyan/5 transition-colors"
                  >
                    <td className="px-6 py-4 text-gray-400 font-mono text-xs">
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-300">
                      {userName(log.user_id)}
                    </td>
                    <td className="px-6 py-4">
                      <span className="inline-flex items-center bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30 px-2 py-0.5 rounded text-xs font-mono">
                        {log.action}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {log.resource}
                      {log.resource_id && (
                        <span className="text-gray-600 font-mono text-xs ml-1">
                          ({log.resource_id.slice(0, 8)})
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      {log.details ? (
                        <button
                          onClick={() =>
                            setExpandedRow(
                              expandedRow === log.id ? null : log.id
                            )
                          }
                          className="flex items-center gap-1 text-xs text-neon-cyan hover:text-neon-cyan/80 font-mono"
                        >
                          {expandedRow === log.id ? (
                            <>
                              Hide <ChevronUp className="h-3 w-3" />
                            </>
                          ) : (
                            <>
                              Show <ChevronDown className="h-3 w-3" />
                            </>
                          )}
                        </button>
                      ) : (
                        <span className="text-xs text-gray-500">-</span>
                      )}
                      {expandedRow === log.id && log.details && (
                        <div className="mt-2 bg-cyber-bg border border-cyber-border rounded p-2 text-xs text-gray-400 font-mono whitespace-pre-wrap max-w-md">
                          {log.details}
                        </div>
                      )}
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
    </div>
  );
}
