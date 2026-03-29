import { useState, useEffect, useRef } from 'react';
import { Send, User, Bot, Wrench, Wifi, WifiOff } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import useWebSocket from '../hooks/useWebSocket';
import { workstreams as wsApi } from '../api/endpoints';
import type { Message } from '../types';

interface ChatInterfaceProps {
  workstreamId: string;
}

// Filter out noisy internal messages
function isVisible(msg: Message): boolean {
  const c = msg.content;
  if (!c || !c.trim()) return false;
  if (c.startsWith('[status:null]')) return false;
  if (c.startsWith('[status:running]')) return false;
  if (c.startsWith('[status:error]') && c.includes('Failed to report')) return false;
  return true;
}

// Detect tool messages
function isToolMessage(content: string): boolean {
  return content.startsWith('\u{1F527} Tool:');
}

export default function ChatInterface({ workstreamId }: ChatInterfaceProps) {
  const [input, setInput] = useState('');
  const [historicalMessages, setHistoricalMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { messages: wsMessages, connected } = useWebSocket(workstreamId);

  useEffect(() => {
    setLoading(true);
    setHistoricalMessages([]);
    initialLoadDone.current = false;

    const fetchMessages = async () => {
      try {
        const res = await wsApi.getMessages(workstreamId);
        setHistoricalMessages(res.data.items || []);
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    };

    fetchMessages();
    // Poll for new messages every 5 seconds (catches any missed by WebSocket)
    const interval = setInterval(fetchMessages, 5000);
    return () => clearInterval(interval);
  }, [workstreamId]);

  // Use historical (polled) messages as source of truth.
  // WS messages only shown if they arrived after the last poll (not yet in historical).
  const allMessages = (() => {
    const historicalIds = new Set(historicalMessages.map(m => m.id).filter(Boolean));
    const visible = historicalMessages.filter(isVisible);

    // Only add WS messages that aren't already in the polled set
    for (const msg of wsMessages) {
      if (msg.id && historicalIds.has(msg.id)) continue;
      // Check by content+source to catch messages without IDs
      const isDupe = historicalMessages.some(
        h => h.source === msg.source && h.content === msg.content
      );
      if (!isDupe && isVisible(msg)) {
        visible.push(msg);
      }
    }
    return visible;
  })();

  const initialLoadDone = useRef(false);
  useEffect(() => {
    if (!initialLoadDone.current && !loading && allMessages.length > 0) {
      // First load — jump to bottom instantly, no animation
      messagesEndRef.current?.scrollIntoView({ behavior: 'instant' });
      initialLoadDone.current = true;
    } else if (initialLoadDone.current) {
      // Subsequent messages — smooth scroll
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [allMessages.length, loading]);

  const handleSend = async () => {
    const trimmed = input.trim();
    if (!trimmed) return;
    setInput('');
    try {
      await wsApi.sendMessage(workstreamId, trimmed);
    } catch {
      // ignore
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const formatTime = (dateStr: string) => {
    if (!dateStr) return '';
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return '';
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return '';
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-neon-cyan" />
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-cyber-card border border-cyber-border rounded-b-lg">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-cyber-border">
        <h3 className="text-sm font-mono font-semibold text-neon-cyan uppercase tracking-wider">Chat</h3>
        <div className="flex items-center gap-1.5 text-xs">
          {connected ? (
            <>
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-neon-green opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-neon-green" />
              </span>
              <Wifi className="h-3.5 w-3.5 text-neon-green" />
              <span className="text-neon-green">Connected</span>
            </>
          ) : (
            <>
              <WifiOff className="h-3.5 w-3.5 text-neon-red" />
              <span className="text-neon-red">Disconnected</span>
            </>
          )}
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3 min-h-0">
        {allMessages.length === 0 && (
          <p className="text-center text-sm text-gray-500 py-8">
            No messages yet. Send a message to get started.
          </p>
        )}
        {allMessages.map((msg, idx) => {
          const isUser = msg.source === 'user';
          const isTool = !isUser && isToolMessage(msg.content);
          const time = formatTime(msg.created_at);

          // Tool calls rendered inline/compact
          if (isTool) {
            return (
              <div key={msg.id || idx} className="flex items-center gap-2 px-3 py-1.5 text-xs">
                <Wrench className="h-3 w-3 text-neon-yellow flex-shrink-0" />
                <span className="text-gray-500 font-mono">{msg.content.replace(/^🔧\s*/, '')}</span>
                {time && <span className="text-gray-700 ml-auto">{time}</span>}
              </div>
            );
          }

          return (
            <div
              key={msg.id || idx}
              className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}
            >
              <div className={`max-w-[85%]`}>
                <div className="flex items-center gap-1.5 mb-1">
                  {isUser ? (
                    <span className="flex items-center gap-1 text-xs text-gray-500 font-mono ml-auto">
                      You
                      <User className="h-3.5 w-3.5" />
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-xs text-gray-500 font-mono">
                      <Bot className="h-3.5 w-3.5" />
                      Agent
                    </span>
                  )}
                </div>
                <div
                  className={`px-4 py-2.5 text-sm ${
                    isUser
                      ? 'bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30 rounded-lg rounded-br-sm'
                      : 'bg-cyber-surface text-gray-300 border border-cyber-border rounded-lg rounded-bl-sm'
                  }`}
                >
                  {isUser ? (
                    <span className="whitespace-pre-wrap">{msg.content}</span>
                  ) : (
                    <div className="prose prose-invert prose-sm max-w-none prose-p:my-1 prose-pre:my-2 prose-pre:bg-cyber-bg prose-pre:border prose-pre:border-cyber-border prose-code:text-neon-green prose-code:before:content-none prose-code:after:content-none prose-headings:text-gray-200 prose-a:text-neon-cyan prose-strong:text-gray-200 prose-li:my-0.5">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>
                        {msg.content}
                      </ReactMarkdown>
                    </div>
                  )}
                </div>
                {time && (
                  <p className={`text-xs text-gray-600 mt-1 ${isUser ? 'text-right' : 'text-left'}`}>
                    {time}
                  </p>
                )}
              </div>
            </div>
          );
        })}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t border-cyber-border p-4">
        <div className="flex items-end gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            rows={1}
            className="input-cyber flex-1 resize-none px-4 py-2.5 text-sm"
          />
          <button
            onClick={handleSend}
            disabled={!input.trim()}
            className="p-2.5 bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30 rounded-lg hover:bg-neon-cyan/20 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
