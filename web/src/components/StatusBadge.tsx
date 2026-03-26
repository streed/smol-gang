const statusConfig: Record<string, { bg: string; text: string; glow: string; dot: string }> = {
  pending: {
    bg: 'bg-neon-yellow/10 border-neon-yellow/30',
    text: 'text-neon-yellow',
    glow: '',
    dot: 'bg-neon-yellow',
  },
  provisioning: {
    bg: 'bg-neon-cyan/10 border-neon-cyan/30',
    text: 'text-neon-cyan',
    glow: '',
    dot: 'bg-neon-cyan animate-pulse',
  },
  running: {
    bg: 'bg-neon-green/10 border-neon-green/30',
    text: 'text-neon-green',
    glow: 'shadow-neon-green',
    dot: 'bg-neon-green animate-pulse',
  },
  completing: {
    bg: 'bg-neon-purple/10 border-neon-purple/30',
    text: 'text-neon-purple',
    glow: '',
    dot: 'bg-neon-purple animate-pulse',
  },
  completed: {
    bg: 'bg-gray-500/10 border-gray-500/30',
    text: 'text-gray-400',
    glow: '',
    dot: 'bg-gray-400',
  },
  failed: {
    bg: 'bg-neon-red/10 border-neon-red/30',
    text: 'text-neon-red',
    glow: 'shadow-neon-red',
    dot: 'bg-neon-red',
  },
  cancelled: {
    bg: 'bg-neon-orange/10 border-neon-orange/30',
    text: 'text-neon-orange',
    glow: '',
    dot: 'bg-neon-orange',
  },
};

interface StatusBadgeProps {
  status: string;
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  const config = statusConfig[status] || statusConfig.completed;

  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded border text-[11px] font-mono font-medium uppercase tracking-wider ${config.bg} ${config.text} ${config.glow}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${config.dot}`} />
      {status}
    </span>
  );
}
