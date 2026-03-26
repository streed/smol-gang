import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  GitBranch,
  ExternalLink,
  CheckCircle,
  XCircle,
  ChevronDown,
  ChevronUp,
  Globe,
} from 'lucide-react';
import { workstreams as wsApi, repos as reposApi } from '../api/endpoints';
import StatusBadge from '../components/StatusBadge';
import ChatInterface from '../components/ChatInterface';
import Terminal from '../components/Terminal';
import type { Workstream, Repository } from '../types';
import toast from 'react-hot-toast';

export default function WorkstreamDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [workstream, setWorkstream] = useState<Workstream | null>(null);
  const [repo, setRepo] = useState<Repository | null>(null);
  const [logs, setLogs] = useState('');
  const [showLogs, setShowLogs] = useState(false);
  const [ports, setPorts] = useState<
    Array<{ name: string; url: string; port: number }>
  >([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'chat' | 'terminal' | 'agent-terminal'>('chat');

  const fetchData = async () => {
    if (!id) return;
    try {
      const wsRes = await wsApi.getWorkstream(id);
      setWorkstream(wsRes.data);
      if (wsRes.data.repository_id) {
        reposApi
          .getRepo(wsRes.data.repository_id)
          .then((r) => setRepo(r.data))
          .catch(() => {});
      }
      wsApi
        .getPorts(id)
        .then((r) => setPorts(r.data.port_mappings || []))
        .catch(() => {});
    } catch {
      toast.error('Failed to load workstream');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [id]);

  const handleFetchLogs = async () => {
    if (!id) return;
    try {
      const res = await wsApi.getLogs(id);
      setLogs(res.data.logs || 'No logs available.');
    } catch {
      setLogs('Failed to fetch logs.');
    }
    setShowLogs(!showLogs);
  };

  const handleComplete = async () => {
    if (!id) return;
    try {
      const res = await wsApi.completeWorkstream(id);
      setWorkstream(res.data);
      toast.success('Workstream completed');
    } catch {
      toast.error('Failed to complete workstream');
    }
  };

  const handleCancel = async () => {
    if (!id) return;
    try {
      const res = await wsApi.cancelWorkstream(id);
      setWorkstream(res.data);
      toast.success('Workstream cancelled');
    } catch {
      toast.error('Failed to cancel workstream');
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-neon-cyan" />
      </div>
    );
  }

  if (!workstream || !id) {
    return (
      <div className="text-center py-12 text-gray-500">
        Workstream not found.
      </div>
    );
  }

  const isActive = ['running', 'provisioning', 'pending'].includes(
    workstream.status
  );

  return (
    <div>
      {/* Header */}
      <div className="flex flex-wrap items-start justify-between gap-4 mb-6">
        <div>
          <div className="flex items-center gap-3 mb-1">
            <h1 className="text-xl font-display font-bold text-gray-100">
              {workstream.name}
            </h1>
            <StatusBadge status={workstream.status} />
          </div>
          <div className="flex items-center gap-4 text-sm">
            {repo && (
              <Link
                to={`/repositories/${repo.id}`}
                className="text-neon-cyan hover:text-neon-cyan/80 transition-colors"
              >
                {repo.name}
              </Link>
            )}
            <span className="flex items-center gap-1 text-gray-400 font-mono">
              <GitBranch className="h-3.5 w-3.5" />
              {workstream.branch_name}
            </span>
            {workstream.pull_request_url && (
              <a
                href={workstream.pull_request_url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 text-neon-green hover:text-neon-green/80"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Pull Request
              </a>
            )}
          </div>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left - Chat/Terminal tabs (2/3) */}
        <div className="lg:col-span-2 h-[600px] flex flex-col">
          <div className="flex bg-cyber-card border-b border-cyber-border rounded-t-lg">
            <button
              onClick={() => setActiveTab('chat')}
              className={`px-4 py-2 text-sm font-medium border-b-2 ${
                activeTab === 'chat'
                  ? 'border-neon-cyan text-neon-cyan'
                  : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              Chat
            </button>
            <button
              onClick={() => setActiveTab('terminal')}
              className={`px-4 py-2 text-sm font-medium border-b-2 ${
                activeTab === 'terminal'
                  ? 'border-neon-cyan text-neon-cyan'
                  : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              Terminal (App)
            </button>
            <button
              onClick={() => setActiveTab('agent-terminal')}
              className={`px-4 py-2 text-sm font-medium border-b-2 ${
                activeTab === 'agent-terminal'
                  ? 'border-neon-cyan text-neon-cyan'
                  : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              Terminal (Agent)
            </button>
          </div>
          <div className="flex-1 min-h-0">
            {activeTab === 'chat' && <ChatInterface workstreamId={id} />}
            {activeTab === 'terminal' && <Terminal workstreamId={id!} container="app" />}
            {activeTab === 'agent-terminal' && <Terminal workstreamId={id!} container="agent" />}
          </div>
        </div>

        {/* Right - Actions Panel (1/3) */}
        <div className="space-y-6">
          {/* Actions */}
          {isActive && (
            <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
              <h3 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
                Actions
              </h3>
              <div className="space-y-2">
                <button
                  onClick={handleComplete}
                  className="btn-neon-green w-full flex items-center justify-center gap-2"
                >
                  <CheckCircle className="h-4 w-4" />
                  Complete
                </button>
                <button
                  onClick={handleCancel}
                  className="btn-neon-red w-full flex items-center justify-center gap-2"
                >
                  <XCircle className="h-4 w-4" />
                  Cancel
                </button>
              </div>
            </div>
          )}

          {/* Port Mappings */}
          {ports.length > 0 && (
            <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
              <h3 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
                Service URLs
              </h3>
              <div className="space-y-2">
                {ports.map((p, idx) => (
                  <a
                    key={idx}
                    href={p.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-3 py-2 bg-cyber-surface rounded-lg text-sm text-neon-cyan hover:text-neon-cyan/80 transition-colors"
                  >
                    <Globe className="h-4 w-4" />
                    <span className="font-medium">{p.name}</span>
                    <span className="text-gray-400 text-xs ml-auto">
                      :{p.port}
                    </span>
                  </a>
                ))}
              </div>
            </div>
          )}

          {/* Workstream Port Mappings */}
          {workstream.port_mappings && workstream.port_mappings.length > 0 && (
            <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
              <h3 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
                Port Mappings
              </h3>
              <div className="space-y-2">
                {workstream.port_mappings.map((pm, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between px-3 py-2 bg-cyber-surface rounded-lg text-sm"
                  >
                    <span className="text-gray-300 font-mono font-medium">{pm.name}</span>
                    <span className="text-gray-500 font-mono">
                      {pm.container_port}/{pm.protocol}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Pod Logs */}
          <div className="bg-cyber-bg border border-cyber-border rounded-lg">
            <button
              onClick={handleFetchLogs}
              className="w-full flex items-center justify-between px-6 py-4 text-sm font-mono font-semibold text-gray-300 hover:bg-cyber-hover transition-colors rounded-lg"
            >
              Pod Logs
              {showLogs ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </button>
            {showLogs && (
              <div className="px-6 pb-4">
                <pre className="bg-cyber-bg text-green-400 text-xs p-4 rounded-lg overflow-x-auto max-h-64 overflow-y-auto font-mono border border-cyber-border">
                  {logs || 'Loading...'}
                </pre>
              </div>
            )}
          </div>

          {/* Details */}
          <div className="bg-cyber-card border border-cyber-border rounded-lg p-6">
            <h3 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider mb-4">
              Details
            </h3>
            <dl className="space-y-2 text-sm">
              {workstream.pod_name && (
                <div className="flex justify-between">
                  <dt className="text-gray-500 font-mono">Pod</dt>
                  <dd className="text-gray-300 font-mono text-xs">
                    {workstream.pod_name}
                  </dd>
                </div>
              )}
              {workstream.service_name && (
                <div className="flex justify-between">
                  <dt className="text-gray-500 font-mono">Service</dt>
                  <dd className="text-gray-300 font-mono text-xs">
                    {workstream.service_name}
                  </dd>
                </div>
              )}
              <div className="flex justify-between">
                <dt className="text-gray-500 font-mono">Created</dt>
                <dd className="text-gray-300 font-mono">
                  {new Date(workstream.created_at).toLocaleString()}
                </dd>
              </div>
              {workstream.completed_at && (
                <div className="flex justify-between">
                  <dt className="text-gray-500 font-mono">Completed</dt>
                  <dd className="text-gray-300 font-mono">
                    {new Date(workstream.completed_at).toLocaleString()}
                  </dd>
                </div>
              )}
            </dl>
          </div>
        </div>
      </div>
    </div>
  );
}
