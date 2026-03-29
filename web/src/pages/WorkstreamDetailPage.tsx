import { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  GitBranch,
  ExternalLink,
  CheckCircle,
  XCircle,
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
  const [ports, setPorts] = useState<
    Array<{ name: string; url: string; port: number }>
  >([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'chat' | 'terminal' | 'agent-logs' | 'changes'>('chat');
  const [agentLogs, setAgentLogs] = useState('');
  const [diffData, setDiffData] = useState<{ stat: string; diff: string }>({ stat: '', diff: '' });
  const agentLogsRef = useRef<HTMLPreElement>(null);

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

  // Poll agent logs when the Agent Logs tab is active
  useEffect(() => {
    if (activeTab !== 'agent-logs' || !id) return;
    let cancelled = false;

    const fetchAgentLogs = async () => {
      try {
        const res = await wsApi.getLogs(id, 'agent');
        if (!cancelled) {
          setAgentLogs(res.data.logs || 'No logs available.');
          // Auto-scroll to bottom
          if (agentLogsRef.current) {
            agentLogsRef.current.scrollTop = agentLogsRef.current.scrollHeight;
          }
        }
      } catch {
        if (!cancelled) setAgentLogs('Failed to fetch agent logs.');
      }
    };

    fetchAgentLogs();
    const interval = setInterval(fetchAgentLogs, 5000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [activeTab, id]);

  // Poll diff when the Changes tab is active
  useEffect(() => {
    if (activeTab !== 'changes' || !id) return;
    let cancelled = false;
    const fetchDiff = async () => {
      try {
        const res = await wsApi.getDiff(id);
        if (!cancelled) setDiffData(res.data);
      } catch { /* ignore */ }
    };
    fetchDiff();
    const interval = setInterval(fetchDiff, 10000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [activeTab, id]);

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
              onClick={() => setActiveTab('agent-logs')}
              className={`px-4 py-2 text-sm font-medium border-b-2 ${
                activeTab === 'agent-logs'
                  ? 'border-neon-cyan text-neon-cyan'
                  : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              Agent Logs
            </button>
            <button
              onClick={() => setActiveTab('changes')}
              className={`px-4 py-2 text-sm font-medium border-b-2 ${
                activeTab === 'changes'
                  ? 'border-neon-cyan text-neon-cyan'
                  : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              Changes
            </button>
          </div>
          <div className="flex-1 min-h-0">
            {activeTab === 'chat' && <ChatInterface workstreamId={id} />}
            {activeTab === 'terminal' && <Terminal workstreamId={id!} container="app" />}
            {activeTab === 'agent-logs' && (
              <div className="h-full bg-cyber-card border border-cyber-border rounded-b-lg overflow-auto p-4">
                <pre
                  ref={agentLogsRef}
                  className="text-xs text-green-400 font-mono whitespace-pre-wrap"
                >
                  {agentLogs || 'Loading agent logs...'}
                </pre>
              </div>
            )}
            {activeTab === 'changes' && (
              <div className="h-full bg-cyber-card border border-cyber-border rounded-b-lg overflow-auto">
                {diffData.stat && (
                  <div className="border-b border-cyber-border px-4 py-3">
                    <pre className="text-xs text-gray-400 font-mono">{diffData.stat}</pre>
                  </div>
                )}
                <div className="p-4">
                  {diffData.diff ? (
                    <pre className="text-xs font-mono leading-5">
                      {diffData.diff.split('\n').map((line, i) => {
                        let cls = 'text-gray-400';
                        if (line.startsWith('+') && !line.startsWith('+++')) cls = 'text-green-400 bg-green-400/10';
                        else if (line.startsWith('-') && !line.startsWith('---')) cls = 'text-red-400 bg-red-400/10';
                        else if (line.startsWith('@@')) cls = 'text-purple-400 bg-purple-400/5';
                        else if (line.startsWith('diff --git')) cls = 'text-neon-cyan font-bold mt-4 border-t border-cyber-border pt-2';
                        else if (line.startsWith('---') || line.startsWith('+++')) cls = 'text-gray-500';
                        return <div key={i} className={`px-2 ${cls}`}>{line || ' '}</div>;
                      })}
                    </pre>
                  ) : (
                    <p className="text-gray-500 text-sm text-center py-8">No changes detected.</p>
                  )}
                </div>
              </div>
            )}
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
