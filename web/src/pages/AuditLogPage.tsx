import { useEffect, useState } from 'react';
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

  const fetchLogs = async () => {
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
  };

  useEffect(() => {
    usersApi
      .listUsers(1)
      .then((res) => setUsersList(res.data.items || []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetchLogs();
  }, [page, filterUser, filterAction, filterDateFrom, filterDateTo]);

  const userName = (userId: string) => {
    const user = usersList.find((u) => u.id === userId);
    return user ? user.name || user.email : userId.slice(0, 8);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Audit Log</h1>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-4 mb-4">
        <select
          value={filterUser}
          onChange={(e) => {
            setFilterUser(e.target.value);
            setPage(1);
          }}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
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
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
        <input
          type="date"
          value={filterDateFrom}
          onChange={(e) => {
            setFilterDateFrom(e.target.value);
            setPage(1);
          }}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
        <span className="text-sm text-gray-500">to</span>
        <input
          type="date"
          value={filterDateTo}
          onChange={(e) => {
            setFilterDateTo(e.target.value);
            setPage(1);
          }}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>

      <div className="bg-white rounded-xl border border-gray-200">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
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
            <tbody className="divide-y divide-gray-100">
              {loading ? (
                <tr>
                  <td colSpan={5} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-600 mx-auto" />
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-8 text-center text-sm text-gray-400"
                  >
                    No audit logs found.
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr
                    key={log.id}
                    className="hover:bg-gray-50 transition-colors"
                  >
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-900">
                      {userName(log.user_id)}
                    </td>
                    <td className="px-6 py-4">
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-indigo-50 text-indigo-700">
                        {log.action}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {log.resource}
                      {log.resource_id && (
                        <span className="text-gray-400 ml-1 font-mono text-xs">
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
                          className="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-700"
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
                        <span className="text-xs text-gray-400">-</span>
                      )}
                      {expandedRow === log.id && log.details && (
                        <div className="mt-2 p-2 bg-gray-50 rounded-lg text-xs text-gray-600 font-mono whitespace-pre-wrap max-w-md">
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
