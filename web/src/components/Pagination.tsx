interface PaginationProps {
  page: number;
  perPage: number;
  total: number;
  onPageChange: (page: number) => void;
}

export default function Pagination({
  page,
  perPage,
  total,
  onPageChange,
}: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / perPage));

  const pages: number[] = [];
  const start = Math.max(1, page - 2);
  const end = Math.min(totalPages, page + 2);
  for (let i = start; i <= end; i++) {
    pages.push(i);
  }

  return (
    <div className="flex items-center justify-between py-3">
      <p className="text-xs font-mono text-gray-500">
        <span className="text-gray-400">{Math.min((page - 1) * perPage + 1, total)}</span>
        <span className="text-gray-600">{' - '}</span>
        <span className="text-gray-400">{Math.min(page * perPage, total)}</span>
        <span className="text-gray-600">{' of '}</span>
        <span className="text-neon-cyan/70">{total}</span>
      </p>
      <div className="flex items-center gap-1">
        <button
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          className="px-3 py-1.5 text-xs font-mono text-gray-400 bg-cyber-card border border-cyber-border rounded hover:border-neon-cyan/30 hover:text-neon-cyan disabled:opacity-30 disabled:cursor-not-allowed transition-all"
        >
          PREV
        </button>
        {pages.map((p) => (
          <button
            key={p}
            onClick={() => onPageChange(p)}
            className={`px-3 py-1.5 text-xs font-mono rounded border transition-all ${
              p === page
                ? 'bg-neon-cyan/10 text-neon-cyan border-neon-cyan/30'
                : 'text-gray-400 bg-cyber-card border-cyber-border hover:border-neon-cyan/30 hover:text-neon-cyan'
            }`}
          >
            {p}
          </button>
        ))}
        <button
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages}
          className="px-3 py-1.5 text-xs font-mono text-gray-400 bg-cyber-card border border-cyber-border rounded hover:border-neon-cyan/30 hover:text-neon-cyan disabled:opacity-30 disabled:cursor-not-allowed transition-all"
        >
          NEXT
        </button>
      </div>
    </div>
  );
}
