import { useEffect, useRef, useState, useCallback } from 'react';

interface TerminalProps {
  workstreamId: string;
  container?: string; // 'app' | 'agent' | 'dind', defaults to 'app'
}

export default function Terminal({ workstreamId, container = 'app' }: TerminalProps) {
  const termRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [connected, setConnected] = useState(false);
  const [buffer, setBuffer] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  const connect = useCallback(() => {
    const wsUrl = import.meta.env.VITE_WS_URL || window.location.origin.replace('http', 'ws');
    const token = localStorage.getItem('token');
    const url = `${wsUrl}/api/v1/ws/workstreams/${workstreamId}/terminal/${container}?token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onmessage = (event) => {
      const text = typeof event.data === 'string'
        ? event.data
        : new TextDecoder().decode(event.data);
      setBuffer((prev) => [...prev.slice(-500), text]); // keep last 500 chunks
    };

    ws.onclose = () => {
      setConnected(false);
      // Reconnect after 3 seconds
      reconnectTimerRef.current = setTimeout(() => {
        connect();
      }, 3000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [workstreamId, container]);

  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [connect]);

  useEffect(() => {
    // Auto-scroll to bottom
    if (termRef.current) {
      termRef.current.scrollTop = termRef.current.scrollHeight;
    }
  }, [buffer]);

  const sendInput = (input: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(input + '\n');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      const value = inputRef.current?.value || '';
      sendInput(value);
      if (inputRef.current) inputRef.current.value = '';
    }
  };

  return (
    <div className="flex flex-col h-full bg-cyber-bg rounded-lg overflow-hidden border border-cyber-border">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 bg-cyber-card border-b border-cyber-border">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${connected ? 'bg-neon-green' : 'bg-neon-red'}`} />
          <span className="text-sm text-gray-300 font-mono">
            {container}@workstream
          </span>
        </div>
        <span className="text-xs text-gray-500 font-mono">
          {connected ? 'Connected' : 'Reconnecting...'}
        </span>
      </div>

      {/* Terminal output */}
      <div
        ref={termRef}
        className="flex-1 p-3 overflow-y-auto font-mono text-sm text-neon-green whitespace-pre-wrap"
        onClick={() => inputRef.current?.focus()}
      >
        {buffer.map((chunk, i) => (
          <span key={i}>{chunk}</span>
        ))}
      </div>

      {/* Input */}
      <div className="flex items-center px-3 py-2 bg-cyber-card border-t border-cyber-border">
        <span className="text-neon-green font-mono text-sm mr-2">$</span>
        <input
          ref={inputRef}
          type="text"
          className="flex-1 bg-transparent text-neon-green font-mono text-sm outline-none placeholder-gray-600"
          placeholder={connected ? 'Type a command...' : 'Connecting...'}
          disabled={!connected}
          onKeyDown={handleKeyDown}
          autoFocus
        />
      </div>
    </div>
  );
}
