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
      await wsApi.sendMessage(workstreamId, trimmed);
      send(trimmed);
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
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-white rounded-xl border border-gray-200">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
        <h3 className="text-sm font-semibold text-gray-900">Chat</h3>
        <div className="flex items-center gap-1.5 text-xs">
          {connected ? (
            <>
              <Wifi className="h-3.5 w-3.5 text-green-500" />
              <span className="text-green-600">Connected</span>
            </>
          ) : (
            <>
              <WifiOff className="h-3.5 w-3.5 text-red-500" />
              <span className="text-red-600">Disconnected</span>
            </>
          )}
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 min-h-0">
        {allMessages.length === 0 && (
          <p className="text-center text-sm text-gray-400 py-8">
            No messages yet. Send a message to get started.
          </p>
        )}
        {allMessages.map((msg, idx) => {
          const isUser = msg.source === 'user';
          const isSystem = msg.source === 'system';

          if (isSystem) {
            return (
              <div key={msg.id || idx} className="flex justify-center">
                <div className="bg-gray-100 text-gray-500 text-xs px-3 py-1.5 rounded-full">
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
                    <span className="flex items-center gap-1 text-xs text-gray-500">
                      {sourceIcon(msg.source)}
                      <span className="capitalize">{msg.source}</span>
                    </span>
                  )}
                  {isUser && (
                    <span className="flex items-center gap-1 text-xs text-gray-500 ml-auto">
                      <span className="capitalize">{msg.source}</span>
                      {sourceIcon(msg.source)}
                    </span>
                  )}
                </div>
                <div
                  className={`px-4 py-2.5 rounded-2xl text-sm whitespace-pre-wrap ${
                    isUser
                      ? 'bg-indigo-600 text-white rounded-br-md'
                      : 'bg-gray-100 text-gray-900 rounded-bl-md'
                  }`}
                >
                  {msg.content}
                </div>
                <p
                  className={`text-xs text-gray-400 mt-1 ${
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
      <div className="border-t border-gray-200 p-4">
        <div className="flex items-end gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            rows={1}
            className="flex-1 resize-none rounded-xl border border-gray-300 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
          />
          <button
            onClick={handleSend}
            disabled={!input.trim()}
            className="p-2.5 bg-indigo-600 text-white rounded-xl hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
