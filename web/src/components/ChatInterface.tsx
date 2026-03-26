import { useState, useEffect, useRef } from 'react';
import { Send, User, Bot, Info, Wifi, WifiOff } from 'lucide-react';
import useWebSocket from '../hooks/useWebSocket';
import { workstreams as wsApi } from '../api/endpoints';
import type { Message } from '../types';

interface ChatInterfaceProps {
  workstreamId: string;
}

export default function ChatInterface({ workstreamId }: ChatInterfaceProps) {
  const [input, setInput] = useState('');
  const [historicalMessages, setHistoricalMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { messages: wsMessages, connected, send } = useWebSocket(workstreamId);

  useEffect(() => {
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
  }, [workstreamId]);

  const allMessages = [...historicalMessages, ...wsMessages];

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [allMessages.length]);

  const handleSend = async () => {
    const trimmed = input.trim();
    if (!trimmed) return;
    setInput('');
    try {
      // Send via REST API only — the WebSocket is for receiving real-time updates
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

  const sourceIcon = (source: string) => {
    switch (source) {
      case 'user':
        return <User className="h-4 w-4" />;
      case 'agent':
        return <Bot className="h-4 w-4" />;
      default:
        return <Info className="h-4 w-4" />;
    }
  };

  const formatTime = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
      });
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
    <div className="flex flex-col h-full bg-cyber-card border border-cyber-border rounded-lg">
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
      <div className="flex-1 overflow-y-auto p-4 space-y-4 min-h-0">
        {allMessages.length === 0 && (
          <p className="text-center text-sm text-gray-500 py-8">
            No messages yet. Send a message to get started.
          </p>
        )}
        {allMessages.map((msg, idx) => {
          const isUser = msg.source === 'user';
          const isSystem = msg.source === 'system';

          if (isSystem) {
            return (
              <div key={msg.id || idx} className="flex justify-center">
                <div className="bg-cyber-surface/50 text-gray-500 border border-cyber-border text-xs px-3 py-1.5 rounded-full">
                  {msg.content}
                </div>
              </div>
            );
          }

          return (
            <div
              key={msg.id || idx}
              className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-[80%] ${
                  isUser ? 'order-1' : 'order-2'
                }`}
              >
                <div className="flex items-center gap-1.5 mb-1">
                  {!isUser && (
                    <span className="flex items-center gap-1 text-xs text-gray-500 font-mono">
                      {sourceIcon(msg.source)}
                      <span className="capitalize">{msg.source}</span>
                    </span>
                  )}
                  {isUser && (
                    <span className="flex items-center gap-1 text-xs text-gray-500 font-mono ml-auto">
                      <span className="capitalize">{msg.source}</span>
                      {sourceIcon(msg.source)}
                    </span>
                  )}
                </div>
                <div
                  className={`px-4 py-2.5 text-sm whitespace-pre-wrap ${
                    isUser
                      ? 'bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30 rounded-lg rounded-br-sm'
                      : 'bg-cyber-surface text-gray-300 border border-cyber-border rounded-lg rounded-bl-sm'
                  }`}
                >
                  {msg.content}
                </div>
                <p
                  className={`text-xs text-gray-600 mt-1 ${
                    isUser ? 'text-right' : 'text-left'
                  }`}
                >
                  {formatTime(msg.created_at)}
                </p>
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
            className="p-2.5 bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/40 rounded-lg hover:bg-neon-cyan/20 hover:shadow-neon-cyan disabled:opacity-50 disabled:cursor-not-allowed transition-all"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
