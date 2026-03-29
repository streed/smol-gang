import { useEffect, useRef, useState, useCallback } from 'react';
import type { Message } from '../types';

interface UseWebSocketReturn {
  messages: Message[];
  connected: boolean;
  send: (content: string) => void;
}

export default function useWebSocket(workstreamId: string): UseWebSocketReturn {
  const [messages, setMessages] = useState<Message[]>([]);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsBase =
      import.meta.env.VITE_WS_URL || `${protocol}//${window.location.host}`;
    const token = localStorage.getItem('token');
    const url = `${wsBase}/api/v1/ws/workstreams/${workstreamId}?token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        const msg: Message = JSON.parse(event.data);
        // Ensure created_at is set for real-time messages
        if (!msg.created_at) {
          msg.created_at = new Date().toISOString();
        }
        setMessages((prev) => [...prev, msg]);
      } catch {
        // ignore non-JSON messages
      }
    };

    ws.onclose = () => {
      setConnected(false);
      reconnectTimerRef.current = setTimeout(() => {
        connect();
      }, 3000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [workstreamId]);

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

  const send = useCallback((content: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ content }));
    }
  }, []);

  return { messages, connected, send };
}
