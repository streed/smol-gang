import { useEffect, useState, useRef, useCallback, FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Send, Plus, Cpu, Settings, LogOut, GitFork, MessageSquare, FileCode, Trash2, Layers, ChevronDown, ChevronUp } from 'lucide-react';
import { useAuthStore } from '../store/auth';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import ReactFlow, {
  Node,
  Edge,
  Background,
  Controls,
  MiniMap,
  Handle,
  MarkerType,
  useNodesState,
  useEdgesState,
  Position,
} from 'reactflow';
import 'reactflow/dist/style.css';
import { plans as plansApi, repos as reposApi, workstreams as wsApi } from '../api/endpoints';
import type { Plan, PlanTask, Repository, Message } from '../types';
import toast from 'react-hot-toast';

// Custom node component for task cards
function TaskNodeComponent({ data }: { data: { task: PlanTask; onSelect: (id: string) => void } }) {
  const { task, onSelect } = data;

  const statusColors: Record<string, string> = {
    pending: 'border-gray-600 bg-cyber-bg',
    ready: 'border-neon-yellow bg-neon-yellow/5',
    claimed: 'border-neon-cyan bg-neon-cyan/5',
    working: 'border-neon-cyan bg-neon-cyan/5',
    pr_open: 'border-neon-purple bg-neon-purple/5',
    in_review: 'border-neon-purple bg-neon-purple/5',
    changes_requested: 'border-neon-orange bg-neon-orange/5',
    merged: 'border-neon-green bg-neon-green/5',
    failed: 'border-neon-red bg-neon-red/5',
    cancelled: 'border-gray-700 bg-gray-800/50',
  };

  const statusDots: Record<string, string> = {
    pending: 'bg-gray-500',
    ready: 'bg-neon-yellow',
    claimed: 'bg-neon-cyan animate-pulse',
    working: 'bg-neon-cyan animate-pulse',
    pr_open: 'bg-neon-purple',
    in_review: 'bg-neon-purple',
    merged: 'bg-neon-green',
    failed: 'bg-neon-red',
    cancelled: 'bg-gray-600',
  };

  return (
    <div
      onClick={() => onSelect(task.id)}
      className={`relative border-2 rounded-lg p-3 min-w-[180px] max-w-[250px] cursor-pointer hover:shadow-lg transition-all ${statusColors[task.status] || statusColors.pending}`}
    >
      <Handle type="target" position={Position.Left} className="!bg-neon-cyan !w-2 !h-2 !border-0" />
      <Handle type="source" position={Position.Right} className="!bg-neon-cyan !w-2 !h-2 !border-0" />
      <div className="flex items-center gap-2 mb-1">
        <span className={`w-2 h-2 rounded-full flex-shrink-0 ${statusDots[task.status] || statusDots.pending}`} />
        <span className="font-mono font-bold text-xs text-gray-200 truncate">{task.id}</span>
      </div>
      <p className="text-xs text-gray-400 line-clamp-2">{task.description}</p>
      <div className="flex items-center justify-between mt-1.5">
        <span className="text-[10px] text-gray-600 capitalize">{task.status.replace('_', ' ')}</span>
        <span className="text-[10px] text-gray-600">W{task.wave}</span>
      </div>
    </div>
  );
}

const nodeTypes = { taskNode: TaskNodeComponent };

interface ChatMessage {
  id: string;
  role: 'user' | 'system';
  content: string;
  planId?: string;
  timestamp: Date;
}

// Task detail panel with activity log + diff tabs
function TaskPanel({ task, onClose, navigate }: { task: PlanTask; onClose: () => void; navigate: (path: string) => void }) {
  const [activeTab, setActiveTab] = useState<'activity' | 'diff'>('activity');
  const [diffData, setDiffData] = useState<{ stat: string; diff: string }>({ stat: '', diff: '' });
  const [messages, setMessages] = useState<Message[]>([]);
  const logEndRef = useRef<HTMLDivElement>(null);

  // Poll activity messages
  useEffect(() => {
    if (!task.workstream_id) return;
    let cancelled = false;
    const fetch = async () => {
      try {
        const res = await wsApi.getMessages(task.workstream_id!, 1, 200);
        if (!cancelled) {
          setMessages((res.data.items || []).filter((m: Message) => {
            const c = m.content;
            return c && c.trim() && !c.startsWith('[status:null]') && !c.startsWith('[status:running]');
          }));
        }
      } catch { /* ignore */ }
    };
    fetch();
    const interval = setInterval(fetch, 3000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [task.workstream_id]);

  // Fetch diff when tab is active
  useEffect(() => {
    if (activeTab !== 'diff' || !task.workstream_id) return;
    let cancelled = false;
    const fetchDiff = async () => {
      try {
        const res = await wsApi.getDiff(task.workstream_id!);
        if (!cancelled) setDiffData(res.data);
      } catch { /* ignore */ }
    };
    fetchDiff();
    const interval = setInterval(fetchDiff, 10000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [activeTab, task.workstream_id]);

  // Auto-scroll activity log
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length]);

  return (
    <div className="absolute top-0 right-0 w-[600px] h-full bg-cyber-card border-l border-cyber-border shadow-xl flex flex-col">
      {/* Header */}
      <div className="p-3 border-b border-cyber-border flex-shrink-0">
        <div className="flex items-center justify-between mb-1.5">
          <span className="font-mono font-bold text-sm text-neon-cyan">{task.id}</span>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-300 text-lg leading-none">×</button>
        </div>
        <p className="text-xs text-gray-400 line-clamp-2 mb-2">{task.description}</p>
        <div className="flex items-center gap-2 text-xs flex-wrap">
          <span className={`capitalize px-1.5 py-0.5 rounded ${
            task.status === 'working' || task.status === 'claimed' ? 'bg-neon-cyan/10 text-neon-cyan' :
            task.status === 'merged' ? 'bg-neon-green/10 text-neon-green' :
            task.status === 'failed' ? 'bg-neon-red/10 text-neon-red' :
            'bg-gray-800 text-gray-400'
          }`}>{task.status.replace('_', ' ')}</span>
          {task.pr_url && (
            <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="text-neon-green hover:underline">PR →</a>
          )}
        </div>
      </div>

      {/* Tabs */}
      {task.workstream_id ? (
        <>
          <div className="flex border-b border-cyber-border flex-shrink-0">
            <button
              onClick={() => setActiveTab('activity')}
              className={`flex items-center gap-1.5 px-4 py-2 text-xs font-mono border-b-2 ${
                activeTab === 'activity' ? 'border-neon-cyan text-neon-cyan' : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              <MessageSquare className="h-3 w-3" /> Activity
            </button>
            <button
              onClick={() => setActiveTab('diff')}
              className={`flex items-center gap-1.5 px-4 py-2 text-xs font-mono border-b-2 ${
                activeTab === 'diff' ? 'border-neon-cyan text-neon-cyan' : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              <FileCode className="h-3 w-3" /> Changes
            </button>
          </div>

          {/* Content */}
          <div className="flex-1 min-h-0">
            {activeTab === 'activity' && (
              <div className="h-full overflow-y-auto p-2 space-y-1">
                {messages.length === 0 && (
                  <div className="flex items-center justify-center h-full">
                    <div className="text-center">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-neon-cyan mx-auto mb-2" />
                      <p className="text-gray-600 text-xs">Agent is working...</p>
                    </div>
                  </div>
                )}
                {messages.map((msg, i) => {
                  const c = msg.content;
                  const isTool = c.startsWith('🔧');
                  const isOutput = c.startsWith('📋');
                  const isError = c.startsWith('❌');
                  const isThought = c.startsWith('💭');
                  const isPR = c.startsWith('📬');
                  const isPlan = c.startsWith('📝');

                  if (isTool) {
                    return (
                      <div key={msg.id || i} className="border-l-2 border-neon-cyan/40 pl-2 py-0.5">
                        <div className="text-[11px] font-mono text-neon-cyan/80">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>{c}</ReactMarkdown>
                        </div>
                      </div>
                    );
                  }

                  if (isOutput) {
                    return (
                      <div key={msg.id || i} className="border-l-2 border-gray-700 pl-2 py-0.5 ml-2">
                        <div className="text-[10px] font-mono text-gray-500 prose prose-invert prose-xs max-w-none prose-pre:my-0.5 prose-pre:bg-cyber-bg prose-pre:border prose-pre:border-cyber-border prose-pre:text-[9px] prose-pre:p-1.5 prose-code:text-gray-400 prose-code:before:content-none prose-code:after:content-none">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>{c.replace(/^📋\s*Output:\s*/, '')}</ReactMarkdown>
                        </div>
                      </div>
                    );
                  }

                  if (isError) {
                    return (
                      <div key={msg.id || i} className="border-l-2 border-neon-red/50 pl-2 py-0.5 bg-neon-red/5 rounded-r">
                        <p className="text-[11px] font-mono text-neon-red">{c}</p>
                      </div>
                    );
                  }

                  if (isThought) {
                    return (
                      <div key={msg.id || i} className="pl-2 py-0.5">
                        <div className="text-[11px] text-gray-500 italic prose prose-invert prose-xs max-w-none prose-p:my-0 prose-code:text-gray-400 prose-code:before:content-none prose-code:after:content-none">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>{c.replace(/^💭\s*/, '')}</ReactMarkdown>
                        </div>
                      </div>
                    );
                  }

                  if (isPR) {
                    return (
                      <div key={msg.id || i} className="border-l-2 border-neon-green/50 pl-2 py-1 bg-neon-green/5 rounded-r">
                        <div className="text-[11px] font-mono text-neon-green prose prose-invert prose-xs max-w-none prose-a:text-neon-green">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>{c}</ReactMarkdown>
                        </div>
                      </div>
                    );
                  }

                  // Default: agent reasoning/text
                  return (
                    <div key={msg.id || i} className="py-1 border-b border-cyber-border/30 last:border-0">
                      <div className="text-xs text-gray-300 prose prose-invert prose-sm max-w-none prose-p:my-1 prose-pre:my-1 prose-pre:bg-cyber-bg prose-pre:border prose-pre:border-cyber-border prose-pre:text-[10px] prose-pre:p-2 prose-code:text-neon-green prose-code:before:content-none prose-code:after:content-none prose-strong:text-gray-200 prose-headings:text-gray-200 prose-a:text-neon-cyan prose-li:my-0">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{c}</ReactMarkdown>
                      </div>
                    </div>
                  );
                })}
                <div ref={logEndRef} />
              </div>
            )}
            {activeTab === 'diff' && (
              <div className="h-full overflow-auto p-3">
                {diffData.stat && (
                  <pre className="text-[10px] text-gray-500 font-mono mb-3">{diffData.stat}</pre>
                )}
                {diffData.diff ? (
                  <pre className="text-[10px] font-mono leading-4">
                    {diffData.diff.split('\n').map((line, i) => {
                      let cls = 'text-gray-500';
                      if (line.startsWith('+') && !line.startsWith('+++')) cls = 'text-green-400 bg-green-400/10';
                      else if (line.startsWith('-') && !line.startsWith('---')) cls = 'text-red-400 bg-red-400/10';
                      else if (line.startsWith('@@')) cls = 'text-purple-400';
                      else if (line.startsWith('diff --git')) cls = 'text-neon-cyan font-bold mt-3 border-t border-cyber-border pt-1';
                      return <div key={i} className={`px-1 ${cls}`}>{line || ' '}</div>;
                    })}
                  </pre>
                ) : (
                  <p className="text-gray-600 text-xs text-center py-8">No changes yet.</p>
                )}
              </div>
            )}
          </div>
        </>
      ) : (
        <div className="flex-1 flex items-center justify-center">
          <p className="text-xs text-gray-600">
            {task.status === 'pending' || task.status === 'ready' ? 'Waiting for agent to start...' : 'No agent assigned'}
          </p>
        </div>
      )}
    </div>
  );
}

export default function OrchestratorPage() {
  const { planId } = useParams<{ planId: string }>();
  const navigate = useNavigate();
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [selectedRepo, setSelectedRepo] = useState('');
  const [currentPlan, setCurrentPlan] = useState<Plan | null>(null);
  const [recentPlans, setRecentPlans] = useState<Plan[]>([]);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [chatCollapsed, setChatCollapsed] = useState(false);
  const [selectedTask, setSelectedTask] = useState<PlanTask | null>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);

  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);

  // Load repos and recent plans
  useEffect(() => {
    reposApi.listRepos(1).then(r => {
      const items = r.data.items || [];
      setRepositories(items);
      if (items.length > 0 && !selectedRepo) setSelectedRepo(items[0].id);
    }).catch(() => {});
    plansApi.list({ per_page: 20 }).then(r => setRecentPlans(r.data.items || [])).catch(() => {});
  }, []);

  // Load specific plan if planId in URL
  useEffect(() => {
    if (planId) {
      loadPlan(planId, true);
    }
  }, [planId]);

  // Poll current plan every 3 seconds for real-time updates
  useEffect(() => {
    if (!currentPlan) return;
    const interval = setInterval(() => loadPlan(currentPlan.id), 3000);
    return () => clearInterval(interval);
  }, [currentPlan?.id]);

  // Scroll chat
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages.length]);

  const loadPlan = async (id: string, isInitial = false) => {
    try {
      const res = await plansApi.get(id);
      setCurrentPlan(res.data);
      buildGraph(res.data.tasks || []);
      // Always update chat from plan conversations
      if (res.data.conversations) {
        try {
          const saved = typeof res.data.conversations === 'string'
            ? JSON.parse(res.data.conversations)
            : res.data.conversations;
          if (Array.isArray(saved) && saved.length > 0) {
            setChatMessages(saved.map((m: ChatMessage) => ({
              ...m,
              timestamp: new Date(m.timestamp),
            })));
          }
        } catch { /* ignore parse errors */ }
      }
    } catch { /* ignore polling errors */ }
  };

  // Save chat to plan when messages change
  useEffect(() => {
    if (currentPlan && chatMessages.length > 0) {
      plansApi.saveConversations(currentPlan.id, chatMessages).catch(() => {});
    }
  }, [chatMessages.length]);

  const buildGraph = useCallback((tasks: PlanTask[]) => {
    if (tasks.length === 0) {
      setNodes([]);
      setEdges([]);
      return;
    }

    // Group by wave for layout
    const waveGroups = new Map<number, PlanTask[]>();
    tasks.forEach(t => {
      const wave = t.wave || 0;
      if (!waveGroups.has(wave)) waveGroups.set(wave, []);
      waveGroups.get(wave)!.push(t);
    });

    const newNodes: Node[] = [];
    const newEdges: Edge[] = [];

    const xSpacing = 300;
    const ySpacing = 120;

    waveGroups.forEach((waveTasks, wave) => {
      waveTasks.forEach((task, idx) => {
        const yOffset = -(waveTasks.length - 1) * ySpacing / 2;
        newNodes.push({
          id: task.id,
          type: 'taskNode',
          position: { x: wave * xSpacing + 50, y: idx * ySpacing + yOffset + 200 },
          data: { task, onSelect: (id: string) => {
            const found = tasks.find(t => t.id === id);
            if (found) setSelectedTask(found);
          }},
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
        });

        task.depends_on?.forEach(dep => {
          newEdges.push({
            id: `${dep}-${task.id}`,
            source: dep,
            target: task.id,
            animated: task.status === 'working' || task.status === 'claimed',
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed, color: '#00f0ff', width: 15, height: 15 },
            style: { stroke: '#00f0ff', strokeWidth: 2 },
          });
        });
      });
    });

    setNodes(newNodes);
    setEdges(newEdges);
  }, []);

  const handleSend = async (e: FormEvent) => {
    e.preventDefault();
    const text = input.trim();
    if (!text || !selectedRepo) return;
    setInput('');

    const userMsg: ChatMessage = { id: Date.now().toString(), role: 'user', content: text, timestamp: new Date() };
    setChatMessages(prev => [...prev, userMsg]);

    setSubmitting(true);
    try {
      if (currentPlan && currentPlan.status === 'pending_approval') {
        // Refine existing plan — re-decompose with additional context
        const refinedPrompt = currentPlan.prompt + '\n\nAdditional instructions:\n' + text;
        // Delete existing tasks and re-create plan with refined prompt
        const tasks = currentPlan.tasks || [];
        for (const t of tasks) {
          await plansApi.removeTask(currentPlan.id, t.id).catch(() => {});
        }
        // Create a new plan with the refined prompt (delete old, create new)
        await plansApi.delete(currentPlan.id).catch(() => {});
        const res = await plansApi.create({ repository_id: selectedRepo, prompt: refinedPrompt, auto_approve: false });
        setChatMessages(prev => [...prev, {
          id: (Date.now() + 1).toString(), role: 'system',
          content: 'Refining plan with your feedback...',
          timestamp: new Date(),
        }]);
        setCurrentPlan(res.data);
        navigate(`/orchestrator/${res.data.id}`, { replace: true });
        plansApi.list({ per_page: 20 }).then(r => setRecentPlans(r.data.items || [])).catch(() => {});
      } else {
        // Create new plan
        const res = await plansApi.create({ repository_id: selectedRepo, prompt: text, auto_approve: false });
        setChatMessages(prev => [...prev, {
          id: (Date.now() + 1).toString(), role: 'system',
          content: 'Plan created. Analyzing complexity and decomposing into tasks...',
          planId: res.data.id, timestamp: new Date(),
        }]);
        setCurrentPlan(res.data);
        navigate(`/orchestrator/${res.data.id}`, { replace: true });
        plansApi.list({ per_page: 20 }).then(r => setRecentPlans(r.data.items || [])).catch(() => {});
      }
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed';
      setChatMessages(prev => [...prev, { id: (Date.now() + 1).toString(), role: 'system', content: `Error: ${msg}`, timestamp: new Date() }]);
    } finally {
      setSubmitting(false);
    }
  };

  const handleApprove = async () => {
    if (!currentPlan) return;
    try {
      await plansApi.approve(currentPlan.id);
      toast.success('Plan approved — execution started');
      setChatMessages(prev => [...prev, { id: Date.now().toString(), role: 'system', content: 'Plan approved! Tasks are being provisioned...', timestamp: new Date() }]);
      loadPlan(currentPlan.id);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to approve';
      toast.error(msg);
    }
  };

  const handleReject = async () => {
    if (!currentPlan) return;
    try {
      await plansApi.reject(currentPlan.id);
      toast.success('Plan rejected');
      loadPlan(currentPlan.id);
    } catch { toast.error('Failed to reject'); }
  };

  const logout = useAuthStore((s) => s.logout);
  const user = useAuthStore((s) => s.user);

  return (
    <div className="flex h-screen bg-cyber-bg">
      {/* Left sidebar: plans + repo */}
      <div className="w-72 border-r border-cyber-border bg-cyber-surface flex flex-col flex-shrink-0">
        {/* Header */}
        <div className="flex items-center gap-2 px-4 py-3 border-b border-cyber-border">
          <Cpu className="h-5 w-5 text-neon-cyan" />
          <span className="font-display font-bold text-neon-cyan tracking-wider text-sm">SMOL</span>
          <span className="font-display font-bold text-gray-500 tracking-wider text-sm">GANG</span>
          <div className="flex-1" />
          <button onClick={() => navigate('/repositories')} className="p-1 text-gray-600 hover:text-gray-400" title="Repositories">
            <GitFork className="h-3.5 w-3.5" />
          </button>
          <button onClick={() => navigate('/workstreams')} className="p-1 text-gray-600 hover:text-gray-400" title="Workstreams">
            <Layers className="h-3.5 w-3.5" />
          </button>
          {user?.role === 'admin' && (
            <button onClick={() => navigate('/users')} className="p-1 text-gray-600 hover:text-gray-400" title="Admin">
              <Settings className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {/* New plan + repo */}
        <div className="p-3 border-b border-cyber-border">
          <button
            onClick={() => { setCurrentPlan(null); setSelectedTask(null); setNodes([]); setEdges([]); setChatMessages([]); navigate('/orchestrator'); }}
            className="w-full btn-neon-cyan text-xs flex items-center justify-center gap-1 mb-2"
          >
            <Plus className="h-3 w-3" /> New Plan
          </button>
          <select
            value={selectedRepo}
            onChange={(e) => setSelectedRepo(e.target.value)}
            className="input-cyber w-full text-xs"
          >
            {repositories.map(r => (
              <option key={r.id} value={r.id}>{r.github_owner}/{r.github_repo}</option>
            ))}
          </select>
        </div>

        {/* Recent plans */}
        <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
          {recentPlans.map(p => (
            <div key={p.id} className="flex items-center group">
              <button
                onClick={() => { navigate(`/orchestrator/${p.id}`); loadPlan(p.id, true); }}
                className={`flex-1 text-left px-3 py-2 rounded-lg text-xs transition-colors ${
                  currentPlan?.id === p.id
                    ? 'bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/20'
                    : 'text-gray-400 hover:bg-cyber-hover hover:text-gray-300 border border-transparent'
                }`}
              >
                <p className="truncate font-medium">{p.prompt.slice(0, 80)}</p>
                <div className="flex items-center gap-2 mt-0.5">
                  <span className={`text-[10px] ${
                    p.status === 'active' ? 'text-neon-green' :
                    p.status === 'complete' ? 'text-neon-green' :
                    p.status === 'halted' ? 'text-neon-red' :
                    'text-gray-600'
                  }`}>{p.status.replace('_', ' ')}</span>
                  <span className="text-[10px] text-gray-700">{new Date(p.created_at).toLocaleDateString()}</span>
                </div>
              </button>
              <button
                onClick={async (e) => {
                  e.stopPropagation();
                  try {
                    await plansApi.delete(p.id);
                    setRecentPlans(prev => prev.filter(x => x.id !== p.id));
                    if (currentPlan?.id === p.id) {
                      setCurrentPlan(null);
                      setNodes([]);
                      setEdges([]);
                      navigate('/orchestrator');
                    }
                    toast.success('Plan deleted');
                  } catch { toast.error('Failed to delete'); }
                }}
                className="p-1 text-gray-700 hover:text-neon-red opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
                title="Delete plan"
              >
                <Trash2 className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>

        {/* User */}
        <div className="border-t border-cyber-border px-3 py-2 flex items-center gap-2">
          <div className="h-6 w-6 rounded bg-neon-cyan/10 border border-neon-cyan/30 flex items-center justify-center">
            <span className="text-[10px] font-mono font-bold text-neon-cyan">{user?.name?.charAt(0).toUpperCase()}</span>
          </div>
          <span className="text-xs text-gray-400 truncate flex-1">{user?.name}</span>
          <button onClick={logout} className="p-1 text-gray-600 hover:text-neon-red" title="Logout">
            <LogOut className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* Main: Prompt + Graph + Chat */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Plan prompt banner */}
        {currentPlan && (
          <div className="border-b border-cyber-border bg-cyber-surface px-4 py-2 flex-shrink-0">
            <div className="flex items-center gap-3">
              <div className="flex-1 min-w-0">
                <p className="text-sm text-gray-300 truncate">{currentPlan.prompt}</p>
              </div>
              <span className={`text-[10px] px-2 py-0.5 rounded-full font-mono ${
                currentPlan.status === 'active' ? 'bg-neon-green/10 text-neon-green border border-neon-green/30' :
                currentPlan.status === 'complete' ? 'bg-neon-green/10 text-neon-green border border-neon-green/30' :
                currentPlan.status === 'pending_approval' ? 'bg-neon-yellow/10 text-neon-yellow border border-neon-yellow/30' :
                currentPlan.status === 'halted' ? 'bg-neon-red/10 text-neon-red border border-neon-red/30' :
                'bg-gray-700 text-gray-400'
              }`}>{currentPlan.status.replace('_', ' ')}</span>
              {currentPlan.complexity !== 'assessing' && (
                <span className="text-[10px] text-gray-600 font-mono">{currentPlan.complexity}</span>
              )}
            </div>
          </div>
        )}

        {/* Graph area */}
        <div className="flex-1 min-h-0 relative bg-cyber-bg">
          {currentPlan && nodes.length > 0 ? (
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              nodeTypes={nodeTypes}
              fitView
              proOptions={{ hideAttribution: true }}
            >
              <Background color="#1a1a2e" gap={20} />
              <Controls className="!bg-cyber-card !border-cyber-border !rounded-lg" />
              <MiniMap
                nodeColor={(n) => {
                  const status = n.data?.task?.status;
                  if (status === 'merged') return '#39ff14';
                  if (status === 'working' || status === 'claimed') return '#00f0ff';
                  if (status === 'failed') return '#ff0040';
                  if (status === 'ready') return '#ffd700';
                  return '#2a2a3e';
                }}
                className="!bg-cyber-card !border-cyber-border !rounded-lg"
              />
            </ReactFlow>
          ) : (
            <div className="flex items-center justify-center h-full text-gray-600">
              <div className="text-center">
                <p className="text-lg font-display">No active plan</p>
                <p className="text-sm mt-1">Describe what you want to build below</p>
              </div>
            </div>
          )}

          {/* Action bar — pending approval */}
          {currentPlan?.status === 'pending_approval' && currentPlan.complexity !== 'assessing' && (currentPlan.tasks?.length || 0) > 0 && (
            <div className="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-3 bg-cyber-card border border-cyber-border rounded-lg px-4 py-2 shadow-lg">
              <span className="text-sm text-gray-300">Review the plan, refine via chat, then approve</span>
              <button onClick={handleApprove} className="btn-neon-green text-xs px-3 py-1">Approve & Start</button>
              <button onClick={handleReject} className="btn-neon-red text-xs px-3 py-1">Reject</button>
            </div>
          )}

          {/* Action bar — active plan */}
          {currentPlan?.status === 'active' && (
            <div className="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-3 bg-cyber-card border border-cyber-border rounded-lg px-4 py-2 shadow-lg">
              <span className="text-sm text-gray-300">Plan is executing</span>
              <button
                onClick={async () => {
                  if (!currentPlan) return;
                  try {
                    await plansApi.complete(currentPlan.id);
                    toast.success('Plan marked complete');
                    loadPlan(currentPlan.id);
                  } catch { toast.error('Failed to complete plan'); }
                }}
                className="btn-neon-green text-xs px-3 py-1"
              >
                Complete Plan
              </button>
            </div>
          )}

          {/* Action bar — halted/failed plan */}
          {(currentPlan?.status === 'halted' || currentPlan?.status === 'rejected') && (
            <div className="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-3 bg-cyber-card border border-neon-red/30 rounded-lg px-4 py-2 shadow-lg">
              <span className="text-sm text-neon-red">Plan {currentPlan.status}</span>
              <button
                onClick={async () => {
                  if (!currentPlan) return;
                  try {
                    await plansApi.restart(currentPlan.id);
                    toast.success('Plan restarted');
                    loadPlan(currentPlan.id);
                  } catch { toast.error('Failed to restart'); }
                }}
                className="btn-neon-cyan text-xs px-3 py-1"
              >
                Restart Plan
              </button>
            </div>
          )}

          {/* Assessing indicator */}
          {currentPlan?.complexity === 'assessing' && (
            <div className="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-3 bg-cyber-card border border-neon-cyan/30 rounded-lg px-4 py-2">
              <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-neon-cyan" />
              <span className="text-sm text-neon-cyan">Analyzing and decomposing task...</span>
            </div>
          )}

          {/* Task detail panel with inline chat/diff/logs */}
          {selectedTask && (
            <TaskPanel
              task={selectedTask}
              onClose={() => setSelectedTask(null)}
              navigate={navigate}
            />
          )}
        </div>

        {/* Bottom: Collapsible Chat */}
        <div className={`border-t border-cyber-border flex flex-col bg-cyber-card flex-shrink-0 transition-all ${chatCollapsed ? 'h-10' : 'h-[280px]'}`}>
          {/* Chat header / toggle */}
          <button
            onClick={() => setChatCollapsed(!chatCollapsed)}
            className="flex items-center justify-between px-4 py-2 text-xs font-mono text-gray-400 hover:text-gray-300 flex-shrink-0 border-b border-cyber-border"
          >
            <span>
              {currentPlan?.status === 'pending_approval' ? 'Plan Refinement' : 'Chat'}
              {chatMessages.length > 0 && <span className="text-gray-600 ml-2">({chatMessages.length})</span>}
            </span>
            {chatCollapsed ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
          </button>

          {!chatCollapsed && (
            <>
              {/* Messages */}
              <div className="flex-1 overflow-y-auto p-4 space-y-3 min-h-0">
                {chatMessages.length === 0 && (
                  <p className="text-center text-gray-600 text-sm py-4">
                    Describe what you want to build. The system will analyze it, create a task plan, and execute it.
                  </p>
                )}
                {chatMessages.map(msg => (
                  <div key={msg.id} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                    <div className={`max-w-[80%] px-3 py-2 rounded-lg text-sm ${
                      msg.role === 'user'
                        ? 'bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30'
                        : 'bg-cyber-surface text-gray-400 border border-cyber-border'
                    }`}>
                      {msg.content}
                    </div>
                  </div>
                ))}
                <div ref={chatEndRef} />
              </div>

              {/* Input */}
              <form onSubmit={handleSend} className="border-t border-cyber-border p-3 flex items-end gap-2">
                <textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(e); } }}
                  placeholder={!selectedRepo ? "Select a repository first" : currentPlan?.status === 'pending_approval' ? "Refine the plan — add details, change scope..." : "Describe what you want to build..."}
                  disabled={!selectedRepo || submitting}
                  rows={2}
                  className="input-cyber flex-1 resize-none text-sm px-3 py-2"
                />
                <button
                  type="submit"
                  disabled={!input.trim() || !selectedRepo || submitting}
                  className="p-2.5 bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/40 rounded-lg hover:bg-neon-cyan/20 disabled:opacity-50 disabled:cursor-not-allowed transition-all flex-shrink-0"
                >
                  <Send className="h-4 w-4" />
                </button>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
